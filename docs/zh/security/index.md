# 安全模型

AgentFense 在**文件系统层面强制执行最小权限访问控制**,即使不可信的 AI agent 尝试访问,也会阻止其访问敏感数据。本页解释安全模型、威胁边界和信任保证。

## TL;DR

**AgentFense 使文件不可见,而不仅仅是不可访问。** 对 `/secrets/.env` 具有 `none` 权限的 agent 无法:
- 读取文件(权限拒绝)
- 在 `ls` 输出中看到文件(不可见)
- Stat 文件(返回"文件未找到")
- 通过任何文件系统操作推断其存在

这在 **FUSE 文件系统层**强制执行,而不是应用逻辑。Agent 无法绕过它。

## 为什么是文件系统级控制?

传统沙盒方法依赖于**应用层权限检查**:

```python
# 传统方法(脆弱)
if user.can_read(path):
    return open(path).read()
else:
    raise PermissionError
```

**问题:**
- 每个代码路径都必须检查权限(容易遗漏)
- Agent 可以通过捕获异常探测文件
- 文件系统元数据(大小、存在性)通常会泄漏
- 开发者必须记得添加检查

**AgentFense 方法:** 在 **FUSE 文件系统**中强制权限:

```python
# AgentFense:无需检查
open(path).read()  # 如果不允许,FUSE 在内核级别拒绝
```

**优势:**
- 零信任:agent 无法绕过权限
- 无需代码更改:与现有工具(`cat`、`grep`、`ls`)一起工作
- 信息隐藏:具有 `none` 权限的文件不可见
- 深度防御:内核/容器级隔离 + FUSE 强制执行

## 安全架构

```
┌─────────────────────────────────────────────────┐
│                 Agent 进程                      │
│          (不可信代码执行)                        │
└─────────────────┬───────────────────────────────┘
                  │ open("/workspace/secret.txt")
                  ▼
┌─────────────────────────────────────────────────┐
│          FUSE 文件系统层                         │
│      (权限强制 + 隐藏)                           │
│   - 检查 "/secret.txt" 的权限                   │
│   - 如果 "none":返回 ENOENT(未找到)              │
│   - 如果 "view":允许 stat/list,拒绝读取          │
│   - 如果 "read":允许读取,拒绝写入                │
│   - 如果 "write":允许所有操作                    │
└─────────────────┬───────────────────────────────┘
                  │ (如果允许)
                  ▼
┌─────────────────────────────────────────────────┐
│              Delta Layer (COW)                  │
│   - 读取:检查 delta,回退到源                     │
│   - 写入:总是写入每个沙盒的 delta                │
│   - 删除:创建白化标记                            │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│            Codebase 存储                        │
│         (不可变源文件)                           │
└─────────────────────────────────────────────────┘
```

## 四级权限

AgentFense 通过四个权限级别提供**细粒度控制**:

### 1. **none** - 不可见(最高安全性)

**功能:**
- 文件**不出现**在目录列表中(`ls`、`find`、`tree`)
- 所有文件系统操作返回 **ENOENT**("没有这样的文件或目录")
- Agent 无法通过任何方式推断文件存在

**使用场景:**
- **密钥**: `.env`、`credentials.json`、`*.key`、`*.pem`
- **私有数据**: `/secrets/**`、`/internal/**`、`/private/**`
- **敏感配置**: API 密钥、数据库密码、令牌

**示例:**

```python
permissions = [
    {"pattern": "**/.env*", "permission": "none"},
    {"pattern": "/secrets/**", "permission": "none"},
]

# Agent 尝试访问 .env
$ ls -la /workspace/
# 输出:(未列出 .env 文件)

$ cat /workspace/.env
# 输出:cat: /workspace/.env: No such file or directory

$ [ -f /workspace/.env ] && echo "exists"
# 输出:(无 - 文件测试返回 false)
```

### 2. **view** - 仅列表(元数据访问)

**功能:**
- 文件**出现在目录列表中**(名称、大小、时间戳)
- **无法读取内容**(打开/读取操作被拒绝)
- 用于显示文件结构而不暴露数据

**使用场景:**
- **大型数据集**:显示数据目录而不暴露实际数据
- **元数据探索**:Agent 可以看到可用文件
- **文档结构**:浏览文档而不读取内容

**示例:**

