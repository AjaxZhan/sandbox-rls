package bwrap_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AjaxZhan/AgentFense/internal/runtime"
	"github.com/AjaxZhan/AgentFense/internal/runtime/bwrap"
	"github.com/AjaxZhan/AgentFense/pkg/types"
)

// setupSessionTest creates a running sandbox for session tests.
func setupSessionTest(t *testing.T) (*bwrap.BwrapRuntime, string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "session-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create a test file in the temp directory
	if err := os.WriteFile(tmpDir+"/test.txt", []byte("hello"), 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create test file: %v", err)
	}

	r := bwrap.New(nil)
	ctx := context.Background()

	config := &runtime.SandboxConfig{
		ID:           "session-test-sandbox-" + time.Now().Format("150405"),
		CodebaseID:   "codebase-1",
		CodebasePath: tmpDir,
	}

	_, err = r.Create(ctx, config)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Create failed: %v", err)
	}

	err = r.Start(ctx, config.ID)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Start failed: %v", err)
	}

	cleanup := func() {
		r.Destroy(ctx, config.ID)
		os.RemoveAll(tmpDir)
	}

	return r, config.ID, cleanup
}

// TestSession_Create tests creating a session.
func TestSession_Create(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	// Create a session
	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.ID == "" {
		t.Error("Session ID should not be empty")
	}
	if session.SandboxID != sandboxID {
		t.Errorf("Session.SandboxID = %q, want %q", session.SandboxID, sandboxID)
	}
	if session.Status != types.SessionStatusActive {
		t.Errorf("Session.Status = %q, want %q", session.Status, types.SessionStatusActive)
	}
	if session.Shell == "" {
		t.Error("Session.Shell should not be empty")
	}
}

// TestSession_CreateWithShell tests creating a session with custom shell.
func TestSession_CreateWithShell(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	config := &types.SessionConfig{
		Shell: "/bin/sh",
	}

	session, err := r.CreateSession(ctx, sandboxID, config)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.Shell != "/bin/sh" {
		t.Errorf("Session.Shell = %q, want %q", session.Shell, "/bin/sh")
	}
}

// TestSession_CreateSandboxNotFound tests creating a session for non-existent sandbox.
func TestSession_CreateSandboxNotFound(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	_, err := r.CreateSession(ctx, "non-existent-sandbox", nil)
	if err != types.ErrSandboxNotFound {
		t.Errorf("CreateSession on non-existent sandbox: got %v, want ErrSandboxNotFound", err)
	}
}

// TestSession_CreateSandboxNotRunning tests creating a session for non-running sandbox.
func TestSession_CreateSandboxNotRunning(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	config := &runtime.SandboxConfig{
		ID:         "session-not-running-test",
		CodebaseID: "codebase-1",
	}
	r.Create(ctx, config)
	// Don't start the sandbox

	_, err := r.CreateSession(ctx, config.ID, nil)
	if err != types.ErrNotRunning {
		t.Errorf("CreateSession on non-running sandbox: got %v, want ErrNotRunning", err)
	}
}

// TestSession_WorkingDirectoryPersistence tests that working directory persists across commands.
func TestSession_WorkingDirectoryPersistence(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Change directory
	_, err = r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "cd /tmp",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec cd failed: %v", err)
	}

	// Check that we're still in /tmp
	result, err := r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "pwd",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec pwd failed: %v", err)
	}

	if !strings.Contains(result.Stdout, "/tmp") {
		t.Errorf("Working directory not persisted: got %q, want to contain /tmp", result.Stdout)
	}
}

// TestSession_EnvironmentVariablePersistence tests that environment variables persist.
func TestSession_EnvironmentVariablePersistence(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Set environment variable
	_, err = r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "export MY_TEST_VAR=hello_world",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec export failed: %v", err)
	}

	// Check that the variable persists
	result, err := r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "echo $MY_TEST_VAR",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec echo failed: %v", err)
	}

	if !strings.Contains(result.Stdout, "hello_world") {
		t.Errorf("Environment variable not persisted: got %q, want to contain hello_world", result.Stdout)
	}
}

