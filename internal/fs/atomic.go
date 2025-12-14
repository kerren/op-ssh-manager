package fs

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to a file atomically by writing to a temp file first
// and then renaming. This ensures the file is never in a partial state.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate random suffix for temp file
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("failed to generate random suffix: %w", err)
	}

	tempPath := path + ".tmp." + hex.EncodeToString(suffix)

	// Write to temp file
	if err := os.WriteFile(tempPath, data, perm); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Rename temp file to target (atomic on most filesystems)
	if err := os.Rename(tempPath, path); err != nil {
		// Clean up temp file on failure
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// AtomicWriteFunc writes data to a file atomically using a writer function
func AtomicWriteFunc(path string, perm os.FileMode, fn func(w io.Writer) error) error {
	dir := filepath.Dir(path)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate random suffix for temp file
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("failed to generate random suffix: %w", err)
	}

	tempPath := path + ".tmp." + hex.EncodeToString(suffix)

	// Create temp file
	f, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Write using the provided function
	if err := fn(f); err != nil {
		f.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to write content: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Rename temp file to target
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// SafeBackup creates a backup of a file if it exists
// Returns the backup path if created, empty string if file didn't exist
func SafeBackup(path string) (string, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	}

	// Generate backup path with timestamp-like suffix
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("failed to generate backup suffix: %w", err)
	}

	backupPath := path + ".bak." + hex.EncodeToString(suffix)

	// Copy file to backup
	src, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Get source file info for permissions
	info, err := src.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat source file: %w", err)
	}

	dst, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode())
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(backupPath)
		return "", fmt.Errorf("failed to copy to backup: %w", err)
	}

	return backupPath, nil
}

// ReadFileIfExists reads a file if it exists, returns empty slice if not
func ReadFileIfExists(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []byte{}, nil
	}
	return data, err
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
