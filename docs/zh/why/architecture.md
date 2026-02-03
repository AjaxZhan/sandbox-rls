# 架构概览

本页面从开发者视角解释 **AgentFense 的内部工作原理**。我们关注**关键机制**(权限检查、Delta Layer、FUSE)和**数据流**(运行命令时发生什么)。

---

## TL;DR: 工作原理

**一句话版本**: AgentFense 将你的代码库挂载为 FUSE 文件系统,拦截每个文件操作(`open`、`readdir`、`stat`),实时检查权限,并通过 Copy-On-Write 隔离写入——所有这些都在轻量级命名空间(bwrap)或容器(Docker)中进行。

**为什么重要**:
- **权限检查发生在系统调用级别** → 即使恶意代码也无法绕过(不只是"希望" Agent 遵守规则)
- **密钥真正不可见** → `none` 权限意味着文件不会出现在 `ls`、`find` 或任何系统调用中
- **多沙盒隔离** → 100 个沙盒共享 1 个代码库(10GB),但写入不冲突(Delta Layer COW)

---

## 四层架构

AgentFense 设计为四层,每层有明确的职责:

```
┌─────────────────────────────────────────────────────────────┐
│                   层 1: 客户端                               │
│      Python SDK  │  Go SDK (未来)  │  REST API              │
│                                                              │
│  用户代码:                                                   │
│    with Sandbox.from_local("./repo") as sb:                 │
│        sb.run("grep TODO *.py")                              │
└──────────────────────────┬──────────────────────────────────┘
                           │ gRPC / REST
┌──────────────────────────▼──────────────────────────────────┐
│                   层 2: 服务                                 │
│   gRPC 服务器  │  REST 网关  │  代码库管理器                │
│                                                              │
│  职责:                                                       │
│  - 管理代码库存储(上传、列表、删除)                          │
│  - 沙盒生命周期(创建、启动、执行、停止)                      │
│  - 协调运行时 + 文件系统 + 权限                             │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                   层 3: 运行时                               │
│   运行时管理器  │  权限引擎  │  执行器                       │
│                                                              │
│  实现:                                                       │
│  - bwrap: 轻量级(Linux 命名空间 + seccomp)                  │
│  - docker: 完全隔离(容器 + 资源限制)                        │
│  - mock: 测试(无实际执行)                                   │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                   层 4: 隔离                                 │
│   FUSE 文件系统  │  Delta Layer (COW)  │  命名空间          │
│                                                              │
│  机制:                                                       │
│  - FUSE: 拦截每个系统调用,强制执行权限                       │
│  - Delta: Copy-On-Write 用于多沙盒写入隔离                  │
│  - bwrap/Docker: 进程/网络/IPC 隔离                         │
└─────────────────────────────────────────────────────────────┘
```

### 层 1: 客户端(面向用户的 SDK)

**目的**: 为开发者提供符合人体工程学的 API,用于创建沙盒、运行命令、管理权限。

**组件**:
- **Python SDK** (`sdk/python/agentfense/`):
  - 高级: `Sandbox.from_local()`(一行代码)
  - 低级: `SandboxClient`(完全控制)
  - 异步支持: `AsyncSandbox`(高并发)
- **REST API** (通过 grpc-gateway 的 HTTP 网关):
  - 用于非 Python/Go 语言
  - 标准 CRUD 操作代码库、沙盒、执行

**示例**:
```python
from agentfense import Sandbox

# 高级 API 抽象了所有 4 层
with Sandbox.from_local("./repo", preset="agent-safe") as sandbox:
    result = sandbox.run("grep TODO *.py")  # 权限透明强制执行
```

---

### 层 2: 服务(gRPC + 业务逻辑)

**目的**: 管理代码库存储、沙盒生命周期,协调运行时和文件系统。

**组件**:
- **gRPC 服务器** (`internal/server/server.go`):
  - 实现 `CodebaseService`(创建、上传、列表、删除)
  - 实现 `SandboxService`(创建、启动、执行、停止、销毁)
  - 验证权限规则,规范化路径
- **代码库管理器** (`internal/codebase/manager.go`):
  - 在磁盘上存储代码库(`storage.codebase_path`)
  - 分配唯一 ID(`cb_<uuid>`)
  - 跟踪元数据(名称、所有者、创建时间)
- **REST 网关**(通过 grpc-gateway 自动生成):
  - 翻译 HTTP → gRPC(无需额外代码)
  - 支持 `curl` 访问、Web 前端集成

