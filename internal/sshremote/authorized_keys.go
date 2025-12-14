package sshremote

import (
	"context"
	"fmt"
	"strings"
	"time"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/kerren/op-ssh-manager/internal/model"
)

const (
	// ManagedBegin marks the start of the managed section
	ManagedBegin = "# OP_SSH_MANAGER_MANAGED_BEGIN"

	// ManagedEnd marks the end of the managed section
	ManagedEnd = "# OP_SSH_MANAGER_MANAGED_END"

	// UnmanagedBelow marks the section below which keys should be preserved
	UnmanagedBelow = "# OP_SSH_MANAGER_UNMANAGED_BELOW"

	// DefaultAuthorizedKeysPath is the default path to authorized_keys
	DefaultAuthorizedKeysPath = ".ssh/authorized_keys"
)

// AuthorizedKeysManager manages authorized_keys files on remote servers
type AuthorizedKeysManager struct {
	Runner *Runner
}

// NewAuthorizedKeysManager creates a new manager
func NewAuthorizedKeysManager() *AuthorizedKeysManager {
	return &AuthorizedKeysManager{
		Runner: NewRunner(),
	}
}

// ApplyOptions contains options for applying authorized keys
type ApplyOptions struct {
	// RemoteUser is the Linux user to update keys for
	RemoteUser string

	// DryRun only shows what would be done
	DryRun bool

	// Backup creates a backup before modifying
	Backup bool

	// Keys are the public keys to apply
	Keys []*model.ResolvedKey
}

// ApplyResult contains the result of applying keys
type ApplyResult struct {
	ServerAlias     string
	RemoteUser      string
	KeysAdded       int
	KeysRemoved     int
	BackupPath      string
	DryRun          bool
	CurrentContent  string
	ProposedContent string
	Diff            string
}

// Apply applies the desired keys to a server's authorized_keys
func (m *AuthorizedKeysManager) Apply(ctx context.Context, alias string, opts *ApplyOptions) (*ApplyResult, error) {
	result := &ApplyResult{
		ServerAlias: alias,
		RemoteUser:  opts.RemoteUser,
		DryRun:      opts.DryRun,
	}

	// Determine authorized_keys path
	authKeysPath := DefaultAuthorizedKeysPath
	if opts.RemoteUser != "" {
		authKeysPath = fmt.Sprintf("/home/%s/.ssh/authorized_keys", opts.RemoteUser)
	}

	// Read current authorized_keys
	current, err := m.readAuthorizedKeys(ctx, alias, authKeysPath)
	if err != nil {
		// File might not exist
		log.Debug("could not read authorized_keys", "error", err)
		current = ""
	}
	result.CurrentContent = current

	// Parse into sections
	before, managed, after, unmanaged := parseAuthorizedKeys(current)

	// Build new managed section
	newManaged := buildManagedSection(opts.Keys)

	// Calculate diff
	oldKeys := extractKeys(managed)
	newKeys := extractKeys(newManaged)
	result.KeysAdded = countNewKeys(oldKeys, newKeys)
	result.KeysRemoved = countRemovedKeys(oldKeys, newKeys)

	// Build new content
	newContent := buildAuthorizedKeys(before, newManaged, after, unmanaged)
	result.ProposedContent = newContent

	// Generate diff for display
	result.Diff = generateDiff(current, newContent)

	if opts.DryRun {
		return result, nil
	}

	// Ensure .ssh directory exists with correct permissions
	sshDir := ".ssh"
	if opts.RemoteUser != "" {
		sshDir = fmt.Sprintf("/home/%s/.ssh", opts.RemoteUser)
	}
	if err := m.Runner.MkdirP(ctx, alias, sshDir, "700"); err != nil {
		return result, apperrors.Wrap(apperrors.ErrSSH, "failed to create .ssh directory", err)
	}

	// Create backup if requested
	if opts.Backup && current != "" {
		backupPath := fmt.Sprintf("%s.bak.%d", authKeysPath, time.Now().Unix())
		if err := m.Runner.WriteFile(ctx, alias, backupPath, []byte(current), "600"); err != nil {
			log.Warn("failed to create backup", "error", err)
		} else {
			result.BackupPath = backupPath
			log.Info("created backup", "path", backupPath)
		}
	}

	// Write new authorized_keys
	if err := m.Runner.WriteFile(ctx, alias, authKeysPath, []byte(newContent), "600"); err != nil {
		return result, apperrors.Wrap(apperrors.ErrSSH, "failed to write authorized_keys", err)
	}

	log.Info("updated authorized_keys",
		"alias", alias,
		"added", result.KeysAdded,
		"removed", result.KeysRemoved)

	return result, nil
}

