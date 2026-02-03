# SDK 文档

AgentFense 提供多种语言的客户端库来与沙盒服务交互。

## 可用的 SDK

### Python SDK

Python SDK 是最完整、最活跃维护的客户端库。

**安装：**
```bash
pip install agentfense
```

**快速示例：**
```python
from agentfense import Sandbox

with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python main.py")
    print(result.stdout)
```

**文档：**

- [总览](python/overview.md) - SDK 能力与架构
- [高层 API](python/high-level.md) - 推荐用于大多数场景
- [异步 API](python/async.md) - 用于高并发场景
- [会话](python/sessions.md) - 持久化 shell 会话
- [权限管理](python/permissions.md) - 权限管理与预设
- [错误处理](python/errors.md) - 异常处理模式

### Go SDK

*即将推出*

Go SDK 计划在未来版本中发布。目前可以直接使用 gRPC 客户端。

### REST API

所有功能都可以通过 REST API 访问（gRPC 的 HTTP 网关）。

**基础 URL：** `http://localhost:8080`

**示例：**
```bash
# 创建沙盒
curl -X POST http://localhost:8080/v1/sandboxes \
  -d '{"codebase_id": "cb_xxx", "permissions": [{"pattern": "**/*", "permission": "PERMISSION_READ"}]}'
```

查看 [gRPC/Proto 参考](../reference/grpc.md) 获取完整 API 规范。

## 选择 SDK

| SDK | 适用场景 | 状态 |
|-----|----------|--------|
| **Python** | AI agent、脚本、自动化 | ✅ 稳定 |
| **Go** | 后端服务、高性能 | 🚧 计划中 |
| **REST** | 语言无关集成 | ✅ 稳定 |

## 支持

- [GitHub Issues](https://github.com/AjaxZhan/AgentFense/issues)
- [API 参考](../reference/index.md)
- [常见问题](../faq/index.md)
