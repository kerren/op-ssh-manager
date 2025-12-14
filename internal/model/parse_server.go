package model

import (
	"fmt"
	"strconv"
	"strings"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/op"
)

// ServerFieldNames defines the expected field names in 1Password items
var ServerFieldNames = struct {
	Alias       string
	Hostname    string
	Port        string
	DefaultUser string
	Keys        string
	JumpAlias   string
	Users       string
	Environment string
	OwnerTeam   string
}{
	Alias:       "alias",
	Hostname:    "hostname",
	Port:        "port",
	DefaultUser: "default_user",
	Keys:        "keys",
	JumpAlias:   "jump_alias",
	Users:       "users",
	Environment: "environment",
	OwnerTeam:   "owner_team",
}

// ParseServerRecord parses a 1Password item into a ServerRecord
func ParseServerRecord(item *op.Item) (*ServerRecord, error) {
	if item == nil {
		return nil, apperrors.New(apperrors.ErrInvalidSchema, "nil item")
	}

	server := &ServerRecord{
		ItemID:    item.ID,
		VaultID:   item.Vault.ID,
		VaultName: item.Vault.Name,
		Tags:      item.Tags,
		Port:      22, // Default port
	}

	// Track which required fields we found
	foundAlias := false
	foundHostname := false

	// Extract fields
	for _, field := range item.Fields {
		label := strings.ToLower(field.Label)
		value := strings.TrimSpace(field.Value)

		switch label {
		case ServerFieldNames.Alias:
			server.Alias = value
			foundAlias = value != ""

		case ServerFieldNames.Hostname:
			server.Hostname = value
			foundHostname = value != ""

		case ServerFieldNames.Port:
			if value != "" {
				port, err := strconv.Atoi(value)
				if err != nil {
					return nil, apperrors.New(apperrors.ErrInvalidSchema,
						fmt.Sprintf("invalid port value: %s", value)).
						WithDetail("item_id", item.ID).
						WithDetail("field", "port")
				}
				if port < 1 || port > 65535 {
					return nil, apperrors.New(apperrors.ErrInvalidSchema,
						fmt.Sprintf("port out of range: %d", port)).
						WithDetail("item_id", item.ID).
						WithDetail("field", "port")
				}
				server.Port = port
			}

		case ServerFieldNames.DefaultUser, "user":
			server.DefaultUser = value

		case ServerFieldNames.Keys:
			if value != "" {
				refs, err := ParseKeyRefs(value)
				if err != nil {
					return nil, apperrors.Wrap(apperrors.ErrInvalidSchema,
						"invalid keys field", err).
						WithDetail("item_id", item.ID).
						WithDetail("field", "keys")
				}
				server.Keys = refs
			}

		case ServerFieldNames.JumpAlias, "jump", "jumphost", "proxy":
			server.JumpAlias = value

		case ServerFieldNames.Users:
			if value != "" {
				users, err := ParseUserAccessList(value)
				if err != nil {
					return nil, apperrors.Wrap(apperrors.ErrInvalidSchema,
						"invalid users field", err).
						WithDetail("item_id", item.ID).
						WithDetail("field", "users")
				}
				server.Users = users
			}

		case ServerFieldNames.Environment, "env":
			server.Environment = value

		case ServerFieldNames.OwnerTeam, "owner", "team":
			server.OwnerTeam = value

		case "notesplain", "notes":
			server.Notes = value
		}
	}

	// Check for jump host tag
	for _, tag := range item.Tags {
		if strings.Contains(strings.ToLower(tag), "jump") {
			server.IsJumpHost = true
			break
		}
	}

	// Fallback: try to extract from item title if alias not set
	if !foundAlias && item.Title != "" {
		server.Alias = sanitizeAlias(item.Title)
		foundAlias = true
	}

	// Validate required fields
	if !foundAlias {
		return nil, apperrors.New(apperrors.ErrInvalidSchema, "missing required field: alias").
			WithDetail("item_id", item.ID).
			WithDetail("item_title", item.Title)
	}

	if !foundHostname {
		return nil, apperrors.New(apperrors.ErrInvalidSchema, "missing required field: hostname").
			WithDetail("item_id", item.ID).
			WithDetail("item_title", item.Title).
			WithDetail("alias", server.Alias)
	}

	return server, nil
}

// sanitizeAlias converts a string to a valid SSH alias
func sanitizeAlias(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces and special characters with hyphens
	replacer := strings.NewReplacer(
		" ", "-",
		"_", "-",
		".", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		"@", "-",
	)
	s = replacer.Replace(s)

	// Remove consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Trim hyphens from start and end
	s = strings.Trim(s, "-")

	return s
}

// ParseServerFromSummary creates a minimal ServerRecord from an ItemSummary
// This is useful for listing servers before fetching full details
func ParseServerFromSummary(summary *op.ItemSummary) *ServerRecord {
	return &ServerRecord{
		ItemID:    summary.ID,
		VaultID:   summary.Vault.ID,
		VaultName: summary.Vault.Name,
		Alias:     sanitizeAlias(summary.Title),
		Tags:      summary.Tags,
	}
}
