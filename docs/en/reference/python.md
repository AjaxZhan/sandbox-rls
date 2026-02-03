# Python API Reference

Complete API reference for the AgentFense Python SDK.

## High-Level API

### Sandbox Class

The main class for sandbox operations, providing a simplified API.

```python
from agentfense import Sandbox
```

**Class Methods:**

| Method | Description |
|--------|-------------|
| `from_local(path, ...)` | Create sandbox from local directory |
| `from_codebase(codebase_id, ...)` | Create sandbox from existing codebase |
| `connect(sandbox_id)` | Connect to existing sandbox |

**Instance Methods:**

| Method | Description |
|--------|-------------|
| `run(command, ...)` | Execute command (simplified) |
| `exec(command, ...)` | Execute command (full parameters) |
| `exec_stream(command, ...)` | Execute command with streaming output |
| `session(shell, env)` | Create persistent session |
| `read_file(path)` | Read file content |
| `write_file(path, content)` | Write file content |
| `list_files(path, recursive)` | List files |
| `start()` | Start sandbox |
| `stop()` | Stop sandbox |
| `destroy(delete_codebase)` | Destroy sandbox |

#### from_local

```python
@classmethod
def from_local(
    path: str,
    preset: str = "agent-safe",
    permissions: Optional[List[dict]] = None,
    runtime: RuntimeType = RuntimeType.BWRAP,
    image: Optional[str] = None,
    resources: Optional[ResourceLimits] = None,
    endpoint: str = "localhost:9000",
    auto_start: bool = True,
    ignore_patterns: Optional[List[str]] = None,
) -> Sandbox
```

Create a sandbox from a local directory.

**Parameters:**
- `path`: Path to local directory
- `preset`: Permission preset name (default: "agent-safe")
- `permissions`: Additional permission rules
- `runtime`: Runtime type (BWRAP or DOCKER)
- `image`: Docker image (required if runtime=DOCKER)
- `resources`: Resource limits
- `endpoint`: Server endpoint
- `auto_start`: Start sandbox automatically
- `ignore_patterns`: Additional ignore patterns for upload

**Example:**

```python
with Sandbox.from_local(
    "./my-project",
    preset="agent-safe",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(memory_bytes=512 * 1024 * 1024),
) as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)
```

#### run

```python
def run(
    command: str,
    timeout: int = 60,
    env: Optional[Dict[str, str]] = None,
    workdir: Optional[str] = None,
    raise_on_error: bool = False,
) -> ExecResult
```

Execute a command in the sandbox.

**Parameters:**
- `command`: Command to execute
- `timeout`: Timeout in seconds (default: 60)
- `env`: Environment variables
- `workdir`: Working directory
- `raise_on_error`: Raise exception on non-zero exit code

**Returns:** `ExecResult` with stdout, stderr, exit_code, duration

### AsyncSandbox Class

Async version of Sandbox with the same API, all methods are async.

```python
from agentfense import AsyncSandbox

async with await AsyncSandbox.from_local("./project") as sandbox:
    result = await sandbox.run("python main.py")
```

## Low-Level API

### SandboxClient Class

Provides full control over the server.

```python
from agentfense import SandboxClient

client = SandboxClient(endpoint="localhost:9000")
```

**Sandbox Management:**

| Method | Description |
|--------|-------------|
| `create_sandbox(...)` | Create sandbox |
| `get_sandbox(sandbox_id)` | Get sandbox info |
| `list_sandboxes(codebase_id)` | List sandboxes |
| `start_sandbox(sandbox_id)` | Start sandbox |
| `stop_sandbox(sandbox_id)` | Stop sandbox |
| `destroy_sandbox(sandbox_id)` | Destroy sandbox |

**Command Execution:**

| Method | Description |
|--------|-------------|
| `exec(sandbox_id, command, ...)` | Execute command |
| `exec_stream(sandbox_id, command, ...)` | Stream execution |

**Session Management:**

| Method | Description |
|--------|-------------|
| `create_session(sandbox_id, ...)` | Create session |
| `session_exec(session_id, command)` | Execute in session |
| `destroy_session(session_id)` | Destroy session |

**Codebase Management:**

| Method | Description |
|--------|-------------|
| `create_codebase(name, owner_id)` | Create codebase |
| `get_codebase(codebase_id)` | Get codebase |
| `delete_codebase(codebase_id)` | Delete codebase |
| `upload_file(codebase_id, path, content)` | Upload file |
| `download_file(codebase_id, path)` | Download file |
| `list_files(codebase_id, path, recursive)` | List files |

