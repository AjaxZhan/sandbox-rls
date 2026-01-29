# Sandbox RLS

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
# Create required directories
sudo mkdir -p /var/lib/sandbox/{codebases,mounts}

# Start the server (gRPC on :9000, REST on :8080)
./bin/sandbox-server -config configs/sandbox-server.yaml
```

### Try It Out

The complete workflow is: **Create Codebase → Upload Files → Create Sandbox → Start Sandbox → Execute Commands → Cleanup**

#### Option 1: Using REST API (curl)

```bash
# 1. Create a codebase (a folder to store your code)
curl -X POST http://localhost:8080/v1/codebases \
  -H "Content-Type: application/json" \
  -d '{"name": "my-project", "owner_id": "user_001"}'

# Response example:
# {"id":"cb_abc123","name":"my-project","owner_id":"user_001",...}

# 2. Create a sandbox with permission rules
# (Use the codebase_id from step 1)
curl -X POST http://localhost:8080/v1/sandboxes \
  -H "Content-Type: application/json" \
  -d '{
    "codebase_id": "cb_9bf1bb9e44dad1ed",
    "permissions": [
      {"pattern": "**/*", "permission": "PERMISSION_READ"},
      {"pattern": "/docs/**", "permission": "PERMISSION_WRITE"}
    ]
  }'

# Response example:
# {"id":"sb_xyz789","codebase_id":"cb_abc123","status":"SANDBOX_STATUS_PENDING",...}

# 3. Start the sandbox
curl -X POST http://localhost:8080/v1/sandboxes/sb_8f4e27a30c41ccb1/start

# 4. Execute a command in the sandbox
curl -X POST http://localhost:8080/v1/sandboxes/sb_8f4e27a30c41ccb1/exec \
  -H "Content-Type: application/json" \
  -d '{"command": "ls -la /workspace"}'

# Response example:
# {"stdout":"total 0\ndrwxr-xr-x 2 root root 40 ...\n","stderr":"","exit_code":0,...}

# 5. Clean up: destroy the sandbox
curl -X DELETE http://localhost:8080/v1/sandboxes/sb_8f4e27a30c41ccb1

# 6. (Optional) Delete the codebase
curl -X DELETE http://localhost:8080/v1/codebases/cb_abc123
```

#### Option 2: Using Python SDK

First, install the SDK:

```bash
cd sdk/python
pip install -e .
```

Then run this complete example that demonstrates all four permission levels:

```python
from sandbox_sdk import SandboxClient

# Connect to the server (gRPC endpoint)
client = SandboxClient(endpoint="localhost:9000")

# NOTE:
# If you see empty stdout for ALL commands while exit_code is always 0,
# your server is probably running with the "mock" runtime (it does NOT really execute commands).
# Start the server with bwrap runtime:
#   ./bin/sandbox-server -config configs/sandbox-server.yaml
# or:
#   ./bin/sandbox-server -runtime bwrap

# ==================================================
# Step 1: Create a codebase and upload test files
# ==================================================
codebase = client.create_codebase(name="permission-demo", owner_id="user_001")
print(f"Created codebase: {codebase.id}")

# Upload files for testing different permissions
test_files = {
    "public/readme.txt": b"This file is readable by everyone.",
    "docs/guide.txt": b"This file is writable.",
    "metadata/info.txt": b"This file is view-only (can see in ls, cannot read).",
    "secrets/.env": b"DB_PASSWORD=super_secret_123",
    "secrets/api_key.txt": b"sk-xxxxxxxxxxxx",
}
for path, content in test_files.items():
    client.upload_file(codebase_id=codebase.id, file_path=path, content=content)
    print(f"  Uploaded: {path}")

# ==================================================
# Step 2: Create sandbox with 4 permission levels
# ==================================================
# Permission priority: file > directory > glob (higher priority wins)
sandbox = client.create_sandbox(
    codebase_id=codebase.id,
    permissions=[
        # Default: read-only for all files
        {"pattern": "**/*", "permission": "read"},
        # /docs directory: writable
        {"pattern": "/docs/**", "permission": "write"},
        # /metadata directory: view-only (can see filename, cannot read content)
        {"pattern": "/metadata/**", "permission": "view"},
        # /secrets directory: completely hidden (invisible)
        {"pattern": "/secrets/**", "permission": "none"},
    ]
)
print(f"Created sandbox: {sandbox.id}")

client.start_sandbox(sandbox.id)
print("Sandbox started!\n")

# Helper function
def run_cmd(cmd, description):
    result = client.exec(sandbox.id, command=cmd)
    print(f"[{description}]")
    print(f"  Command: {cmd}")
    print(f"  Exit code: {result.exit_code}")
    if result.stdout.strip():
        print(f"  Stdout: {result.stdout.strip()}")
    if result.stderr.strip():
        print(f"  Stderr: {result.stderr.strip()}")
    print()
    return result

# ==================================================
# Step 3: Test "read" permission (/public)
# ==================================================
print("=" * 50)
print("TEST: 'read' permission (/public)")
print("  - Can list files: YES")
print("  - Can read content: YES")
print("  - Can write: NO")
print("=" * 50)
run_cmd("ls /workspace/public", "List /public directory")
run_cmd("cat /workspace/public/readme.txt", "Read file content")
run_cmd("echo 'modified' > /workspace/public/readme.txt", "Try to write (should fail)")

