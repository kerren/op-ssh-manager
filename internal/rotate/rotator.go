package rotate

import (
	"context"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/kerren/op-ssh-manager/internal/model"
	"github.com/kerren/op-ssh-manager/internal/op"
	"github.com/kerren/op-ssh-manager/internal/sshremote"
)

// Rotator handles SSH key rotation
type Rotator struct {
	OPClient  *op.Client
	SSHRunner *sshremote.Runner
	AuthKeys  *sshremote.AuthorizedKeysManager
}

// NewRotator creates a new key rotator
func NewRotator(opClient *op.Client) *Rotator {
	return &Rotator{
		OPClient:  opClient,
		SSHRunner: sshremote.NewRunner(),
		AuthKeys:  sshremote.NewAuthorizedKeysManager(),
	}
}

// RotateOptions contains options for key rotation
type RotateOptions struct {
	// OldKeyRef is the reference to the key to rotate
	OldKeyRef model.KeyRef

	// NewKeyTitle is the title for the new key (optional, defaults to old key title + timestamp)
	NewKeyTitle string

	// KeyType is the type of SSH key to generate (optional, defaults to ed25519)
	KeyType op.SSHKeyType

	// DryRun only shows what would be done without making changes
	DryRun bool

	// NoDelete keeps the old key after rotation instead of archiving it
	NoDelete bool

	// Force skips confirmation prompts
	Force bool

	// Inventory is the server inventory to use
	Inventory *model.Inventory

	// Vaults to search for the key
	Vaults []string

	// KeyTag is the tag used for SSH keys (for copying to new key)
	KeyTag string
}

// RotateResult contains the results of a key rotation
type RotateResult struct {
	// OldKey is the key that was rotated
	OldKey *model.ResolvedKey

	// NewKey is the newly created key
	NewKey *model.ResolvedKey

	// ServersUpdated is a list of servers that were updated
	ServersUpdated []string

	// ServersFailed contains servers that failed to update
	ServersFailed map[string]error

	// ConnectionsVerified is the number of successful connection tests
	ConnectionsVerified int

	// ConnectionsFailed contains servers with failed connection tests
	ConnectionsFailed map[string]error

	// OldKeyArchived indicates if the old key was archived
	OldKeyArchived bool

	// DryRun indicates if this was a dry run
	DryRun bool
}

// Phase represents a rotation phase
type Phase string

const (
	PhaseResolveOldKey    Phase = "resolve_old_key"
	PhaseFindServers      Phase = "find_servers"
	PhaseCreateNewKey     Phase = "create_new_key"
	PhaseAddNewKey        Phase = "add_new_key"
	PhaseVerifyConnection Phase = "verify_connection"
	PhaseRemoveOldKey     Phase = "remove_old_key"
	PhaseArchiveOldKey    Phase = "archive_old_key"
)

// ProgressCallback is called during rotation to report progress
type ProgressCallback func(phase Phase, message string, current, total int)

