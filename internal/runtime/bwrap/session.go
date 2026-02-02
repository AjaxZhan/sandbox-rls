package bwrap

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AjaxZhan/AgentFense/internal/logging"
	rt "github.com/AjaxZhan/AgentFense/internal/runtime"
	"github.com/AjaxZhan/AgentFense/pkg/types"
	"github.com/creack/pty"
)

// sessionState holds internal state for a session.
type sessionState struct {
	session    *types.Session
	sandboxID  string
	cmd        *exec.Cmd
	pty        *os.File // PTY master
	cancel     context.CancelFunc
	mu         sync.Mutex
	outputBuf  strings.Builder
	readyChan  chan struct{} // Signals when shell is ready
	hasOwnPgid bool          // Whether the process has its own process group (Linux only)
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "sess_" + hex.EncodeToString(bytes)
}

// CreateSession creates a new shell session within a sandbox.
func (r *BwrapRuntime) CreateSession(ctx context.Context, sandboxID string, config *types.SessionConfig) (*types.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Verify sandbox exists and is running
	state, ok := r.states[sandboxID]
	if !ok {
		return nil, types.ErrSandboxNotFound
	}
	if state.sandbox.Status != types.StatusRunning {
		return nil, types.ErrNotRunning
	}

	// Default shell
	shell := "/bin/sh"
	if config != nil && config.Shell != "" {
		shell = config.Shell
	}

	// Create session
	sessionID := generateSessionID()
	session := &types.Session{
		ID:        sessionID,
		SandboxID: sandboxID,
		Status:    types.SessionStatusActive,
		Shell:     shell,
		CreatedAt: time.Now(),
	}

	// Initialize sessions map if needed
	if r.sessions == nil {
		r.sessions = make(map[string]*sessionState)
	}

	// Create session context
	sessionCtx, cancel := context.WithCancel(context.Background())

	// Build shell command
	var cmd *exec.Cmd
	args := []string{"-i"}
	if strings.Contains(shell, "bash") {
		args = []string{"--norc", "--noprofile", "-i"}
	}
	if r.isLinux && state.fuseMountPoint != "" {
		// On Linux, use bwrap with shell
		cmd = r.buildBwrapShellCommand(sessionCtx, state.config, shell, args, state.fuseMountPoint)
	} else {
		// Compatibility mode: direct shell
		cmd = exec.CommandContext(sessionCtx, shell, args...)
		if state.fuseMountPoint != "" {
			cmd.Dir = state.fuseMountPoint
		} else if state.config.CodebasePath != "" {
			cmd.Dir = state.config.CodebasePath
		}
	}

	// Set up process group for cleanup (Linux only)
	// On macOS/Darwin, Setpgid can cause issues with PTY
	hasOwnPgid := false
	if goruntime.GOOS == "linux" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		hasOwnPgid = true
	}

	// Set initial environment variables
	cmd.Env = os.Environ()
	if config != nil {
		for k, v := range config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	// Set PS1 to a simple prompt to avoid issues
	cmd.Env = append(cmd.Env, "PS1=$ ")

	// Start PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	// Store session state
	sessState := &sessionState{
		session:    session,
		sandboxID:  sandboxID,
		cmd:        cmd,
		pty:        ptmx,
		cancel:     cancel,
		readyChan:  make(chan struct{}),
		hasOwnPgid: hasOwnPgid,
	}

	// Start output reader goroutine
	go sessState.readOutput()

	// Wait for shell to be ready
	select {
	case <-sessState.readyChan:
		// Shell is ready
	case <-time.After(2 * time.Second):
		// Timeout waiting for shell, but continue anyway
	}

	// Store PID
	if cmd.Process != nil {
		session.PID = cmd.Process.Pid
	}

	r.sessions[sessionID] = sessState

	return session, nil
}

// buildBwrapShellCommand builds a bwrap command for a persistent shell.
func (r *BwrapRuntime) buildBwrapShellCommand(ctx context.Context, config *rt.SandboxConfig, shell string, shellArgs []string, fuseMountPoint string) *exec.Cmd {
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

	// Bind the FUSE mount point to /workspace
	if fuseMountPoint != "" {
		workdir := "/workspace"
		if config.MountPoint != "" {
			workdir = config.MountPoint
		}
		args = append(args, "--bind", fuseMountPoint, workdir)
		args = append(args, "--chdir", workdir)
	}

	// Add shell with options for non-interactive initialization
	args = append(args, shell)
	args = append(args, shellArgs...)

	return exec.CommandContext(ctx, r.config.BwrapPath, args...)
}