```python
permissions = [
    {"pattern": "/metadata/**", "permission": "view"},
]

# Agent 可以看到文件结构
$ ls -lh /workspace/metadata/
# 输出:schema.json (1.2KB), config.yaml (850B)

# 但无法读取内容
$ cat /workspace/metadata/schema.json
# 输出:cat: /workspace/metadata/schema.json: Permission denied
```

**注意:** `view` 权限在 macOS 上使用 Docker Desktop 时有已知限制(VirtioFS 问题)。使用 `read` 作为后备。

### 3. **read** - 只读(代码的默认值)

**功能:**
- 文件出现在列表中
- 可以读取内容(`cat`、`grep`、`diff`)
- **无法修改**(写入/追加/删除被拒绝)

**使用场景:**
- **源代码**:对应用代码的只读访问
- **文档**:浏览文档而不修改
- **配置文件**:读取配置而不更改
- **日志**:查看日志而不篡改

**示例:**

```python
permissions = [
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "/docs/**", "permission": "read"},
]

# Agent 可以读取文件
$ cat /workspace/app.py
# 输出:(文件内容)

# 但无法修改
$ echo "malicious" >> /workspace/app.py
# 输出:bash: /workspace/app.py: Permission denied
```

### 4. **write** - 完全访问(谨慎使用)

**功能:**
- 完全读/写访问
- 可以创建、修改、删除文件
- 应该只授予**受控输出目录**

**使用场景:**
- **输出目录**: `/output/**`、`/results/**`、`/artifacts/**`
- **临时目录**: `/tmp/**`、`/workspace/tmp/**`
- **构建目录**: `/build/**`、`/dist/**`
- **测试文件**(在测试场景中)

**示例:**

```python
permissions = [
    {"pattern": "**/*", "permission": "read"},          # 默认:只读
    {"pattern": "/output/**", "permission": "write"},   # 允许写入输出
    {"pattern": "/tmp/**", "permission": "write"},      # 允许临时文件
]

# Agent 可以写入输出
$ echo "analysis complete" > /workspace/output/result.txt
# 输出:(成功)

# 但无法写入其他路径
$ echo "malicious" > /workspace/config.yaml
# 输出:bash: /workspace/config.yaml: Permission denied
```

## 隔离边界

AgentFense 提供**三层隔离**:

### 层 1:进程隔离

| 运行时 | 机制 | 隔离强度 | 使用场景 |
|--------|------|----------|----------|
| **bwrap** | Linux 命名空间(PID、mount、network、IPC、UTS) | 中等 | 开发、快速迭代 |
| **Docker** | 完整容器带 cgroups + 命名空间 | 强 | 生产、严格隔离 |

**防止:**
- 访问 `/workspace` 外的主机文件系统
- 网络访问(可配置)
- 杀死主机进程
- 逃逸到主机环境

### 层 2:文件系统隔离(FUSE)

**FUSE 挂载**提供:
- 对每个文件操作的权限强制执行
- 文件系统级隐藏(`none` 权限)
- 只读强制执行(`view`、`read` 权限)

**防止:**
- 读取权限规则之外的文件
- 通过文件系统探测发现隐藏文件
- 修改只读文件
- 绕过应用层权限检查

### 层 3:写入隔离(Delta Layer)

**写时复制**提供:
- 每个沙盒的写入隔离
- 保护共享 codebase 免受破坏
- 失败时回滚(临时写入)

**防止:**
- 一个沙盒破坏另一个沙盒的数据
- 对源 codebase 的永久损坏
- 跨并发沙盒的写入冲突

## 威胁模型

### AgentFense 防范的威胁

| 威胁 | 保护 | 如何 |
|------|------|------|
| **密钥暴露** | ✅ 强 | 具有 `none` 权限的文件在内核级别不可见 |
| **源代码篡改** | ✅ 强 | `read` 权限防止写入;delta layer 隔离允许的写入 |
| **权限提升** | ✅ 中等 | bwrap/Docker 隔离防止逃逸到主机 |
| **通过写入的数据泄露** | ✅ 强 | 只有指定的输出目录可写 |
| **文件系统探测** | ✅ 强 | `none` 权限隐藏存在;无元数据泄漏 |
| **资源耗尽** | ⚠️ 中等 | 资源限制(CPU、内存、PID)强制执行上限 |
| **网络攻击** | ⚠️ 可配置 | 默认禁用网络;如需要可启用 |

