# Sandbox RLS (Row-Level Security)

A sandbox infrastructure service for AI Agents with fine-grained file permission control.

## Features

- **Fine-grained Permission Control**: Support none/view/read/write four-level permissions with glob patterns, directory-level, and file-level priority override
- **Lightweight Isolation**: Based on bubblewrap (bwrap) for millisecond-level sandbox startup
- **Runtime Abstraction**: Extensible architecture supporting future migration to Docker/gVisor
- **Multi-Sandbox Codebase Sharing**: Same folder can be accessed by multiple Agents with different permissions

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Client Layer                           │
│              Go SDK / Python SDK / CLI                      │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                     Service Layer                           │
│          gRPC Server + REST Gateway (grpc-gateway)          │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                     Runtime Layer                           │
│    Sandbox Manager │ Permission Engine │ Executor           │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                   Isolation Layer                           │
│              bwrap Runtime │ FUSE FileSystem                │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- bubblewrap (`bwrap`) installed
- protoc (for development)

### Installation

```bash
# Clone the repository
git clone https://github.com/ajaxzhan/sandbox-rls.git
cd sandbox-rls

# Install dependencies
go mod tidy

# Generate proto code (for development)
./scripts/gen-proto.sh

# Build
go build -o bin/sandbox-server ./cmd/sandbox-server
```

### Running the Server

```bash
./bin/sandbox-server -config configs/sandbox-server.yaml
```

### Using the SDK

#### Go SDK

```go
import "github.com/ajaxzhan/sandbox-rls/sdk/go"

client, _ := sandbox.NewClient("localhost:9000")

// Create a sandbox
sb, _ := client.CreateSandbox(ctx, &sandbox.CreateSandboxRequest{
    CodebaseID: "cb_123",
    Permissions: []sandbox.PermissionRule{
        {Pattern: "/docs/**", Permission: sandbox.PermWrite},
        {Pattern: "**/*.py", Permission: sandbox.PermRead},
    },
})

// Start the sandbox
client.StartSandbox(ctx, sb.ID)

// Execute command
result, _ := client.Exec(ctx, sb.ID, &sandbox.ExecRequest{
    Command: "ls -la /workspace",
})
fmt.Println(result.Stdout)
```

#### Python SDK

```python
from sandbox_sdk import SandboxClient

client = SandboxClient(endpoint="localhost:9000")

sandbox = client.create_sandbox(
    codebase_id="cb_123",
    permissions=[
        {"pattern": "/docs/**", "permission": "write"},
        {"pattern": "**/*.py", "permission": "read"},
    ]
)

client.start_sandbox(sandbox.id)

result = client.exec(sandbox.id, command="ls -la /workspace")
print(result.stdout)
```

## Project Structure

```
sandbox-rls/
├── api/proto/           # Protobuf definitions
├── cmd/                 # Entry points
│   ├── sandbox-server/  # Main server
│   └── sandbox-cli/     # CLI tool
├── internal/
│   ├── server/          # gRPC + REST server
│   ├── sandbox/         # Core business logic
│   ├── runtime/         # Runtime abstraction layer
│   │   ├── bwrap/       # bubblewrap implementation
│   │   └── docker/      # Docker implementation (reserved)
│   ├── fs/              # FUSE filesystem
│   └── codebase/        # Codebase management
├── pkg/types/           # Public types
├── sdk/
│   ├── go/              # Go SDK
│   └── python/          # Python SDK
├── test/
│   ├── integration/     # Integration tests
│   └── e2e/             # End-to-end tests
└── configs/             # Configuration examples
```

## Development

### Running Tests

```bash
# Unit tests
go test ./...

# Integration tests
go test ./test/integration/...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### TDD Development Flow

This project follows TDD (Test-Driven Development):

1. **RED**: Write a failing test
2. **GREEN**: Write minimal code to pass
3. **REFACTOR**: Improve code quality

## Permission Model

Four permission levels:

| Level | Description |
|-------|-------------|
| `none` | Completely invisible, not shown in `ls` |
| `view` | Visible in `ls`, but cannot read content |
| `read` | Can read file content |
| `write` | Can modify file |

Priority: File-level > Directory-level > Glob pattern

## License

MIT
