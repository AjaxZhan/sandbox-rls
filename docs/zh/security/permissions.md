# 权限系统

本指南深入探讨 AgentFense 的权限系统:四个权限级别、模式匹配规则、优先级算法,以及构建安全访问策略的实践示例。

## 权限级别

### none - 完全不可见

**语义:** 从 agent 的角度来看,文件或目录不存在。

**Agent 的体验:**

| 操作 | 结果 |
|------|------|
| `ls /workspace/` | 文件未列出 |
| `cat /workspace/secret.txt` | `No such file or directory` |
| `stat /workspace/secret.txt` | `No such file or directory` |
| `[ -f /workspace/secret.txt ]` | 返回 false(文件测试失败) |
| `find /workspace -name secret.txt` | 无结果 |
| `grep pattern /workspace/**` | 文件被跳过(未找到) |

**实现:** FUSE 层对具有 `none` 权限的路径的所有操作返回 `ENOENT`(错误代码 2)。

**使用场景:**
- **密钥和凭证**: `.env`、`.env.*`、`credentials.json`、`config/secrets.yaml`
- **私钥**: `*.key`、`*.pem`、`*.p12`、`id_rsa`、`id_ed25519`
- **内部工具**: `/scripts/admin/**`、`/tools/internal/**`
- **敏感数据**: `/customer_data/**`、`/pii/**`、`/financial/**`

**示例:**

```python
permissions = [
    {"pattern": "**/.env*", "permission": "none"},
    {"pattern": "**/secrets/**", "permission": "none"},
    {"pattern": "**/*.key", "permission": "none"},
    {"pattern": "**/*.pem", "permission": "none"},
    {"pattern": "**/credentials*", "permission": "none"},
]
```

### view - 仅元数据

**语义:** 文件出现在列表中,但内容不可访问。

**Agent 的体验:**

| 操作 | 结果 |
|------|------|
| `ls -l /workspace/` | 文件列出,包含大小、权限、时间戳 |
| `cat /workspace/data.json` | `Permission denied` |
| `stat /workspace/data.json` | 成功(返回元数据) |
| `file /workspace/data.json` | 显示文件类型(例如,"JSON data") |
| `wc -l /workspace/data.json` | `Permission denied` |

**实现:** FUSE 允许 `stat()` 和目录列表,但拒绝用于读取的 `open()`。

**使用场景:**
- **大型数据集**:显示文件目录而不暴露内容
- **元数据探索**:让 agent 看到结构(文件名、大小)以便规划
- **文档索引**:列出文档而不显示内容

**示例:**

```python
permissions = [
    {"pattern": "/data/catalog/**", "permission": "view"},
    {"pattern": "/metadata/**", "permission": "view"},
]

# Agent 可以探索结构
$ ls -lh /workspace/data/catalog/
# 输出:users.csv (1.2MB), transactions.csv (5.4MB), products.csv (340KB)

# 但无法读取内容
$ head /workspace/data/catalog/users.csv
# 输出:head: cannot open '/workspace/data/catalog/users.csv' for reading: Permission denied
```

**已知限制:**
- **macOS + Docker Desktop**: VirtioFS 不能正确处理仅元数据访问。即使具有 `view` 权限,文件也显示为"No such file"。**解决方法:** 使用 `read` 权限或在 Linux 上使用 bwrap 运行时。

### read - 只读访问

**语义:** 文件完全可读,但无法修改。

**Agent 的体验:**

| 操作 | 结果 |
|------|------|
| `cat /workspace/app.py` | ✅ 成功(显示内容) |
| `grep pattern /workspace/**` | ✅ 搜索文件内容 |
| `diff /workspace/a.txt /workspace/b.txt` | ✅ 工作 |
| `echo "new" > /workspace/app.py` | ❌ Permission denied |
| `sed -i 's/old/new/' /workspace/app.py` | ❌ Permission denied |
| `rm /workspace/app.py` | ❌ Permission denied |

**实现:** FUSE 允许 `open(O_RDONLY)`,但拒绝 `open(O_WRONLY)`、`open(O_RDWR)`、`unlink()` 和 `rename()`。

**使用场景:**
- **源代码**:应用代码、库、模块
- **配置文件**:读取配置而不允许更改
- **文档**: README、指南、API 文档
- **静态资源**:图像、CSS、JavaScript(用于 Web 应用)

**示例:**

