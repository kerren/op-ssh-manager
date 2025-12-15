package model

// ServerRecord represents a managed SSH server from 1Password
type ServerRecord struct {
	// ItemID is the 1Password item ID
	ItemID string `json:"item_id"`

	// VaultID is the 1Password vault ID
	VaultID string `json:"vault_id"`

	// VaultName is the 1Password vault name
	VaultName string `json:"vault_name,omitempty"`

	// Alias is the SSH Host alias (unique identifier)
	Alias string `json:"alias"`

	// Hostname is the IP address or DNS name
	Hostname string `json:"hostname"`

	// Port is the SSH port (default: 22)
	Port int `json:"port"`

	// DefaultUser is the default SSH user for this host
	DefaultUser string `json:"default_user,omitempty"`

	// Keys is a list of key references for this server
	Keys []KeyRef `json:"keys"`

	// JumpAlias is the alias of the jump host (if any)
	JumpAlias string `json:"jump_alias,omitempty"`

	// Users contains per-user access configurations
	Users []UserAccess `json:"users,omitempty"`

	// Tags contains additional tags from 1Password
	Tags []string `json:"tags,omitempty"`

	// Notes contains any notes from the 1Password item
	Notes string `json:"notes,omitempty"`

	// Environment is an optional environment label (e.g., "production", "staging")
	Environment string `json:"environment,omitempty"`

	// OwnerTeam is an optional team ownership label
	OwnerTeam string `json:"owner_team,omitempty"`

	// IsJumpHost indicates if this server is a jump host
	IsJumpHost bool `json:"is_jump_host"`
}

// GetUser returns the default user or a specific user mapping if available
func (s *ServerRecord) GetUser(email string) string {
	// Check for per-user mapping
	for _, u := range s.Users {
		if u.Email == email {
			return u.LinuxUser
		}
	}
	return s.DefaultUser
}

// GetKeysForUser returns the keys available to a specific user
func (s *ServerRecord) GetKeysForUser(email string) []KeyRef {
	// Check for per-user mapping
	for _, u := range s.Users {
		if u.Email == email && len(u.Keys) > 0 {
			return u.Keys
		}
	}
	return s.Keys
}

// HasJumpHost returns true if this server requires a jump host
func (s *ServerRecord) HasJumpHost() bool {
	return s.JumpAlias != ""
}

// EffectivePort returns the port to use (default 22)
func (s *ServerRecord) EffectivePort() int {
	if s.Port == 0 {
		return 22
	}
	return s.Port
}

// Inventory represents the complete set of managed servers and keys
type Inventory struct {
	// Servers is a map of alias to server record
	Servers map[string]*ServerRecord `json:"servers"`

	// Keys is a map of key item ID to resolved key info
	Keys map[string]*ResolvedKey `json:"keys"`

	// JumpHosts is a list of server aliases that are jump hosts
	JumpHosts []string `json:"jump_hosts"`
}

// NewInventory creates a new empty inventory
func NewInventory() *Inventory {
	return &Inventory{
		Servers:   make(map[string]*ServerRecord),
		Keys:      make(map[string]*ResolvedKey),
		JumpHosts: []string{},
	}
}

// AddServer adds a server to the inventory
func (i *Inventory) AddServer(server *ServerRecord) {
	i.Servers[server.Alias] = server
	if server.IsJumpHost {
		i.JumpHosts = append(i.JumpHosts, server.Alias)
	}
}

// GetServer retrieves a server by alias
func (i *Inventory) GetServer(alias string) (*ServerRecord, bool) {
	server, ok := i.Servers[alias]
	return server, ok
}

// GetJumpPath returns the chain of jump hosts needed to reach a server
func (i *Inventory) GetJumpPath(alias string) []string {
	var path []string
	seen := make(map[string]bool)

	current := alias
	for {
		server, ok := i.Servers[current]
		if !ok || server.JumpAlias == "" {
			break
		}

		// Detect cycles
		if seen[server.JumpAlias] {
			break
		}
		seen[server.JumpAlias] = true

		path = append([]string{server.JumpAlias}, path...)
		current = server.JumpAlias
	}

	return path
}

// ResolvedKey represents a resolved SSH key with its public key
type ResolvedKey struct {
	// ItemID is the 1Password item ID
	ItemID string `json:"item_id"`

	// VaultID is the 1Password vault ID
	VaultID string `json:"vault_id"`

	// Account is the 1Password account user UUID that owns this key
	// This is used in agent.toml to support multiple 1Password accounts
	Account string `json:"account,omitempty"`

	// Title is the key's title in 1Password
	Title string `json:"title"`

	// PublicKey is the full public key string
	PublicKey string `json:"public_key"`

	// KeyType is the SSH key type (e.g., "ssh-ed25519", "ssh-rsa")
	KeyType string `json:"key_type"`

	// Fingerprint is the key fingerprint
	Fingerprint string `json:"fingerprint,omitempty"`

	// LocalPath is the path to the cached public key file
	LocalPath string `json:"local_path,omitempty"`

	// SourceRef is the original key reference that resolved to this key
	SourceRef KeyRef `json:"source_ref"`
}

// MissingKey represents a key that couldn't be resolved
type MissingKey struct {
	// Ref is the key reference that couldn't be resolved
	Ref KeyRef `json:"ref"`

	// Reason explains why the key couldn't be resolved
	Reason string `json:"reason"`

	// ServerAlias is the server that referenced this key
	ServerAlias string `json:"server_alias"`
}
