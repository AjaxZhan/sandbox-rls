// Package bwrap provides a sandbox runtime implementation using bubblewrap (bwrap).
// On Linux, it uses bwrap for actual process isolation.
// On other systems (macOS, Windows), it falls back to a local process executor for development/testing.
package bwrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/ajaxzhan/sandbox-rls/internal/fs"
	rt "github.com/ajaxzhan/sandbox-rls/internal/runtime"
	"github.com/ajaxzhan/sandbox-rls/pkg/types"
)

// Config holds configuration for the BwrapRuntime.
type Config struct {
	// BwrapPath is the path to the bwrap binary (default: "bwrap")
	BwrapPath string

	// DefaultTimeout is the default timeout for operations
	DefaultTimeout time.Duration

	// WorkDir is the base directory for sandbox working directories
	WorkDir string

	// FUSEMountBase is the base directory for FUSE mount points
	FUSEMountBase string

	// EnableNetworking allows network access in sandboxes
	EnableNetworking bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		BwrapPath:        "bwrap",
		DefaultTimeout:   30 * time.Second,
		WorkDir:          "/tmp/sandbox-rls",
		FUSEMountBase:    "/tmp/sandbox-rls/fuse",
		EnableNetworking: false,
	}
}

// sandboxState holds internal state for a sandbox.
type sandboxState struct {
	sandbox        *types.Sandbox
	config         *rt.SandboxConfig
	cmd            *exec.Cmd          // The running process (if any)
	cancel         context.CancelFunc
	fuseFS         *fs.SandboxFS      // FUSE filesystem instance
	fuseCancel     context.CancelFunc // Cancel function for FUSE mount
	fuseMountPoint string             // Path where FUSE is mounted
	deltaDir       string             // Delta directory path for COW
}

// BwrapRuntime implements runtime.RuntimeWithExecutor using bubblewrap.
type BwrapRuntime struct {
	mu       sync.RWMutex
	config   *Config
	states   map[string]*sandboxState
	sessions map[string]*sessionState // Session ID -> session state
	isLinux  bool
}

// New creates a new BwrapRuntime with the given configuration.
func New(config *Config) *BwrapRuntime {
	if config == nil {
		config = DefaultConfig()
	}

	// Ensure FUSE mount base directory exists
	if config.FUSEMountBase != "" {
		os.MkdirAll(config.FUSEMountBase, 0755)
	}

	return &BwrapRuntime{
		config:  config,
		states:  make(map[string]*sandboxState),
		isLinux: runtime.GOOS == "linux",
	}
}

// Name returns the name of this runtime implementation.
func (r *BwrapRuntime) Name() string {
	if r.isLinux {
		return "bwrap"
	}
	return "bwrap-compat" // Compatibility mode for non-Linux
}

