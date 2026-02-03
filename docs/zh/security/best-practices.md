# 安全最佳实践

本指南提供在真实环境中部署 AgentFense 的生产就绪安全模式。遵循这些建议以最大化安全性、可靠性和可维护性。

## 权限设计原则

### 1. 最小权限原则

**始终从最小权限开始,仅授予必要的权限。**

```python
# ✅ 好:从限制性开始,添加所需内容
permissions = [
    {"pattern": "**/*", "permission": "none", "priority": 0},      # 默认:拒绝所有
    {"pattern": "/src/**", "permission": "read", "priority": 10},  # 显式允许源代码
    {"pattern": "/output/**", "permission": "write", "priority": 10},  # 显式允许输出
]

# ❌ 坏:从宽松开始,尝试限制
permissions = [
    {"pattern": "**/*", "permission": "write"},  # 过于宽松
    {"pattern": "/secrets/**", "permission": "none"},  # 容易遗漏路径
]
```

**原因:** 过度宽松的默认值会造成安全漏洞。攻击者利用您*忘记*限制的内容,而不是您*记得*允许的内容。

### 2. 深度防御

**分层多个安全控制,不要依赖单一机制。**

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./project",
    preset="agent-safe",  # 层 1:权限控制
    runtime=RuntimeType.DOCKER,  # 层 2:容器隔离
    resources=ResourceLimits(  # 层 3:资源限制
        memory_bytes=512 * 1024 * 1024,
        pids_limit=100,
    ),
    # 层 4:网络禁用(Docker 中默认)
) as sandbox:
    result = sandbox.run("untrusted_command")
```

**层级:**
1. **文件系统权限**:隐藏密钥,限制写入
2. **进程隔离**:防止逃逸到主机
3. **资源限制**:防止 DoS 攻击
4. **网络隔离**:防止数据泄露

### 3. 失败时安全

**有疑问时,拒绝访问。永远不要失败开放。**

```python
# ✅ 好:默认为最严格权限
permissions = [
    {"pattern": "**/*", "permission": "none"},  # 默认:拒绝
    # ... 显式允许 ...
]

# ❌ 坏:默认为宽松
permissions = [
    {"pattern": "**/*", "permission": "write"},  # 危险的默认值
]
```

### 4. 显式优于隐式

**对安全关键规则要明确,不要依赖模式匹配的微妙之处。**

```python
# ✅ 好:高优先级的显式规则
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},  # 显式
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},  # 显式
    {"pattern": "**/*.key", "permission": "none", "priority": 100},  # 显式
]

# ⚠️ 风险:依赖模式匹配顺序
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/secrets/**", "permission": "none"},  # 没有显式优先级
]
```

## 密钥管理

### 隐藏所有密钥模式

使用全面的模式捕获所有密钥变体:

```python
SECRET_PATTERNS = [
    # 环境文件
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "**/env.*", "permission": "none", "priority": 100},
    
    # 凭证
    {"pattern": "**/credentials*", "permission": "none", "priority": 100},
    {"pattern": "**/secrets/**", "permission": "none", "priority": 100},
    
    # 密钥和证书
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
    {"pattern": "**/*.p12", "permission": "none", "priority": 100},
    {"pattern": "**/*.pfx", "permission": "none", "priority": 100},
    
    # SSH 密钥
    {"pattern": "**/id_rsa*", "permission": "none", "priority": 100},
    {"pattern": "**/id_ed25519*", "permission": "none", "priority": 100},
    {"pattern": "**/id_ecdsa*", "permission": "none", "priority": 100},
    
    # 云提供商凭证
    {"pattern": "**/.aws/**", "permission": "none", "priority": 100},
    {"pattern": "**/.gcp/**", "permission": "none", "priority": 100},
    {"pattern": "**/.azure/**", "permission": "none", "priority": 100},
    
    # 令牌文件
    {"pattern": "**/*token*", "permission": "none", "priority": 100},
    {"pattern": "**/*secret*", "permission": "none", "priority": 100},
    
    # Git 历史(可能包含泄露的密钥)
    {"pattern": "**/.git/**", "permission": "none", "priority": 50},
]
```

### 使用 .sandboxignore 进行上传

创建 `.sandboxignore` 文件以防止敏感文件被上传:

```
# .sandboxignore (使用 .gitignore 语法)

# 密钥
.env*
secrets/
credentials*
*.key
*.pem

