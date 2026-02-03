# API 参考

AgentFense 完整的 API 参考文档。

## 可用参考

### [Python API 参考](python.md)

从 Python SDK 源代码自动生成的文档，包括：

- `Sandbox` / `AsyncSandbox` 类
- `SandboxClient` / `AsyncSandboxClient` 类
- 权限类型和枚举
- 异常类
- 工具函数

### [gRPC/Proto 参考](grpc.md)

Protocol Buffer 定义和 gRPC 服务规范：

- `SandboxService` - 沙盒生命周期和执行
- `CodebaseService` - 代码库和文件管理
- 消息类型和字段文档

### [Go API 参考](go.md)

*即将推出*

内部服务器组件的 Go 包文档。

## 快速链接

- **Python SDK**：从 [高层 API](../sdk/python/high-level.md) 开始
- **REST API**：使用 `:8080` 的 HTTP 网关（查看 [gRPC 参考](grpc.md) 获取端点）
- **gRPC**：直接连接到 `:9000` 使用生成的客户端

## 版本管理

API 文档与 AgentFense 版本同步。请确保使用与您安装版本匹配的文档版本。

当前版本：查看 [GitHub Releases](https://github.com/AjaxZhan/AgentFense/releases)
