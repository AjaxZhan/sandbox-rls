// Package types defines the core domain types for the sandbox service.
package types

import (
	"time"
)

// SandboxStatus represents the current state of a sandbox.
type SandboxStatus string

const (
	StatusPending  SandboxStatus = "pending"
	StatusRunning  SandboxStatus = "running"
	StatusStopped  SandboxStatus = "stopped"
	StatusError    SandboxStatus = "error"
)

// Permission represents the access level for a file or directory.
type Permission string

const (
	PermNone  Permission = "none"  // Completely invisible, not shown in ls
	PermView  Permission = "view"  // Can see filename in ls, but cannot read content
	PermRead  Permission = "read"  // Can view and read content
	PermWrite Permission = "write" // Can modify file
)

// PermissionLevel returns the numeric level of a permission for comparison.
// Higher level means more permissive.
func (p Permission) Level() int {
	switch p {
	case PermNone:
		return 0
	case PermView:
		return 1
	case PermRead:
		return 2
	case PermWrite:
		return 3
	default:
		return 0
	}
}

// PatternType indicates how a permission pattern should be matched.
type PatternType string

const (
	PatternGlob      PatternType = "glob"      // e.g., *.md, **/*.json
	PatternDirectory PatternType = "directory" // e.g., /docs/
	PatternFile      PatternType = "file"      // e.g., /config.yaml (highest priority)
)

// PermissionRule defines a permission rule for file access.
type PermissionRule struct {
	Pattern    string      `json:"pattern"`    // Path pattern: "/docs/**", "*.md", "/config.yaml"
	Type       PatternType `json:"type"`       // Pattern type: glob/directory/file
	Permission Permission  `json:"permission"` // Access level: none/view/read/write
	Priority   int         `json:"priority"`   // Higher priority rules override lower ones
}

// Sandbox represents an isolated execution environment.
type Sandbox struct {
	ID          string           `json:"id"`
	CodebaseID  string           `json:"codebase_id"`
	Permissions []PermissionRule `json:"permissions"`
	Status      SandboxStatus    `json:"status"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	StartedAt   *time.Time       `json:"started_at,omitempty"`
	StoppedAt   *time.Time       `json:"stopped_at,omitempty"`
	ExpiresAt   *time.Time       `json:"expires_at,omitempty"`

	// Runtime information (not serialized to JSON)
	PID        int    `json:"-"`
	MountPoint string `json:"-"`
}

// Codebase represents a user's file folder that can be mounted into sandboxes.
type Codebase struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"-"` // Internal storage path (not exposed)
	OwnerID   string    `json:"owner_id"`
	Size      int64     `json:"size"`
	FileCount int       `json:"file_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ExecRequest represents a command execution request.
type ExecRequest struct {
	Command string            `json:"command"`
	Stdin   string            `json:"stdin,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"workdir,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`
}

// ExecResult represents the result of a command execution.
type ExecResult struct {
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration"`
}

// ExecRecord represents a historical record of command execution.
type ExecRecord struct {
	ID         string        `json:"id"`
	SandboxID  string        `json:"sandbox_id"`
	Command    string        `json:"command"`
	Stdin      string        `json:"stdin,omitempty"`
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	ExitCode   int           `json:"exit_code"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt time.Time     `json:"finished_at"`
	Duration   time.Duration `json:"duration"`
}

// CreateSandboxRequest represents a request to create a new sandbox.
type CreateSandboxRequest struct {
	CodebaseID  string            `json:"codebase_id"`
	Permissions []PermissionRule  `json:"permissions"`
	ExpiresIn   *time.Duration    `json:"expires_in,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// CreateCodebaseRequest represents a request to create a new codebase.
type CreateCodebaseRequest struct {
	Name    string `json:"name"`
	OwnerID string `json:"owner_id"`
}
