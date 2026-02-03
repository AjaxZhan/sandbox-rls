# High-Level API Guide

The `Sandbox` class provides the simplest way to create and manage sandboxes with automatic resource cleanup. This guide covers all methods, parameters, and usage patterns.

---

## Table of Contents

- [Creating Sandboxes](#creating-sandboxes)
  - [from_local()](#from_local)
  - [from_codebase()](#from_codebase)
  - [connect()](#connect)
- [Lifecycle Management](#lifecycle-management)
- [Executing Commands](#executing-commands)
- [Working with Sessions](#working-with-sessions)
- [File Operations](#file-operations)
- [Properties](#properties)
- [Complete Examples](#complete-examples)

---

## Creating Sandboxes

### `Sandbox.from_local()`

Create a sandbox from a local directory. This is the **recommended** method for most use cases.

#### Signature

```python
@classmethod
def from_local(
    cls,
    path: str,
    preset: Optional[str] = "view-only",
    permissions: Optional[List[Union[PermissionRule, Dict]]] = None,
    runtime: RuntimeType = RuntimeType.BWRAP,
    image: Optional[str] = None,
    resources: Optional[ResourceLimits] = None,
    endpoint: str = "localhost:9000",
    secure: bool = False,
    owner_id: Optional[str] = None,
    codebase_name: Optional[str] = None,
    ignore_patterns: Optional[List[str]] = None,
    labels: Optional[Dict[str, str]] = None,
    auto_start: bool = True,
) -> "Sandbox"
```

#### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `path` | `str` | **Required** | Path to local directory containing files |
| `preset` | `str` | `"view-only"` | Permission preset name (`"view-only"`, `"read-only"`, `"agent-safe"`, `"development"`, `"full-access"`) |
| `permissions` | `List[PermissionRule \| Dict]` | `None` | Additional permission rules (merged with preset) |
| `runtime` | `RuntimeType` | `RuntimeType.BWRAP` | Runtime type (`BWRAP` or `DOCKER`) |
| `image` | `str` | `None` | Docker image name (required for Docker runtime) |
| `resources` | `ResourceLimits` | `None` | Resource limits (memory, CPU, PIDs) |
| `endpoint` | `str` | `"localhost:9000"` | AgentFense server gRPC endpoint |
| `secure` | `bool` | `False` | Use TLS for connection |
| `owner_id` | `str` | Auto-generated | Owner identifier for the codebase |
| `codebase_name` | `str` | Derived from path | Name for the codebase |
| `ignore_patterns` | `List[str]` | `None` | Additional gitignore-style patterns to skip during upload |
| `labels` | `Dict[str, str]` | `None` | Key-value labels for organization/filtering |
| `auto_start` | `bool` | `True` | Whether to automatically start the sandbox |

#### Returns

`Sandbox` instance ready for use (running if `auto_start=True`).

#### Raises

- `ValueError`: If `path` doesn't exist or isn't a directory
- `SandboxError`: If sandbox creation or startup fails
- `ConnectionError`: If cannot connect to AgentFense server

#### What It Does

1. **Validates path**: Checks that the directory exists
2. **Creates codebase**: Allocates storage on the server
3. **Uploads files**: Recursively walks the directory and uploads all files (respecting `.gitignore`, `.sandboxignore`, and `ignore_patterns`)
4. **Merges permissions**: Combines preset with custom `permissions`
5. **Creates sandbox**: Configures runtime and permission rules
6. **Starts sandbox** (if `auto_start=True`): Mounts FUSE filesystem and starts isolation
7. **Returns**: `Sandbox` instance (use as context manager for auto-cleanup)

#### Basic Example

```python
from agentfense import Sandbox

# Simplest usage: upload current directory with view-only access
with Sandbox.from_local(".") as sandbox:
    result = sandbox.run("ls -la")
    print(result.stdout)
```

#### Example: Custom Permissions

```python
from agentfense import Sandbox, PermissionRule, Permission

with Sandbox.from_local(
    "./my-project",
    preset="read-only",  # Start with read-only base
    permissions=[
        # Add write access to output directory
        PermissionRule(pattern="/output/**", permission=Permission.WRITE),
        # Block access to secrets
        PermissionRule(pattern="/secrets/**", permission=Permission.NONE),
    ]
) as sandbox:
    result = sandbox.run("python process.py")
```

#### Example: Docker Runtime with Resource Limits

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./backend",
    preset="agent-safe",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB
        cpu_quota=50000,  # 50% of one CPU core
        pids_limit=100,   # Max 100 processes
    ),
    labels={
        "team": "backend",
        "env": "testing",
    }
) as sandbox:
    # Sandbox is isolated in Docker container with enforced limits
    result = sandbox.run("pytest", timeout=300)
    print(f"Tests completed in {result.duration}")
```

#### Example: Ignore Patterns

```python
# Skip large files and build artifacts during upload
with Sandbox.from_local(
    "./project",
    ignore_patterns=[
        "*.log",
        "dist/**",
        "build/**",
        "*.mp4",
        "*.zip",
    ]
) as sandbox:
    result = sandbox.run("make test")
```

#### Important Notes

- **Automatic cleanup**: Context manager (`with` statement) automatically destroys sandbox and deletes codebase on exit
- **Default preset**: If `preset=None`, defaults to `"view-only"` (safest option)
- **File size limit**: Files larger than 50 MB are skipped by default (configured in `walk_directory()`)
- **Gitignore support**: Automatically respects `.gitignore` and `.sandboxignore` in the root directory
- **Owner ID**: Auto-generated based on current user if not provided

---

### `Sandbox.from_codebase()`

Create a sandbox from an **existing** codebase. Use this when you want to reuse the same files across multiple sandboxes with different permissions.

#### Signature

```python
@classmethod
def from_codebase(
    cls,
    codebase_id: str,
    preset: Optional[str] = "view-only",
    permissions: Optional[List[Union[PermissionRule, Dict]]] = None,
    runtime: RuntimeType = RuntimeType.BWRAP,
    image: Optional[str] = None,
    resources: Optional[ResourceLimits] = None,
    endpoint: str = "localhost:9000",
    secure: bool = False,
    labels: Optional[Dict[str, str]] = None,
    auto_start: bool = True,
) -> "Sandbox"
```

#### Parameters

Same as `from_local()` except:
- **`codebase_id`** (required): ID of the existing codebase to use
- No `path`, `owner_id`, `codebase_name`, `ignore_patterns` (codebase already exists)

#### Returns

`Sandbox` instance (does **not** own the codebase, won't delete it on cleanup).

#### Use Cases

- **Multiple sandboxes from same code**: Test different permission sets on identical files
- **Shared repository**: Multiple users accessing the same codebase
- **Performance**: Skip re-uploading files for repeated runs

#### Example: Two Sandboxes with Different Permissions

```python
from agentfense import Sandbox, SandboxClient

# First, create a codebase manually
client = SandboxClient()
codebase = client.create_codebase(name="shared-repo", owner_id="team_A")

# Upload files once
for file_path, content in walk_directory("./project"):
    client.upload_file(codebase.id, file_path, content)

# Create read-only sandbox
with Sandbox.from_codebase(
    codebase_id=codebase.id,
    preset="read-only",
    labels={"purpose": "analysis"},
) as sandbox1:
    result1 = sandbox1.run("pylint .")

# Create writable sandbox (separate execution)
with Sandbox.from_codebase(
    codebase_id=codebase.id,
    preset="development",
    labels={"purpose": "testing"},
) as sandbox2:
    result2 = sandbox2.run("pytest --cov")

# Codebase still exists after both sandboxes are destroyed
client.delete_codebase(codebase.id)  # Clean up manually
```

---

### `Sandbox.connect()`

Connect to an **existing, running** sandbox. Use this to reconnect after a disconnection or to access a sandbox created by another process.

#### Signature

```python
@classmethod
def connect(
    cls,
    sandbox_id: str,
    endpoint: str = "localhost:9000",
    secure: bool = False,
) -> "Sandbox"
```

#### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `sandbox_id` | `str` | **Required** | ID of the existing sandbox |
| `endpoint` | `str` | `"localhost:9000"` | AgentFense server endpoint |
| `secure` | `bool` | `False` | Use TLS for connection |

#### Returns

`Sandbox` instance connected to the existing sandbox (does **not** own the sandbox, won't destroy it on cleanup).

#### Example: Reconnect After Network Issue

```python
from agentfense import Sandbox

# Original creation
sandbox = Sandbox.from_local("./project", auto_start=True)
sandbox_id = sandbox.id
print(f"Sandbox created: {sandbox_id}")

# ... network interruption or process restart ...

# Reconnect to the same sandbox
sandbox = Sandbox.connect(sandbox_id)
result = sandbox.run("whoami")
print(result.stdout)

# Manual cleanup (since we didn't create it)
sandbox.destroy()
```

#### Example: Access Sandbox Created by Another Process

```python
# Process A creates a long-lived sandbox
sandbox = Sandbox.from_local("./data", auto_start=True)
print(f"Sandbox ID: {sandbox.id}")  # Share this ID

# Process B connects to the same sandbox
sandbox_id = "sb_abc123"  # From Process A
sandbox = Sandbox.connect(sandbox_id)
result = sandbox.run("cat /workspace/data.csv")
```

---

## Lifecycle Management

### `start()`

Start a sandbox that's in `PENDING` or `STOPPED` state.

```python
def start(self) -> "Sandbox"
```

**Returns**: Self (for method chaining)

**Example**:
```python
sandbox = Sandbox.from_local("./project", auto_start=False)
print(f"Status: {sandbox.status}")  # PENDING

sandbox.start()
print(f"Status: {sandbox.status}")  # RUNNING

result = sandbox.run("python main.py")
```

---

### `stop()`

Stop a running sandbox (but don't destroy it). You can restart it later with `start()`.

```python
def stop(self) -> "Sandbox"
```

**Returns**: Self (for method chaining)

**Example**:
```python
with Sandbox.from_local("./project") as sandbox:
    sandbox.run("setup.sh")
    
    # Pause execution
    sandbox.stop()
    print("Sandbox stopped, but not destroyed")
    
    # Resume
    sandbox.start()
    sandbox.run("main.sh")
```

---

### `refresh()`

Refresh sandbox metadata from the server (updates `status`, `started_at`, etc.).

```python
def refresh(self) -> "Sandbox"
```

**Returns**: Self (for method chaining)

**Example**:
```python
sandbox = Sandbox.connect("sb_abc123")
print(f"Status: {sandbox.status}")  # Cached value

sandbox.refresh()
print(f"Status: {sandbox.status}")  # Fresh from server
```

---

### `destroy()`

Destroy the sandbox and optionally delete the codebase. **Irreversible**.

```python
def destroy(self, delete_codebase: Optional[bool] = None) -> None
```

**Parameters**:
- `delete_codebase`: If `True`, deletes codebase. If `False`, keeps it. If `None`, deletes only if this `Sandbox` created it (default).

**Example**:
```python
# Automatic cleanup (recommended)
with Sandbox.from_local("./project") as sandbox:
    sandbox.run("test.sh")
# Sandbox + codebase automatically destroyed here

# Manual cleanup
sandbox = Sandbox.from_local("./project")
try:
    sandbox.run("test.sh")
finally:
    sandbox.destroy()  # Explicit cleanup

# Keep codebase for reuse
sandbox = Sandbox.from_local("./project")
sandbox.run("test.sh")
sandbox.destroy(delete_codebase=False)  # Sandbox destroyed, codebase remains
```

---

## Executing Commands

### `run()`

Execute a command with simplified parameters and sensible defaults. **Recommended** for most use cases.

#### Signature

```python
def run(
    self,
    command: str,
    timeout: int = 60,
    env: Optional[Dict[str, str]] = None,
    workdir: Optional[str] = None,
    raise_on_error: bool = False,
) -> ExecResult
```

#### Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `command` | `str` | **Required** | Shell command to execute |
| `timeout` | `int` | `60` | Timeout in **seconds** (not timedelta) |
| `env` | `Dict[str, str]` | `None` | Environment variables to set |
| `workdir` | `str` | `None` | Working directory (default: `/workspace`) |
| `raise_on_error` | `bool` | `False` | Raise exception if exit code != 0 |

#### Returns

`ExecResult` with:
- `stdout`: str
- `stderr`: str
- `exit_code`: int
- `duration`: timedelta
- `command`: str (for debugging)

#### Example: Basic Execution

```python
result = sandbox.run("python --version")
print(f"Output: {result.stdout}")
print(f"Exit code: {result.exit_code}")
print(f"Duration: {result.duration}")
```

#### Example: With Environment Variables

```python
result = sandbox.run(
    "python app.py",
    env={
        "DEBUG": "1",
        "API_KEY": "test-key",
    },
    timeout=120,
)
```

#### Example: Error Handling

```python
from agentfense.exceptions import CommandExecutionError

# Option 1: Check exit code manually
result = sandbox.run("make build")
if result.exit_code != 0:
    print(f"Build failed: {result.stderr}")

# Option 2: Raise exception on failure
try:
    result = sandbox.run("make build", raise_on_error=True)
except CommandExecutionError as e:
    print(f"Build failed with code {e.exit_code}")
    print(f"Error: {e.stderr}")
```

---

### `exec()`

Execute a command with full control (supports stdin, timedelta timeout). Use when you need advanced features.

#### Signature

```python
def exec(
    self,
    command: str,
    stdin: Optional[str] = None,
    env: Optional[Dict[str, str]] = None,
    workdir: Optional[str] = None,
    timeout: Optional[timedelta] = None,
) -> ExecResult
```

#### Parameters

Same as `run()` except:
- **`stdin`**: Optional input to pass to command
- **`timeout`**: `timedelta` object (not int seconds)

#### Example: With Stdin

```python
from datetime import timedelta

result = sandbox.exec(
    command="python analyze.py",
    stdin="line1\nline2\nline3",
    timeout=timedelta(minutes=5),
)
```

---

### `exec_stream()`

Execute a command and stream output in real-time. Use for long-running commands where you want live updates.

#### Signature

```python
def exec_stream(
    self,
    command: str,
    stdin: Optional[str] = None,
    env: Optional[Dict[str, str]] = None,
    workdir: Optional[str] = None,
    timeout: Optional[timedelta] = None,
) -> Iterator[bytes]
```

#### Returns

Iterator yielding output chunks (bytes).

#### Example: Live Output

```python
for chunk in sandbox.exec_stream("./long-running-task.sh"):
    print(chunk.decode(), end="", flush=True)
```

#### Example: Save to File

```python
with open("output.log", "wb") as f:
    for chunk in sandbox.exec_stream("pytest --verbose"):
        f.write(chunk)
```

---

## Working with Sessions

See [Sessions Guide](./sessions.md) for detailed information.

### `session()`

Create a stateful shell session. Commands executed in a session preserve `cwd`, environment variables, and background processes.

```python
def session(
    self,
    shell: str = "/bin/sh",
    env: Optional[Dict[str, str]] = None,
) -> SessionWrapper
```

**Example**:
```python
with sandbox.session() as session:
    session.exec("cd /workspace/backend")
    session.exec("source venv/bin/activate")
    session.exec("export DEBUG=1")
    result = session.exec("python manage.py test")
```

---

## File Operations

### `read_file()`

Read a file from the sandbox as a UTF-8 string.

```python
def read_file(self, path: str) -> str
```

**Example**:
```python
content = sandbox.read_file("/workspace/output.txt")
print(content)
```

---

### `read_file_bytes()`

Read a file as raw bytes (for binary files).

```python
def read_file_bytes(self, path: str) -> bytes
```

**Example**:
```python
image_data = sandbox.read_file_bytes("/workspace/chart.png")
with open("local_chart.png", "wb") as f:
    f.write(image_data)
```

---

### `write_file()`

Write a file to the sandbox.

```python
def write_file(self, path: str, content: Union[str, bytes]) -> None
```

**Example**:
```python
# Write text file
sandbox.write_file("/workspace/config.json", '{"debug": true}')

# Write binary file
sandbox.write_file("/workspace/data.bin", b"\x00\x01\x02\x03")
```

---

### `list_files()`

List files in a directory.

```python
def list_files(self, path: str = "", recursive: bool = False) -> List[str]
```

**Example**:
```python
# List root directory
files = sandbox.list_files()
print(files)  # ['main.py', 'README.md', 'src/']

# List recursively
all_files = sandbox.list_files(recursive=True)
print(all_files)  # ['main.py', 'README.md', 'src/utils.py', 'src/tests/test_utils.py']
```

---

## Properties

### `id`

```python
@property
def id(self) -> str
```

Unique sandbox identifier.

```python
print(f"Sandbox ID: {sandbox.id}")  # sb_abc123def456
```

---

### `codebase_id`

```python
@property
def codebase_id(self) -> str
```

ID of the associated codebase.

```python
print(f"Codebase ID: {sandbox.codebase_id}")  # cb_xyz789
```

---

### `status`

```python
@property
def status(self) -> SandboxStatus
```

Current status: `PENDING`, `RUNNING`, `STOPPED`, or `ERROR`.

```python
from agentfense import SandboxStatus

if sandbox.status == SandboxStatus.RUNNING:
    sandbox.run("echo 'ready'")
```

---

### `runtime`

```python
@property
def runtime(self) -> RuntimeType
```

Runtime type (`BWRAP` or `DOCKER`).

```python
print(f"Using runtime: {sandbox.runtime.value}")  # bwrap
```

---

### `info`

```python
@property
def info(self) -> SandboxInfo
```

Full sandbox metadata (includes timestamps, labels, resources, etc.).

```python
print(f"Created at: {sandbox.info.created_at}")
print(f"Labels: {sandbox.info.labels}")
```

---

### `codebase`

```python
@property
def codebase(self) -> Codebase
```

Full codebase metadata.

```python
print(f"Codebase name: {sandbox.codebase.name}")
print(f"File count: {sandbox.codebase.file_count}")
print(f"Total size: {sandbox.codebase.size} bytes")
```

---

## Complete Examples

### Example 1: CI/CD Test Runner

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

def run_ci_tests(project_path: str) -> bool:
    """Run CI tests in an isolated sandbox with resource limits."""
    with Sandbox.from_local(
        project_path,
        preset="development",
        runtime=RuntimeType.DOCKER,
        image="python:3.11-slim",
        resources=ResourceLimits(
            memory_bytes=1024 * 1024 * 1024,  # 1 GB
            pids_limit=200,
        ),
        labels={"ci": "true", "env": "test"},
    ) as sandbox:
        # Install dependencies
        install_result = sandbox.run("pip install -r requirements.txt", timeout=300)
        if install_result.exit_code != 0:
            print(f"Dependency installation failed: {install_result.stderr}")
            return False
        
        # Run tests
        test_result = sandbox.run("pytest --cov --cov-report=xml", timeout=600)
        
        # Download coverage report
        if test_result.exit_code == 0:
            coverage_xml = sandbox.read_file("/workspace/coverage.xml")
            with open("coverage.xml", "w") as f:
                f.write(coverage_xml)
        
        return test_result.exit_code == 0

# Usage
success = run_ci_tests("./my-project")
print(f"Tests {'passed' if success else 'failed'}")
```

### Example 2: AI Agent with Output Collection

```python
from agentfense import Sandbox, PermissionRule, Permission

def run_agent_task(code_path: str, task: str) -> dict:
    """Run an AI agent task and collect results."""
    with Sandbox.from_local(
        code_path,
        preset="agent-safe",  # Blocks secrets, allows read + write to /output
        permissions=[
            PermissionRule(pattern="/logs/**", permission=Permission.WRITE),
        ],
    ) as sandbox:
        # Run agent
        with sandbox.session() as session:
            session.exec("cd /workspace")
            session.exec(f"export TASK='{task}'")
            result = session.exec("python agent.py", timeout=300)
        
        # Collect outputs
        outputs = {}
        for file_path in sandbox.list_files("/output", recursive=True):
            full_path = f"/workspace/output/{file_path}"
            outputs[file_path] = sandbox.read_file(full_path)
        
        # Collect logs
        logs = []
        for log_file in sandbox.list_files("/logs", recursive=True):
            full_path = f"/workspace/logs/{log_file}"
            logs.append(sandbox.read_file(full_path))
        
        return {
            "exit_code": result.exit_code,
            "stdout": result.stdout,
            "stderr": result.stderr,
            "duration": result.duration,
            "outputs": outputs,
            "logs": logs,
        }

# Usage
results = run_agent_task("./agent-code", "analyze repository")
print(f"Agent completed in {results['duration']}")
print(f"Generated {len(results['outputs'])} output files")
```

### Example 3: Multi-Stage Build Process

```python
from agentfense import Sandbox

def multi_stage_build(source_path: str):
    """Run a multi-stage build with state preservation."""
    with Sandbox.from_local(
        source_path,
        preset="development",
        runtime=RuntimeType.DOCKER,
        image="node:18",
    ) as sandbox:
        with sandbox.session() as session:
            # Stage 1: Install dependencies
            print("Installing dependencies...")
            result = session.exec("npm install")
            if result.exit_code != 0:
                raise Exception(f"Install failed: {result.stderr}")
            
            # Stage 2: Run linter
            print("Running linter...")
            session.exec("npm run lint")
            
            # Stage 3: Run tests
            print("Running tests...")
            test_result = session.exec("npm test")
            
            # Stage 4: Build production bundle
            print("Building...")
            session.exec("npm run build")
            
            # Stage 5: Collect artifacts
            print("Collecting artifacts...")
            build_files = sandbox.list_files("/workspace/dist", recursive=True)
            
            for file_path in build_files:
                content = sandbox.read_file_bytes(f"/workspace/dist/{file_path}")
                local_path = f"./build/{file_path}"
                os.makedirs(os.path.dirname(local_path), exist_ok=True)
                with open(local_path, "wb") as f:
                    f.write(content)
            
            print(f"Build complete! {len(build_files)} files generated.")
            return test_result.exit_code == 0

# Usage
success = multi_stage_build("./frontend")
```

---

## Best Practices

1. **Always use context managers** (`with` statement) to ensure cleanup
2. **Use presets** as a base, extend with `permissions` for custom needs
3. **Set appropriate timeouts** to prevent hung commands
4. **Use sessions** for multi-step workflows that depend on state
5. **Check exit codes** or use `raise_on_error=True` for critical commands
6. **Label your sandboxes** for easier debugging and filtering
7. **Reuse codebases** when running the same code with different configurations
8. **Monitor resource usage** in production (set `resources` parameter)

---

## See Also

- [Async API Guide](./async.md) - Concurrent sandbox execution
- [Sessions Guide](./sessions.md) - Stateful command execution
- [Permissions Guide](./permissions.md) - Fine-grained access control
- [Error Handling Guide](./errors.md) - Exception handling patterns