**沙盒创建时的数据流**:
1. 客户端: `create_sandbox(codebase_id, permissions)`
2. 服务器验证权限规则(模式、类型)
3. 服务器分配沙盒 ID(`sb_<uuid>`)
4. 服务器创建 delta 目录(`<mount_path>/delta/<sandbox_id>`)
5. 服务器通知运行时层:"准备沙盒"

---

### 层 3: 运行时(隔离 + 执行)

**目的**: 在隔离环境(命名空间/容器)中执行命令,强制执行资源限制。

**组件**:
- **运行时接口** (`internal/runtime/runtime.go`):
  - `Create()`: 准备沙盒环境
  - `Start()`: 激活隔离(挂载 FUSE,启动容器)
  - `Exec()`: 运行命令,返回 stdout/stderr/exit_code
  - `Stop()`, `Destroy()`: 清理

**实现**:

#### bwrap 运行时 (`internal/runtime/bwrap/`)
- **机制**: Linux 命名空间(mount、PID、network、IPC) + bubblewrap
- **优势**: 轻量级(<1s 启动),低开销(~5MB/沙盒)
- **限制**: 仅 Linux,需要 `bwrap` 二进制,隔离比 VM 弱
- **用例**: 高并发(200+ 沙盒),受信任的代码

**工作原理**:
```bash
# AgentFense 在底层运行这个
bwrap \
  --ro-bind /usr /usr \
  --bind /path/to/fuse-mount /workspace \  # 带权限的 FUSE 挂载
  --unshare-all \
  --die-with-parent \
  bash -c "grep TODO *.py"
```

#### Docker 运行时 (`internal/runtime/docker/`)
- **机制**: Docker 容器与绑定挂载
- **优势**: 完全隔离(进程/网络/文件系统),自定义镜像,资源限制(CPU/内存/PIDs)
- **限制**: 启动较慢(~5-10s),开销较高(~512MB/容器)
- **用例**: 不可信代码,合规要求,需要自定义环境

**工作原理**:
```bash
# AgentFense 在底层运行这个
docker run --rm \
  -v /path/to/fuse-mount:/workspace \  # FUSE 挂载(权限强制执行)
  --memory=512m --cpus=1 --pids-limit=100 \
  --network=none \  # 可选:禁用网络
  python:3.11-slim bash -c "grep TODO *.py"
```

#### Mock 运行时 (`internal/runtime/mock/`)
- **目的**: 仅测试(无实际执行,返回空结果)
- **用例**: 服务器/API 逻辑测试,无隔离开销

**会话支持**(有状态 Shell):
- **问题**: 每个 `exec()` 调用启动新 shell → 丢失 `cd`、环境变量
- **解决方案**: `SessionManager` 维护持久 shell 进程
- **实现**: 通过持久 bash 进程多路复用 stdin/stdout

```python
with sandbox.session() as session:
    session.exec("cd /workspace/src")  # 状态持久化
    result = session.exec("pwd")       # → /workspace/src
    session.exec("export VAR=value")
    result = session.exec("echo $VAR") # → value
```

---

### 层 4: 隔离(FUSE + Delta + 命名空间)

**目的**: 在内核级强制执行权限,跨沙盒隔离写入,防止进程逃逸。

**组件**:

#### FUSE 文件系统 (`internal/fs/fuse.go`)

**什么是 FUSE?**
- **用户空间文件系统**: 允许你将自定义文件系统实现为常规程序(无需内核模块)
- **工作原理**: 内核将系统调用(`open`、`readdir`、`stat`)转发到你的程序 → 你的程序决定返回什么
- **生产使用**: Google (GCS Fuse)、Dropbox (文件同步)、S3 挂载、加密文件系统

**AgentFense 的 FUSE 实现**:
1. **挂载代码库**: 映射 `/path/to/source` → `/path/to/fuse-mount`
2. **拦截系统调用**: 每个 `open()`、`readdir()`、`stat()` 都通过 FUSE
3. **检查权限**: 对于每个操作,检查权限规则 → 允许或拒绝
4. **返回结果**: 如果允许,代理到源;如果拒绝,返回 `EACCES`;如果 `none`,返回 `ENOENT`(文件不存在)

**权限强制执行示例**:

