# Core Concepts

This guide explains the key concepts in AgentFense: **Codebase**, **Sandbox**, **Permission**, **Session**, **Preset**, and **Delta Layer**. Understanding these concepts helps you design effective permission policies and use AgentFense efficiently.

## Codebase

**A Codebase is file storage on the server**—the source of truth for your code and data.

### What It Does

- Stores files uploaded from local directories or created programmatically
- Provides versioned, immutable file storage (files cannot be modified in place)
- Serves as the data source for one or more Sandboxes
- Tracks metadata: file count, total size, owner, timestamps

### Key Characteristics

| Characteristic | Description |
|----------------|-------------|
| **Persistent** | Files persist even after all Sandboxes are destroyed |
| **Shareable** | Multiple Sandboxes can use the same Codebase |
| **Isolated** | Each Codebase is isolated from others (no cross-contamination) |

### Lifecycle

```
Create → Upload Files → Create Sandbox(es) → Delete
```

### When to Use Multiple Codebases

- **One codebase per project**: Store all files for a single project
- **Versioned codebases**: Create new codebase for each release or snapshot
- **Multi-tenancy**: Separate codebase per user or team

### Example

```python
from agentfense import SandboxClient

client = SandboxClient(endpoint="localhost:9000")

# Create a codebase
codebase = client.create_codebase(
    name="my-app-v1.0",
    owner_id="team_123",
)

# Upload files
client.upload_file(codebase.id, "app.py", b"print('v1.0')")
client.upload_file(codebase.id, "config.yaml", b"version: 1.0")

# List files
files = client.list_files(codebase.id, path="/", recursive=True)
for f in files:
    print(f"{f.path} ({f.size} bytes)")

# Download a file
content = client.download_file(codebase.id, "app.py")
print(content.decode())

# Clean up (only after destroying all sandboxes)
client.delete_codebase(codebase.id)
```

### High-Level API (Automatic Codebase Management)

When using `Sandbox.from_local()`, codebases are created and deleted automatically:

```python
from agentfense import Sandbox

# Codebase is created, used, and deleted automatically
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("ls /workspace")
# Codebase deleted here
```

## Sandbox

**A Sandbox is an isolated execution environment** where untrusted code runs against a Codebase with enforced permissions.

### What It Does

- Mounts a Codebase as a FUSE filesystem at `/workspace`
- Enforces permission rules at the filesystem level
- Provides process isolation (bwrap or Docker)
- Captures writes to a per-sandbox Delta Layer (Copy-On-Write)
- Executes commands with controlled environment, working directory, and timeout

### Architecture

```
┌─────────────────────────────────────────┐
│            Sandbox Process              │
│   (bwrap namespace / Docker container)  │
├─────────────────────────────────────────┤
│         FUSE Filesystem Mount           │
│         /workspace (enforced)           │
├─────────────────────────────────────────┤
│          Delta Layer (COW)              │
│    (per-sandbox write isolation)        │
├─────────────────────────────────────────┤
│          Codebase Storage               │
│       (immutable source files)          │
└─────────────────────────────────────────┘
```

### Lifecycle

```
Create → Start → Exec/Session → Stop → Destroy
```

### Sandbox States

| State | Description | Available Operations |
|-------|-------------|---------------------|
| **PENDING** | Created but not started | Start, Destroy |
| **RUNNING** | Active and ready for commands | Exec, Session, Stop, Destroy |
| **STOPPED** | Stopped but not destroyed | Start, Destroy |
| **ERROR** | Failed to start or crashed | Destroy |

### Runtime Types

AgentFense supports two isolation mechanisms:

| Runtime | Isolation | Startup Time | Use Case |
|---------|-----------|--------------|----------|
| **bwrap** | Linux namespaces (lightweight) | ~50ms | Fast iteration, development, low overhead |
| **Docker** | Container (strong isolation) | ~500ms | Production, strict isolation, custom images |

### Resource Limits

Control resource usage with `ResourceLimits`:

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./project",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB RAM
        cpu_quota=50000,                 # 50% CPU (50000/100000)
        pids_limit=100,                  # Max 100 processes
    ),
) as sandbox:
    result = sandbox.run("python heavy_task.py")
```

### Example: Manual Sandbox Lifecycle

```python
from agentfense import SandboxClient

client = SandboxClient()

# Create sandbox
sandbox = client.create_sandbox(
    codebase_id="cb_xxx",
    permissions=[{"pattern": "**/*", "permission": "read"}],
)

# Start (allocates resources, mounts FUSE)
client.start_sandbox(sandbox.id)

# Execute commands
result = client.exec(sandbox.id, command="ls /workspace")
print(result.stdout)

# Stop (unmounts FUSE, stops processes)
client.stop_sandbox(sandbox.id)

