# Sandbox-RLS Roadmap

本文档描述 Sandbox-RLS 的产品定位、开发路线和技术决策。

## 产品定位

**面向个人开发者的轻量级 AI Agent 沙箱**，核心价值是：

- 🎯 **细粒度文件权限控制**：`none/view/read/write` 四级权限 + glob 模式
- 🚀 **轻量易用**：单机部署，无需 K8s/云服务
- 🔒 **安全隔离**：让 Agent 在真实代码库上工作，同时保护敏感文件

我们**不追求**云厂商级别的 MicroVM 隔离、百万并发、Serverless 弹性等能力。

---

## 当前状态

### 已实现 ✅

| 功能 | 说明 |
|------|------|
| **细粒度权限控制** | FUSE 文件系统层面的 `none/view/read/write` 四级权限 |
| **Glob 模式匹配** | 支持 `**/*.py`、`/secrets/**` 等模式 |
| **bwrap 隔离** | 基于 bubblewrap 的 namespace 隔离 |
| **多沙箱共享 Codebase** | 同一份代码可被多个 Agent 以不同权限访问 |
| **Python SDK** | 完整的 Python 客户端 |
| **gRPC + REST API** | 双协议支持 |
| **Session 支持** | 有状态 shell sessions，保持工作目录、环境变量 |
| **Docker Runtime** | 可选的 Docker 隔离，支持自定义镜像 |
| **资源限制** | 内存、CPU、进程数限制 |

### 主要缺陷 ❌

| 问题 | 影响 |
|------|------|
| **bwrap 隔离较弱** | 不适合运行完全不可信的代码 |
| **单机架构** | 无法水平扩展 |

---

## 开发路线

### Phase 1: 核心功能完善 ✅ 已完成

让 Sandbox 能真正支持 Agent 的日常开发任务。

#### 1.1 Session 支持 ✅

**已实现**：有状态的 shell sessions，支持工作目录和环境变量保持。

```python
# 创建 session
session = sandbox.create_session()
session.exec("cd /workspace")      # 状态保持
session.exec("npm install")        # ✅ 在 /workspace 执行
session.exec("export FOO=bar")     # ✅ 环境变量保持
session.exec("echo $FOO")          # ✅ 输出 bar
```

**实现细节**：
- 长期运行的 shell 进程（`/bin/bash`）
- PTY 支持（用于交互式命令）
- 进程组管理（清理后台进程）
- Session 生命周期管理（创建、关闭、自动清理）

#### 1.2 资源限制 ✅

**已实现**：通过 Docker runtime 支持资源限制。

```python
# 创建带资源限制的沙箱
sandbox = client.create_sandbox(
    codebase_id=codebase.id,
    resource_limits={
        "memory_bytes": 512 * 1024 * 1024,  # 512MB
        "cpu_millicores": 1000,              # 1 CPU
        "max_pids": 100,                     # 最大进程数
    }
)
```

**实现细节**：
- 通过 Docker 容器实现资源隔离
- 支持内存、CPU、进程数限制
- 可在创建时指定

#### 1.3 命令超时与熔断

**问题**：死循环或长时间运行的命令会卡住系统。

```python
# 目标
result = sandbox.exec("npm install", timeout=60)  # 60秒超时
if result.timed_out:
    print("命令超时，已自动终止")
```

---

### Phase 2: 隔离增强 + 开发者体验

提升安全性和易用性。

#### 2.1 Docker Runtime ✅

**已实现**：完整的 Docker 隔离支持。

```python
# 使用 Docker runtime
sandbox = client.create_sandbox(
    codebase_id=codebase.id,
    runtime="docker",
    image="python:3.11-slim",  # 指定运行环境
    resource_limits={
        "memory_bytes": 512 * 1024 * 1024,
        "cpu_millicores": 1000,
    }
)
```

**实现细节**：
- 实现 `runtime.Runtime` 接口的 Docker 版本
- 支持自定义镜像
- Codebase 目录挂载到容器内部
- 完整的 Session 支持（与 bwrap 一致）
- 容器生命周期管理

#### 2.2 一键启动 API

**问题**：当前创建沙箱流程太繁琐。

```python
# 当前（繁琐）
codebase = client.create_codebase(name="test", owner_id="user1")
client.upload_file(codebase.id, "main.py", b"print('hello')")
sandbox = client.create_sandbox(codebase_id=codebase.id, permissions=[...])
client.start_sandbox(sandbox.id)

# 目标（简单）
with Sandbox.from_local("./my-project", preset="agent-safe") as sandbox:
    result = sandbox.run("python main.py")
```

#### 2.3 预设权限模板

