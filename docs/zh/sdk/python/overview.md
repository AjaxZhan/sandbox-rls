# Python SDK 总览

## TL;DR

- **是什么**:AgentFense Python SDK 提供完整的工具集,用于在沙盒中运行不可信代码,并实现文件级权限控制。
- **核心价值**:从 5+ 个手工步骤(创建存储、上传文件、配置权限、启动运行时、执行命令)简化到 1 行代码,并自动清理资源。
- **适用场景**:需要让 AI Agent、脚本或自动化工具操作真实代码库,但不想授予完整文件系统访问权限或暴露机密信息。

---

## 解决什么问题?

### 面向 AI 应用开发者

**痛点**:你希望让 AI Agent 与真实代码库交互,但:
- 授予完整文件系统访问会暴露 `.env` 文件、API 密钥和凭据
- 手动配置权限容易出错(你会忘记屏蔽 `/secrets/` 目录)
- 手动创建隔离环境需要 5+ 次 API 调用和清理逻辑
- 没有内置方式追踪 Agent 读取、写入或尝试访问了什么

**代价**:花费数小时调试权限泄漏、编写清理代码,或更糟——生产环境中凭据被暴露。

**AgentFense SDK 方案**:
```python
from agentfense import Sandbox

# 一行代码:上传文件、设置权限、启动沙盒、自动清理
with Sandbox.from_local("./my-project", preset="agent-safe") as sandbox:
    result = sandbox.run("python analyze.py")
```

这会自动:
- 屏蔽对 `.env*`、`secrets/`、`*.key`、`*.pem` 等模式的访问
- 允许读取所有其他文件
- 允许写入 `/output` 和 `/tmp`
- 完成后清理沙盒和代码库

### 面向平台工程师

**痛点**:你在构建一个平台,用户提交代码/脚本以在共享代码库上运行:
- 每个用户需要不同的权限级别(只读、写入特定路径等)
- 需要强制执行资源限制(内存、CPU、进程数)
- 手动容器编排复杂(生命周期、网络、清理)
- 没有标准方式审计每次执行访问了什么

**AgentFense SDK 方案**:通过高层 API 抽象基础设施复杂性,同时在需要时保留细粒度控制。

### 面向安全团队

**痛点**:在生产环境运行不可信自动化:
- 传统沙盒是"全有或全无"(完全访问或完全无访问)
- 文件级审计追踪难以实现
- 机密管理需要自定义基础设施
- 需要证明特定文件真正不可访问(而不只是"相信我们")

**AgentFense SDK 方案**:基于 FUSE 的权限强制意味着被屏蔽的文件对沙盒进程来说是字面意义上的不可见——它们不会出现在 `ls` 中,访问时返回"文件不存在"。

---

## 为什么重要?

### 从"脚本"到"平台能力"

**使用 AgentFense SDK 之前**:
```python
# 手动方式:约 50 行代码,容易出错
client = SandboxClient("localhost:9000")
cb = client.create_codebase(...)
for file in os.walk(...):
    client.upload_file(cb.id, ...)
rules = [...]  # 手动定义每个模式
sb = client.create_sandbox(cb.id, permissions=rules)
client.start_sandbox(sb.id)
try:
    result = client.exec(sb.id, cmd)
finally:
    client.destroy_sandbox(sb.id)
    client.delete_codebase(cb.id)
```

**使用 AgentFense SDK**:
```python
# 3 行代码:上传、执行、清理
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    result = sandbox.run("make test")
```

### 从"人工检查"到"设计保证"

**避免的人为错误**:
- 忘记删除沙盒/代码库 → 上下文管理器确保清理
- 权限模式中的拼写错误 → 预设覆盖 80% 的用例
- 遗漏 `.env.local` 在屏蔽列表中 → `agent-safe` 预设包含 `.env*` 通配符
- 并发执行中的竞态条件 → Delta 层提供写时复制隔离

