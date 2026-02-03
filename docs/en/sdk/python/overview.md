# Python SDK Overview

## TL;DR

- **What**: AgentFense Python SDK provides a complete toolkit for running untrusted code in sandboxes with file-level permission control.
- **Value**: Go from 5+ manual steps (create storage, upload files, configure permissions, start runtime, execute) to 1 line of code with automatic cleanup.
- **Use when**: You need to run AI agents, scripts, or automation against real codebases without giving them full filesystem access or risking your secrets.

---

## What Problem Does This Solve?

### For Developers Building AI Applications

**Problem**: You want to let an AI agent interact with a real codebase, but:
- Giving full filesystem access exposes `.env` files, API keys, and credentials
- Manual permission setup is error-prone (you'll forget to block `/secrets/`)
- Creating isolated environments manually requires 5+ API calls and cleanup logic
- No built-in way to track what the agent reads, writes, or attempts to access

**Cost**: Hours spent debugging permission leaks, writing cleanup code, or worse—credentials exposed in production.

**AgentFense SDK Solution**:
```python
from agentfense import Sandbox

# One line: upload files, set permissions, start sandbox, auto-cleanup
with Sandbox.from_local("./my-project", preset="agent-safe") as sandbox:
    result = sandbox.run("python analyze.py")
```

This automatically:
- Blocks access to `.env*`, `secrets/`, `*.key`, `*.pem` patterns
- Allows read on all other files
- Allows write to `/output` and `/tmp`
- Cleans up sandbox + codebase when done

### For Platform Engineers

**Problem**: You're building a platform where users submit code/scripts to run against shared codebases:
- Each user needs different permission levels (read-only, write to specific paths, etc.)
- You need to enforce resource limits (memory, CPU, process count)
- Manual container orchestration is complex (lifecycle, networking, cleanup)
- No standard way to audit what each execution accessed

**AgentFense SDK Solution**: Abstract away infrastructure complexity with high-level APIs while retaining fine-grained control when needed.

### For Security Teams

**Problem**: Running untrusted automation in production environments:
- Traditional sandboxes are all-or-nothing (full access or none)
- File-level audit trails are hard to implement
- Secrets management requires custom infrastructure
- Need proof that specific files are genuinely inaccessible (not just "trust us")

**AgentFense SDK Solution**: FUSE-backed permission enforcement means blocked files are literally invisible to the sandbox process—they don't appear in `ls` and return "no such file" on access.

---

## Why Is This Important?

### From "Scripts" to "Platform Capability"

**Before AgentFense SDK**:
```python
# Manual approach: ~50 lines, easy to get wrong
client = SandboxClient("localhost:9000")
cb = client.create_codebase(...)
for file in os.walk(...):
    client.upload_file(cb.id, ...)
rules = [...]  # manually define every pattern
sb = client.create_sandbox(cb.id, permissions=rules)
client.start_sandbox(sb.id)
try:
    result = client.exec(sb.id, cmd)
finally:
    client.destroy_sandbox(sb.id)
    client.delete_codebase(cb.id)
```

**With AgentFense SDK**:
```python
# 3 lines: upload, execute, cleanup
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    result = sandbox.run("make test")
```

### From "Manual Checks" to "Enforced by Design"

**Human Errors Prevented**:
- Forgetting to delete sandboxes/codebases → Context manager ensures cleanup
- Typos in permission patterns → Pre-tested presets cover 80% of use cases
- Missing `.env.local` in your block list → `agent-safe` preset includes `.env*` glob
- Race conditions in concurrent execution → Delta layer provides COW isolation

### From "Hope It Works" to "Observable & Auditable"

The SDK doesn't just execute code—it provides structured data for every operation:
- `ExecResult` includes exit code, stdout, stderr, duration
- Exceptions map to specific error types (timeout, permission denied, not found)
- Permission rules are declarative and inspectable
- All operations are idempotent and retryable

---

## When You Must Use It

Use AgentFense Python SDK when:

✅ **Running untrusted code** (AI agents, user scripts, third-party plugins) against real codebases  
✅ **Path-level permission control** is required (not just read/write at repository level)  
✅ **Secrets must be invisible** to automation (not just "don't read them")  
✅ **Concurrent execution** where multiple tasks/users can't pollute each other's state  
✅ **Resource limits** needed (memory, CPU, process count) enforced at runtime  
✅ **Audit trail** required—you need to know what automation accessed/modified  
✅ **Stateful execution** needed (persist `cwd`, environment variables across commands)  
✅ **Cleanup is critical**—leaked sandboxes/containers are unacceptable in your environment  

Don't use it when:
- You trust the code completely (just run it directly)
- You only need stdout/stderr (use `subprocess` module)
- Filesystem isolation isn't required (Docker alone might suffice)

---

## Installation

### From PyPI (Recommended)

```bash
pip install agentfense
```

### From Source (Development)

```bash
cd sdk/python
pip install -e .
```

### Dependencies

- Python 3.8+
- gRPC (installed automatically)
- Running AgentFense server (see [Server Setup](../../getting-started/installation.md))

---

## API Layers: Choosing the Right Level

The SDK provides three API layers, each optimized for different use cases:

### 1. High-Level Sync API (`Sandbox`)

**Use when**: You want the simplest possible integration with automatic resource management.

**Key Methods**:
- `Sandbox.from_local(path)` - Create from local directory
- `sandbox.run(command)` - Execute and get result
- `sandbox.session()` - Stateful shell

**Characteristics**:
- Context manager support (`with` statement)
- Automatic cleanup
- Sensible defaults
- 80% of use cases

```python
from agentfense import Sandbox

with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("pytest")
    print(result.stdout)
```

### 2. High-Level Async API (`AsyncSandbox`)

**Use when**: You need to run multiple sandboxes concurrently or integrate with async frameworks (FastAPI, aiohttp, asyncio).

**Characteristics**:
- Fully async/await compatible
- Same API surface as `Sandbox`
- Ideal for I/O-bound workloads
- Scales to 100+ concurrent sandboxes

```python
from agentfense import AsyncSandbox
import asyncio

async def run_tests():
    async with await AsyncSandbox.from_local("./project") as sandbox:
        result = await sandbox.run("pytest")
        return result.stdout

asyncio.run(run_tests())
```

### 3. Low-Level API (`SandboxClient`)

**Use when**: You need fine-grained control over codebase management, want to reuse codebases across sandboxes, or are building your own abstractions.

**Characteristics**:
- Explicit control over every step
- Reusable codebases
- Advanced configuration
- ~20% of use cases (specialized)

```python
from agentfense import SandboxClient

client = SandboxClient(endpoint="localhost:9000")

# Create codebase once
codebase = client.create_codebase(name="shared-repo", owner_id="team_A")

# Upload files manually
client.upload_file(codebase.id, "main.py", b"...")

# Create multiple sandboxes from same codebase
sandbox1 = client.create_sandbox(codebase.id, permissions=[...])
sandbox2 = client.create_sandbox(codebase.id, permissions=[...])

# Explicit lifecycle
client.start_sandbox(sandbox1.id)
result = client.exec(sandbox1.id, command="python main.py")
client.destroy_sandbox(sandbox1.id)
```

### Choosing Between Sync and Async

| Criteria | Use Sync (`Sandbox`) | Use Async (`AsyncSandbox`) |
|----------|---------------------|---------------------------|
| **Concurrency** | 1-5 sandboxes sequentially | 10+ sandboxes in parallel |
| **Framework** | Flask, Django, scripts | FastAPI, aiohttp, asyncio |
| **Execution Model** | Blocking I/O is fine | Need async/await |
| **Complexity** | Simpler (no event loop) | Requires async understanding |
| **Typical Latency** | ms-level operations | µs-level context switching |

**Rule of Thumb**: Start with sync `Sandbox`. Migrate to `AsyncSandbox` only when you measure concurrency bottlenecks.

---

## Quick Start Examples

### Minimal Example: Run One Command

```python
from agentfense import Sandbox

# Simplest possible usage
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("ls -la")
    print(result.stdout)
```

**What happens**:
1. Uploads all files from `./my-project`
2. Creates sandbox with `view-only` preset (default)
3. Starts sandbox
4. Executes `ls -la`
5. Destroys sandbox + codebase on exit

### With Custom Permissions

```python
from agentfense import Sandbox, PermissionRule, Permission

with Sandbox.from_local(
    "./my-project",
    preset="read-only",
    permissions=[
        PermissionRule(pattern="/output/**", permission=Permission.WRITE),
        PermissionRule(pattern="/secrets/**", permission=Permission.NONE),
    ]
) as sandbox:
    result = sandbox.run("python process.py")
```

### Using Docker Runtime with Resource Limits

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./my-project",
    preset="agent-safe",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB
        pids_limit=100,
    ),
) as sandbox:
    result = sandbox.run("pytest", timeout=300)
