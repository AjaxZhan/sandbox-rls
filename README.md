# Sandbox RLS

> Sandbox with fine-grained file access control for AI Agents

Sandbox RLS lets you run untrusted AI agent code **against a real codebase** while enforcing **path-based, least-privilege access** at the file level.

## Why it matters

The best “agent interface” keeps winning: **a bash-capable agent + a filesystem**. It’s simple, universal, and already supported by every toolchain we use.

FUSE show why: once you can *mount* a world, you can make an agent productive with plain old `ls`, `cat`, `grep`, and `find`. But there’s a missing layer of infrastructure: the filesystem is usually **all-or-nothing**. If you mount a real repo, you often mount *everything*.

Sandbox RLS fills that gap: a filesystem-backed sandbox that can make paths **writable**, **read-only**, **list-only**, or completely **invisible**—so you can give an agent a real codebase without giving it full trust.

## Why this exists (the pain)

When you give an agent a repository, you usually end up with one of these bad choices:

- **All-or-nothing access**: the agent can read everything (including secrets) if it needs to edit docs or run tests.
- **Coarse container isolation**: the process is isolated, but the mounted workspace is still basically “full trust”.
- **Brittle ad-hoc filtering**: you try to strip files out of prompts, but the agent can still read them at runtime.

Sandbox RLS is built for the missing middle ground: **“You can edit `/docs`, you can *see* `/metadata`, you can read everything else, and `/secrets` does not exist.”**

## What you get

- **Four permission levels**: `none` / `view` / `read` / `write`
- **Glob + prefix patterns** with deterministic priority (more specific rules win)
- **Multi-agent sharing**: multiple sandboxes can mount the same codebase with different rules
- **Fast startup**: lightweight isolation based on bubblewrap (`bwrap`)

## 30-second example (Python SDK)

Give an agent a real repo with one line of code - using built-in permission presets that automatically hide secrets:

```python
from sandbox_rls import Sandbox

# One-liner: create sandbox from local directory with "agent-safe" preset
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)
# Automatically cleans up when done
```

The `agent-safe` preset provides sensible defaults: read all files, write to /output and /tmp, hide secrets (.env, *.key, etc.).

For fine-grained control, customize permissions:

```python
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
```

## Permission model (at a glance)

| Level | What the agent can do |
|-------|------------------------|
| `none`  | Path is **invisible** (not shown in `ls`, behaves like it doesn’t exist) |
| `view`  | Can **list** names (e.g. `ls`), but cannot read file content |
| `read`  | Can read file content |
| `write` | Can read + modify / create files |

Rule priority: **more specific wins** (file-level > directory-level > glob).

## Features

- **Fine-grained permission control**: `none` / `view` / `read` / `write` with glob patterns and deterministic priority rules
- **Lightweight isolation**: bubblewrap (`bwrap`) runtime for fast sandbox startup
- **Docker runtime**: optional Docker-based isolation with custom image support and resource limits
- **Delta Layer (COW)**: Copy-On-Write isolation prevents concurrent write conflicts; changes are atomically synced on exec completion
- **Stateful sessions**: persistent shell sessions that maintain working directory and environment variables
- **Resource limits**: memory, CPU, and process count limits (via Docker runtime)
- **Multi-sandbox codebase sharing**: same folder can be mounted by multiple agents with different permissions
- **Permission presets**: built-in presets like `agent-safe` that hide secrets automatically

## Platform Notes

### macOS + Docker Desktop Limitation

When using the **Docker runtime on macOS**, the `view` permission level may not work correctly. Files with `view` permission will appear as "No such file or directory" inside the container, even though the underlying FUSE implementation works correctly.

**Root cause**: Docker Desktop on macOS uses VirtioFS to share files between the host and the Linux VM. When binding a FUSE mount point into a container, VirtioFS doesn't correctly propagate certain FUSE responses for `view`-permission files.