# ==================================================
# Step 4: Test "write" permission (/docs)
# ==================================================
print("=" * 50)
print("TEST: 'write' permission (/docs)")
print("  - Can list files: YES")
print("  - Can read content: YES")
print("  - Can write: YES")
print("=" * 50)
run_cmd("ls /workspace/docs", "List /docs directory")
run_cmd("cat /workspace/docs/guide.txt", "Read file content")
run_cmd("echo 'new content' > /workspace/docs/new_file.txt", "Create new file")
run_cmd("cat /workspace/docs/new_file.txt", "Verify new file")

# ==================================================
# Step 5: Test "view" permission (/metadata)
# ==================================================
print("=" * 50)
print("TEST: 'view' permission (/metadata)")
print("  - Can list files: YES")
print("  - Can read content: NO")
print("  - Can write: NO")
print("=" * 50)
run_cmd("ls /workspace/metadata", "List /metadata directory (should see files)")
run_cmd("cat /workspace/metadata/info.txt", "Try to read (should fail)")

# ==================================================
# Step 6: Test "none" permission (/secrets)
# ==================================================
print("=" * 50)
print("TEST: 'none' permission (/secrets)")
print("  - Can list files: NO (directory appears empty or missing)")
print("  - Can read content: NO")
print("  - Can write: NO")
print("=" * 50)
run_cmd("ls /workspace/secrets 2>&1 || echo 'Directory not accessible'", "List /secrets (should be hidden)")
run_cmd("cat /workspace/secrets/.env", "Try to read .env (should fail)")
run_cmd("ls /workspace", "List workspace root (secrets dir should NOT appear)")

# ==================================================
# Step 7: Clean up
# ==================================================
print("=" * 50)
client.destroy_sandbox(sandbox.id)
client.delete_codebase(codebase.id)
print("Cleanup complete!")
```

**Expected output summary:**

| Permission | `ls` (list) | `cat` (read) | `echo >` (write) |
|------------|-------------|--------------|------------------|
| `none`     | ❌ Hidden   | ❌ ENOENT    | ❌ ENOENT        |
| `view`     | ✅ Visible  | ❌ EACCES    | ❌ EACCES        |
| `read`     | ✅ Visible  | ✅ Success   | ❌ EACCES        |
| `write`    | ✅ Visible  | ✅ Success   | ✅ Success       |

#### Option 3: Using Go SDK

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    sandbox "github.com/ajaxzhan/sandbox-rls/sdk/go"
)

func main() {
    ctx := context.Background()
    client, err := sandbox.NewClient("localhost:9000")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Step 1: Create a codebase
    cb, _ := client.CreateCodebase(ctx, &sandbox.CreateCodebaseRequest{
        Name:    "my-project",
        OwnerID: "user_001",
    })
    fmt.Printf("Created codebase: %s\n", cb.ID)

    // Step 2: Create a sandbox
    sb, _ := client.CreateSandbox(ctx, &sandbox.CreateSandboxRequest{
        CodebaseID: cb.ID,
        Permissions: []sandbox.PermissionRule{
            {Pattern: "**/*", Permission: sandbox.PermRead},
            {Pattern: "/docs/**", Permission: sandbox.PermWrite},
        },
    })
    fmt.Printf("Created sandbox: %s\n", sb.ID)

    // Step 3: Start and execute
    client.StartSandbox(ctx, sb.ID)
    
    result, _ := client.Exec(ctx, sb.ID, &sandbox.ExecRequest{
        Command: "ls -la /workspace",
    })
    fmt.Printf("Output:\n%s\n", result.Stdout)

    // Step 4: Cleanup
    client.DestroySandbox(ctx, sb.ID)
    client.DeleteCodebase(ctx, cb.ID)
}
```

### Verify Permission Control

To see the permission system in action:

```bash
# Create a codebase and upload sensitive files
curl -X POST http://localhost:8080/v1/codebases \
  -d '{"name": "secure-project", "owner_id": "user_001"}'

# Create sandbox with restrictive permissions
curl -X POST http://localhost:8080/v1/sandboxes \
  -d '{
    "codebase_id": "<CODEBASE_ID>",
    "permissions": [
      {"pattern": "**/*.py", "permission": "PERMISSION_READ"},
      {"pattern": "**/secrets/**", "permission": "PERMISSION_NONE"},
      {"pattern": "/output/**", "permission": "PERMISSION_WRITE"}
    ]
  }'

# Start sandbox and test permissions
curl -X POST http://localhost:8080/v1/sandboxes/<SANDBOX_ID>/start

# Can read Python files
curl -X POST http://localhost:8080/v1/sandboxes/<SANDBOX_ID>/exec \
  -d '{"command": "cat /workspace/main.py"}'

# Cannot see secrets directory (will appear empty or hidden)
curl -X POST http://localhost:8080/v1/sandboxes/<SANDBOX_ID>/exec \
  -d '{"command": "ls /workspace/secrets"}'

# Can write to /output directory
curl -X POST http://localhost:8080/v1/sandboxes/<SANDBOX_ID>/exec \
  -d '{"command": "echo result > /workspace/output/result.txt"}'
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