# 云凭证
.aws/
.gcp/
.azure/

# Git 历史
.git/

# 构建工件
node_modules/
__pycache__/
dist/
build/
```

### 环境变量注入

不要通过沙盒中的环境变量暴露密钥:

```python
# ❌ 坏:环境变量中的密钥(agent 可访问)
result = sandbox.run(
    "python app.py",
    env={"DATABASE_PASSWORD": "secret123"}  # Agent 可以读取这个!
)

# ✅ 好:将密钥保留在服务器端,使用安全凭证注入
# 选项 1:使用密钥管理服务(Vault、AWS Secrets Manager)
# 选项 2:以严格权限只读挂载密钥
# 选项 3:使用应用层身份验证(服务器端验证的 API 令牌)
```

## 上传安全

### 验证上传的文件

```python
from pathlib import Path
from agentfense import Sandbox

def safe_upload(project_path: str) -> Sandbox:
    """带安全检查的上传项目。"""
    
    # 上传前检查敏感文件
    sensitive_files = []
    for pattern in [".env*", "*.key", "*.pem", "credentials*"]:
        sensitive_files.extend(Path(project_path).rglob(pattern))
    
    if sensitive_files:
        raise ValueError(f"发现敏感文件: {sensitive_files}。上传前请删除。")
    
    # 以严格权限上传
    return Sandbox.from_local(
        project_path,
        preset="agent-safe",
        ignore_patterns=[
            ".env*",
            "secrets/",
            "*.key",
            "*.pem",
            ".git/",
        ],
    )
```

### 扫描硬编码密钥

上传前使用 `detect-secrets` 或 `truffleHog` 等工具:

```bash
# 安装 detect-secrets
pip install detect-secrets

# 扫描代码库
detect-secrets scan --all-files ./my-project

# 如果发现密钥则阻止上传
if detect-secrets scan --all-files ./my-project; then
    echo "✅ 未检测到密钥"
else
    echo "❌ 检测到密钥! 上传前修复。"
    exit 1
fi
```

## 运行时配置

### 生产服务器设置

```yaml
# configs/production.yaml

server:
  grpc_addr: ":9000"
  http_addr: ":8080"

storage:
  codebase_path: "/data/agentfense/codebases"  # 隔离存储
  mount_path: "/data/agentfense/mounts"
  db_path: "/data/agentfense/metadata.db"

runtime:
  type: "docker"  # 使用 Docker 实现更强隔离
  default_timeout: "60s"
  max_timeout: "30m"  # 防止无限挂起
  
  docker:
    host: ""
    default_image: "ubuntu:22.04"
    network_mode: "none"  # 默认禁用网络
    enable_networking: false  # 强制无网络

sandbox:
  default_expiration: "6h"  # 自动清理过期沙盒
  max_per_user: 20  # 防止资源耗尽
  cleanup_interval: "5m"

logging:
  level: "info"
  format: "json"  # 用于 SIEM 集成的结构化日志
```

### 资源限制

设置激进的资源限制以防止滥用:

```python
from agentfense import Sandbox, ResourceLimits

# 不可信工作负载的保守限制
UNTRUSTED_LIMITS = ResourceLimits(
    memory_bytes=256 * 1024 * 1024,  # 256 MB
    cpu_quota=25000,                 # 25% CPU
    pids_limit=50,                   # 最多 50 个进程
)

# AI agent 的标准限制
AGENT_LIMITS = ResourceLimits(
    memory_bytes=512 * 1024 * 1024,  # 512 MB
    cpu_quota=50000,                 # 50% CPU
    pids_limit=100,                  # 最多 100 个进程
)

# 构建任务的更高限制
BUILD_LIMITS = ResourceLimits(
    memory_bytes=2 * 1024 * 1024 * 1024,  # 2 GB
    cpu_quota=100000,                      # 100% CPU
    pids_limit=200,
)
```

### 网络隔离

**默认:** 除非明确要求,否则禁用网络。

```yaml
# 服务器配置:禁用网络
runtime:
  docker:
    network_mode: "none"
    enable_networking: false
```

**需要网络时:**

```python
from agentfense import Sandbox, RuntimeType

