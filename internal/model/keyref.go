package model

import (
	"fmt"
	"strings"
)

// KeyRef represents a reference to an SSH key in 1Password
type KeyRef struct {
	// Vault is the vault name or ID (optional if searching all vaults)
	Vault string `json:"vault,omitempty"`

	// Item is the item title or ID
	Item string `json:"item"`

	// Raw is the original reference string
	Raw string `json:"raw,omitempty"`
}

// ParseKeyRef parses a key reference string into a KeyRef
// Supported formats:
//   - "vault=<vaultNameOrId>;item=<itemIdOrTitle>"
//   - "item=<itemIdOrTitle>" (vault resolved by search)
//   - "<itemIdOrTitle>" (simple format, vault resolved by search)
//   - "op://<vault>/<item>" (secret reference format)
func ParseKeyRef(s string) (KeyRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return KeyRef{}, fmt.Errorf("empty key reference")
	}

	ref := KeyRef{Raw: s}

	// Check for secret reference format: op://<vault>/<item>
	if strings.HasPrefix(s, "op://") {
		parts := strings.SplitN(strings.TrimPrefix(s, "op://"), "/", 3)
		if len(parts) < 2 {
			return KeyRef{}, fmt.Errorf("invalid secret reference format: %s", s)
		}
		ref.Vault = parts[0]
		ref.Item = parts[1]
		return ref, nil
	}

	// Check for key=value format
	if strings.Contains(s, "=") {
		parts := strings.Split(s, ";")
		for _, part := range parts {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(kv[0]))
			value := strings.TrimSpace(kv[1])

			switch key {
			case "vault":
				ref.Vault = value
			case "item":
				ref.Item = value
			}
		}

		if ref.Item == "" {
			return KeyRef{}, fmt.Errorf("missing item in key reference: %s", s)
		}

		return ref, nil
	}

	// Simple format: just the item name/ID
	ref.Item = s
	return ref, nil
}

// ParseKeyRefs parses a multi-line string of key references
func ParseKeyRefs(s string) ([]KeyRef, error) {
	if s == "" {
		return nil, nil
	}

	lines := strings.Split(s, "\n")
	var refs []KeyRef

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ref, err := ParseKeyRef(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		refs = append(refs, ref)
	}

	return refs, nil
}

// String returns the string representation of a KeyRef
func (r KeyRef) String() string {
	if r.Raw != "" {
		return r.Raw
	}
	if r.Vault != "" {
		return fmt.Sprintf("vault=%s;item=%s", r.Vault, r.Item)
	}
	return fmt.Sprintf("item=%s", r.Item)
}

// IsEmpty returns true if the key reference is empty
func (r KeyRef) IsEmpty() bool {
	return r.Item == ""
}

// Equals checks if two key references refer to the same key
func (r KeyRef) Equals(other KeyRef) bool {
	// If both have vault specified, both must match
	if r.Vault != "" && other.Vault != "" {
		return r.Vault == other.Vault && r.Item == other.Item
	}
	// Otherwise just compare items
	return r.Item == other.Item
}

// UniqueKey returns a unique string key for deduplication
func (r KeyRef) UniqueKey() string {
	if r.Vault != "" {
		return r.Vault + "/" + r.Item
	}
	return r.Item
}