```python
permissions = [
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    {"pattern": "/docs/**", "permission": "read"},
    {"pattern": "/config/**", "permission": "read"},
]

# Agent 可以读取源代码
$ cat /workspace/app.py
# 输出:(文件内容)

# 但无法修改
$ echo "malicious_code()" >> /workspace/app.py
# 输出:bash: /workspace/app.py: Permission denied
```

### write - 完全访问

**语义:** 完全读、写、修改和删除权限。

**Agent 的体验:**

| 操作 | 结果 |
|------|------|
| `cat /workspace/output/log.txt` | ✅ 成功 |
| `echo "data" > /workspace/output/log.txt` | ✅ 成功 |
| `echo "more" >> /workspace/output/log.txt` | ✅ 成功(追加) |
| `rm /workspace/output/log.txt` | ✅ 成功 |
| `mkdir /workspace/output/subdir` | ✅ 成功 |

**实现:** FUSE 允许所有文件操作:读取、写入、追加、删除、重命名、chmod 等。

**使用场景:**
- **输出目录**: `/output/**`、`/results/**`、`/artifacts/**`、`/reports/**`
- **临时目录**: `/tmp/**`、`/workspace/tmp/**`、`/.cache/**`
- **构建目录**: `/build/**`、`/dist/**`、`/.next/**`(用于框架)
- **测试文件**: `/tests/fixtures/**`(在测试场景中)

**示例:**

```python
permissions = [
    {"pattern": "**/*", "permission": "read"},          # 默认:只读
    {"pattern": "/output/**", "permission": "write"},   # 输出可写
    {"pattern": "/tmp/**", "permission": "write"},      # 临时可写
    {"pattern": "/build/**", "permission": "write"},    # 构建工件可写
]

# Agent 可以写入输出
$ echo "Analysis complete" > /workspace/output/report.txt
# 输出:(成功)

$ mkdir /workspace/output/charts
$ echo "data" > /workspace/output/charts/plot.json
# 输出:(成功)
```

## 模式匹配

### 模式类型

AgentFense 支持三种类型的模式,每种都有不同的匹配行为和优先级:

#### 1. Glob 模式(最常见)

**语法:** 使用 `**` 和 `*` 通配符

| 模式 | 匹配 | 优先级(默认) |
|------|------|-------------|
| `**/*` | 所有文件(递归) | 1 (glob) |
| `**/*.py` | 所有 Python 文件 | 1 (glob) |
| `*.txt` | 仅根目录中的 `.txt` 文件 | 1 (glob) |
| `/secrets/**` | `/secrets/` 下的所有文件 | 1 (glob) |
| `**/.env*` | 任何地方的 `.env`、`.env.local` 等 | 1 (glob) |
| `**/node_modules/**` | 所有 `node_modules` 目录 | 1 (glob) |

**示例:**

```python
# 隐藏任何地方的所有 .env 文件
{"pattern": "**/.env*", "permission": "none"}

# 所有 Python 文件只读
{"pattern": "**/*.py", "permission": "read"}

# /secrets/ 下的所有文件隐藏
{"pattern": "/secrets/**", "permission": "none"}
```

#### 2. 目录模式

**语法:** 以 `/` 结尾的路径

| 模式 | 匹配 | 优先级(默认) |
|------|------|-------------|
| `/docs/` | `/docs/` 及子目录中的所有文件 | 2 (directory) |
| `/src/utils/` | `/src/utils/` 及子目录中的所有文件 | 2 (directory) |

**示例:**

```python
# /docs/ 中的所有文件可写
{"pattern": "/docs/", "permission": "write", "type": "directory"}

# /src/ 中的所有文件只读
{"pattern": "/src/", "permission": "read", "type": "directory"}
```

**注意:** 目录模式需要规则中的显式 `"type": "directory"`,否则模式将被视为 glob。

#### 3. 文件模式(精确匹配)

**语法:** 精确文件路径

| 模式 | 匹配 | 优先级(默认) |
|------|------|-------------|
| `/config.yaml` | 仅 `/config.yaml` | 3 (file) |
| `/secrets/public.key` | 仅 `/secrets/public.key` | 3 (file) |

**示例:**

```python
# 特定文件隐藏
{"pattern": "/secrets/private.key", "permission": "none", "type": "file"}

# 特定文件可写(例外)
{"pattern": "/output/current.txt", "permission": "write", "type": "file"}
```

### 模式优先级