# 使用带出口过滤的桥接网络
with Sandbox.from_local(
    "./project",
    runtime=RuntimeType.DOCKER,
    # 自定义 Docker 配置(需要服务器端设置)
    labels={"network_policy": "egress_only"},  # 网络策略的自定义标签
) as sandbox:
    # 网络访问由 Docker 网络策略控制
    result = sandbox.run("curl https://api.example.com")
```

**注意:** 网络策略需要服务器端 Docker 配置(网络插件、防火墙规则)。

## 监控和审计

### 启用全面日志记录

```yaml
# 服务器配置
logging:
  level: "info"
  format: "json"
```

**记录所有安全事件:**
- 沙盒创建(谁、何时、权限)
- 命令执行(命令、退出代码、持续时间)
- 权限拒绝(路径、操作、权限级别)
- 资源限制违规(内存、CPU、PID)

### 告警规则

监控可疑模式并发出告警:

```python
# 示例:对高权限拒绝率发出告警(伪代码)
def check_permission_denials(logs):
    """如果权限拒绝过多则告警。"""
    denials = [log for log in logs if log.get("event") == "permission_denied"]
    
    if len(denials) > 10:  # 监控窗口内 >10 次拒绝
        alert(
            severity="medium",
            message=f"高权限拒绝率: {len(denials)} 次拒绝",
            sandbox_id=denials[0].get("sandbox_id"),
        )
```

**推荐告警:**

| 事件 | 阈值 | 严重性 |
|------|------|--------|
| 权限拒绝 | 每个沙盒 >10/分钟 | 中等 |
| 资源限制超出 | 任何发生 | 高 |
| 沙盒创建率 | 每用户 >100/小时 | 中等 |
| 长时间运行会话 | >1 小时 | 低 |
| 沙盒启动失败 | >3 次连续 | 高 |

### 审计跟踪

维护不可变的审计日志以符合合规性:

```json
{
  "timestamp": "2026-02-03T10:15:30Z",
  "event": "sandbox_created",
  "user_id": "user_001",
  "sandbox_id": "sb_abc123",
  "codebase_id": "cb_xyz789",
  "permissions": [...],
  "runtime": "docker",
  "image": "python:3.11-slim",
  "resources": {"memory_bytes": 536870912, "pids_limit": 100}
}

{
  "timestamp": "2026-02-03T10:15:35Z",
  "event": "command_executed",
  "sandbox_id": "sb_abc123",
  "command": "python main.py",
  "exit_code": 0,
  "duration_ms": 1250
}

{
  "timestamp": "2026-02-03T10:15:40Z",
  "event": "permission_denied",
  "sandbox_id": "sb_abc123",
  "path": "/workspace/.env",
  "operation": "open",
  "permission": "none",
  "user_id": "user_001"
}
```

**保留期:** 至少保留审计日志 90 天(取决于合规性)。

## 测试权限规则

### 集成测试

生产前测试权限规则:

```python
import pytest
from agentfense import Sandbox

def test_secrets_hidden():
    """验证密钥不可访问。"""
    permissions = [
        {"pattern": "**/*", "permission": "read"},
        {"pattern": "**/.env*", "permission": "none", "priority": 100},
    ]
    
    with Sandbox.from_local("./test-project", permissions=permissions) as sandbox:
        # 尝试读取 .env
        result = sandbox.run("cat /workspace/.env")
        assert result.exit_code != 0, "密钥文件应该不可访问"
        assert "No such file" in result.stderr, "文件应该不可见"
        
        # 尝试列出 .env
        result = sandbox.run("ls -la /workspace/")
        assert ".env" not in result.stdout, "密钥不应出现在列表中"

def test_output_writable():
    """验证输出目录可写。"""
    permissions = [
        {"pattern": "**/*", "permission": "read"},
        {"pattern": "/output/**", "permission": "write"},
    ]
    
    with Sandbox.from_local("./test-project", permissions=permissions) as sandbox:
        # 写入 /output
        result = sandbox.run("echo test > /workspace/output/result.txt")
        assert result.exit_code == 0, "应该能够写入 /output"
        
        # 验证文件存在
        result = sandbox.run("cat /workspace/output/result.txt")
        assert result.stdout.strip() == "test"

def test_code_read_only():
    """验证源代码只读。"""
    permissions = [
        {"pattern": "**/*.py", "permission": "read"},
    ]
    
    with Sandbox.from_local("./test-project", permissions=permissions) as sandbox:
        # 尝试修改代码
        result = sandbox.run("echo malicious >> /workspace/app.py")
        assert result.exit_code != 0, "不应该能够修改代码"
