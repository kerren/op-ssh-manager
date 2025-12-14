package rotation

import (
	"fmt"

	"github.com/kerren/op-ssh-manager/internal/model"
)

// Scope defines the scope of a key rotation
type Scope string

const (
	// ScopeAll rotates keys on all servers
	ScopeAll Scope = "all"

	// ScopeServer rotates keys on a specific server
	ScopeServer Scope = "server"

	// ScopeTag rotates keys on servers with a specific tag
	ScopeTag Scope = "tag"
)

// RotationPlan describes the plan for rotating a key
type RotationPlan struct {
	// OldKeyRef is the key being replaced
	OldKeyRef model.KeyRef

	// Scope is the rotation scope
	Scope Scope

	// ScopeValue is the value for the scope (server alias or tag)
	ScopeValue string

	// AffectedServers lists servers that will be affected
	AffectedServers []*AffectedServer

	// NewKeyVault is the vault for the new key
	NewKeyVault string
}

// AffectedServer represents a server affected by the rotation
type AffectedServer struct {
	// Server is the server record
	Server *model.ServerRecord

	// AffectedUsers are the users on this server using the key
	AffectedUsers []string

	// Reason explains why this server is affected
	Reason string
}

// Planner creates rotation plans
type Planner struct {
	Inventory *model.Inventory
}

// NewPlanner creates a new rotation planner
func NewPlanner(inv *model.Inventory) *Planner {
	return &Planner{Inventory: inv}
}

// CreatePlan creates a rotation plan for a key
func (p *Planner) CreatePlan(oldKeyRef model.KeyRef, scope Scope, scopeValue string) (*RotationPlan, error) {
	plan := &RotationPlan{
		OldKeyRef:  oldKeyRef,
		Scope:      scope,
		ScopeValue: scopeValue,
	}

	// Find all servers using this key
	for _, server := range p.Inventory.Servers {
		affected := p.checkServerAffected(server, oldKeyRef, scope, scopeValue)
		if affected != nil {
			plan.AffectedServers = append(plan.AffectedServers, affected)
		}
	}

	if len(plan.AffectedServers) == 0 {
		return nil, fmt.Errorf("no servers would be affected by this rotation")
	}

	return plan, nil
}

// checkServerAffected checks if a server is affected by the rotation
func (p *Planner) checkServerAffected(server *model.ServerRecord, keyRef model.KeyRef, scope Scope, scopeValue string) *AffectedServer {
	// Check scope
	switch scope {
	case ScopeServer:
		if server.Alias != scopeValue {
			return nil
		}
	case ScopeTag:
		hasTag := false
		for _, tag := range server.Tags {
			if tag == scopeValue {
				hasTag = true
				break
			}
		}
		if !hasTag {
			return nil
		}
	}

	// Check if server uses this key
	usesKey := false
	var affectedUsers []string

	for _, ref := range server.Keys {
		if ref.Equals(keyRef) {
			usesKey = true
			affectedUsers = append(affectedUsers, server.DefaultUser)
			break
		}
	}

	// Check per-user keys
	for _, user := range server.Users {
		for _, ref := range user.Keys {
			if ref.Equals(keyRef) {
				usesKey = true
				affectedUsers = append(affectedUsers, user.LinuxUser)
			}
		}
	}

	if !usesKey {
		return nil
	}

	return &AffectedServer{
		Server:        server,
		AffectedUsers: affectedUsers,
		Reason:        fmt.Sprintf("uses key %s", keyRef.String()),
	}
}

// Summary returns a summary of the rotation plan
func (p *RotationPlan) Summary() string {
	return fmt.Sprintf("Key rotation plan:\n  Key: %s\n  Scope: %s %s\n  Affected servers: %d",
		p.OldKeyRef.String(),
		p.Scope,
		p.ScopeValue,
		len(p.AffectedServers))
}

// GetAffectedServerAliases returns a list of affected server aliases
func (p *RotationPlan) GetAffectedServerAliases() []string {
	aliases := make([]string, len(p.AffectedServers))
	for i, s := range p.AffectedServers {
		aliases[i] = s.Server.Alias
	}
	return aliases
}
