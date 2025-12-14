package op

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
)

// Document represents a 1Password document item
type Document struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Vault     VaultRef `json:"vault"`
	Version   int      `json:"version"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Tags      []string `json:"tags,omitempty"`
}

// DocumentMetadata contains metadata for a config history document
type DocumentMetadata struct {
	Timestamp   time.Time `json:"timestamp"`
	HostCount   int       `json:"host_count"`
	KeyCount    int       `json:"key_count"`
	ContentHash string    `json:"content_hash"`
	Version     string    `json:"version"`
}

// ListDocuments lists all document items, optionally filtered by vault
func (c *Client) ListDocuments(ctx context.Context, vault ...string) ([]ItemSummary, error) {
	return c.ListItemsByCategory(ctx, "DOCUMENT", vault...)
}

// GetDocument retrieves a document item (metadata only)
func (c *Client) GetDocument(ctx context.Context, idOrTitle string, vault ...string) (*Document, error) {
	args := []string{"document", "get", idOrTitle, "--output", "/dev/null"}

	if len(vault) > 0 && vault[0] != "" {
		args = append(args, "--vault", vault[0])
	}

	// Get the item metadata
	item, err := c.GetItem(ctx, idOrTitle, vault...)
	if err != nil {
		return nil, err
	}

	if item.Category != "DOCUMENT" {
		return nil, apperrors.New(apperrors.ErrInvalidSchema, "item is not a document").
			WithDetail("category", item.Category)
	}

	doc := &Document{
		ID:        item.ID,
		Title:     item.Title,
		Vault:     item.Vault,
		Version:   item.Version,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
		Tags:      item.Tags,
	}

	return doc, nil
}

// DownloadDocument downloads a document's content to a file or returns it
func (c *Client) DownloadDocument(ctx context.Context, idOrTitle string, destPath string, vault ...string) error {
	args := []string{"document", "get", idOrTitle, "--output", destPath}

	if len(vault) > 0 && vault[0] != "" {
		args = append(args, "--vault", vault[0])
	}

	_, err := c.Exec(ctx, args...)
	return err
}

// GetDocumentContent retrieves a document's content as bytes
func (c *Client) GetDocumentContent(ctx context.Context, idOrTitle string, vault ...string) ([]byte, error) {
	// Create a temp file to download to
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("op-doc-%d", time.Now().UnixNano()))
	defer os.Remove(tmpFile)

	if err := c.DownloadDocument(ctx, idOrTitle, tmpFile, vault...); err != nil {
		return nil, err
	}

	return os.ReadFile(tmpFile)
}

// CreateDocument creates a new document item from content
func (c *Client) CreateDocument(ctx context.Context, title string, content []byte, vault string, tags ...string) (*Document, error) {
	// Write content to a temp file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("op-doc-create-%d", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, content, 0600); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrFileSystem, "failed to create temp file", err)
	}
	defer os.Remove(tmpFile)

	args := []string{"document", "create", tmpFile, "--title", title}

	if vault != "" {
		args = append(args, "--vault", vault)
	}

	for _, tag := range tags {
		args = append(args, "--tags", tag)
	}

	return ExecJSON[Document](c, ctx, args...)
}

// UpdateDocumentContent updates an existing document's content
func (c *Client) UpdateDocumentContent(ctx context.Context, idOrTitle string, content []byte, vault ...string) error {
	// Write content to a temp file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("op-doc-update-%d", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, content, 0600); err != nil {
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to create temp file", err)
	}
	defer os.Remove(tmpFile)

	args := []string{"document", "edit", idOrTitle, tmpFile}

	if len(vault) > 0 && vault[0] != "" {
		args = append(args, "--vault", vault[0])
	}

	_, err := c.Exec(ctx, args...)
	return err
}

// FindOrCreateConfigHistoryDocument finds or creates the config history document
func (c *Client) FindOrCreateConfigHistoryDocument(ctx context.Context, tag, vault string) (*Document, error) {
	// Try to find existing document with the tag
	items, err := c.ListItemsByTag(ctx, tag, vault)
	if err != nil {
		return nil, err
	}

	// Filter for documents
	for _, item := range items {
		if item.Category == "DOCUMENT" {
			doc, err := c.GetDocument(ctx, item.ID, vault)
			if err == nil {
				return doc, nil
			}
		}
	}

	// No existing document found, create one
	initialContent := []byte("# op-ssh-manager Config History\n\nThis document stores SSH configuration history.\nManaged automatically by op-ssh-manager.\n")

	return c.CreateDocument(ctx, "op-ssh-manager Config History", initialContent, vault, tag)
}

// StoreConfigHistory stores the generated SSH config as a document with metadata
func (c *Client) StoreConfigHistory(ctx context.Context, tag, vault string, content []byte, meta DocumentMetadata) error {
	// Find or create the config history document
	doc, err := c.FindOrCreateConfigHistoryDocument(ctx, tag, vault)
	if err != nil {
		return err
	}

	// Calculate content hash
	hash := sha256.Sum256(content)
	meta.ContentHash = hex.EncodeToString(hash[:])

	// Create header with metadata
	header := fmt.Sprintf(`# op-ssh-manager Generated SSH Config
# Generated: %s
# Hosts: %d
# Keys: %d
# Content Hash: %s
# Version: %s

`,
		meta.Timestamp.Format(time.RFC3339),
		meta.HostCount,
		meta.KeyCount,
		meta.ContentHash,
		meta.Version,
	)

	fullContent := append([]byte(header), content...)

	// Update the document
	return c.UpdateDocumentContent(ctx, doc.ID, fullContent, vault)
}

// ContentHash calculates the SHA256 hash of content
func ContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