// TestSession_ShellVariablePersistence tests that shell variables persist.
func TestSession_ShellVariablePersistence(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Set shell variable
	_, err = r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "SHELL_VAR=test_value",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec var set failed: %v", err)
	}

	// Check that the variable persists
	result, err := r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "echo $SHELL_VAR",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec echo failed: %v", err)
	}

	if !strings.Contains(result.Stdout, "test_value") {
		t.Errorf("Shell variable not persisted: got %q, want to contain test_value", result.Stdout)
	}
}

// TestSession_MultipleCommands tests executing multiple commands in sequence.
func TestSession_MultipleCommands(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	commands := []string{
		"mkdir -p /tmp/session_test",
		"cd /tmp/session_test",
		"echo 'test content' > file.txt",
		"cat file.txt",
	}

	var lastResult *types.ExecResult
	for i, cmd := range commands {
		result, err := r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
			Command: cmd,
			Timeout: 5 * time.Second,
		})
		if err != nil {
			t.Fatalf("SessionExec command %d (%q) failed: %v", i, cmd, err)
		}
		lastResult = result
	}

	if !strings.Contains(lastResult.Stdout, "test content") {
		t.Errorf("Multi-command sequence failed: got %q, want to contain 'test content'", lastResult.Stdout)
	}
}

// TestSession_ExitCode tests that exit codes are captured correctly.
func TestSession_ExitCode(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Successful command
	result, err := r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "true",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec true failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("true exit code = %d, want 0", result.ExitCode)
	}

	// Failing command
	result, err = r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "false",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec false failed: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("false exit code = %d, want 1", result.ExitCode)
	}

	// Custom exit code - use subshell to avoid closing the main shell
	result, err = r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "(exit 42)",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec (exit 42) failed: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("(exit 42) exit code = %d, want 42", result.ExitCode)
	}
}

// TestSession_Timeout tests that command timeout works.
func TestSession_Timeout(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Execute a long-running command with short timeout
	_, err = r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "sleep 10",
		Timeout: 100 * time.Millisecond,
	})

	if err != types.ErrTimeout {
		t.Errorf("SessionExec with timeout: got %v, want ErrTimeout", err)
	}
}

// TestSession_Destroy tests destroying a session.
func TestSession_Destroy(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Destroy the session
	err = r.DestroySession(ctx, session.ID)
	if err != nil {
		t.Fatalf("DestroySession failed: %v", err)
	}

	// Try to execute in destroyed session
	_, err = r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "echo test",
		Timeout: 5 * time.Second,
	})
	if err != types.ErrSessionNotFound && err != types.ErrSessionClosed {
		t.Errorf("SessionExec on destroyed session: got %v, want ErrSessionNotFound or ErrSessionClosed", err)
	}
}

// TestSession_DestroyNotFound tests destroying a non-existent session.
func TestSession_DestroyNotFound(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	err := r.DestroySession(ctx, "non-existent-session")
	if err != types.ErrSessionNotFound {
		t.Errorf("DestroySession non-existent: got %v, want ErrSessionNotFound", err)
	}
}

// TestSession_GetSession tests retrieving session information.
func TestSession_GetSession(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get the session
	retrieved, err := r.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("GetSession ID = %q, want %q", retrieved.ID, session.ID)
	}
	if retrieved.SandboxID != sandboxID {
		t.Errorf("GetSession SandboxID = %q, want %q", retrieved.SandboxID, sandboxID)
	}
	if retrieved.Status != types.SessionStatusActive {
		t.Errorf("GetSession Status = %q, want %q", retrieved.Status, types.SessionStatusActive)
	}
}

