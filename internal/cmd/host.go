package cmd

import (
	"github.com/spf13/cobra"
)

var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "Manage SSH host entries in 1Password",
	Long:  `Commands for creating and managing SSH host entries in 1Password.`,
}

func init() {
	rootCmd.AddCommand(hostCmd)
}