### AgentFense 不防范的威胁

| 威胁 | 状态 | 缓解措施 |
|------|------|----------|
| **侧信道时序攻击** | ❌ 不保护 | 对 AI agent 不是主要关注点;如关键则使用常量时间加密 |
| **从允许文件推断** | ⚠️ 有限 | Agent 可以从**允许的**文件内容推断;最小化可读文件 |
| **网络数据泄露** | ❌ 如启用网络 | 禁用网络或使用出口过滤 |
| **CPU/GPU 侧信道** | ❌ 不保护 | 对高安全场景使用物理隔离 |
| **内核漏洞利用** | ⚠️ 有限 | Docker 提供比 bwrap 更强的隔离;保持内核更新 |

## 信任边界

AgentFense 建立这些信任边界:

```
┌──────────────────────────────────────────────┐
│              不可信区域                       │
│                                              │
│  ┌────────────────────────────────────┐     │
│  │       Agent 进程                   │     │
│  │   (不可信代码执行)                  │     │
│  └────────────────────────────────────┘     │
│                                              │
│  - 只能按权限访问文件                       │
│  - 无法逃逸隔离                             │
│  - 无法看到/修改隐藏文件                     │
└──────────────────┬───────────────────────────┘
                   │ FUSE + 隔离
┌──────────────────▼───────────────────────────┐
│             可信区域                         │
│                                              │
│  - AgentFense 服务器                        │
│  - 权限引擎                                  │
│  - Codebase 存储                            │
│  - 主机文件系统                              │
└──────────────────────────────────────────────┘
```

**假设:**
1. **服务器可信**:AgentFense 服务器进程以提升的权限运行(用于 FUSE 挂载)
2. **内核可信**:依赖 Linux 内核进行命名空间隔离
3. **配置可信**:权限规则由可信管理员设置

**关键洞察:** 即使 agent 代码被破坏或恶意,它**无法绕过文件系统权限**,因为强制执行发生在其控制之外。

## 强制执行机制

### 1. FUSE 权限检查

每个文件系统操作都触发权限评估:

```go
// FUSE 权限检查的伪代码
func (fs *FuseFS) Open(path string, flags int) (File, error) {
    perm := fs.permEngine.Evaluate(path)
    
    switch perm {
    case PermNone:
        return nil, syscall.ENOENT  // 文件未找到(不可见)
    case PermView:
        if flags & O_RDONLY != 0 {
            return nil, syscall.EACCES  // 读取被拒绝
        }
        return fs.statOnly(path)  // 仅元数据
    case PermRead:
        if flags & O_WRONLY != 0 || flags & O_RDWR != 0 {
            return nil, syscall.EACCES  // 写入被拒绝
        }
        return fs.openRead(path)
    case PermWrite:
        return fs.openWrite(path)  // 完全访问
    }
}
```

### 2. 基于优先级的规则匹配

当多个规则匹配一个路径时,**最高优先级获胜**:

1. 显式 `priority` 字段(如果设置)
2. 模式类型:`file`(3) > `directory`(2) > `glob`(1)
3. 模式特异性(更长/更具体的获胜)

**示例:**

```python
rules = [
    {"pattern": "**/*", "permission": "read"},                  # 优先级:0,类型:glob
    {"pattern": "/secrets/**", "permission": "none"},           # 优先级:0,类型:glob
    {"pattern": "/secrets/public.key", "permission": "read"},   # 优先级:0,类型:file
]

# 结果:
# /app/main.py           → read (被 **/* 匹配)
# /secrets/private.key   → none (被 /secrets/** 匹配)
# /secrets/public.key    → read (file 模式覆盖 glob)
```

**为什么这对安全很重要:**
- **默认拒绝并例外**:设置限制性基础规则,添加特定例外
- **最小权限**:默认为最小访问,仅在需要时授予更多
- **无歧义**:确定性优先级防止意外暴露

### 3. Delta Layer 同步

写入隔离到每个沙盒的 delta 目录:

```
源(不可变):      /var/lib/agentfense/codebases/cb_123/app.py
Sandbox A delta: /var/lib/agentfense/mounts/sb_A/delta/app.py
Sandbox B delta: /var/lib/agentfense/mounts/sb_B/delta/app.py
```

**执行完成时:**
- Delta 同步到源(如有冲突则 Last-Writer-Wins)
- 防止并发写入破坏
- 临时写入(如果沙盒崩溃会丢失)

