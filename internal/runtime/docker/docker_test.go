//go:build integration
// +build integration

package docker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	rt "github.com/AjaxZhan/AgentFense/internal/runtime"
	"github.com/AjaxZhan/AgentFense/pkg/types"
)

// These tests require a running Docker daemon and are tagged as integration tests.
// Run with: go test -tags=integration ./internal/runtime/docker/...

func skipIfNoDocker(t *testing.T) {
	// Try to create a Docker client to verify Docker is available
	cfg := DefaultConfig()
	_, err := New(cfg)
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}
}

func TestDockerRuntime_Name(t *testing.T) {
	skipIfNoDocker(t)

	cfg := DefaultConfig()
	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	if runtime.Name() != "docker" {
		t.Errorf("Expected name 'docker', got '%s'", runtime.Name())
	}
}

func TestDockerRuntime_CreateAndDestroy(t *testing.T) {
	skipIfNoDocker(t)

	cfg := DefaultConfig()
	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create a sandbox
	sandboxConfig := &rt.SandboxConfig{
		ID:         "test-sandbox-1",
		CodebaseID: "test-codebase",
		Docker: &types.DockerConfig{
			Image: "alpine:latest",
		},
	}

	sandbox, err := runtime.Create(ctx, sandboxConfig)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	if sandbox.ID != "test-sandbox-1" {
		t.Errorf("Expected sandbox ID 'test-sandbox-1', got '%s'", sandbox.ID)
	}

	if sandbox.Status != types.StatusPending {
		t.Errorf("Expected status 'pending', got '%s'", sandbox.Status)
	}

	if sandbox.Runtime != types.RuntimeDocker {
		t.Errorf("Expected runtime 'docker', got '%s'", sandbox.Runtime)
	}

	// Destroy the sandbox
	err = runtime.Destroy(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("Failed to destroy sandbox: %v", err)
	}

	// Verify it's gone
	_, err = runtime.Get(ctx, sandbox.ID)
	if err != types.ErrSandboxNotFound {
		t.Errorf("Expected ErrSandboxNotFound, got %v", err)
	}
}

func TestDockerRuntime_StartStopDestroy(t *testing.T) {
	skipIfNoDocker(t)

	cfg := DefaultConfig()
	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create a sandbox
	sandboxConfig := &rt.SandboxConfig{
		ID:         "test-sandbox-2",
		CodebaseID: "test-codebase",
		Docker: &types.DockerConfig{
			Image: "alpine:latest",
		},
	}

	sandbox, err := runtime.Create(ctx, sandboxConfig)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer runtime.Destroy(ctx, sandbox.ID)

	// Start the sandbox
	err = runtime.Start(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("Failed to start sandbox: %v", err)
	}

	// Verify status
	sandbox, err = runtime.Get(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("Failed to get sandbox: %v", err)
	}

	if sandbox.Status != types.StatusRunning {
		t.Errorf("Expected status 'running', got '%s'", sandbox.Status)
	}

	// Stop the sandbox
	err = runtime.Stop(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("Failed to stop sandbox: %v", err)
	}

	// Verify status
	sandbox, err = runtime.Get(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("Failed to get sandbox: %v", err)
	}

	if sandbox.Status != types.StatusStopped {
		t.Errorf("Expected status 'stopped', got '%s'", sandbox.Status)
	}
}