// Rotate performs the key rotation
func (r *Rotator) Rotate(ctx context.Context, opts RotateOptions, progress ProgressCallback) (*RotateResult, error) {
	result := &RotateResult{
		ServersFailed:     make(map[string]error),
		ConnectionsFailed: make(map[string]error),
		DryRun:            opts.DryRun,
	}

	if progress == nil {
		progress = func(phase Phase, message string, current, total int) {}
	}

	// Phase 1: Resolve the old key
	progress(PhaseResolveOldKey, "Resolving old key...", 0, 1)
	oldKey, err := r.resolveKey(ctx, opts.OldKeyRef, opts.Vaults)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrNotFound, "failed to resolve old key", err)
	}
	result.OldKey = oldKey
	log.Info("resolved old key", "title", oldKey.Title, "id", oldKey.ItemID)
	progress(PhaseResolveOldKey, fmt.Sprintf("Resolved: %s", oldKey.Title), 1, 1)

	// Phase 2: Find all servers using this key
	progress(PhaseFindServers, "Finding servers using this key...", 0, 1)
	servers := r.findServersUsingKey(opts.Inventory, oldKey)
	if len(servers) == 0 {
		return nil, apperrors.New(apperrors.ErrNotFound, "no servers found using this key")
	}
	log.Info("found servers using key", "count", len(servers))
	progress(PhaseFindServers, fmt.Sprintf("Found %d servers", len(servers)), 1, 1)

	// Phase 3: Create new key
	progress(PhaseCreateNewKey, "Creating new SSH key...", 0, 1)
	newKey, err := r.createNewKey(ctx, opts, oldKey)
	if err != nil {
		return nil, err
	}
	result.NewKey = newKey
	log.Info("created new key", "title", newKey.Title, "id", newKey.ItemID)
	progress(PhaseCreateNewKey, fmt.Sprintf("Created: %s", newKey.Title), 1, 1)

	// Phase 4: Add new key to all servers
	for i, server := range servers {
		progress(PhaseAddNewKey, fmt.Sprintf("Adding key to %s...", server.Alias), i, len(servers))

		if opts.DryRun {
			log.Info("[DRY RUN] would add new key to server", "alias", server.Alias)
			result.ServersUpdated = append(result.ServersUpdated, server.Alias)
			continue
		}

		err := r.addKeyToServer(ctx, server, newKey)
		if err != nil {
			log.Error("failed to add key to server", "alias", server.Alias, "error", err)
			result.ServersFailed[server.Alias] = err
			continue
		}

		result.ServersUpdated = append(result.ServersUpdated, server.Alias)
		log.Info("added new key to server", "alias", server.Alias)
	}
	progress(PhaseAddNewKey, fmt.Sprintf("Added key to %d servers", len(result.ServersUpdated)), len(servers), len(servers))

	// Check if we can proceed with verification
	if len(result.ServersUpdated) == 0 {
		return nil, apperrors.New(apperrors.ErrSSH, "failed to add new key to any server, aborting rotation")
	}

	// Phase 5: Verify connections using new key
	for i, alias := range result.ServersUpdated {
		progress(PhaseVerifyConnection, fmt.Sprintf("Testing connection to %s...", alias), i, len(result.ServersUpdated))

		if opts.DryRun {
			log.Info("[DRY RUN] would test connection to server", "alias", alias)
			result.ConnectionsVerified++
			continue
		}

		err := r.testConnection(ctx, alias)
		if err != nil {
			log.Error("connection test failed", "alias", alias, "error", err)
			result.ConnectionsFailed[alias] = err
			continue
		}

		result.ConnectionsVerified++
		log.Info("connection verified", "alias", alias)
	}
	progress(PhaseVerifyConnection, fmt.Sprintf("Verified %d connections", result.ConnectionsVerified), len(result.ServersUpdated), len(result.ServersUpdated))

	// Check if all connections verified before removing old key
	if result.ConnectionsVerified == 0 {
		return result, apperrors.New(apperrors.ErrSSH, "all connection tests failed, old key NOT removed for safety")
	}

	if len(result.ConnectionsFailed) > 0 {
		log.Warn("some connection tests failed, proceeding with caution",
			"verified", result.ConnectionsVerified,
			"failed", len(result.ConnectionsFailed))
	}

	// Phase 6: Remove old key from servers
	for i, alias := range result.ServersUpdated {
		progress(PhaseRemoveOldKey, fmt.Sprintf("Removing old key from %s...", alias), i, len(result.ServersUpdated))

		if opts.DryRun {
			log.Info("[DRY RUN] would remove old key from server", "alias", alias)
			continue
		}

		// Only remove from servers where connection was verified
		if _, failed := result.ConnectionsFailed[alias]; failed {
			log.Warn("skipping old key removal due to failed connection test", "alias", alias)
			continue
		}

		err := r.removeKeyFromServer(ctx, opts.Inventory.Servers[alias], oldKey)
		if err != nil {
			log.Error("failed to remove old key from server", "alias", alias, "error", err)
			// Continue anyway, old key removal failure is not critical
		} else {
			log.Info("removed old key from server", "alias", alias)
		}
	}
	progress(PhaseRemoveOldKey, "Old key removed from servers", len(result.ServersUpdated), len(result.ServersUpdated))

	// Phase 7: Archive old key in 1Password
	if !opts.NoDelete {
		progress(PhaseArchiveOldKey, "Archiving old key in 1Password...", 0, 1)

		if opts.DryRun {
			log.Info("[DRY RUN] would archive old key", "id", oldKey.ItemID)
		} else {
			err := r.OPClient.ArchiveItem(ctx, oldKey.ItemID, oldKey.VaultID)
			if err != nil {
				log.Error("failed to archive old key", "error", err)
				// Don't fail the whole operation, just report it
			} else {
				result.OldKeyArchived = true
				log.Info("archived old key", "id", oldKey.ItemID)
			}
		}
		progress(PhaseArchiveOldKey, "Old key archived", 1, 1)
	}

	return result, nil
}

