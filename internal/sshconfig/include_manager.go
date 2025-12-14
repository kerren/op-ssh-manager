package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/fs"
	"github.com/kerren/op-ssh-manager/internal/log"
)

const (
	// IncludeComment is the marker comment for the managed include line
	IncludeComment = "# Managed by op-ssh-manager - do not edit this line"

	// PrecedenceManaged places include at top (managed settings win)
	PrecedenceManaged = "managed"

	// PrecedenceUser places include at bottom (user settings win)
	PrecedenceUser = "user"
)

// IncludeManager handles the Include directive in ~/.ssh/config
type IncludeManager struct {
	MainConfigPath   string
	ManagedConfig    string
	Precedence       string
	BackupOnFirstMod bool
}

// NewIncludeManager creates a new IncludeManager
func NewIncludeManager(mainConfig, managedConfig, precedence string) *IncludeManager {
	return &IncludeManager{
		MainConfigPath:   mainConfig,
		ManagedConfig:    managedConfig,
		Precedence:       precedence,
		BackupOnFirstMod: true,
	}
}

// EnsureInclude ensures the Include directive exists in the main SSH config
// It returns true if modifications were made
func (m *IncludeManager) EnsureInclude() (bool, error) {
	// Get relative path for Include directive
	includePath := m.getIncludePath()
	includeLine := fmt.Sprintf("Include %s", includePath)

	// Read current config
	content, err := fs.ReadFileIfExists(m.MainConfigPath)
	if err != nil {
		return false, apperrors.Wrap(apperrors.ErrFileSystem, "failed to read SSH config", err)
	}

	// Check if Include already exists
	if m.hasInclude(string(content), includePath) {
		log.Debug("Include directive already present", "path", includePath)
		return false, nil
	}

	// Create backup if this is the first modification
	if m.BackupOnFirstMod && len(content) > 0 {
		if _, err := m.createBackup(); err != nil {
			return false, err
		}
	}

	// Add the Include directive
	newContent := m.addInclude(string(content), includeLine)

	// Ensure .ssh directory exists
	sshDir := filepath.Dir(m.MainConfigPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return false, apperrors.Wrap(apperrors.ErrFileSystem, "failed to create .ssh directory", err)
	}

	// Write the updated config
	if err := fs.AtomicWrite(m.MainConfigPath, []byte(newContent), 0600); err != nil {
		return false, apperrors.Wrap(apperrors.ErrFileSystem, "failed to write SSH config", err)
	}

	log.Info("added Include directive to SSH config", "path", m.MainConfigPath, "include", includePath)
	return true, nil
}

// RemoveInclude removes the managed Include directive
func (m *IncludeManager) RemoveInclude() (bool, error) {
	content, err := os.ReadFile(m.MainConfigPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, apperrors.Wrap(apperrors.ErrFileSystem, "failed to read SSH config", err)
	}

	includePath := m.getIncludePath()
	if !m.hasInclude(string(content), includePath) {
		return false, nil
	}

	newContent := m.removeInclude(string(content), includePath)

	if err := fs.AtomicWrite(m.MainConfigPath, []byte(newContent), 0600); err != nil {
		return false, apperrors.Wrap(apperrors.ErrFileSystem, "failed to write SSH config", err)
	}

	return true, nil
}

// getIncludePath returns the path to use in the Include directive
func (m *IncludeManager) getIncludePath() string {
	// Try to use relative path from home
	homeDir, _ := os.UserHomeDir()
	if strings.HasPrefix(m.ManagedConfig, homeDir) {
		return "~" + strings.TrimPrefix(m.ManagedConfig, homeDir)
	}
	return m.ManagedConfig
}

// hasInclude checks if the Include directive already exists
func (m *IncludeManager) hasInclude(content, includePath string) bool {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "include ") {
			// Extract the path from Include directive
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				existingPath := parts[1]
				// Normalize paths for comparison
				if m.normalizePath(existingPath) == m.normalizePath(includePath) {
					return true
				}
			}
		}
	}
	return false
}

// normalizePath normalizes a path for comparison
func (m *IncludeManager) normalizePath(path string) string {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		homeDir, _ := os.UserHomeDir()
		path = filepath.Join(homeDir, path[2:])
	}
	// Clean the path
	return filepath.Clean(path)
}

// addInclude adds the Include directive to the content
func (m *IncludeManager) addInclude(content, includeLine string) string {
	// Build the include block
	includeBlock := fmt.Sprintf("%s\n%s\n", IncludeComment, includeLine)

	content = strings.TrimRight(content, "\n")

	if m.Precedence == PrecedenceManaged {
		// Add at top for managed precedence
		if content == "" {
			return includeBlock
		}
		return includeBlock + "\n" + content + "\n"
	}

	// Add at bottom for user precedence
	if content == "" {
		return includeBlock
	}
	return content + "\n\n" + includeBlock
}

// removeInclude removes the Include directive and its comment
func (m *IncludeManager) removeInclude(content, includePath string) string {
	var result []string
	lines := strings.Split(content, "\n")
	skipNext := false

	for _, line := range lines {
		// Skip the comment line before include
		if strings.TrimSpace(line) == IncludeComment {
			skipNext = true
			continue
		}

		if skipNext {
			skipNext = false
			// Check if this is our include line
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "include ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 && m.normalizePath(parts[1]) == m.normalizePath(includePath) {
					continue // Skip this line
				}
			}
		}

		// Also check for include line without comment
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "include ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 && m.normalizePath(parts[1]) == m.normalizePath(includePath) {
				continue
			}
		}

		result = append(result, line)
	}

	// Clean up multiple blank lines
	return cleanupBlankLines(strings.Join(result, "\n"))
}

// createBackup creates a backup of the main SSH config
func (m *IncludeManager) createBackup() (string, error) {
	backupPath, err := fs.SafeBackup(m.MainConfigPath)
	if err != nil {
		return "", apperrors.Wrap(apperrors.ErrFileSystem, "failed to create backup", err)
	}
	if backupPath != "" {
		log.Info("created backup of SSH config", "backup", backupPath)
	}
	return backupPath, nil
}

// cleanupBlankLines removes excessive blank lines
func cleanupBlankLines(content string) string {
	// Replace 3+ consecutive newlines with 2
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(content) + "\n"
}

// GetIncludeStatus returns the current status of the Include directive
type IncludeStatus struct {
	Exists     bool
	Position   string // "top", "bottom", "middle", or "none"
	LineNumber int
	Path       string
}

// GetStatus returns the current status of the Include directive
func (m *IncludeManager) GetStatus() (*IncludeStatus, error) {
	content, err := fs.ReadFileIfExists(m.MainConfigPath)
	if err != nil {
		return nil, err
	}

	status := &IncludeStatus{
		Exists:   false,
		Position: "none",
	}

	if len(content) == 0 {
		return status, nil
	}

	includePath := m.getIncludePath()
	lines := strings.Split(string(content), "\n")
	totalNonEmpty := 0
	includeLineNum := -1

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		totalNonEmpty++

		if strings.HasPrefix(strings.ToLower(line), "include ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && m.normalizePath(parts[1]) == m.normalizePath(includePath) {
				status.Exists = true
				status.LineNumber = i + 1
				status.Path = parts[1]
				includeLineNum = totalNonEmpty
			}
		}
	}

	if status.Exists {
		if includeLineNum == 1 {
			status.Position = "top"
		} else if includeLineNum == totalNonEmpty {
			status.Position = "bottom"
		} else {
			status.Position = "middle"
		}
	}

	return status, nil
}
