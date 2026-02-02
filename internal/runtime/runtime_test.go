// Package runtime_test contains contract tests for the Runtime interface.
// These tests define the expected behavior that all Runtime implementations must satisfy.
package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/AjaxZhan/AgentFense/internal/runtime"
	"github.com/AjaxZhan/AgentFense/internal/runtime/mock"
	"github.com/AjaxZhan/AgentFense/pkg/types"
)

// RuntimeTestSuite defines contract tests for any Runtime implementation.
// Pass any runtime.Runtime implementation to run the full test suite.
type RuntimeTestSuite struct {
	// Factory creates a new Runtime instance for each test.
	// This ensures tests are isolated from each other.
	Factory func() runtime.Runtime
}

// Run executes all contract tests for the runtime.
func (s *RuntimeTestSuite) Run(t *testing.T) {
	t.Run("Create", s.TestCreate)
	t.Run("Start", s.TestStart)
	t.Run("Stop", s.TestStop)
	t.Run("Destroy", s.TestDestroy)
	t.Run("Get", s.TestGet)
	t.Run("List", s.TestList)
	t.Run("Lifecycle", s.TestLifecycle)
}

func (s *RuntimeTestSuite) TestCreate(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-1",
		CodebaseID:   "codebase-1",
		CodebasePath: "/tmp/codebase",
		MountPoint:   "/tmp/mount",
		Labels:       map[string]string{"env": "test"},
		Permissions: []types.PermissionRule{
			{Pattern: "**", Type: types.PatternGlob, Permission: types.PermRead},
		},
	}

	sandbox, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify sandbox properties
	if sandbox.ID != config.ID {
		t.Errorf("ID = %q, want %q", sandbox.ID, config.ID)
	}
	if sandbox.CodebaseID != config.CodebaseID {
		t.Errorf("CodebaseID = %q, want %q", sandbox.CodebaseID, config.CodebaseID)
	}
	if sandbox.Status != types.StatusPending {
		t.Errorf("Status = %q, want %q", sandbox.Status, types.StatusPending)
	}
	if len(sandbox.Permissions) != len(config.Permissions) {
		t.Errorf("Permissions length = %d, want %d", len(sandbox.Permissions), len(config.Permissions))
	}
	if sandbox.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func (s *RuntimeTestSuite) TestStart(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	// Create a sandbox first
	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-start",
		CodebaseID:   "codebase-1",
		CodebasePath: "/tmp/codebase",
		MountPoint:   "/tmp/mount",
	}
	_, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test starting the sandbox
	err = r.Start(ctx, config.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify status changed to running
	sandbox, err := r.Get(ctx, config.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if sandbox.Status != types.StatusRunning {
		t.Errorf("Status = %q, want %q", sandbox.Status, types.StatusRunning)
	}
	if sandbox.StartedAt == nil {
		t.Error("StartedAt should be set")
	}

	// Test starting an already running sandbox should fail
	err = r.Start(ctx, config.ID)
	if err != types.ErrAlreadyRunning {
		t.Errorf("Start on running sandbox: got %v, want ErrAlreadyRunning", err)
	}

	// Test starting non-existent sandbox
	err = r.Start(ctx, "non-existent")
	if err != types.ErrSandboxNotFound {
		t.Errorf("Start on non-existent sandbox: got %v, want ErrSandboxNotFound", err)
	}
}

func (s *RuntimeTestSuite) TestStop(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	// Create and start a sandbox
	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-stop",
		CodebaseID:   "codebase-1",
		CodebasePath: "/tmp/codebase",
		MountPoint:   "/tmp/mount",
	}
	_, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	err = r.Start(ctx, config.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Test stopping the sandbox
	err = r.Stop(ctx, config.ID)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify status changed to stopped
	sandbox, err := r.Get(ctx, config.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if sandbox.Status != types.StatusStopped {
		t.Errorf("Status = %q, want %q", sandbox.Status, types.StatusStopped)
	}
	if sandbox.StoppedAt == nil {
		t.Error("StoppedAt should be set")
	}

	// Test stopping an already stopped sandbox should fail
	err = r.Stop(ctx, config.ID)
	if err != types.ErrNotRunning {
		t.Errorf("Stop on stopped sandbox: got %v, want ErrNotRunning", err)
	}

	// Test stopping non-existent sandbox
	err = r.Stop(ctx, "non-existent")
	if err != types.ErrSandboxNotFound {
		t.Errorf("Stop on non-existent sandbox: got %v, want ErrSandboxNotFound", err)
	}
}

func (s *RuntimeTestSuite) TestDestroy(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	// Create a sandbox
	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-destroy",
		CodebaseID:   "codebase-1",
		CodebasePath: "/tmp/codebase",
		MountPoint:   "/tmp/mount",
	}
	_, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test destroying the sandbox
	err = r.Destroy(ctx, config.ID)
	if err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	// Verify sandbox no longer exists
	_, err = r.Get(ctx, config.ID)
	if err != types.ErrSandboxNotFound {
		t.Errorf("Get after destroy: got %v, want ErrSandboxNotFound", err)
	}

	// Test destroying non-existent sandbox
	err = r.Destroy(ctx, "non-existent")
	if err != types.ErrSandboxNotFound {
		t.Errorf("Destroy on non-existent sandbox: got %v, want ErrSandboxNotFound", err)
	}
}

func (s *RuntimeTestSuite) TestGet(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	// Test getting non-existent sandbox
	_, err := r.Get(ctx, "non-existent")
	if err != types.ErrSandboxNotFound {
		t.Errorf("Get on non-existent sandbox: got %v, want ErrSandboxNotFound", err)
	}

	// Create a sandbox
	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-get",
		CodebaseID:   "codebase-1",
		CodebasePath: "/tmp/codebase",
		MountPoint:   "/tmp/mount",
		Labels:       map[string]string{"key": "value"},
	}
	_, err = r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test getting the sandbox
	sandbox, err := r.Get(ctx, config.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if sandbox.ID != config.ID {
		t.Errorf("ID = %q, want %q", sandbox.ID, config.ID)
	}
	if sandbox.Labels["key"] != "value" {
		t.Errorf("Labels[key] = %q, want %q", sandbox.Labels["key"], "value")
	}
}

func (s *RuntimeTestSuite) TestList(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	// List on empty runtime should return empty list
	sandboxes, err := r.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sandboxes) != 0 {
		t.Errorf("List on empty runtime: got %d sandboxes, want 0", len(sandboxes))
	}

	// Create multiple sandboxes
	for i := 0; i < 3; i++ {
		config := &runtime.SandboxConfig{
			ID:           "test-sandbox-list-" + string(rune('a'+i)),
			CodebaseID:   "codebase-1",
			CodebasePath: "/tmp/codebase",
			MountPoint:   "/tmp/mount",
		}
		_, err := r.Create(ctx, config)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List should return all sandboxes
	sandboxes, err = r.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sandboxes) != 3 {
		t.Errorf("List: got %d sandboxes, want 3", len(sandboxes))
	}
}

