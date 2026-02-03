# 权限预设

权限预设是针对常见使用场景的预配置规则集。无需手动定义权限规则,您可以从预设开始并根据需要自定义。本指南涵盖内置预设、如何扩展它们以及如何创建自定义预设。

## 内置预设

AgentFense 包含五个针对不同场景优化的内置预设:

### agent-safe (AI Agent 的默认预设)

**使用场景:** AI agent 探索代码库,具有受控输出和隐藏的密钥。

**理念:** 允许 agent 自由读取和探索代码库,写入指定的输出目录,但隐藏所有密钥和敏感数据。

**规则:**

| 模式 | 权限 | 优先级 | 原因 |
|------|------|--------|------|
| `**/*` | `read` | 0 | 默认:读取所有文件 |
| `/output/**` | `write` | 10 | 允许写入结果 |
| `/tmp/**` | `write` | 10 | 允许临时文件 |
| `**/.env*` | `none` | 100 | 隐藏环境文件 |
| `**/secrets/**` | `none` | 100 | 隐藏密钥目录 |
| `**/*.key` | `none` | 100 | 隐藏私钥 |
| `**/*.pem` | `none` | 100 | 隐藏 PEM 证书 |
| `**/credentials*` | `none` | 100 | 隐藏凭证文件 |
| `**/.git/**` | `none` | 50 | 隐藏 git 历史 |

**示例:**

```python
from agentfense import Sandbox

# 使用 agent-safe 预设(默认)
with Sandbox.from_local("./my-project") as sandbox:
    # Agent 可以读取源代码
    result = sandbox.run("cat /workspace/app.py")
    print(result.stdout)
    
    # Agent 可以写入 /output
    result = sandbox.run("echo 'results' > /workspace/output/report.txt")
    assert result.exit_code == 0
    
    # Agent 无法看到密钥
    result = sandbox.run("ls /workspace/.env")
    assert result.exit_code != 0  # 文件未找到
```

**何时使用:**
- 在代码库上运行 AI agent(Claude、GPT)
- 自动化代码分析工具
- 文档生成器
- 测试助手

### read-only (安全浏览)

**使用场景:** 探索代码库而无任何写入访问权限。

**理念:** 授予对所有文件的完全读取访问权限,拒绝所有写入。

**规则:**

| 模式 | 权限 | 优先级 |
|------|------|--------|
| `**/*` | `read` | 0 |

**示例:**

```python
from agentfense import Sandbox

# 只读模式
with Sandbox.from_local("./my-project", preset="read-only") as sandbox:
    # 所有读取都有效
    result = sandbox.run("cat /workspace/config.yaml")
    print(result.stdout)
    
    # 所有写入都失败
    result = sandbox.run("echo test > /workspace/output.txt")
    assert result.exit_code != 0  # 权限拒绝
```

**何时使用:**
- AI agent 进行代码审查
- 静态分析工具
- 文档搜索
- 安全探索未知代码库

### full-access (可信自动化)

**使用场景:** 具有完全读/写访问权限的可信自动化。

**理念:** 授予对所有文件的完全访问权限(谨慎使用)。

**规则:**

| 模式 | 权限 | 优先级 |
|------|------|--------|
| `**/*` | `write` | 0 |

**示例:**

```python
from agentfense import Sandbox

# 完全访问(谨慎使用!)
with Sandbox.from_local("./my-project", preset="full-access") as sandbox:
    # 可以读取和修改任何文件
    result = sandbox.run("sed -i 's/v1/v2/' /workspace/version.txt")
    assert result.exit_code == 0
```

**何时使用:**
- 可信自动化脚本(例如,构建系统)
- 在隔离环境中测试
- 开发/调试(永远不要在生产环境中使用不可信代码)

**⚠️ 警告:** 此预设授予完全写入访问权限。仅与可信代码或在完全隔离的环境中使用。

### development (开发者友好)

**使用场景:** 带安全防护的本地开发。

**理念:** 除密钥外的完全访问(防止意外暴露)。

**规则:**

| 模式 | 权限 | 优先级 |
|------|------|--------|
| `**/*` | `write` | 0 |
| `**/.env*` | `none` | 100 |
| `**/secrets/**` | `none` | 100 |
| `**/*.key` | `none` | 100 |
| `**/*.pem` | `none` | 100 |

**示例:**

```python
from agentfense import Sandbox

# 开发模式
with Sandbox.from_local("./my-project", preset="development") as sandbox:
    # 可以修改源代码
    result = sandbox.run("echo 'print(1)' >> /workspace/test.py")
    assert result.exit_code == 0
    
    # 但密钥被隐藏
    result = sandbox.run("cat /workspace/.env")
    assert result.exit_code != 0  # 文件未找到
```