// readOutput reads output from PTY in the background.
func (s *sessionState) readOutput() {
	reader := bufio.NewReader(s.pty)
	ready := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				// Log error but don't fail
			}
			return
		}

		// Normalize CRLF to LF (PTY uses CRLF)
		line = strings.ReplaceAll(line, "\r\n", "\n")
		line = strings.ReplaceAll(line, "\r", "")

		s.mu.Lock()
		s.outputBuf.WriteString(line)
		s.mu.Unlock()

		// Signal ready after first output
		if !ready {
			ready = true
			close(s.readyChan)
		}
	}
}

// DestroySession destroys a session and kills all its child processes.
func (r *BwrapRuntime) DestroySession(ctx context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	sessState, ok := r.sessions[sessionID]
	if !ok {
		return types.ErrSessionNotFound
	}

	// Cancel context
	if sessState.cancel != nil {
		sessState.cancel()
	}

	// Kill process group (only if we have our own process group)
	// On non-Linux systems, kill(-pgid) could kill the parent process group
	if sessState.cmd != nil && sessState.cmd.Process != nil {
		if sessState.hasOwnPgid {
			pgid, err := syscall.Getpgid(sessState.cmd.Process.Pid)
			if err == nil {
				syscall.Kill(-pgid, syscall.SIGKILL)
			} else {
				sessState.cmd.Process.Kill()
			}
		} else {
			// Only kill the child process directly
			sessState.cmd.Process.Kill()
		}
	}

	// Close PTY
	if sessState.pty != nil {
		sessState.pty.Close()
	}

	// Update session status
	sessState.session.Status = types.SessionStatusClosed
	now := time.Now()
	sessState.session.ClosedAt = &now

	// Remove from map
	delete(r.sessions, sessionID)

	return nil
}

// GetSession retrieves information about a session.
func (r *BwrapRuntime) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessState, ok := r.sessions[sessionID]
	if !ok {
		return nil, types.ErrSessionNotFound
	}

	// Return a copy
	session := *sessState.session
	return &session, nil
}

// ListSessions returns all sessions for a sandbox.
func (r *BwrapRuntime) ListSessions(ctx context.Context, sandboxID string) ([]*types.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*types.Session
	for _, sessState := range r.sessions {
		if sessState.sandboxID == sandboxID {
			session := *sessState.session
			sessions = append(sessions, &session)
		}
	}

	if sessions == nil {
		sessions = []*types.Session{}
	}

	return sessions, nil
}

