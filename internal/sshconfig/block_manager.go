package sshconfig

import (
	"fmt"
	"strings"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/fs"
	"github.com/kerren/op-ssh-manager/internal/log"
)

const (
	// ManagedBlockBegin marks the start of the managed section
	ManagedBlockBegin = "# OP_SSH_MANAGER_MANAGED_BEGIN"

	// ManagedBlockEnd marks the end of the managed section
	ManagedBlockEnd = "# OP_SSH_MANAGER_MANAGED_END"

	// ManagedBlockWarning is the warning comment in the managed block
	ManagedBlockWarning = "# DO NOT EDIT - This section is managed by op-ssh-manager"
)

// BlockManager handles inline managed blocks in ~/.ssh/config
// This is the fallback mode when Include is not available
type BlockManager struct {
	ConfigPath string
	Precedence string
}

// NewBlockManager creates a new BlockManager
func NewBlockManager(configPath, precedence string) *BlockManager {
	return &BlockManager{
		ConfigPath: configPath,
		Precedence: precedence,
	}
}

// UpdateManagedBlock updates the managed block in the SSH config
func (m *BlockManager) UpdateManagedBlock(content []byte) error {
	// Read current config
	existing, err := fs.ReadFileIfExists(m.ConfigPath)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to read SSH config", err)
	}

	// Parse existing content
	before, _, after, hasBlock := m.parseContent(string(existing))

	// Build new content
	var result strings.Builder

	managedBlock := m.buildManagedBlock(string(content))

	if m.Precedence == PrecedenceManaged {
		// Managed block at top
		result.WriteString(managedBlock)
		if before != "" {
			result.WriteString("\n\n")
			result.WriteString(before)
		}
		if after != "" {
			if before == "" && !hasBlock {
				result.WriteString("\n\n")
			}
			result.WriteString(after)
		}
	} else {
		// Managed block at bottom
		if before != "" {
			result.WriteString(before)
			result.WriteString("\n\n")
		}
		result.WriteString(managedBlock)
		if after != "" {
			result.WriteString("\n\n")
			result.WriteString(after)
		}
	}

	finalContent := cleanupBlankLines(result.String())

	// Write the updated config
	if err := fs.AtomicWrite(m.ConfigPath, []byte(finalContent), 0600); err != nil {
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to write SSH config", err)
	}

	log.Info("updated managed block in SSH config", "path", m.ConfigPath)
	return nil
}

// RemoveManagedBlock removes the managed block from the SSH config
func (m *BlockManager) RemoveManagedBlock() error {
	existing, err := fs.ReadFileIfExists(m.ConfigPath)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to read SSH config", err)
	}

	before, _, after, hasBlock := m.parseContent(string(existing))
	if !hasBlock {
		return nil // Nothing to remove
	}

	var result strings.Builder
	if before != "" {
		result.WriteString(before)
	}
	if after != "" {
		if before != "" {
			result.WriteString("\n\n")
		}
		result.WriteString(after)
	}

	finalContent := cleanupBlankLines(result.String())

	if err := fs.AtomicWrite(m.ConfigPath, []byte(finalContent), 0600); err != nil {
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to write SSH config", err)
	}

	return nil
}

// parseContent parses the SSH config into before/managed/after sections
func (m *BlockManager) parseContent(content string) (before, managed, after string, hasBlock bool) {
	lines := strings.Split(content, "\n")

	var beforeLines, managedLines, afterLines []string
	inManagedBlock := false
	foundEnd := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == ManagedBlockBegin {
			inManagedBlock = true
			hasBlock = true
			continue
		}

		if trimmed == ManagedBlockEnd {
			inManagedBlock = false
			foundEnd = true
			continue
		}

		if inManagedBlock {
			managedLines = append(managedLines, line)
		} else if foundEnd {
			afterLines = append(afterLines, line)
		} else if !hasBlock {
			beforeLines = append(beforeLines, line)
		} else {
			// We've seen the block but not the end - treat as before
			beforeLines = append(beforeLines, line)
		}
	}

	before = strings.TrimSpace(strings.Join(beforeLines, "\n"))
	managed = strings.TrimSpace(strings.Join(managedLines, "\n"))
	after = strings.TrimSpace(strings.Join(afterLines, "\n"))

	return before, managed, after, hasBlock
}

// buildManagedBlock builds the managed block with markers
func (m *BlockManager) buildManagedBlock(content string) string {
	var result strings.Builder

	result.WriteString(ManagedBlockBegin)
	result.WriteString("\n")
	result.WriteString(ManagedBlockWarning)
	result.WriteString("\n\n")
	result.WriteString(strings.TrimSpace(content))
	result.WriteString("\n\n")
	result.WriteString(ManagedBlockEnd)

	return result.String()
}

// GetManagedBlock returns the current managed block content
func (m *BlockManager) GetManagedBlock() (string, error) {
	content, err := fs.ReadFileIfExists(m.ConfigPath)
	if err != nil {
		return "", err
	}

	_, managed, _, _ := m.parseContent(string(content))
	return managed, nil
}

// HasManagedBlock checks if the config has a managed block
func (m *BlockManager) HasManagedBlock() (bool, error) {
	content, err := fs.ReadFileIfExists(m.ConfigPath)
	if err != nil {
		return false, err
	}

	_, _, _, hasBlock := m.parseContent(string(content))
	return hasBlock, nil
}

// BlockStatus represents the status of the managed block
type BlockStatus struct {
	HasBlock   bool
	LineStart  int
	LineEnd    int
	HostCount  int
}

// GetBlockStatus returns detailed status of the managed block
func (m *BlockManager) GetBlockStatus() (*BlockStatus, error) {
	content, err := fs.ReadFileIfExists(m.ConfigPath)
	if err != nil {
		return nil, err
	}

	status := &BlockStatus{}
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == ManagedBlockBegin {
			status.HasBlock = true
			status.LineStart = i + 1
		}

		if trimmed == ManagedBlockEnd {
			status.LineEnd = i + 1
		}

		if status.HasBlock && strings.HasPrefix(strings.ToLower(trimmed), "host ") {
			status.HostCount++
		}
	}

	return status, nil
}

// ValidateBlockIntegrity checks if the managed block markers are properly paired
func (m *BlockManager) ValidateBlockIntegrity() error {
	content, err := fs.ReadFileIfExists(m.ConfigPath)
	if err != nil {
		return err
	}

	contentStr := string(content)
	beginCount := strings.Count(contentStr, ManagedBlockBegin)
	endCount := strings.Count(contentStr, ManagedBlockEnd)

	if beginCount != endCount {
		return fmt.Errorf("mismatched managed block markers: %d BEGIN vs %d END", beginCount, endCount)
	}

	if beginCount > 1 {
		return fmt.Errorf("multiple managed blocks found (%d)", beginCount)
	}

	return nil
}
