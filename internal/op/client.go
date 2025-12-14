package op

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	apperrors "github.com/kerren/op-ssh-manager/internal/errors"
	"github.com/kerren/op-ssh-manager/internal/log"
)

// Client wraps the 1Password CLI (op) for executing commands
type Client struct {
	// OpPath is the path to the op binary (defaults to "op")
	OpPath string

	// Timeout is the default timeout for commands
	Timeout time.Duration
}

// ExecResult contains the result of executing an op command
type ExecResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// NewClient creates a new 1Password CLI client
func NewClient() *Client {
	return &Client{
		OpPath:  "op",
		Timeout: 30 * time.Second,
	}
}

// IsInstalled checks if the 1Password CLI is installed and accessible
func (c *Client) IsInstalled() bool {
	_, err := exec.LookPath(c.OpPath)
	return err == nil
}

// Exec executes an op command and returns the result
func (c *Client) Exec(ctx context.Context, args ...string) (*ExecResult, error) {
	log.Debug("executing op command", "args", args)

	// Create context with timeout if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, c.OpPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &ExecResult{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		return result, c.classifyError(result, err)
	}

	return result, nil
}

// ExecJSON executes an op command and parses the JSON output
func ExecJSON[T any](c *Client, ctx context.Context, args ...string) (*T, error) {
	// Ensure JSON format is requested
	args = append(args, "--format", "json")

	result, err := c.Exec(ctx, args...)
	if err != nil {
		return nil, err
	}

	var data T
	if err := json.Unmarshal(result.Stdout, &data); err != nil {
		return nil, apperrors.Wrap(apperrors.ErrOPExec, "failed to parse JSON response", err)
	}

	return &data, nil
}

// classifyError converts op CLI errors to typed errors
func (c *Client) classifyError(result *ExecResult, err error) error {
	stderr := string(result.Stderr)
	stderrLower := strings.ToLower(stderr)

	// Check for specific error patterns
	switch {
	case strings.Contains(stderrLower, "sign in"):
		return apperrors.New(apperrors.ErrAuthRequired, "1Password authentication required").
			WithRemediation("Run 'op signin' or enable app integration in 1Password desktop app")

	case strings.Contains(stderrLower, "not signed in"):
		return apperrors.New(apperrors.ErrAuthRequired, "Not signed in to 1Password").
			WithRemediation("Run 'op signin' or enable app integration in 1Password desktop app")

	case strings.Contains(stderrLower, "session expired"):
		return apperrors.New(apperrors.ErrAuthRequired, "1Password session expired").
			WithRemediation("Run 'op signin' to refresh your session")

	case strings.Contains(stderrLower, "isn't a vault"):
		return apperrors.New(apperrors.ErrNotFound, "Vault not found").
			WithDetail("stderr", stderr)

	case strings.Contains(stderrLower, "isn't an item"):
		return apperrors.New(apperrors.ErrNotFound, "Item not found").
			WithDetail("stderr", stderr)

	case strings.Contains(stderrLower, "not found"):
		return apperrors.New(apperrors.ErrNotFound, "Resource not found").
			WithDetail("stderr", stderr)

	case strings.Contains(stderrLower, "permission denied"):
		return apperrors.New(apperrors.ErrNoAccess, "Permission denied").
			WithDetail("stderr", stderr)

	case strings.Contains(stderrLower, "you do not have access"):
		return apperrors.New(apperrors.ErrNoAccess, "Access denied to resource").
			WithDetail("stderr", stderr)

	default:
		return apperrors.Wrap(apperrors.ErrOPExec, fmt.Sprintf("op command failed (exit %d)", result.ExitCode), err).
			WithDetail("stderr", stderr)
	}
}

// GetVersion returns the version of the installed op CLI
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	result, err := c.Exec(ctx, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

// Whoami returns information about the current signed-in user
func (c *Client) Whoami(ctx context.Context) (*WhoamiInfo, error) {
	return ExecJSON[WhoamiInfo](c, ctx, "whoami")
}

// WhoamiInfo contains information about the signed-in user
type WhoamiInfo struct {
	URL         string `json:"url"`
	Email       string `json:"email"`
	UserUUID    string `json:"user_uuid"`
	AccountUUID string `json:"account_uuid"`
}

// IsSignedIn checks if the user is signed in to 1Password
func (c *Client) IsSignedIn(ctx context.Context) bool {
	_, err := c.Whoami(ctx)
	return err == nil
}

// Account returns information about the current account
func (c *Client) Account(ctx context.Context) (*AccountInfo, error) {
	return ExecJSON[AccountInfo](c, ctx, "account", "get")
}

// AccountInfo contains information about the 1Password account
type AccountInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	Type      string `json:"type"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}