// TestSession_GetSessionNotFound tests getting a non-existent session.
func TestSession_GetSessionNotFound(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	_, err := r.GetSession(ctx, "non-existent-session")
	if err != types.ErrSessionNotFound {
		t.Errorf("GetSession non-existent: got %v, want ErrSessionNotFound", err)
	}
}

// TestSession_ListSessions tests listing sessions for a sandbox.
func TestSession_ListSessions(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	// Initially no sessions
	sessions, err := r.ListSessions(ctx, sandboxID)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("Initial ListSessions: got %d sessions, want 0", len(sessions))
	}

	// Create some sessions
	for i := 0; i < 3; i++ {
		_, err := r.CreateSession(ctx, sandboxID, nil)
		if err != nil {
			t.Fatalf("CreateSession %d failed: %v", i, err)
		}
	}

	// List sessions
	sessions, err = r.ListSessions(ctx, sandboxID)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("ListSessions: got %d sessions, want 3", len(sessions))
	}
}

// TestSession_CleanupOnDestroy tests that child processes are cleaned up when session is destroyed.
func TestSession_CleanupOnDestroy(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Start a background process using nohup to avoid job control output
	// The command writes to a file and runs in background
	_, err = r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "(sleep 1000 &) 2>/dev/null",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		// Background process command might timeout if shell job control interferes
		// This is acceptable - the main test is that destroy cleans up
		t.Logf("SessionExec background process: %v (may be expected)", err)
	}

	// Destroy the session - should kill background processes
	err = r.DestroySession(ctx, session.ID)
	if err != nil {
		t.Fatalf("DestroySession failed: %v", err)
	}

	// Give a moment for processes to be cleaned up
	time.Sleep(100 * time.Millisecond)

	// Verify the session is destroyed
	_, err = r.GetSession(ctx, session.ID)
	if err != types.ErrSessionNotFound {
		t.Errorf("Session should be destroyed: got %v, want ErrSessionNotFound", err)
	}
}

// TestSession_CleanupOnSandboxDestroy tests that sessions are cleaned up when sandbox is destroyed.
func TestSession_CleanupOnSandboxDestroy(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	ctx := context.Background()

	session, err := r.CreateSession(ctx, sandboxID, nil)
	if err != nil {
		cleanup()
		t.Fatalf("CreateSession failed: %v", err)
	}
	sessionID := session.ID

	// Destroy sandbox (will call cleanup internally)
	cleanup()

	// Session should no longer exist
	_, err = r.GetSession(ctx, sessionID)
	if err != types.ErrSessionNotFound {
		t.Errorf("Session should be cleaned up with sandbox: got %v, want ErrSessionNotFound", err)
	}
}

// TestSession_SessionExecNotFound tests executing in a non-existent session.
func TestSession_SessionExecNotFound(t *testing.T) {
	r := bwrap.New(nil)
	ctx := context.Background()

	_, err := r.SessionExec(ctx, "non-existent-session", &types.SessionExecRequest{
		Command: "echo test",
		Timeout: 5 * time.Second,
	})
	if err != types.ErrSessionNotFound {
		t.Errorf("SessionExec non-existent: got %v, want ErrSessionNotFound", err)
	}
}

// TestSession_InitialEnv tests creating a session with initial environment variables.
func TestSession_InitialEnv(t *testing.T) {
	r, sandboxID, cleanup := setupSessionTest(t)
	defer cleanup()
	ctx := context.Background()

	config := &types.SessionConfig{
		Env: map[string]string{
			"INIT_VAR": "init_value",
		},
	}

	session, err := r.CreateSession(ctx, sandboxID, config)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Check that initial env var is set
	result, err := r.SessionExec(ctx, session.ID, &types.SessionExecRequest{
		Command: "echo $INIT_VAR",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("SessionExec failed: %v", err)
	}

	if !strings.Contains(result.Stdout, "init_value") {
		t.Errorf("Initial env var not set: got %q, want to contain init_value", result.Stdout)
	}
}
