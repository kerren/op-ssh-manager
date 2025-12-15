package fs

import (
	"fmt"
	"os"
	"runtime"
)

// SSHDirPerms is the recommended permission for .ssh directory
const SSHDirPerms = 0700

// SSHConfigPerms is the recommended permission for SSH config files
const SSHConfigPerms = 0600

// SSHKeyPerms is the recommended permission for SSH private keys
const SSHKeyPerms = 0600

// SSHPubKeyPerms is the recommended permission for SSH public keys
// Using 0600 to restrict access to owner only for better security
const SSHPubKeyPerms = 0600

// EnsureSSHDirPerms ensures the .ssh directory has correct permissions
func EnsureSSHDirPerms(path string) error {
	if runtime.GOOS == "windows" {
		// Windows handles permissions differently via ACLs
		// Best effort: just ensure the directory exists
		return os.MkdirAll(path, SSHDirPerms)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(path, SSHDirPerms); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Set permissions
	if err := os.Chmod(path, SSHDirPerms); err != nil {
		return fmt.Errorf("failed to set directory permissions: %w", err)
	}

	return nil
}

// EnsureFilePerms ensures a file has the correct permissions
func EnsureFilePerms(path string, perm os.FileMode) error {
	if runtime.GOOS == "windows" {
		// Windows handles permissions differently
		return nil
	}

	return os.Chmod(path, perm)
}

// CheckSSHDirPerms checks if the .ssh directory has correct permissions
func CheckSSHDirPerms(path string) (bool, error) {
	if runtime.GOOS == "windows" {
		// Windows handles permissions differently
		return true, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a directory", path)
	}

	// Check if permissions are too permissive
	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		return false, nil
	}

	return true, nil
}

// CheckSSHConfigPerms checks if an SSH config file has correct permissions
func CheckSSHConfigPerms(path string) (bool, error) {
	if runtime.GOOS == "windows" {
		// Windows handles permissions differently
		return true, nil
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// File doesn't exist yet, that's okay
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// Check if permissions are too permissive
	mode := info.Mode().Perm()
	if mode&0077 != 0 {
		return false, nil
	}

	return true, nil
}

// FixSSHPerms fixes SSH-related file and directory permissions
func FixSSHPerms(sshDir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	// Fix .ssh directory
	if err := os.Chmod(sshDir, SSHDirPerms); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to fix .ssh directory permissions: %w", err)
	}

	return nil
}