// readAuthorizedKeys reads the authorized_keys file from a remote server
func (m *AuthorizedKeysManager) readAuthorizedKeys(ctx context.Context, alias, path string) (string, error) {
	content, err := m.Runner.ReadFile(ctx, alias, path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// parseAuthorizedKeys parses an authorized_keys file into sections
func parseAuthorizedKeys(content string) (before, managed, after, unmanaged string) {
	lines := strings.Split(content, "\n")

	var beforeLines, managedLines, afterLines, unmanagedLines []string
	section := "before" // before, managed, after, unmanaged

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch section {
		case "before":
			if trimmed == ManagedBegin {
				section = "managed"
				continue
			}
			if trimmed == UnmanagedBelow {
				section = "unmanaged"
				unmanagedLines = append(unmanagedLines, line)
				continue
			}
			beforeLines = append(beforeLines, line)

		case "managed":
			if trimmed == ManagedEnd {
				section = "after"
				continue
			}
			managedLines = append(managedLines, line)

		case "after":
			if trimmed == UnmanagedBelow {
				section = "unmanaged"
				unmanagedLines = append(unmanagedLines, line)
				continue
			}
			afterLines = append(afterLines, line)

		case "unmanaged":
			unmanagedLines = append(unmanagedLines, line)
		}
	}

	before = strings.TrimSpace(strings.Join(beforeLines, "\n"))
	managed = strings.TrimSpace(strings.Join(managedLines, "\n"))
	after = strings.TrimSpace(strings.Join(afterLines, "\n"))
	unmanaged = strings.TrimSpace(strings.Join(unmanagedLines, "\n"))

	return
}

// buildManagedSection builds the managed section content
func buildManagedSection(keys []*model.ResolvedKey) string {
	var lines []string

	lines = append(lines, "# Managed by op-ssh-manager - DO NOT EDIT")
	lines = append(lines, fmt.Sprintf("# Updated: %s", time.Now().Format(time.RFC3339)))

	for _, key := range keys {
		// Add comment with key info
		lines = append(lines, fmt.Sprintf("# Key: %s (%s)", key.Title, key.ItemID))
		lines = append(lines, strings.TrimSpace(key.PublicKey))
	}

	return strings.Join(lines, "\n")
}

// buildAuthorizedKeys rebuilds the complete authorized_keys file
func buildAuthorizedKeys(before, managed, after, unmanaged string) string {
	var parts []string

	if before != "" {
		parts = append(parts, before)
	}

	// Managed section with markers
	managedSection := fmt.Sprintf("%s\n%s\n%s", ManagedBegin, managed, ManagedEnd)
	parts = append(parts, managedSection)

	if after != "" {
		parts = append(parts, after)
	}

	if unmanaged != "" {
		parts = append(parts, unmanaged)
	}

	content := strings.Join(parts, "\n\n")

	// Ensure file ends with newline
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return content
}

// extractKeys extracts public key strings from a section
func extractKeys(section string) map[string]bool {
	keys := make(map[string]bool)
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Normalize by taking just the key part (type + data, ignore comment)
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			keyID := parts[0] + " " + parts[1]
			keys[keyID] = true
		}
	}
	return keys
}

// countNewKeys counts keys in new that aren't in old
func countNewKeys(old, new map[string]bool) int {
	count := 0
	for k := range new {
		if !old[k] {
			count++
		}
	}
	return count
}

// countRemovedKeys counts keys in old that aren't in new
func countRemovedKeys(old, new map[string]bool) int {
	count := 0
	for k := range old {
		if !new[k] {
			count++
		}
	}
	return count
}

// generateDiff generates a simple diff between old and new content
func generateDiff(old, new string) string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	var diff []string

	// Very simple diff - just show additions and removals
	oldSet := make(map[string]bool)
	for _, line := range oldLines {
		if line != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			oldSet[line] = true
		}
	}

	newSet := make(map[string]bool)
	for _, line := range newLines {
		if line != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			newSet[line] = true
		}
	}

	// Removed lines
	for line := range oldSet {
		if !newSet[line] {
			diff = append(diff, "- "+line)
		}
	}

	// Added lines
	for line := range newSet {
		if !oldSet[line] {
			diff = append(diff, "+ "+line)
		}
	}

	return strings.Join(diff, "\n")
}
