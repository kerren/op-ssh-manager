package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for classification
var (
	// ErrAuthRequired indicates 1Password authentication is required
	ErrAuthRequired = errors.New("1password authentication required")

	// ErrNotFound indicates the requested item was not found
	ErrNotFound = errors.New("item not found")

	// ErrNoAccess indicates the user doesn't have access to the item/vault
	ErrNoAccess = errors.New("access denied")

	// ErrInvalidSchema indicates the item doesn't match the expected schema
	ErrInvalidSchema = errors.New("invalid schema")

	// ErrOPNotInstalled indicates the 1Password CLI is not installed
	ErrOPNotInstalled = errors.New("1password CLI (op) not installed")

	// ErrOPExec indicates a general 1Password CLI execution error
	ErrOPExec = errors.New("1password CLI execution failed")

	// ErrValidation indicates a validation error
	ErrValidation = errors.New("validation error")

	// ErrFileSystem indicates a filesystem operation error
	ErrFileSystem = errors.New("filesystem error")

	// ErrSSH indicates an SSH-related error
	ErrSSH = errors.New("ssh error")
)

// AppError is a structured application error with context
type AppError struct {
	// Kind is the sentinel error type
	Kind error
	// Message is a human-readable description
	Message string
	// Details contains additional context
	Details map[string]string
	// Wrapped is the underlying error
	Wrapped error
	// Remediation suggests how to fix the error
	Remediation string
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Wrapped)
	}
	return e.Message
}

// Unwrap implements errors.Unwrap
func (e *AppError) Unwrap() error {
	return e.Wrapped
}

// Is implements errors.Is
func (e *AppError) Is(target error) bool {
	return errors.Is(e.Kind, target)
}

// New creates a new AppError
func New(kind error, message string) *AppError {
	return &AppError{
		Kind:    kind,
		Message: message,
		Details: make(map[string]string),
	}
}

// Wrap wraps an error with additional context
func Wrap(kind error, message string, err error) *AppError {
	return &AppError{
		Kind:    kind,
		Message: message,
		Wrapped: err,
		Details: make(map[string]string),
	}
}

// WithDetail adds a detail to the error
func (e *AppError) WithDetail(key, value string) *AppError {
	e.Details[key] = value
	return e
}

// WithRemediation adds remediation instructions
func (e *AppError) WithRemediation(remediation string) *AppError {
	e.Remediation = remediation
	return e
}

// IsAuthRequired checks if the error indicates authentication is required
func IsAuthRequired(err error) bool {
	return errors.Is(err, ErrAuthRequired)
}

// IsNotFound checks if the error indicates an item was not found
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsNoAccess checks if the error indicates access was denied
func IsNoAccess(err error) bool {
	return errors.Is(err, ErrNoAccess)
}

// IsInvalidSchema checks if the error indicates an invalid schema
func IsInvalidSchema(err error) bool {
	return errors.Is(err, ErrInvalidSchema)
}

// IsOPNotInstalled checks if the error indicates 1Password CLI is not installed
func IsOPNotInstalled(err error) bool {
	return errors.Is(err, ErrOPNotInstalled)
}

// ValidationErrors collects multiple validation errors
type ValidationErrors struct {
	Errors []error
}

// Error implements the error interface
func (v *ValidationErrors) Error() string {
	if len(v.Errors) == 0 {
		return "no validation errors"
	}
	if len(v.Errors) == 1 {
		return v.Errors[0].Error()
	}
	return fmt.Sprintf("%d validation errors (first: %v)", len(v.Errors), v.Errors[0])
}

// Add adds an error to the collection
func (v *ValidationErrors) Add(err error) {
	v.Errors = append(v.Errors, err)
}

// HasErrors returns true if there are any errors
func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

// Unwrap returns the underlying errors for errors.Is/As
func (v *ValidationErrors) Unwrap() []error {
	return v.Errors
}
