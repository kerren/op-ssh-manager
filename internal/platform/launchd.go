package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.kerren.op-ssh-manager</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>StartInterval</key>
    <integer>%d</integer>
    <key>StandardOutPath</key>
    <string>%s/op-ssh-manager.log</string>
    <key>StandardErrorPath</key>
    <string>%s/op-ssh-manager.error.log</string>
</dict>
</plist>
`

// LaunchdInstaller installs macOS LaunchAgent
type LaunchdInstaller struct {
	BinaryPath     string
	LaunchAgentDir string
	LogDir         string
}

// NewLaunchdInstaller creates a new launchd installer
func NewLaunchdInstaller() (*LaunchdInstaller, error) {
	binaryPath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	paths, err := DefaultPaths()
	if err != nil {
		return nil, err
	}

	return &LaunchdInstaller{
		BinaryPath:     binaryPath,
		LaunchAgentDir: paths.LaunchAgentsDir,
		LogDir:         paths.LogDir,
	}, nil
}

// Install installs the LaunchAgent
func (i *LaunchdInstaller) Install(intervalSeconds int) error {
	// Ensure directories exist
	if err := os.MkdirAll(i.LaunchAgentDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents dir: %w", err)
	}
	if err := os.MkdirAll(i.LogDir, 0755); err != nil {
		return fmt.Errorf("failed to create log dir: %w", err)
	}

	// Unload if already loaded
	plistPath := filepath.Join(i.LaunchAgentDir, "com.kerren.op-ssh-manager.plist")
	exec.Command("launchctl", "unload", plistPath).Run()

	// Write plist
	plistContent := fmt.Sprintf(launchdPlistTemplate,
		i.BinaryPath, intervalSeconds, i.LogDir, i.LogDir)
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Load plist
	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("failed to load plist: %w", err)
	}

	return nil
}

// Uninstall removes the LaunchAgent
func (i *LaunchdInstaller) Uninstall() error {
	plistPath := filepath.Join(i.LaunchAgentDir, "com.kerren.op-ssh-manager.plist")

	// Unload
	exec.Command("launchctl", "unload", plistPath).Run()

	// Remove plist
	os.Remove(plistPath)

	return nil
}

// Status returns the status of the agent
func (i *LaunchdInstaller) Status() (string, error) {
	out, err := exec.Command("launchctl", "list", "com.kerren.op-ssh-manager").Output()
	if err != nil {
		return "not installed", nil
	}
	return string(out), nil
}