```python
# 目标：内置常用权限模板
PRESETS = {
    "agent-safe": [
        {"pattern": "**/*", "permission": "read"},
        {"pattern": "/output/**", "permission": "write"},
        {"pattern": "**/.env*", "permission": "none"},
        {"pattern": "**/secrets/**", "permission": "none"},
        {"pattern": "**/*.key", "permission": "none"},
    ],
    "read-only": [
        {"pattern": "**/*", "permission": "read"},
    ],
    "full-access": [
        {"pattern": "**/*", "permission": "write"},
    ],
}

sandbox = Sandbox.from_local("./project", preset="agent-safe")
```

#### 2.4 CLI 工具

```bash
# 快速测试
$ sandbox-rls run ./my-project "npm test"

# 交互式沙箱
$ sandbox-rls shell ./my-project --preset=agent-safe

# 查看沙箱状态
$ sandbox-rls list
$ sandbox-rls logs <sandbox-id>
```

#### 2.5 Go SDK

提供与 Python SDK 功能对等的 Go 客户端。

---

### Phase 3: 多 Agent 协作 + 生态

支持复杂的多 Agent 场景。

#### 3.1 文件锁与冲突检测

**问题**：多个 Agent 同时写同一文件可能导致冲突。

```python
# 目标
with sandbox.lock_file("/src/main.py"):
    sandbox.exec("echo 'new code' >> /src/main.py")
# 其他 Agent 在此期间尝试写入会等待或失败
```

#### 3.2 Agent 间通信

```python
# 目标：简单的事件机制
sandbox_a.emit("backend-ready", {"port": 3000})
sandbox_b.wait_for("backend-ready")
sandbox_b.exec("curl localhost:3000")
```

#### 3.3 外部数据源支持

让 Agent 通过文件系统接口访问非文件数据：

```python
# 目标：S3 存储作为 Codebase
codebase = client.create_codebase(
    source="s3://my-bucket/code",
    credentials={...}
)

# 目标：数据库作为虚拟文件系统
codebase = client.create_codebase(
    source="postgres://...",
    mapping={
        "/users.json": "SELECT * FROM users",
        "/orders.csv": "SELECT * FROM orders",
    }
)
```

#### 3.4 配置文件支持

```yaml
# sandbox.yaml（项目根目录）
version: 1
preset: agent-safe

permissions:
  - pattern: "/src/**"
    permission: read
  - pattern: "/tests/**"
    permission: write

resources:
  memory: 512M
  cpu: 1.0
  timeout: 60s

runtime: docker
image: node:18-slim
```

---

### Phase 4: 可选增强（按需）

以下功能根据实际需求决定是否实现。

#### 4.1 gVisor Runtime

比 Docker 更强的隔离，适合运行完全不可信的代码。

```python
sandbox = client.create_sandbox(
    runtime="gvisor",  # 使用 gVisor
    ...
)
```

#### 4.2 Warm Pool（预热池）

减少沙箱冷启动时间。

```python
# 预先创建一批空闲沙箱
pool = SandboxPool(size=5, preset="agent-safe")
sandbox = pool.acquire()  # 毫秒级获取
```

#### 4.3 监控与可观测性

- Prometheus metrics
- 结构化日志
- 分布式追踪（OpenTelemetry）

---

## 技术决策记录

### 为什么选择 bwrap 而不是 Docker？

- ✅ 更轻量，启动更快
- ✅ 无需 Docker daemon
- ✅ 对 FUSE 支持更好
- ❌ 隔离性不如 Docker

**决策**：bwrap 作为默认 runtime，Docker 作为可选增强。

### 为什么不支持 MicroVM（Firecracker）？

- 需要 `/dev/kvm` 和 root 权限
- 资源开销大
- 个人开发者场景不需要这么强的隔离

**决策**：不支持，超出产品定位。

### 为什么不支持休眠/唤醒（CRIU）？

- 技术复杂度高
- bwrap 不原生支持
- 个人开发者场景沙箱生命周期短，不需要

**决策**：不支持，超出产品定位。

---

## 版本规划（暂定）

| 版本 | 主要内容 | 状态 |
|------|----------|------|
| v0.1 | 基础功能：权限控制、bwrap 隔离、Python SDK | ✅ 已完成 |
| v0.2 | Session 支持、资源限制 | ✅ 已完成 |
| v0.3 | Docker Runtime | ✅ 已完成 |
| v0.4 | 一键启动 API、预设模板、CLI 工具 | 📋 计划中 |
| v0.5 | Go SDK、配置文件支持 | 📋 计划中 |
| v1.0 | 多 Agent 协作、生产就绪 | 📋 计划中 |

---

## 参与贡献

欢迎贡献代码、文档或提出建议！请查看 [Contributing Guide](CONTRIBUTING.md)（待创建）。

如果你对某个功能特别感兴趣，欢迎在 Issues 中讨论。
