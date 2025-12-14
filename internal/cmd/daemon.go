package cmd

import (
	"context"
	"fmt"
	"runtime"

	"github.com/kerren/op-ssh-manager/internal/cli"
	"github.com/kerren/op-ssh-manager/internal/daemon"
	"github.com/kerren/op-ssh-manager/internal/platform"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage background sync daemon",
	Long:  `Commands for managing the background sync daemon.`,
}

var daemonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the sync daemon",
	Long: `Run the sync daemon in the foreground.

The daemon will sync on the configured schedule and on startup.
Use Ctrl+C to stop.`,
	RunE: runDaemonRun,
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the daemon as a system service",
	Long: `Install the daemon as a system service.

On Linux: Creates a systemd user service and timer
On macOS: Creates a LaunchAgent
On Windows: Creates a Scheduled Task`,
	RunE: runDaemonInstall,
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the daemon service",
	Long:  `Remove the daemon from system services.`,
	RunE:  runDaemonUninstall,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long:  `Show the current status of the daemon service.`,
	RunE:  runDaemonStatus,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonRunCmd)
	daemonCmd.AddCommand(daemonInstallCmd)
	daemonCmd.AddCommand(daemonUninstallCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
}

func runDaemonRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cfg := GetConfig()

	// Create sync function
	syncFunc := func(ctx context.Context) error {
		return syncCmd.RunE(syncCmd, nil)
	}

	// Create and run daemon
	d := daemon.New(cfg.Daemon.Schedule, cfg.Paths.LogDir+"/.lock", syncFunc)
	return d.Run(ctx)
}

func runDaemonInstall(cmd *cobra.Command, args []string) error {
	output := cli.NewOutput(IsJSONOutput())

	switch runtime.GOOS {
	case "linux":
		installer, err := platform.NewSystemdInstaller()
		if err != nil {
			return err
		}
		if err := installer.Install("1h"); err != nil {
			return err
		}
		output.PrintSuccess("installed systemd user service")
		output.PrintLine("  Check status: systemctl --user status op-ssh-manager.timer")

	case "darwin":
		installer, err := platform.NewLaunchdInstaller()
		if err != nil {
			return err
		}
		if err := installer.Install(3600); err != nil {
			return err
		}
		output.PrintSuccess("installed LaunchAgent")
		output.PrintLine("  Check status: launchctl list com.kerren.op-ssh-manager")

	case "windows":
		output.PrintWarning("Windows scheduled task installation not yet implemented")
		output.PrintLine("To manually create a scheduled task:")
		output.PrintLine("  schtasks /create /tn \"op-ssh-manager\" /tr \"%s daemon run\" /sc hourly", "op-ssh-manager")
		return nil

	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return nil
}

func runDaemonUninstall(cmd *cobra.Command, args []string) error {
	output := cli.NewOutput(IsJSONOutput())

	switch runtime.GOOS {
	case "linux":
		installer, err := platform.NewSystemdInstaller()
		if err != nil {
			return err
		}
		if err := installer.Uninstall(); err != nil {
			return err
		}
		output.PrintSuccess("uninstalled systemd user service")

	case "darwin":
		installer, err := platform.NewLaunchdInstaller()
		if err != nil {
			return err
		}
		if err := installer.Uninstall(); err != nil {
			return err
		}
		output.PrintSuccess("uninstalled LaunchAgent")

	case "windows":
		output.PrintLine("To remove the scheduled task:")
		output.PrintLine("  schtasks /delete /tn \"op-ssh-manager\" /f")
		return nil

	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	output := cli.NewOutput(IsJSONOutput())

	var status string

	switch runtime.GOOS {
	case "linux":
		installer, err := platform.NewSystemdInstaller()
		if err != nil {
			return err
		}
		s, err := installer.Status()
		if err != nil {
			return err
		}
		status = s

	case "darwin":
		installer, err := platform.NewLaunchdInstaller()
		if err != nil {
			return err
		}
		s, err := installer.Status()
		if err != nil {
			return err
		}
		status = s

	default:
		status = "not available on " + runtime.GOOS
	}

	output.PrintLine("Daemon status: %s", status)
	return nil
}
