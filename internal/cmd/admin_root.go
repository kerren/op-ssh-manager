package cmd

import (
	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative commands",
	Long: `Administrative commands for managing vaults, users, and enterprise features.

These commands require elevated 1Password permissions and should be used
with caution. Most users will not need these commands.`,
}

func init() {
	rootCmd.AddCommand(adminCmd)
}
