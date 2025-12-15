package cmd

import (
	"context"
	"fmt"

	"github.com/kerren/op-ssh-manager/internal/agentcfg"
	"github.com/kerren/op-ssh-manager/internal/cli"
	"github.com/kerren/op-ssh-manager/internal/config"
	"github.com/kerren/op-ssh-manager/internal/fs"
	"github.com/kerren/op-ssh-manager/internal/inventory"
	"github.com/kerren/op-ssh-manager/internal/keys"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/kerren/op-ssh-manager/internal/op"
	"github.com/kerren/op-ssh-manager/internal/sshconfig"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize SSH configuration from 1Password",
	Long: `Sync discovers servers and SSH keys from 1Password and generates
the local SSH configuration files.

This command:
  1. Lists all server items tagged with 'op-ssh-manager/server'
  2. Fetches details and validates each server item
  3. Resolves SSH key references (skipping inaccessible keys)
  4. Exports public keys to the local cache
  5. Generates ~/.ssh/op-ssh-manager.conf
  6. Updates the 1Password SSH agent config (agent.toml)
  7. Ensures the Include directive in ~/.ssh/config
  8. Stores a copy of the config in 1Password for history`,
	RunE: runSync,
}

var (
	skipAgentToml     bool
	skipConfigHistory bool
)

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().BoolVar(&skipAgentToml, "skip-agent-toml", false, "skip updating 1Password agent.toml")
	syncCmd.Flags().BoolVar(&skipConfigHistory, "skip-history", false, "skip storing config in 1Password")
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cfg := GetConfig()
	output := cli.NewOutput(IsJSONOutput())

	// Create 1Password client
	opClient := op.NewClient()

	// Check if op is available
	if !opClient.IsInstalled() {
		output.PrintError(fmt.Errorf("1Password CLI (op) not found in PATH"))
		return fmt.Errorf("op not installed")
	}

	// Check if signed in
	if !opClient.IsSignedIn(ctx) {
		output.PrintError(fmt.Errorf("not signed in to 1Password"))
		output.PrintLine("\nRun 'op signin' or enable app integration in 1Password desktop app")
		return fmt.Errorf("not signed in")
	}

	// Build tags config
	tags := inventory.TagConfig{
		Server: cfg.Tags.Server,
		Jump:   cfg.Tags.Jump,
		Key:    cfg.Tags.Key,
	}

	// Build inventory
	log.Info("discovering servers from 1Password")
	builder := inventory.NewBuilder(opClient, tags, cfg.Vaults)

	buildResult, keyResult, err := builder.BuildWithKeys(ctx)
	if err != nil {
		output.PrintError(err)
		return err
	}

	// Report parse errors
	if len(buildResult.ParseErrors) > 0 {
		output.PrintWarning("failed to parse %d items:", len(buildResult.ParseErrors))
		for _, pe := range buildResult.ParseErrors {
			output.PrintLine("  - %s (%s): %v", pe.ItemTitle, pe.ItemID, pe.Error)
		}
	}

	// Report validation errors
	if buildResult.Validation != nil && buildResult.Validation.Errors.HasErrors() {
		output.PrintValidationErrors(buildResult.Validation.Errors)
		return fmt.Errorf("validation failed")
	}

	// Report validation warnings
	if buildResult.Validation != nil {
		for _, w := range buildResult.Validation.Warnings {
			output.PrintWarning("%s", w)
		}
	}

	// Report missing keys
	if keyResult != nil && len(keyResult.Missing) > 0 {
		output.PrintWarning("%d keys could not be resolved:", len(keyResult.Missing))
		for _, mk := range keyResult.Missing {
			output.PrintLine("  - %s (server: %s): %s", mk.Ref.String(), mk.ServerAlias, mk.Reason)
		}
	}

	// Dry run check
	if IsDryRun() {
		output.PrintLine("\n[DRY RUN] Would perform the following actions:")
		output.PrintLine("  - Write %d public keys to %s", len(keyResult.Resolved), cfg.Paths.KeyCacheDir)
		output.PrintLine("  - Generate SSH config with %d hosts", buildResult.ServerCount)
		output.PrintLine("  - Write config to %s", cfg.Paths.ManagedConfig)
		if !skipAgentToml {
			output.PrintLine("  - Update agent.toml at %s", cfg.Paths.AgentToml)
		}
		output.PrintLine("  - Ensure Include directive in %s", cfg.Paths.MainSSHConfig)
		return nil
	}

	// Write public keys to cache
	log.Info("writing public keys to cache")
	keyCache := keys.NewCache(cfg.Paths.KeyCacheDir)
	if err := keyCache.WriteKeys(keyResult.Resolved); err != nil {
		output.PrintError(err)
		return err
	}

	// Update local paths in inventory for config generation
	for _, resolved := range keyResult.Resolved {
		buildResult.Inventory.Keys[resolved.SourceRef.UniqueKey()] = resolved
	}

	// Get current user email for per-user mappings
	var userEmail string
	if whoami, err := opClient.Whoami(ctx); err == nil {
		userEmail = whoami.Email
	}

	// Generate SSH config
	log.Info("generating SSH configuration")
	genConfig := sshconfig.DefaultGeneratorConfig()
	genConfig.UserEmail = userEmail
	if cfg.SSH.IdentityAgent != "" {
		genConfig.IdentityAgent = cfg.SSH.IdentityAgent
	}

	generator := sshconfig.NewGenerator(genConfig)
	genResult, err := generator.Generate(buildResult.Inventory)
	if err != nil {
		output.PrintError(err)
		return err
	}

	// Write managed config file
	log.Info("writing managed SSH config", "path", cfg.Paths.ManagedConfig)
	if cfg.SSH.Mode == "include" {
		if err := writeSSHConfigWithInclude(cfg, genResult.Content); err != nil {
			output.PrintError(err)
			return err
		}
	} else {
		if err := writeSSHConfigInline(cfg, genResult.Content); err != nil {
			output.PrintError(err)
			return err
		}
	}

	// Update agent.toml
	if !skipAgentToml {
		log.Info("updating 1Password agent config", "path", cfg.Paths.AgentToml)
		agentMgr := agentcfg.NewAgentTomlManager(cfg.Paths.AgentToml)
		if err := agentMgr.UpdateManagedBlock(buildResult.Inventory); err != nil {
			output.PrintWarning("failed to update agent.toml: %v", err)
			// Don't fail on agent.toml errors - it's optional
		}
	}

	// Store config history in 1Password
	if !skipConfigHistory {
		log.Info("storing config history in 1Password")
		meta := op.DocumentMetadata{
			Timestamp: genResult.GeneratedAt,
			HostCount: genResult.HostCount,
			KeyCount:  len(keyResult.Resolved),
			Version:   Version,
		}

		// Use first vault if specified, or empty for default
		vault := ""
		if len(cfg.Vaults) > 0 {
			vault = cfg.Vaults[0]
		}

		if err := opClient.StoreConfigHistory(ctx, cfg.Tags.ConfigHistory, vault, genResult.Content, meta); err != nil {
			output.PrintWarning("failed to store config history: %v", err)
			// Don't fail on history storage errors
		}
	}

	// Print summary
	summary := &cli.Summary{
		Operation: "Sync Complete",
		DryRun:    false,
		Items: map[string]int{
			"Servers":       buildResult.ServerCount,
			"Jump hosts":    buildResult.JumpHostCount,
			"Keys resolved": len(keyResult.Resolved),
			"Keys missing":  len(keyResult.Missing),
		},
	}

	if buildResult.Validation != nil && len(buildResult.Validation.Warnings) > 0 {
		summary.Warnings = buildResult.Validation.Warnings
	}

	output.PrintSummary(summary)

	return nil
}

func writeSSHConfigWithInclude(cfg *config.Config, content []byte) error {
	// Write the managed config file
	if err := fs.AtomicWrite(cfg.Paths.ManagedConfig, content, 0600); err != nil {
		return err
	}

	// Ensure Include directive exists
	includeMgr := sshconfig.NewIncludeManager(
		cfg.Paths.MainSSHConfig,
		cfg.Paths.ManagedConfig,
		cfg.SSH.Precedence,
	)

	modified, err := includeMgr.EnsureInclude()
	if err != nil {
		return err
	}

	if modified {
		log.Info("added Include directive to SSH config")
	}

	return nil
}

func writeSSHConfigInline(cfg *config.Config, content []byte) error {
	blockMgr := sshconfig.NewBlockManager(
		cfg.Paths.MainSSHConfig,
		cfg.SSH.Precedence,
	)

	return blockMgr.UpdateManagedBlock(content)
}
