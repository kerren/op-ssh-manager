package model

import (
	"fmt"
	"strings"
)

// UserAccess represents per-user access configuration for a server
type UserAccess struct {
	// Email is the user's email address (1Password identifier)
	Email string `json:"email"`

	// LinuxUser is the Linux/SSH username to use
	LinuxUser string `json:"linux_user"`

	// Keys is the list of key references for this user (overrides server keys)
	Keys []KeyRef `json:"keys,omitempty"`
}

// ParseUserAccess parses a user access line
// Format: "email=<user@company.com>;user=<linuxUser>;keys=<ref1,ref2,...>"
func ParseUserAccess(s string) (UserAccess, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return UserAccess{}, fmt.Errorf("empty user access line")
	}

	ua := UserAccess{}

	parts := strings.Split(s, ";")
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(kv[1])

		switch key {
		case "email":
			ua.Email = value
		case "user":
			ua.LinuxUser = value
		case "keys":
			// Parse comma-separated key references
			keyParts := strings.Split(value, ",")
			for _, k := range keyParts {
				k = strings.TrimSpace(k)
				if k == "" {
					continue
				}
				ref, err := ParseKeyRef(k)
				if err != nil {
					return UserAccess{}, fmt.Errorf("invalid key reference '%s': %w", k, err)
				}
				ua.Keys = append(ua.Keys, ref)
			}
		}
	}

	if ua.Email == "" {
		return UserAccess{}, fmt.Errorf("missing email in user access: %s", s)
	}

	if ua.LinuxUser == "" {
		return UserAccess{}, fmt.Errorf("missing user in user access: %s", s)
	}

	return ua, nil
}

// ParseUserAccessList parses a multi-line string of user access entries
func ParseUserAccessList(s string) ([]UserAccess, error) {
	if s == "" {
		return nil, nil
	}

	lines := strings.Split(s, "\n")
	var users []UserAccess

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ua, err := ParseUserAccess(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		users = append(users, ua)
	}

	return users, nil
}

// String returns the string representation of a UserAccess
func (ua UserAccess) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("email=%s", ua.Email))
	parts = append(parts, fmt.Sprintf("user=%s", ua.LinuxUser))

	if len(ua.Keys) > 0 {
		var keyStrs []string
		for _, k := range ua.Keys {
			keyStrs = append(keyStrs, k.String())
		}
		parts = append(parts, fmt.Sprintf("keys=%s", strings.Join(keyStrs, ",")))
	}

	return strings.Join(parts, ";")
}

// HasCustomKeys returns true if this user has custom key mappings
func (ua UserAccess) HasCustomKeys() bool {
	return len(ua.Keys) > 0
}
