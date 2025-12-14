package cmd

import (
	"fmt"
	"os"

	"github.com/kerren/op-ssh-manager/internal/config"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	jsonOut   bool
	verbose   bool
	dryRun    bool
	appConfig *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "op-ssh-manager",
	Short: "Manage SSH configurations using 1Password",
	Long: `op-ssh-manager is a cross-platform CLI that manages SSH hosts and key selection
via 1Password. It generates SSH client configuration without deleting or breaking
unmanaged user entries, selects the correct 1Password SSH key per host and per user,
supports jump hosts automatically, and can apply/rotate authorized_keys on servers.

Key features:
  - Discovers managed servers and SSH keys from 1Password using tags
  - Generates/updates SSH client configuration safely
  - Selects correct 1Password SSH key per host using the 1Password SSH agent
  - Supports jump hosts (ProxyJump) automatically
  - Can apply and rotate authorized_keys on servers
  - Produces auditable reports (CSV/XLSX/HTML)
  - Optional background sync service`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for version command
		if cmd.Name() == "version" {
			return nil
		}

		// Initialize logger
		log.Init(verbose)

		// Load configuration
		var err error
		appConfig, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		return nil
	},
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.config/op-ssh-manager/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show what would be done without making changes")
}

// GetConfig returns the loaded application configuration
func GetConfig() *config.Config {
	return appConfig
}

// IsJSONOutput returns true if JSON output is requested
func IsJSONOutput() bool {
	return jsonOut
}

// IsDryRun returns true if dry-run mode is enabled
func IsDryRun() bool {
	return dryRun
}

// IsVerbose returns true if verbose mode is enabled
func IsVerbose() bool {
	return verbose
}

// ExitWithError prints an error and exits with code 1
func ExitWithError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