```

### 部署前检查清单

部署到生产前:

- [ ] 所有密钥已隐藏(用 `cat`、`ls`、`find`、`grep` 测试)
- [ ] 输出目录可写
- [ ] 源代码只读(用于 agent-safe 预设)
- [ ] 资源限制已强制执行(用内存/CPU 密集型任务测试)
- [ ] 网络已禁用(用 `curl`、`wget`、`ping` 测试)
- [ ] 沙盒自动过期(检查清理间隔)
- [ ] 审计日志已启用并转发到 SIEM
- [ ] 为可疑活动配置了告警

## 事件响应

### 检测妥协

**潜在妥协的迹象:**
1. 高权限拒绝率(agent 探测密钥)
2. 异常命令(权限提升尝试)
3. 资源耗尽(DoS 攻击)
4. 长时间运行会话(持久性尝试)

### 响应工作流

1. **隔离:** 立即销毁沙盒
2. **调查:** 审查沙盒的审计日志
3. **遏制:** 如果恶意则阻止用户
4. **修复:** 修复暴露的密钥,修补漏洞
5. **学习:** 更新权限规则以防止再次发生

```python
# 示例:紧急沙盒终止
from agentfense import SandboxClient

client = SandboxClient()

# 销毁被入侵的沙盒
client.destroy_sandbox("sb_suspicious_123")

# 阻止用户(应用层,不是 SDK)
# block_user(user_id="user_malicious")
```

## 多租户

### 隔离用户

每个用户/租户使用单独的 codebase:

```python
from agentfense import SandboxClient

client = SandboxClient()

# 用户 A 的 codebase
codebase_a = client.create_codebase(name="project-a", owner_id="user_a")

# 用户 B 的 codebase(与 A 隔离)
codebase_b = client.create_codebase(name="project-b", owner_id="user_b")

# 沙盒按 codebase 隔离
sandbox_a = client.create_sandbox(codebase_id=codebase_a.id, permissions=[...])
sandbox_b = client.create_sandbox(codebase_id=codebase_b.id, permissions=[...])
```

### 配额执行

防止每个租户的资源耗尽:

```yaml
sandbox:
  max_per_user: 20  # 每个用户的最大并发沙盒数
```

### 单独环境

对于高安全场景,每个租户使用专用服务器实例:

```
租户 A → AgentFense 服务器 A(隔离基础设施)
租户 B → AgentFense 服务器 B(隔离基础设施)
```

## 安全开发工作流

### 1. 定义安全要求

在实现功能之前:
- 哪些数据需要保护?
- agent 必须执行哪些操作?
- 可接受的风险水平是什么?

### 2. 设计权限

根据需求创建权限规则:

```python
# 示例:代码审查 agent
CODE_REVIEW_PERMISSIONS = [
    # 读取源代码
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    
    # 查看文档结构
    {"pattern": "/docs/**", "permission": "view"},
    
    # 写入审查评论
    {"pattern": "/review/**", "permission": "write"},
    
    # 隐藏密钥
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
]
```

### 3. 测试权限

编写集成测试(见[测试权限规则](#测试权限规则))。

### 4. 带监控部署

生产前启用日志记录和告警。

### 5. 基于事件迭代

在每次安全事件或险情后更新权限规则。

## 总结

| 实践 | 为什么重要 | 如何实现 |
|------|-----------|----------|
| **最小权限** | 最小化攻击面 | 从 `none` 开始,显式允许 |
| **深度防御** | 无单点故障 | 权限 + 隔离 + 资源限制 |
| **隐藏密钥** | 防止凭证盗窃 | 使用全面的密钥模式 |
| **资源限制** | 防止 DoS 攻击 | 设置内存、CPU、PID 限制 |
| **网络隔离** | 防止数据泄露 | 默认禁用网络 |
| **审计日志** | 检测和调查事件 | 记录所有沙盒操作(JSON 格式) |
| **测试** | 捕获错误配置 | 为权限规则编写集成测试 |
| **监控** | 早期检测攻击 | 对权限拒绝、资源违规发出告警 |

## 下一步

- **[权限系统](permissions.md)** - 深入权限规则
- **[Preset 指南](presets.md)** - 预配置权限模板
- **[安全模型](index.md)** - 整体安全架构
