package agentcfg

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/fs"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/kerren/op-ssh-manager/internal/model"
)

const (
	// ManagedBlockBegin marks the start of the managed section
	ManagedBlockBegin = "# OP_SSH_MANAGER_MANAGED_BEGIN"

	// ManagedBlockEnd marks the end of the managed section
	ManagedBlockEnd = "# OP_SSH_MANAGER_MANAGED_END"

	// ManagedBlockWarning is the warning comment
	ManagedBlockWarning = "# DO NOT EDIT - This section is managed by op-ssh-manager"
)

// AgentTomlManager manages the 1Password SSH agent configuration
type AgentTomlManager struct {
	ConfigPath string
}

// NewAgentTomlManager creates a new AgentTomlManager
func NewAgentTomlManager(configPath string) *AgentTomlManager {
	return &AgentTomlManager{
		ConfigPath: configPath,
	}
}

// GenerateRules generates agent.toml rules from an inventory
func (m *AgentTomlManager) GenerateRules(inv *model.Inventory) string {
	var buf bytes.Buffer

	// Get sorted list of server aliases for deterministic output
	aliases := make([]string, 0, len(inv.Servers))
	for alias := range inv.Servers {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	for _, alias := range aliases {
		server := inv.Servers[alias]
		keys := server.Keys

		if len(keys) == 0 {
			continue
		}

		// Collect unique key item IDs for this server
		var keyIDs []string
		for _, keyRef := range keys {
			if resolved, ok := inv.Keys[keyRef.UniqueKey()]; ok {
				keyIDs = append(keyIDs, resolved.ItemID)
			}
		}

		if len(keyIDs) == 0 {
			continue
		}

		// Generate TOML entry for this host
		buf.WriteString(fmt.Sprintf("[[ssh-keys]]\n"))
		buf.WriteString(fmt.Sprintf("# Server: %s\n", alias))

		// Use item IDs for matching
		if len(keyIDs) == 1 {
			buf.WriteString(fmt.Sprintf("item = \"%s\"\n", keyIDs[0]))
		} else {
			buf.WriteString("item = [\n")
			for _, id := range keyIDs {
				buf.WriteString(fmt.Sprintf("    \"%s\",\n", id))
			}
			buf.WriteString("]\n")
		}

		// Match on hostname
		buf.WriteString(fmt.Sprintf("host = \"%s\"\n", server.Hostname))

		buf.WriteString("\n")
	}

	return buf.String()
}

// UpdateManagedBlock updates the managed block in agent.toml
func (m *AgentTomlManager) UpdateManagedBlock(inv *model.Inventory) error {
	rules := m.GenerateRules(inv)
	return m.writeManagedBlock(rules)
}

// writeManagedBlock writes the managed block to the config file
func (m *AgentTomlManager) writeManagedBlock(content string) error {
	// Read existing content
	existing, err := fs.ReadFileIfExists(m.ConfigPath)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to read agent.toml", err)
	}

	// Parse existing content
	before, _, after, _ := m.parseContent(string(existing))

	// Build new content
	var result strings.Builder

	if before != "" {
		result.WriteString(before)
		result.WriteString("\n\n")
	}

	result.WriteString(m.buildManagedBlock(content))

	if after != "" {
		result.WriteString("\n\n")
		result.WriteString(after)
	}

	finalContent := cleanupBlankLines(result.String())

	// Write the updated config
	if err := fs.AtomicWrite(m.ConfigPath, []byte(finalContent), 0600); err != nil {
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to write agent.toml", err)
	}

	log.Info("updated managed block in agent.toml", "path", m.ConfigPath)
	return nil
}

// parseContent parses the config into before/managed/after sections
func (m *AgentTomlManager) parseContent(content string) (before, managed, after string, hasBlock bool) {
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
		}
	}

	before = strings.TrimSpace(strings.Join(beforeLines, "\n"))
	managed = strings.TrimSpace(strings.Join(managedLines, "\n"))
	after = strings.TrimSpace(strings.Join(afterLines, "\n"))

	return before, managed, after, hasBlock
}

// buildManagedBlock builds the managed block with markers
func (m *AgentTomlManager) buildManagedBlock(content string) string {
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

// RemoveManagedBlock removes the managed block
func (m *AgentTomlManager) RemoveManagedBlock() error {
	existing, err := fs.ReadFileIfExists(m.ConfigPath)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to read agent.toml", err)
	}

	before, _, after, hasBlock := m.parseContent(string(existing))
	if !hasBlock {
		return nil
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
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to write agent.toml", err)
	}

	return nil
}

// HasManagedBlock checks if the config has a managed block
func (m *AgentTomlManager) HasManagedBlock() (bool, error) {
	content, err := fs.ReadFileIfExists(m.ConfigPath)
	if err != nil {
		return false, err
	}

	_, _, _, hasBlock := m.parseContent(string(content))
	return hasBlock, nil
}

// cleanupBlankLines removes excessive blank lines
func cleanupBlankLines(content string) string {
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(content) + "\n"
}

// GenerateBasicConfig generates a basic agent.toml configuration
// This is used when no config exists yet
func GenerateBasicConfig() string {
	return `# 1Password SSH Agent Configuration
# See: https://developer.1password.com/docs/ssh/agent/config/

# Example: Restrict keys by vault
# [[ssh-keys]]
# vault = "Development"

# Example: Restrict keys by item
# [[ssh-keys]]
# item = "my-ssh-key"
# host = "github.com"
`
}
