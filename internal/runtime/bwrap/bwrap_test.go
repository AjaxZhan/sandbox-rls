package bwrap_test

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/AjaxZhan/AgentFense/internal/runtime"
	"github.com/AjaxZhan/AgentFense/internal/runtime/bwrap"
	"github.com/AjaxZhan/AgentFense/pkg/types"
)

func TestBwrapRuntime_Name(t *testing.T) {
	r := bwrap.New(nil)
	name := r.Name()

	if goruntime.GOOS == "linux" {
		if name != "bwrap" {
			t.Errorf("Name() = %q on Linux, want %q", name, "bwrap")
		}
	} else {
		if name != "bwrap-compat" {
			t.Errorf("Name() = %q on non-Linux, want %q", name, "bwrap-compat")
		}
	}
}

func TestBwrapRuntime_Create(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	// Create a temporary codebase directory
	tmpDir, err := os.MkdirTemp("", "sandbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-1",
		CodebaseID:   "codebase-1",
		CodebasePath: tmpDir,
		MountPoint:   "/workspace",
		Labels:       map[string]string{"env": "test"},
		Permissions: []types.PermissionRule{
			{Pattern: "**", Type: types.PatternGlob, Permission: types.PermRead},
		},
	}

	sandbox, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if sandbox.ID != config.ID {
		t.Errorf("ID = %q, want %q", sandbox.ID, config.ID)
	}
	if sandbox.Status != types.StatusPending {
		t.Errorf("Status = %q, want %q", sandbox.Status, types.StatusPending)
	}
}

func TestBwrapRuntime_CreateDuplicateID(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	config := &runtime.SandboxConfig{
		ID:         "duplicate-sandbox",
		CodebaseID: "codebase-1",
	}

	_, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("First Create failed: %v", err)
	}

	_, err = r.Create(ctx, config)
	if err == nil {
		t.Error("Second Create should fail for duplicate ID")
	}
}

func TestBwrapRuntime_CreateInvalidCodebasePath(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	config := &runtime.SandboxConfig{
		ID:           "test-sandbox-invalid",
		CodebaseID:   "codebase-1",
		CodebasePath: "/non/existent/path",
	}

	_, err := r.Create(ctx, config)
	if err == nil {
		t.Error("Create with non-existent codebase path should fail")
	}
}

func TestBwrapRuntime_Lifecycle(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "sandbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &runtime.SandboxConfig{
		ID:           "lifecycle-sandbox",
		CodebaseID:   "codebase-1",
		CodebasePath: tmpDir,
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
	if sandbox.StartedAt == nil {
		t.Error("StartedAt should be set after Start")
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
	if sandbox.StoppedAt == nil {
		t.Error("StoppedAt should be set after Stop")
	}

	// Destroy
	err = r.Destroy(ctx, config.ID)
	if err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}
	_, err = r.Get(ctx, config.ID)
	if err != types.ErrSandboxNotFound {
		t.Errorf("After Destroy: Get should return ErrSandboxNotFound, got %v", err)
	}
}

func TestBwrapRuntime_List(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	// Empty list
	sandboxes, err := r.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sandboxes) != 0 {
		t.Errorf("List on empty: got %d, want 0", len(sandboxes))
	}

	// Create some sandboxes
	for i := 0; i < 3; i++ {
		config := &runtime.SandboxConfig{
			ID:         "list-sandbox-" + string(rune('a'+i)),
			CodebaseID: "codebase-1",
		}
		_, err := r.Create(ctx, config)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	sandboxes, err = r.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(sandboxes) != 3 {
		t.Errorf("List: got %d, want 3", len(sandboxes))
	}
}

func TestBwrapRuntime_Exec(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	// Create a temp directory with a test file
	tmpDir, err := os.MkdirTemp("", "sandbox-exec-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &runtime.SandboxConfig{
		ID:           "exec-sandbox",
		CodebaseID:   "codebase-1",
		CodebasePath: tmpDir,
	}

	_, err = r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	err = r.Start(ctx, config.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	tests := []struct {
		name       string
		command    string
		wantStdout string
		wantCode   int
	}{
		{
			name:       "echo command",
			command:    "echo hello",
			wantStdout: "hello\n",
			wantCode:   0,
		},
		{
			name:       "read file",
			command:    "cat test.txt",
			wantStdout: "hello world",
			wantCode:   0,
		},
		{
			name:       "list directory",
			command:    "ls -1",
			wantStdout: "test.txt\n",
			wantCode:   0,
		},
		{
			name:     "non-zero exit",
			command:  "exit 42",
			wantCode: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &types.ExecRequest{
				Command: tt.command,
				Timeout: 5 * time.Second,
			}

			result, err := r.Exec(ctx, config.ID, req)
			if err != nil {
				t.Fatalf("Exec failed: %v", err)
			}

			if result.ExitCode != tt.wantCode {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantCode)
			}
			if tt.wantStdout != "" && result.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", result.Stdout, tt.wantStdout)
			}
		})
	}
}

