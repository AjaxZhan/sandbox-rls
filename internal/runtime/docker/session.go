package docker

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	rt "github.com/AjaxZhan/AgentFense/internal/runtime"
	"github.com/AjaxZhan/AgentFense/pkg/types"
	"github.com/docker/docker/api/types/container"
)

// sessionState holds internal state for a session.
type sessionState struct {
	session     *types.Session
	sandboxID   string
	containerID string
	execID      string             // Docker exec instance ID
	conn        io.ReadWriteCloser // Hijacked connection for PTY
	cancel      context.CancelFunc
	mu          sync.Mutex
	outputBuf   strings.Builder
	readyChan   chan struct{} // Signals when shell is ready
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails (extremely rare)
		return fmt.Sprintf("sess_%x", time.Now().UnixNano())
	}
	return "sess_" + hex.EncodeToString(b)
}

// generateMarker generates a unique marker string for command output parsing.
func generateMarker() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based marker if crypto/rand fails (extremely rare)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// CreateSession creates a new shell session within a sandbox.
func (r *DockerRuntime) CreateSession(ctx context.Context, sandboxID string, config *types.SessionConfig) (*types.Session, error) {
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

	// Create session context
	sessionCtx, cancel := context.WithCancel(context.Background())

	// Build exec configuration for a persistent shell
	// IMPORTANT: alpine images often don't have bash, and /bin/sh doesn't support bash-only flags.
	args := []string{"-i"}
	if strings.Contains(shell, "bash") {
		args = []string{"--norc", "--noprofile", "-i"}
	}
	execConfig := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          append([]string{shell}, args...),
	}

	// Set initial environment variables
	execConfig.Env = []string{"PS1=$ ", "TERM=xterm"}
	if config != nil {
		for k, v := range config.Env {
			execConfig.Env = append(execConfig.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Create exec instance
	execResp, err := r.client.ContainerExecCreate(ctx, state.containerID, execConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec with TTY
	attachResp, err := r.client.ContainerExecAttach(sessionCtx, execResp.ID, container.ExecAttachOptions{
		Tty: true,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}

	// Store session state
	sessState := &sessionState{
		session:     session,
		sandboxID:   sandboxID,
		containerID: state.containerID,
		execID:      execResp.ID,
		conn:        attachResp.Conn,
		cancel:      cancel,
		readyChan:   make(chan struct{}),
	}

	// Start output reader goroutine
	go sessState.readOutput(attachResp.Reader)

	// Wait for shell to be ready
	select {
	case <-sessState.readyChan:
		// Shell is ready
	case <-time.After(2 * time.Second):
		// Timeout waiting for shell, but continue anyway
	}

	r.sessions[sessionID] = sessState

	return session, nil
}

// readOutput reads output from the exec connection in the background.
func (s *sessionState) readOutput(reader io.Reader) {
	bufReader := bufio.NewReader(reader)
	ready := false

	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				// Log error but don't fail
			}
			return
		}

		// Normalize CRLF to LF
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
func (r *DockerRuntime) DestroySession(ctx context.Context, sessionID string) error {
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

	// Close connection
	if sessState.conn != nil {
		sessState.conn.Close()
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
func (r *DockerRuntime) GetSession(ctx context.Context, sessionID string) (*types.Session, error) {
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
func (r *DockerRuntime) ListSessions(ctx context.Context, sandboxID string) ([]*types.Session, error) {
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
func (r *DockerRuntime) SessionExec(ctx context.Context, sessionID string, req *types.SessionExecRequest) (*types.ExecResult, error) {
	r.mu.RLock()
	sessState, ok := r.sessions[sessionID]
	if !ok {
		r.mu.RUnlock()
		return nil, types.ErrSessionNotFound
	}
	r.mu.RUnlock()

	if sessState.session.Status != types.SessionStatusActive {
		return nil, types.ErrSessionClosed
	}

	// Generate unique marker for this command
	marker := generateMarker()
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

	// Send command to connection
	sessState.mu.Lock()
	_, err := sessState.conn.Write([]byte(wrappedCmd))
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
			sessState.conn.Write([]byte{0x03}) // Ctrl-C / SIGINT
			sessState.mu.Unlock()
			// Give a brief moment for the interrupt to be processed
			time.Sleep(50 * time.Millisecond)
			return nil, types.ErrTimeout
		case <-ticker.C:
			sessState.mu.Lock()
			currentOutput := sessState.outputBuf.String()
			sessState.mu.Unlock()

			// Look for end marker with exit code pattern
			endMarkerWithNewline := "\n" + endMarker
			endIdx := strings.LastIndex(currentOutput, endMarkerWithNewline)
			if endIdx != -1 {
				endIdx++ // Skip the leading newline

				// Find the exit code after end marker
				afterEnd := currentOutput[endIdx+len(endMarker):]
				endEndIdx := strings.Index(afterEnd, "___")
				if endEndIdx != -1 {
					exitCodeStr := strings.TrimSpace(afterEnd[:endEndIdx])
					n, _ := fmt.Sscanf(exitCodeStr, "%d", &exitCode)
					if n == 0 {
						exitCode = 0
					}

					// Extract output between markers
					startMarkerFull := startMarker + "\n"
					startIdx := strings.LastIndex(currentOutput, startMarkerFull)
					if startIdx != -1 {
						contentStart := startIdx + len(startMarkerFull)
						contentEnd := endIdx

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

					// Clean up output
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
func (r *DockerRuntime) SessionExecStream(ctx context.Context, sessionID string, req *types.SessionExecRequest, output chan<- []byte) error {
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

// Verify interface compliance at compile time
var _ rt.SessionManager = (*DockerRuntime)(nil)
