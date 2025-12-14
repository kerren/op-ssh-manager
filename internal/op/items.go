package op

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
)

// ItemSummary contains basic information about an item from list operations
type ItemSummary struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Version         int      `json:"version"`
	Vault           VaultRef `json:"vault"`
	Category        string   `json:"category"`
	LastEditedBy    string   `json:"last_edited_by"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	AdditionalInfo  string   `json:"additional_information,omitempty"`
	Tags            []string `json:"tags,omitempty"`
}

// VaultRef is a reference to a vault
type VaultRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Item contains the full details of a 1Password item
type Item struct {
	ID             string       `json:"id"`
	Title          string       `json:"title"`
	Version        int          `json:"version"`
	Vault          VaultRef     `json:"vault"`
	Category       string       `json:"category"`
	LastEditedBy   string       `json:"last_edited_by"`
	CreatedAt      string       `json:"created_at"`
	UpdatedAt      string       `json:"updated_at"`
	AdditionalInfo string       `json:"additional_information,omitempty"`
	URLs           []ItemURL    `json:"urls,omitempty"`
	Sections       []Section    `json:"sections,omitempty"`
	Fields         []Field      `json:"fields,omitempty"`
	Tags           []string     `json:"tags,omitempty"`
}

// ItemURL represents a URL associated with an item
type ItemURL struct {
	Label   string `json:"label,omitempty"`
	Primary bool   `json:"primary,omitempty"`
	Href    string `json:"href"`
}

// Section represents a section within an item
type Section struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// Field represents a field within an item
type Field struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Purpose   string   `json:"purpose,omitempty"`
	Label     string   `json:"label"`
	Value     string   `json:"value,omitempty"`
	Reference string   `json:"reference,omitempty"`
	Section   *Section `json:"section,omitempty"`
	// For SSH keys
	Entropy float64 `json:"entropy,omitempty"`
}

// SSHKeyItem represents a 1Password SSH key item
type SSHKeyItem struct {
	Item
	PublicKey  string `json:"-"` // Extracted from fields
	PrivateKey string `json:"-"` // Reference only, never stored
	KeyType    string `json:"-"` // e.g., "ssh-ed25519", "ssh-rsa"
}

// ListItemsByTag returns all items with a specific tag
func (c *Client) ListItemsByTag(ctx context.Context, tag string, vaults ...string) ([]ItemSummary, error) {
	args := []string{"item", "list", "--tags", tag}

	// Add vault filter if specified
	for _, vault := range vaults {
		if vault != "" {
			args = append(args, "--vault", vault)
		}
	}

	items, err := ExecJSON[[]ItemSummary](c, ctx, args...)
	if err != nil {
		return nil, err
	}

	if items == nil {
		return []ItemSummary{}, nil
	}

	return *items, nil
}

// ListItemsByCategory returns all items of a specific category
func (c *Client) ListItemsByCategory(ctx context.Context, category string, vaults ...string) ([]ItemSummary, error) {
	args := []string{"item", "list", "--categories", category}

	for _, vault := range vaults {
		if vault != "" {
			args = append(args, "--vault", vault)
		}
	}

	items, err := ExecJSON[[]ItemSummary](c, ctx, args...)
	if err != nil {
		return nil, err
	}

	if items == nil {
		return []ItemSummary{}, nil
	}

	return *items, nil
}

// GetItem retrieves the full details of an item
func (c *Client) GetItem(ctx context.Context, idOrTitle string, vault ...string) (*Item, error) {
	args := []string{"item", "get", idOrTitle}

	if len(vault) > 0 && vault[0] != "" {
		args = append(args, "--vault", vault[0])
	}

	return ExecJSON[Item](c, ctx, args...)
}

// GetSSHKey retrieves an SSH key item and extracts key information
func (c *Client) GetSSHKey(ctx context.Context, idOrTitle string, vault ...string) (*SSHKeyItem, error) {
	item, err := c.GetItem(ctx, idOrTitle, vault...)
	if err != nil {
		return nil, err
	}

	if item.Category != "SSH_KEY" {
		return nil, apperrors.New(apperrors.ErrInvalidSchema, "item is not an SSH key").
			WithDetail("category", item.Category)
	}

	sshKey := &SSHKeyItem{Item: *item}

	// Extract public key from fields
	for _, field := range item.Fields {
		switch field.Label {
		case "public key":
			sshKey.PublicKey = field.Value
			// Try to determine key type from public key
			if parts := strings.Fields(field.Value); len(parts) > 0 {
				sshKey.KeyType = parts[0]
			}
		}
	}

	if sshKey.PublicKey == "" {
		return nil, apperrors.New(apperrors.ErrInvalidSchema, "SSH key item has no public key field")
	}

	return sshKey, nil
}

// GetFieldValue extracts a field value by label from an item
func (i *Item) GetFieldValue(label string) string {
	labelLower := strings.ToLower(label)
	for _, field := range i.Fields {
		if strings.ToLower(field.Label) == labelLower {
			return field.Value
		}
	}
	return ""
}

// GetFieldValueInSection extracts a field value from a specific section
func (i *Item) GetFieldValueInSection(sectionLabel, fieldLabel string) string {
	sectionLower := strings.ToLower(sectionLabel)
	fieldLower := strings.ToLower(fieldLabel)

	for _, field := range i.Fields {
		if field.Section != nil {
			if strings.ToLower(field.Section.Label) == sectionLower &&
				strings.ToLower(field.Label) == fieldLower {
				return field.Value
			}
		}
	}
	return ""
}

// GetAllFieldsInSection returns all fields in a specific section
func (i *Item) GetAllFieldsInSection(sectionLabel string) []Field {
	sectionLower := strings.ToLower(sectionLabel)
	var fields []Field

	for _, field := range i.Fields {
		if field.Section != nil && strings.ToLower(field.Section.Label) == sectionLower {
			fields = append(fields, field)
		}
	}
	return fields
}

// CreateItem creates a new item in 1Password
func (c *Client) CreateItem(ctx context.Context, category, title, vault string, fields map[string]string) (*Item, error) {
	args := []string{"item", "create", "--category", category, "--title", title}

	if vault != "" {
		args = append(args, "--vault", vault)
	}

	// Add fields as arguments
	for label, value := range fields {
		args = append(args, fmt.Sprintf("%s=%s", label, value))
	}

	return ExecJSON[Item](c, ctx, args...)
}

// CreateItemFromTemplate creates a new item from a JSON template
func (c *Client) CreateItemFromTemplate(ctx context.Context, template []byte, vault string) (*Item, error) {
	args := []string{"item", "create"}

	if vault != "" {
		args = append(args, "--vault", vault)
	}

	// Use stdin for template
	args = append(args, "--template", "-")

	// For now, we'll encode the template as a flag value
	// The actual implementation would use stdin
	var templateData map[string]interface{}
	if err := json.Unmarshal(template, &templateData); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrOPExec, "invalid template JSON", err)
	}

	// Build field arguments from template
	if fields, ok := templateData["fields"].([]interface{}); ok {
		for _, f := range fields {
			if field, ok := f.(map[string]interface{}); ok {
				label := field["label"].(string)
				value := field["value"].(string)
				args = append(args, fmt.Sprintf("%s=%s", label, value))
			}
		}
	}

	if title, ok := templateData["title"].(string); ok {
		args = append(args, "--title", title)
	}

	if category, ok := templateData["category"].(string); ok {
		args = append(args, "--category", category)
	}

	return ExecJSON[Item](c, ctx, args...)
}

// EditItem edits an existing item's fields
func (c *Client) EditItem(ctx context.Context, idOrTitle string, fields map[string]string, vault ...string) (*Item, error) {
	args := []string{"item", "edit", idOrTitle}

	if len(vault) > 0 && vault[0] != "" {
		args = append(args, "--vault", vault[0])
	}

	for label, value := range fields {
		args = append(args, fmt.Sprintf("%s=%s", label, value))
	}

	return ExecJSON[Item](c, ctx, args...)
}

// AddTag adds a tag to an item
func (c *Client) AddTag(ctx context.Context, idOrTitle, tag string, vault ...string) error {
	args := []string{"item", "edit", idOrTitle, "--tags", tag}

	if len(vault) > 0 && vault[0] != "" {
		args = append(args, "--vault", vault[0])
	}

	_, err := c.Exec(ctx, args...)
	return err
}

// DeleteItem moves an item to the trash
func (c *Client) DeleteItem(ctx context.Context, idOrTitle string, vault ...string) error {
	args := []string{"item", "delete", idOrTitle}

	if len(vault) > 0 && vault[0] != "" {
		args = append(args, "--vault", vault[0])
	}

	_, err := c.Exec(ctx, args...)
	return err
}

// ArchiveItem archives an item (moves to archive)
func (c *Client) ArchiveItem(ctx context.Context, idOrTitle string, vault ...string) error {
	args := []string{"item", "delete", idOrTitle, "--archive"}

	if len(vault) > 0 && vault[0] != "" {
		args = append(args, "--vault", vault[0])
	}

	_, err := c.Exec(ctx, args...)
	return err
}