| 系统调用 | 路径 | 权限 | 结果 |
|---------|------|------|------|
| `open("/workspace/src/main.py", O_RDONLY)` | `/src/main.py` | `read` | ✅ 返回文件描述符 |
| `open("/workspace/secrets/.env", O_RDONLY)` | `/secrets/.env` | `none` | ❌ 返回 `ENOENT`(无此文件) |
| `open("/workspace/docs/README.md", O_WRONLY)` | `/docs/README.md` | `read`(非 write) | ❌ 返回 `EACCES`(权限拒绝) |
| `readdir("/workspace")` | `/` | - | 返回列表不含 `/secrets`(不可见) |

**为什么用 FUSE,而非 LD_PRELOAD 或系统调用 hook?**
- **LD_PRELOAD**: 只拦截 libc 调用,易于绕过(直接系统调用)
- **系统调用 hook** (ptrace): 高开销,脆弱(破坏调试器)
- **FUSE**: 内核强制执行,无法绕过,标准 Linux 特性

#### 权限引擎 (`internal/fs/permission.go`)

**权限模型**:

```
模式类型:
  - file: 精确匹配 (/config.yaml)
  - directory: 前缀匹配 (/docs/ 匹配 /docs/README.md)
  - glob: 通配符 (**/*.py, /secrets/**)

权限级别:
  - none: 不可见(不出现在 ls 中,表现为不存在)
  - view: 仅列表(可以在 ls 中看到名称,无法读取内容)
  - read: 可以读取文件内容
  - write: 可以读取 + 修改/创建文件
```

**优先级算法**(确定性规则解析):
1. **显式 Priority 字段**(如果用户设置)
2. **模式类型**: file (3) > directory (2) > glob (1)
3. **模式特异性**: `/secrets/public.key` > `/secrets/**` > `**/*`

**示例解析**:
```python
rules = [
    {"pattern": "**/*", "permission": "read"},           # 优先级: 1 (glob)
    {"pattern": "/secrets/**", "permission": "none"},    # 优先级: 1 (glob,但更具体)
    {"pattern": "/secrets/public.key", "permission": "read"},  # 优先级: 3 (file)
]

# 查询: /secrets/public.key
# 匹配: 所有 3 条规则匹配
# 胜者: /secrets/public.key (最高优先级: file > glob)
# 结果: read (公钥是可读例外)
```

#### Delta Layer (Copy-On-Write) (`internal/fs/delta.go`)

**问题**: 多个沙盒共享同一代码库 → 如果一个写入 `/output/report.txt`,其他不应看到它(隔离),但每个沙盒复制整个代码库成本高(100 个用户需 1TB)。

**解决方案**: 文件级 Copy-On-Write (COW)。

**工作原理**:

```
目录结构:
/storage/
  codebases/
    cb_001/          ← 源代码库(10GB,共享)
      src/
      docs/
      tests/
  mounts/
    delta/
      sb_user_a/     ← 用户 A 的更改(5MB)
        output/
          report.txt
      sb_user_b/     ← 用户 B 的更改(5MB)
        output/
          results.json
    mount_sb_user_a/  ← 用户 A 的 FUSE 挂载
    mount_sb_user_b/  ← 用户 B 的 FUSE 挂载
```

**读取逻辑**(FUSE 拦截读取的 `open`):
1. 检查 delta 目录: `/storage/mounts/delta/sb_user_a/path/to/file`
2. 如果存在 → 返回 delta 版本(用户的更改)
3. 如果不存在 → 回退到源: `/storage/codebases/cb_001/path/to/file`

**写入逻辑**(FUSE 拦截写入的 `open`):
1. 写入到 delta: `/storage/mounts/delta/sb_user_a/path/to/file`
2. 如需要创建父目录
3. 源保持不变(其他沙盒看到原始)

**删除逻辑**(白化标记):
1. 在 delta 中创建 `.wh.<filename>`(例如 `.wh.old.txt`)
2. 读取时: 如果 `.wh.old.txt` 存在 → 返回 `ENOENT`(文件已删除)
3. 源文件仍存在(其他沙盒不受影响)

**同步逻辑**(在 `exec()` 完成时):
- 选项 1: **不同步**(临时沙盒的默认值)
- 选项 2: **同步到源**(持久化更改的 Last-Writer-Wins)
  - 复制 delta 文件 → 源
  - 删除白化标记(应用删除)
  - 冲突解决: 较新时间戳获胜

**资源使用**:
- 100 个沙盒,10GB 代码库,每个写入 5MB:
  - 传统: 10GB × 100 = 1TB
  - Delta Layer: 10GB + 5MB × 100 = 10.5GB (减少 98%)

---

## 数据流:运行命令时发生什么

