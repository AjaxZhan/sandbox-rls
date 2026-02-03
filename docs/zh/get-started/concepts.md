# 核心概念

本指南解释 AgentFense 的关键概念:**Codebase**、**Sandbox**、**Permission**、**Session**、**Preset** 和 **Delta Layer**。理解这些概念有助于设计有效的权限策略并高效使用 AgentFense。

## Codebase

**Codebase 是服务器上的文件存储**——代码和数据的真实来源。

### 功能

- 存储从本地目录上传或通过编程创建的文件
- 提供版本化、不可变的文件存储(文件无法原地修改)
- 作为一个或多个 Sandbox 的数据源
- 跟踪元数据:文件数量、总大小、所有者、时间戳

### 关键特性

| 特性 | 描述 |
|------|------|
| **持久化** | 即使所有 Sandbox 被销毁,文件仍然存在 |
| **可共享** | 多个 Sandbox 可以使用同一个 Codebase |
| **隔离** | 每个 Codebase 与其他 Codebase 隔离(无交叉污染) |

### 生命周期

```
创建 → 上传文件 → 创建 Sandbox → 删除
```

### 何时使用多个 Codebase

- **每个项目一个 codebase**:存储单个项目的所有文件
- **版本化 codebase**:为每个发布或快照创建新的 codebase
- **多租户**:每个用户或团队单独的 codebase

### 示例

```python
from agentfense import SandboxClient

client = SandboxClient(endpoint="localhost:9000")

# 创建 codebase
codebase = client.create_codebase(
    name="my-app-v1.0",
    owner_id="team_123",
)

# 上传文件
client.upload_file(codebase.id, "app.py", b"print('v1.0')")
client.upload_file(codebase.id, "config.yaml", b"version: 1.0")

# 列出文件
files = client.list_files(codebase.id, path="/", recursive=True)
for f in files:
    print(f"{f.path} ({f.size} bytes)")

# 下载文件
content = client.download_file(codebase.id, "app.py")
print(content.decode())

# 清理(仅在销毁所有 sandbox 后)
client.delete_codebase(codebase.id)
```

### 高层 API(自动 Codebase 管理)

使用 `Sandbox.from_local()` 时,codebase 自动创建和删除:

```python
from agentfense import Sandbox

# Codebase 自动创建、使用和删除
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("ls /workspace")
# Codebase 在此处删除
```

## Sandbox

**Sandbox 是隔离的执行环境**,不可信代码在其中针对带权限强制的 Codebase 运行。

### 功能

- 将 Codebase 作为 FUSE 文件系统挂载到 `/workspace`
- 在文件系统层面强制权限规则
- 提供进程隔离(bwrap 或 Docker)
- 捕获写入到每个沙盒的 Delta Layer(写时复制)
- 使用受控环境、工作目录和超时执行命令

### 架构

```
┌─────────────────────────────────────────┐
│            Sandbox 进程                  │
│   (bwrap namespace / Docker container)  │
├─────────────────────────────────────────┤
│         FUSE 文件系统挂载                │
│         /workspace (强制权限)            │
├─────────────────────────────────────────┤
│          Delta Layer (COW)              │
│    (每个沙盒的写入隔离)                   │
├─────────────────────────────────────────┤
│          Codebase 存储                  │
│       (不可变源文件)                      │
└─────────────────────────────────────────┘
```

### 生命周期

```
创建 → 启动 → 执行/会话 → 停止 → 销毁
```

### Sandbox 状态

| 状态 | 描述 | 可用操作 |
|------|------|----------|
| **PENDING** | 已创建但未启动 | 启动、销毁 |
| **RUNNING** | 活跃且准备好执行命令 | 执行、会话、停止、销毁 |
| **STOPPED** | 已停止但未销毁 | 启动、销毁 |
| **ERROR** | 启动失败或崩溃 | 销毁 |

### 运行时类型

AgentFense 支持两种隔离机制:

