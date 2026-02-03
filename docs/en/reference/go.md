# Go API Reference

*Coming soon*

Go package documentation for AgentFense server internals.

## Planned Packages

### `github.com/AjaxZhan/AgentFense/pkg/types`

Core types and data structures:
- Permission types and rules
- Sandbox configuration
- Resource limits
- Error types

### `github.com/AjaxZhan/AgentFense/internal/fs`

Filesystem layer:
- FUSE implementation
- Permission engine
- Delta layer (COW)

### `github.com/AjaxZhan/AgentFense/internal/runtime`

Runtime abstraction:
- Runtime interface
- bwrap implementation
- Docker implementation
- Session management

### `github.com/AjaxZhan/AgentFense/internal/server`

gRPC server:
- SandboxService implementation
- CodebaseService implementation
- Middleware and interceptors

## For Now

Use the source code directly:
- [Browse on GitHub](https://github.com/AjaxZhan/AgentFense/tree/main/internal)
- [GoDoc](https://pkg.go.dev/github.com/AjaxZhan/AgentFense) (when published)

## Contributing

Help us improve the Go documentation! See [CONTRIBUTING.md](https://github.com/AjaxZhan/AgentFense/blob/main/CONTRIBUTING.md).
