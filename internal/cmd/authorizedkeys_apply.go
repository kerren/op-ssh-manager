package cmd

import (
	"context"
	"fmt"

	"github.com/kerren/op-ssh-manager/internal/cli"
	"github.com/kerren/op-ssh-manager/internal/inventory"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/kerren/op-ssh-manager/internal/model"
	"github.com/kerren/op-ssh-manager/internal/op"
	"github.com/kerren/op-ssh-manager/internal/sshremote"
	"github.com/spf13/cobra"
)

var authorizedKeysCmd = &cobra.Command{
	Use:   "authorized-keys",
	Short: "Manage authorized_keys on remote servers",
	Long:  `Commands for managing SSH authorized_keys files on remote servers.`,
}

var authorizedKeysApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply authorized keys to servers",
	Long: `Apply the desired SSH public keys to remote servers' authorized_keys files.

This command:
  1. Connects to the specified server(s)
  2. Reads the current authorized_keys file
  3. Updates only the managed section (between markers)
  4. Preserves all unmanaged keys

Examples:
  op-ssh-manager authorized-keys apply --server prod-db
  op-ssh-manager authorized-keys apply --all
  op-ssh-manager authorized-keys apply --server staging --dry-run`,
	RunE: runAuthorizedKeysApply,
}

var (
	applyServer string
	applyAll    bool
	applyBackup bool
)

func init() {
	rootCmd.AddCommand(authorizedKeysCmd)
	authorizedKeysCmd.AddCommand(authorizedKeysApplyCmd)

	authorizedKeysApplyCmd.Flags().StringVar(&applyServer, "server", "", "server alias to apply keys to")
	authorizedKeysApplyCmd.Flags().BoolVar(&applyAll, "all", false, "apply to all servers")
	authorizedKeysApplyCmd.Flags().BoolVar(&applyBackup, "backup", true, "create backup before modifying")
}

func runAuthorizedKeysApply(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cfg := GetConfig()
	output := cli.NewOutput(IsJSONOutput())

	if applyServer == "" && !applyAll {
		return fmt.Errorf("must specify --server or --all")
	}

	// Build inventory
	opClient := op.NewClient()
	if !opClient.IsInstalled() {
		return fmt.Errorf("1Password CLI (op) not found")
	}
	if !opClient.IsSignedIn(ctx) {
		return fmt.Errorf("not signed in to 1Password")
	}

	tags := inventory.TagConfig{
		Server: cfg.Tags.Server,
		Jump:   cfg.Tags.Jump,
		Key:    cfg.Tags.Key,
	}

	builder := inventory.NewBuilder(opClient, tags, cfg.Vaults)
	buildResult, keyResult, err := builder.BuildWithKeys(ctx)
	if err != nil {
		return err
	}

	// Determine which servers to apply to
	var servers []*model.ServerRecord
	if applyAll {
		for _, s := range buildResult.Inventory.Servers {
			servers = append(servers, s)
		}
	} else {
		server, ok := buildResult.Inventory.GetServer(applyServer)
		if !ok {
			return fmt.Errorf("server '%s' not found in inventory", applyServer)
		}
		servers = append(servers, server)
	}

	if len(servers) == 0 {
		output.PrintWarning("no servers to apply to")
		return nil
	}

	// Apply to each server
	manager := sshremote.NewAuthorizedKeysManager()
	var results []*sshremote.ApplyResult

	for _, server := range servers {
		log.Info("applying keys to server", "alias", server.Alias)

		// Get keys for this server
		var serverKeys []*model.ResolvedKey
		for _, keyRef := range server.Keys {
			if resolved, ok := buildResult.Inventory.Keys[keyRef.UniqueKey()]; ok {
				serverKeys = append(serverKeys, resolved)
			}
		}

		if len(serverKeys) == 0 {
			output.PrintWarning("no resolved keys for server %s", server.Alias)
			continue
		}

		opts := &sshremote.ApplyOptions{
			RemoteUser: server.DefaultUser,
			DryRun:     IsDryRun(),
			Backup:     applyBackup,
			Keys:       serverKeys,
		}

		result, err := manager.Apply(ctx, server.Alias, opts)
		if err != nil {
			output.PrintError(fmt.Errorf("failed to apply to %s: %w", server.Alias, err))
			continue
		}

		results = append(results, result)

		if IsDryRun() {
			output.PrintLine("\n[DRY RUN] %s:", server.Alias)
			output.PrintLine("  Keys to add: %d", result.KeysAdded)
			output.PrintLine("  Keys to remove: %d", result.KeysRemoved)
			if result.Diff != "" {
				output.PrintLine("  Changes:")
				for _, line := range splitLines(result.Diff) {
					output.PrintLine("    %s", line)
				}
			}
		} else {
			output.PrintSuccess("applied to %s (added: %d, removed: %d)",
				server.Alias, result.KeysAdded, result.KeysRemoved)
			if result.BackupPath != "" {
				output.PrintLine("  Backup: %s", result.BackupPath)
			}
		}
	}

	// Print summary
	summary := &cli.Summary{
		Operation: "Authorized Keys Apply",
		DryRun:    IsDryRun(),
		Items: map[string]int{
			"Servers processed": len(results),
			"Total keys":        len(keyResult.Resolved),
		},
	}
	output.PrintSummary(summary)

	return nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	for _, line := range []string{s} {
		lines = append(lines, line)
	}
	return lines
}