func TestDockerRuntime_Exec(t *testing.T) {
	skipIfNoDocker(t)

	cfg := DefaultConfig()
	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create and start a sandbox
	sandboxConfig := &rt.SandboxConfig{
		ID:         "test-sandbox-exec",
		CodebaseID: "test-codebase",
		Docker: &types.DockerConfig{
			Image: "alpine:latest",
		},
	}

	sandbox, err := runtime.Create(ctx, sandboxConfig)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer runtime.Destroy(ctx, sandbox.ID)

	err = runtime.Start(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("Failed to start sandbox: %v", err)
	}

	// Execute a command
	result, err := runtime.Exec(ctx, sandbox.ID, &types.ExecRequest{
		Command: "echo 'hello world'",
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to exec command: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	// The output may contain some Docker mux headers, so just check for content
	if len(result.Stdout) == 0 {
		t.Errorf("Expected non-empty stdout")
	}
}

func TestDockerRuntime_ExecWithEnv(t *testing.T) {
	skipIfNoDocker(t)

	cfg := DefaultConfig()
	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create and start a sandbox
	sandboxConfig := &rt.SandboxConfig{
		ID:         "test-sandbox-env",
		CodebaseID: "test-codebase",
		Docker: &types.DockerConfig{
			Image: "alpine:latest",
		},
	}

	sandbox, err := runtime.Create(ctx, sandboxConfig)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer runtime.Destroy(ctx, sandbox.ID)

	err = runtime.Start(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("Failed to start sandbox: %v", err)
	}

	// Execute a command with environment variables
	result, err := runtime.Exec(ctx, sandbox.ID, &types.ExecRequest{
		Command: "echo $MY_VAR",
		Env: map[string]string{
			"MY_VAR": "test_value",
		},
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to exec command: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
}

func TestDockerRuntime_ResourceLimits(t *testing.T) {
	skipIfNoDocker(t)

	cfg := DefaultConfig()
	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create a sandbox with resource limits
	sandboxConfig := &rt.SandboxConfig{
		ID:         "test-sandbox-limits",
		CodebaseID: "test-codebase",
		Docker: &types.DockerConfig{
			Image: "alpine:latest",
		},
		Resources: &types.ResourceLimits{
			Memory:    64 * 1024 * 1024, // 64MB
			PidsLimit: 50,
		},
	}

	sandbox, err := runtime.Create(ctx, sandboxConfig)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer runtime.Destroy(ctx, sandbox.ID)

	err = runtime.Start(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("Failed to start sandbox: %v", err)
	}

	// Verify the sandbox is running
	sandbox, err = runtime.Get(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("Failed to get sandbox: %v", err)
	}

	if sandbox.Status != types.StatusRunning {
		t.Errorf("Expected status 'running', got '%s'", sandbox.Status)
	}
}

func TestDockerRuntime_WithFUSE(t *testing.T) {
	skipIfNoDocker(t)

	// Create a temporary directory for the codebase
	tempDir, err := os.MkdirTemp("", "docker-test-codebase")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello from codebase"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := DefaultConfig()
	cfg.FUSEMountBase = filepath.Join(os.TempDir(), "docker-test-fuse")
	os.MkdirAll(cfg.FUSEMountBase, 0755)
	defer os.RemoveAll(cfg.FUSEMountBase)

	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create a sandbox with codebase
	sandboxConfig := &rt.SandboxConfig{
		ID:           "test-sandbox-fuse",
		CodebaseID:   "test-codebase",
		CodebasePath: tempDir,
		Permissions: []types.PermissionRule{
			{Pattern: "**/*", Permission: types.PermRead},
		},
		Docker: &types.DockerConfig{
			Image: "alpine:latest",
		},
	}

	sandbox, err := runtime.Create(ctx, sandboxConfig)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer runtime.Destroy(ctx, sandbox.ID)

	err = runtime.Start(ctx, sandbox.ID)
	if err != nil {
		// FUSE may not be available in all environments
		t.Skipf("Failed to start sandbox (FUSE may not be available): %v", err)
	}

	// Try to read the file
	result, err := runtime.Exec(ctx, sandbox.ID, &types.ExecRequest{
		Command: "cat /workspace/test.txt",
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to exec command: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
}

func TestDockerRuntime_List(t *testing.T) {
	skipIfNoDocker(t)

	cfg := DefaultConfig()
	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runtime: %v", err)
	}
	defer runtime.Close()

	ctx := context.Background()

	// Create multiple sandboxes
	for i := 0; i < 3; i++ {
		sandboxConfig := &rt.SandboxConfig{
			ID:         "test-sandbox-list-" + string(rune('a'+i)),
			CodebaseID: "test-codebase",
			Docker: &types.DockerConfig{
				Image: "alpine:latest",
			},
		}

		_, err := runtime.Create(ctx, sandboxConfig)
		if err != nil {
			t.Fatalf("Failed to create sandbox %d: %v", i, err)
		}
	}

	// List sandboxes
	sandboxes, err := runtime.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list sandboxes: %v", err)
	}

	if len(sandboxes) != 3 {
		t.Errorf("Expected 3 sandboxes, got %d", len(sandboxes))
	}

	// Clean up
	for i := 0; i < 3; i++ {
		runtime.Destroy(ctx, "test-sandbox-list-"+string(rune('a'+i)))
	}
}