### 从"希望能工作"到"可观测与可审计"

SDK 不仅仅执行代码——它为每个操作提供结构化数据:
- `ExecResult` 包含退出码、stdout、stderr、持续时间
- 异常映射到特定错误类型(超时、权限拒绝、未找到)
- 权限规则是声明式且可检查的
- 所有操作都是幂等的和可重试的

---

## 何时必须使用

在以下情况使用 AgentFense Python SDK:

✅ **运行不可信代码**(AI Agent、用户脚本、第三方插件)操作真实代码库  
✅ **需要路径级权限控制**(不只是仓库级别的读/写)  
✅ **机密必须对自动化不可见**(不只是"不要读取它们")  
✅ **并发执行**,多个任务/用户不能污染彼此的状态  
✅ **需要资源限制**(内存、CPU、进程数)在运行时强制执行  
✅ **需要审计追踪**——你需要知道自动化访问/修改了什么  
✅ **需要有状态执行**(跨命令保持 `cwd`、环境变量)  
✅ **清理至关重要**——在你的环境中泄漏沙盒/容器是不可接受的  

不要使用的情况:
- 你完全信任代码(直接运行即可)
- 只需要 stdout/stderr(使用 `subprocess` 模块)
- 不需要文件系统隔离(Docker 单独可能就够了)

---

## 安装

### 从 PyPI 安装(推荐)

```bash
pip install agentfense
```

### 从源码安装(开发)

```bash
cd sdk/python
pip install -e .
```

### 依赖

- Python 3.8+
- gRPC(自动安装)
- 运行中的 AgentFense 服务器(参见[服务器设置](../../getting-started/installation.md))

---

## API 分层:选择正确的级别

SDK 提供三个 API 层,每个都针对不同用例优化:

### 1. 高层同步 API (`Sandbox`)

**适用于**:你想要最简单的集成方式并自动管理资源。

**核心方法**:
- `Sandbox.from_local(path)` - 从本地目录创建
- `sandbox.run(command)` - 执行并获取结果
- `sandbox.session()` - 有状态 shell

**特点**:
- 上下文管理器支持(`with` 语句)
- 自动清理
- 合理的默认值
- 覆盖 80% 的用例

```python
from agentfense import Sandbox

with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("pytest")
    print(result.stdout)
```

### 2. 高层异步 API (`AsyncSandbox`)

**适用于**:需要并发运行多个沙盒或与异步框架(FastAPI、aiohttp、asyncio)集成。

**特点**:
- 完全 async/await 兼容
- 与 `Sandbox` 相同的 API 接口
- 适合 I/O 密集型工作负载
- 可扩展到 100+ 并发沙盒

```python
from agentfense import AsyncSandbox
import asyncio

async def run_tests():
    async with await AsyncSandbox.from_local("./project") as sandbox:
        result = await sandbox.run("pytest")
        return result.stdout

asyncio.run(run_tests())
```

### 3. 低层 API (`SandboxClient`)

**适用于**:需要对代码库管理的细粒度控制、想要跨沙盒重用代码库,或正在构建自己的抽象。

**特点**:
- 对每个步骤的显式控制
- 可重用的代码库
- 高级配置
- 约 20% 的用例(专业化)

```python
from agentfense import SandboxClient

client = SandboxClient(endpoint="localhost:9000")

# 创建代码库一次
codebase = client.create_codebase(name="shared-repo", owner_id="team_A")

# 手动上传文件
client.upload_file(codebase.id, "main.py", b"...")

# 从同一代码库创建多个沙盒
sandbox1 = client.create_sandbox(codebase.id, permissions=[...])
sandbox2 = client.create_sandbox(codebase.id, permissions=[...])

# 显式生命周期
client.start_sandbox(sandbox1.id)
result = client.exec(sandbox1.id, command="python main.py")
client.destroy_sandbox(sandbox1.id)
```

### 选择同步还是异步

