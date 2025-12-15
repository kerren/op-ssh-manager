package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// Paths contains all platform-specific paths used by the application
type Paths struct {
	// HomeDir is the user's home directory
	HomeDir string

	// SSHDir is the .ssh directory
	SSHDir string

	// MainSSHConfig is the main SSH config file path
	MainSSHConfig string

	// ManagedSSHConfig is the managed SSH config file path
	ManagedSSHConfig string

	// KeyCacheDir is the directory for cached public keys
	KeyCacheDir string

	// AgentToml is the 1Password SSH agent config file path
	AgentToml string

	// AppConfigDir is the application config directory
	AppConfigDir string

	// LogDir is the directory for log files
	LogDir string

	// LockFile is the path to the lock file for daemon
	LockFile string

	// SystemdUserDir is the systemd user unit directory (Linux only)
	SystemdUserDir string

	// LaunchAgentsDir is the LaunchAgents directory (macOS only)
	LaunchAgentsDir string
}

// DefaultPaths returns the default paths for the current platform
func DefaultPaths() (*Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	p := &Paths{
		HomeDir:          homeDir,
		SSHDir:           filepath.Join(homeDir, ".ssh"),
		MainSSHConfig:    filepath.Join(homeDir, ".ssh", "config"),
		ManagedSSHConfig: filepath.Join(homeDir, ".ssh", "op-ssh-manager.conf"),
		KeyCacheDir:      filepath.Join(homeDir, ".ssh", "op-ssh-manager", "keys"),
		LogDir:           filepath.Join(homeDir, ".ssh", "op-ssh-manager", "logs"),
		LockFile:         filepath.Join(homeDir, ".ssh", "op-ssh-manager", ".lock"),
	}

	// Platform-specific paths
	switch runtime.GOOS {
	case "darwin":
		p.AppConfigDir = filepath.Join(homeDir, ".config", "op-ssh-manager")
		p.AgentToml = filepath.Join(homeDir, ".config", "1Password", "ssh", "agent.toml")
		p.LaunchAgentsDir = filepath.Join(homeDir, "Library", "LaunchAgents")
	case "linux":
		// Follow XDG Base Directory Specification
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			configHome = filepath.Join(homeDir, ".config")
		}
		p.AppConfigDir = filepath.Join(configHome, "op-ssh-manager")
		p.AgentToml = filepath.Join(configHome, "1Password", "ssh", "agent.toml")
		p.SystemdUserDir = filepath.Join(configHome, "systemd", "user")
	case "windows":
		// Windows uses AppData
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(homeDir, "AppData", "Roaming")
		}
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		p.AppConfigDir = filepath.Join(appData, "op-ssh-manager")
		p.AgentToml = filepath.Join(localAppData, "1Password", "config", "ssh", "agent.toml")
		// Windows SSH directory is typically in user profile
		p.SSHDir = filepath.Join(homeDir, ".ssh")
		p.MainSSHConfig = filepath.Join(homeDir, ".ssh", "config")
		p.ManagedSSHConfig = filepath.Join(homeDir, ".ssh", "op-ssh-manager.conf")
		p.KeyCacheDir = filepath.Join(homeDir, ".ssh", "op-ssh-manager", "keys")
	default:
		// Fallback to Linux-like paths
		p.AppConfigDir = filepath.Join(homeDir, ".config", "op-ssh-manager")
		p.AgentToml = filepath.Join(homeDir, ".config", "1Password", "ssh", "agent.toml")
	}

	return p, nil
}

// GetSSHAgentSocket returns the path to the 1Password SSH agent socket
func GetSSHAgentSocket() string {
	// Check environment variable first
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		return sock
	}

	homeDir, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		// macOS uses a Unix socket in Library
		return filepath.Join(homeDir, "Library", "Group Containers", "2BUA8C4S2C.com.1password", "t", "agent.sock")
	case "linux":
		// Linux uses XDG_RUNTIME_DIR or fallback
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			runtimeDir = filepath.Join("/run", "user", strconv.Itoa(os.Getuid()))
		}
		return filepath.Join(runtimeDir, "1Password", "agent.sock")
	case "windows":
		// Windows uses a named pipe (not a socket path)
		return `\\.\pipe\openssh-ssh-agent`
	default:
		return ""
	}
}

// GetIdentityAgent returns the IdentityAgent value for SSH config
func GetIdentityAgent() string {
	switch runtime.GOOS {
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, "Library", "Group Containers", "2BUA8C4S2C.com.1password", "t", "agent.sock")
	case "linux":
		// Use the environment variable reference for portability
		return "~/.1password/agent.sock"
	case "windows":
		// Windows OpenSSH has a hardcoded pipe path
		return ""
	default:
		return ""
	}
}

// EnsureDir ensures a directory exists with the given permissions
func EnsureDir(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsMacOS returns true if running on macOS
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// GetOS returns the current operating system name
func GetOS() string {
	return runtime.GOOS
}

// GetArch returns the current architecture
func GetArch() string {
	return runtime.GOARCH
}
