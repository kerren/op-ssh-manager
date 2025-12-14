package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	// Vaults is a list of 1Password vault names/IDs to search for items
	Vaults []string `yaml:"vaults,omitempty"`

	// Tags configuration for 1Password item discovery
	Tags TagConfig `yaml:"tags"`

	// Paths configuration for file locations
	Paths PathsConfig `yaml:"paths"`

	// SSH configuration options
	SSH SSHConfig `yaml:"ssh"`

	// Sync configuration
	Sync SyncConfig `yaml:"sync"`

	// Daemon configuration
	Daemon DaemonConfig `yaml:"daemon"`
}

// TagConfig holds tag names used for 1Password item discovery
type TagConfig struct {
	Server        string `yaml:"server"`
	Jump          string `yaml:"jump"`
	Key           string `yaml:"key"`
	ConfigHistory string `yaml:"config_history"`
	UserVault     string `yaml:"user_vault"`
}

// PathsConfig holds file path configurations
type PathsConfig struct {
	SSHDir          string `yaml:"ssh_dir"`
	MainSSHConfig   string `yaml:"main_ssh_config"`
	ManagedConfig   string `yaml:"managed_config"`
	KeyCacheDir     string `yaml:"key_cache_dir"`
	AgentToml       string `yaml:"agent_toml"`
	AppConfigDir    string `yaml:"app_config_dir"`
	LogDir          string `yaml:"log_dir"`
}

// SSHConfig holds SSH-related configuration
type SSHConfig struct {
	// Precedence determines where Include is placed: "managed" (top) or "user" (bottom)
	Precedence string `yaml:"precedence"`

	// Mode is either "include" (recommended) or "inline" (fallback)
	Mode string `yaml:"mode"`

	// IdentityAgent is the path to the 1Password SSH agent socket (auto-detected if empty)
	IdentityAgent string `yaml:"identity_agent"`
}

// SyncConfig holds sync-related configuration
type SyncConfig struct {
	// AutoSync enables automatic sync before ssh command
	AutoSync bool `yaml:"auto_sync"`

	// TTL is the minimum time between automatic syncs
	TTL time.Duration `yaml:"ttl"`
}

// DaemonConfig holds daemon/scheduler configuration
type DaemonConfig struct {
	// Schedule is a cron expression for sync scheduling
	Schedule string `yaml:"schedule"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()

	return &Config{
		Vaults: []string{}, // Search all accessible vaults by default
		Tags: TagConfig{
			Server:        "op-ssh-manager/server",
			Jump:          "op-ssh-manager/jump",
			Key:           "op-ssh-manager/key",
			ConfigHistory: "op-ssh-manager/config-history",
			UserVault:     "op-ssh-manager/user-vault",
		},
		Paths: PathsConfig{
			SSHDir:          filepath.Join(homeDir, ".ssh"),
			MainSSHConfig:   filepath.Join(homeDir, ".ssh", "config"),
			ManagedConfig:   filepath.Join(homeDir, ".ssh", "op-ssh-manager.conf"),
			KeyCacheDir:     filepath.Join(homeDir, ".ssh", "op-ssh-manager", "keys"),
			AgentToml:       filepath.Join(homeDir, ".config", "1Password", "ssh", "agent.toml"),
			AppConfigDir:    filepath.Join(homeDir, ".config", "op-ssh-manager"),
			LogDir:          filepath.Join(homeDir, ".ssh", "op-ssh-manager", "logs"),
		},
		SSH: SSHConfig{
			Precedence:    "managed", // Include at top so managed settings win
			Mode:          "include", // Use Include directive by default
			IdentityAgent: "",        // Auto-detect
		},
		Sync: SyncConfig{
			AutoSync: true,
			TTL:      5 * time.Minute,
		},
		Daemon: DaemonConfig{
			Schedule: "0 * * * *", // Every hour
		},
	}
}

// Load reads the configuration from a file, falling back to defaults
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Determine config file path
	if path == "" {
		path = filepath.Join(cfg.Paths.AppConfigDir, "config.yaml")
	}

	// Try to read the config file
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist, use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate and expand paths
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to a file
func (c *Config) Save(path string) error {
	if path == "" {
		path = filepath.Join(c.Paths.AppConfigDir, "config.yaml")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validate checks the configuration for errors
func (c *Config) validate() error {
	// Validate precedence
	if c.SSH.Precedence != "managed" && c.SSH.Precedence != "user" {
		return fmt.Errorf("ssh.precedence must be 'managed' or 'user', got '%s'", c.SSH.Precedence)
	}

	// Validate mode
	if c.SSH.Mode != "include" && c.SSH.Mode != "inline" {
		return fmt.Errorf("ssh.mode must be 'include' or 'inline', got '%s'", c.SSH.Mode)
	}

	return nil
}

// ExpandPath expands ~ in paths to the user's home directory
func ExpandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[1:])
	}
	return path
}
