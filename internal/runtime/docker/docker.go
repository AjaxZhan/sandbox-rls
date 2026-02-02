// Package docker provides a sandbox runtime implementation using Docker containers.
// It creates isolated execution environments using Docker with FUSE filesystem
// for permission enforcement, similar to the bwrap runtime but with stronger isolation.
package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/AjaxZhan/AgentFense/internal/fs"
	"github.com/AjaxZhan/AgentFense/internal/logging"
	rt "github.com/AjaxZhan/AgentFense/internal/runtime"
	"github.com/AjaxZhan/AgentFense/pkg/types"
	"github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"golang.org/x/sys/unix"
)

// Config holds configuration for the DockerRuntime.
type Config struct {
	// DockerHost is the Docker daemon socket address (default: uses DOCKER_HOST env or unix:///var/run/docker.sock)
	DockerHost string

	// DefaultImage is the default Docker image to use (default: "ubuntu:22.04")
	DefaultImage string

	// DefaultTimeout is the default timeout for operations
	DefaultTimeout time.Duration

	// FUSEMountBase is the base directory for FUSE mount points
	FUSEMountBase string

	// NetworkMode is the default network mode for containers ("none", "bridge", "host")
	NetworkMode string

	// EnableNetworking allows network access in sandboxes (maps to "bridge" if true, "none" if false)
	EnableNetworking bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DockerHost:       "", // Uses default Docker socket
		DefaultImage:     "ubuntu:22.04",
		DefaultTimeout:   30 * time.Second,
		FUSEMountBase:    "/tmp/agentfense/fuse",
		NetworkMode:      "none",
		EnableNetworking: false,
	}
}

const defaultFuseMountTimeout = 30 * time.Second

func fuseMountTimeout() time.Duration {
	// On macOS, go-fuse waits for mount_macfuse/mount_osxfuse to finish
	// negotiating INIT/STATFS, which can regularly exceed 5 seconds (especially
	// on first mount or under load).
	if runtime.GOOS == "darwin" {
		return 60 * time.Second
	}
	return defaultFuseMountTimeout
}

func forceUnmount(path string) {
	if path == "" {
		return
	}

	// Best-effort unmount with a few retries. This is intentionally defensive:
	// if a mount attempt "finishes late" (after caller timed out), we must not
	// leave stale mounts behind that later cause EBUSY/resource busy.
	delay := 5 * time.Millisecond
	for try := 0; try < 10; try++ {
		err := unix.Unmount(path, 0)
		if err == nil || err == unix.EINVAL || err == unix.ENOENT {
			return
		}
		if runtime.GOOS == "darwin" && err == unix.EBUSY {
			// Force unmount on macOS if the mount is busy.
			_ = unix.Unmount(path, unix.MNT_FORCE)
		}
		time.Sleep(delay)
		delay = 2*delay + 5*time.Millisecond
	}
}

func cleanupStaleFuseMounts(base string) {
	if base == "" {
		return
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		mp := filepath.Join(base, e.Name())
		forceUnmount(mp)
		_ = os.RemoveAll(mp)
	}
}

// isNoSuchFileError checks if the error indicates the file/directory doesn't exist.
// This is used to filter out expected errors during cleanup when the mount directory
// has already been removed.
func isNoSuchFileError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "no such file or directory") ||
		strings.Contains(errStr, "does not exist")
}

// sandboxState holds internal state for a sandbox.
type sandboxState struct {
	sandbox        *types.Sandbox
	config         *rt.SandboxConfig
	containerID    string             // Docker container ID
	fuseFS         *fs.SandboxFS      // FUSE filesystem instance
	fuseCancel     context.CancelFunc // Cancel function for FUSE mount
	fuseMountPoint string             // Path where FUSE is mounted
}

// DockerRuntime implements runtime.RuntimeWithSession using Docker containers.
type DockerRuntime struct {
	mu       sync.RWMutex
	config   *Config
	client   *client.Client
	states   map[string]*sandboxState
	sessions map[string]*sessionState
}

