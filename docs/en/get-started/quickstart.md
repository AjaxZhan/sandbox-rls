# Quick Start

Get up and running with AgentFense in **5 minutes**. This guide shows you how to create your first sandbox, execute commands with permission control, and clean up resources.

## What You'll Learn

- Install AgentFense server and Python SDK
- Create a sandbox from a local directory in **one line**
- Execute commands with automatic permission enforcement
- Use context managers for automatic cleanup

## Prerequisites

- **Python 3.8+** installed
- **Go 1.21+** (for server)
- **Linux or macOS** (Windows WSL2 supported)
- **bubblewrap** (`bwrap`) or **Docker** for isolation

## Step 1: Install Server

### Option A: Binary Release (Recommended)

```bash
# Download and extract (replace VERSION with latest)
wget https://github.com/AjaxZhan/AgentFense/releases/download/VERSION/agentfense-server-linux-amd64.tar.gz
tar -xzf agentfense-server-linux-amd64.tar.gz
cd agentfense-server

# Start server (gRPC :9000, REST :8080)
./agentfense-server -config config.yaml
```

### Option B: Build from Source

```bash
git clone https://github.com/AjaxZhan/AgentFense.git
cd AgentFense

# Install dependencies and build
go mod tidy
go build -o bin/agentfense-server ./cmd/agentfense-server

# Start server
./bin/agentfense-server -config configs/agentfense-server.yaml
```

### Option C: Docker (Quick Test)

```bash
docker run -d \
  -p 9000:9000 -p 8080:8080 \
  -v /tmp/agentfense:/var/lib/agentfense \
  --privileged \
  agentfense/agentfense-server:latest
```

**Verify server is running:**

```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

## Step 2: Install Python SDK

```bash
pip install agentfense
```

**Verify installation:**

```bash
python -c "from agentfense import Sandbox; print('AgentFense SDK installed')"
```

## Step 3: Create Your First Sandbox

Create a test project:

```bash
mkdir my-project
cd my-project
echo "print('Hello from AgentFense')" > hello.py
echo "SECRET_KEY=my-secret" > .env
```

**Run your first sandbox (one-liner):**

```python
from agentfense import Sandbox

# One line: create sandbox with "agent-safe" preset
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python hello.py")
    print(result.stdout)  # Output: Hello from AgentFense
```

**What just happened?**

1. `Sandbox.from_local()` uploaded files to server
2. Created sandbox with `agent-safe` preset (hides `.env`, allows read all, write to `/output` and `/tmp`)
3. Started the sandbox
4. Executed `python hello.py` inside isolated environment
5. Automatically cleaned up on exit

## Step 4: Test Permission Control

Try to access the hidden secret file:

```python
from agentfense import Sandbox

with Sandbox.from_local("./my-project") as sandbox:
    # This works - hello.py is readable
    result = sandbox.run("cat /workspace/hello.py")
    print(result.stdout)  # Output: print('Hello from AgentFense')
    
    # This fails - .env is hidden (permission: none)
    result = sandbox.run("cat /workspace/.env")
    print(result.exit_code)  # Output: 1 (file not found)
    print(result.stderr)     # Output: cat: /workspace/.env: No such file or directory
    
    # Even ls cannot see it
    result = sandbox.run("ls -la /workspace/")
    print(result.stdout)  # Output: hello.py (no .env listed)
```

**Key insight:** The `.env` file is **invisible at the filesystem level**. The agent cannot access it even if it tries—no additional logic needed.

## Step 5: Custom Permissions

Define your own permission rules:

```python
from agentfense import Sandbox

# Custom permissions
PERMISSIONS = [
    {"pattern": "**/*", "permission": "read"},         # Default: read-only
    {"pattern": "/output/**", "permission": "write"},  # Allow writes to /output
    {"pattern": "**/.env*", "permission": "none"},     # Hide all .env files
    {"pattern": "/docs/**", "permission": "view"},     # Can list but not read
]

