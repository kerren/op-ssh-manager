package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/kerren/op-ssh-manager/internal/cli"
	"github.com/kerren/op-ssh-manager/internal/fs"
	"github.com/kerren/op-ssh-manager/internal/op"
	"github.com/kerren/op-ssh-manager/internal/platform"
	"github.com/kerren/op-ssh-manager/internal/sshconfig"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check environment and diagnose issues",
	Long: `Doctor checks your environment for proper configuration and
diagnoses common issues with 1Password SSH integration.

Checks performed:
  - 1Password CLI (op) installation and version
  - 1Password authentication status
  - SSH directory and permissions
  - 1Password SSH agent configuration
  - Managed config file status
  - Include directive status`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

// CheckResult represents the result of a single check
type CheckResult struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // "pass", "warn", "fail"
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cfg := GetConfig()
	output := cli.NewOutput(IsJSONOutput())

	checks := []CheckResult{}

	// Check 1: op CLI installed
	check := checkOPInstalled(ctx)
	checks = append(checks, check)
	printCheck(output, check)

	if check.Status == "fail" {
		// Can't continue without op
		return printSummary(output, checks)
	}

	// Check 2: op version
	check = checkOPVersion(ctx)
	checks = append(checks, check)
	printCheck(output, check)

	// Check 3: Authentication status
	check = checkAuth(ctx)
	checks = append(checks, check)
	printCheck(output, check)

	// Check 4: SSH directory
	check = checkSSHDir(cfg.Paths.SSHDir)
	checks = append(checks, check)
	printCheck(output, check)

	// Check 5: SSH directory permissions
	check = checkSSHDirPerms(cfg.Paths.SSHDir)
	checks = append(checks, check)
	printCheck(output, check)

	// Check 6: 1Password SSH agent
	check = checkSSHAgent()
	checks = append(checks, check)
	printCheck(output, check)

	// Check 7: Managed config file
	check = checkManagedConfig(cfg.Paths.ManagedConfig)
	checks = append(checks, check)
	printCheck(output, check)

	// Check 8: Include directive
	check = checkIncludeDirective(cfg.Paths.MainSSHConfig, cfg.Paths.ManagedConfig)
	checks = append(checks, check)
	printCheck(output, check)

	// Check 9: Key cache directory
	check = checkKeyCacheDir(cfg.Paths.KeyCacheDir)
	checks = append(checks, check)
	printCheck(output, check)

	// Check 10: Agent.toml
	check = checkAgentToml(cfg.Paths.AgentToml)
	checks = append(checks, check)
	printCheck(output, check)

	return printSummary(output, checks)
}

func checkOPInstalled(ctx context.Context) CheckResult {
	client := op.NewClient()

	if !client.IsInstalled() {
		return CheckResult{
			Name:        "1Password CLI",
			Status:      "fail",
			Message:     "op command not found in PATH",
			Remediation: "Install 1Password CLI from https://developer.1password.com/docs/cli/",
		}
	}

	return CheckResult{
		Name:    "1Password CLI",
		Status:  "pass",
		Message: "op command found",
	}
}

func checkOPVersion(ctx context.Context) CheckResult {
	client := op.NewClient()

	version, err := client.GetVersion(ctx)
	if err != nil {
		return CheckResult{
			Name:        "1Password CLI Version",
			Status:      "warn",
			Message:     fmt.Sprintf("could not get version: %v", err),
			Remediation: "Ensure op command works correctly",
		}
	}

	return CheckResult{
		Name:    "1Password CLI Version",
		Status:  "pass",
		Message: fmt.Sprintf("version %s", version),
	}
}

func checkAuth(ctx context.Context) CheckResult {
	client := op.NewClient()

	whoami, err := client.Whoami(ctx)
	if err != nil {
		return CheckResult{
			Name:        "1Password Authentication",
			Status:      "fail",
			Message:     "not signed in",
			Remediation: "Run 'op signin' or enable app integration in 1Password desktop app",
		}
	}

	return CheckResult{
		Name:    "1Password Authentication",
		Status:  "pass",
		Message: fmt.Sprintf("signed in as %s", whoami.Email),
	}
}

func checkSSHDir(sshDir string) CheckResult {
	if !fs.DirExists(sshDir) {
		return CheckResult{
			Name:        "SSH Directory",
			Status:      "warn",
			Message:     fmt.Sprintf("%s does not exist", sshDir),
			Remediation: "Run 'mkdir -p ~/.ssh && chmod 700 ~/.ssh'",
		}
	}

	return CheckResult{
		Name:    "SSH Directory",
		Status:  "pass",
		Message: fmt.Sprintf("%s exists", sshDir),
	}
}

