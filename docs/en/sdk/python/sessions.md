# Sessions Guide

Sessions provide **stateful command execution** in AgentFense sandboxes. This guide explains what sessions are, why they're essential, and how to use them effectively.

---

## Table of Contents

- [What is a Session?](#what-is-a-session)
- [Why Use Sessions?](#why-use-sessions)
- [Creating Sessions](#creating-sessions)
- [Session State Management](#session-state-management)
- [Common Use Cases](#common-use-cases)
- [Session vs Direct Execution](#session-vs-direct-execution)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

---

## What is a Session?

A **Session** is a persistent shell process running inside a sandbox that **preserves state** across multiple command executions.

### Without Session (Direct Execution)

```python
sandbox.run("cd /workspace/backend")  # Shell process #1
sandbox.run("source venv/bin/activate")  # Shell process #2 (fresh start)
sandbox.run("python manage.py test")  # Shell process #3 (fresh start)
# ❌ Each command runs in a NEW shell - no state preserved
```

**Result**: The `cd` and `source` commands have **no effect** on subsequent commands because each `run()` starts a fresh shell.

---

### With Session (Stateful Execution)

```python
with sandbox.session() as session:
    session.exec("cd /workspace/backend")  # Shell process persists
    session.exec("source venv/bin/activate")  # Same shell, virtualenv activated
    session.exec("python manage.py test")  # Same shell, still in backend/ with venv
# ✓ All commands run in the SAME shell - state persists
```

**Result**: Each command builds on the previous state. The final `python` command runs in `/workspace/backend/` with the virtualenv activated.

---

## Why Use Sessions?

Sessions are **required** when you need to:

1. **Change working directory (`cd`)**
   ```python
   with sandbox.session() as session:
       session.exec("cd /workspace/src")
       session.exec("pytest")  # Runs in /workspace/src
   ```

2. **Set environment variables (`export`)**
   ```python
   with sandbox.session() as session:
       session.exec("export DEBUG=1")
       session.exec("python app.py")  # DEBUG=1 is set
   ```

3. **Activate virtual environments (`source`)**
   ```python
   with sandbox.session() as session:
       session.exec("source venv/bin/activate")
       session.exec("pip list")  # Shows venv packages
   ```

4. **Run background processes**
   ```python
   with sandbox.session() as session:
       session.exec("redis-server &")  # Starts background server
       session.exec("python test.py")  # Can connect to redis
   ```

5. **Multi-step workflows with dependencies**
   ```python
   with sandbox.session() as session:
       session.exec("npm install")  # Install dependencies
       session.exec("npm run build")  # Uses installed deps
       session.exec("npm test")  # Uses built artifacts
   ```

---

## Creating Sessions

### Sync API

```python
from agentfense import Sandbox

with Sandbox.from_local("./project") as sandbox:
    with sandbox.session() as session:
        result = session.exec("pwd")
        print(result.stdout)  # /workspace
```

---

### Async API

```python
from agentfense import AsyncSandbox

async with await AsyncSandbox.from_local("./project") as sandbox:
    async with await sandbox.session() as session:
        result = await session.exec("pwd")
        print(result.stdout)  # /workspace
```

---

### Session Parameters

```python
def session(
    self,
    shell: str = "/bin/sh",
    env: Optional[Dict[str, str]] = None,
) -> SessionWrapper
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `shell` | `str` | `"/bin/sh"` | Shell binary to use (`/bin/sh`, `/bin/bash`, `/bin/zsh`, etc.) |
| `env` | `Dict[str, str]` | `None` | Initial environment variables |

**Example: Custom Shell**
```python
with sandbox.session(shell="/bin/bash") as session:
    # Bash-specific syntax (arrays, etc.)
    session.exec("declare -a arr=(1 2 3)")
```

**Example: Initial Environment**
```python
with sandbox.session(env={"DEBUG": "1", "ENV": "test"}) as session:
    result = session.exec("printenv DEBUG")
    print(result.stdout)  # 1
```

---

## Session State Management

### Working Directory

The current working directory (`cwd`) persists across commands:

```python
with sandbox.session() as session:
    session.exec("cd /workspace/backend")
    result = session.exec("pwd")
    print(result.stdout)  # /workspace/backend
    
    session.exec("cd ../frontend")
    result = session.exec("pwd")
    print(result.stdout)  # /workspace/frontend
```

---

### Environment Variables

Environment variables set with `export` persist:

```python
with sandbox.session() as session:
    session.exec("export API_URL=http://localhost:8000")
    session.exec("export DB_NAME=testdb")
    
    result = session.exec("printenv API_URL")
    print(result.stdout)  # http://localhost:8000
```

---

### Aliases and Functions

Shell aliases and functions are preserved:

```python
with sandbox.session(shell="/bin/bash") as session:
    session.exec("alias ll='ls -la'")
    result = session.exec("ll")
    print(result.stdout)  # Runs ls -la
    
    session.exec("myfunction() { echo 'Hello from function'; }")
    result = session.exec("myfunction")
    print(result.stdout)  # Hello from function
```

---

### Background Processes

Background processes (`&`) continue running:

```python
with sandbox.session() as session:
    # Start a background server
    session.exec("python -m http.server 8000 &")
    
    # Wait for server to start
    import time
    time.sleep(2)
    
    # Server is still running
    result = session.exec("curl http://localhost:8000")
    print(result.stdout)  # HTML directory listing
```

**Important**: Background processes are killed when the session closes.

---

## Common Use Cases

### Use Case 1: Python Virtual Environment

```python
with sandbox.session() as session:
    # Create virtualenv
    session.exec("python -m venv venv")
    
    # Activate it
    session.exec("source venv/bin/activate")
    
    # Install dependencies
    session.exec("pip install pytest requests")
    
    # Run tests (using venv's pytest)
    result = session.exec("pytest tests/")
    print(result.exit_code)
```

---

### Use Case 2: Node.js Project Setup

```python
with sandbox.session() as session:
    # Install dependencies
    session.exec("cd /workspace")
    session.exec("npm install")
    
    # Run linter
    lint_result = session.exec("npm run lint")
    
    # Run tests
    test_result = session.exec("npm test")
    
    # Build production bundle
    if test_result.exit_code == 0:
        session.exec("npm run build")
```

---

### Use Case 3: Database Setup for Tests

```python
with sandbox.session() as session:
    # Start database in background
    session.exec("mysqld --datadir=/tmp/mysql &")
    
    # Wait for startup
    import time
    time.sleep(3)
    
    # Run migrations
    session.exec("python manage.py migrate")
    
    # Run tests
    result = session.exec("python manage.py test")
```

---

### Use Case 4: Multi-Stage CI Pipeline

```python
def ci_pipeline(sandbox):
    """Run full CI pipeline in one session."""
    with sandbox.session() as session:
        # Stage 1: Setup
        session.exec("cd /workspace")
        session.exec("export CI=true")
        
        # Stage 2: Install
        result = session.exec("pip install -r requirements.txt")
        if result.exit_code != 0:
            return False, "Install failed"
        
        # Stage 3: Lint
        result = session.exec("flake8 .")
        if result.exit_code != 0:
            return False, "Linting failed"
        
        # Stage 4: Test
        result = session.exec("pytest --cov")
        if result.exit_code != 0:
            return False, "Tests failed"
        
        # Stage 5: Build
        result = session.exec("python setup.py bdist_wheel")
        if result.exit_code != 0:
            return False, "Build failed"
        
        return True, "Pipeline succeeded"

# Usage
with Sandbox.from_local("./project") as sandbox:
    success, message = ci_pipeline(sandbox)
    print(message)
```

---

### Use Case 5: Interactive Debugging

```python
with sandbox.session() as session:
    # Set up environment
    session.exec("cd /workspace")
    session.exec("source .env")
    
    # Run debugger commands
    session.exec("python -m pdb main.py")
    session.exec("break main.py:42")
    session.exec("continue")
    
    # Inspect variables
    result = session.exec("print(local_vars)")
    print(result.stdout)
```

---

## Session vs Direct Execution

### When to Use Direct Execution (`run()` / `exec()`)

✅ **One-off commands**: Single independent commands  
✅ **No state needed**: Command doesn't depend on previous state  
✅ **Maximum isolation**: Each command starts fresh  
✅ **Simplicity**: No need to manage session lifecycle  

**Example**:
```python
# These are independent - no session needed
sandbox.run("python --version")
sandbox.run("ls -la")
sandbox.run("cat README.md")
```

---

### When to Use Sessions (`session()`)

✅ **Working directory changes**: Need `cd` to persist  
✅ **Environment setup**: Need `export`, `source`, `alias`  
✅ **Virtualenv/tools**: Activating Python venv, Node.js NVM, etc.  
✅ **Background services**: Start server, run tests against it  
✅ **Multi-step workflows**: Each step builds on previous  

**Example**:
```python
# These depend on each other - session required
with sandbox.session() as session:
    session.exec("cd backend/")
    session.exec("source venv/bin/activate")
    session.exec("pytest")
```

---

### Performance Comparison

| Aspect | Direct Execution | Session |
|--------|------------------|---------|
| **Startup overhead** | ~10-50ms per command | ~50ms once, then ~5-20ms per command |
| **State isolation** | Complete (new shell each time) | Shared (same shell) |
| **Best for** | Independent commands | Dependent workflows |

**Recommendation**: Use sessions for 3+ dependent commands. For 1-2 independent commands, use direct execution.

---

## Best Practices

### 1. Always Use Context Manager

```python
# ✓ Good: Auto cleanup
with sandbox.session() as session:
    session.exec("command")
# Session automatically closed

# ✗ Bad: Manual cleanup (easy to forget)
session = sandbox.session()
session.exec("command")
session.close()  # Easy to forget
```

---

### 2. Check Exit Codes

```python
with sandbox.session() as session:
    result = session.exec("make build")
    if result.exit_code != 0:
        print(f"Build failed: {result.stderr}")
        return  # Exit early on failure
    
    # Only continue if build succeeded
    session.exec("make test")
```

---

### 3. Set Environment Early

```python
# ✓ Good: Set env at session creation
with sandbox.session(env={"CI": "1", "DEBUG": "0"}) as session:
    session.exec("pytest")

# ✗ Less efficient: Set env per command
with sandbox.session() as session:
    session.exec("export CI=1")  # Extra command
    session.exec("pytest")
```

---

### 4. Use Absolute Paths When Possible

```python
# ✓ Good: Absolute path (unambiguous)
with sandbox.session() as session:
    session.exec("python /workspace/scripts/test.py")

# ✗ Risky: Relative path (depends on cwd)
with sandbox.session() as session:
    session.exec("python scripts/test.py")  # Fails if cwd != /workspace
```

---

### 5. Limit Session Scope

```python
# ✓ Good: One session per logical workflow
with sandbox.session() as session:
    # Backend tests
    session.exec("cd /workspace/backend")
    session.exec("pytest")

with sandbox.session() as session:
    # Frontend tests (fresh session)
    session.exec("cd /workspace/frontend")
    session.exec("npm test")

# ✗ Bad: Reuse session for unrelated workflows
with sandbox.session() as session:
    session.exec("cd /workspace/backend")
    session.exec("pytest")
    session.exec("cd /workspace/frontend")  # Risk of cross-contamination
    session.exec("npm test")
```

---

## Troubleshooting

### Problem: `cd` Doesn't Persist

**Symptom**:
```python
sandbox.run("cd /workspace/src")
result = sandbox.run("pwd")
print(result.stdout)  # /workspace (not /workspace/src)
```

**Solution**: Use a session:
```python
with sandbox.session() as session:
    session.exec("cd /workspace/src")
    result = session.exec("pwd")
    print(result.stdout)  # /workspace/src
```

---

### Problem: Environment Variable Not Set

**Symptom**:
```python
sandbox.run("export DEBUG=1")
result = sandbox.run("printenv DEBUG")
print(result.stdout)  # (empty)
```

**Solution**: Use a session or pass `env`:
```python
# Option 1: Session
with sandbox.session() as session:
    session.exec("export DEBUG=1")
    result = session.exec("printenv DEBUG")
    print(result.stdout)  # 1

# Option 2: Pass env parameter
result = sandbox.run("printenv DEBUG", env={"DEBUG": "1"})
print(result.stdout)  # 1
```

---

### Problem: Background Process Terminates

**Symptom**:
```python
with sandbox.session() as session:
    session.exec("redis-server &")
# Session closed, redis-server killed
```

**Solution**: Keep session open as long as you need the background process:
```python
with sandbox.session() as session:
    session.exec("redis-server &")
    time.sleep(2)  # Wait for startup
    
    # Use the server while session is open
    result = session.exec("redis-cli ping")
    print(result.stdout)  # PONG
# Now session closes and kills background processes
```

---

### Problem: Session Hangs on Interactive Command

**Symptom**:
```python
with sandbox.session() as session:
    session.exec("python")  # Hangs waiting for input
```

**Solution**: Avoid interactive commands or use stdin:
```python
# Option 1: Non-interactive flag
session.exec("python -c 'print(1+1)'")

# Option 2: Use stdin (not supported in sessions, use direct exec)
result = sandbox.exec("python", stdin="print(1+1)\nexit()\n")
```

---

## See Also

- [High-Level API Guide](./high-level.md) - `Sandbox` class reference
- [Async API Guide](./async.md) - Async session usage
- [Best Practices](../best-practices.md) - General SDK recommendations