```

### Stateful Execution (Session)

```python
with Sandbox.from_local("./my-project") as sandbox:
    with sandbox.session() as session:
        # All these commands run in the same shell process
        session.exec("cd /workspace/backend")
        session.exec("source venv/bin/activate")
        session.exec("export DEBUG=1")
        result = session.exec("python manage.py test")
        print(result.stdout)
```

### Async: Run 10 Sandboxes Concurrently

```python
import asyncio
from agentfense import AsyncSandbox

async def run_in_sandbox(project_path):
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        return await sandbox.run("make test")

async def main():
    projects = [f"./project-{i}" for i in range(10)]
    tasks = [run_in_sandbox(p) for p in projects]
    results = await asyncio.gather(*tasks)
    for i, result in enumerate(results):
        print(f"Project {i}: {result.exit_code}")

asyncio.run(main())
```

---

## Key Concepts

### Codebase vs Sandbox

- **Codebase**: File storage. Holds your source code, config files, etc. Reusable.
- **Sandbox**: Execution environment. Ties a codebase to permissions + runtime. Ephemeral.

**When to reuse codebases**:
- Running the same code with different permissions
- Multiple users accessing the same repository
- Testing different configurations on identical code

**When to create new codebases**:
- Each user has their own copy of files
- Files change frequently between runs

### Permission Presets

| Preset | Use Case | Read | Write | Blocks |
|--------|----------|------|-------|--------|
| `view-only` | Audit/inspection | File list only | None | N/A |
| `read-only` | Static analysis | All files | None | N/A |
| `agent-safe` | AI agents | All (except secrets) | `/output`, `/tmp` | `.env*`, `secrets/`, `*.key` |
| `development` | Dev automation | All (except secrets) | All (except secrets) | `.env*`, `secrets/`, `*.key` |
| `full-access` | Trusted scripts | All | All | None |

**Customization**: All presets can be extended with `permissions=[...]` parameter.

### Runtime Types

- **bwrap** (default): Lightweight Linux namespace isolation. Fast startup (~50ms). Requires Linux host.
- **docker**: Full container isolation. Cross-platform. Slower startup (~500ms). Stronger security boundary.

**Choose bwrap when**: Speed matters, Linux host, light isolation sufficient.  
**Choose Docker when**: Windows/Mac dev, need networking control, require image customization.

### Sessions vs Direct Execution

**Direct Execution** (`sandbox.run()`, `sandbox.exec()`):
- Each command starts fresh shell
- No state preserved between calls
- Fast for one-off commands

**Session** (`sandbox.session()`):
- Persistent shell process
- Preserves `cwd`, environment variables
- Slower startup, faster subsequent commands
- Required for: `cd`, `source`, `export`, background processes

---

## Error Handling Philosophy

The SDK uses **semantic exceptions** instead of generic errors:

```python
from agentfense import (
    Sandbox,
    CommandTimeoutError,
    CommandExecutionError,
    PermissionDeniedError,
    SandboxNotRunningError,
)

