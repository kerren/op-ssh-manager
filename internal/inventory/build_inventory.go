package inventory

import (
	"context"
	"fmt"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/kerren/op-ssh-manager/internal/model"
	"github.com/kerren/op-ssh-manager/internal/op"
)

// Builder builds an inventory from 1Password
type Builder struct {
	Client  *op.Client
	Tags    TagConfig
	Vaults  []string
	Account string // 1Password account user UUID for multi-account support
}

// TagConfig holds the tag names for item discovery
type TagConfig struct {
	Server string
	Jump   string
	Key    string
}

// NewBuilder creates a new inventory builder
func NewBuilder(client *op.Client, tags TagConfig, vaults []string) *Builder {
	return &Builder{
		Client: client,
		Tags:   tags,
		Vaults: vaults,
	}
}

// WithAccount sets the account user UUID for the builder
// This is used to include account info in agent.toml for multi-account support
func (b *Builder) WithAccount(accountUserUUID string) *Builder {
	b.Account = accountUserUUID
	return b
}

// BuildResult contains the results of building an inventory
type BuildResult struct {
	Inventory     *model.Inventory
	Validation    *model.ValidationResult
	ParseErrors   []ParseError
	ServerCount   int
	JumpHostCount int
}

// ParseError represents an error parsing a single item
type ParseError struct {
	ItemID    string
	ItemTitle string
	Error     error
}

// Build builds the complete inventory from 1Password
func (b *Builder) Build(ctx context.Context) (*BuildResult, error) {
	result := &BuildResult{
		Inventory:   model.NewInventory(),
		ParseErrors: make([]ParseError, 0),
	}

	// List server items
	log.Debug("listing server items", "tag", b.Tags.Server)
	serverItems, err := b.Client.ListItemsByTag(ctx, b.Tags.Server, b.Vaults...)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrOPExec, "failed to list server items", err)
	}

	log.Info("found server items", "count", len(serverItems))

	// Also list jump host items if using a different tag
	var jumpItems []op.ItemSummary
	if b.Tags.Jump != "" && b.Tags.Jump != b.Tags.Server {
		jumpItems, err = b.Client.ListItemsByTag(ctx, b.Tags.Jump, b.Vaults...)
		if err != nil {
			log.Warn("failed to list jump host items", "error", err)
		}
	}

	// Combine and deduplicate
	allItems := make(map[string]op.ItemSummary)
	for _, item := range serverItems {
		allItems[item.ID] = item
	}
	for _, item := range jumpItems {
		allItems[item.ID] = item
	}

	// Fetch and parse each item
	for _, summary := range allItems {
		item, err := b.Client.GetItem(ctx, summary.ID, summary.Vault.ID)
		if err != nil {
			log.Warn("failed to get item details",
				"item_id", summary.ID,
				"title", summary.Title,
				"error", err)
			result.ParseErrors = append(result.ParseErrors, ParseError{
				ItemID:    summary.ID,
				ItemTitle: summary.Title,
				Error:     err,
			})
			continue
		}

		server, err := model.ParseServerRecord(item)
		if err != nil {
			log.Warn("failed to parse server record",
				"item_id", item.ID,
				"title", item.Title,
				"error", err)
			result.ParseErrors = append(result.ParseErrors, ParseError{
				ItemID:    item.ID,
				ItemTitle: item.Title,
				Error:     err,
			})
			continue
		}

		result.Inventory.AddServer(server)
		result.ServerCount++

		if server.IsJumpHost {
			result.JumpHostCount++
		}
	}

	// Validate the inventory
	result.Validation = model.ValidateInventory(result.Inventory)

	return result, nil
}

// BuildWithKeys builds the inventory and resolves all keys
func (b *Builder) BuildWithKeys(ctx context.Context) (*BuildResult, *ResolutionResult, error) {
	// Build base inventory
	buildResult, err := b.Build(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Resolve keys
	resolver := NewKeyResolver(b.Client, b.Vaults)
	if b.Account != "" {
		resolver.WithAccount(b.Account)
	}
	keyResult, err := resolver.ResolveAllKeys(ctx, buildResult.Inventory)
	if err != nil {
		return buildResult, nil, fmt.Errorf("failed to resolve keys: %w", err)
	}

	return buildResult, keyResult, nil
}

// Summary generates a summary of the build result
type Summary struct {
	TotalServers    int
	JumpHosts       int
	ResolvedKeys    int
	MissingKeys     int
	ParseErrors     int
	ValidationErrs  int
	Warnings        []string
}

// GetSummary returns a summary of the build and resolution results
func GetSummary(build *BuildResult, keys *ResolutionResult) *Summary {
	s := &Summary{
		TotalServers: build.ServerCount,
		JumpHosts:    build.JumpHostCount,
		ParseErrors:  len(build.ParseErrors),
	}

	if build.Validation != nil {
		s.ValidationErrs = len(build.Validation.Errors.Errors)
		s.Warnings = build.Validation.Warnings
	}

	if keys != nil {
		s.ResolvedKeys = len(keys.Resolved)
		s.MissingKeys = len(keys.Missing)
	}

	return s
}
