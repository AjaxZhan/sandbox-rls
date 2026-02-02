# Sandbox RLS

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![PyPI](https://img.shields.io/pypi/v/sandbox-rls?logo=pypi&logoColor=white)](https://pypi.org/project/sandbox-rls/)
[![Release](https://img.shields.io/github/v/release/ajaxzhan/sandbox-rls?logo=github)](https://github.com/ajaxzhan/sandbox-rls/releases)
[![FUSE](https://img.shields.io/badge/FUSE-filesystem-orange)](https://github.com/libfuse/libfuse)

> Filesystem sandbox with fine-grained, path-based access control for AI agents

Run untrusted AI agent code **against a real codebase** while enforcing **least-privilege access** at the file level.

## Motivation

The best agent interface remains simple: **bash + filesystem**. With FUSE, you can mount any world and make an agent productive with plain `ls`, `cat`, `grep`, and `find`.

But there's a gap: filesystems are usually **all-or-nothing**. Mount a real repo, and you often expose *everything*—including secrets.

Sandbox RLS fills that gap with four permission levels:

| Level | What the agent can do |
|-------|------------------------|
| `none`  | Path is **invisible** (hidden from `ls`, behaves like it doesn't exist) |
| `view`  | Can **list** names (`ls`), but cannot read file content |
| `read`  | Can read file content |
| `write` | Can read + modify / create files |

**Example policy**: *"You can edit `/docs`, see `/metadata`, read everything else, but `/secrets` does not exist."*

## Quick Start

```python
from sandbox_rls import Sandbox

# One-liner: create sandbox from local directory with "agent-safe" preset
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)
```

The `agent-safe` preset: read all files, write to `/output` and `/tmp`, hide secrets (`.env`, `*.key`, etc.).

For custom permissions:

```python
sandbox = client.create_sandbox(
    codebase_id=codebase.id,
    permissions=[
        {"pattern": "**/*", "permission": "read"},           # Default: read-only
        {"pattern": "/docs/**", "permission": "write"},      # Writable
        {"pattern": "/metadata/**", "permission": "view"},   # List-only
        {"pattern": "/secrets/**", "permission": "none"},    # Hidden
    ]
)
```

## AI Agent Example

Build secure AI agents that execute bash commands with permission control:

```python
from anthropic import Anthropic
from sandbox_rls import Sandbox

# Define what the agent can access
PERMISSIONS = [
    {"pattern": "**/*", "permission": "read"},      # Read all by default
    {"pattern": "output/*", "permission": "write"}, # Can write to output/
    {"pattern": ".env", "permission": "none"},      # Hide secrets
]

client = Anthropic()

with Sandbox.from_local("./project", permissions=PERMISSIONS) as sandbox:
    # Agent generates bash command
    response = client.messages.create(
        model="claude-sonnet-4-20250514",
        messages=[{"role": "user", "content": "List all Python files"}],
        system="Output bash commands in ```bash``` blocks."
    )
    
    # Execute safely in sandbox - permissions enforced at filesystem level
    cmd = extract_command(response)  # e.g., "find . -name '*.py'"
    result = sandbox.run(cmd)
    print(result.stdout)
```

The agent cannot access `.env` even if it tries - the file is invisible at the filesystem level.

See [`example/ticket-agent/`](example/ticket-agent/) for a complete interactive demo.

## Features

- **Fine-grained permissions**: `none` / `view` / `read` / `write` with glob patterns
- **Lightweight isolation**: bubblewrap (`bwrap`) for fast startup
- **Docker runtime**: Full isolation with custom images and resource limits
- **Delta Layer (COW)**: Copy-On-Write isolation for multi-sandbox write safety
- **Stateful sessions**: Persistent shell with working directory and environment
- **Async SDK**: Full async/await support for high-concurrency scenarios
- **Permission presets**: Built-in presets (`agent-safe`, `read-only`, `full-access`)

## Installation

### Server

```bash
git clone https://github.com/ajaxzhan/sandbox-rls.git
cd sandbox-rls

go mod tidy
go build -o bin/sandbox-server ./cmd/sandbox-server

# Start (gRPC :9000, REST :8080)
./bin/sandbox-server -config configs/sandbox-server.yaml
```

**Prerequisites**: Go 1.21+, bubblewrap (`bwrap`)

### Python SDK

```bash
pip install -e sdk/python/
```

## Usage

### High-Level API (Recommended)

```python
from sandbox_rls import Sandbox, RuntimeType, ResourceLimits

# Basic usage
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)

# With Docker and resource limits
with Sandbox.from_local(
    "./my-project",
    preset="agent-safe",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(memory_bytes=512 * 1024 * 1024, pids_limit=100),
) as sandbox:
    with sandbox.session() as session:
        session.exec("pip install -r requirements.txt")
        result = session.exec("pytest")
        print(result.stdout)
```

### Async SDK

For high-concurrency scenarios, use the async API:

```python
import asyncio
from sandbox_rls import AsyncSandbox

async def main():
    async with await AsyncSandbox.from_local("./my-project") as sandbox:
        result = await sandbox.run("python main.py")
        print(result.stdout)
        
        # Async sessions
        async with sandbox.session() as session:
            await session.exec("cd /workspace")
            result = await session.exec("npm test")

asyncio.run(main())
```

The async SDK provides the same API as the sync version, with `await` for all operations.

### Permission Presets

| Preset | Description |
|--------|-------------|
| `agent-safe` | Read all, write to `/output` & `/tmp`, hide secrets |
| `read-only` | Read all files, no write access |
| `full-access` | Full read/write access |
| `development` | Full access except secrets |

```python
from sandbox_rls import list_presets, extend_preset

# Extend a preset
rules = extend_preset("agent-safe", additions=[
    {"pattern": "/custom/**", "permission": "write"}
])
```

### Error Handling

```python
from sandbox_rls import Sandbox, CommandTimeoutError, CommandExecutionError

try:
    with Sandbox.from_local("./project") as sandbox:
        result = sandbox.run("python main.py", timeout=30, raise_on_error=True)
except CommandTimeoutError:
    print("Command timed out")
except CommandExecutionError as e:
    print(f"Failed (exit {e.exit_code}): {e.stderr}")
```

### Low-Level API

For full control, use `SandboxClient` directly:

```python
from sandbox_rls import SandboxClient

client = SandboxClient(endpoint="localhost:9000")

# Create codebase → upload files → create sandbox → start → exec → cleanup
codebase = client.create_codebase(name="my-project", owner_id="user_001")
client.upload_file(codebase.id, "main.py", b"print('hello')")

sandbox = client.create_sandbox(
    codebase_id=codebase.id,
    permissions=[{"pattern": "**/*", "permission": "read"}],
)
client.start_sandbox(sandbox.id)

result = client.exec(sandbox.id, command="python /workspace/main.py")
print(result.stdout)

client.destroy_sandbox(sandbox.id)
client.delete_codebase(codebase.id)
```

### REST API

```bash
# Create codebase
curl -X POST http://localhost:8080/v1/codebases \
  -d '{"name": "my-project", "owner_id": "user_001"}'

# Create sandbox
curl -X POST http://localhost:8080/v1/sandboxes \
  -d '{"codebase_id": "cb_xxx", "permissions": [{"pattern": "**/*", "permission": "PERMISSION_READ"}]}'

# Start → exec → cleanup
curl -X POST http://localhost:8080/v1/sandboxes/sb_xxx/start
curl -X POST http://localhost:8080/v1/sandboxes/sb_xxx/exec -d '{"command": "ls /workspace"}'
curl -X DELETE http://localhost:8080/v1/sandboxes/sb_xxx
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Client Layer                           │
│              Go SDK / Python SDK / REST API                 │
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

**Delta Layer (COW)**: Multiple sandboxes can share the same codebase with isolated writes. Each sandbox writes to its own delta directory; changes sync to source on completion (Last-Writer-Wins).

## Performance

The architecture is designed to be lightweight. Each sandbox consumes minimal resources:

| Component | Per-Sandbox Overhead |
|-----------|---------------------|
| Memory | ~5 MB |
| Processes | ~2 |
| FUSE mount | 1 |
| Docker container | 1 (Docker runtime only) |

**Stress test results** on a 2-core / 4GB RAM server (Docker runtime):

| Metric | Result |
|--------|--------|
| Max concurrent sandboxes | **100+** (tested up to 120) |
| Memory at 100 sandboxes | ~67% usage |
| Stability | No crashes, clean resource cleanup |

**Recommended capacity** (conservative):

| Server Spec | Suggested Max Sandboxes |
|-------------|------------------------|
| 2 cores / 4 GB | 50–80 |
| 4 cores / 8 GB | 150–200 |
| 8 cores / 16 GB | 400+ |

The bottleneck is typically memory, not CPU or FUSE. For higher concurrency, consider sandbox pooling or on-demand creation.

## Comparison

| Capability | Sandbox-RLS | E2B | Docker | Others |
|------------|-------------|-----|--------|--------|
| Path-based least privilege | ✅ (glob + priority) | ❌ | ⚠️ coarse | ⚠️ varies |
| Hidden paths (`none`) | ✅ invisible | ❌ | ❌ | ⚠️ varies |
| List-only paths (`view`) | ✅ | ❌ | ❌ | ❌ |
| Multi-sandbox codebase sharing | ✅ | ⚠️ | ⚠️ | ⚠️ varies |

## Roadmap

**Completed**: Session support, Docker runtime, resource limits, Delta Layer (COW), one-liner API, permission presets, semantic exceptions, async SDK.

**Next**: CLI tool, Go SDK, configuration files, file locking, agent communication.

**Out of scope**: MicroVM isolation, hibernate/wake (CRIU), million-scale concurrency.



## Development

```bash
# Run tests
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Known Limitations

### macOS + Docker Desktop

The `view` permission level may not work correctly on macOS with Docker Desktop due to VirtioFS limitations. Files appear as "No such file" inside containers.

**Workarounds**: Use Linux, use `read` instead of `view`, or use `bwrap` runtime.

| Permission | Linux | macOS (Docker Desktop) |
|------------|-------|------------------------|
| `none` | ✅ | ✅ |
| `view` | ✅ | ❌ |
| `read` | ✅ | ✅ |
| `write` | ✅ | ✅ |

## Examples

| Example | Description |
|---------|-------------|
| [`example/ticket-agent/`](example/ticket-agent/) | Interactive AI agent with permission demo (write/read/view/none) |

## References

- [FUSE is All You Need](https://jakobemmerling.de/posts/fuse-is-all-you-need/)
- [Tursodatabase's Agentfs](https://github.com/tursodatabase/agentfs)
- [Anthropic's Agent Skill](https://github.com/anthropics/skills)

## License

MIT