| 运行时 | 隔离 | 启动时间 | 使用场景 |
|--------|------|----------|----------|
| **bwrap** | Linux 命名空间(轻量级) | ~50ms | 快速迭代、开发、低开销 |
| **Docker** | 容器(强隔离) | ~500ms | 生产环境、严格隔离、自定义镜像 |

### 资源限制

使用 `ResourceLimits` 控制资源使用:

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./project",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB 内存
        cpu_quota=50000,                 # 50% CPU (50000/100000)
        pids_limit=100,                  # 最多 100 个进程
    ),
) as sandbox:
    result = sandbox.run("python heavy_task.py")
```

### 示例:手动 Sandbox 生命周期

```python
from agentfense import SandboxClient

client = SandboxClient()

# 创建 sandbox
sandbox = client.create_sandbox(
    codebase_id="cb_xxx",
    permissions=[{"pattern": "**/*", "permission": "read"}],
)

# 启动(分配资源,挂载 FUSE)
client.start_sandbox(sandbox.id)

# 执行命令
result = client.exec(sandbox.id, command="ls /workspace")
print(result.stdout)

# 停止(卸载 FUSE,停止进程)
client.stop_sandbox(sandbox.id)

# 销毁(清理所有资源)
client.destroy_sandbox(sandbox.id)
```

## Permission

**权限定义 agent 对 Codebase 中每个文件或目录可以做什么**。

### 四级权限

AgentFense 提供四个级别的文件访问控制:

| 级别 | 可见性 | 读取内容 | 写入/修改 | 示例使用场景 |
|------|--------|----------|----------|--------------|
| **none** | ❌ 不可见 | ❌ | ❌ | 隐藏密钥、凭证、私钥 |
| **view** | ✅ 在 `ls` 中 | ❌ | ❌ | 显示文件结构而不暴露内容 |
| **read** | ✅ | ✅ | ❌ | 源代码、文档、日志(只读) |
| **write** | ✅ | ✅ | ✅ | 输出目录、临时文件、生成的报告 |

### 权限模式

支持三种类型的模式:

| 类型 | 语法 | 示例 | 匹配 |
|------|------|------|------|
| **glob** | 通配符模式 | `**/*.py` | 所有 Python 文件(递归) |
|  |  | `/secrets/**` | `/secrets` 下的所有文件 |
|  |  | `*.env*` | `.env`、`.env.local`、`.env.production` |
| **directory** | 以 `/` 结尾的路径 | `/docs/` | `/docs` 及子目录中的所有文件 |
| **file** | 精确文件路径 | `/config.yaml` | 仅 `/config.yaml` |

### 优先级算法

当多个规则匹配一个路径时,AgentFense 使用这个确定性优先级:

1. **显式优先级**(如果通过 `priority` 字段设置)
2. **模式类型**: `file` (3) > `directory` (2) > `glob` (1)
3. **模式特异性**:更具体的模式获胜

示例:

```python
permissions = [
    {"pattern": "**/*", "permission": "read"},           # 优先级:0 (glob)
    {"pattern": "/secrets/**", "permission": "none"},    # 优先级:0 (glob)
    {"pattern": "/secrets/public.key", "permission": "read"},  # 优先级:0 (file)
]

# 结果:
# - /secrets/private.key → none (被 /secrets/** 匹配)
# - /secrets/public.key → read (file 模式覆盖 glob)
# - /app/main.py → read (被 **/* 匹配)
```

### 示例:构建权限规则

```python
from agentfense import Sandbox

# 场景:允许读取所有,写入 /output,隐藏密钥
PERMISSIONS = [
    # 基础规则:读取所有
    {"pattern": "**/*", "permission": "read", "priority": 0},
    
    # 允许写入输出目录
    {"pattern": "/output/**", "permission": "write", "priority": 10},
    
    # 隐藏密钥(更高优先级覆盖基础)
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    
    # 例外:公钥应该可读
    {"pattern": "/secrets/public.key", "permission": "read", "priority": 200},
]