## Types

### Permission Enum

```python
from agentfense import Permission

Permission.NONE   # Invisible (hidden from ls)
Permission.VIEW   # List only (can see name, cannot read)
Permission.READ   # Can read content
Permission.WRITE  # Can read and write
```

### PatternType Enum

```python
from agentfense import PatternType

PatternType.GLOB       # Wildcard pattern, e.g., **/*.py
PatternType.DIRECTORY  # Directory prefix, e.g., /docs/
PatternType.FILE       # Exact file, e.g., /config.yaml
```

### RuntimeType Enum

```python
from agentfense import RuntimeType

RuntimeType.BWRAP   # bubblewrap (lightweight)
RuntimeType.DOCKER  # Docker container (full isolation)
```

### PermissionRule Dataclass

```python
from agentfense import PermissionRule, Permission, PatternType

rule = PermissionRule(
    pattern="**/*.py",
    permission=Permission.READ,
    type=PatternType.GLOB,
    priority=0  # Optional, auto-calculated
)
```

### ResourceLimits Dataclass

```python
from agentfense import ResourceLimits

limits = ResourceLimits(
    memory_bytes=512 * 1024 * 1024,  # 512 MB
    cpu_quota=50000,                  # 50% CPU
    pids_limit=100                    # Max 100 processes
)
```

### ExecResult Dataclass

```python
@dataclass
class ExecResult:
    stdout: str       # Standard output
    stderr: str       # Standard error
    exit_code: int    # Exit code
    duration: float   # Execution time (seconds)
```

## Exceptions

### Exception Hierarchy

```
SandboxError (base)
├── ConnectionError          # Connection failed
├── InvalidConfigurationError # Invalid configuration
├── SandboxNotFoundError     # Sandbox not found
├── SandboxNotRunningError   # Sandbox not running
├── CommandTimeoutError      # Command timed out
├── CommandExecutionError    # Command failed
├── PermissionDeniedError    # Permission denied
├── SessionError             # Session error
│   ├── SessionNotFoundError # Session not found
│   └── SessionClosedError   # Session closed
├── CodebaseError            # Codebase error
│   ├── CodebaseNotFoundError # Codebase not found
│   ├── FileNotFoundError    # File not found
│   └── UploadError          # Upload failed
└── ResourceLimitExceededError # Resource limit exceeded
```

### Common Exception Handling

```python
from agentfense import (
    Sandbox,
    SandboxError,
    CommandTimeoutError,
    CommandExecutionError,
)

try:
    with Sandbox.from_local("./project") as sandbox:
        result = sandbox.run("python main.py", timeout=30, raise_on_error=True)
except CommandTimeoutError:
    print("Command timed out")
except CommandExecutionError as e:
    print(f"Command failed (exit {e.exit_code}): {e.stderr}")
except SandboxError as e:
    print(f"Sandbox error: {e}")
```

## Presets

```python
from agentfense import list_presets, get_preset, extend_preset, register_preset

# List all presets
list_presets()  # ['agent-safe', 'read-only', 'full-access', 'development', 'view-only']

# Get preset rules
rules = get_preset("agent-safe")

# Extend preset
rules = extend_preset("agent-safe", additions=[
    {"pattern": "/custom/**", "permission": "write"}
])

# Register custom preset
register_preset("my-preset", [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/output/**", "permission": "write"},
])
```

### Built-in Presets

| Preset | Description |
|--------|-------------|
| `agent-safe` | Read all, write to /output and /tmp, hide secrets |
| `read-only` | Read all files, no write access |
| `full-access` | Full read/write access |
| `development` | Full access except secrets |
| `view-only` | Can see file names but not read content |

## Utility Functions

```python
from agentfense.utils import (
    walk_directory,      # Walk directory tree
    parse_ignore_file,   # Parse .gitignore
    human_readable_size, # Human-readable file size
    generate_codebase_name,  # Generate codebase name
    count_files,         # Count files
)
```

## See Also

- [Python SDK Guide](../sdk/python/overview.md)
- [High-Level API Guide](../sdk/python/high-level.md)
- [Async API Guide](../sdk/python/async.md)
- [Error Handling](../sdk/python/errors.md)