当多个模式匹配一个路径时,**最高优先级获胜**。优先级由以下决定:

1. **显式 `priority` 字段**(如果设置) - 最高获胜
2. **模式类型** - `file`(3) > `directory`(2) > `glob`(1)
3. **模式特异性** - 更具体的模式获胜

#### 示例:优先级实际应用

```python
rules = [
    {"pattern": "**/*", "permission": "read"},                  # 优先级:0,类型:glob(1)
    {"pattern": "/secrets/**", "permission": "none"},           # 优先级:0,类型:glob(1)
    {"pattern": "/secrets/public.key", "permission": "read"},   # 优先级:0,类型:file(3)
]

# 评估:
# /app/main.py           → "read" (被 **/* 匹配)
# /secrets/private.key   → "none" (被 /secrets/** 匹配 - 比 **/* 更具体)
# /secrets/public.key    → "read" (类型:file 覆盖 glob /secrets/**)
```

**为什么有效:**
- `**/*` 匹配所有内容,优先级为 1(glob)
- `/secrets/**` 也有优先级 1(glob),但对 `/secrets/` 下的路径更具体
- `/secrets/public.key` 有优先级 3(file),**覆盖** glob `/secrets/**`

#### 示例:显式优先级

```python
rules = [
    {"pattern": "**/*", "permission": "read", "priority": 0},
    {"pattern": "**/.env*", "permission": "none", "priority": 100},  # 高优先级
    {"pattern": "/config/.env.public", "permission": "read", "priority": 200},  # 更高
]

# 评估:
# /app/main.py                → "read" (优先级 0)
# /config/.env                → "none" (优先级 100 覆盖 0)
# /config/.env.local          → "none" (优先级 100 覆盖 0)
# /config/.env.public         → "read" (优先级 200 覆盖 100)
```

## 构建权限策略

### 策略 1:默认拒绝 + 显式允许

**推荐用于安全关键场景。**

```python
permissions = [
    # 步骤 1:默认为最大限制(所有不可见)
    {"pattern": "**/*", "permission": "none", "priority": 0},
    
    # 步骤 2:显式允许特定路径
    {"pattern": "/src/**", "permission": "read", "priority": 10},
    {"pattern": "/docs/**", "permission": "read", "priority": 10},
    {"pattern": "/output/**", "permission": "write", "priority": 10},
]

# 结果:
# /src/app.py        → read (显式允许)
# /data/secret.json  → none (默认拒绝)
# /output/log.txt    → write (显式允许)
```

### 策略 2:默认允许 + 显式拒绝

**推荐用于 agent 探索场景。**

```python
permissions = [
    # 步骤 1:默认为读取访问(agent 可以探索)
    {"pattern": "**/*", "permission": "read", "priority": 0},
    
    # 步骤 2:显式隐藏敏感路径
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/.git/**", "permission": "none", "priority": 50},
    
    # 步骤 3:允许写入特定目录
    {"pattern": "/output/**", "permission": "write", "priority": 10},
    {"pattern": "/tmp/**", "permission": "write", "priority": 10},
]

# 结果:
# /src/app.py        → read (默认允许)
# /.env              → none (显式隐藏)
# /secrets/api.key   → none (显式隐藏)
# /output/log.txt    → write (显式允许)
```

### 策略 3:分层权限

**推荐用于具有多个权限区域的复杂项目。**

```python
permissions = [
    # 层 1:基础规则(读取所有)
    {"pattern": "**/*", "permission": "read", "priority": 0},
    
    # 层 2:密钥(隐藏)
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    
    # 层 3:元数据(仅查看)
    {"pattern": "/metadata/**", "permission": "view", "priority": 20},
    {"pattern": "/schemas/**", "permission": "view", "priority": 20},
    
    # 层 4:可写区域
    {"pattern": "/output/**", "permission": "write", "priority": 10},
    {"pattern": "/logs/**", "permission": "write", "priority": 10},
    {"pattern": "/tmp/**", "permission": "write", "priority": 10},
    
    # 层 5:例外(特定文件)
    {"pattern": "/secrets/public.key", "permission": "read", "priority": 200},
    {"pattern": "/config/readonly.yaml", "permission": "view", "priority": 200},
]
```

## 常见模式

### 隐藏所有密钥