func checkSSHDirPerms(sshDir string) CheckResult {
	if runtime.GOOS == "windows" {
		return CheckResult{
			Name:    "SSH Directory Permissions",
			Status:  "pass",
			Message: "Windows (ACLs not checked)",
		}
	}

	ok, err := fs.CheckSSHDirPerms(sshDir)
	if err != nil {
		return CheckResult{
			Name:    "SSH Directory Permissions",
			Status:  "warn",
			Message: err.Error(),
		}
	}

	if !ok {
		return CheckResult{
			Name:        "SSH Directory Permissions",
			Status:      "warn",
			Message:     "permissions may be too open",
			Remediation: "Run 'chmod 700 ~/.ssh'",
		}
	}

	return CheckResult{
		Name:    "SSH Directory Permissions",
		Status:  "pass",
		Message: "permissions OK (700)",
	}
}

func checkSSHAgent() CheckResult {
	socket := platform.GetSSHAgentSocket()

	switch runtime.GOOS {
	case "windows":
		return CheckResult{
			Name:    "1Password SSH Agent",
			Status:  "pass",
			Message: "Windows uses named pipe (automatic)",
		}
	default:
		if socket == "" {
			return CheckResult{
				Name:        "1Password SSH Agent",
				Status:      "warn",
				Message:     "SSH_AUTH_SOCK not set",
				Remediation: "Enable SSH agent in 1Password settings",
			}
		}

		if _, err := os.Stat(socket); os.IsNotExist(err) {
			return CheckResult{
				Name:        "1Password SSH Agent",
				Status:      "warn",
				Message:     fmt.Sprintf("agent socket not found at %s", socket),
				Remediation: "Enable SSH agent in 1Password settings and restart the app",
			}
		}

		return CheckResult{
			Name:    "1Password SSH Agent",
			Status:  "pass",
			Message: fmt.Sprintf("socket found at %s", socket),
		}
	}
}

func checkManagedConfig(path string) CheckResult {
	if !fs.FileExists(path) {
		return CheckResult{
			Name:    "Managed SSH Config",
			Status:  "warn",
			Message: "not found (run 'sync' to create)",
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return CheckResult{
			Name:    "Managed SSH Config",
			Status:  "warn",
			Message: err.Error(),
		}
	}

	return CheckResult{
		Name:    "Managed SSH Config",
		Status:  "pass",
		Message: fmt.Sprintf("exists (%d bytes)", info.Size()),
	}
}

func checkIncludeDirective(mainConfig, managedConfig string) CheckResult {
	includeMgr := sshconfig.NewIncludeManager(mainConfig, managedConfig, "managed")

	status, err := includeMgr.GetStatus()
	if err != nil {
		return CheckResult{
			Name:    "Include Directive",
			Status:  "warn",
			Message: err.Error(),
		}
	}

	if !status.Exists {
		return CheckResult{
			Name:    "Include Directive",
			Status:  "warn",
			Message: "not found in SSH config",
			Remediation: "Run 'sync' to add Include directive",
		}
	}

	return CheckResult{
		Name:    "Include Directive",
		Status:  "pass",
		Message: fmt.Sprintf("found at line %d (position: %s)", status.LineNumber, status.Position),
	}
}

func checkKeyCacheDir(dir string) CheckResult {
	if !fs.DirExists(dir) {
		return CheckResult{
			Name:    "Key Cache Directory",
			Status:  "warn",
			Message: "not found (will be created on sync)",
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return CheckResult{
			Name:    "Key Cache Directory",
			Status:  "warn",
			Message: err.Error(),
		}
	}

	keyCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".pub" {
			keyCount++
		}
	}

	return CheckResult{
		Name:    "Key Cache Directory",
		Status:  "pass",
		Message: fmt.Sprintf("%d public keys cached", keyCount),
	}
}

func checkAgentToml(path string) CheckResult {
	if !fs.FileExists(path) {
		return CheckResult{
			Name:    "Agent Config (agent.toml)",
			Status:  "warn",
			Message: "not found (optional, will be created on sync)",
		}
	}

	return CheckResult{
		Name:    "Agent Config (agent.toml)",
		Status:  "pass",
		Message: "exists",
	}
}

func printCheck(output *cli.Output, check CheckResult) {
	if IsJSONOutput() {
		return // JSON output handled in summary
	}

	var prefix string
	switch check.Status {
	case "pass":
		prefix = "✓"
	case "warn":
		prefix = "⚠"
	case "fail":
		prefix = "✗"
	}

	output.PrintLine("%s %s: %s", prefix, check.Name, check.Message)

	if check.Status != "pass" && check.Remediation != "" {
		output.PrintLine("  → %s", check.Remediation)
	}
}

func printSummary(output *cli.Output, checks []CheckResult) error {
	if IsJSONOutput() {
		output.PrintJSON(checks)
		return nil
	}

	// Count results
	var passed, warned, failed int
	for _, c := range checks {
		switch c.Status {
		case "pass":
			passed++
		case "warn":
			warned++
		case "fail":
			failed++
		}
	}

	output.PrintLine("")
	output.PrintLine("Summary: %d passed, %d warnings, %d failed", passed, warned, failed)

	if failed > 0 {
		return fmt.Errorf("%d checks failed", failed)
	}

	return nil
}
