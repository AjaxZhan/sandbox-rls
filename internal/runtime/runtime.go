// Package runtime defines the interface for sandbox runtime implementations.
package runtime

import (
	"context"

	"github.com/AjaxZhan/AgentFense/pkg/types"
)

// SandboxConfig contains configuration for creating a new sandbox.
type SandboxConfig struct {
	ID           string
	CodebaseID   string
	CodebasePath string // Actual filesystem path to the codebase
	Permissions  []types.PermissionRule
	MountPoint   string // Where to mount the FUSE filesystem
	Labels       map[string]string

	// Runtime configuration
	Runtime   types.RuntimeType     // Runtime type: bwrap, docker
	Resources *types.ResourceLimits // Resource limits (memory, CPU, pids)
	Docker    *types.DockerConfig   // Docker-specific configuration
}

// Runtime defines the interface for sandbox runtime implementations.
// Different implementations (bwrap, docker, gvisor) can be used interchangeably.
type Runtime interface {
	// Name returns the name of this runtime implementation.
	Name() string

	// Create creates a new sandbox but does not start it.
	Create(ctx context.Context, config *SandboxConfig) (*types.Sandbox, error)

	// Start starts a previously created sandbox.
	Start(ctx context.Context, sandboxID string) error

	// Stop stops a running sandbox without destroying it.
	Stop(ctx context.Context, sandboxID string) error

	// Destroy destroys a sandbox, releasing all resources.
	Destroy(ctx context.Context, sandboxID string) error

	// Get retrieves information about a sandbox.
	Get(ctx context.Context, sandboxID string) (*types.Sandbox, error)

	// List returns all sandboxes managed by this runtime.
	List(ctx context.Context) ([]*types.Sandbox, error)
}

// Executor defines the interface for executing commands in a sandbox.
type Executor interface {
	// Exec executes a command in the sandbox and returns the result.
	Exec(ctx context.Context, sandboxID string, req *types.ExecRequest) (*types.ExecResult, error)

	// ExecStream executes a command and streams output.
	ExecStream(ctx context.Context, sandboxID string, req *types.ExecRequest, output chan<- []byte) error
}

// RuntimeWithExecutor combines Runtime and Executor interfaces.
type RuntimeWithExecutor interface {
	Runtime
	Executor
}

// SessionManager defines the interface for managing stateful shell sessions.
// Sessions maintain a persistent shell process that preserves state across
// multiple command executions (working directory, environment variables, etc.).
type SessionManager interface {
	// CreateSession creates a new shell session within a sandbox.
	// The session maintains a persistent shell process.
	CreateSession(ctx context.Context, sandboxID string, config *types.SessionConfig) (*types.Session, error)

	// DestroySession destroys a session and kills all its child processes.
	DestroySession(ctx context.Context, sessionID string) error

	// GetSession retrieves information about a session.
	GetSession(ctx context.Context, sessionID string) (*types.Session, error)

	// ListSessions returns all sessions for a sandbox.
	ListSessions(ctx context.Context, sandboxID string) ([]*types.Session, error)

	// SessionExec executes a command within a session, preserving state.
	// Unlike Exec, this runs in the context of the persistent shell,
	// so working directory and environment changes persist.
	SessionExec(ctx context.Context, sessionID string, req *types.SessionExecRequest) (*types.ExecResult, error)

	// SessionExecStream executes a command within a session and streams output.
	SessionExecStream(ctx context.Context, sessionID string, req *types.SessionExecRequest, output chan<- []byte) error
}

// RuntimeWithSession combines RuntimeWithExecutor and SessionManager interfaces.
type RuntimeWithSession interface {
	RuntimeWithExecutor
	SessionManager
}