// SessionExec executes a command within a session, preserving state.
func (r *BwrapRuntime) SessionExec(ctx context.Context, sessionID string, req *types.SessionExecRequest) (*types.ExecResult, error) {
	r.mu.RLock()
	sessState, ok := r.sessions[sessionID]
	if !ok {
		r.mu.RUnlock()
		return nil, types.ErrSessionNotFound
	}

	// Get sandbox state for delta sync
	sandboxState, sandboxOk := r.states[sessState.sandboxID]
	r.mu.RUnlock()

	if sessState.session.Status != types.SessionStatusActive {
		return nil, types.ErrSessionClosed
	}

	// Defer delta sync to after command execution
	defer func() {
		if sandboxOk && sandboxState.fuseFS != nil && sandboxState.fuseFS.DeltaEnabled() {
			if syncErr := sandboxState.fuseFS.Sync(); syncErr != nil {
				logging.Warn("Sync delta failed",
					logging.String("session_id", sessionID),
					logging.Err(syncErr),
				)
			}
		}
	}()

	// Generate unique marker for this command
	markerBytes := make([]byte, 8)
	rand.Read(markerBytes)
	marker := hex.EncodeToString(markerBytes)
	startMarker := fmt.Sprintf("___START_%s___", marker)
	endMarker := fmt.Sprintf("___END_%s_", marker)

	// Build command with markers
	// The end marker includes the exit code: ___END_<marker>_<exitcode>___
	wrappedCmd := fmt.Sprintf("echo '%s'; %s; echo '%s'$?'___'\n", startMarker, req.Command, endMarker)

	// Set timeout
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Clear output buffer
	sessState.mu.Lock()
	sessState.outputBuf.Reset()
	sessState.mu.Unlock()

	start := time.Now()

	// Send command to PTY
	sessState.mu.Lock()
	_, err := sessState.pty.WriteString(wrappedCmd)
	sessState.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to write command: %w", err)
	}

	// Wait for end marker with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var output string
	var exitCode int
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Send Ctrl-C to interrupt running command before returning timeout error
			sessState.mu.Lock()
			sessState.pty.Write([]byte{0x03}) // Ctrl-C / SIGINT
			sessState.mu.Unlock()
			// Give a brief moment for the interrupt to be processed
			time.Sleep(50 * time.Millisecond)
			return nil, types.ErrTimeout
		case <-ticker.C:
			sessState.mu.Lock()
			currentOutput := sessState.outputBuf.String()
			sessState.mu.Unlock()

			// Look for end marker with exit code pattern: ___END_xxx_N___
			// The full end marker is: ___END_<marker>_<exitcode>___
			// Important: Look for the marker at the START of a line to avoid matching
			// the echoed command line. The marker should appear as a standalone line.

			// Find the end marker that appears at the beginning of a line
			endMarkerWithNewline := "\n" + endMarker
			endIdx := strings.LastIndex(currentOutput, endMarkerWithNewline)
			if endIdx != -1 {
				endIdx++ // Skip the leading newline

				// Find the exit code after end marker (format: ___END_xxx_N___)
				afterEnd := currentOutput[endIdx+len(endMarker):]
				endEndIdx := strings.Index(afterEnd, "___")
				if endEndIdx != -1 {
					exitCodeStr := strings.TrimSpace(afterEnd[:endEndIdx])
					n, _ := fmt.Sscanf(exitCodeStr, "%d", &exitCode)
					if n == 0 {
						// Failed to parse exit code
						exitCode = 0
					}

					// Extract output between markers
					// Find the last occurrence of the full start marker line
					startMarkerFull := startMarker + "\n"
					startIdx := strings.LastIndex(currentOutput, startMarkerFull)
					if startIdx != -1 {
						// Content starts right after the start marker line
						contentStart := startIdx + len(startMarkerFull)

						// Content ends at the end marker
						contentEnd := endIdx

						// Trim trailing newline before end marker
						for contentEnd > contentStart && currentOutput[contentEnd-1] == '\n' {
							contentEnd--
						}

						if contentEnd > contentStart {
							output = currentOutput[contentStart:contentEnd]
						}
					}

					// Clear the processed output from buffer
					sessState.mu.Lock()
					fullEndIdx := endIdx + len(endMarker) + endEndIdx + 3
					if fullEndIdx < len(currentOutput) {
						remaining := currentOutput[fullEndIdx:]
						sessState.outputBuf.Reset()
						sessState.outputBuf.WriteString(remaining)
					} else {
						sessState.outputBuf.Reset()
					}
					sessState.mu.Unlock()

					// Clean up output: remove any terminal control sequences
					output = cleanTerminalOutput(output)

					return &types.ExecResult{
						Stdout:   output,
						ExitCode: exitCode,
						Duration: time.Since(start),
					}, nil
				}
			}
		}
	}
}

// SessionExecStream executes a command within a session and streams output.
func (r *BwrapRuntime) SessionExecStream(ctx context.Context, sessionID string, req *types.SessionExecRequest, output chan<- []byte) error {
	defer close(output)

	// For now, use SessionExec and send result as single chunk
	result, err := r.SessionExec(ctx, sessionID, req)
	if err != nil {
		return err
	}

	if result.Stdout != "" {
		output <- []byte(result.Stdout)
	}
	if result.Stderr != "" {
		output <- []byte(result.Stderr)
	}

	return nil
}

// cleanTerminalOutput removes terminal control sequences and cleans up output.
func cleanTerminalOutput(s string) string {
	// Remove common terminal control sequences
	// Remove backspace and the character before it
	for strings.Contains(s, "\b") {
		idx := strings.Index(s, "\b")
		if idx > 0 {
			s = s[:idx-1] + s[idx+1:]
		} else {
			s = s[1:]
		}
	}

	// Remove carriage returns
	s = strings.ReplaceAll(s, "\r", "")

	// Trim whitespace
	s = strings.TrimSpace(s)

	return s
}

// cleanupSessions cleans up all sessions for a sandbox.
func (r *BwrapRuntime) cleanupSessions(sandboxID string) {
	// Note: caller should hold r.mu lock
	for sessionID, sessState := range r.sessions {
		if sessState.sandboxID == sandboxID {
			// Cancel and kill
			if sessState.cancel != nil {
				sessState.cancel()
			}
			if sessState.cmd != nil && sessState.cmd.Process != nil {
				// Only use kill(-pgid) if we have our own process group
				if sessState.hasOwnPgid {
					pgid, err := syscall.Getpgid(sessState.cmd.Process.Pid)
					if err == nil {
						syscall.Kill(-pgid, syscall.SIGKILL)
					} else {
						sessState.cmd.Process.Kill()
					}
				} else {
					sessState.cmd.Process.Kill()
				}
			}
			if sessState.pty != nil {
				sessState.pty.Close()
			}
			delete(r.sessions, sessionID)
		}
	}
}
