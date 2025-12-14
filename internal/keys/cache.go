package keys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/fs"
	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/kerren/op-ssh-manager/internal/model"
)

// Cache manages the local public key cache
type Cache struct {
	Dir string
}

// NewCache creates a new key cache
func NewCache(dir string) *Cache {
	return &Cache{Dir: dir}
}

// EnsureDir ensures the cache directory exists
func (c *Cache) EnsureDir() error {
	if err := os.MkdirAll(c.Dir, 0700); err != nil {
		return apperrors.Wrap(apperrors.ErrFileSystem, "failed to create key cache directory", err)
	}
	return nil
}

// WriteKey writes a public key to the cache and returns its path
func (c *Cache) WriteKey(key *model.ResolvedKey) (string, error) {
	if err := c.EnsureDir(); err != nil {
		return "", err
	}

	// Use item ID for filename to ensure uniqueness
	filename := sanitizeFilename(key.ItemID) + ".pub"
	path := filepath.Join(c.Dir, filename)

	// Format the public key properly
	pubKey := c.formatPublicKey(key)

	// Write atomically
	if err := fs.AtomicWrite(path, []byte(pubKey), 0644); err != nil {
		return "", apperrors.Wrap(apperrors.ErrFileSystem, "failed to write public key", err)
	}

	log.Debug("wrote public key to cache", "path", path, "key", key.Title)
	return path, nil
}

// WriteKeys writes multiple keys and updates their LocalPath
func (c *Cache) WriteKeys(keys []*model.ResolvedKey) error {
	if err := c.EnsureDir(); err != nil {
		return err
	}

	for _, key := range keys {
		path, err := c.WriteKey(key)
		if err != nil {
			return err
		}
		key.LocalPath = path
	}

	return nil
}

// formatPublicKey formats the public key with a comment
func (c *Cache) formatPublicKey(key *model.ResolvedKey) string {
	pubKey := strings.TrimSpace(key.PublicKey)

	// Check if key already has a comment
	parts := strings.Fields(pubKey)
	if len(parts) >= 3 {
		// Key already has a comment
		return pubKey + "\n"
	}

	// Add comment with key title
	comment := fmt.Sprintf("op-ssh-manager:%s", key.Title)
	return pubKey + " " + comment + "\n"
}

// GetKeyPath returns the expected path for a key
func (c *Cache) GetKeyPath(itemID string) string {
	filename := sanitizeFilename(itemID) + ".pub"
	return filepath.Join(c.Dir, filename)
}

// KeyExists checks if a key exists in the cache
func (c *Cache) KeyExists(itemID string) bool {
	return fs.FileExists(c.GetKeyPath(itemID))
}

// ReadKey reads a cached public key
func (c *Cache) ReadKey(itemID string) (string, error) {
	path := c.GetKeyPath(itemID)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// DeleteKey removes a key from the cache
func (c *Cache) DeleteKey(itemID string) error {
	path := c.GetKeyPath(itemID)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// CleanupStaleKeys removes keys that are no longer in the inventory
func (c *Cache) CleanupStaleKeys(validIDs map[string]bool) ([]string, error) {
	entries, err := os.ReadDir(c.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var removed []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".pub") {
			continue
		}

		// Extract item ID from filename
		itemID := strings.TrimSuffix(name, ".pub")

		if !validIDs[itemID] {
			path := filepath.Join(c.Dir, name)
			if err := os.Remove(path); err != nil {
				log.Warn("failed to remove stale key", "path", path, "error", err)
			} else {
				removed = append(removed, itemID)
				log.Debug("removed stale key", "path", path)
			}
		}
	}

	return removed, nil
}

// ListCachedKeys returns all cached key IDs
func (c *Cache) ListCachedKeys() ([]string, error) {
	entries, err := os.ReadDir(c.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".pub") {
			ids = append(ids, strings.TrimSuffix(name, ".pub"))
		}
	}

	return ids, nil
}

// sanitizeFilename sanitizes a string for use as a filename
func sanitizeFilename(s string) string {
	// Replace characters that are problematic in filenames
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(s)
}

// GetHomePath returns the path with ~ prefix for use in SSH config
func (c *Cache) GetHomePath(fullPath string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fullPath
	}

	if strings.HasPrefix(fullPath, homeDir) {
		return "~" + strings.TrimPrefix(fullPath, homeDir)
	}
	return fullPath
}