让我们追踪从客户端到系统调用强制执行的完整请求。

### 分步示例

**用户代码**:
```python
from agentfense import Sandbox

with Sandbox.from_local(
    "./repo",
    permissions=[
        {"pattern": "**/*", "permission": "read"},
        {"pattern": "/secrets/**", "permission": "none"},
    ],
    runtime="bwrap",
) as sandbox:
    result = sandbox.run("cat /workspace/src/main.py")
```

**时间线**(底层发生什么):

#### 阶段 1: 沙盒创建(设置)

1. **客户端 → gRPC**: `CreateCodebase(name="repo")`
   - 服务器分配 ID: `cb_12345`
   - 创建目录: `/storage/codebases/cb_12345/`

2. **客户端 → gRPC**: `UploadFiles(codebase_id="cb_12345", files=...)`
   - 复制 `./repo/*` → `/storage/codebases/cb_12345/`

3. **客户端 → gRPC**: `CreateSandbox(codebase_id="cb_12345", permissions=[...])`
   - 服务器验证规则
   - 分配 ID: `sb_67890`
   - 创建 delta: `/storage/mounts/delta/sb_67890/`

4. **客户端 → gRPC**: `StartSandbox(sandbox_id="sb_67890")`
   - 服务器通知运行时:"启动沙盒"
   - 运行时启动 FUSE 挂载:
     - 源: `/storage/codebases/cb_12345/`
     - Delta: `/storage/mounts/delta/sb_67890/`
     - 挂载点: `/storage/mounts/mount_sb_67890/`
     - 权限规则: 加载到 FUSE 服务器
   - 运行时准备 bwrap 命令(但还不执行)

#### 阶段 2: 命令执行

5. **客户端 → gRPC**: `Exec(sandbox_id="sb_67890", command="cat /workspace/src/main.py")`
   - 服务器询问运行时:"执行命令"
   - 运行时运行:
     ```bash
     bwrap \
       --bind /storage/mounts/mount_sb_67890 /workspace \
       bash -c "cat /workspace/src/main.py"
     ```

6. **在 bwrap 命名空间内**:
   - 进程启动: `bash -c "cat /workspace/src/main.py"`
   - `cat` 调用 `open("/workspace/src/main.py", O_RDONLY)`
   - **内核 → FUSE**: "对 /workspace/src/main.py 的 open 请求"

7. **FUSE 服务器** (AgentFense):
   - 规范化路径: `/workspace/src/main.py` → `/src/main.py`
   - 检查权限: 匹配规则,找到 `**/*` → `read` 权限
   - 检查 delta: `/storage/mounts/delta/sb_67890/src/main.py` → 不存在
   - 回退到源: `/storage/codebases/cb_12345/src/main.py` → 存在
   - 日志: `[INFO] Read allowed: /src/main.py`
   - 向内核返回文件描述符

8. **内核 → cat**: 返回文件描述符
   - `cat` 读取文件内容
   - 打印到 stdout

9. **运行时 → 服务器**: 命令完成(stdout, stderr, exit_code=0)

10. **服务器 → 客户端**: 返回 `ExecuteResult(stdout="...", exit_code=0)`

#### 阶段 3: 权限拒绝(如果 Agent 尝试读取密钥?)

**用户运行**: `sandbox.run("cat /workspace/secrets/.env")`

6. **在 bwrap 内**: `cat` 调用 `open("/workspace/secrets/.env", O_RDONLY)`

7. **FUSE 服务器**:
   - 规范化: `/workspace/secrets/.env` → `/secrets/.env`
   - 检查权限: 匹配 `/secrets/**` → `none` 权限
   - 日志: `[WARN] Path invisible: /secrets/.env (permission: none)`
   - 返回 `ENOENT`(无此文件或目录)

8. **内核 → cat**: 错误 `ENOENT`
   - `cat` 打印: `cat: /workspace/secrets/.env: No such file or directory`
   - 退出码: 1

**关键洞察**: Agent 无法判断 `.env` 是否存在或从未存在——它真正不可见。

---

## 关键机制解释

### 1. 为什么密钥不可见(不只是被阻止)

**传统黑名单**(返回"权限拒绝"):
```
$ ls /workspace
src/  docs/  .env  ← Agent 看到 .env 存在
$ cat .env
cat: .env: Permission denied  ← Agent 知道文件是敏感的
```

**AgentFense `none` 权限**(返回"无此文件"):
```
$ ls /workspace
src/  docs/  ← .env 不出现
$ cat .env
cat: .env: No such file or directory  ← Agent 零知识
```