with Sandbox.from_local("./project", permissions=PERMISSIONS) as sandbox:
    # 可以读取大多数文件
    sandbox.run("cat /workspace/README.md")  # ✅
    
    # 可以写入 /output
    sandbox.run("echo result > /workspace/output/log.txt")  # ✅
    
    # 看不到私钥
    sandbox.run("ls /workspace/secrets/")  # 仅 public.key(没有 private.key)
    
    # 可以读取公钥
    sandbox.run("cat /workspace/secrets/public.key")  # ✅
```

## Session

**Session 是持久化的 shell 进程**,在多次命令执行中维护状态。

### 为什么使用 Session?

没有 session,每个 `exec()` 都启动新的 shell:

```python
# 每个命令在新 shell 中运行(状态丢失)
sandbox.run("cd /workspace/src")      # 改变目录
sandbox.run("pwd")                    # 输出:/workspace(不是 /workspace/src)
```

有了 session,状态持久化:

```python
# 命令在同一个 shell 中运行(状态保留)
with sandbox.session() as session:
    session.exec("cd /workspace/src")
    result = session.exec("pwd")
    print(result.stdout)  # 输出:/workspace/src ✅
```

### Session 保留什么

- **工作目录**: `cd` 命令持久化
- **环境变量**: `export VAR=value` 持久化
- **Shell 状态**:激活的 virtualenv、source 的文件
- **后台进程**:用 `&` 启动的进程继续运行

### 示例:多步骤工作流

```python
from agentfense import Sandbox

with Sandbox.from_local("./python-app") as sandbox:
    with sandbox.session() as session:
        # 设置环境
        session.exec("cd /workspace")
        session.exec("export DEBUG=1")
        session.exec("source venv/bin/activate")
        
        # 安装依赖(venv 已激活)
        session.exec("pip install -r requirements.txt")
        
        # 运行测试(DEBUG=1 和 venv 仍然活跃)
        result = session.exec("pytest tests/")
        print(result.stdout)
```

### Session 生命周期

```
创建 → 执行(多次) → 关闭
```

Session 在以下情况自动关闭:
- 上下文管理器退出(`with session:`)
- Sandbox 停止或销毁
- 超时发生(可配置)

### 低层 Session API

```python
client = SandboxClient()

# 创建 session
session = client.create_session(
    sandbox_id=sandbox.id,
    shell="/bin/bash",  # 或 /bin/sh, /bin/zsh
    env={"PYTHONPATH": "/workspace/lib"},
)

# 执行命令
result = client.session_exec(session.id, command="cd /workspace")
result = client.session_exec(session.id, command="ls")

# 关闭 session(可选 - sandbox 停止时自动关闭)
client.close_session(session.id)
```

## Preset

**Preset 是预配置的权限模板**,用于常见使用场景。

### 内置 Preset

| Preset | 描述 | 使用场景 |
|--------|------|----------|
| **agent-safe** | 读取所有,写入 `/output` & `/tmp`,隐藏密钥 | AI agent 默认值(安全探索 + 受控输出) |
| **read-only** | 读取所有文件,无写入权限 | 代码审查、静态分析、安全浏览 |
| **full-access** | 读写所有文件 | 可信自动化、测试、开发 |
| **development** | 除密钥外的完全访问 | 带安全防护的本地开发 |
| **view-only** | 查看文件名,无法读取内容 | 探索文件结构而不暴露数据 |

### 使用 Preset

```python
from agentfense import Sandbox

# 直接使用 preset
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    result = sandbox.run("ls")
```

### 扩展 Preset

```python
from agentfense import extend_preset

# 从 agent-safe 开始,添加自定义规则
rules = extend_preset(
    base="agent-safe",
    additions=[
        {"pattern": "/logs/**", "permission": "write"},  # 也允许日志写入
    ],
    overrides=[
        {"pattern": "**/.git/**", "permission": "read"},  # 覆盖:使 .git 可读
    ],
)

with Sandbox.from_local("./project", permissions=rules) as sandbox:
    result = sandbox.run("echo log > /workspace/logs/app.log")  # ✅
```

### 创建自定义 Preset

```python
from agentfense import register_preset

