# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AgentFense is a filesystem-backed sandbox system that enforces fine-grained, path-based access control for AI agents. It lets you run untrusted code against a real codebase while enforcing least-privilege access at the file level using four permission levels: `none` (invisible), `view` (list-only), `read`, and `write`.

## Essential Commands

### Building and Running

```bash
# Generate protobuf code (required after .proto changes)
./scripts/gen-proto.sh

# Build the server
go build -o bin/agentfense-server ./cmd/agentfense-server

# Run the server (gRPC :9000, REST :8080)
./bin/agentfense-server -config configs/agentfense-server.yaml

# Run with specific runtime (overrides config)
./bin/agentfense-server -runtime bwrap
./bin/agentfense-server -runtime docker
./bin/agentfense-server -runtime mock  # For testing without isolation
```

### Testing

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/fs/...
go test ./internal/runtime/bwrap/...

# Run with verbose output
go test -v ./internal/fs/

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Integration tests
go test ./test/integration/...
```

### Python SDK

```bash
# Install Python SDK in development mode
cd sdk/python
pip install -e .

# Run Python SDK tests (if they exist)
cd sdk/python
pytest
```

## Architecture

### Four-Layer Design

1. **Client Layer**: Go SDK (TODO), Python SDK, REST API
2. **Service Layer**: gRPC server + REST gateway (grpc-gateway)
3. **Runtime Layer**: Sandbox Manager, Permission Engine, Executor
4. **Isolation Layer**: bwrap/Docker runtimes, FUSE filesystem, Delta Layer (COW)

### Core Components

**Permission System** (`internal/fs/permission.go`):
- Implements pattern-based access control with deterministic priority
- Rules sorted by: explicit Priority → PatternType (file > directory > glob) → pattern specificity
- More specific patterns always win (e.g., `/secrets/public.key` beats `/secrets/**`)

**FUSE Filesystem** (`internal/fs/fuse.go`):
- Mounts codebases as FUSE filesystems with permission enforcement
- Every file operation checked against permission rules
- Paths with `none` permission are completely invisible (don't appear in `ls`)

**Delta Layer (COW)** (`internal/fs/delta.go`):
- Provides Copy-On-Write isolation for multi-sandbox write safety
- Writes go to per-sandbox delta directories
- Reads check delta first, fallback to source
- Deletes create whiteout markers (`.wh.*` files)
- Syncs delta → source on exec() completion using Last-Writer-Wins

**Runtime Abstraction** (`internal/runtime/runtime.go`):
- Three implementations: `bwrap` (lightweight), `docker` (full isolation), `mock` (testing)
- Key interfaces: `Runtime` (lifecycle), `Executor` (commands), `SessionManager` (stateful shells)
- Sessions maintain persistent shell processes with working directory and environment state

**Server Layer** (`internal/server/server.go`):
- Combines gRPC and REST endpoints (grpc-gateway handles translation)
- Manages codebase storage and sandbox lifecycle
- Coordinates between runtime, filesystem, and permission engine

### Data Flow

**Sandbox Creation**:
1. Client creates codebase → uploads files to storage
2. Client creates sandbox with permission rules
3. Server validates rules, creates delta directory
4. Runtime creates FUSE mount with permission engine
5. Runtime starts isolation (bwrap namespace or Docker container)

**Command Execution**:
1. Client sends exec request
2. Server validates sandbox is running
3. FUSE filesystem enforces permissions on all file access
4. Runtime executes command in isolated environment
5. Delta layer captures writes to per-sandbox directory
6. On completion, changes sync to source (LWW if conflicts)

## Key Patterns

### Adding Permission Rules

Permission rules use pattern matching with three types:
- **file**: Exact match (`/config.yaml`) - highest priority
- **directory**: Prefix match (`/docs/` matches `/docs/file.txt`)
- **glob**: Wildcard match (`**/*.py`, `/secrets/**`)

Rules are automatically prioritized. You don't need to set `Priority` manually unless overriding.

```go
rules := []types.PermissionRule{
    {Pattern: "**/*", Type: types.PatternGlob, Permission: types.PermRead},        // Default: read all
    {Pattern: "/secrets/**", Type: types.PatternGlob, Permission: types.PermNone}, // Hide secrets
    {Pattern: "/docs/**", Type: types.PatternGlob, Permission: types.PermWrite},   // Writable docs
}
```

### Implementing a New Runtime

1. Implement `runtime.RuntimeWithExecutor` interface
2. Handle sandbox lifecycle: Create → Start → Exec → Stop → Destroy
3. Mount FUSE filesystem at the mount point provided in `SandboxConfig`
4. Execute commands within the isolated environment
5. Optional: Implement `SessionManager` for stateful shell support

See `internal/runtime/bwrap/bwrap.go` or `internal/runtime/docker/docker.go` as reference.

### Testing FUSE Filesystem

FUSE tests require actual mount points. The test setup:
1. Creates temporary directories for source and mount
2. Starts FUSE server in goroutine
3. Waits for mount to be ready
4. Runs test operations
5. Unmounts and cleans up

Use `MountWithReady()` to get notification when mount succeeds.

## Configuration

Server config: `configs/agentfense-server.yaml`

Key settings:
- `runtime.type`: `bwrap`, `docker`, or `mock`
- `storage.codebase_path`: Where codebases are stored (must be absolute for Docker)
- `storage.mount_path`: Base for FUSE mounts and delta directories
- `runtime.docker.enable_networking`: Control network access in sandboxes

Override via flags:
```bash
./bin/agentfense-server -runtime docker -grpc-addr :9001
```

## Important Notes

### Storage Paths
All storage paths MUST be absolute when using Docker runtime (required for bind mounts). The server automatically normalizes paths on startup via `normalizeStoragePaths()` in `cmd/agentfense-server/main.go`.

### Permission Priority
The permission system uses a sophisticated priority algorithm:
1. Explicit `Priority` field (if set)
2. Pattern type: file (3) > directory (2) > glob (1)
3. Pattern specificity: exact path > prefix match > glob

You rarely need to set Priority manually. The system auto-prioritizes based on specificity.

### Delta Layer Sync
Delta sync happens on exec() completion, not continuously. If sandbox crashes, delta changes may be lost. This is intentional - sandboxes are ephemeral.

### Mock Runtime
The mock runtime (`-runtime mock`) executes no commands and returns empty results. Use it for testing server/API logic without actual isolation. If you see exit_code=0 but empty stdout for all commands, you're probably using mock runtime.

## Common Issues

**FUSE mount errors**: Ensure `fuse` or `osxfuse` is installed. On macOS, may need to allow kernel extension.

**Permission denied in sandbox**: Check permission rules match actual paths. Paths must start with `/workspace` inside sandbox.

**Docker containers not cleaning up**: Docker runtime should auto-remove containers. Check `docker ps -a` and manually remove if needed.

**Empty output from commands**: If using mock runtime, switch to bwrap or docker for actual execution.

## Development Workflow

This project follows TDD (Test-Driven Development):
1. **RED**: Write a failing test for the feature
2. **GREEN**: Write minimal code to make test pass
3. **REFACTOR**: Improve code quality while keeping tests green

When modifying `.proto` files, always regenerate with `./scripts/gen-proto.sh` before building.