**实现**:
- `readdir()`(列出目录): 返回前过滤掉 `none` 路径
- `stat()`(检查文件存在): 对 `none` 路径返回 `ENOENT`
- `open()`: 对 `none` 路径返回 `ENOENT`

### 2. `view` 权限(仅列表)

**用例**: Agent 需要看到"存在哪些配置"但无法读取值。

```python
{"pattern": "/configs/**", "permission": "view"}
```

**行为**:
- `readdir("/workspace/configs")` → 返回 `["api.yaml", "db.yaml"]` ✅
- `open("/workspace/configs/api.yaml", O_RDONLY)` → `EACCES`(权限拒绝) ❌

**实现**:
- `readdir()`: 允许(显示文件名)
- 读取的 `open()`: 拒绝(`EACCES`)
- 用于文档生成、模式发现

### 3. 多沙盒隔离(Delta Layer)

**场景**: 2 个用户在同一代码库上运行 Agent,都写入 `/output/report.txt`。

**没有 Delta Layer**(传统 Docker):
- 用户 A 的容器: 写入 `/output/report.txt`(内容:"A 的数据")
- 用户 B 的容器: 写入 `/output/report.txt`(内容:"B 的数据")
- **问题**: 最后写入获胜,A 的数据丢失(或需要单独卷 → 1TB 存储)

**使用 Delta Layer**:
- 用户 A 的沙盒:
  - 写入: `/storage/mounts/delta/sb_user_a/output/report.txt`(内容:"A 的数据")
  - 读取: Delta 优先,回退到源
  - 看到: `/output/report.txt` = "A 的数据"

- 用户 B 的沙盒:
  - 写入: `/storage/mounts/delta/sb_user_b/output/report.txt`(内容:"B 的数据")
  - 读取: Delta 优先,回退到源
  - 看到: `/output/report.txt` = "B 的数据"

- 源: `/storage/codebases/cb_001/output/`(空,未触及)

**结果**: 两个用户都看到自己的数据,零冲突,10GB 共享基础 + 每用户 5MB。

---

## 性能特征

### 基准测试(1 个沙盒,bwrap 运行时)

| 操作 | 相对原生的开销 |
|------|---------------|
| `open()`(权限检查) | +50μs (0.05ms) |
| `readdir()`(过滤 `none` 路径) | 1000 个文件 +100μs |
| `grep -r "TODO" src/`(10,000 个文件) | +3% (FUSE 开销) |
| 命令启动 | +200ms (FUSE 挂载) |

### 可扩展性(100 个并发沙盒)

| 运行时 | 存储 | 内存 | CPU | 最大并发 |
|--------|------|------|-----|---------|
| bwrap | 10.5GB | 800MB | 10% (空闲) | 200+ |
| Docker | 10.5GB | 1.3GB | 15% (空闲) | 150+ |

**瓶颈**:
- 内存: 每个 FUSE 挂载 + delta ~8MB
- 文件描述符: 每个沙盒打开 ~50 FD
- CPU: FUSE 系统调用处理(最小)

---

## 故障模式与恢复

### 1. FUSE 挂载失败

**症状**: `fusermount: mount failed: Transport endpoint not connected`

**原因**: FUSE 进程崩溃,陈旧挂载

**恢复**:
- 卸载: `fusermount -u /path/to/mount`
- 检查日志: FUSE 服务器 stderr
- 重启沙盒

### 2. Delta 同步冲突(LWW)

**症状**: 用户 A 和 B 都编辑了 `/src/file.py`,同步使用 Last-Writer-Wins

**结果**: B 的更改覆盖 A 的(如果 B 最后完成)

**缓解**:
- 使用临时沙盒(不同步 delta → 源)
- 实现 3-way 合并(未来)
- 多沙盒编辑期间锁定文件(未来)

### 3. 权限拒绝调试

**症状**: Agent 意外得到"权限拒绝"

**调试**:
1. 检查 FUSE 日志: `[WARN] Write denied: /src/main.py (permission: read)`
2. 验证权限规则: 模式是否匹配路径?
3. 检查优先级: 是否有更具体的规则覆盖?

---

## 下一步

- **动手试用**: [快速开始](../quickstart.md)
- **真实场景**: [使用场景](./use-cases.md)
- **对比替代方案**: [vs. Docker/E2B](./comparison.md)
- **贡献**: [GitHub 仓库](https://github.com/AjaxZhan/AgentFense)
