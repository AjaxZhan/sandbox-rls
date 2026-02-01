# Sandbox SDK for Python

Python client library for the Sandbox RLS service - a sandbox infrastructure for AI Agents with fine-grained file permission control.

## Installation

```bash
pip install sandbox-sdk
```

Or install from source:

```bash
cd sdk/python
pip install -e .
```

## Quick Start (High-Level API)

The easiest way to use the SDK is with the high-level `Sandbox` class:

```python
from sandbox_sdk import Sandbox

# One-liner to create a sandbox from a local directory
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)
```

This automatically:
1. Creates a codebase
2. Uploads all files from the directory
3. Creates a sandbox with the "agent-safe" preset
4. Starts the sandbox
5. Cleans up everything when done

### Using Docker Runtime with Resource Limits

```python
from sandbox_sdk import Sandbox, RuntimeType, ResourceLimits

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
    # Use a session for stateful commands
    with sandbox.session() as session:
        session.exec("cd /workspace")
        session.exec("pip install -r requirements.txt")
        result = session.exec("pytest")
        print(result.stdout)
```

### Permission Presets

The SDK includes several built-in permission presets:

| Preset | Description |
|--------|-------------|
| `agent-safe` | Read all files, write to /output and /tmp, hide secrets (.env, *.key, etc.) |
| `read-only` | Read all files, no write access |
| `full-access` | Full read/write access to all files |
| `development` | Full access except secrets |
| `view-only` | Can see file names but not read content |

```python
from sandbox_sdk import list_presets, get_preset, extend_preset

# List all available presets
print(list_presets())  # ['agent-safe', 'development', 'full-access', 'read-only', 'view-only']

# Extend a preset with custom rules
rules = extend_preset(
    "agent-safe",
    additions=[{"pattern": "/custom/**", "permission": "write"}],
)
```

### Error Handling

The SDK provides semantic exception classes:

```python
from sandbox_sdk import Sandbox, SandboxError, CommandTimeoutError, CommandExecutionError

try:
    with Sandbox.from_local("./my-project") as sandbox:
        result = sandbox.run("python main.py", timeout=30, raise_on_error=True)
except CommandTimeoutError as e:
    print(f"Command timed out: {e}")
except CommandExecutionError as e:
    print(f"Command failed with exit code {e.exit_code}: {e.stderr}")
except SandboxError as e:
    print(f"Sandbox error: {e}")
```

## Low-Level API

For fine-grained control, use the `SandboxClient` directly:

```python
from sandbox_sdk import SandboxClient, RuntimeType, ResourceLimits

# Connect to the sandbox server
client = SandboxClient(endpoint="localhost:9000")

# Create a codebase (file storage)
codebase = client.create_codebase(name="my-project", owner_id="user_123")

# Upload a file
client.upload_file(codebase.id, "hello.py", b'print("Hello, World!")')

# Create a sandbox with permissions and Docker runtime
sandbox = client.create_sandbox(
    codebase_id=codebase.id,
    permissions=[
        {"pattern": "**/*.py", "permission": "read"},
        {"pattern": "/docs/**", "permission": "write"},
    ],
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(memory_bytes=256 * 1024 * 1024),
)

# Start the sandbox
client.start_sandbox(sandbox.id)

# Execute a command
result = client.exec(sandbox.id, command="python hello.py")
print(result.stdout)  # Output: Hello, World!

# Clean up
client.destroy_sandbox(sandbox.id)
client.delete_codebase(codebase.id)
```

## Features

### Permission Levels

The SDK supports four permission levels:

| Level | Description |
|-------|-------------|
| `none` | Completely invisible, not shown in `ls` |
| `view` | Visible in `ls`, but cannot read content |
| `read` | Can read file content |
| `write` | Can modify file |

### Permission Patterns

You can use glob patterns, directory paths, or file paths:

```python
from sandbox_sdk import PermissionRule, Permission, PatternType

permissions = [
    # Glob pattern - matches all .py files
    PermissionRule(pattern="**/*.py", permission=Permission.READ),
    
    # Directory - applies to entire directory
    PermissionRule(pattern="/docs/", permission=Permission.WRITE, type=PatternType.DIRECTORY),
    
    # Specific file
    PermissionRule(pattern="/config.yaml", permission=Permission.VIEW, type=PatternType.FILE),
]

sandbox = client.create_sandbox(codebase_id=codebase.id, permissions=permissions)
```

### Stateful Sessions

Sessions maintain a persistent shell process that preserves working directory and environment variables:

```python
with client.session(sandbox.id) as session:
    session.exec("cd /workspace/src")     # Change directory
    session.exec("export DEBUG=1")         # Set environment variable
    session.exec("source venv/bin/activate")  # Activate virtualenv
    result = session.exec("python main.py")   # All above state is preserved
```

### Command Execution

Execute commands with optional stdin, environment variables, and timeout:

