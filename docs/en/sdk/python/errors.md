# Error Handling Guide

The AgentFense SDK uses **semantic exceptions** to provide clear, actionable error information. This guide covers all exception types, common scenarios, and best practices for error handling.

---

## Table of Contents

- [Exception Hierarchy](#exception-hierarchy)
- [Core Exceptions](#core-exceptions)
- [Command Exceptions](#command-exceptions)
- [Sandbox Exceptions](#sandbox-exceptions)
- [Codebase Exceptions](#codebase-exceptions)
- [Session Exceptions](#session-exceptions)
- [Resource Exceptions](#resource-exceptions)
- [Handling Patterns](#handling-patterns)
- [Best Practices](#best-practices)

---

## Exception Hierarchy

All SDK exceptions inherit from `SandboxError`:

```
SandboxError (base)
├── ConnectionError
├── SandboxNotFoundError
├── SandboxNotRunningError
├── CommandTimeoutError
├── CommandExecutionError
├── PermissionDeniedError
├── CodebaseError
│   ├── CodebaseNotFoundError
│   ├── FileNotFoundError
│   └── UploadError
├── SessionError
│   ├── SessionNotFoundError
│   └── SessionClosedError
├── ResourceLimitExceededError
└── InvalidConfigurationError
```

**Benefits**:
- Catch all SDK errors: `except SandboxError`
- Catch specific errors: `except CommandTimeoutError`
- Rich error context (exit code, stderr, file path, etc.)

---

## Core Exceptions

### `SandboxError`

**Base exception** for all SDK errors.

```python
from agentfense import SandboxError

try:
    with Sandbox.from_local("./project") as sandbox:
        sandbox.run("command")
except SandboxError as e:
    # Catches ANY SDK error
    print(f"Sandbox error: {e}")
    print(f"Details: {e.details}")
    if e.grpc_code:
        print(f"gRPC code: {e.grpc_code}")
```

**Attributes**:
- `message` (str): Human-readable error description
- `details` (Optional[str]): Additional context
- `grpc_code` (Optional[int]): Original gRPC status code

**Use when**: You want to catch **any** SDK error.

---

### `ConnectionError`

**Failed to connect** to the AgentFense server.

```python
from agentfense import ConnectionError, Sandbox

try:
    sandbox = Sandbox.from_local("./project", endpoint="invalid:9999")
except ConnectionError as e:
    print(f"Cannot connect to {e.endpoint}")
    print(f"Details: {e.details}")
    # Retry with different endpoint or fail gracefully
```

**Attributes**:
- `endpoint` (str): The endpoint that failed
- `details` (str): Connection error details

**Common causes**:
- Server not running
- Wrong endpoint (host:port)
- Network issues
- Firewall blocking connection

**Solution**:
1. Check server is running: `./bin/agentfense-server`
2. Verify endpoint: default is `localhost:9000`
3. Check network connectivity

---

## Command Exceptions

### `CommandTimeoutError`

**Command exceeded timeout** duration.

```python
from agentfense import CommandTimeoutError

try:
    result = sandbox.run("sleep 100", timeout=10)
except CommandTimeoutError as e:
    print(f"Command timed out after {e.timeout_seconds}s")
    print(f"Command: {e.command}")
    # Command may still be running in sandbox
```

**Attributes**:
- `command` (str): The command that timed out
- `timeout_seconds` (float): The timeout duration
- `details` (str): Additional context

**Common causes**:
- Long-running operation (build, test, download)
- Infinite loop in user code
- Blocking I/O without timeout
- Hung process

**Solutions**:
1. Increase timeout: `sandbox.run(cmd, timeout=300)`
2. Use streaming for progress: `sandbox.exec_stream(cmd)`
3. Check for infinite loops in code
4. Use sessions with background processes

**Example: Handling Timeout**
```python
try:
    result = sandbox.run("pytest", timeout=60)
except CommandTimeoutError:
    # Try with longer timeout
    result = sandbox.run("pytest", timeout=300)
```

---

### `CommandExecutionError`

**Command exited with non-zero status** and `raise_on_error=True`.

```python
from agentfense import CommandExecutionError

try:
    result = sandbox.run("make build", raise_on_error=True)
except CommandExecutionError as e:
    print(f"Command failed: {e.command}")
    print(f"Exit code: {e.exit_code}")
    print(f"Stdout: {e.stdout}")
    print(f"Stderr: {e.stderr}")
    
    # Check specific error types
    if "ModuleNotFoundError" in e.stderr:
        print("Missing dependency - run pip install")
    elif e.exit_code == 127:
        print("Command not found")
```

**Attributes**:
- `command` (str): The command that failed
- `exit_code` (int): Non-zero exit code
- `stdout` (str): Standard output (truncated to 500 chars in error message)
- `stderr` (str): Standard error (truncated to 500 chars in error message)

**Common exit codes**:
- `1`: General error
- `2`: Misuse of shell command
- `126`: Command cannot execute (permission problem)
- `127`: Command not found
- `130`: Script terminated by Ctrl+C
- `137`: Process killed (SIGKILL), often due to OOM

**Solutions**:
1. Check stderr for error details
2. Verify dependencies installed
3. Check file permissions
4. Ensure command exists in PATH
5. Check resource limits (memory, disk)

**Example: Parse Error Type**
```python
try:
    sandbox.run("python script.py", raise_on_error=True)
except CommandExecutionError as e:
    if "SyntaxError" in e.stderr:
        print("Python syntax error")
    elif "ImportError" in e.stderr:
        print("Missing import")
    elif e.exit_code == 137:
        print("Out of memory - increase resources")
    else:
        print(f"Unknown error: {e.stderr}")
```

---

### `PermissionDeniedError`

**Permission rules blocked the operation**.

```python
from agentfense import PermissionDeniedError

with Sandbox.from_local(
    "./project",
    preset="read-only",
) as sandbox:
    try:
        sandbox.run("echo 'data' > /workspace/output.txt")
    except PermissionDeniedError as e:
        print(f"Permission denied: {e.operation}")
        if e.path:
            print(f"Path: {e.path}")
        # Need to update permissions
```

**Attributes**:
- `operation` (str): Operation that was denied
- `path` (Optional[str]): File path (if applicable)
- `details` (str): Additional context

**Common causes**:
- Trying to write to read-only path
- Accessing files with `none` permission
- Attempting operations on view-only files

**Solutions**:
1. Check permission rules: `sandbox.info.permissions`
2. Use appropriate preset: `agent-safe`, `development`
3. Add write permissions: `extend_preset("read-only", additions=[...])`
4. Test permissions before production

---

## Sandbox Exceptions

### `SandboxNotFoundError`

**Sandbox doesn't exist or was deleted**.

```python
from agentfense import SandboxNotFoundError

try:
    sandbox = Sandbox.connect("sb_invalid_id")
except SandboxNotFoundError as e:
    print(f"Sandbox not found: {e.sandbox_id}")
    # Create a new sandbox instead
```

**Attributes**:
- `sandbox_id` (str): The missing sandbox ID
- `details` (str): Additional context

**Common causes**:
- Wrong sandbox ID
- Sandbox was destroyed
- Sandbox expired (if expiration was set)

---

### `SandboxNotRunningError`

**Sandbox isn't in RUNNING state**.

```python
from agentfense import SandboxNotRunningError, SandboxStatus

try:
    result = sandbox.run("ls")
except SandboxNotRunningError as e:
    print(f"Sandbox not running (status: {e.current_status})")
    
    # Start it if stopped
    if e.current_status == "stopped":
        sandbox.start()
        result = sandbox.run("ls")
```

**Attributes**:
- `sandbox_id` (str): Sandbox ID
- `current_status` (str): Current status ("pending", "stopped", "error")
- `details` (str): Additional context

**Common causes**:
- Forgot to call `sandbox.start()`
- Sandbox was stopped
- Sandbox in error state

**Solution**:
1. Check status: `sandbox.status`
2. Start if needed: `sandbox.start()`
3. Refresh status: `sandbox.refresh()`

---

## Codebase Exceptions

### `CodebaseNotFoundError`

**Codebase doesn't exist or was deleted**.

```python
from agentfense import CodebaseNotFoundError

try:
    codebase = client.get_codebase("cb_invalid")
except CodebaseNotFoundError as e:
    print(f"Codebase not found: {e.codebase_id}")
```

**Attributes**:
- `codebase_id` (str): The missing codebase ID

---

### `FileNotFoundError`

**File doesn't exist in the codebase**.

```python
from agentfense import FileNotFoundError

try:
    content = client.download_file("cb_123", "nonexistent.txt")
except FileNotFoundError as e:
    print(f"File not found: {e.file_path}")
    print(f"Codebase: {e.codebase_id}")
```

**Attributes**:
- `codebase_id` (str): Codebase ID
- `file_path` (str): Missing file path

---

### `UploadError`

**File upload failed**.

```python
from agentfense import UploadError

try:
    client.upload_file("cb_123", "large_file.zip", huge_content)
except UploadError as e:
    print(f"Upload failed: {e.file_path}")
    print(f"Details: {e.details}")
```

**Attributes**:
- `file_path` (str): File that failed to upload
- `codebase_id` (str): Target codebase
- `details` (str): Error details

**Common causes**:
- Network interruption
- File too large
- Disk full on server
- Invalid file path

---

## Session Exceptions

### `SessionNotFoundError`

**Session doesn't exist or was closed**.

```python
from agentfense import SessionNotFoundError

try:
    session = client.get_session("session_invalid")
except SessionNotFoundError as e:
    print(f"Session not found: {e.session_id}")
```

**Attributes**:
- `session_id` (str): Missing session ID

---

### `SessionClosedError`

**Attempting to use a closed session**.

```python
from agentfense import SessionClosedError

session = sandbox.session()
session.close()

try:
    session.exec("ls")  # Session already closed
except SessionClosedError as e:
    print(f"Session closed: {e.session_id}")
    # Create new session
    with sandbox.session() as new_session:
        new_session.exec("ls")
```

**Attributes**:
- `session_id` (str): Closed session ID

---

## Resource Exceptions

### `ResourceLimitExceededError`

**Sandbox exceeded resource limits** (memory, CPU, processes).

```python
from agentfense import ResourceLimitExceededError

try:
    result = sandbox.run("./memory_hog.sh")
except ResourceLimitExceededError as e:
    print(f"Resource limit exceeded: {e.resource_type}")
    print(f"Limit: {e.limit}")
    # Increase resources or optimize code
```

**Attributes**:
- `resource_type` (str): "memory", "cpu", "pids", etc.
- `limit` (str): The limit value
- `details` (str): Additional context

**Common causes**:
- Memory leak in user code
- Too many processes spawned
- CPU-intensive operation with strict limits

**Solutions**:
1. Increase limits: `ResourceLimits(memory_bytes=1024*1024*1024)`
2. Optimize code
3. Use streaming for large data
4. Profile code to find bottlenecks

---

### `InvalidConfigurationError`

**Invalid configuration provided** to SDK.

```python
from agentfense import InvalidConfigurationError

try:
    sandbox = Sandbox.from_local(
        "./project",
        runtime=RuntimeType.DOCKER,
        # Missing required 'image' for docker
    )
except InvalidConfigurationError as e:
    print(f"Invalid config: {e.message}")
```

**Common causes**:
- Missing required parameters
- Invalid enum values
- Conflicting options
- Malformed permission rules

---

## Handling Patterns

### Pattern 1: Specific Error Handling

```python
from agentfense import (
    Sandbox,
    CommandTimeoutError,
    CommandExecutionError,
    PermissionDeniedError,
)

try:
    with Sandbox.from_local("./project") as sandbox:
        result = sandbox.run("make test", timeout=60, raise_on_error=True)
except CommandTimeoutError:
    print("Tests timed out - increase timeout or optimize tests")
except CommandExecutionError as e:
    print(f"Tests failed (exit {e.exit_code}): {e.stderr}")
except PermissionDeniedError:
    print("Permission denied - check sandbox permissions")
except Exception as e:
    print(f"Unexpected error: {e}")
```

---

### Pattern 2: Retry with Fallback

```python
from agentfense import CommandTimeoutError, CommandExecutionError

def run_with_retry(sandbox, command, max_retries=3):
    """Run command with retry logic."""
    for attempt in range(max_retries):
        try:
            return sandbox.run(command, timeout=60, raise_on_error=True)
        except CommandTimeoutError:
            if attempt < max_retries - 1:
                print(f"Timeout (attempt {attempt+1}), retrying...")
                continue
            raise
        except CommandExecutionError as e:
            if e.exit_code == 137:  # OOM
                print("Out of memory - cannot retry")
                raise
            if attempt < max_retries - 1:
                print(f"Failed (attempt {attempt+1}), retrying...")
                continue
            raise
```

---

### Pattern 3: Graceful Degradation

```python
from agentfense import PermissionDeniedError

def save_output(sandbox, content):
    """Try multiple output locations."""
    locations = ["/output/result.txt", "/tmp/result.txt", "/workspace/result.txt"]
    
    for location in locations:
        try:
            sandbox.write_file(location, content)
            print(f"Saved to {location}")
            return
        except PermissionDeniedError:
            continue
    
    print("Warning: Could not save output anywhere")
```

---

### Pattern 4: Detailed Error Logging

```python
import logging
from agentfense import CommandExecutionError, SandboxError

logger = logging.getLogger(__name__)

try:
    result = sandbox.run("python script.py", raise_on_error=True)
except CommandExecutionError as e:
    logger.error(
        "Command execution failed",
        extra={
            "command": e.command,
            "exit_code": e.exit_code,
            "stdout": e.stdout[:200],
            "stderr": e.stderr[:200],
            "sandbox_id": sandbox.id,
        }
    )
    raise
except SandboxError as e:
    logger.error(
        "Sandbox error",
        extra={
            "error_type": type(e).__name__,
            "message": e.message,
            "details": e.details,
            "sandbox_id": sandbox.id if hasattr(sandbox, 'id') else None,
        }
    )
    raise
```

---

### Pattern 5: Context Manager with Cleanup

```python
from agentfense import SandboxError

class SafeSandbox:
    """Wrapper with guaranteed cleanup."""
    
    def __init__(self, path):
        self.path = path
        self.sandbox = None
    
    def __enter__(self):
        try:
            self.sandbox = Sandbox.from_local(self.path)
            return self.sandbox
        except Exception as e:
            print(f"Failed to create sandbox: {e}")
            raise
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.sandbox:
            try:
                self.sandbox.destroy()
            except SandboxError as e:
                print(f"Warning: Cleanup failed: {e}")
        
        # Don't suppress exceptions
        return False

# Usage
with SafeSandbox("./project") as sandbox:
    sandbox.run("test.sh")
```

---

## Best Practices

### 1. Catch Specific Exceptions First

```python
# ✓ Good: Most specific to least specific
try:
    sandbox.run("command", raise_on_error=True)
except CommandTimeoutError:
    # Handle timeout
    pass
except CommandExecutionError:
    # Handle execution failure
    pass
except SandboxError:
    # Handle any other sandbox error
    pass

# ✗ Bad: Catch base exception first (unreachable code)
try:
    sandbox.run("command")
except SandboxError:  # Catches everything
    pass
except CommandTimeoutError:  # Never reached!
    pass
```

---

### 2. Use `raise_on_error` for Critical Commands

```python
# ✓ Good: Fail fast on critical errors
try:
    sandbox.run("pip install -r requirements.txt", raise_on_error=True)
    sandbox.run("pytest", raise_on_error=True)
except CommandExecutionError as e:
    print(f"Critical command failed: {e.stderr}")
    # Don't continue if setup fails

# ✗ Bad: Manual exit code checking (verbose)
result = sandbox.run("pip install -r requirements.txt")
if result.exit_code != 0:
    print(f"Failed: {result.stderr}")
    return
```

---

### 3. Provide Actionable Error Messages

```python
from agentfense import PermissionDeniedError, CommandExecutionError

try:
    sandbox.run("echo 'test' > /secrets/file.txt")
except PermissionDeniedError:
    print("""
    Permission denied: Cannot write to /secrets/
    
    Fix: Use an output directory instead:
        sandbox.run("echo 'test' > /output/file.txt")
    
    Or: Update permissions to allow write access:
        permissions = extend_preset("agent-safe", additions=[
            {"pattern": "/secrets/**", "permission": "write"}
        ])
    """)
```

---

### 4. Don't Catch All Exceptions Silently

```python
# ✗ Bad: Silent failure
try:
    sandbox.run("command")
except:
    pass  # What went wrong?!

# ✓ Good: Log or re-raise
try:
    sandbox.run("command")
except SandboxError as e:
    logger.error(f"Sandbox error: {e}")
    raise
```

---

### 5. Clean Up Resources Even on Error

```python
# ✓ Good: Context manager ensures cleanup
try:
    with Sandbox.from_local("./project") as sandbox:
        sandbox.run("command")
except SandboxError:
    pass  # Sandbox still cleaned up

# ✗ Risky: Manual cleanup (might be skipped)
sandbox = Sandbox.from_local("./project")
try:
    sandbox.run("command")
finally:
    sandbox.destroy()  # Better, but context manager is safer
```

---

## See Also

- [High-Level API Guide](./high-level.md) - Sandbox API reference
- [Permissions Guide](./permissions.md) - Understanding PermissionDeniedError
- [Best Practices](../best-practices.md) - General SDK recommendations
