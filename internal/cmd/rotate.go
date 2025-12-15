package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/kerren/op-ssh-manager/internal/cli"
	"github.com/kerren/op-ssh-manager/internal/inventory"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/kerren/op-ssh-manager/internal/model"
	"github.com/kerren/op-ssh-manager/internal/op"
	"github.com/kerren/op-ssh-manager/internal/rotate"
	"github.com/spf13/cobra"
)

var rotateCmd = &cobra.Command{
	Use:   "rotate-key <key>",
	Short: "Rotate an SSH key",
	Long: `Rotate an SSH key by creating a new key, deploying it to all servers that use
the old key, verifying connections, and then removing the old key.

The rotation process:
  1. Find the specified key in 1Password
  2. Identify all servers using this key
  3. Create a new SSH key in 1Password
  4. Add the new key to all servers' authorized_keys
  5. Test SSH connections to all servers using the new key
  6. Remove the old key from all servers
  7. Archive the old key in 1Password (unless --no-delete is specified)

Key reference formats supported:
  - Key title or ID: "my-ssh-key"
  - With vault: "vault=MyVault;item=my-ssh-key"
  - Secret reference: "op://MyVault/my-ssh-key"

Examples:
  op-ssh-manager rotate-key my-ssh-key
  op-ssh-manager rotate-key "vault=Personal;item=dev-server-key"
  op-ssh-manager rotate-key my-key --new-title "my-key-2024"
  op-ssh-manager rotate-key my-key --dry-run
  op-ssh-manager rotate-key my-key --no-delete`,
	Args: cobra.ExactArgs(1),
	RunE: runRotateKey,
}

var (
	rotateNewTitle string
	rotateKeyType  string
	rotateNoDelete bool
	rotateForce    bool
)

func init() {
	rootCmd.AddCommand(rotateCmd)

	rotateCmd.Flags().StringVar(&rotateNewTitle, "new-title", "", "title for the new key (default: old title + rotation date)")
	rotateCmd.Flags().StringVar(&rotateKeyType, "key-type", "", "SSH key type: ed25519, rsa2048, rsa3072, rsa4096 (default: ed25519)")
	rotateCmd.Flags().BoolVar(&rotateNoDelete, "no-delete", false, "keep the old key after rotation instead of archiving it")
	rotateCmd.Flags().BoolVar(&rotateForce, "force", false, "skip confirmation prompts")
}