| Permission | Linux (Native Docker) | macOS (Docker Desktop) |
|------------|----------------------|------------------------|
| `none`     | ✅ Works              | ✅ Works                |
| `view`     | ✅ Works              | ❌ Files appear missing |
| `read`     | ✅ Works              | ✅ Works                |
| `write`    | ✅ Works              | ✅ Works                |

**Workarounds**:
- **Recommended**: Run the Docker runtime on Linux for full `view` permission support
- Use `read` permission instead of `view` if you only need to prevent writes
- Use the `bwrap` runtime on Linux (not available on macOS)

This limitation only affects macOS + Docker Desktop. The `bwrap` runtime on Linux and native Docker on Linux work correctly with all permission levels.

## Comparison

The table below focuses on the agent-centric question: **can you safely run untrusted code against a real codebase with least-privilege filesystem access?**

| Capability | **Sandbox-RLS** | **E2B** | **Anthropic `sandbox-runtime` (`srt`)** | **Docker** | **Other sandboxes** |
|---|---|---|---|---|---|
| Primary focus | Anything as FS + path RLS | Hosted execution sandbox | OS-level process sandbox | Container isolation | Stronger isolation boundary |
| Real repo as a mounted workspace | ✅ | ⚠️ (sandbox FS, not “codebase view”) | ⚠️ (wrap commands, not a codebase layer) | ✅ | ⚠️ varies |
| Path-based least privilege | ✅ (glob + priority) | ❌ | ✅ (allow/deny) | ⚠️ (coarse by default) | ⚠️ varies |
| Hidden paths (`none`, not visible in `ls`) | ✅ | ❌ | ⚠️ (blocked, not “invisible”) | ❌ | ⚠️ varies |
| List-only paths (`view`) | ✅ | ❌ | ❌ | ❌ | ⚠️ varies |
| Share one codebase across multiple sandboxes | ✅ | ⚠️ | ⚠️ | ⚠️ | ⚠️ varies |
| Network restrictions | ⚠️ (not the core feature) | ✅ | ✅ | ✅ (needs config) | ✅ (needs config) |

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the detailed development plan.

**Completed:**

| Feature | Status |
|---------|--------|
| Session support | ✅ Stateful shell sessions with working directory and env var persistence |
| Docker runtime | ✅ Full Docker isolation with custom image support |
| Resource limits | ✅ Memory, CPU, and process count limits |
| Delta Layer (COW) | ✅ Copy-On-Write isolation for multi-sandbox write safety |
| One-liner API | ✅ `Sandbox.from_local()` for instant sandbox creation |
| Permission presets | ✅ Built-in presets: agent-safe, read-only, full-access, etc. |
| Error handling | ✅ Semantic exceptions: CommandTimeoutError, PermissionDeniedError, etc. |

**Current priorities:**

| Phase | Focus | Key Features |
|-------|-------|--------------|
| **Phase 2** | Developer Experience | CLI tool, Go SDK, configuration files |
| **Phase 3** | Multi-agent | File locking, agent communication, external data sources |

**What we're NOT building** (out of scope):
- MicroVM isolation (Firecracker) - overkill for individual developers
- Hibernate/wake (CRIU) - too complex, short-lived sandboxes don't need it
- Million-scale concurrency - we're a single-node solution

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
│  bwrap Runtime │ Docker Runtime │ FUSE FS │ Delta Layer     │
└─────────────────────────────────────────────────────────────┘
```

### Delta Layer (COW)

When multiple sandboxes share the same codebase, the Delta Layer provides write isolation:

```
Sandbox A                    Sandbox B
    │                            │
    ▼                            ▼
┌─────────┐                ┌─────────┐
│ Delta A │                │ Delta B │  ← Writes go here (COW)
└────┬────┘                └────┬────┘
     │                          │
     └──────────┬───────────────┘
                ▼
        ┌──────────────┐
        │Shared Storage│  ← Reads fallback here
        │  (Codebase)  │  ← Sync on exec() return
        └──────────────┘
