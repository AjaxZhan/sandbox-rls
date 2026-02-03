# 权限管理指南

AgentFense 为沙盒文件操作提供**路径级访问控制**。本指南涵盖权限级别、模式、预设和保护沙盒的最佳实践。

---

## 目录

- [权限级别](#权限级别)
- [权限规则](#权限规则)
- [模式类型](#模式类型)
- [内置预设](#内置预设)
- [自定义权限](#自定义权限)
- [扩展预设](#扩展预设)
- [规则优先级](#规则优先级)
- [常见模式](#常见模式)
- [最佳实践](#最佳实践)

---

## 权限级别

AgentFense 支持**四个权限级别**,从最严格到最宽松:

| 级别 | 符号 | 描述 | `ls` | `cat` | 写入 |
|-------|--------|-------------|------|-------|-------|
| **NONE** | `none` | 完全不可见 | ❌ | ❌ | ❌ |
| **VIEW** | `view` | 在目录列表中可见 | ✓ | ❌ | ❌ |
| **READ** | `read` | 可以读取文件内容 | ✓ | ✓ | ❌ |
| **WRITE** | `write` | 完全读/写访问 | ✓ | ✓ | ✓ |

### NONE - 完全不可见

具有 `none` 权限的文件**不会出现在目录列表中**,并返回"文件不存在"错误。

```python
from agentfense import Sandbox, PermissionRule, Permission

with Sandbox.from_local(
    "./project",
    permissions=[
        PermissionRule(pattern="/secrets/**", permission=Permission.NONE),
    ]
) as sandbox:
    # ❌ 文件不可见
    result = sandbox.run("ls /workspace/secrets/")
    print(result.stderr)  # 没有那个文件或目录
    
    result = sandbox.run("cat /workspace/secrets/api.key")
    print(result.stderr)  # 没有那个文件或目录
```

**使用场景**:机密、凭据、私钥必须完全隐藏。

---

### VIEW - 仅列表

文件在 `ls` 中**可见**但无法读取。

```python
permissions=[
    PermissionRule(pattern="/config/**", permission=Permission.VIEW),
]

# ✓ 可以看到文件存在
result = sandbox.run("ls /workspace/config/")
print(result.stdout)  # database.yaml  api.yaml

# ❌ 无法读取内容
result = sandbox.run("cat /workspace/config/database.yaml")
print(result.stderr)  # 权限被拒绝
```

**使用场景**:你希望 agent 知道文件存在但不读取它们(例如,配置文件模板)。

---

### READ - 只读

文件可以**读取但不能修改**。

```python
permissions=[
    PermissionRule(pattern="**/*.py", permission=Permission.READ),
]

# ✓ 可以读取
result = sandbox.run("cat /workspace/main.py")
print(result.stdout)  # (文件内容)

# ❌ 无法写入
result = sandbox.run("echo 'new line' >> /workspace/main.py")
print(result.stderr)  # 权限被拒绝
```

**使用场景**:静态分析、代码审查、无修改风险的检查。

---

### WRITE - 完全访问

文件可以**创建、读取、修改和删除**。

```python
permissions=[
    PermissionRule(pattern="/output/**", permission=Permission.WRITE),
]

# ✓ 可以创建
sandbox.run("echo 'result' > /workspace/output/result.txt")

# ✓ 可以读取
result = sandbox.run("cat /workspace/output/result.txt")

# ✓ 可以修改
sandbox.run("echo 'more' >> /workspace/output/result.txt")

# ✓ 可以删除
sandbox.run("rm /workspace/output/result.txt")
```

**使用场景**:输出目录、临时文件、构建产物。

---

## 权限规则

**PermissionRule** 定义文件模式的访问控制。

### PermissionRule 类

```python
from agentfense import PermissionRule, Permission, PatternType

rule = PermissionRule(
    pattern="**/*.py",
    permission=Permission.READ,
    type=PatternType.GLOB,
    priority=0,
)
```

#### 字段

| 字段 | 类型 | 默认值 | 描述 |
|-------|------|---------|-------------|
| `pattern` | `str` | **必需** | 要匹配的文件模式 |
| `permission` | `Permission` | `Permission.READ` | 访问级别 |
| `type` | `PatternType` | `PatternType.GLOB` | 模式匹配类型 |
| `priority` | `int` | `0` | 规则优先级(越高越优先) |

---

### 字典格式

你也可以将规则指定为字典:

```python
permissions=[
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
]
```

---

## 模式类型

AgentFense 支持三种模式匹配类型:

### GLOB - 通配符模式(默认)

使用 `**` 和 `*` 通配符进行灵活匹配。

```python
PatternType.GLOB  # 或省略(默认)

# 示例:
"**/*.py"           # 任何位置的所有 .py 文件
"**/.env*"          # 任何位置以 .env 开头的所有文件
"/src/**"           # /src 下的所有文件
"*.txt"             # 仅根目录中的所有 .txt 文件
"**/*test*.js"      # 名称中包含 "test" 的所有文件
```

**语法**:
- `**` - 匹配任意数量的目录
- `*` - 匹配单个路径组件中的任意字符
- `?` - 匹配单个字符

---

### DIRECTORY - 前缀匹配

匹配目录下的所有文件(包括子目录)。

```python
PermissionRule(
    pattern="/docs/",
    permission=Permission.WRITE,
    type=PatternType.DIRECTORY,
)

# 匹配:
# /workspace/docs/README.md
# /workspace/docs/guides/setup.md
# /workspace/docs/api/reference.md
```

**重要**:目录模式应以 `/` 结尾。

---

### FILE - 精确匹配

仅匹配特定文件路径。

```python
PermissionRule(
    pattern="/config.yaml",
    permission=Permission.READ,
    type=PatternType.FILE,
)

# 匹配:
# /workspace/config.yaml

# 不匹配:
# /workspace/config.yaml.bak
# /workspace/src/config.yaml
```

---

## 内置预设

AgentFense 包含**五个内置预设**用于常见场景:

### 1. `view-only`(默认)

**用例**:审计、检查、文件枚举而不读取。

```python
from agentfense import PRESET_VIEW_ONLY, get_preset

# 规则:
[
    {"pattern": "**/*", "permission": "view"},
]
```

**功能**:
- ✓ `ls` 在任何地方都可用
- ❌ 无法读取任何文件
- ❌ 无法在任何地方写入

---

### 2. `read-only`

**用例**:静态分析、代码审查、linting。

```python
from agentfense import PRESET_READ_ONLY

# 规则:
[
    {"pattern": "**/*", "permission": "read"},
]
```

**功能**:
- ✓ 可以读取所有文件
- ❌ 无法在任何地方写入

---

### 3. `agent-safe`(AI Agent 推荐)

**用例**:使用安全默认值运行 AI agent。

```python
from agentfense import PRESET_AGENT_SAFE

# 规则:
[
    {"pattern": "**/*", "permission": "read", "priority": 0},
    {"pattern": "/output/**", "permission": "write", "priority": 10},
    {"pattern": "/tmp/**", "permission": "write", "priority": 10},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "**/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
    {"pattern": "**/credentials*", "permission": "none", "priority": 100},
    {"pattern": "**/.git/**", "permission": "none", "priority": 50},
]
```

**功能**:
- ✓ 读取所有文件(机密除外)
- ✓ 写入 `/output` 和 `/tmp`
- ❌ 无法访问 `.env*`、`secrets/`、`*.key`、`*.pem`、`credentials*`、`.git/`

**推荐用于**:AI agent、不可信自动化、代码分析工具。

---

### 4. `development`

**用例**:具有机密保护的开发自动化。

```python
from agentfense import PRESET_DEVELOPMENT

# 规则:
[
    {"pattern": "**/*", "permission": "write", "priority": 0},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "**/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
]
```

**功能**:
- ✓ 完全写入访问(机密除外)
- ❌ 无法访问 `.env*`、`secrets/`、`*.key`、`*.pem`

**推荐用于**:构建脚本、测试自动化、CI/CD。

---

### 5. `full-access`

**用例**:无限制的可信代码。

```python
from agentfense import PRESET_FULL_ACCESS

# 规则:
[
    {"pattern": "**/*", "permission": "write", "priority": 0},
]
```

**功能**:
- ✓ 在任何地方完全读/写访问

**警告**:无机密泄漏保护。仅用于完全可信的代码。

---

## 自定义权限

### 创建自定义规则

```python
from agentfense import Sandbox, PermissionRule, Permission

permissions = [
    # 默认:读取所有
    PermissionRule(pattern="**/*", permission=Permission.READ),
    
    # 允许写入特定目录
    PermissionRule(pattern="/logs/**", permission=Permission.WRITE),
    PermissionRule(pattern="/cache/**", permission=Permission.WRITE),
    
    # 隐藏机密
    PermissionRule(pattern="/secrets/**", permission=Permission.NONE),
    
    # 配置模板仅查看
    PermissionRule(pattern="/templates/**", permission=Permission.VIEW),
]

with Sandbox.from_local("./project", permissions=permissions) as sandbox:
    sandbox.run("python main.py")
```

---

### 使用字典格式

```python
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/output/**", "permission": "write"},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
]

with Sandbox.from_local("./project", permissions=permissions) as sandbox:
    ...
```

---

## 扩展预设

### 向预设添加规则

使用 `extend_preset()` 向内置预设添加自定义规则:

```python
from agentfense import Sandbox, extend_preset

permissions = extend_preset(
    "agent-safe",  # 基础预设
    additions=[
        {"pattern": "/workspace/data/**", "permission": "write"},
        {"pattern": "/workspace/logs/**", "permission": "write"},
    ]
)

with Sandbox.from_local("./project", permissions=permissions) as sandbox:
    ...
```

**结果**:所有 `agent-safe` 规则 + 你的自定义规则。

---

### 覆盖预设规则

使用具有更高优先级的 `overrides` 覆盖预设行为:

```python
permissions = extend_preset(
    "agent-safe",
    overrides=[
        # 允许读取 .env(覆盖 agent-safe 的屏蔽)
        {"pattern": "**/.env*", "permission": "read"},
    ]
)
```

**重要**:覆盖会自动获得比基础规则更高的优先级。

---

## 规则优先级

当多个规则匹配一个文件时,**优先级最高**的规则获胜。

### 自动优先级

AgentFense 根据模式特异性自动分配优先级:

| 模式类型 | 自动优先级 | 示例 |
|--------------|---------------|---------|
| FILE | 3 | `/config.yaml` |
| DIRECTORY | 2 | `/docs/` |
| GLOB | 1 | `**/*.py` |

更具体的模式(精确路径)胜过通用模式(通配符)。

---

### 手动优先级

你可以手动设置优先级以覆盖自动排序:

```python
permissions = [
    # 默认:读取所有(优先级 0)
    PermissionRule(pattern="**/*", permission=Permission.READ, priority=0),
    
    # 屏蔽机密(优先级 100 - 胜过默认)
    PermissionRule(pattern="/secrets/**", permission=Permission.NONE, priority=100),
    
    # 例外:允许读取这个特定机密(优先级 200 - 胜过屏蔽)
    PermissionRule(pattern="/secrets/public.key", permission=Permission.READ, priority=200),
]
```

**规则**:优先级值越高 = 优先级越高。

---

## 常见模式

### 模式 1:只读带输出目录

```python
permissions = [
    PermissionRule(pattern="**/*", permission=Permission.READ),
    PermissionRule(pattern="/output/**", permission=Permission.WRITE),
]
```

**用例**:生成报告的静态分析。

---

### 模式 2:隐藏所有机密

```python
secret_patterns = [
    "**/.env*",
    "**/secrets/**",
    "**/*.key",
    "**/*.pem",
    "**/*credentials*",
    "**/*password*",
    "**/token*",
]

permissions = [
    PermissionRule(pattern="**/*", permission=Permission.READ),
] + [
    PermissionRule(pattern=p, permission=Permission.NONE, priority=100)
    for p in secret_patterns
]
```

---

### 模式 3:选择性写入访问

```python
permissions = [
    # 默认:只读
    PermissionRule(pattern="**/*", permission=Permission.READ),
    
    # 允许写入特定子目录
    PermissionRule(pattern="/docs/**", permission=Permission.WRITE),
    PermissionRule(pattern="/tests/**", permission=Permission.WRITE),
    PermissionRule(pattern="/output/**", permission=Permission.WRITE),
]
```

---

### 模式 4:特定语言访问

```python
permissions = [
    # Python 文件:只读
    PermissionRule(pattern="**/*.py", permission=Permission.READ),
    
    # JavaScript 文件:只读
    PermissionRule(pattern="**/*.js", permission=Permission.READ),
    
    # 配置文件:仅查看(知道它们存在,无法读取)
    PermissionRule(pattern="**/*.yaml", permission=Permission.VIEW),
    PermissionRule(pattern="**/*.json", permission=Permission.VIEW),
    
    # 其他所有:无(不可见)
    PermissionRule(pattern="**/*", permission=Permission.NONE, priority=-1),
]
```

---

### 模式 5:测试环境

```python
permissions = [
    # 读取源代码
    PermissionRule(pattern="/src/**", permission=Permission.READ),
    
    # 对测试完全访问
    PermissionRule(pattern="/tests/**", permission=Permission.WRITE),
    
    # 对测试输出完全访问
    PermissionRule(pattern="/coverage/**", permission=Permission.WRITE),
    PermissionRule(pattern="/.pytest_cache/**", permission=Permission.WRITE),
    
    # 隐藏生产机密
    PermissionRule(pattern="/secrets/**", permission=Permission.NONE, priority=100),
]
```

---

## 最佳实践

### 1. 使用预设作为基础

```python
# ✓ 好:从预设开始,根据需要扩展
permissions = extend_preset(
    "agent-safe",
    additions=[{"pattern": "/data/**", "permission": "write"}]
)

# ✗ 避免:从头开始编写所有规则(容易出错)
permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/output/**", "permission": "write"},
    # ...忘记屏蔽 .env 文件!
]
```

---

### 2. 使用高优先级屏蔽机密

```python
permissions = [
    PermissionRule(pattern="**/*", permission=Permission.READ),
    
    # 高优先级确保机密始终被屏蔽
    PermissionRule(pattern="**/.env*", permission=Permission.NONE, priority=1000),
    PermissionRule(pattern="**/secrets/**", permission=Permission.NONE, priority=1000),
]
```

---

### 3. 生产前测试权限

```python
# 测试:agent 能读取它应该读取的内容吗?
with Sandbox.from_local("./test-project", permissions=permissions) as sandbox:
    result = sandbox.run("cat /workspace/main.py")
    assert result.exit_code == 0, "应该读取 main.py"
    
    result = sandbox.run("cat /workspace/.env")
    assert result.exit_code != 0, "不应该读取 .env"
```

---

### 4. 记录自定义规则

```python
permissions = [
    # 允许读取进行分析
    PermissionRule(pattern="**/*", permission=Permission.READ),
    
    # 写入访问所需用于:
    # - 生成的文档(docs/)
    # - 测试报告(reports/)
    # - 构建产物(dist/)
    PermissionRule(pattern="/docs/**", permission=Permission.WRITE),
    PermissionRule(pattern="/reports/**", permission=Permission.WRITE),
    PermissionRule(pattern="/dist/**", permission=Permission.WRITE),
]
```

---

### 5. 使用环境特定预设

```python
def get_permissions(environment: str):
    if environment == "production":
        return get_preset("read-only")  # 最大安全性
    elif environment == "staging":
        return get_preset("agent-safe")  # 平衡
    elif environment == "development":
        return get_preset("development")  # 更多自由
    else:
        return get_preset("view-only")  # 默认安全
```

---

## 注册自定义预设

为你的组织创建可重用的预设:

```python
from agentfense import register_preset

register_preset("data-science", [
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.ipynb", "permission": "read"},
    {"pattern": "**/*.csv", "permission": "read"},
    {"pattern": "**/*.parquet", "permission": "read"},
    {"pattern": "/output/**", "permission": "write"},
    {"pattern": "/models/**", "permission": "write"},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
])

# 像内置预设一样使用
with Sandbox.from_local("./ml-project", preset="data-science") as sandbox:
    sandbox.run("python train.py")
```

---

## 另请参阅

- [高层 API 指南](./high-level.md) - 在 Sandbox 中使用权限
- [错误处理指南](./errors.md) - 权限相关异常
- [最佳实践](../best-practices.md) - 安全指南
