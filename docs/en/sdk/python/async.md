# Async API Guide

The `AsyncSandbox` and `AsyncSandboxClient` classes provide fully async/await compatible APIs for concurrent sandbox execution. This guide covers async patterns, performance considerations, and best practices.

---

## Table of Contents

- [When to Use Async](#when-to-use-async)
- [AsyncSandbox Overview](#asyncsandbox-overview)
- [Creating Async Sandboxes](#creating-async-sandboxes)
- [Concurrent Execution Patterns](#concurrent-execution-patterns)
- [Async Sessions](#async-sessions)
- [AsyncSandboxClient (Low-Level)](#asyncsandboxclient-low-level)
- [Performance Comparison](#performance-comparison)
- [Best Practices](#best-practices)
- [Migration from Sync](#migration-from-sync)

---

## When to Use Async

### Use Async When:

✅ **High concurrency**: Running 10+ sandboxes simultaneously  
✅ **I/O-bound workloads**: Most time spent waiting for network/disk  
✅ **Async framework integration**: FastAPI, aiohttp, asyncio-based services  
✅ **Long-running commands**: Want to start multiple commands and wait for all  
✅ **Resource efficiency**: Need to handle 100+ concurrent sandboxes without spawning 100+ threads  

### Stay with Sync When:

❌ **Simple scripts**: Sequential execution of 1-5 sandboxes  
❌ **CPU-bound workloads**: Async doesn't help (GIL limits Python parallelism)  
❌ **Sync framework**: Flask, Django, or synchronous codebase  
❌ **Team unfamiliarity**: Async/await adds complexity if team isn't comfortable  

---

## AsyncSandbox Overview

`AsyncSandbox` mirrors the sync `Sandbox` API but with async/await:

| Sync API | Async API | Difference |
|----------|-----------|------------|
| `Sandbox.from_local()` | `await AsyncSandbox.from_local()` | Awaitable classmethod |
| `sandbox.run()` | `await sandbox.run()` | Awaitable method |
| `with sandbox:` | `async with sandbox:` | Async context manager |
| `sandbox.session()` | `await sandbox.session()` | Returns AsyncSessionWrapper |
| `with sandbox.session():` | `async with session:` | Async context manager |

**Key Difference**: All I/O operations are awaitable, allowing the event loop to handle other tasks while waiting.

---

## Creating Async Sandboxes

### `AsyncSandbox.from_local()`

```python
@classmethod
async def from_local(
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
) -> "AsyncSandbox"
```

**Example**:
```python
import asyncio
from agentfense import AsyncSandbox

async def main():
    # Create async sandbox
    async with await AsyncSandbox.from_local("./project") as sandbox:
        result = await sandbox.run("pytest")
        print(result.stdout)

asyncio.run(main())
```

### `AsyncSandbox.from_codebase()`

```python
async with await AsyncSandbox.from_codebase(
    codebase_id="cb_xyz123",
    preset="agent-safe",
) as sandbox:
    result = await sandbox.run("make test")
```

### `AsyncSandbox.connect()`

```python
# Reconnect to existing sandbox
sandbox = await AsyncSandbox.connect("sb_abc123")
result = await sandbox.run("whoami")
await sandbox.destroy()
```

---

## Concurrent Execution Patterns

### Pattern 1: Run Multiple Sandboxes in Parallel

**Problem**: Execute the same command across 10 different projects.

```python
import asyncio
from agentfense import AsyncSandbox

async def test_project(project_path: str) -> tuple[str, bool]:
    """Run tests in a project and return (name, success)."""
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        result = await sandbox.run("pytest", timeout=300)
        return project_path, result.exit_code == 0

async def test_all_projects():
    """Test 10 projects concurrently."""
    projects = [f"./project-{i}" for i in range(10)]
    
    # Run all in parallel
    tasks = [test_project(p) for p in projects]
    results = await asyncio.gather(*tasks)
    
    # Print results
    for project, success in results:
        status = "✓ PASS" if success else "✗ FAIL"
        print(f"{status} {project}")

asyncio.run(test_all_projects())
```

**Why Async**: Instead of 10 * 60s = 600s sequential, runs in ~60s (all parallel).

---

### Pattern 2: Fan-Out with Different Commands

**Problem**: Run different analysis tools on the same codebase concurrently.

```python
async def run_analysis(project_path: str) -> dict:
    """Run multiple analysis tools in parallel."""
    
    async def pylint_check(sandbox):
        result = await sandbox.run("pylint .", timeout=120)
        return "pylint", result.stdout
    
    async def mypy_check(sandbox):
        result = await sandbox.run("mypy .", timeout=120)
        return "mypy", result.stdout
    
    async def pytest_coverage(sandbox):
        result = await sandbox.run("pytest --cov", timeout=300)
        return "coverage", result.stdout
    
    # Create sandbox once
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        # Run all checks concurrently
        tasks = [
            pylint_check(sandbox),
            mypy_check(sandbox),
            pytest_coverage(sandbox),
        ]
        results = await asyncio.gather(*tasks)
        
        return {tool: output for tool, output in results}

# Usage
results = asyncio.run(run_analysis("./my-project"))
print(results["pylint"])
```

---

### Pattern 3: Concurrent File Operations

**Problem**: Download multiple files from a sandbox in parallel.

```python
async def download_outputs(sandbox: AsyncSandbox, file_paths: list[str]) -> dict[str, str]:
    """Download multiple files concurrently."""
    
    async def download_one(path: str) -> tuple[str, str]:
        content = await sandbox.read_file(path)
        return path, content
    
    tasks = [download_one(path) for path in file_paths]
    results = await asyncio.gather(*tasks)
    
    return {path: content for path, content in results}

# Usage
async with await AsyncSandbox.from_local("./project") as sandbox:
    await sandbox.run("./generate-reports.sh")
    
    files_to_download = [
        "/workspace/output/report1.txt",
        "/workspace/output/report2.txt",
        "/workspace/output/report3.txt",
    ]
    
    contents = await download_outputs(sandbox, files_to_download)
    for path, content in contents.items():
        print(f"{path}: {len(content)} bytes")
```

---

### Pattern 4: Pipeline with Dependencies

**Problem**: Run stages where stage N depends on stage N-1.

```python
async def ci_pipeline(project_path: str) -> bool:
    """Run CI pipeline stages sequentially within one sandbox."""
    async with await AsyncSandbox.from_local(
        project_path,
        preset="development",
        runtime=RuntimeType.DOCKER,
        image="python:3.11-slim",
    ) as sandbox:
        async with await sandbox.session() as session:
            # Stage 1: Install dependencies
            result = await session.exec("pip install -r requirements.txt")
            if result.exit_code != 0:
                print("Dependency install failed")
                return False
            
            # Stage 2: Linting (runs after install)
            result = await session.exec("flake8 .")
            if result.exit_code != 0:
                print("Linting failed")
                return False
            
            # Stage 3: Tests (runs after linting)
            result = await session.exec("pytest --cov")
            if result.exit_code != 0:
                print("Tests failed")
                return False
            
            # Stage 4: Build (runs after tests)
            result = await session.exec("python setup.py bdist_wheel")
            return result.exit_code == 0

# Usage
success = asyncio.run(ci_pipeline("./my-package"))
```

**Note**: This is sequential (no parallelism within one pipeline), but you can run **multiple pipelines** in parallel:

```python
async def run_pipelines():
    projects = ["./project-A", "./project-B", "./project-C"]
    tasks = [ci_pipeline(p) for p in projects]
    results = await asyncio.gather(*tasks)  # 3 pipelines in parallel
    return results

results = asyncio.run(run_pipelines())
```

---

### Pattern 5: Timeout with Cancellation

**Problem**: Run multiple sandboxes but cancel all if any fails or takes too long.

```python
async def test_with_timeout(project_path: str, timeout: int) -> str:
    """Run tests with global timeout."""
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        result = await sandbox.run("pytest", timeout=timeout)
        if result.exit_code != 0:
            raise Exception(f"Tests failed in {project_path}")
        return f"{project_path}: PASS"

async def test_all_with_global_timeout():
    """Run all tests, but cancel if any fails or global timeout."""
    projects = [f"./project-{i}" for i in range(5)]
    
    try:
        # 5-minute global timeout
        results = await asyncio.wait_for(
            asyncio.gather(*[test_with_timeout(p, 60) for p in projects]),
            timeout=300
        )
        print("All tests passed:", results)
    except asyncio.TimeoutError:
        print("Global timeout exceeded!")
    except Exception as e:
        print(f"Test failure: {e}")

asyncio.run(test_all_with_global_timeout())
```

---

## Async Sessions

### Creating Async Sessions

```python
async with await AsyncSandbox.from_local("./project") as sandbox:
    async with await sandbox.session() as session:
        await session.exec("cd /workspace")
        await session.exec("npm install")
        result = await session.exec("npm test")
```

### Parallel Commands in Separate Sessions

**Problem**: Run independent stateful workflows in parallel.

```python
async def workflow_A(sandbox: AsyncSandbox):
    async with await sandbox.session() as session:
        await session.exec("cd /workspace/backend")
        await session.exec("pytest backend/")

async def workflow_B(sandbox: AsyncSandbox):
    async with await sandbox.session() as session:
        await session.exec("cd /workspace/frontend")
        await session.exec("npm test")

# Run both workflows concurrently (different sessions)
async with await AsyncSandbox.from_local("./monorepo") as sandbox:
    await asyncio.gather(
        workflow_A(sandbox),
        workflow_B(sandbox),
    )
```

**Important**: Each workflow has its own session (separate shell process), so state doesn't conflict.

---

## AsyncSandboxClient (Low-Level)

For fine-grained control, use `AsyncSandboxClient` directly.

### Example: Reuse Codebase Across Async Sandboxes

```python
from agentfense import AsyncSandboxClient, RuntimeType

async def multi_sandbox_analysis(project_path: str):
    """Create one codebase, multiple sandboxes."""
    async with AsyncSandboxClient(endpoint="localhost:9000") as client:
        # Create and upload codebase once
        codebase = await client.create_codebase(name="shared", owner_id="team")
        
        # Upload files (sync walk_directory, async upload)
        from agentfense.utils import walk_directory
        for rel_path, content in walk_directory(project_path):
            await client.upload_file(codebase.id, rel_path, content)
        
        # Create multiple sandboxes from same codebase
        sandbox1 = await client.create_sandbox(
            codebase.id,
            permissions=[{"pattern": "**/*", "permission": "read"}],
            runtime=RuntimeType.BWRAP,
        )
        
        sandbox2 = await client.create_sandbox(
            codebase.id,
            permissions=[{"pattern": "**/*", "permission": "write"}],
            runtime=RuntimeType.DOCKER,
            image="python:3.11-slim",
        )
        
        # Start both sandboxes
        await asyncio.gather(
            client.start_sandbox(sandbox1.id),
            client.start_sandbox(sandbox2.id),
        )
        
        # Run commands concurrently
        results = await asyncio.gather(
            client.exec(sandbox1.id, command="pylint ."),
            client.exec(sandbox2.id, command="pytest --cov"),
        )
        
        # Cleanup
        await asyncio.gather(
            client.destroy_sandbox(sandbox1.id),
            client.destroy_sandbox(sandbox2.id),
        )
        await client.delete_codebase(codebase.id)
        
        return results

results = asyncio.run(multi_sandbox_analysis("./project"))
```

---

## Performance Comparison

### Benchmark: 10 Sandboxes Running 10-Second Command

**Sync (Sequential)**:
```python
import time
from agentfense import Sandbox

start = time.time()
results = []
for i in range(10):
    with Sandbox.from_local("./project") as sandbox:
        result = sandbox.run("sleep 10")
        results.append(result)
elapsed = time.time() - start
print(f"Time: {elapsed:.1f}s")  # ~100s (10 * 10s)
```

**Async (Parallel)**:
```python
import time
import asyncio
from agentfense import AsyncSandbox

async def run_one():
    async with await AsyncSandbox.from_local("./project") as sandbox:
        return await sandbox.run("sleep 10")

async def main():
    start = time.time()
    tasks = [run_one() for _ in range(10)]
    results = await asyncio.gather(*tasks)
    elapsed = time.time() - start
    print(f"Time: {elapsed:.1f}s")  # ~10s (all parallel)
    return results

asyncio.run(main())
```

**Speedup**: ~10x (10 sandboxes run concurrently instead of sequentially).

---

### Scalability: 100 Concurrent Sandboxes

Async can handle 100+ concurrent sandboxes with minimal memory overhead:

```python
async def run_100_sandboxes():
    """Run 100 sandboxes concurrently."""
    async def run_one(i: int):
        async with await AsyncSandbox.from_local(f"./project-{i}") as sandbox:
            return await sandbox.run("echo 'done'")
    
    tasks = [run_one(i) for i in range(100)]
    results = await asyncio.gather(*tasks)
    print(f"Completed {len(results)} sandboxes")

asyncio.run(run_100_sandboxes())
```

**Memory Usage**: ~50-100 MB (async overhead minimal compared to thread-per-sandbox).

---

## Best Practices

### 1. Use `asyncio.gather()` for Parallel Execution

```python
# ✓ Good: Run in parallel
tasks = [sandbox.run(cmd) for cmd in commands]
results = await asyncio.gather(*tasks)

# ✗ Bad: Sequential (defeats async purpose)
results = []
for cmd in commands:
    result = await sandbox.run(cmd)
    results.append(result)
```

---

### 2. Handle Exceptions in `gather()`

```python
# ✓ Good: Catch exceptions per task
tasks = [run_test(project) for project in projects]
results = await asyncio.gather(*tasks, return_exceptions=True)

for project, result in zip(projects, results):
    if isinstance(result, Exception):
        print(f"{project} failed: {result}")
    else:
        print(f"{project} succeeded")

# ✗ Bad: One failure cancels all
try:
    results = await asyncio.gather(*tasks)  # First exception stops all
except Exception as e:
    print(f"Failed: {e}")  # Don't know which task failed
```

---

### 3. Use Semaphores to Limit Concurrency

```python
async def run_with_limit(projects: list[str], max_concurrent: int = 5):
    """Run sandboxes with max concurrency limit."""
    semaphore = asyncio.Semaphore(max_concurrent)
    
    async def run_one(project: str):
        async with semaphore:  # Only 5 at a time
            async with await AsyncSandbox.from_local(project) as sandbox:
                return await sandbox.run("make test")
    
    tasks = [run_one(p) for p in projects]
    return await asyncio.gather(*tasks)

# Run 100 projects, but only 5 concurrent sandboxes
results = asyncio.run(run_with_limit([f"./p-{i}" for i in range(100)], max_concurrent=5))
```

---

### 4. Close Clients Properly

```python
# ✓ Good: Use context manager
async with AsyncSandboxClient(endpoint="localhost:9000") as client:
    # ... use client ...
# Automatically closed

# ✗ Bad: Manual close (easy to forget)
client = AsyncSandboxClient(endpoint="localhost:9000")
# ... use client ...
await client.close()  # Easy to forget if exception occurs
```

---

### 5. Avoid Mixing Sync and Async

```python
# ✗ Bad: Blocking call in async function
async def bad_example():
    async with await AsyncSandbox.from_local("./project") as sandbox:
        result = await sandbox.run("test.sh")
        
        # Blocking I/O in async context!
        with open("output.txt", "w") as f:  # Blocks event loop
            f.write(result.stdout)

# ✓ Good: Use async file I/O
import aiofiles

async def good_example():
    async with await AsyncSandbox.from_local("./project") as sandbox:
        result = await sandbox.run("test.sh")
        
        async with aiofiles.open("output.txt", "w") as f:
            await f.write(result.stdout)
```

---

## Migration from Sync

Converting sync code to async is straightforward:

### Step 1: Change Imports

```python
# Sync
from agentfense import Sandbox

# Async
from agentfense import AsyncSandbox
```

---

### Step 2: Add `async`/`await`

```python
# Sync
def run_tests(project_path: str):
    with Sandbox.from_local(project_path) as sandbox:
        result = sandbox.run("pytest")
        return result.exit_code == 0

# Async
async def run_tests(project_path: str):
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        result = await sandbox.run("pytest")
        return result.exit_code == 0
```

---

### Step 3: Update Context Managers

```python
# Sync
with sandbox.session() as session:
    session.exec("cd /workspace")

# Async
async with await sandbox.session() as session:
    await session.exec("cd /workspace")
```

---

### Step 4: Use `asyncio.run()` at Entry Point

```python
# Sync
if __name__ == "__main__":
    success = run_tests("./project")
    print(success)

# Async
import asyncio

if __name__ == "__main__":
    success = asyncio.run(run_tests("./project"))
    print(success)
```

---

## See Also

- [High-Level API Guide](./high-level.md) - Sync API reference
- [Sessions Guide](./sessions.md) - Stateful execution
- [Python asyncio docs](https://docs.python.org/3/library/asyncio.html) - Official asyncio guide
