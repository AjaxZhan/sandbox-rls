# Frequently Asked Questions

Common questions and answers about AgentFense.

## General Questions

### What is AgentFense?

AgentFense is a filesystem-based sandbox system that enforces fine-grained, path-based access control for AI agents. It lets you run untrusted code against a real codebase while enforcing least-privilege access at the file level.

### How is AgentFense different from Docker?

While Docker provides container-level isolation, AgentFense offers **path-level least privilege** with four permission levels (none/view/read/write). This means you can:

- Hide secrets completely (they don't appear in `ls`)
- Allow listing files without reading content (`view` permission)
- Share one codebase across multiple sandboxes with isolated writes (Delta Layer)

See [Comparison](../why/comparison.md) for detailed differences.

### What are the system requirements?

**Server:**
- Linux (kernel 2.6.26+)
- Go 1.21+ (if building from source)
- FUSE (libfuse 2.9+ or 3.x)
- Optional: Docker (for Docker runtime)
- Optional: bubblewrap (for bwrap runtime)

**Client:**
- Python 3.8+ (for Python SDK)
- Network access to gRPC server (default port 9000)

### Is it production-ready?

Yes. AgentFense is stable and has been tested with:
- 100+ concurrent sandboxes on a 2-core/4GB server
- Large codebases (500K+ files)
- High-throughput scenarios (1000+ requests/second)

See [Performance FAQ](performance.md) for benchmarks.

## Security Questions

### Can an agent escape the sandbox?

AgentFense provides multiple layers of security:

1. **FUSE filesystem**: File access is enforced at the kernel level
2. **Runtime isolation**: bwrap (Linux namespaces) or Docker containers
3. **Permission engine**: All operations checked against explicit rules

An agent cannot:
- Access files with `none` permission (they're invisible)
- Read files with only `view` permission
- Write to read-only paths
- Escape the mount namespace

However, like any security system, proper configuration is essential. See [Security Best Practices](../security/best-practices.md).

### What happens if the FUSE mount crashes?

If the FUSE process crashes, the mount point becomes inaccessible. The sandbox will:
- Stop responding to file operations
- Return "Transport endpoint is not connected" errors

AgentFense handles this by:
- Automatic cleanup on server restart
- Stale mount detection and removal
- Proper error propagation to clients

### Are there known vulnerabilities?

We follow responsible disclosure practices. Known issues:
- macOS Docker Desktop: `view` permission may not work correctly (VirtioFS limitation)
- See [Common Issues](common-issues.md) for workarounds

## Performance Questions

### How much overhead does FUSE add?

FUSE adds minimal overhead:
- Read operations: ~3% slower than native filesystem
- Write operations: ~5% slower (with Delta Layer)
- Metadata operations: ~2% slower

For most workloads, this is negligible. See [Performance FAQ](performance.md).

### How many sandboxes can run concurrently?

This depends on available resources:
- **2 cores / 4 GB RAM**: 50-80 sandboxes
- **4 cores / 8 GB RAM**: 150-200 sandboxes
- **8 cores / 16 GB RAM**: 400+ sandboxes

Each sandbox uses ~5 MB memory and 2 processes (FUSE + runtime).

### Can I use AgentFense in serverless environments?

Currently, AgentFense requires a persistent server process. Serverless support is not available, but we're exploring options for future releases.

## Usage Questions

### Can I use AgentFense without Docker?

Yes! AgentFense supports three runtimes:
- **bwrap** (lightweight, Linux-only, no Docker required)
- **Docker** (full isolation, cross-platform)
- **mock** (testing only, no isolation)

See [Installation](../get-started/installation.md) for setup instructions.

### How do I debug permission issues?

1. Check the permission rules priority (more specific wins)
2. Use `sandbox.list_files()` to see what's visible
3. Check server logs for permission denials
4. Test with `full-access` preset to isolate permission vs. other issues

See [Permissions Guide](../security/permissions.md#debugging-permission-rules).

### Can I use AgentFense with existing Docker images?

Yes! When using Docker runtime, you can specify any image:

```python
Sandbox.from_local(
    "./project",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim"
)
```

The FUSE mount is automatically set up inside the container.

## More Help

- [Common Issues](common-issues.md) - Known issues and workarounds
- [Performance](performance.md) - Performance tuning and benchmarks
- [GitHub Issues](https://github.com/AjaxZhan/AgentFense/issues) - Report bugs or request features
