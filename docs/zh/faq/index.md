# 常见问题

关于 AgentFense 的常见问题和解答。

## 一般问题

### AgentFense 是什么？

AgentFense 是一个基于文件系统的沙盒系统，为 AI agent 提供细粒度、基于路径的访问控制。它允许您在真实代码库上运行不可信代码，同时在文件级别强制执行最小权限访问。

### AgentFense 与 Docker 有什么不同？

Docker 提供容器级隔离，而 AgentFense 提供**路径级最小权限**，具有四种权限级别（none/view/read/write）。这意味着您可以：

- 完全隐藏密钥（它们不会出现在 `ls` 中）
- 允许列出文件但不读取内容（`view` 权限）
- 在多个沙盒之间共享一个代码库，写入隔离（Delta Layer）

详细差异请参阅 [方案对比](../why/comparison.md)。

### 系统要求是什么？

**服务器：**
- Linux (内核 2.6.26+)
- Go 1.21+（如果从源码构建）
- FUSE (libfuse 2.9+ 或 3.x)
- 可选：Docker（用于 Docker 运行时）
- 可选：bubblewrap（用于 bwrap 运行时）

**客户端：**
- Python 3.8+（用于 Python SDK）
- 可访问 gRPC 服务器的网络（默认端口 9000）

### 是否可以用于生产环境？

是的。AgentFense 已经过测试：
- 在 2 核/4GB 服务器上支持 100+ 并发沙盒
- 大型代码库（50 万+ 文件）
- 高吞吐量场景（1000+ 请求/秒）

查看 [性能常见问题](performance.md) 了解基准测试。

## 安全问题

### agent 能逃逸沙盒吗？

AgentFense 提供多层安全保障：

1. **FUSE 文件系统**：文件访问在内核级别强制执行
2. **运行时隔离**：bwrap（Linux 命名空间）或 Docker 容器
3. **权限引擎**：所有操作都根据显式规则检查

agent 无法：
- 访问具有 `none` 权限的文件（它们不可见）
- 读取仅具有 `view` 权限的文件
- 写入只读路径
- 逃逸挂载命名空间

但是，与任何安全系统一样，正确配置至关重要。请参阅 [安全最佳实践](../security/best-practices.md)。

### 如果 FUSE 挂载崩溃会怎样？

如果 FUSE 进程崩溃，挂载点将变得不可访问。沙盒将：
- 停止响应文件操作
- 返回"传输端点未连接"错误

AgentFense 通过以下方式处理：
- 服务器重启时自动清理
- 检测并删除陈旧挂载
- 向客户端正确传播错误

### 是否存在已知漏洞？

我们遵循负责任的披露实践。已知问题：
- macOS Docker Desktop：`view` 权限可能无法正常工作（VirtioFS 限制）
- 查看 [常见问题](common-issues.md) 了解解决方法

## 性能问题

### FUSE 增加了多少开销？

FUSE 增加的开销很小：
- 读取操作：比原生文件系统慢约 3%
- 写入操作：慢约 5%（带 Delta Layer）
- 元数据操作：慢约 2%

对于大多数工作负载，这可以忽略不计。查看 [性能常见问题](performance.md)。

### 可以同时运行多少个沙盒？

这取决于可用资源：
- **2 核 / 4 GB RAM**：50-80 个沙盒
- **4 核 / 8 GB RAM**：150-200 个沙盒
- **8 核 / 16 GB RAM**：400+ 个沙盒

每个沙盒使用约 5 MB 内存和 2 个进程（FUSE + 运行时）。

### 可以在无服务器环境中使用 AgentFense 吗？

目前，AgentFense 需要持久的服务器进程。不支持无服务器，但我们正在为未来版本探索选项。

## 使用问题

### 可以不使用 Docker 吗？

可以！AgentFense 支持三种运行时：
- **bwrap**（轻量级，仅 Linux，无需 Docker）
- **Docker**（完全隔离，跨平台）
- **mock**（仅测试，无隔离）

查看 [安装指南](../get-started/installation.md) 了解设置说明。

### 如何调试权限问题？

1. 检查权限规则优先级（更具体的优先）
2. 使用 `sandbox.list_files()` 查看可见内容
3. 检查服务器日志中的权限拒绝
4. 使用 `full-access` 预设测试，以隔离权限与其他问题

查看 [权限指南](../security/permissions.md#troubleshooting)。

### 可以使用现有的 Docker 镜像吗？

可以！使用 Docker 运行时时，您可以指定任何镜像：

```python
Sandbox.from_local(
    "./project",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim"
)
```

FUSE 挂载会自动在容器内设置。

## 更多帮助

- [常见问题](common-issues.md) - 已知问题和解决方法
- [性能](performance.md) - 性能调优和基准测试
- [GitHub Issues](https://github.com/AjaxZhan/AgentFense/issues) - 报告错误或请求功能