```

- **Read**: Delta first, fallback to source
- **Write**: Copy-On-Write to Delta directory
- **Delete**: Whiteout markers in Delta
- **Sync**: LWW (Last-Writer-Wins) on exec() completion

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
# Start the server (gRPC on :9000, REST on :8080)
./bin/sandbox-server -config test-config.yaml
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

First, install the SDK in a virtual environment (recommended to avoid system Python conflicts):

```bash
# Create and activate a virtual environment
python3 -m venv .venv
source .venv/bin/activate  # On Windows: .venv\Scripts\activate

# Install the SDK in development mode
pip install -e sdk/python/
```

> **Note**: On modern Debian/Ubuntu systems (Python 3.12+), installing packages with `pip` outside a virtual environment will fail with "externally-managed-environment" error. Using a virtual environment is the recommended approach.

##### Quick Start (High-Level API)

The easiest way to use the SDK:

```python
from sandbox_rls import Sandbox, RuntimeType, ResourceLimits

# One-liner: create sandbox from local directory
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)

# With Docker runtime and resource limits
with Sandbox.from_local(
    "./my-project",
    preset="agent-safe",  # Built-in preset that hides secrets
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB
        pids_limit=100,
    ),
) as sandbox:
    # Use sessions for stateful commands
    with sandbox.session() as session:
        session.exec("cd /workspace")
        session.exec("pip install -r requirements.txt")
        result = session.exec("pytest")
        print(result.stdout)
```

##### Permission Presets

Built-in presets for common scenarios:

| Preset | Description |
|--------|-------------|
| `agent-safe` | Read all, write to /output & /tmp, hide secrets (.env, *.key, etc.) |
| `read-only` | Read all files, no write access |
| `full-access` | Full read/write access |
| `development` | Full access except secrets |

```python
from sandbox_rls import list_presets, extend_preset

# List available presets
print(list_presets())  # ['agent-safe', 'development', 'full-access', 'read-only', 'view-only']

# Extend a preset with custom rules
rules = extend_preset(
    "agent-safe",
    additions=[{"pattern": "/custom/**", "permission": "write"}],
)
```

##### Error Handling

```python
from sandbox_rls import Sandbox, SandboxError, CommandTimeoutError, CommandExecutionError

try:
    with Sandbox.from_local("./project") as sandbox:
        result = sandbox.run("python main.py", timeout=30, raise_on_error=True)
except CommandTimeoutError as e:
    print(f"Command timed out: {e}")
except CommandExecutionError as e:
    print(f"Command failed (exit {e.exit_code}): {e.stderr}")
except SandboxError as e:
    print(f"Sandbox error: {e}")
```

##### Complete Example (Low-Level API)

For fine-grained control, use `SandboxClient` directly. This example demonstrates all four permission levels:

```python
from sandbox_rls import SandboxClient, RuntimeType, ResourceLimits

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
    ],
    # Optional: use Docker runtime with resource limits
    runtime=RuntimeType.BWRAP,  # or RuntimeType.DOCKER
    # resources=ResourceLimits(memory_bytes=256*1024*1024),
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

#### [TODO] Option 3: Using Go SDK 

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

> Note: the REST API uses enum-style values like `PERMISSION_READ`, while the SDKs use string values like `read` / `write` / `view` / `none`.

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
│   │   ├── bwrap/       # bubblewrap implementation (with session support)
│   │   ├── docker/      # Docker implementation (with session & resource limits)
│   │   └── mock/        # Mock runtime for testing
│   ├── fs/              # FUSE filesystem + Delta Layer (COW)
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

## References

- [FUSE is All You Need - Giving agents access to anything via filesystems](https://jakobemmerling.de/posts/fuse-is-all-you-need/)
- [Tursodatabase's Agentfs](https://github.com/tursodatabase/agentfs)
- [Anthropic's Agent Skill](https://github.com/anthropics/skills)

## License

MIT