// resolveKey resolves a key reference to a full ResolvedKey
func (r *Rotator) resolveKey(ctx context.Context, ref model.KeyRef, vaults []string) (*model.ResolvedKey, error) {
	var vaultArg string
	if ref.Vault != "" {
		vaultArg = ref.Vault
	} else if len(vaults) == 1 {
		vaultArg = vaults[0]
	}

	sshKey, err := r.OPClient.GetSSHKey(ctx, ref.Item, vaultArg)
	if err != nil {
		return nil, err
	}

	return &model.ResolvedKey{
		ItemID:    sshKey.ID,
		VaultID:   sshKey.Vault.ID,
		Title:     sshKey.Title,
		PublicKey: sshKey.PublicKey,
		KeyType:   sshKey.KeyType,
		SourceRef: ref,
	}, nil
}

// findServersUsingKey finds all servers in the inventory that use the given key
func (r *Rotator) findServersUsingKey(inv *model.Inventory, key *model.ResolvedKey) []*model.ServerRecord {
	var servers []*model.ServerRecord

	for _, server := range inv.Servers {
		// Check server-level keys
		for _, keyRef := range server.Keys {
			if r.keyMatchesRef(key, keyRef) {
				servers = append(servers, server)
				break
			}
		}
	}

	return servers
}

// keyMatchesRef checks if a resolved key matches a key reference
func (r *Rotator) keyMatchesRef(key *model.ResolvedKey, ref model.KeyRef) bool {
	// Match by item ID
	if key.ItemID == ref.Item {
		return true
	}

	// Match by title
	if strings.EqualFold(key.Title, ref.Item) {
		return true
	}

	// Match by vault + item
	if ref.Vault != "" && key.VaultID == ref.Vault && key.ItemID == ref.Item {
		return true
	}

	return false
}

// createNewKey creates a new SSH key based on the old key
func (r *Rotator) createNewKey(ctx context.Context, opts RotateOptions, oldKey *model.ResolvedKey) (*model.ResolvedKey, error) {
	if opts.DryRun {
		// Return a fake key for dry run
		return &model.ResolvedKey{
			ItemID:    "dry-run-new-key-id",
			VaultID:   oldKey.VaultID,
			Title:     r.generateNewKeyTitle(opts.NewKeyTitle, oldKey.Title),
			PublicKey: "ssh-ed25519 AAAAC3... (dry-run placeholder)",
			KeyType:   "ssh-ed25519",
		}, nil
	}

	newTitle := r.generateNewKeyTitle(opts.NewKeyTitle, oldKey.Title)

	keyType := opts.KeyType
	if keyType == "" {
		// Try to match the old key type
		keyType = r.detectKeyType(oldKey.KeyType)
	}

	createOpts := op.CreateSSHKeyOptions{
		Title:   newTitle,
		Vault:   oldKey.VaultID,
		KeyType: keyType,
	}

	// Add key tag if specified
	if opts.KeyTag != "" {
		createOpts.Tags = []string{opts.KeyTag}
	}

	sshKey, err := r.OPClient.CreateSSHKey(ctx, createOpts)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrOPExec, "failed to create new SSH key", err)
	}

	return &model.ResolvedKey{
		ItemID:    sshKey.ID,
		VaultID:   sshKey.Vault.ID,
		Title:     sshKey.Title,
		PublicKey: sshKey.PublicKey,
		KeyType:   sshKey.KeyType,
	}, nil
}

// generateNewKeyTitle generates a title for the new key
func (r *Rotator) generateNewKeyTitle(customTitle, oldTitle string) string {
	if customTitle != "" {
		return customTitle
	}

	// Generate a new title based on the old one with a timestamp
	timestamp := time.Now().Format("2006-01-02")

	// If old title has a date suffix, replace it
	if idx := strings.LastIndex(oldTitle, " (rotated "); idx != -1 {
		return oldTitle[:idx] + fmt.Sprintf(" (rotated %s)", timestamp)
	}

	return fmt.Sprintf("%s (rotated %s)", oldTitle, timestamp)
}

// detectKeyType detects the SSH key type from the key type string
func (r *Rotator) detectKeyType(keyType string) op.SSHKeyType {
	switch strings.ToLower(keyType) {
	case "ssh-rsa":
		return op.SSHKeyTypeRSA4096
	case "ssh-ed25519":
		return op.SSHKeyTypeEd25519
	default:
		return op.SSHKeyTypeEd25519
	}
}

