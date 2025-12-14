package op

import (
	"context"
	"strings"
)

// Vault represents a 1Password vault
type Vault struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type,omitempty"`
	Type        string `json:"type,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	Items       int    `json:"items,omitempty"`
}

// User represents a 1Password user
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Type      string `json:"type,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Group represents a 1Password group
type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
	Type        string `json:"type,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// VaultPermission represents a permission grant on a vault
type VaultPermission struct {
	VaultID     string
	UserOrGroup string
	Permissions []string
}

// ListVaults lists all accessible vaults
func (c *Client) ListVaults(ctx context.Context) ([]Vault, error) {
	vaults, err := ExecJSON[[]Vault](c, ctx, "vault", "list")
	if err != nil {
		return nil, err
	}
	if vaults == nil {
		return []Vault{}, nil
	}
	return *vaults, nil
}

// GetVault retrieves details about a specific vault
func (c *Client) GetVault(ctx context.Context, nameOrID string) (*Vault, error) {
	return ExecJSON[Vault](c, ctx, "vault", "get", nameOrID)
}

// CreateVault creates a new vault
func (c *Client) CreateVault(ctx context.Context, name, description string) (*Vault, error) {
	args := []string{"vault", "create", name}

	if description != "" {
		args = append(args, "--description", description)
	}

	return ExecJSON[Vault](c, ctx, args...)
}

// DeleteVault deletes a vault (use with caution!)
func (c *Client) DeleteVault(ctx context.Context, nameOrID string) error {
	_, err := c.Exec(ctx, "vault", "delete", nameOrID)
	return err
}

// ListUsers lists all users in the account
func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	users, err := ExecJSON[[]User](c, ctx, "user", "list")
	if err != nil {
		return nil, err
	}
	if users == nil {
		return []User{}, nil
	}
	return *users, nil
}

// GetUser retrieves details about a specific user
func (c *Client) GetUser(ctx context.Context, emailOrID string) (*User, error) {
	return ExecJSON[User](c, ctx, "user", "get", emailOrID)
}

// ListGroups lists all groups in the account
func (c *Client) ListGroups(ctx context.Context) ([]Group, error) {
	groups, err := ExecJSON[[]Group](c, ctx, "group", "list")
	if err != nil {
		return nil, err
	}
	if groups == nil {
		return []Group{}, nil
	}
	return *groups, nil
}

// GetGroup retrieves details about a specific group
func (c *Client) GetGroup(ctx context.Context, nameOrID string) (*Group, error) {
	return ExecJSON[Group](c, ctx, "group", "get", nameOrID)
}

// GrantUserAccess grants a user access to a vault
func (c *Client) GrantUserAccess(ctx context.Context, vault, user string, permissions ...string) error {
	args := []string{"vault", "user", "grant", "--vault", vault, "--user", user}

	if len(permissions) > 0 {
		args = append(args, "--permissions", strings.Join(permissions, ","))
	}

	_, err := c.Exec(ctx, args...)
	return err
}

// RevokeUserAccess revokes a user's access to a vault
func (c *Client) RevokeUserAccess(ctx context.Context, vault, user string) error {
	_, err := c.Exec(ctx, "vault", "user", "revoke", "--vault", vault, "--user", user)
	return err
}

// GrantGroupAccess grants a group access to a vault
func (c *Client) GrantGroupAccess(ctx context.Context, vault, group string, permissions ...string) error {
	args := []string{"vault", "group", "grant", "--vault", vault, "--group", group}

	if len(permissions) > 0 {
		args = append(args, "--permissions", strings.Join(permissions, ","))
	}

	_, err := c.Exec(ctx, args...)
	return err
}

// RevokeGroupAccess revokes a group's access to a vault
func (c *Client) RevokeGroupAccess(ctx context.Context, vault, group string) error {
	_, err := c.Exec(ctx, "vault", "group", "revoke", "--vault", vault, "--group", group)
	return err
}

// ListVaultUsers lists all users with access to a vault
func (c *Client) ListVaultUsers(ctx context.Context, vault string) ([]User, error) {
	users, err := ExecJSON[[]User](c, ctx, "vault", "user", "list", "--vault", vault)
	if err != nil {
		return nil, err
	}
	if users == nil {
		return []User{}, nil
	}
	return *users, nil
}

// ListVaultGroups lists all groups with access to a vault
func (c *Client) ListVaultGroups(ctx context.Context, vault string) ([]Group, error) {
	groups, err := ExecJSON[[]Group](c, ctx, "vault", "group", "list", "--vault", vault)
	if err != nil {
		return nil, err
	}
	if groups == nil {
		return []Group{}, nil
	}
	return *groups, nil
}

// UserVaultNaming returns the vault name for a per-user vault
func UserVaultNaming(email string, prefix string) string {
	if prefix == "" {
		prefix = "ssh-keys"
	}
	// Replace @ with - for vault naming
	sanitizedEmail := strings.ReplaceAll(email, "@", "-at-")
	return prefix + "/" + sanitizedEmail
}
