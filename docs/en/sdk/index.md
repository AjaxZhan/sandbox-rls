# SDK Documentation

AgentFense provides client libraries for multiple languages to interact with the sandbox service.

## Available SDKs

### Python SDK

The Python SDK is the most complete and actively maintained client library.

**Installation:**
```bash
pip install agentfense
```

**Quick Example:**
```python
from agentfense import Sandbox

with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)
```

**Documentation:**

- [Overview](python/overview.md) - SDK capabilities and architecture
- [High-Level API](python/high-level.md) - Recommended API for most use cases
- [Async API](python/async.md) - For high-concurrency scenarios
- [Sessions](python/sessions.md) - Persistent shell sessions
- [Permissions](python/permissions.md) - Permission management and presets
- [Error Handling](python/errors.md) - Exception handling patterns

### Go SDK

*Coming soon*

The Go SDK is planned for future releases. For now, you can use the gRPC client directly.

### REST API

All functionality is available via REST API (HTTP gateway to gRPC).

**Base URL:** `http://localhost:8080`

**Example:**
```bash
# Create sandbox
curl -X POST http://localhost:8080/v1/sandboxes \
  -d '{"codebase_id": "cb_xxx", "permissions": [{"pattern": "**/*", "permission": "PERMISSION_READ"}]}'
```

See [gRPC/Proto Reference](../reference/grpc.md) for complete API specification.

## Choosing an SDK

| SDK | Best For | Status |
|-----|----------|--------|
| **Python** | AI agents, scripting, automation | ✅ Stable |
| **Go** | Backend services, high performance | 🚧 Planned |
| **REST** | Language-agnostic integration | ✅ Stable |

## Support

- [GitHub Issues](https://github.com/AjaxZhan/AgentFense/issues)
- [API Reference](../reference/index.md)
- [FAQ](../faq/index.md)