// addKeyToServer adds the new key to a server's authorized_keys
func (r *Rotator) addKeyToServer(ctx context.Context, server *model.ServerRecord, newKey *model.ResolvedKey) error {
	// Read current authorized_keys
	authKeysPath := sshremote.DefaultAuthorizedKeysPath
	if server.DefaultUser != "" {
		authKeysPath = fmt.Sprintf("/home/%s/.ssh/authorized_keys", server.DefaultUser)
	}

	current, err := r.SSHRunner.ReadFile(ctx, server.Alias, authKeysPath)
	if err != nil {
		// File might not exist
		log.Debug("could not read authorized_keys", "error", err)
		current = nil
	}

	// Check if key already exists
	if strings.Contains(string(current), strings.TrimSpace(newKey.PublicKey)) {
		log.Info("new key already exists on server", "alias", server.Alias)
		return nil
	}

	// Append the new key with a comment
	keyLine := fmt.Sprintf("\n# Added by op-ssh-manager rotation: %s (%s)\n%s\n",
		newKey.Title, newKey.ItemID, strings.TrimSpace(newKey.PublicKey))

	newContent := string(current) + keyLine

	// Ensure .ssh directory exists
	sshDir := ".ssh"
	if server.DefaultUser != "" {
		sshDir = fmt.Sprintf("/home/%s/.ssh", server.DefaultUser)
	}
	if err := r.SSHRunner.MkdirP(ctx, server.Alias, sshDir, "700"); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Write the updated file
	if err := r.SSHRunner.WriteFile(ctx, server.Alias, authKeysPath, []byte(newContent), "600"); err != nil {
		return fmt.Errorf("failed to write authorized_keys: %w", err)
	}

	return nil
}

// testConnection tests the SSH connection to a server
func (r *Rotator) testConnection(ctx context.Context, alias string) error {
	return r.SSHRunner.TestConnection(ctx, alias)
}

// removeKeyFromServer removes the old key from a server's authorized_keys
func (r *Rotator) removeKeyFromServer(ctx context.Context, server *model.ServerRecord, oldKey *model.ResolvedKey) error {
	// Read current authorized_keys
	authKeysPath := sshremote.DefaultAuthorizedKeysPath
	if server.DefaultUser != "" {
		authKeysPath = fmt.Sprintf("/home/%s/.ssh/authorized_keys", server.DefaultUser)
	}

	current, err := r.SSHRunner.ReadFile(ctx, server.Alias, authKeysPath)
	if err != nil {
		return fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	// Extract the key data portion (type + base64) to match against
	oldKeyParts := strings.Fields(strings.TrimSpace(oldKey.PublicKey))
	if len(oldKeyParts) < 2 {
		return fmt.Errorf("invalid old key format")
	}
	oldKeyData := oldKeyParts[0] + " " + oldKeyParts[1]

	// Filter out lines containing the old key
	lines := strings.Split(string(current), "\n")
	var newLines []string
	skipNext := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comment lines that reference the old key
		if strings.Contains(trimmed, oldKey.ItemID) || strings.Contains(trimmed, oldKey.Title) {
			skipNext = true
			continue
		}

		// Skip the actual key line
		if strings.Contains(line, oldKeyData) {
			skipNext = false
			continue
		}

		if skipNext && trimmed == "" {
			skipNext = false
			continue
		}

		skipNext = false
		newLines = append(newLines, line)
	}

	newContent := strings.Join(newLines, "\n")

	// Ensure file ends with newline
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	// Write the updated file
	if err := r.SSHRunner.WriteFile(ctx, server.Alias, authKeysPath, []byte(newContent), "600"); err != nil {
		return fmt.Errorf("failed to write authorized_keys: %w", err)
	}

	return nil
}

// GetServersUsingKey returns a list of servers that use a specific key (for display purposes)
func (r *Rotator) GetServersUsingKey(inv *model.Inventory, keyRef model.KeyRef, vaults []string) ([]string, error) {
	ctx := context.Background()

	key, err := r.resolveKey(ctx, keyRef, vaults)
	if err != nil {
		return nil, err
	}

	servers := r.findServersUsingKey(inv, key)
	aliases := make([]string, len(servers))
	for i, s := range servers {
		aliases[i] = s.Alias
	}

	return aliases, nil
}
