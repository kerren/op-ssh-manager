package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/kerren/op-ssh-manager/internal/cli"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh <alias> [-- <ssh args>]",
	Short: "SSH into a managed server",
	Long: `SSH connects to a managed server by alias.

This command optionally syncs the configuration first (unless --no-sync is used),
then executes ssh with the given alias and any additional arguments.

Examples:
  op-ssh-manager ssh prod-db
  op-ssh-manager ssh staging -- -L 5432:localhost:5432
  op-ssh-manager ssh dev --no-sync`,
	Args:               cobra.MinimumNArgs(1),
	RunE:               runSSH,
	DisableFlagParsing: false,
}

var (
	noSync bool
)

func init() {
	rootCmd.AddCommand(sshCmd)

	sshCmd.Flags().BoolVar(&noSync, "no-sync", false, "skip syncing before connecting")
}

func runSSH(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cfg := GetConfig()
	output := cli.NewOutput(IsJSONOutput())

	if len(args) < 1 {
		return fmt.Errorf("server alias required")
	}

	alias := args[0]
	sshArgs := []string{alias}

	// Collect additional SSH args (after --)
	if dashIdx := cmd.ArgsLenAtDash(); dashIdx >= 0 && dashIdx < len(args) {
		sshArgs = append(sshArgs, args[dashIdx:]...)
	} else if len(args) > 1 {
		sshArgs = append(sshArgs, args[1:]...)
	}

	// Auto-sync if enabled
	if !noSync && cfg.Sync.AutoSync {
		log.Debug("auto-syncing before SSH connection")

		// Run sync
		syncCmd.SetArgs([]string{})
		if err := syncCmd.ExecuteContext(ctx); err != nil {
			output.PrintWarning("sync failed: %v (continuing with existing config)", err)
		}
	}

	// Execute SSH
	return execSSH(sshArgs)
}

// execSSH executes the ssh command, replacing the current process
func execSSH(args []string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh command not found: %w", err)
	}

	log.Debug("executing ssh", "path", sshPath, "args", args)

	// Build full args (including the command name as argv[0])
	fullArgs := append([]string{"ssh"}, args...)

	// Replace current process with ssh
	// This ensures proper signal handling and terminal behavior
	return syscall.Exec(sshPath, fullArgs, os.Environ())
}

// RunSSHWithOutput runs SSH and captures output (for verification)
func RunSSHWithOutput(alias string, command string) ([]byte, error) {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		alias,
	}

	if command != "" {
		args = append(args, command)
	}

	cmd := exec.Command("ssh", args...)
	return cmd.CombinedOutput()
}

// VerifySSHConnection verifies that SSH connection works
func VerifySSHConnection(alias string) error {
	output, err := RunSSHWithOutput(alias, "true")
	if err != nil {
		return fmt.Errorf("SSH verification failed: %w (output: %s)", err, string(output))
	}
	return nil
}