# 定义自定义 preset
register_preset("ci-pipeline", [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/build/**", "permission": "write"},
    {"pattern": "/dist/**", "permission": "write"},
    {"pattern": "**/node_modules/**", "permission": "none"},
    {"pattern": "**/.env*", "permission": "none"},
])

# 像内置 preset 一样使用
with Sandbox.from_local("./app", preset="ci-pipeline") as sandbox:
    sandbox.run("npm run build")
```

## Delta Layer (写时复制)

**Delta Layer 提供写入隔离**,当多个 Sandbox 共享同一个 Codebase 时。

### 问题:写入冲突

没有隔离时,多个 Sandbox 写入同一个 Codebase 会冲突:

```
Sandbox A 写入 /workspace/output.txt → 覆盖
Sandbox B 写入 /workspace/output.txt → 冲突!
```

### 解决方案:每个 Sandbox 的 Delta Layer

每个 Sandbox 写入自己的 **delta 目录**:

```
Codebase (不可变):    /var/lib/agentfense/codebases/cb_123/
Sandbox A delta:      /var/lib/agentfense/mounts/sb_A/delta/
Sandbox B delta:      /var/lib/agentfense/mounts/sb_B/delta/
```

### 工作原理

1. **读取**:先检查 delta,回退到源 Codebase
2. **写入**:总是写入 delta(源保持不变)
3. **删除**:创建白化标记(`.wh.<filename>`)
4. **同步**:执行完成时,delta 同步到源(Last-Writer-Wins)

### 行为

| 操作 | 发生什么 |
|------|----------|
| 读取 `/workspace/file.txt` | 如果 delta 中存在,从 delta 读取;否则从 codebase 读取 |
| 写入 `/workspace/file.txt` | 写入 `delta/file.txt`(codebase 不变) |
| 删除 `/workspace/file.txt` | 创建 `delta/.wh.file.txt`(白化标记) |
| 执行完成 | 同步 delta → codebase(冲突时 LWW) |

### 示例

```python
from agentfense import Sandbox

# 两个 sandbox 共享同一个 codebase
with Sandbox.from_local("./project") as sandbox_a:
    sandbox_a.run("echo A > /workspace/output.txt")
    
with Sandbox.from_local("./project") as sandbox_b:
    sandbox_b.run("echo B > /workspace/output.txt")

# 两者完成后,codebase 有:
# - output.txt 内容为 "B"(Last-Writer-Wins)
```

### 何时关心 Delta Layer

- **多 sandbox 并发**:在同一个 codebase 上运行多个 agent
- **写入安全**:确保 sandbox 不会破坏彼此的数据
- **回滚**:Delta 更改是临时的(如果 sandbox 崩溃会丢失)

### 禁用同步(临时写入)

如果不希望写入持久化到 codebase:

```python
# 选项 1:使用只读 preset
with Sandbox.from_local("./project", preset="read-only") as sandbox:
    # 所有写入都失败(权限拒绝)
    pass

# 选项 2:允许写入但不同步(自定义服务器配置)
# 在服务器配置中设置 sync_on_completion=false
```

## 总结

| 概念 | 是什么 | 关键洞察 |
|------|--------|----------|
| **Codebase** | 服务器上的文件存储 | 不可变的真实来源,可共享 |
| **Sandbox** | 隔离的执行环境 | 在文件系统层面强制权限 |
| **Permission** | 访问控制规则 | 四个级别(none/view/read/write)带优先级 |
| **Session** | 持久化 shell 进程 | 保留工作目录和环境 |
| **Preset** | 预配置的权限模板 | 常见场景的可重用模式 |
| **Delta Layer** | 每个沙盒的写入隔离 | COW 带完成时同步(LWW) |

## 下一步

- **[安装指南](installation.md)** - 设置服务器和依赖
- **[安全模型](../security/index.md)** - 权限如何强制执行
- **[权限系统](../security/permissions.md)** - 深入规则和模式
- **[Preset 指南](../security/presets.md)** - 使用和扩展 preset
- **[最佳实践](../security/best-practices.md)** - 生产部署模式