with Sandbox.from_local("./my-project", permissions=PERMISSIONS) as sandbox:
    # Read any file
    result = sandbox.run("cat hello.py")
    
    # Write to /output
    result = sandbox.run("echo 'result' > /workspace/output/result.txt")
    assert result.exit_code == 0
    
    # Cannot write elsewhere
    result = sandbox.run("echo 'test' > /workspace/data.txt")
    assert result.exit_code != 0  # Permission denied
```

## Step 6: Use Sessions (Persistent Shell)

For multi-step workflows, use sessions to preserve working directory and environment:

```python
from agentfense import Sandbox

with Sandbox.from_local("./my-project") as sandbox:
    # Create a persistent session
    with sandbox.session() as session:
        session.exec("cd /workspace")
        session.exec("export DEBUG=1")
        session.exec("source venv/bin/activate")  # If venv exists
        
        # All previous state is preserved
        result = session.exec("python hello.py")
        print(result.stdout)
```

## Step 7: Docker Runtime + Resource Limits

For stronger isolation and resource control:

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./my-project",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB
        pids_limit=100,                  # Max 100 processes
    ),
) as sandbox:
    result = sandbox.run("python hello.py")
    print(result.stdout)
```

## Complete Example: AI Agent with Permission Control

Build a secure AI agent that executes bash commands:

```python
from anthropic import Anthropic
from agentfense import Sandbox
import re

client = Anthropic()

PERMISSIONS = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/output/**", "permission": "write"},
    {"pattern": "**/.env*", "permission": "none"},
]

def extract_bash_commands(text: str) -> list:
    """Extract commands from ```bash``` blocks"""
    return re.findall(r'```bash\n(.*?)```', text, re.DOTALL)

with Sandbox.from_local("./my-project", permissions=PERMISSIONS) as sandbox:
    response = client.messages.create(
        model="claude-sonnet-4-20250514",
        messages=[{"role": "user", "content": "List all Python files"}],
        system="Output bash commands in ```bash``` blocks."
    )
    
    commands = extract_bash_commands(response.content[0].text)
    for cmd in commands:
        result = sandbox.run(cmd.strip(), timeout=10)
        print(f"$ {cmd}\n{result.stdout}")
```

The agent **cannot access `.env`** even if it generates `cat .env`—the file is invisible at the filesystem level.

## Next Steps

Now that you have a working sandbox, explore:

- **[Core Concepts](concepts.md)** - Understand codebases, permissions, sessions, and delta layer
- **[Installation Guide](installation.md)** - Detailed server setup, dependencies, configuration
- **[Security Model](../security/index.md)** - How AgentFense enforces permissions
- **[Permission System](../security/permissions.md)** - Deep dive into four permission levels
- **[Best Practices](../security/best-practices.md)** - Production deployment and security patterns

## Troubleshooting

### Server connection refused

**Problem:** `ConnectionError: failed to connect to localhost:9000`

**Solution:** Ensure server is running: `./agentfense-server -config config.yaml`

### Command exits with permission denied

**Problem:** `Permission denied` errors when writing files

**Solution:** Check permission rules—files default to read-only unless explicitly granted `write` permission. Use `extend_preset()` or add explicit write rules.

### .env file still visible

**Problem:** Secret files appear in `ls` output

**Solution:** Ensure pattern matches correctly. Use `**/.env*` for all directories or `/path/to/.env` for specific files. Priority rules apply—more specific patterns override general ones.

### Docker runtime fails on macOS

**Problem:** `view` permission doesn't work correctly

**Solution:** This is a known limitation with Docker Desktop on macOS (VirtioFS issue). Use `read` instead of `view`, or use `bwrap` runtime on Linux.

## Quick Reference

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

# Basic usage
with Sandbox.from_local("./path") as sb:
    result = sb.run("command")

# Custom permissions
with Sandbox.from_local("./path", permissions=[...]) as sb:
    result = sb.run("command")

# Docker + resource limits
with Sandbox.from_local(
    "./path",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(memory_bytes=512*1024*1024),
) as sb:
    result = sb.run("command")

# Sessions (persistent shell)
with sandbox.session() as session:
    session.exec("cd /workspace")
    result = session.exec("command")

# Error handling
result = sandbox.run("command", timeout=30, raise_on_error=True)
```