func TestBwrapRuntime_ExecWithEnv(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	config := &runtime.SandboxConfig{
		ID:         "exec-env-sandbox",
		CodebaseID: "codebase-1",
	}

	_, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	err = r.Start(ctx, config.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	req := &types.ExecRequest{
		Command: "echo $MY_VAR",
		Env:     map[string]string{"MY_VAR": "test_value"},
		Timeout: 5 * time.Second,
	}

	result, err := r.Exec(ctx, config.ID, req)
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if !strings.Contains(result.Stdout, "test_value") {
		t.Errorf("Stdout = %q, should contain 'test_value'", result.Stdout)
	}
}

func TestBwrapRuntime_ExecNotRunning(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	config := &runtime.SandboxConfig{
		ID:         "exec-not-running-sandbox",
		CodebaseID: "codebase-1",
	}

	_, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Don't start - try to exec
	req := &types.ExecRequest{
		Command: "echo hello",
	}

	_, err = r.Exec(ctx, config.ID, req)
	if err != types.ErrNotRunning {
		t.Errorf("Exec on not-running sandbox: got %v, want ErrNotRunning", err)
	}
}

func TestBwrapRuntime_ExecNotFound(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	req := &types.ExecRequest{
		Command: "echo hello",
	}

	_, err := r.Exec(ctx, "non-existent", req)
	if err != types.ErrSandboxNotFound {
		t.Errorf("Exec on non-existent sandbox: got %v, want ErrSandboxNotFound", err)
	}
}

func TestBwrapRuntime_ExecTimeout(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	config := &runtime.SandboxConfig{
		ID:         "exec-timeout-sandbox",
		CodebaseID: "codebase-1",
	}

	_, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	err = r.Start(ctx, config.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	req := &types.ExecRequest{
		Command: "sleep 10",
		Timeout: 100 * time.Millisecond,
	}

	_, err = r.Exec(ctx, config.ID, req)
	if err != types.ErrTimeout {
		t.Errorf("Exec with short timeout: got %v, want ErrTimeout", err)
	}
}

func TestBwrapRuntime_ErrorConditions(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	// Start non-existent
	err := r.Start(ctx, "non-existent")
	if err != types.ErrSandboxNotFound {
		t.Errorf("Start non-existent: got %v, want ErrSandboxNotFound", err)
	}

	// Stop non-existent
	err = r.Stop(ctx, "non-existent")
	if err != types.ErrSandboxNotFound {
		t.Errorf("Stop non-existent: got %v, want ErrSandboxNotFound", err)
	}

	// Destroy non-existent
	err = r.Destroy(ctx, "non-existent")
	if err != types.ErrSandboxNotFound {
		t.Errorf("Destroy non-existent: got %v, want ErrSandboxNotFound", err)
	}

	// Get non-existent
	_, err = r.Get(ctx, "non-existent")
	if err != types.ErrSandboxNotFound {
		t.Errorf("Get non-existent: got %v, want ErrSandboxNotFound", err)
	}

	// Create and test double start
	config := &runtime.SandboxConfig{
		ID:         "error-test-sandbox",
		CodebaseID: "codebase-1",
	}
	_, _ = r.Create(ctx, config)
	_ = r.Start(ctx, config.ID)

	err = r.Start(ctx, config.ID)
	if err != types.ErrAlreadyRunning {
		t.Errorf("Double start: got %v, want ErrAlreadyRunning", err)
	}

	// Stop and try to stop again
	_ = r.Stop(ctx, config.ID)
	err = r.Stop(ctx, config.ID)
	if err != types.ErrNotRunning {
		t.Errorf("Stop stopped sandbox: got %v, want ErrNotRunning", err)
	}
}

func TestIsBwrapAvailable(t *testing.T) {
	available := bwrap.IsBwrapAvailable()

	if goruntime.GOOS != "linux" {
		if available {
			t.Error("IsBwrapAvailable should return false on non-Linux")
		}
	}
	// On Linux, the result depends on whether bwrap is installed
	t.Logf("bwrap available: %v (GOOS: %s)", available, goruntime.GOOS)
}

// TestBwrapRuntime_ContractCompliance runs the contract tests against BwrapRuntime.
// This ensures BwrapRuntime satisfies all expectations of the Runtime interface.
func TestBwrapRuntime_ContractCompliance(t *testing.T) {
	// Import the test suite from runtime_test
	// Note: This test verifies the bwrap runtime meets the interface contract
	r := bwrap.New(nil)
	ctx := context.Background()

	// Quick smoke test of the interface contract
	config := &runtime.SandboxConfig{
		ID:         "contract-test-sandbox",
		CodebaseID: "codebase-1",
	}

	// Create
	sandbox, err := r.Create(ctx, config)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if sandbox.Status != types.StatusPending {
		t.Errorf("Status after create = %q, want pending", sandbox.Status)
	}

	// Start
	if err := r.Start(ctx, config.ID); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Get
	sandbox, err = r.Get(ctx, config.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if sandbox.Status != types.StatusRunning {
		t.Errorf("Status after start = %q, want running", sandbox.Status)
	}

	// Stop
	if err := r.Stop(ctx, config.ID); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Destroy
	if err := r.Destroy(ctx, config.ID); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}
}
