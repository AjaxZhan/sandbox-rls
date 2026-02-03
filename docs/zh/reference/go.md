# Go API 参考

*即将推出*

AgentFense 服务器内部的 Go 包文档。

## 计划中的包

### `github.com/AjaxZhan/AgentFense/pkg/types`

核心类型和数据结构：
- 权限类型和规则
- 沙盒配置
- 资源限制
- 错误类型

### `github.com/AjaxZhan/AgentFense/internal/fs`

文件系统层：
- FUSE 实现
- 权限引擎
- Delta 层（COW）

### `github.com/AjaxZhan/AgentFense/internal/runtime`

运行时抽象：
- 运行时接口
- bwrap 实现
- Docker 实现
- 会话管理

### `github.com/AjaxZhan/AgentFense/internal/server`

gRPC 服务器：
- SandboxService 实现
- CodebaseService 实现
- 中间件和拦截器

## 目前

直接使用源代码：
- [在 GitHub 上浏览](https://github.com/AjaxZhan/AgentFense/tree/main/internal)
- [GoDoc](https://pkg.go.dev/github.com/AjaxZhan/AgentFense)（发布后）

## 贡献

帮助我们改进 Go 文档！查看 [CONTRIBUTING.md](https://github.com/AjaxZhan/AgentFense/blob/main/CONTRIBUTING.md)。
