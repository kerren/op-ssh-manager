package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/kerren/op-ssh-manager/internal/cli"
	"github.com/kerren/op-ssh-manager/internal/model"
	"github.com/kerren/op-ssh-manager/internal/op"
	"github.com/spf13/cobra"
)

var hostCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new SSH host entry in 1Password",
	Long: `Create a new SSH host entry in 1Password with the specified details.

The host entry will be created as a "Server" type item and tagged with the
configured server tag (default: op-ssh-manager/server) so it can be discovered
by the sync command.

Required fields:
  --alias      Unique SSH host alias (used in SSH config)
  --hostname   IP address or DNS hostname
  --vault      1Password vault to store the entry

Optional fields:
  --port         SSH port (default: 22)
  --default-user Default SSH username
  --keys         SSH key references (comma-separated)
  --jump-alias   Alias of jump host to use for ProxyJump
  --environment  Environment label (e.g., production, staging)
  --owner-team   Team that owns this server

Examples:
  # Create a basic host entry
  op-ssh-manager host create --alias prod-web --hostname 10.0.1.100 --vault Production

  # Create with all options
  op-ssh-manager host create \
    --alias prod-db \
    --hostname db.example.com \
    --port 2222 \
    --default-user ubuntu \
    --keys "vault=Production;item=prod-key" \
    --jump-alias prod-bastion \
    --environment production \
    --owner-team platform \
    --vault Production`,
	RunE: runHostCreate,
}

var (
	hostAlias       string
	hostHostname    string
	hostPort        int
	hostDefaultUser string
	hostKeys        string
	hostJumpAlias   string
	hostEnvironment string
	hostOwnerTeam   string
	hostVault       string
)

func init() {
	hostCmd.AddCommand(hostCreateCmd)

	// Required flags
	hostCreateCmd.Flags().StringVar(&hostAlias, "alias", "", "unique SSH host alias (required)")
	hostCreateCmd.Flags().StringVar(&hostHostname, "hostname", "", "IP address or DNS hostname (required)")
	hostCreateCmd.Flags().StringVar(&hostVault, "vault", "", "1Password vault to store the entry (required)")
	hostCreateCmd.MarkFlagRequired("alias")
	hostCreateCmd.MarkFlagRequired("hostname")
	hostCreateCmd.MarkFlagRequired("vault")

	// Optional flags
	hostCreateCmd.Flags().IntVar(&hostPort, "port", 22, "SSH port")
	hostCreateCmd.Flags().StringVar(&hostDefaultUser, "default-user", "", "default SSH username")
	hostCreateCmd.Flags().StringVar(&hostKeys, "keys", "", "SSH key references (comma-separated)")
	hostCreateCmd.Flags().StringVar(&hostJumpAlias, "jump-alias", "", "alias of jump host for ProxyJump")
	hostCreateCmd.Flags().StringVar(&hostEnvironment, "environment", "", "environment label (e.g., production)")
	hostCreateCmd.Flags().StringVar(&hostOwnerTeam, "owner-team", "", "team that owns this server")
}

func runHostCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	output := cli.NewOutput(IsJSONOutput())
	cfg := GetConfig()

	// Validate port range
	if hostPort < 1 || hostPort > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", hostPort)
	}

	// Validate key references if provided
	if hostKeys != "" {
		if _, err := model.ParseKeyRefs(hostKeys); err != nil {
			return fmt.Errorf("invalid keys format: %w", err)
		}
	}

	// Build the title from the alias
	title := hostAlias

	// Build fields map
	fields := map[string]string{
		model.ServerFieldNames.Alias:    hostAlias,
		model.ServerFieldNames.Hostname: hostHostname,
	}

	// Add optional fields if provided
	if hostPort != 22 {
		fields[model.ServerFieldNames.Port] = strconv.Itoa(hostPort)
	}
	if hostDefaultUser != "" {
		fields[model.ServerFieldNames.DefaultUser] = hostDefaultUser
	}
	if hostKeys != "" {
		fields[model.ServerFieldNames.Keys] = hostKeys
	}
	if hostJumpAlias != "" {
		fields[model.ServerFieldNames.JumpAlias] = hostJumpAlias
	}
	if hostEnvironment != "" {
		fields[model.ServerFieldNames.Environment] = hostEnvironment
	}
	if hostOwnerTeam != "" {
		fields[model.ServerFieldNames.OwnerTeam] = hostOwnerTeam
	}

	// Get the server tag from config
	serverTag := cfg.Tags.Server
	if serverTag == "" {
		serverTag = "op-ssh-manager/server"
	}

	// Handle dry-run before checking 1Password (allows testing without op installed)
	if IsDryRun() {
		output.PrintLine("[DRY RUN] Would create Server item in vault: %s", hostVault)
		output.PrintLine("[DRY RUN] Title: %s", title)
		output.PrintLine("[DRY RUN] Fields:")
		for k, v := range fields {
			output.PrintLine("[DRY RUN]   %s: %s", k, v)
		}
		output.PrintLine("[DRY RUN] Would add tag: %s", serverTag)
		return nil
	}

	// Check 1Password CLI availability
	opClient := op.NewClient()
	if !opClient.IsInstalled() {
		return fmt.Errorf("1Password CLI (op) not found")
	}
	if !opClient.IsSignedIn(ctx) {
		return fmt.Errorf("not signed in to 1Password")
	}

	// Create the item with category "Server"
	item, err := opClient.CreateItem(ctx, "Server", title, hostVault, fields, serverTag)
	if err != nil {
		return fmt.Errorf("failed to create host entry: %w", err)
	}

	output.PrintSuccess("Created host entry: %s", hostAlias)
	output.PrintLine("  Item ID: %s", item.ID)
	output.PrintLine("  Vault:   %s", item.Vault.Name)
	output.PrintLine("  Tag:     %s", serverTag)

	if IsJSONOutput() {
		return output.PrintJSON(map[string]interface{}{
			"id":       item.ID,
			"title":    item.Title,
			"vault":    item.Vault.Name,
			"vault_id": item.Vault.ID,
			"alias":    hostAlias,
			"hostname": hostHostname,
			"tag":      serverTag,
		})
	}

	output.PrintLine("")
	output.PrintLine("Run 'op-ssh-manager sync' to update your SSH configuration.")

	return nil
}
