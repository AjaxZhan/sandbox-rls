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

## Quick Start

```python
from sandbox_sdk import SandboxClient

# Connect to the sandbox server
client = SandboxClient(endpoint="localhost:9000")

# Create a codebase (file storage)
codebase = client.create_codebase(name="my-project", owner_id="user_123")

# Upload a file
client.upload_file(codebase.id, "hello.py", b'print("Hello, World!")')

# Create a sandbox with permissions
sandbox = client.create_sandbox(
    codebase_id=codebase.id,
    permissions=[
        {"pattern": "**/*.py", "permission": "read"},
        {"pattern": "/docs/**", "permission": "write"},
    ]
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

The client can be used as a context manager:

```python
with SandboxClient(endpoint="localhost:9000") as client:
    sandbox = client.create_sandbox(codebase_id="cb_123")
    # ... use the sandbox ...
# Connection is automatically closed
```

## API Reference

### SandboxClient

- `create_sandbox(codebase_id, permissions, labels, expires_in)` - Create a new sandbox
- `get_sandbox(sandbox_id)` - Get sandbox info
- `list_sandboxes(codebase_id)` - List all sandboxes
- `start_sandbox(sandbox_id)` - Start a sandbox
- `stop_sandbox(sandbox_id)` - Stop a sandbox
- `destroy_sandbox(sandbox_id)` - Destroy a sandbox
- `exec(sandbox_id, command, stdin, env, workdir, timeout)` - Execute command
- `exec_stream(sandbox_id, command, ...)` - Execute with streaming output
- `create_codebase(name, owner_id)` - Create a codebase
- `get_codebase(codebase_id)` - Get codebase info
- `list_codebases(owner_id)` - List codebases
- `delete_codebase(codebase_id)` - Delete a codebase
- `upload_file(codebase_id, file_path, content)` - Upload a file
- `download_file(codebase_id, file_path)` - Download a file
- `list_files(codebase_id, path, recursive)` - List files

## Development

```bash
# Install dev dependencies
pip install -e ".[dev]"

# Run tests
pytest

# Format code
black sandbox_sdk tests
ruff check --fix sandbox_sdk tests
```

## License

MIT
