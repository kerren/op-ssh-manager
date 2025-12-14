package model

import (
	"fmt"
	"regexp"
	"strings"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
)

// ValidationResult contains the results of inventory validation
type ValidationResult struct {
	Errors   *apperrors.ValidationErrors
	Warnings []string
}

// IsValid returns true if there are no validation errors
func (r *ValidationResult) IsValid() bool {
	return !r.Errors.HasErrors()
}

// ValidateInventory performs comprehensive validation on an inventory
func ValidateInventory(inv *Inventory) *ValidationResult {
	result := &ValidationResult{
		Errors:   &apperrors.ValidationErrors{},
		Warnings: []string{},
	}

	// Collect all aliases for duplicate and reference checking
	aliases := make(map[string]string) // alias -> item ID

	for alias, server := range inv.Servers {
		// Validate each server
		validateServer(server, result)

		// Check for duplicate aliases
		if existingID, exists := aliases[alias]; exists {
			result.Errors.Add(fmt.Errorf("duplicate alias '%s': items %s and %s", alias, existingID, server.ItemID))
		}
		aliases[alias] = server.ItemID
	}

	// Validate jump host references (second pass after all servers collected)
	for _, server := range inv.Servers {
		if server.JumpAlias != "" {
			if _, exists := inv.Servers[server.JumpAlias]; !exists {
				result.Errors.Add(fmt.Errorf("server '%s' references non-existent jump host '%s'", server.Alias, server.JumpAlias))
			}
		}
	}

	// Check for circular jump host references
	for alias := range inv.Servers {
		if cycle := detectJumpCycle(inv, alias); cycle != "" {
			result.Errors.Add(fmt.Errorf("circular jump host reference detected: %s", cycle))
		}
	}

	return result
}

// validateServer validates a single server record
func validateServer(server *ServerRecord, result *ValidationResult) {
	prefix := fmt.Sprintf("server '%s'", server.Alias)

	// Validate alias format
	if !isValidAlias(server.Alias) {
		result.Errors.Add(fmt.Errorf("%s: invalid alias format (must be alphanumeric with hyphens)", prefix))
	}

	// Validate hostname
	if server.Hostname == "" {
		result.Errors.Add(fmt.Errorf("%s: hostname is required", prefix))
	} else if !isValidHostname(server.Hostname) {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: hostname '%s' may not be valid", prefix, server.Hostname))
	}

	// Validate port
	if server.Port < 1 || server.Port > 65535 {
		result.Errors.Add(fmt.Errorf("%s: port %d is out of valid range (1-65535)", prefix, server.Port))
	}

	// Warn if no keys defined (might be intentional for password auth)
	if len(server.Keys) == 0 && len(server.Users) == 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: no SSH keys defined", prefix))
	}

	// Validate key references
	for i, keyRef := range server.Keys {
		if keyRef.Item == "" {
			result.Errors.Add(fmt.Errorf("%s: key reference %d has empty item", prefix, i+1))
		}
	}

	// Validate user access entries
	for i, user := range server.Users {
		userPrefix := fmt.Sprintf("%s user %d", prefix, i+1)

		if user.Email == "" {
			result.Errors.Add(fmt.Errorf("%s: email is required", userPrefix))
		} else if !isValidEmail(user.Email) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: email '%s' may not be valid", userPrefix, user.Email))
		}

		if user.LinuxUser == "" {
			result.Errors.Add(fmt.Errorf("%s: linux user is required", userPrefix))
		} else if !isValidLinuxUser(user.LinuxUser) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: linux user '%s' may not be valid", userPrefix, user.LinuxUser))
		}
	}
}

// detectJumpCycle detects if a server is part of a circular jump chain
func detectJumpCycle(inv *Inventory, startAlias string) string {
	visited := make(map[string]bool)
	path := []string{startAlias}

	current := startAlias
	for {
		server, ok := inv.Servers[current]
		if !ok || server.JumpAlias == "" {
			return ""
		}

		if visited[server.JumpAlias] {
			// Found a cycle
			path = append(path, server.JumpAlias)
			return strings.Join(path, " -> ")
		}

		visited[current] = true
		path = append(path, server.JumpAlias)
		current = server.JumpAlias
	}
}

// Validation helper functions

var aliasRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*$`)

func isValidAlias(alias string) bool {
	return aliasRegex.MatchString(alias) && len(alias) <= 64
}

var hostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`)
var ipv4Regex = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)

func isValidHostname(hostname string) bool {
	// Accept IPv4 addresses
	if ipv4Regex.MatchString(hostname) {
		return true
	}
	// Accept hostnames
	return hostnameRegex.MatchString(hostname)
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

var linuxUserRegex = regexp.MustCompile(`^[a-z_][a-z0-9_-]*[$]?$`)

func isValidLinuxUser(user string) bool {
	return linuxUserRegex.MatchString(user) && len(user) <= 32
}

// ValidateServerRecord validates a single server record in isolation
func ValidateServerRecord(server *ServerRecord) *apperrors.ValidationErrors {
	result := &ValidationResult{
		Errors:   &apperrors.ValidationErrors{},
		Warnings: []string{},
	}
	validateServer(server, result)
	return result.Errors
}