| 标准 | 使用同步 (`Sandbox`) | 使用异步 (`AsyncSandbox`) |
|----------|---------------------|---------------------------|
| **并发性** | 1-5 个沙盒顺序执行 | 10+ 沙盒并行 |
| **框架** | Flask、Django、脚本 | FastAPI、aiohttp、asyncio |
| **执行模型** | 阻塞 I/O 可以接受 | 需要 async/await |
| **复杂度** | 更简单(无事件循环) | 需要理解异步 |
| **典型延迟** | 毫秒级操作 | 微秒级上下文切换 |

**经验法则**:从同步 `Sandbox` 开始。只有在测量到并发瓶颈时才迁移到 `AsyncSandbox`。

---

## 快速开始示例

### 最小示例:运行一个命令

```python
from agentfense import Sandbox

# 最简单的用法
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("ls -la")
    print(result.stdout)
```

**发生了什么**:
1. 从 `./my-project` 上传所有文件
2. 使用 `view-only` 预设(默认)创建沙盒
3. 启动沙盒
4. 执行 `ls -la`
5. 退出时销毁沙盒 + 代码库

### 使用自定义权限

```python
from agentfense import Sandbox, PermissionRule, Permission

with Sandbox.from_local(
    "./my-project",
    preset="read-only",
    permissions=[
        PermissionRule(pattern="/output/**", permission=Permission.WRITE),
        PermissionRule(pattern="/secrets/**", permission=Permission.NONE),
    ]
) as sandbox:
    result = sandbox.run("python process.py")
```

### 使用 Docker 运行时和资源限制

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./my-project",
    preset="agent-safe",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB
        pids_limit=100,
    ),
) as sandbox:
    result = sandbox.run("pytest", timeout=300)
```

### 有状态执行(Session)

```python
with Sandbox.from_local("./my-project") as sandbox:
    with sandbox.session() as session:
        # 所有这些命令在同一个 shell 进程中运行
        session.exec("cd /workspace/backend")
        session.exec("source venv/bin/activate")
        session.exec("export DEBUG=1")
        result = session.exec("python manage.py test")
        print(result.stdout)
```

### 异步:并发运行 10 个沙盒

```python
import asyncio
from agentfense import AsyncSandbox

async def run_in_sandbox(project_path):
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        return await sandbox.run("make test")

async def main():
    projects = [f"./project-{i}" for i in range(10)]
    tasks = [run_in_sandbox(p) for p in projects]
    results = await asyncio.gather(*tasks)
    for i, result in enumerate(results):
        print(f"Project {i}: {result.exit_code}")

asyncio.run(main())
```

---

## 核心概念

### Codebase vs Sandbox

- **Codebase**(代码库):文件存储。保存你的源代码、配置文件等。可重用。
- **Sandbox**(沙盒):执行环境。将代码库与权限 + 运行时绑定。临时性的。

**何时重用代码库**:
- 使用不同权限运行相同代码
- 多个用户访问同一仓库
- 在相同代码上测试不同配置

**何时创建新代码库**:
- 每个用户有自己的文件副本
- 文件在运行之间频繁变化

### 权限预设

| 预设 | 用例 | 读取 | 写入 | 屏蔽 |
|--------|----------|------|-------|--------|
| `view-only` | 审计/检查 | 仅文件列表 | 无 | N/A |
| `read-only` | 静态分析 | 所有文件 | 无 | N/A |
| `agent-safe` | AI Agent | 所有(除机密) | `/output`、`/tmp` | `.env*`、`secrets/`、`*.key` |
| `development` | 开发自动化 | 所有(除机密) | 所有(除机密) | `.env*`、`secrets/`、`*.key` |
| `full-access` | 可信脚本 | 所有 | 所有 | 无 |

**自定义**:所有预设都可以通过 `permissions=[...]` 参数扩展。

### 运行时类型

- **bwrap**(默认):轻量级 Linux 命名空间隔离。快速启动(约 50ms)。需要 Linux 主机。
- **docker**:完整容器隔离。跨平台。启动较慢(约 500ms)。更强的安全边界。

**选择 bwrap**:速度重要、Linux 主机、轻度隔离足够。  
**选择 Docker**:Windows/Mac 开发、需要网络控制、需要镜像定制。

### Session vs 直接执行

**直接执行**(`sandbox.run()`、`sandbox.exec()`):
- 每个命令启动新的 shell
- 调用之间不保留状态
- 适合一次性命令

**Session**(`sandbox.session()`):
- 持久化 shell 进程
- 保留 `cwd`、环境变量
- 启动较慢,后续命令更快
- 必需场景:`cd`、`source`、`export`、后台进程

---

## 错误处理哲学

SDK 使用**语义化异常**而非通用错误:

```python
from agentfense import (
    Sandbox,
    CommandTimeoutError,
    CommandExecutionError,
    PermissionDeniedError,
    SandboxNotRunningError,
)

