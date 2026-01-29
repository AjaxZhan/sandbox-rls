// Package types defines error types for the sandbox service.
package types

import (
	"errors"
	"fmt"
)

// Common errors
var (
	ErrSandboxNotFound  = errors.New("sandbox not found")
	ErrCodebaseNotFound = errors.New("codebase not found")
	ErrInvalidStatus    = errors.New("invalid sandbox status for this operation")
	ErrAlreadyRunning   = errors.New("sandbox is already running")
	ErrNotRunning       = errors.New("sandbox is not running")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotVisible       = errors.New("file or directory not visible")
	ErrInvalidPattern   = errors.New("invalid permission pattern")
	ErrTimeout          = errors.New("operation timed out")
)

// PermissionError represents a file permission error with context.
type PermissionError struct {
	Path       string
	Operation  string
	Permission Permission
	Required   Permission
}

func (e *PermissionError) Error() string {
	return fmt.Sprintf(
		"permission denied: %s on '%s' requires %s permission, but only has %s",
		e.Operation, e.Path, e.Required, e.Permission,
	)
}

// SandboxError represents a sandbox-related error with context.
type SandboxError struct {
	SandboxID string
	Op        string
	Err       error
}

func (e *SandboxError) Error() string {
	return fmt.Sprintf("sandbox %s: %s: %v", e.SandboxID, e.Op, e.Err)
}

func (e *SandboxError) Unwrap() error {
	return e.Err
}

// ExecError represents a command execution error.
type ExecError struct {
	SandboxID string
	Command   string
	ExitCode  int
	Stderr    string
}

func (e *ExecError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("exec failed in sandbox %s: exit code %d: %s", e.SandboxID, e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("exec failed in sandbox %s: exit code %d", e.SandboxID, e.ExitCode)
}