// New creates a new DockerRuntime with the given configuration.
func New(config *Config) (*DockerRuntime, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create Docker client
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	if config.DockerHost != "" {
		opts = append(opts, client.WithHost(config.DockerHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Verify Docker connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx); err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	// Ensure FUSE mount base directory exists
	if config.FUSEMountBase != "" {
		os.MkdirAll(config.FUSEMountBase, 0755)
		// The runtime currently does not persist sandbox state across restarts.
		// If the server crashes or mount/unmount races occur, we can end up with
		// stale mounts that break all subsequent runs with "resource busy".
		cleanupStaleFuseMounts(config.FUSEMountBase)
	}

	return &DockerRuntime{
		config:   config,
		client:   cli,
		states:   make(map[string]*sandboxState),
		sessions: make(map[string]*sessionState),
	}, nil
}

// Name returns the name of this runtime implementation.
func (r *DockerRuntime) Name() string {
	return "docker"
}

// Create creates a new sandbox but does not start it.
func (r *DockerRuntime) Create(ctx context.Context, config *rt.SandboxConfig) (*types.Sandbox, error) {
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

	// Determine Docker image
	image := r.config.DefaultImage
	if config.Docker != nil && config.Docker.Image != "" {
		image = config.Docker.Image
	}

	sandbox := &types.Sandbox{
		ID:          config.ID,
		CodebaseID:  config.CodebaseID,
		Permissions: config.Permissions,
		Status:      types.StatusPending,
		Labels:      config.Labels,
		CreatedAt:   time.Now(),
		MountPoint:  config.MountPoint,
		Runtime:     types.RuntimeDocker,
		Image:       image,
		Resources:   config.Resources,
	}

	r.states[config.ID] = &sandboxState{
		sandbox: sandbox,
		config:  config,
	}

	return sandbox, nil
}

// Start starts a previously created sandbox.
func (r *DockerRuntime) Start(ctx context.Context, sandboxID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[sandboxID]
	if !ok {
		return types.ErrSandboxNotFound
	}

	if state.sandbox.Status == types.StatusRunning {
		return types.ErrAlreadyRunning
	}

	// Set up FUSE filesystem for permission enforcement
	var fuseMountPoint string
	if state.config.CodebasePath != "" {
		// Create FUSE mount point directory
		fuseMountPoint = filepath.Join(r.config.FUSEMountBase, sandboxID)
		// If we somehow have a stale mountpoint from a previous failed attempt,
		// try to clean it up before proceeding.
		forceUnmount(fuseMountPoint)
		if err := os.MkdirAll(fuseMountPoint, 0755); err != nil {
			return fmt.Errorf("failed to create FUSE mount point: %w", err)
		}

		// Create FUSE filesystem with permission rules
		fuseConfig := &fs.SandboxFSConfig{
			SourceDir:  state.config.CodebasePath,
			MountPoint: fuseMountPoint,
			Rules:      state.config.Permissions,
		}

		sandboxFS, err := fs.NewSandboxFS(fuseConfig)
		if err != nil {
			os.RemoveAll(fuseMountPoint)
			return fmt.Errorf("failed to create FUSE filesystem: %w", err)
		}

		// Start FUSE mount in a goroutine
		fuseCtx, fuseCancel := context.WithCancel(context.Background())
		fuseReady := make(chan error, 1)

		go func() {
			if err := sandboxFS.MountWithReady(fuseCtx, fuseReady); err != nil && err != context.Canceled {
				// Ignore "no such file or directory" errors that occur during normal cleanup.
				// This happens when Destroy() removes the mount directory before the FUSE
				// goroutine finishes unmounting - it's expected behavior, not a real error.
				if !os.IsNotExist(err) && !isNoSuchFileError(err) {
					logging.Error("FUSE mount error",
						logging.String("sandbox_id", sandboxID),
						logging.Err(err),
					)
				}
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
		case <-time.After(fuseMountTimeout()):
			fuseCancel()
			// If the mount finishes after we time out, the mount goroutine will
			// attempt to unmount (and might fail with EBUSY). Ensure we clean up
			// late mounts to avoid accumulating stale mountpoints.
			go func(mp string, ready <-chan error) {
				select {
				case <-ready:
				case <-time.After(2 * time.Minute):
				}
				forceUnmount(mp)
				_ = os.RemoveAll(mp)
			}(fuseMountPoint, fuseReady)
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

	// Create Docker container
	containerID, err := r.createContainer(ctx, state, fuseMountPoint)
	if err != nil {
		// Clean up FUSE on error
		if state.fuseCancel != nil {
			state.fuseCancel()
		}
		if fuseMountPoint != "" {
			time.Sleep(100 * time.Millisecond)
			forceUnmount(fuseMountPoint)
			os.RemoveAll(fuseMountPoint)
		}
		return fmt.Errorf("failed to create container: %w", err)
	}

	state.containerID = containerID
	state.sandbox.ContainerID = containerID

	// Start the container
	if err := r.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		// Clean up container and FUSE on error
		r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		if state.fuseCancel != nil {
			state.fuseCancel()
		}
		if fuseMountPoint != "" {
			time.Sleep(100 * time.Millisecond)
			forceUnmount(fuseMountPoint)
			os.RemoveAll(fuseMountPoint)
		}
		return fmt.Errorf("failed to start container: %w", err)
	}

	state.sandbox.Status = types.StatusRunning
	now := time.Now()
	state.sandbox.StartedAt = &now

	return nil
}

// createContainer creates a Docker container for the sandbox.
func (r *DockerRuntime) createContainer(ctx context.Context, state *sandboxState, fuseMountPoint string) (string, error) {
	config := state.config
	sandbox := state.sandbox

	// Container configuration
	containerConfig := &container.Config{
		Image: sandbox.Image,
		Cmd:   []string{"sleep", "infinity"}, // Keep container running
		Tty:   true,
		Labels: map[string]string{
			"agentfense.sandbox-id":  sandbox.ID,
			"agentfense.codebase-id": sandbox.CodebaseID,
		},
	}

	// Add custom labels
	for k, v := range config.Labels {
		containerConfig.Labels["agentfense.label."+k] = v
	}

	// Host configuration
	hostConfig := &container.HostConfig{
		AutoRemove: false,
	}

	// Set up bind mount for FUSE filesystem
	if fuseMountPoint != "" {
		workdir := "/workspace"
		if config.MountPoint != "" {
			workdir = config.MountPoint
		}

		hostConfig.Mounts = []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: fuseMountPoint,
				Target: workdir,
				BindOptions: &mount.BindOptions{
					Propagation: mount.PropagationRSlave,
				},
			},
		}
		containerConfig.WorkingDir = workdir
	}

	// Set network mode
	networkMode := r.config.NetworkMode
	if config.Docker != nil && config.Docker.Network != "" {
		networkMode = config.Docker.Network
	} else if !r.config.EnableNetworking {
		networkMode = "none"
	}
	hostConfig.NetworkMode = container.NetworkMode(networkMode)

	// Set resource limits
	if config.Resources != nil {
		hostConfig.Resources = container.Resources{}
		if config.Resources.Memory > 0 {
			hostConfig.Resources.Memory = config.Resources.Memory
		}
		if config.Resources.CPUQuota > 0 {
			hostConfig.Resources.CPUQuota = config.Resources.CPUQuota
		}
		if config.Resources.CPUShares > 0 {
			hostConfig.Resources.CPUShares = config.Resources.CPUShares
		}
		if config.Resources.PidsLimit > 0 {
			pidsLimit := config.Resources.PidsLimit
			hostConfig.Resources.PidsLimit = &pidsLimit
		}
	}

	// Set environment variables
	if config.Docker != nil && len(config.Docker.Env) > 0 {
		for k, v := range config.Docker.Env {
			containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Create the container
	resp, err := r.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "sandbox-"+sandbox.ID)
	if err != nil {
		// If the image is missing locally, attempt to pull it and retry once.
		// This improves first-run UX (especially on macOS/Windows) and matches common Docker workflows.
		if strings.Contains(err.Error(), "No such image:") || strings.Contains(err.Error(), "not found") {
			if pullErr := r.pullImageIfNeeded(sandbox.Image); pullErr == nil {
				resp, err = r.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "sandbox-"+sandbox.ID)
				if err == nil {
					return resp.ID, nil
				}
			}
		}
		return "", err
	}

	return resp.ID, nil
}

// pullImageIfNeeded pulls the given image tag if it's not available locally.
// Uses a separate background context to avoid inheriting short RPC deadlines.
func (r *DockerRuntime) pullImageIfNeeded(image string) error {
	if image == "" {
		return fmt.Errorf("image is empty")
	}

	pullCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	reader, err := r.client.ImagePull(pullCtx, image, imagetypes.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Drain pull output so the pull actually completes.
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// Stop stops a running sandbox without destroying it.
func (r *DockerRuntime) Stop(ctx context.Context, sandboxID string) error {
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

	// Stop the container
	if state.containerID != "" {
		timeout := 10
		stopOptions := container.StopOptions{Timeout: &timeout}
		if err := r.client.ContainerStop(ctx, state.containerID, stopOptions); err != nil {
			// Log but don't fail - container might already be stopped
			logging.Warn("Failed to stop container",
				logging.String("container_id", state.containerID),
				logging.Err(err),
			)
		}
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
		forceUnmount(state.fuseMountPoint)
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
func (r *DockerRuntime) Destroy(ctx context.Context, sandboxID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[sandboxID]
	if !ok {
		return types.ErrSandboxNotFound
	}

	// Clean up all sessions for this sandbox
	r.cleanupSessions(sandboxID)

	// Remove the container
	if state.containerID != "" {
		removeOptions := container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}
		if err := r.client.ContainerRemove(ctx, state.containerID, removeOptions); err != nil {
			// Log but don't fail - container might not exist
			logging.Warn("Failed to remove container",
				logging.String("container_id", state.containerID),
				logging.Err(err),
			)
		}
	}

	// Unmount FUSE filesystem
	if state.fuseCancel != nil {
		state.fuseCancel()
	}

	// Give FUSE a moment to unmount
	time.Sleep(100 * time.Millisecond)

	// Clean up mount point directory
	if state.fuseMountPoint != "" {
		forceUnmount(state.fuseMountPoint)
		os.RemoveAll(state.fuseMountPoint)
	}

	delete(r.states, sandboxID)
	return nil
}

// Get retrieves information about a sandbox.
func (r *DockerRuntime) Get(ctx context.Context, sandboxID string) (*types.Sandbox, error) {
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
func (r *DockerRuntime) List(ctx context.Context) ([]*types.Sandbox, error) {
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
func (r *DockerRuntime) Exec(ctx context.Context, sandboxID string, req *types.ExecRequest) (*types.ExecResult, error) {
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

	containerID := state.containerID
	r.mu.RUnlock()

	// Set timeout if specified
	timeout := req.Timeout
	if timeout == 0 {
		timeout = r.config.DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	// Create exec configuration
	execConfig := container.ExecOptions{
		Cmd:          []string{"/bin/sh", "-c", req.Command},
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  req.Stdin != "",
	}

	// Set working directory if specified
	if req.WorkDir != "" {
		execConfig.WorkingDir = req.WorkDir
	}

	// Set environment variables
	for k, v := range req.Env {
		execConfig.Env = append(execConfig.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create exec instance
	execResp, err := r.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec
	attachResp, err := r.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// Write stdin if provided
	if req.Stdin != "" {
		go func() {
			defer attachResp.CloseWrite()
			io.Copy(attachResp.Conn, strings.NewReader(req.Stdin))
		}()
	}

	// Read output using stdcopy to properly demultiplex stdout/stderr
	// Docker uses a multiplexed stream format with 8-byte headers
	var stdoutBuf, stderrBuf bytes.Buffer
	_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader)
	if err != nil && err != io.EOF {
		// Check for timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, types.ErrTimeout
		}
	}

	duration := time.Since(start)

	// If we already hit deadline, treat as timeout rather than trying to inspect exec.
	// Otherwise ContainerExecInspect will return context deadline exceeded and mask the real cause.
	if ctx.Err() == context.DeadlineExceeded {
		return nil, types.ErrTimeout
	}

	// Get exit code
	inspectResp, err := r.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec: %w", err)
	}

	result := &types.ExecResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: inspectResp.ExitCode,
		Duration: duration,
	}

	return result, nil
}

// ExecStream executes a command and streams output.
func (r *DockerRuntime) ExecStream(ctx context.Context, sandboxID string, req *types.ExecRequest, output chan<- []byte) error {
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

	containerID := state.containerID
	r.mu.RUnlock()

	// Set timeout if specified
	timeout := req.Timeout
	if timeout == 0 {
		timeout = r.config.DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create exec configuration
	execConfig := container.ExecOptions{
		Cmd:          []string{"/bin/sh", "-c", req.Command},
		AttachStdout: true,
		AttachStderr: true,
		AttachStdin:  req.Stdin != "",
	}

	// Set working directory if specified
	if req.WorkDir != "" {
		execConfig.WorkingDir = req.WorkDir
	}

	// Set environment variables
	for k, v := range req.Env {
		execConfig.Env = append(execConfig.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create exec instance
	execResp, err := r.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		close(output)
		return fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec
	attachResp, err := r.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		close(output)
		return fmt.Errorf("failed to attach to exec: %w", err)
	}

	// Write stdin if provided
	if req.Stdin != "" {
		go func() {
			defer attachResp.CloseWrite()
			io.Copy(attachResp.Conn, strings.NewReader(req.Stdin))
		}()
	}

	// Stream output using stdcopy to properly demultiplex stdout/stderr
	// We use a streamWriter that sends data to the output channel
	go func() {
		defer close(output)
		defer attachResp.Close()

		// Create writers that send to the output channel
		writer := &streamWriter{output: output, ctx: ctx}

		// Use stdcopy to demultiplex - both stdout and stderr go to the same output channel
		_, _ = stdcopy.StdCopy(writer, writer, attachResp.Reader)
	}()

	// Wait for exec to complete
	for {
		inspectResp, err := r.client.ContainerExecInspect(ctx, execResp.ID)
		if err != nil {
			return fmt.Errorf("failed to inspect exec: %w", err)
		}
		if !inspectResp.Running {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return nil
}

// streamWriter implements io.Writer and sends data to a channel.
type streamWriter struct {
	output chan<- []byte
	ctx    context.Context
}

func (w *streamWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	data := make([]byte, len(p))
	copy(data, p)
	select {
	case w.output <- data:
		return len(p), nil
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	}
}

// Close closes the Docker client and cleans up resources.
func (r *DockerRuntime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clean up all sandboxes
	ctx := context.Background()
	for sandboxID, state := range r.states {
		// Clean up sessions
		r.cleanupSessions(sandboxID)

		// Remove container
		if state.containerID != "" {
			r.client.ContainerRemove(ctx, state.containerID, container.RemoveOptions{Force: true})
		}

		// Unmount FUSE
		if state.fuseCancel != nil {
			state.fuseCancel()
		}

		// Remove mount point
		if state.fuseMountPoint != "" {
			time.Sleep(100 * time.Millisecond)
			os.RemoveAll(state.fuseMountPoint)
		}
	}

	r.states = make(map[string]*sandboxState)
	r.sessions = make(map[string]*sessionState)

	return r.client.Close()
}

// cleanupSessions cleans up all sessions for a sandbox.
// Note: caller should hold r.mu lock
func (r *DockerRuntime) cleanupSessions(sandboxID string) {
	for sessionID, sessState := range r.sessions {
		if sessState.sandboxID == sandboxID {
			// Cancel context
			if sessState.cancel != nil {
				sessState.cancel()
			}
			// Close connection
			if sessState.conn != nil {
				sessState.conn.Close()
			}
			delete(r.sessions, sessionID)
		}
	}
}

// Verify interface compliance at compile time
var _ rt.RuntimeWithExecutor = (*DockerRuntime)(nil)
