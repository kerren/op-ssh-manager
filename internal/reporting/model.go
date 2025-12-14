package reporting

import (
	"strings"
	"time"

	"github.com/kerren/op-ssh-manager/internal/model"
)

// InventoryRow represents a single row in an inventory report
type InventoryRow struct {
	// Server information
	Alias       string `json:"alias"`
	Hostname    string `json:"hostname"`
	Port        int    `json:"port"`
	Environment string `json:"environment,omitempty"`
	OwnerTeam   string `json:"owner_team,omitempty"`

	// User information
	RemoteUser string `json:"remote_user,omitempty"`
	UserEmail  string `json:"user_email,omitempty"`

	// Jump host information
	JumpAlias string `json:"jump_alias,omitempty"`
	JumpPath  string `json:"jump_path,omitempty"`

	// Key information
	KeyItemIDs   []string `json:"key_item_ids"`
	KeyTitles    []string `json:"key_titles"`
	KeysResolved int      `json:"keys_resolved"`
	KeysMissing  int      `json:"keys_missing"`

	// Tags
	Tags []string `json:"tags,omitempty"`
}

// ReportMetadata contains metadata about a report
type ReportMetadata struct {
	GeneratedAt  time.Time `json:"generated_at"`
	GeneratedBy  string    `json:"generated_by,omitempty"`
	AccountName  string    `json:"account_name,omitempty"`
	VaultScope   []string  `json:"vault_scope,omitempty"`
	TotalServers int       `json:"total_servers"`
	TotalKeys    int       `json:"total_keys"`
}

// Report contains a complete inventory report
type Report struct {
	Metadata ReportMetadata  `json:"metadata"`
	Rows     []*InventoryRow `json:"rows"`
}

// BuildReport builds a report from an inventory
func BuildReport(inv *model.Inventory, meta ReportMetadata) *Report {
	report := &Report{
		Metadata: meta,
		Rows:     make([]*InventoryRow, 0),
	}

	for _, server := range inv.Servers {
		// Create base row for default user
		row := buildServerRow(server, inv)
		report.Rows = append(report.Rows, row)

		// Create additional rows for per-user mappings
		for _, user := range server.Users {
			userRow := buildUserRow(server, user, inv)
			report.Rows = append(report.Rows, userRow)
		}
	}

	report.Metadata.TotalServers = len(inv.Servers)
	report.Metadata.TotalKeys = len(inv.Keys)

	return report
}

// buildServerRow builds a report row for a server
func buildServerRow(server *model.ServerRecord, inv *model.Inventory) *InventoryRow {
	row := &InventoryRow{
		Alias:       server.Alias,
		Hostname:    server.Hostname,
		Port:        server.EffectivePort(),
		Environment: server.Environment,
		OwnerTeam:   server.OwnerTeam,
		RemoteUser:  server.DefaultUser,
		JumpAlias:   server.JumpAlias,
		Tags:        server.Tags,
	}

	// Build jump path
	jumpPath := inv.GetJumpPath(server.Alias)
	if len(jumpPath) > 0 {
		row.JumpPath = strings.Join(jumpPath, " -> ")
	}

	// Collect key information
	for _, keyRef := range server.Keys {
		if resolved, ok := inv.Keys[keyRef.UniqueKey()]; ok {
			row.KeyItemIDs = append(row.KeyItemIDs, resolved.ItemID)
			row.KeyTitles = append(row.KeyTitles, resolved.Title)
			row.KeysResolved++
		} else {
			row.KeysMissing++
		}
	}

	return row
}

// buildUserRow builds a report row for a per-user mapping
func buildUserRow(server *model.ServerRecord, user model.UserAccess, inv *model.Inventory) *InventoryRow {
	row := &InventoryRow{
		Alias:       server.Alias,
		Hostname:    server.Hostname,
		Port:        server.EffectivePort(),
		Environment: server.Environment,
		OwnerTeam:   server.OwnerTeam,
		RemoteUser:  user.LinuxUser,
		UserEmail:   user.Email,
		JumpAlias:   server.JumpAlias,
		Tags:        server.Tags,
	}

	// Build jump path
	jumpPath := inv.GetJumpPath(server.Alias)
	if len(jumpPath) > 0 {
		row.JumpPath = strings.Join(jumpPath, " -> ")
	}

	// Use user-specific keys if defined, otherwise server keys
	keys := user.Keys
	if len(keys) == 0 {
		keys = server.Keys
	}

	for _, keyRef := range keys {
		if resolved, ok := inv.Keys[keyRef.UniqueKey()]; ok {
			row.KeyItemIDs = append(row.KeyItemIDs, resolved.ItemID)
			row.KeyTitles = append(row.KeyTitles, resolved.Title)
			row.KeysResolved++
		} else {
			row.KeysMissing++
		}
	}

	return row
}

// GetHeaders returns the column headers for the report
func GetHeaders() []string {
	return []string{
		"Alias",
		"Hostname",
		"Port",
		"User",
		"Email",
		"Jump Host",
		"Jump Path",
		"Keys",
		"Keys Resolved",
		"Keys Missing",
		"Environment",
		"Owner Team",
		"Tags",
	}
}

// ToStringSlice converts a row to a string slice for tabular output
func (r *InventoryRow) ToStringSlice() []string {
	return []string{
		r.Alias,
		r.Hostname,
		intToString(r.Port),
		r.RemoteUser,
		r.UserEmail,
		r.JumpAlias,
		r.JumpPath,
		strings.Join(r.KeyTitles, ", "),
		intToString(r.KeysResolved),
		intToString(r.KeysMissing),
		r.Environment,
		r.OwnerTeam,
		strings.Join(r.Tags, ", "),
	}
}

func intToString(i int) string {
	if i == 0 {
		return ""
	}
	return strings.TrimLeft(strings.Repeat("0", 10)+string(rune('0'+i%10)), "0")
}