```python
secret_patterns = [
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "**/secrets/**", "permission": "none", "priority": 100},
    {"pattern": "**/*.key", "permission": "none", "priority": 100},
    {"pattern": "**/*.pem", "permission": "none", "priority": 100},
    {"pattern": "**/*.p12", "permission": "none", "priority": 100},
    {"pattern": "**/credentials*", "permission": "none", "priority": 100},
    {"pattern": "**/*_secret*", "permission": "none", "priority": 100},
    {"pattern": "**/*_token*", "permission": "none", "priority": 100},
    {"pattern": "**/id_rsa*", "permission": "none", "priority": 100},
    {"pattern": "**/id_ed25519*", "permission": "none", "priority": 100},
]
```

### 代码审查模式(只读)

```python
code_review = [
    # 所有代码可读
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    {"pattern": "**/*.ts", "permission": "read"},
    {"pattern": "**/*.go", "permission": "read"},
    {"pattern": "**/*.java", "permission": "read"},
    
    # 文档可读
    {"pattern": "**/README*", "permission": "read"},
    {"pattern": "**/*.md", "permission": "read"},
    
    # 隐藏密钥
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    {"pattern": "/secrets/**", "permission": "none", "priority": 100},
    
    # 隐藏构建工件和依赖
    {"pattern": "**/node_modules/**", "permission": "none"},
    {"pattern": "**/__pycache__/**", "permission": "none"},
    {"pattern": "**/dist/**", "permission": "none"},
    {"pattern": "**/build/**", "permission": "none"},
]
```

### CI/CD 流水线模式

```python
ci_cd = [
    # 读取源代码
    {"pattern": "**/*.py", "permission": "read"},
    {"pattern": "**/*.js", "permission": "read"},
    {"pattern": "/package.json", "permission": "read"},
    {"pattern": "/requirements.txt", "permission": "read"},
    
    # 写入构建输出
    {"pattern": "/build/**", "permission": "write"},
    {"pattern": "/dist/**", "permission": "write"},
    {"pattern": "/.cache/**", "permission": "write"},
    
    # 隐藏密钥
    {"pattern": "**/.env*", "permission": "none", "priority": 100},
    
    # 隐藏不必要的文件
    {"pattern": "**/node_modules/**", "permission": "none"},
    {"pattern": "**/.git/**", "permission": "none"},
]
```

## 调试权限规则

### 测试权限评估

使用 SDK 测试哪个权限应用于路径:

```python
from agentfense import Sandbox

permissions = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/secrets/**", "permission": "none"},
    {"pattern": "/secrets/public.key", "permission": "read"},
]

with Sandbox.from_local("./project", permissions=permissions) as sandbox:
    # 测试 /secrets/private.key
    result = sandbox.run("cat /workspace/secrets/private.key")
    if result.exit_code != 0:
        print(f"访问被拒绝: {result.stderr}")  # 期望:No such file or directory
    
    # 测试 /secrets/public.key
    result = sandbox.run("cat /workspace/secrets/public.key")
    if result.exit_code == 0:
        print(f"访问授予: {result.stdout}")
```

### 常见调试步骤

1. **检查模式语法**:确保递归使用 `**`,单层使用 `*`
2. **验证优先级**:更高优先级应覆盖更低优先级
3. **测试特异性**:更具体的模式应获胜(file > directory > glob)
4. **检查路径前缀**:像 `/secrets/**` 这样的模式需要前导 `/`

## 性能考虑

### 规则数量影响

- **小规则集(<10 条规则)**:可忽略开销(每个文件操作约 1μs)
- **中等规则集(10-50 条规则)**:低开销(每个文件操作约 5-10μs)
- **大规则集(>100 条规则)**:中等开销(每个文件操作约 50μs)

**建议:** 为获得最佳性能,将规则集保持在 50 条以下。使用 glob 模式折叠多个相似规则。

### 模式优化

```python
# 效率较低:许多特定模式
rules = [
    {"pattern": "/secrets/api.key", "permission": "none"},
    {"pattern": "/secrets/db.key", "permission": "none"},
    {"pattern": "/secrets/jwt.key", "permission": "none"},
    # ... 20 条更多相似规则
]

# 更高效:单个 glob 模式
rules = [
    {"pattern": "/secrets/**", "permission": "none"},
]
```

## 下一步

- **[Preset 指南](presets.md)** - 预配置权限模板
- **[最佳实践](best-practices.md)** - 生产部署安全模式
- **[安全模型](index.md)** - 整体安全架构