# Destroy (cleans up all resources)
client.destroy_sandbox(sandbox.id)
```

## Permission

**Permissions define what an agent can do with each file or directory** in the Codebase.

### Four Permission Levels

AgentFense provides four levels of file access control:

| Level | Visibility | Read Content | Write/Modify | Example Use Case |
|-------|-----------|--------------|--------------|------------------|
| **none** | ❌ Invisible | ❌ | ❌ | Hide secrets, credentials, private keys |
| **view** | ✅ In `ls` | ❌ | ❌ | Show file structure without exposing content |
| **read** | ✅ | ✅ | ❌ | Source code, documentation, logs (read-only) |
| **write** | ✅ | ✅ | ✅ | Output directories, temp files, generated reports |

### Permission Patterns

Three types of patterns are supported:

| Type | Syntax | Example | Matches |
|------|--------|---------|---------|
| **glob** | Wildcard patterns | `**/*.py` | All Python files recursively |
|  |  | `/secrets/**` | All files under `/secrets` |
|  |  | `*.env*` | `.env`, `.env.local`, `.env.production` |
| **directory** | Path ending with `/` | `/docs/` | All files in `/docs` and subdirectories |
| **file** | Exact file path | `/config.yaml` | Only `/config.yaml` |

### Priority Algorithm

When multiple rules match a path, AgentFense uses this deterministic priority:

1. **Explicit Priority** (if set via `priority` field)
2. **Pattern Type**: `file` (3) > `directory` (2) > `glob` (1)
3. **Pattern Specificity**: More specific patterns win

Example:

```python
permissions = [
    {"pattern": "**/*", "permission": "read"},           # Priority: 0 (glob)
    {"pattern": "/secrets/**", "permission": "none"},    # Priority: 0 (glob)
    {"pattern": "/secrets/public.key", "permission": "read"},  # Priority: 0 (file)
]

# Result:
# - /secrets/private.key → none (matched by /secrets/**)
# - /secrets/public.key → read (file pattern overrides glob)
# - /app/main.py → read (matched by **/*) 
```

### Example: Building Permission Rules

```python
from agentfense import Sandbox

# Scenario: Allow read all, write to /output, hide secrets
PERMISSIONS = [
    # Base rule: read everything
    {"pattern": "**/*", "permission": "read", "priority": 0},
    
    # Allow writes to output directory
    {"pattern": "/output/**", "permission": "write", "priority": 10},
    
    # Hide secrets (higher priority overrides base)
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    
    # Exception: public key should be readable
    {"pattern": "/secrets/public.key", "permission": "read", "priority": 200},
]

with Sandbox.from_local("./project", permissions=PERMISSIONS) as sandbox:
    # Can read most files
    sandbox.run("cat /workspace/README.md")  # ✅
    
    # Can write to /output
    sandbox.run("echo result > /workspace/output/log.txt")  # ✅
    
    # Cannot see private keys
    sandbox.run("ls /workspace/secrets/")  # public.key only (no private.key)
    
    # Can read public key
    sandbox.run("cat /workspace/secrets/public.key")  # ✅
```

## Session

**A Session is a persistent shell process** that maintains state across multiple command executions.

### Why Use Sessions?

Without sessions, each `exec()` starts a new shell:

```python
# Each command runs in a new shell (state is lost)
sandbox.run("cd /workspace/src")      # Changes directory
sandbox.run("pwd")                    # Output: /workspace (NOT /workspace/src)
```

With sessions, state persists:

```python
# Commands run in the same shell (state is preserved)
with sandbox.session() as session:
    session.exec("cd /workspace/src")
    result = session.exec("pwd")
    print(result.stdout)  # Output: /workspace/src ✅
```

### What Sessions Preserve

- **Working Directory**: `cd` commands persist
- **Environment Variables**: `export VAR=value` persists
- **Shell State**: Activated virtualenvs, sourced files
- **Background Processes**: Processes started with `&` continue running

### Example: Multi-Step Workflow

```python
from agentfense import Sandbox

with Sandbox.from_local("./python-app") as sandbox:
    with sandbox.session() as session:
        # Setup environment
        session.exec("cd /workspace")
        session.exec("export DEBUG=1")
        session.exec("source venv/bin/activate")
        
        # Install dependencies (venv is active)
        session.exec("pip install -r requirements.txt")
        
        # Run tests (DEBUG=1 and venv still active)
        result = session.exec("pytest tests/")
        print(result.stdout)
```

### Session Lifecycle

```
Create → Exec (multiple times) → Close
```

Sessions are automatically closed when:
- Context manager exits (`with session:`)
- Sandbox is stopped or destroyed
- Timeout occurs (configurable)

### Low-Level Session API

```python
client = SandboxClient()

# Create session
session = client.create_session(
    sandbox_id=sandbox.id,
    shell="/bin/bash",  # Or /bin/sh, /bin/zsh
    env={"PYTHONPATH": "/workspace/lib"},
)

# Execute commands
result = client.session_exec(session.id, command="cd /workspace")
result = client.session_exec(session.id, command="ls")