// Create creates a new sandbox but does not start it.
func (r *BwrapRuntime) Create(ctx context.Context, config *rt.SandboxConfig) (*types.Sandbox, error) {
	if config.ID == "" {
		return nil, fmt.Errorf("sandbox ID is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if sandbox already exists
	if _, exists := r.states[config.ID]; exists {
		return nil, fmt.Errorf("sandbox %s already exists", config.ID)
	}

	// Validate codebase path exists
	if config.CodebasePath != "" {
		if _, err := os.Stat(config.CodebasePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("codebase path does not exist: %s", config.CodebasePath)
		}
	}

	sandbox := &types.Sandbox{
		ID:          config.ID,
		CodebaseID:  config.CodebaseID,
		Permissions: config.Permissions,
		Status:      types.StatusPending,
		Labels:      config.Labels,
		CreatedAt:   time.Now(),
		MountPoint:  config.MountPoint,
	}

	r.states[config.ID] = &sandboxState{
		sandbox: sandbox,
		config:  config,
	}

	return sandbox, nil
}

// Start starts a previously created sandbox.
func (r *BwrapRuntime) Start(ctx context.Context, sandboxID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[sandboxID]
	if !ok {
		return types.ErrSandboxNotFound
	}

	if state.sandbox.Status == types.StatusRunning {
		return types.ErrAlreadyRunning
	}

	// On Linux, set up FUSE filesystem for permission enforcement
	// On other systems, skip FUSE (compatibility mode - no permission enforcement)
	if r.isLinux && state.config.CodebasePath != "" {
		// Create FUSE mount point directory
		fuseMountPoint := filepath.Join(r.config.FUSEMountBase, sandboxID)
		if err := os.MkdirAll(fuseMountPoint, 0755); err != nil {
			return fmt.Errorf("failed to create FUSE mount point: %w", err)
		}

		// Create Delta directory for COW support
		deltaDir := filepath.Join(r.config.WorkDir, "delta", sandboxID)
		if err := os.MkdirAll(deltaDir, 0755); err != nil {
			os.RemoveAll(fuseMountPoint)
			return fmt.Errorf("failed to create delta dir: %w", err)
		}
		state.deltaDir = deltaDir

		// Create FUSE filesystem with permission rules and delta layer
		fuseConfig := &fs.SandboxFSConfig{
			SourceDir:  state.config.CodebasePath,
			DeltaDir:   deltaDir,
			MountPoint: fuseMountPoint,
			Rules:      state.config.Permissions,
		}

		sandboxFS, err := fs.NewSandboxFS(fuseConfig)
		if err != nil {
			os.RemoveAll(fuseMountPoint)
			os.RemoveAll(deltaDir)
			return fmt.Errorf("failed to create FUSE filesystem: %w", err)
		}

		// Start FUSE mount in a goroutine
		fuseCtx, fuseCancel := context.WithCancel(context.Background())
		fuseReady := make(chan error, 1)

		go func() {
			// This blocks until fuseCtx is cancelled, but signals ready via channel
			if err := sandboxFS.MountWithReady(fuseCtx, fuseReady); err != nil && err != context.Canceled {
				// Log error but don't fail - the context was cancelled
				fmt.Printf("FUSE mount error for sandbox %s: %v\n", sandboxID, err)
			}
		}()

		// Wait for FUSE mount to be ready (with timeout)
		select {
		case err := <-fuseReady:
			if err != nil {
				fuseCancel()
				os.RemoveAll(fuseMountPoint)
				return fmt.Errorf("FUSE mount failed: %w", err)
			}
		case <-time.After(5 * time.Second):
			fuseCancel()
			os.RemoveAll(fuseMountPoint)
			return fmt.Errorf("FUSE mount timeout")
		}

		// Double check FUSE is mounted
		if !sandboxFS.IsMounted() {
			fuseCancel()
			os.RemoveAll(fuseMountPoint)
			return fmt.Errorf("FUSE filesystem failed to mount")
		}

		// Store FUSE state
		state.fuseFS = sandboxFS
		state.fuseCancel = fuseCancel
		state.fuseMountPoint = fuseMountPoint
	}

	state.sandbox.Status = types.StatusRunning
	now := time.Now()
	state.sandbox.StartedAt = &now

	return nil
}

// Stop stops a running sandbox without destroying it.
func (r *BwrapRuntime) Stop(ctx context.Context, sandboxID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[sandboxID]
	if !ok {
		return types.ErrSandboxNotFound
	}

	if state.sandbox.Status != types.StatusRunning {
		return types.ErrNotRunning
	}

	// Clean up all sessions for this sandbox
	r.cleanupSessions(sandboxID)

	// Cancel any running processes
	if state.cancel != nil {
		state.cancel()
	}

	// Kill the process if it's still running
	if state.cmd != nil && state.cmd.Process != nil {
		_ = state.cmd.Process.Kill()
		state.cmd = nil
	}

	// Unmount FUSE filesystem
	if state.fuseCancel != nil {
		state.fuseCancel()
		state.fuseCancel = nil
	}

	// Give FUSE a moment to unmount
	time.Sleep(100 * time.Millisecond)

	// Clean up mount point directory
	if state.fuseMountPoint != "" {
		os.RemoveAll(state.fuseMountPoint)
		state.fuseMountPoint = ""
	}

	state.fuseFS = nil

	state.sandbox.Status = types.StatusStopped
	now := time.Now()
	state.sandbox.StoppedAt = &now

	return nil
}

// Destroy destroys a sandbox, releasing all resources.
func (r *BwrapRuntime) Destroy(ctx context.Context, sandboxID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[sandboxID]
	if !ok {
		return types.ErrSandboxNotFound
	}

	// Clean up all sessions for this sandbox
	r.cleanupSessions(sandboxID)

	// Stop any running processes first
	if state.cancel != nil {
		state.cancel()
	}
	if state.cmd != nil && state.cmd.Process != nil {
		_ = state.cmd.Process.Kill()
	}

	// Unmount FUSE filesystem
	if state.fuseCancel != nil {
		state.fuseCancel()
	}

	// Give FUSE a moment to unmount
	time.Sleep(100 * time.Millisecond)

	// Clean up mount point directory
	if state.fuseMountPoint != "" {
		os.RemoveAll(state.fuseMountPoint)
	}

	// Clean up delta directory
	if state.deltaDir != "" {
		os.RemoveAll(state.deltaDir)
	}

	delete(r.states, sandboxID)
	return nil
}

// Get retrieves information about a sandbox.
func (r *BwrapRuntime) Get(ctx context.Context, sandboxID string) (*types.Sandbox, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, ok := r.states[sandboxID]
	if !ok {
		return nil, types.ErrSandboxNotFound
	}

	// Return a copy
	sandbox := *state.sandbox
	return &sandbox, nil
}

// List returns all sandboxes managed by this runtime.
func (r *BwrapRuntime) List(ctx context.Context) ([]*types.Sandbox, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*types.Sandbox, 0, len(r.states))
	for _, state := range r.states {
		sandbox := *state.sandbox
		result = append(result, &sandbox)
	}
	return result, nil
}

// Exec executes a command in the sandbox and returns the result.
func (r *BwrapRuntime) Exec(ctx context.Context, sandboxID string, req *types.ExecRequest) (*types.ExecResult, error) {
	r.mu.RLock()
	state, ok := r.states[sandboxID]
	if !ok {
		r.mu.RUnlock()
		return nil, types.ErrSandboxNotFound
	}

	if state.sandbox.Status != types.StatusRunning {
		r.mu.RUnlock()
		return nil, types.ErrNotRunning
	}

	config := state.config
	fuseMountPoint := state.fuseMountPoint
	fuseFS := state.fuseFS
	r.mu.RUnlock()

	// Set timeout if specified
	timeout := req.Timeout
	if timeout == 0 {
		timeout = r.config.DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	var cmd *exec.Cmd
	if r.isLinux {
		cmd = r.buildBwrapCommand(ctx, config, req, fuseMountPoint)
	} else {
		// Compatibility mode: run command directly with FUSE mount
		cmd = r.buildLocalCommand(ctx, config, req, fuseMountPoint)
	}

	// Capture output
	stdout, err := cmd.Output()
	duration := time.Since(start)

	result := &types.ExecResult{
		Duration: duration,
	}

	if err != nil {
		// Check for timeout first - context deadline exceeded takes priority
		if ctx.Err() == context.DeadlineExceeded {
			return nil, types.ErrTimeout
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
			result.Stdout = string(stdout)
			// Non-zero exit is not an error from our perspective
			// Sync delta even on non-zero exit (command completed, just with error code)
			if fuseFS != nil && fuseFS.DeltaEnabled() {
				if syncErr := fuseFS.Sync(); syncErr != nil {
					fmt.Printf("warning: sync delta failed for sandbox %s: %v\n", sandboxID, syncErr)
				}
			}
			return result, nil
		}

		return nil, fmt.Errorf("exec failed: %w", err)
	}

	result.Stdout = string(stdout)
	result.ExitCode = 0

	// Sync delta changes to shared storage after successful exec
	if fuseFS != nil && fuseFS.DeltaEnabled() {
		if syncErr := fuseFS.Sync(); syncErr != nil {
			fmt.Printf("warning: sync delta failed for sandbox %s: %v\n", sandboxID, syncErr)
		}
	}

	return result, nil
}

// ExecStream executes a command and streams output.
func (r *BwrapRuntime) ExecStream(ctx context.Context, sandboxID string, req *types.ExecRequest, output chan<- []byte) error {
	r.mu.RLock()
	state, ok := r.states[sandboxID]
	if !ok {
		r.mu.RUnlock()
		close(output)
		return types.ErrSandboxNotFound
	}

	if state.sandbox.Status != types.StatusRunning {
		r.mu.RUnlock()
		close(output)
		return types.ErrNotRunning
	}

	config := state.config
	fuseMountPoint := state.fuseMountPoint
	fuseFS := state.fuseFS
	r.mu.RUnlock()

	// Set timeout if specified
	timeout := req.Timeout
	if timeout == 0 {
		timeout = r.config.DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if r.isLinux {
		cmd = r.buildBwrapCommand(ctx, config, req, fuseMountPoint)
	} else {
		cmd = r.buildLocalCommand(ctx, config, req, fuseMountPoint)
	}

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		close(output)
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		close(output)
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Stream output
	go func() {
		defer close(output)
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case output <- data:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	waitErr := cmd.Wait()

	// Sync delta changes to shared storage after command completes
	if fuseFS != nil && fuseFS.DeltaEnabled() {
		if syncErr := fuseFS.Sync(); syncErr != nil {
			fmt.Printf("warning: sync delta failed for sandbox %s: %v\n", sandboxID, syncErr)
		}
	}

	return waitErr
}

// buildBwrapCommand builds a bwrap command for Linux.
// fuseMountPoint is the FUSE-mounted directory with permission enforcement.
func (r *BwrapRuntime) buildBwrapCommand(ctx context.Context, config *rt.SandboxConfig, req *types.ExecRequest, fuseMountPoint string) *exec.Cmd {
	args := []string{
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/lib64", "/lib64",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/sbin", "/sbin",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--unshare-pid",
		"--unshare-uts",
		"--unshare-ipc",
		"--die-with-parent",
	}

	// Network isolation
	if !r.config.EnableNetworking {
		args = append(args, "--unshare-net")
	}

	// Bind the FUSE mount point (with permission enforcement) to /workspace
	if fuseMountPoint != "" {
		workdir := "/workspace"
		if config.MountPoint != "" {
			workdir = config.MountPoint
		}
		// Use the FUSE mount point which enforces permissions
		args = append(args, "--bind", fuseMountPoint, workdir)
		args = append(args, "--chdir", workdir)
	}

	// Add the shell command
	args = append(args, "/bin/sh", "-c", req.Command)

	cmd := exec.CommandContext(ctx, r.config.BwrapPath, args...)

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set up stdin if provided
	if req.Stdin != "" {
		cmd.Stdin = nil // We'll handle this differently if needed
	}

	return cmd
}

// buildLocalCommand builds a local command for non-Linux systems (development mode).
// fuseMountPoint is the FUSE-mounted directory with permission enforcement.
func (r *BwrapRuntime) buildLocalCommand(ctx context.Context, config *rt.SandboxConfig, req *types.ExecRequest, fuseMountPoint string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", req.Command)

	// Set working directory - prefer FUSE mount point for permission enforcement
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	} else if fuseMountPoint != "" {
		cmd.Dir = fuseMountPoint
	} else if config.CodebasePath != "" {
		cmd.Dir = config.CodebasePath
	}

	// Set environment variables
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set up process group for cleanup
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return cmd
}

// IsBwrapAvailable checks if bwrap is available on the system.
func IsBwrapAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("bwrap")
	return err == nil
}

// Verify interface compliance at compile time
var _ rt.RuntimeWithExecutor = (*BwrapRuntime)(nil)