**何时使用:**
- 使用 AI 助手进行本地开发
- 原型设计和实验
- 调试 agent 工作流

### view-only (元数据探索)

**使用场景:** 查看文件结构而不暴露内容。

**理念:** 允许 agent 查看文件名和元数据,但拒绝读取内容。

**规则:**

| 模式 | 权限 | 优先级 |
|------|------|--------|
| `**/*` | `view` | 0 |

**示例:**

```python
from agentfense import Sandbox

# 仅查看模式
with Sandbox.from_local("./my-project", preset="view-only") as sandbox:
    # 可以列出文件
    result = sandbox.run("ls -lh /workspace/")
    print(result.stdout)  # 显示文件名、大小、时间戳
    
    # 无法读取内容
    result = sandbox.run("cat /workspace/app.py")
    assert result.exit_code != 0  # 权限拒绝
```

**何时使用:**
- 文件结构探索
- 元数据索引
- 规划工具(agent 在请求访问之前看到可用内容)

**注意:** `view` 权限在 macOS 上使用 Docker Desktop 时有限制。如果遇到问题,请改用 `read`。

## 使用预设

### 方法 1:高层 API

```python
from agentfense import Sandbox

# 按名称使用预设
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    result = sandbox.run("python main.py")
```

### 方法 2:低层 API

```python
from agentfense import SandboxClient, get_preset_dicts

client = SandboxClient()
codebase = client.create_codebase(name="my-project", owner_id="user_001")

# 获取预设为字典供 API 使用
permissions = get_preset_dicts("agent-safe")

sandbox = client.create_sandbox(
    codebase_id=codebase.id,
    permissions=permissions,
)
```

### 列出可用预设

```python
from agentfense import list_presets

# 获取所有预设名称
presets = list_presets()
print(presets)  # ['agent-safe', 'development', 'full-access', 'read-only', 'view-only']
```

## 扩展预设

预设是起点。使用 `extend_preset()` 自定义它们:

### 添加规则

```python
from agentfense import extend_preset, Sandbox

# 从 agent-safe 开始,添加对 /logs 的写入访问
rules = extend_preset(
    base="agent-safe",
    additions=[
        {"pattern": "/logs/**", "permission": "write"},
    ],
)

with Sandbox.from_local("./project", permissions=rules) as sandbox:
    # 现在可以写入 /logs(除了 /output 和 /tmp)
    result = sandbox.run("echo 'log entry' > /workspace/logs/app.log")
    assert result.exit_code == 0
```

### 覆盖规则

```python
from agentfense import extend_preset

# 从 agent-safe 开始,使 .git 可读(覆盖隐藏)
rules = extend_preset(
    base="agent-safe",
    overrides=[
        {"pattern": "**/.git/**", "permission": "read"},  # 覆盖:取消隐藏 .git
    ],
)

# .git 现在可读(覆盖 agent-safe 的 "none")
```

**`overrides` 如何工作:**
- 自动分配比所有基础规则更高的优先级
- 确保您的覆盖优先
- 用于预设规则的例外

### 组合添加和覆盖

```python
from agentfense import extend_preset

rules = extend_preset(
    base="read-only",
    additions=[
        {"pattern": "/output/**", "permission": "write"},  # 添加对输出的写入
    ],
    overrides=[
        {"pattern": "**/.env*", "permission": "none"},  # 隐藏 .env 文件
    ],
)

# 结果:
# - 所有文件可读(基础:read-only)
# - /output/** 可写(添加)
# - .env 文件隐藏(覆盖)
```

## 创建自定义预设

注册您自己的预设以便跨项目重用:

### 注册预设

```python
from agentfense import register_preset

# 为 CI/CD 定义自定义预设
register_preset("ci-pipeline", [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/build/**", "permission": "write"},
    {"pattern": "/dist/**", "permission": "write"},
    {"pattern": "/.cache/**", "permission": "write"},
    {"pattern": "**/node_modules/**", "permission": "none"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
])

# 像内置预设一样使用它
from agentfense import Sandbox

with Sandbox.from_local("./app", preset="ci-pipeline") as sandbox:
    sandbox.run("npm run build")
```

### 自定义预设的最佳实践

1. **描述性命名**:使用解释使用场景的名称(`ci-pipeline`、`data-science`、`code-review`)
2. **记录预设**:添加解释规则的注释
3. **设置显式优先级**:对于安全关键规则(例如,隐藏密钥),使用高优先级
4. **彻底测试**:在生产使用之前验证预设行为符合预期

### 示例:数据科学预设

