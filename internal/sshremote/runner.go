package sshremote

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/log"
)

// Runner executes commands on remote servers via SSH
type Runner struct {
	// SSHPath is the path to the ssh binary
	SSHPath string

	// DefaultTimeout is the default command timeout
	DefaultTimeout time.Duration

	// BatchMode enables SSH batch mode (no prompts)
	BatchMode bool

	// ConnectTimeout is the SSH connection timeout
	ConnectTimeout int
}

// NewRunner creates a new SSH runner
func NewRunner() *Runner {
	return &Runner{
		SSHPath:        "ssh",
		DefaultTimeout: 30 * time.Second,
		BatchMode:      true,
		ConnectTimeout: 10,
	}
}

// RunResult contains the result of running a remote command
type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Run executes a command on a remote server
func (r *Runner) Run(ctx context.Context, alias string, command string) (*RunResult, error) {
	return r.RunWithOptions(ctx, alias, command, nil)
}

// RunOptions contains options for running a remote command
type RunOptions struct {
	Timeout        time.Duration
	BatchMode      *bool
	ConnectTimeout *int
	User           string
	IdentityFile   string
	ExtraArgs      []string
}

// RunWithOptions executes a command with additional options
func (r *Runner) RunWithOptions(ctx context.Context, alias string, command string, opts *RunOptions) (*RunResult, error) {
	args := r.buildArgs(alias, command, opts)

	timeout := r.DefaultTimeout
	if opts != nil && opts.Timeout > 0 {
		timeout = opts.Timeout
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Debug("executing remote command", "alias", alias, "command", command)

	cmd := exec.CommandContext(ctx, r.SSHPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &RunResult{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, apperrors.New(apperrors.ErrSSH, "command timed out")
		}
		return result, apperrors.Wrap(apperrors.ErrSSH,
			fmt.Sprintf("command failed (exit %d)", result.ExitCode), err).
			WithDetail("stderr", string(stderr.Bytes()))
	}

	return result, nil
}

// buildArgs builds the SSH command arguments
func (r *Runner) buildArgs(alias string, command string, opts *RunOptions) []string {
	var args []string

	// Batch mode
	batchMode := r.BatchMode
	if opts != nil && opts.BatchMode != nil {
		batchMode = *opts.BatchMode
	}
	if batchMode {
		args = append(args, "-o", "BatchMode=yes")
	}

	// Connect timeout
	connectTimeout := r.ConnectTimeout
	if opts != nil && opts.ConnectTimeout != nil {
		connectTimeout = *opts.ConnectTimeout
	}
	args = append(args, "-o", fmt.Sprintf("ConnectTimeout=%d", connectTimeout))

	// User
	if opts != nil && opts.User != "" {
		args = append(args, "-l", opts.User)
	}

	// Identity file
	if opts != nil && opts.IdentityFile != "" {
		args = append(args, "-i", opts.IdentityFile)
	}

	// Extra args
	if opts != nil && len(opts.ExtraArgs) > 0 {
		args = append(args, opts.ExtraArgs...)
	}

	// Alias
	args = append(args, alias)

	// Command
	if command != "" {
		args = append(args, command)
	}

	return args
}

// ReadFile reads a file from a remote server
func (r *Runner) ReadFile(ctx context.Context, alias, path string) ([]byte, error) {
	command := fmt.Sprintf("cat %s", shellQuote(path))
	result, err := r.Run(ctx, alias, command)
	if err != nil {
		return nil, err
	}
	return result.Stdout, nil
}

// WriteFile writes a file to a remote server
func (r *Runner) WriteFile(ctx context.Context, alias, path string, content []byte, mode string) error {
	// Use heredoc to write content
	command := fmt.Sprintf("cat > %s << 'OPSSHMGREOF'\n%s\nOPSSHMGREOF", shellQuote(path), string(content))
	_, err := r.Run(ctx, alias, command)
	if err != nil {
		return err
	}

	// Set permissions if specified
	if mode != "" {
		chmodCmd := fmt.Sprintf("chmod %s %s", mode, shellQuote(path))
		_, err = r.Run(ctx, alias, chmodCmd)
	}

	return err
}

// FileExists checks if a file exists on a remote server
func (r *Runner) FileExists(ctx context.Context, alias, path string) (bool, error) {
	command := fmt.Sprintf("test -f %s", shellQuote(path))
	result, err := r.Run(ctx, alias, command)
	if err != nil {
		if result != nil && result.ExitCode == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// MkdirP creates a directory (and parents) on a remote server
func (r *Runner) MkdirP(ctx context.Context, alias, path string, mode string) error {
	command := fmt.Sprintf("mkdir -p %s", shellQuote(path))
	if _, err := r.Run(ctx, alias, command); err != nil {
		return err
	}

	if mode != "" {
		chmodCmd := fmt.Sprintf("chmod %s %s", mode, shellQuote(path))
		_, err := r.Run(ctx, alias, chmodCmd)
		return err
	}

	return nil
}

// TestConnection tests if a connection to a server works
func (r *Runner) TestConnection(ctx context.Context, alias string) error {
	result, err := r.Run(ctx, alias, "true")
	if err != nil {
		return fmt.Errorf("connection test failed: %w (stderr: %s)", err, string(result.Stderr))
	}
	return nil
}

// shellQuote quotes a string for safe use in a shell command
func shellQuote(s string) string {
	// Simple quoting - wrap in single quotes and escape single quotes
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