try:
    with Sandbox.from_local("./project") as sandbox:
        result = sandbox.run("make build", timeout=60, raise_on_error=True)
except CommandTimeoutError as e:
    # 命令耗时 > 60 秒
    print(f"超时: {e.command}")
except CommandExecutionError as e:
    # 非零退出码
    print(f"失败,退出码 {e.exit_code}: {e.stderr}")
except PermissionDeniedError as e:
    # 尝试访问被屏蔽的文件
    print(f"权限拒绝: {e.path}")
except SandboxNotRunningError as e:
    # 沙盒未启动或已停止
    print(f"沙盒未运行: {e.current_status}")
```

**优势**:
- 捕获特定错误而无需解析错误消息
- 对超时 vs 失败采取不同处理
- 丰富的错误上下文(退出码、stderr、命令等)

---

## 下一步

- **高层 API**:参见[高层 API 指南](./high-level.md)了解 `Sandbox` 类详情
- **异步 API**:参见[异步 API 指南](./async.md)了解并发执行
- **Sessions**:参见[Sessions 指南](./sessions.md)了解有状态执行
- **权限**:参见[权限指南](./permissions.md)了解访问控制
- **错误处理**:参见[错误处理指南](./errors.md)了解异常模式
- **示例**:查看仓库中的 `examples/` 目录

---

## 性能特征

| 操作 | 延迟 | 注释 |
|-----------|---------|-------|
| `Sandbox.from_local()` | 100-500ms | 主要是文件上传 |
| `sandbox.run()` (bwrap) | 10-50ms | 每命令开销 |
| `sandbox.run()` (docker) | 50-200ms | 容器启动 |
| `session.exec()` (创建后) | 5-20ms | 重用 shell 进程 |
| 文件上传 (1 MB) | 10-50ms | 取决于网络 |
| 文件下载 (1 MB) | 10-50ms | 取决于网络 |

**优化建议**:
- 尽可能重用代码库
- 对多个命令使用 session
- 并发沙盒使用异步 API
- 批量文件操作

---

## 与替代方案对比

| 方法 | 设置复杂度 | 权限粒度 | 审计追踪 | 清理 | 安全性 |
|----------|------------------|----------------------|-------------|---------|--------|
| **AgentFense SDK** | 1 行 | 路径级 | 内置 | 自动 | 强制执行 |
| **直接使用 Docker** | 10+ 行 | 容器级 | 手动 | 手动 | 取决于 |
| **subprocess + chroot** | 20+ 行 | 手动 | 无 | 手动 | 容易出错 |
| **每次执行用 VM** | 100+ 行 | VM 级 | 手动 | 手动 | 慢 |
| **无隔离** | 0 行 | 无 | 无 | N/A | ❌ 不安全 |

**何时使用其他方案**:
- **仅 Docker**:你不需要文件级权限
- **subprocess**:你完全信任代码
- **VM**:你需要内核级隔离(AgentFense 使用容器/命名空间)