```python
from agentfense import register_preset

register_preset("data-science", [
    # 读取代码和配置
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.ipynb", "permission": "read"},
    {"pattern": "**/*.yaml", "permission": "read"},
    {"pattern": "**/*.json", "permission": "read"},
    
    # 查看数据文件(无内容访问)
    {"pattern": "/data/**", "permission": "view"},
    
    # 写入输出和缓存
    {"pattern": "/output/**", "permission": "write"},
    {"pattern": "/models/**", "permission": "write"},
    {"pattern": "/.cache/**", "permission": "write"},
    
    # 隐藏密钥
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/credentials/**", "permission": "none", "priority": 100},
])
```

### 示例:文档生成器预设

```python
from agentfense import register_preset

register_preset("doc-generator", [
    # 读取源文件
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    {"pattern": "**/*.ts", "permission": "read"},
    
    # 读取现有文档
    {"pattern": "**/*.md", "permission": "read"},
    {"pattern": "/docs/**", "permission": "read"},
    
    # 写入生成的文档
    {"pattern": "/docs/generated/**", "permission": "write"},
    {"pattern": "/docs/api/**", "permission": "write"},
    
    # 隐藏私有/内部代码
    {"pattern": "**/internal/**", "permission": "none"},
    {"pattern": "**/_private/**", "permission": "none"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
])
```

## 预设比较

| 预设 | 读代码 | 写代码 | 写输出 | 密钥可见 | 使用场景 |
|------|--------|--------|--------|----------|----------|
| **agent-safe** | ✅ | ❌ | ✅ (/output, /tmp) | ❌ 隐藏 | AI agent、自动化 |
| **read-only** | ✅ | ❌ | ❌ | ✅ 可见 | 代码审查、分析 |
| **full-access** | ✅ | ✅ | ✅ | ✅ 可见 | 可信自动化 |
| **development** | ✅ | ✅ | ✅ | ❌ 隐藏 | 本地开发 |
| **view-only** | 📋 元数据 | ❌ | ❌ | 📋 元数据 | 结构探索 |

## 在预设之间迁移

### 从 read-only 到 agent-safe

```python
# 之前:read-only
with Sandbox.from_local("./project", preset="read-only") as sandbox:
    # 无法在任何地方写入
    pass

# 之后:agent-safe
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    # 可以写入 /output 和 /tmp
    sandbox.run("echo result > /workspace/output/log.txt")
```

**为什么迁移:** 使 agent 能够保存结果,同时保持代码只读。

### 从 agent-safe 到 development

```python
# 之前:agent-safe(代码是只读的)
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    result = sandbox.run("echo test >> /workspace/app.py")
    assert result.exit_code != 0  # 失败

# 之后:development(代码可写,密钥隐藏)
with Sandbox.from_local("./project", preset="development") as sandbox:
    result = sandbox.run("echo test >> /workspace/app.py")
    assert result.exit_code == 0  # 成功
```

**为什么迁移:** 允许 agent 修改代码(例如,重构、测试),同时保持密钥隐藏。

### 从 full-access 到 agent-safe

```python
# 之前:full-access(危险)
with Sandbox.from_local("./project", preset="full-access") as sandbox:
    # 可以读取密钥和修改代码
    pass

# 之后:agent-safe(更安全)
with Sandbox.from_local("./project", preset="agent-safe") as sandbox:
    # 密钥隐藏,代码只读,输出可写
    pass
```

**为什么迁移:** 从可信到不可信自动化过渡时降低风险。

## 故障排查

### 预设未找到

**错误:** `ValueError: Unknown preset 'my-preset'`

**解决方案:** 检查预设名称拼写。使用 `list_presets()` 列出可用预设。

### 密钥仍然可见

**问题:** 像 `.env` 这样的文件出现在 `ls` 输出中

**诊断:**
1. 检查是否使用正确的预设:`agent-safe`、`development` 或带有密钥规则的自定义
2. 验证模式匹配:`**/.env*` 匹配 `.env`、`.env.local` 等
3. 检查优先级:密钥规则应比基础规则具有更高优先级

**解决方案:**

```python
from agentfense import extend_preset

# 确保密钥具有高优先级
rules = extend_preset(
    base="read-only",
    overrides=[
        {"pattern": "**/.env*", "permission": "none", "priority": 100},
    ],
)
```

### 无法写入输出目录

**问题:** 写入 `/output` 时出现 `Permission denied`

**诊断:** 检查您的预设是否允许写入 `/output`。`read-only` 和 `view-only` 不允许。

**解决方案:** 使用 `agent-safe`、`full-access` 或扩展您的预设:

```python
from agentfense import extend_preset

rules = extend_preset(
    base="read-only",
    additions=[
        {"pattern": "/output/**", "permission": "write"},
    ],
)
```

## 下一步

- **[权限系统](permissions.md)** - 深入权限规则
- **[最佳实践](best-practices.md)** - 生产部署模式
- **[安全模型](index.md)** - 整体安全架构