try:
    with Sandbox.from_local("./project") as sandbox:
        result = sandbox.run("make build", timeout=60, raise_on_error=True)
except CommandTimeoutError as e:
    # Command took > 60 seconds
    print(f"Timed out: {e.command}")
except CommandExecutionError as e:
    # Non-zero exit code
    print(f"Failed with code {e.exit_code}: {e.stderr}")
except PermissionDeniedError as e:
    # Tried to access blocked file
    print(f"Permission denied: {e.path}")
except SandboxNotRunningError as e:
    # Sandbox not started or already stopped
    print(f"Sandbox not running: {e.current_status}")
```

**Benefits**:
- Catch specific errors without parsing error messages
- Different handling for timeout vs failure
- Rich error context (exit code, stderr, command, etc.)

---

## Next Steps

- **High-Level API**: See [High-Level API Guide](./high-level.md) for `Sandbox` class details
- **Async API**: See [Async API Guide](./async.md) for concurrent execution
- **Sessions**: See [Sessions Guide](./sessions.md) for stateful execution
- **Permissions**: See [Permissions Guide](./permissions.md) for access control
- **Error Handling**: See [Error Handling Guide](./errors.md) for exception patterns
- **Examples**: Check `examples/` directory in the repository

---

## Performance Characteristics

| Operation | Latency | Notes |
|-----------|---------|-------|
| `Sandbox.from_local()` | 100-500ms | Dominated by file upload |
| `sandbox.run()` (bwrap) | 10-50ms | Per-command overhead |
| `sandbox.run()` (docker) | 50-200ms | Container startup |
| `session.exec()` (after creation) | 5-20ms | Reuses shell process |
| File upload (1 MB) | 10-50ms | Depends on network |
| File download (1 MB) | 10-50ms | Depends on network |

**Optimization Tips**:
- Reuse codebases when possible
- Use sessions for multiple commands
- Async API for concurrent sandboxes
- Batch file operations

---

## Comparison with Alternatives

| Approach | Setup Complexity | Permission Granularity | Audit Trail | Cleanup | Safety |
|----------|------------------|----------------------|-------------|---------|--------|
| **AgentFense SDK** | 1 line | Path-level | Built-in | Automatic | Enforced |
| **Docker directly** | 10+ lines | Container-level | Manual | Manual | Depends |
| **subprocess + chroot** | 20+ lines | Manual | None | Manual | Error-prone |
| **VM per execution** | 100+ lines | VM-level | Manual | Manual | Slow |
| **No isolation** | 0 lines | None | None | N/A | ❌ Unsafe |

**When to use something else**:
- **Just Docker**: You don't need file-level permissions
- **subprocess**: You trust the code completely
- **VMs**: You need kernel-level isolation (AgentFense uses containers/namespaces)