```python
from datetime import timedelta

result = client.exec(
    sandbox_id=sandbox.id,
    command="python script.py",
    stdin="input data",
    env={"DEBUG": "1"},
    workdir="/workspace/src",
    timeout=timedelta(seconds=30),
)

print(f"Exit code: {result.exit_code}")
print(f"stdout: {result.stdout}")
print(f"stderr: {result.stderr}")
print(f"Duration: {result.duration}")
```

### Streaming Output

For long-running commands, use streaming:

```python
for chunk in client.exec_stream(sandbox.id, command="long-running-task"):
    print(chunk.decode(), end="")
```

### File Operations

```python
# List files
files = client.list_files(codebase.id, path="/src", recursive=True)
for f in files:
    print(f"{f.path} - {f.size} bytes")

# Download a file
content = client.download_file(codebase.id, "config.yaml")
print(content.decode())

# Upload a file
result = client.upload_file(codebase.id, "new_file.txt", b"file content")
print(f"Uploaded {result.size} bytes, checksum: {result.checksum}")
```

## Context Manager

Both the high-level `Sandbox` and low-level `SandboxClient` support context managers:

```python
# High-level (recommended)
with Sandbox.from_local("./project") as sandbox:
    sandbox.run("make test")
# Automatically cleans up sandbox and codebase

# Low-level
with SandboxClient(endpoint="localhost:9000") as client:
    sandbox = client.create_sandbox(codebase_id="cb_123")
    # ... use the sandbox ...
# Connection is automatically closed
```

## API Reference

### High-Level API

#### Sandbox Class

- `Sandbox.from_local(path, preset, permissions, runtime, image, resources, ...)` - Create sandbox from local directory
- `Sandbox.from_codebase(codebase_id, preset, ...)` - Create sandbox from existing codebase
- `Sandbox.connect(sandbox_id)` - Connect to existing sandbox
- `sandbox.run(command, timeout, env, workdir, raise_on_error)` - Execute command (simplified)
- `sandbox.exec(command, stdin, env, workdir, timeout)` - Execute command (full API)
- `sandbox.session(shell, env)` - Create stateful session
- `sandbox.read_file(path)` - Read file as string
- `sandbox.write_file(path, content)` - Write file
- `sandbox.list_files(path, recursive)` - List files
- `sandbox.start()` / `sandbox.stop()` - Lifecycle control
- `sandbox.destroy(delete_codebase)` - Clean up

#### Presets

- `get_preset(name)` - Get preset as PermissionRule list
- `get_preset_dicts(name)` - Get preset as dict list
- `extend_preset(base, additions, overrides)` - Extend preset
- `list_presets()` - List available presets
- `register_preset(name, rules)` - Register custom preset

### Low-Level API

#### SandboxClient

- `create_sandbox(codebase_id, permissions, runtime, image, resources, labels, expires_in)` - Create sandbox
- `get_sandbox(sandbox_id)` - Get sandbox info
- `list_sandboxes(codebase_id)` - List sandboxes
- `start_sandbox(sandbox_id)` - Start sandbox
- `stop_sandbox(sandbox_id)` - Stop sandbox
- `destroy_sandbox(sandbox_id)` - Destroy sandbox
- `exec(sandbox_id, command, stdin, env, workdir, timeout)` - Execute command
- `exec_stream(sandbox_id, command, ...)` - Execute with streaming
- `create_session(sandbox_id, shell, env)` - Create session
- `session_exec(session_id, command, timeout)` - Execute in session
- `create_codebase(name, owner_id)` - Create codebase
- `get_codebase(codebase_id)` - Get codebase info
- `list_codebases(owner_id)` - List codebases
- `delete_codebase(codebase_id)` - Delete codebase
- `upload_file(codebase_id, file_path, content)` - Upload file
- `download_file(codebase_id, file_path)` - Download file
- `list_files(codebase_id, path, recursive)` - List files

### Types

- `RuntimeType` - BWRAP, DOCKER
- `ResourceLimits` - memory_bytes, cpu_quota, cpu_shares, pids_limit
- `Permission` - NONE, VIEW, READ, WRITE
- `PatternType` - GLOB, DIRECTORY, FILE
- `SandboxStatus` - PENDING, RUNNING, STOPPED, ERROR
- `SessionStatus` - UNKNOWN, ACTIVE, CLOSED

### Exceptions

- `SandboxError` - Base exception
- `SandboxNotFoundError` - Sandbox not found
- `SandboxNotRunningError` - Sandbox not running
- `CodebaseError` - Codebase operation error
- `CodebaseNotFoundError` - Codebase not found
- `CommandTimeoutError` - Command timed out
- `CommandExecutionError` - Command failed (non-zero exit)
- `PermissionDeniedError` - Permission denied
- `SessionError` - Session operation error
- `ConnectionError` - Connection to service failed

## Development

```bash
# Install dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Format code
black sandbox_sdk tests
ruff check --fix sandbox_sdk tests

# Type check
mypy sandbox_sdk
```

## License

MIT
