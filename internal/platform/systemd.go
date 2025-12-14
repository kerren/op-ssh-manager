package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const systemdUnitTemplate = `[Unit]
Description=op-ssh-manager sync daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s daemon run
Restart=on-failure
RestartSec=30

[Install]
WantedBy=default.target
`

const systemdTimerTemplate = `[Unit]
Description=op-ssh-manager periodic sync

[Timer]
OnBootSec=5min
OnUnitActiveSec=%s
Persistent=true

[Install]
WantedBy=timers.target
`

// SystemdInstaller installs systemd user services
type SystemdInstaller struct {
	BinaryPath string
	UnitDir    string
}

// NewSystemdInstaller creates a new systemd installer
func NewSystemdInstaller() (*SystemdInstaller, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	paths, err := DefaultPaths()
	if err != nil {
		return nil, err
	}

	return &SystemdInstaller{
		BinaryPath: binaryPath,
		UnitDir:    paths.SystemdUserDir,
	}, nil
}

// Install installs the systemd user service and timer
func (i *SystemdInstaller) Install(interval string) error {
	// Ensure unit directory exists
	if err := os.MkdirAll(i.UnitDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user dir: %w", err)
	}

	// Write service unit
	servicePath := filepath.Join(i.UnitDir, "op-ssh-manager.service")
	serviceContent := fmt.Sprintf(systemdUnitTemplate, i.BinaryPath)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service unit: %w", err)
	}

	// Write timer unit
	timerPath := filepath.Join(i.UnitDir, "op-ssh-manager.timer")
	timerContent := fmt.Sprintf(systemdTimerTemplate, interval)
	if err := os.WriteFile(timerPath, []byte(timerContent), 0644); err != nil {
		return fmt.Errorf("failed to write timer unit: %w", err)
	}

	// Reload systemd
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable and start timer
	if err := exec.Command("systemctl", "--user", "enable", "--now", "op-ssh-manager.timer").Run(); err != nil {
		return fmt.Errorf("failed to enable timer: %w", err)
	}

	return nil
}

// Uninstall removes the systemd user service and timer
func (i *SystemdInstaller) Uninstall() error {
	// Stop and disable timer
	exec.Command("systemctl", "--user", "stop", "op-ssh-manager.timer").Run()
	exec.Command("systemctl", "--user", "disable", "op-ssh-manager.timer").Run()

	// Stop service
	exec.Command("systemctl", "--user", "stop", "op-ssh-manager.service").Run()

	// Remove unit files
	os.Remove(filepath.Join(i.UnitDir, "op-ssh-manager.service"))
	os.Remove(filepath.Join(i.UnitDir, "op-ssh-manager.timer"))

	// Reload systemd
	exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

// Status returns the status of the service
func (i *SystemdInstaller) Status() (string, error) {
	out, err := exec.Command("systemctl", "--user", "status", "op-ssh-manager.timer").Output()
	if err != nil {
		// Check if service exists
		if strings.Contains(string(out), "could not be found") {
			return "not installed", nil
		}
		return "unknown", err
	}
	return string(out), nil
}
