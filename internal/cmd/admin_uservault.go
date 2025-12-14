package cmd

import (
	"context"
	"fmt"

	"github.com/kerren/op-ssh-manager/internal/cli"
	"github.com/kerren/op-ssh-manager/internal/op"
	"github.com/spf13/cobra"
)

var adminUserVaultCmd = &cobra.Command{
	Use:   "user-vault",
	Short: "Manage per-user SSH key vaults",
	Long:  `Commands for managing per-user SSH key vaults.`,
}

var adminUserVaultCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a per-user vault",
	Long: `Create a dedicated vault for a user's SSH keys.

The vault name follows the convention: ssh-keys/<email>
The user and optionally admin groups are granted access.`,
	RunE: runAdminUserVaultCreate,
}

var (
	userVaultEmail  string
	userVaultPrefix string
	adminGroup      string
)

func init() {
	adminCmd.AddCommand(adminUserVaultCmd)
	adminUserVaultCmd.AddCommand(adminUserVaultCreateCmd)

	adminUserVaultCreateCmd.Flags().StringVar(&userVaultEmail, "email", "", "user email address (required)")
	adminUserVaultCreateCmd.Flags().StringVar(&userVaultPrefix, "prefix", "ssh-keys", "vault name prefix")
	adminUserVaultCreateCmd.Flags().StringVar(&adminGroup, "admin-group", "", "admin group to grant access")
	adminUserVaultCreateCmd.MarkFlagRequired("email")
}

func runAdminUserVaultCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	output := cli.NewOutput(IsJSONOutput())

	opClient := op.NewClient()
	if !opClient.IsInstalled() {
		return fmt.Errorf("1Password CLI (op) not found")
	}
	if !opClient.IsSignedIn(ctx) {
		return fmt.Errorf("not signed in to 1Password")
	}

	// Generate vault name
	vaultName := op.UserVaultNaming(userVaultEmail, userVaultPrefix)

	if IsDryRun() {
		output.PrintLine("[DRY RUN] Would create vault: %s", vaultName)
		output.PrintLine("[DRY RUN] Would grant access to: %s", userVaultEmail)
		if adminGroup != "" {
			output.PrintLine("[DRY RUN] Would grant access to group: %s", adminGroup)
		}
		return nil
	}

	// Create vault
	vault, err := opClient.CreateVault(ctx, vaultName, fmt.Sprintf("SSH keys for %s", userVaultEmail))
	if err != nil {
		return fmt.Errorf("failed to create vault: %w", err)
	}

	output.PrintSuccess("created vault: %s (ID: %s)", vaultName, vault.ID)

	// Grant user access
	if err := opClient.GrantUserAccess(ctx, vault.ID, userVaultEmail, "allow_editing"); err != nil {
		output.PrintWarning("failed to grant user access: %v", err)
	} else {
		output.PrintSuccess("granted access to %s", userVaultEmail)
	}

	// Grant admin group access if specified
	if adminGroup != "" {
		if err := opClient.GrantGroupAccess(ctx, vault.ID, adminGroup, "allow_editing", "allow_managing"); err != nil {
			output.PrintWarning("failed to grant admin group access: %v", err)
		} else {
			output.PrintSuccess("granted access to group %s", adminGroup)
		}
	}

	return nil
}
