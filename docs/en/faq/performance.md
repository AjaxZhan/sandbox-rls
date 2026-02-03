# Performance

Performance characteristics, benchmarks, and optimization tips.

## Benchmarks

### System Specifications

All benchmarks were run on:
- **CPU:** 2 vCPUs (Intel Xeon)
- **RAM:** 4 GB
- **Disk:** SSD
- **OS:** Ubuntu 22.04 LTS
- **Runtime:** Docker

### Concurrency

| Metric | Value |
|--------|-------|
| Max concurrent sandboxes | 120+ (tested) |
| Recommended max (2 cores/4GB) | 50-80 |
| Memory per sandbox | ~5 MB |
| Processes per sandbox | 2 (FUSE + runtime) |

### Latency

| Operation | Latency (p50) | Latency (p99) |
|-----------|---------------|---------------|
| Create sandbox | 150 ms | 300 ms |
| Start sandbox (bwrap) | 80 ms | 150 ms |
| Start sandbox (Docker) | 250 ms | 500 ms |
| Execute command | 20 ms | 50 ms |
| File read (1KB) | 0.5 ms | 2 ms |
| File read (1MB) | 15 ms | 30 ms |

### Throughput

| Operation | Throughput |
|-----------|------------|
| Command executions/sec | 1000+ |
| File reads/sec (small files) | 5000+ |
| File writes/sec (small files) | 3000+ |

### FUSE Overhead

Compared to native filesystem:

| Operation | Overhead |
|-----------|----------|
| Read (small files) | +3% |
| Read (large files) | +2% |
| Write (small files) | +5% |
| Write (large files) | +4% |
| Metadata (stat, readdir) | +2% |

## Optimization Tips

### 1. Choose the Right Runtime

```python
# Fast startup, lower overhead (Linux only)
runtime=RuntimeType.BWRAP

# Full isolation, cross-platform
runtime=RuntimeType.DOCKER
```

**Recommendation:** Use bwrap for development and high-throughput scenarios. Use Docker for production isolation.

### 2. Reuse Codebases

```python
# Bad: Creates new codebase each time
for i in range(100):
    with Sandbox.from_local("./project") as sb:
        sb.run("test")

# Good: Create codebase once, reuse
client = SandboxClient()
codebase = client.create_codebase("project", "user")
# ... upload files once ...

for i in range(100):
    sandbox = client.create_sandbox(codebase.id, ...)
    # ... use sandbox ...
    client.destroy_sandbox(sandbox.id)
```

### 3. Batch Operations

```python
# Bad: Multiple round-trips
sandbox.exec("mkdir /workspace/output")
sandbox.exec("touch /workspace/output/file1.txt")
sandbox.exec("touch /workspace/output/file2.txt")

# Good: Single command
sandbox.exec("mkdir -p /workspace/output && touch /workspace/output/file{1,2}.txt")

# Better: Use session
with sandbox.session() as session:
    session.exec("mkdir -p /workspace/output")
    session.exec("cd /workspace/output")
    session.exec("touch file1.txt file2.txt")
```

### 4. Limit Resource Usage

```python
from agentfense import ResourceLimits

sandbox = Sandbox.from_local(
    "./project",
    resources=ResourceLimits(
        memory_bytes=256 * 1024 * 1024,  # 256 MB
        cpu_quota=50000,  # 50% of one core
        pids_limit=100,
    )
)
```

### 5. Clean Up Properly

```python
# Always destroy sandboxes when done
sandbox.destroy(delete_codebase=True)

# Or use context manager (automatic cleanup)
with Sandbox.from_local("./project") as sandbox:
    # ... use sandbox ...
    pass  # Automatically cleaned up
```

### 6. Use Async for Concurrency

```python
import asyncio
from agentfense import AsyncSandbox

async def run_tests():
    # Run 10 sandboxes concurrently
    tasks = []
    for i in range(10):
        async with await AsyncSandbox.from_local("./project") as sb:
            task = sb.run("pytest")
            tasks.append(task)
    
    results = await asyncio.gather(*tasks)
    return results

asyncio.run(run_tests())
```

## Capacity Planning

### Single Server Recommendations

| Server Spec | Concurrent Sandboxes | Notes |
|-------------|----------------------|-------|
| 2 cores, 4 GB | 50-80 | Entry level |
| 4 cores, 8 GB | 150-200 | Recommended |
| 8 cores, 16 GB | 400+ | High capacity |

### Multi-Server Setup

For > 500 concurrent sandboxes:
1. Deploy multiple AgentFense servers
2. Use a load balancer (round-robin or least-connections)
3. Share codebase storage (NFS, S3, etc.)

## Monitoring

Key metrics to track:

```python
# Server metrics
- Active sandboxes count
- FUSE mount count
- Memory usage
- CPU usage
- Disk I/O

# Application metrics
- Sandbox creation rate
- Command execution latency
- File operation latency
- Error rate
```

## Known Bottlenecks

1. **Memory:** Each sandbox uses ~5 MB. High concurrency requires more RAM.
2. **FUSE mounts:** Each sandbox requires one mount. Linux has limits (~100K mounts).
3. **File descriptors:** Increase `ulimit -n` if you see "too many open files".
4. **Delta Layer:** Large deltas slow down reads. Export snapshots periodically.

## Future Improvements

Planned performance enhancements:
- Sandbox pooling (pre-warmed sandboxes)
- Shared FUSE mount (reduce per-sandbox overhead)
- Batch API (create multiple sandboxes in one call)
- Caching layer (reduce repeated file reads)
