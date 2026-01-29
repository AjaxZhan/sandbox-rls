// Package runtime defines the interface for sandbox runtime implementations.
package runtime

import (
	"context"

	"github.com/ajaxzhan/sandbox-rls/pkg/types"
)

// SandboxConfig contains configuration for creating a new sandbox.
type SandboxConfig struct {
	ID          string
	CodebaseID  string
	CodebasePath string // Actual filesystem path to the codebase
	Permissions []types.PermissionRule
	MountPoint  string // Where to mount the FUSE filesystem
	Labels      map[string]string
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