func runRotateKey(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cfg := GetConfig()
	output := cli.NewOutput(IsJSONOutput())

	// Parse key reference
	keyRef, err := model.ParseKeyRef(args[0])
	if err != nil {
		return fmt.Errorf("invalid key reference: %w", err)
	}

	// Build inventory
	opClient := op.NewClient()
	if !opClient.IsInstalled() {
		return fmt.Errorf("1Password CLI (op) not found")
	}
	if !opClient.IsSignedIn(ctx) {
		return fmt.Errorf("not signed in to 1Password")
	}

	output.PrintLine("Building inventory...")

	tags := inventory.TagConfig{
		Server: cfg.Tags.Server,
		Jump:   cfg.Tags.Jump,
		Key:    cfg.Tags.Key,
	}

	builder := inventory.NewBuilder(opClient, tags, cfg.Vaults)
	buildResult, _, err := builder.BuildWithKeys(ctx)
	if err != nil {
		return fmt.Errorf("failed to build inventory: %w", err)
	}

	// Create rotator
	rotator := rotate.NewRotator(opClient)

	// Get list of servers that will be affected
	serverAliases, err := rotator.GetServersUsingKey(buildResult.Inventory, keyRef, cfg.Vaults)
	if err != nil {
		return fmt.Errorf("failed to find servers using key: %w", err)
	}

	if len(serverAliases) == 0 {
		output.PrintWarning("No servers found using key: %s", keyRef.String())
		return nil
	}

	// Show what will be affected
	output.PrintLine("")
	output.PrintLine("Key: %s", keyRef.String())
	output.PrintLine("Servers to update (%d):", len(serverAliases))
	for _, alias := range serverAliases {
		output.PrintLine("  - %s", alias)
	}
	output.PrintLine("")

	// Confirm unless force or dry-run
	if !rotateForce && !IsDryRun() {
		output.PrintWarning("This will rotate the SSH key on %d servers.", len(serverAliases))
		output.PrintLine("Use --dry-run to preview changes, or --force to skip this prompt.")
		return fmt.Errorf("rotation cancelled (use --force to proceed)")
	}

	// Parse key type
	var keyType op.SSHKeyType
	switch strings.ToLower(rotateKeyType) {
	case "ed25519", "":
		keyType = op.SSHKeyTypeEd25519
	case "rsa2048":
		keyType = op.SSHKeyTypeRSA2048
	case "rsa3072":
		keyType = op.SSHKeyTypeRSA3072
	case "rsa4096":
		keyType = op.SSHKeyTypeRSA4096
	default:
		return fmt.Errorf("invalid key type: %s", rotateKeyType)
	}

	// Set up rotation options
	opts := rotate.RotateOptions{
		OldKeyRef:   keyRef,
		NewKeyTitle: rotateNewTitle,
		KeyType:     keyType,
		DryRun:      IsDryRun(),
		NoDelete:    rotateNoDelete,
		Force:       rotateForce,
		Inventory:   buildResult.Inventory,
		Vaults:      cfg.Vaults,
		KeyTag:      cfg.Tags.Key,
	}

	// Progress callback
	progressCallback := func(phase rotate.Phase, message string, current, total int) {
		if IsJSONOutput() {
			return
		}

		prefix := ""
		switch phase {
		case rotate.PhaseResolveOldKey:
			prefix = "[1/7]"
		case rotate.PhaseFindServers:
			prefix = "[2/7]"
		case rotate.PhaseCreateNewKey:
			prefix = "[3/7]"
		case rotate.PhaseAddNewKey:
			prefix = "[4/7]"
		case rotate.PhaseVerifyConnection:
			prefix = "[5/7]"
		case rotate.PhaseRemoveOldKey:
			prefix = "[6/7]"
		case rotate.PhaseArchiveOldKey:
			prefix = "[7/7]"
		}

		if total > 1 {
			output.PrintLine("%s %s (%d/%d)", prefix, message, current+1, total)
		} else {
			output.PrintLine("%s %s", prefix, message)
		}
	}

	// Perform rotation
	output.PrintLine("")
	if IsDryRun() {
		output.PrintLine("[DRY RUN] Simulating key rotation...")
	} else {
		output.PrintLine("Starting key rotation...")
	}
	output.PrintLine("")

	result, err := rotator.Rotate(ctx, opts, progressCallback)
	if err != nil {
		// If we have partial results, show them
		if result != nil {
			printRotationResult(output, result)
		}
		return fmt.Errorf("rotation failed: %w", err)
	}

	// Print results
	output.PrintLine("")
	printRotationResult(output, result)

	return nil
}

func printRotationResult(output *cli.Output, result *rotate.RotateResult) {
	if result.DryRun {
		output.PrintLine("[DRY RUN] Results:")
	} else {
		output.PrintLine("Rotation Results:")
	}
	output.PrintLine(strings.Repeat("-", 40))

	if result.OldKey != nil {
		output.PrintLine("Old Key: %s (%s)", result.OldKey.Title, result.OldKey.ItemID)
	}

	if result.NewKey != nil {
		output.PrintLine("New Key: %s (%s)", result.NewKey.Title, result.NewKey.ItemID)
	}

	output.PrintLine("")
	output.PrintLine("Servers Updated: %d", len(result.ServersUpdated))
	for _, alias := range result.ServersUpdated {
		status := "+"
		if _, failed := result.ConnectionsFailed[alias]; failed {
			status = "!"
		}
		output.PrintLine("  [%s] %s", status, alias)
	}

	if len(result.ServersFailed) > 0 {
		output.PrintLine("")
		output.PrintWarning("Servers Failed: %d", len(result.ServersFailed))
		for alias, err := range result.ServersFailed {
			output.PrintLine("  [x] %s: %v", alias, err)
		}
	}

	output.PrintLine("")
	output.PrintLine("Connection Tests:")
	output.PrintLine("  Verified: %d", result.ConnectionsVerified)
	if len(result.ConnectionsFailed) > 0 {
		output.PrintLine("  Failed: %d", len(result.ConnectionsFailed))
		for alias, err := range result.ConnectionsFailed {
			log.Debug("connection test failed", "alias", alias, "error", err)
		}
	}

	if result.OldKeyArchived {
		output.PrintLine("")
		output.PrintSuccess("Old key archived in 1Password")
	} else if !result.DryRun {
		output.PrintLine("")
		output.PrintLine("Old key was NOT archived (--no-delete or archive failed)")
	}

	// Summary
	output.PrintLine("")
	if len(result.ServersFailed) == 0 && len(result.ConnectionsFailed) == 0 {
		output.PrintSuccess("Key rotation completed successfully!")
	} else if result.ConnectionsVerified > 0 {
		output.PrintWarning("Key rotation completed with some warnings")
	} else {
		output.PrintError(fmt.Errorf("key rotation completed with errors"))
	}
}