func (s *RuntimeTestSuite) TestLifecycle(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-lifecycle",
		CodebaseID:   "codebase-1",
		CodebasePath: "/tmp/codebase",
		MountPoint:   "/tmp/mount",
	}

	// Create
	sandbox, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if sandbox.Status != types.StatusPending {
		t.Errorf("After Create: Status = %q, want %q", sandbox.Status, types.StatusPending)
	}

	// Start
	err = r.Start(ctx, config.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	sandbox, _ = r.Get(ctx, config.ID)
	if sandbox.Status != types.StatusRunning {
		t.Errorf("After Start: Status = %q, want %q", sandbox.Status, types.StatusRunning)
	}

	// Stop
	err = r.Stop(ctx, config.ID)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	sandbox, _ = r.Get(ctx, config.ID)
	if sandbox.Status != types.StatusStopped {
		t.Errorf("After Stop: Status = %q, want %q", sandbox.Status, types.StatusStopped)
	}

	// Destroy
	err = r.Destroy(ctx, config.ID)
	if err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}
	_, err = r.Get(ctx, config.ID)
	if err != types.ErrSandboxNotFound {
		t.Error("After Destroy: sandbox should not exist")
	}
}

// TestMockRuntime runs the contract tests against the mock implementation.
func TestMockRuntime(t *testing.T) {
	suite := &RuntimeTestSuite{
		Factory: func() runtime.Runtime {
			return mock.New()
		},
	}
	suite.Run(t)
}

// ExecutorTestSuite defines contract tests for any Executor implementation.
type ExecutorTestSuite struct {
	// Factory creates a new RuntimeWithExecutor instance for each test.
	Factory func() runtime.RuntimeWithExecutor
}

// Run executes all contract tests for the executor.
func (s *ExecutorTestSuite) Run(t *testing.T) {
	t.Run("Exec", s.TestExec)
	t.Run("ExecNotRunning", s.TestExecNotRunning)
	t.Run("ExecNotFound", s.TestExecNotFound)
}

func (s *ExecutorTestSuite) TestExec(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	// Create and start a sandbox
	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-exec",
		CodebaseID:   "codebase-1",
		CodebasePath: "/tmp/codebase",
		MountPoint:   "/tmp/mount",
	}
	_, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	err = r.Start(ctx, config.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Execute a command
	req := &types.ExecRequest{
		Command: "echo hello",
		Timeout: 5 * time.Second,
	}
	result, err := r.Exec(ctx, config.ID, req)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func (s *ExecutorTestSuite) TestExecNotRunning(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	// Create but don't start a sandbox
	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-exec-not-running",
		CodebaseID:   "codebase-1",
		CodebasePath: "/tmp/codebase",
		MountPoint:   "/tmp/mount",
	}
	_, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Execute should fail
	req := &types.ExecRequest{
		Command: "echo hello",
	}
	_, err = r.Exec(ctx, config.ID, req)
	if err != types.ErrNotRunning {
		t.Errorf("Exec on not-running sandbox: got %v, want ErrNotRunning", err)
	}
}

func (s *ExecutorTestSuite) TestExecNotFound(t *testing.T) {
	r := s.Factory()
	ctx := context.Background()

	req := &types.ExecRequest{
		Command: "echo hello",
	}
	_, err := r.Exec(ctx, "non-existent", req)
	if err != types.ErrSandboxNotFound {
		t.Errorf("Exec on non-existent sandbox: got %v, want ErrSandboxNotFound", err)
	}
}

// TestMockExecutor runs the executor contract tests against the mock implementation.
func TestMockExecutor(t *testing.T) {
	suite := &ExecutorTestSuite{
		Factory: func() runtime.RuntimeWithExecutor {
			return mock.New()
		},
	}
	suite.Run(t)
}
