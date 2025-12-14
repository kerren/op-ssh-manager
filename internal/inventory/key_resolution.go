package inventory

import (
	"context"
	"strings"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/kerren/op-ssh-manager/internal/model"
	"github.com/kerren/op-ssh-manager/internal/op"
)

// KeyResolver resolves key references to actual SSH keys
type KeyResolver struct {
	Client *op.Client
	Vaults []string // Optional vault scope
}

// NewKeyResolver creates a new KeyResolver
func NewKeyResolver(client *op.Client, vaults []string) *KeyResolver {
	return &KeyResolver{
		Client: client,
		Vaults: vaults,
	}
}

// ResolutionResult contains the results of key resolution
type ResolutionResult struct {
	Resolved []*model.ResolvedKey
	Missing  []*model.MissingKey
}

// ResolveKeys resolves a list of key references
func (r *KeyResolver) ResolveKeys(ctx context.Context, refs []model.KeyRef, serverAlias string) *ResolutionResult {
	result := &ResolutionResult{
		Resolved: make([]*model.ResolvedKey, 0),
		Missing:  make([]*model.MissingKey, 0),
	}

	// Deduplicate refs
	seen := make(map[string]bool)

	for _, ref := range refs {
		uniqueKey := ref.UniqueKey()
		if seen[uniqueKey] {
			continue
		}
		seen[uniqueKey] = true

		resolved, err := r.resolveKey(ctx, ref)
		if err != nil {
			log.Debug("failed to resolve key",
				"ref", ref.String(),
				"server", serverAlias,
				"error", err)

			result.Missing = append(result.Missing, &model.MissingKey{
				Ref:         ref,
				Reason:      classifyResolutionError(err),
				ServerAlias: serverAlias,
			})
			continue
		}

		resolved.SourceRef = ref
		result.Resolved = append(result.Resolved, resolved)
	}

	return result
}

// resolveKey resolves a single key reference
func (r *KeyResolver) resolveKey(ctx context.Context, ref model.KeyRef) (*model.ResolvedKey, error) {
	var vaultArg string
	if ref.Vault != "" {
		vaultArg = ref.Vault
	} else if len(r.Vaults) == 1 {
		vaultArg = r.Vaults[0]
	}

	// Get the SSH key item
	sshKey, err := r.Client.GetSSHKey(ctx, ref.Item, vaultArg)
	if err != nil {
		return nil, err
	}

	return &model.ResolvedKey{
		ItemID:    sshKey.ID,
		VaultID:   sshKey.Vault.ID,
		Title:     sshKey.Title,
		PublicKey: sshKey.PublicKey,
		KeyType:   sshKey.KeyType,
	}, nil
}

// ResolveAllKeys resolves all keys referenced in an inventory
func (r *KeyResolver) ResolveAllKeys(ctx context.Context, inv *model.Inventory) (*ResolutionResult, error) {
	allResult := &ResolutionResult{
		Resolved: make([]*model.ResolvedKey, 0),
		Missing:  make([]*model.MissingKey, 0),
	}

	// Collect all unique key references
	allRefs := make(map[string]model.KeyRef)
	refToServers := make(map[string][]string)

	for _, server := range inv.Servers {
		// Server-level keys
		for _, ref := range server.Keys {
			key := ref.UniqueKey()
			allRefs[key] = ref
			refToServers[key] = append(refToServers[key], server.Alias)
		}

		// User-level keys
		for _, user := range server.Users {
			for _, ref := range user.Keys {
				key := ref.UniqueKey()
				allRefs[key] = ref
				refToServers[key] = append(refToServers[key], server.Alias)
			}
		}
	}

	// Resolve each unique key
	for key, ref := range allRefs {
		servers := refToServers[key]
		serverDesc := servers[0]
		if len(servers) > 1 {
			serverDesc = servers[0] + " (and others)"
		}

		resolved, err := r.resolveKey(ctx, ref)
		if err != nil {
			log.Debug("failed to resolve key",
				"ref", ref.String(),
				"servers", servers,
				"error", err)

			allResult.Missing = append(allResult.Missing, &model.MissingKey{
				Ref:         ref,
				Reason:      classifyResolutionError(err),
				ServerAlias: serverDesc,
			})
			continue
		}

		resolved.SourceRef = ref
		allResult.Resolved = append(allResult.Resolved, resolved)

		// Add to inventory
		inv.Keys[key] = resolved
	}

	return allResult, nil
}

// classifyResolutionError classifies an error for user-friendly messages
func classifyResolutionError(err error) string {
	if apperrors.IsNotFound(err) {
		return "item not found"
	}
	if apperrors.IsNoAccess(err) {
		return "access denied (you may not have access to this key)"
	}
	if apperrors.IsAuthRequired(err) {
		return "authentication required"
	}
	if apperrors.IsInvalidSchema(err) {
		return "item is not a valid SSH key"
	}
	return err.Error()
}

// ExtractKeyFingerprint extracts the fingerprint from a public key
func ExtractKeyFingerprint(publicKey string) string {
	// Simple extraction - for full fingerprint calculation,
	// we'd need to parse and hash the key
	parts := strings.Fields(publicKey)
	if len(parts) >= 2 {
		// Return abbreviated key data
		keyData := parts[1]
		if len(keyData) > 20 {
			return keyData[:10] + "..." + keyData[len(keyData)-10:]
		}
		return keyData
	}
	return ""
}