# Close session (optional - auto-closed when sandbox stops)
client.close_session(session.id)
```

## Preset

**Presets are pre-configured permission templates** for common use cases.

### Built-In Presets

| Preset | Description | Use Case |
|--------|-------------|----------|
| **agent-safe** | Read all, write to `/output` & `/tmp`, hide secrets | Default for AI agents (safe exploration + controlled output) |
| **read-only** | Read all files, no write access | Code review, static analysis, safe browsing |
| **full-access** | Read and write all files | Trusted automation, testing, development |
| **development** | Full access except secrets | Local development with safety guardrails |
| **view-only** | See file names, cannot read content | Exploring file structure without exposing data |

### Using Presets

```python
from agentfense import Sandbox

# Use a preset directly
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    result = sandbox.run("ls")
```

### Extending Presets

```python
from agentfense import extend_preset

# Start with agent-safe, add custom rules
rules = extend_preset(
    base="agent-safe",
    additions=[
        {"pattern": "/logs/**", "permission": "write"},  # Also allow log writes
    ],
    overrides=[
        {"pattern": "**/.git/**", "permission": "read"},  # Override: make .git readable
    ],
)

with Sandbox.from_local("./project", permissions=rules) as sandbox:
    result = sandbox.run("echo log > /workspace/logs/app.log")  # ✅
```

### Creating Custom Presets

```python
from agentfense import register_preset

# Define a custom preset
register_preset("ci-pipeline", [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/build/**", "permission": "write"},
    {"pattern": "/dist/**", "permission": "write"},
    {"pattern": "**/node_modules/**", "permission": "none"},
    {"pattern": "**/.env*", "permission": "none"},
])

# Use it like built-in presets
with Sandbox.from_local("./app", preset="ci-pipeline") as sandbox:
    sandbox.run("npm run build")
```

## Delta Layer (Copy-On-Write)

**The Delta Layer provides write isolation** when multiple Sandboxes share the same Codebase.

### Problem: Write Conflicts

Without isolation, multiple Sandboxes writing to the same Codebase would conflict:

```
Sandbox A writes /workspace/output.txt → overwrites
Sandbox B writes /workspace/output.txt → conflict!
```

### Solution: Per-Sandbox Delta Layer

Each Sandbox writes to its own **delta directory**:

```
Codebase (immutable):    /var/lib/agentfense/codebases/cb_123/
Sandbox A delta:         /var/lib/agentfense/mounts/sb_A/delta/
Sandbox B delta:         /var/lib/agentfense/mounts/sb_B/delta/
```

### How It Works

1. **Read**: Check delta first, fallback to source Codebase
2. **Write**: Always write to delta (source remains untouched)
3. **Delete**: Create whiteout marker (`.wh.<filename>`)
4. **Sync**: On exec completion, delta syncs to source (Last-Writer-Wins)

### Behavior

| Operation | What Happens |
|-----------|--------------|
| Read `/workspace/file.txt` | If exists in delta, read from delta; else read from codebase |
| Write `/workspace/file.txt` | Write to `delta/file.txt` (codebase untouched) |
| Delete `/workspace/file.txt` | Create `delta/.wh.file.txt` (whiteout marker) |
| Exec completes | Sync delta → codebase (LWW if conflicts) |

### Example

```python
from agentfense import Sandbox

# Two sandboxes share the same codebase
with Sandbox.from_local("./project") as sandbox_a:
    sandbox_a.run("echo A > /workspace/output.txt")
    
with Sandbox.from_local("./project") as sandbox_b:
    sandbox_b.run("echo B > /workspace/output.txt")

# After both complete, codebase has:
# - output.txt with content "B" (Last-Writer-Wins)
```

### When to Care About Delta Layer

- **Multi-sandbox concurrency**: Running multiple agents on the same codebase
- **Write safety**: Ensuring sandboxes don't corrupt each other's data
- **Rollback**: Delta changes are ephemeral (lost if sandbox crashes)

### Disabling Sync (Ephemeral Writes)

If you don't want writes to persist to the codebase:

```python
# Option 1: Use read-only preset
with Sandbox.from_local("./project", preset="read-only") as sandbox:
    # All writes fail (permission denied)
    pass

# Option 2: Allow writes but don't sync (custom server config)
# Set sync_on_completion=false in server config
```

## Summary

| Concept | What It Is | Key Insight |
|---------|-----------|-------------|
| **Codebase** | File storage on server | Immutable source of truth, shareable |
| **Sandbox** | Isolated execution environment | Enforces permissions at filesystem level |
| **Permission** | Access control rules | Four levels (none/view/read/write) with priority |
| **Session** | Persistent shell process | Preserves working directory and environment |
| **Preset** | Pre-configured permission template | Reusable patterns for common scenarios |
| **Delta Layer** | Per-sandbox write isolation | COW with sync-on-completion (LWW) |

## Next Steps

- **[Installation Guide](installation.md)** - Set up server and dependencies
- **[Security Model](../security/index.md)** - How permissions are enforced
- **[Permission System](../security/permissions.md)** - Deep dive into rules and patterns
- **[Presets Guide](../security/presets.md)** - Using and extending presets
- **[Best Practices](../security/best-practices.md)** - Production deployment patterns