## 安全最佳实践

### 1. 默认拒绝,显式允许

```python
# 好:从最小权限开始,添加所需内容
permissions = [
    {"pattern": "**/*", "permission": "none"},      # 默认:不可见
    {"pattern": "/src/**", "permission": "read"},   # 显式允许读取源代码
    {"pattern": "/output/**", "permission": "write"}, # 显式允许写入输出
]

# 坏:过度宽松的默认
permissions = [
    {"pattern": "**/*", "permission": "write"},     # 所有可写(危险)
]
```

### 2. 高优先级隐藏密钥

```python
# 确保即使其他规则匹配,密钥也被隐藏
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},  # 高优先级
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
]
```

### 3. 使用 Preset 作为起点

```python
from agentfense import extend_preset

# 从 agent-safe 开始,自定义
rules = extend_preset(
    base="agent-safe",  # 隐藏密钥,允许全部读取,写入 /output & /tmp
    additions=[
        {"pattern": "/logs/**", "permission": "write"},  # 也允许日志写入
    ],
)
```

### 4. 除非需要,否则禁用网络

```yaml
# config.yaml
runtime:
  docker:
    network_mode: "none"         # 无网络访问
    enable_networking: false     # 强制无网络
```

### 5. 设置资源限制

```python
from agentfense import Sandbox, ResourceLimits

with Sandbox.from_local(
    "./project",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 最大 512 MB
        pids_limit=100,                  # 最多 100 个进程
        cpu_quota=50000,                 # 50% CPU
    ),
) as sandbox:
    sandbox.run("untrusted_command")
```

## 审计和监控

跟踪沙盒活动进行安全分析:

### 服务器日志

AgentFense 记录所有沙盒操作:

```json
{
  "level": "info",
  "time": "2026-02-03T10:15:30Z",
  "msg": "sandbox_created",
  "sandbox_id": "sb_abc123",
  "codebase_id": "cb_xyz789",
  "user_id": "user_001",
  "permissions": [...]
}

{
  "level": "info",
  "time": "2026-02-03T10:15:35Z",
  "msg": "command_executed",
  "sandbox_id": "sb_abc123",
  "command": "cat /workspace/app.py",
  "exit_code": 0,
  "duration_ms": 45
}

{
  "level": "warn",
  "time": "2026-02-03T10:15:40Z",
  "msg": "permission_denied",
  "sandbox_id": "sb_abc123",
  "path": "/workspace/.env",
  "operation": "open",
  "permission": "none"
}
```

### 监控内容

| 事件 | 为什么监控 | 告警阈值 |
|------|-----------|----------|
| `permission_denied` | Agent 尝试访问受限文件 | 每个沙盒 >10/分钟 |
| `command_executed` | 所有命令的审计跟踪 | 记录所有用于取证 |
| `sandbox_created` | 跟踪沙盒创建率 | 每用户 >100/小时 |
| `resource_limit_exceeded` | 资源耗尽尝试 | 任何发生 |
| `session_duration` | 长时间运行的会话可能表示问题 | >1 小时 |

## 常见陷阱

### 1. 忘记隐藏 `.git` 目录

```python
# 问题:.git 可能包含敏感提交历史
permissions = [
    {"pattern": "**/*", "permission": "read"},
]

# 解决方案:显式隐藏 .git
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "**/.git/**", "permission": "none"},  # 隐藏 git 历史
]
```

### 2. 模式拼写错误暴露密钥

```python
# 错误:模式中的拼写错误(密钥仍然可见)
{"pattern": "/secret/**", "permission": "none"},  # 拼写错误:"secret" 而不是 "secrets"

# 正确:
{"pattern": "/secrets/**", "permission": "none"},
```

**缓解措施:** 在生产前测试权限规则。使用集成测试验证密钥被隐藏。

### 3. 过度宽泛的写权限

```python
# 危险:Agent 可以修改源代码
{"pattern": "**/*", "permission": "write"}

# 安全:只有特定输出目录可写
{"pattern": "/output/**", "permission": "write"}
{"pattern": "/tmp/**", "permission": "write"}
```

## 下一步

- **[权限系统](permissions.md)** - 深入权限规则和模式
- **[Preset 指南](presets.md)** - 使用和扩展权限预设
- **[最佳实践](best-practices.md)** - 生产部署安全模式
