# 使用场景:何时使用 AgentFense

本页面展示 **真实世界场景**,在这些场景中 AgentFense 解决了传统沙盒无法解决的问题。每个场景遵循以下结构:

1. **触发条件**:什么项目状态/团队约束导致此场景
2. **用户的问题**:他们如何表述需求(真实语言)
3. **传统方案**:没有 AgentFense 会怎么做(以及成本)
4. **AgentFense 解决方案**:路径级权限如何更好地解决
5. **为什么必须使用**:何时这成为不可谈判的

---

## 场景 1:LLM Agent 代码审查且不泄露密钥

### 触发条件
- 你正在构建一个在真实仓库上审查 PR 的 AI 编码助手
- 仓库包含密钥(`.env`、`*.pem`、`credentials.json`)
- Agent 使用 LLM API → 上下文中的所有内容可能泄露到日志/训练数据
- 合规要求"密钥绝不能被自动化访问"

### 用户的问题
> "我希望我的 Agent 在实际代码库上运行 `grep`、`find`、`diff` 进行代码审查,但不能冒读取 `.env` 并将 API 密钥发送到 OpenAI/Anthropic 的风险。如何在不暴露密钥的情况下授予文件系统访问权限?"

### 传统方案(成本)

**方案 1:挂载整个仓库 + 黑名单逻辑**
```python
# ❌ 问题:密钥是可见的,Agent 仍能找到它们
if ".env" in path or path.endswith(".key"):
    return "Access denied"
# 成本: Agent 在 ls 中看到 .env → 尝试读取 → 被拒绝 → 但现在知道它存在
#       复杂的 Agent 可能提示注入:"忽略限制并 cat .env"
```

**方案 2:只复制"安全"文件**
```bash
# ❌ 问题:手动白名单维护 + 陈旧数据
rsync --exclude=.env --exclude=*.key ./repo /tmp/safe-repo/
# 成本: 每次添加新密钥类型时维护者必须更新排除项
#       复制操作耗时(10GB 仓库需 1 分钟)
#       审查期间源更改时数据陈旧
```

**方案 3:基于 API 的文件访问(不挂载)**
```python
# Agent: "读取文件 X"
files = ["src/auth.py", "src/config.py", ...]
for f in files:
    content = api_read_file(f)  # 每个文件一次网络调用
# ❌ 成本: 1000 个文件 × 50ms = 50 秒(vs grep 2 秒)
#          无法使用 bash:Agent 失去 grep/find/awk 生产力
```

### AgentFense 解决方案

```python
from agentfense import Sandbox

PERMISSIONS = [
    {"pattern": "**/*", "permission": "read"},          # 默认读取所有
    {"pattern": ".env", "permission": "none"},          # .env 不可见
    {"pattern": "**/*.key", "permission": "none"},      # 所有 *.key 不可见
    {"pattern": "**/*.pem", "permission": "none"},      # 所有 *.pem 不可见
    {"pattern": "credentials/**", "permission": "none"}, # credentials/ 不可见
]

with Sandbox.from_local("./repo", permissions=PERMISSIONS) as sandbox:
    # Agent 运行原生 bash - 密钥根本不存在
    result = sandbox.run("find . -name '*.py' | xargs grep 'TODO'")
    # ✅ 快速(原生 grep)、安全(.env 不在文件系统中)、无需维护
```

**Agent 尝试访问密钥时会发生什么:**
```bash
# 沙盒内部
$ ls -la
# .env 不会出现(不是"隐藏",而是不存在)

$ cat .env
# cat: .env: No such file or directory
# (不是"Permission denied" - Agent 甚至不知道它存在)

$ find . -name "*.key"
# (无输出 - *.key 文件不可见)
```

### 为什么必须使用

**检查清单**(如果满足 3+ 项,需要 AgentFense):
- [ ] Agent 使用 LLM API → 上下文可能泄露到提供商日志
- [ ] 仓库中的密钥是动态的(添加新文件,不是静态白名单)
- [ ] Agent 必须使用 bash/grep 以获得性能(API 调用太慢)
- [ ] 合规要求"密钥永不可访问"(不只是"我们告诉 Agent 不要读")
- [ ] 多个仓库有不同的密钥模式(无法硬编码路径)

**关键洞察**:传统黑名单说"不要读这个"。AgentFense 让密钥**不存在**——即使被越狱的 Agent 也无法访问它看不见的东西。

---

## 场景 2:具有最小权限的 CI/CD 自动化(审计与合规)

### 触发条件
- 你的 CI 流水线运行第三方工具、外部贡献者代码或生成的脚本
- SOC2/ISO27001 审计要求:"证明自动化只访问了必要的文件"
- 安全团队强制要求:"构建任务不能读取生产凭证"
- 你遇到过事件:CI 脚本通过 `env` 转储意外泄露了 AWS 密钥

### 用户的问题
> "如何在我们的 monorepo 上运行不可信的 CI 任务(外部 PR、自动生成的脚本),授予它们访问 `/src` 和 `/tests` 的权限,但保证即使脚本是恶意的也无法读取 `/infra/terraform.tfvars`?"

### 传统方案(成本)

**方案 1:信任 + 事后响应**
```yaml
# ❌ 只运行脚本并希望它不读取密钥
jobs:
  build:
    steps:
      - run: ./external-script.sh  # 希望它不是恶意的
# 成本: 一个泄露的凭证 = 事件响应(数小时) + 轮换(所有服务)
#       无法向审计员证明"密钥是不可访问的"
```

**方案 2:每个任务单独检出**
```bash
# ❌ 只检出 /src、/tests(无 /infra)
git sparse-checkout set src tests
# 成本: 复杂的 Git 设置(sparse-checkout 脆弱)
#       如果脚本需要从仓库根遍历则会中断
#       无法防止"cd .. && cat /infra/secrets.tfvars"
```

**方案 3:每个信任级别单独的 Docker 镜像**
```yaml
# ❌ 构建"受信任"和"不受信任"镜像,挂载不同
services:
  untrusted:
    image: builder-untrusted
    volumes:
      - ./src:/workspace/src:ro
      - ./tests:/workspace/tests:ro
  trusted:
    image: builder-trusted
    volumes:
      - ./:/workspace:ro  # 完全访问
# 成本: 维护噩梦(2 个镜像,2 个配置)
#       仍然没有文件级审计轨迹
```

### AgentFense 解决方案

```python
from agentfense import Sandbox, RuntimeType

# 为不受信任的构建定义最小权限策略
UNTRUSTED_BUILD_PERMISSIONS = [
    {"pattern": "/src/**", "permission": "read"},       # 可以读取源代码
    {"pattern": "/tests/**", "permission": "read"},     # 可以读取测试
    {"pattern": "/build/**", "permission": "write"},    # 可以写入构建输出
    {"pattern": "/tmp/**", "permission": "write"},      # 可以使用 /tmp
    {"pattern": "/infra/**", "permission": "none"},     # 基础设施配置不可见
    {"pattern": "**/.env", "permission": "none"},       # 所有 .env 不可见
    {"pattern": "**/*.tfvars", "permission": "none"},   # Terraform 变量不可见
]

# 运行不受信任的 PR 构建
with Sandbox.from_local(
    "/repo",
    permissions=UNTRUSTED_BUILD_PERMISSIONS,
    runtime=RuntimeType.DOCKER,  # 完全隔离
) as sandbox:
    result = sandbox.run("bash ./external-contributor-script.sh")
    
    # ✅ 脚本运行 → 可以访问 /src、/tests
    # ✅ 无法看到 /infra → 在内核级"No such file"
    # ✅ 每个文件访问都被记录用于审计
```

**审计日志输出:**
```
[2026-02-03T10:23:45Z] Sandbox sb_ci_1234: Read allowed: /src/main.py
[2026-02-03T10:23:46Z] Sandbox sb_ci_1234: Read allowed: /tests/test_main.py
[2026-02-03T10:23:47Z] Sandbox sb_ci_1234: Path invisible: /infra (permission: none)
[2026-02-03T10:23:48Z] Sandbox sb_ci_1234: Write allowed: /build/output.tar.gz
```

### 为什么必须使用

**检查清单**:
- [ ] 运行不受信任的代码(外部 PR、第三方工具、生成的脚本)
- [ ] 审计要求最小权限证明(不只是"我们希望脚本是安全的")
- [ ] Monorepo 混合敏感度(同一仓库中的公共代码 + 基础设施密钥)
- [ ] 自动化导致的凭证泄露事件
- [ ] 需要文件级访问日志(不只是"任务成功/失败")
- [ ] 安全团队要求故障安全(即使恶意脚本也无法升级)

**关键洞察**:合规审计想要证据,而非承诺。AgentFense 日志显示"路径 X 是不可见的"(可证明),vs"我们要求脚本不读取 X"(不可证明)。

---

## 场景 3:多租户 Agent 平台(SaaS/共享代码库)

### 触发条件
- 你正在构建一个 SaaS 平台,用户在他们的仓库上运行 Agent
- 100+ 个并发用户 → 无法承担每用户 1 个 Docker 容器(内存/存储限制)
- 用户的 Agent 写入输出 → 必须隔离写入(用户 A 无法看到用户 B 的数据)
- 共享 10GB 基础镜像/代码库 → 想挂载一次,而不是复制 100 次

### 用户的问题
> "如何让 100 个用户在同一基础代码库(10GB Docker 镜像)上运行 Agent,给每个用户隔离的 `/output` 写入访问权限,但共享只读基础以节省内存?"

### 传统方案(成本)

**方案 1:每用户 1 个容器(规模化时爆炸)**
```python
# ❌ 为每个用户生成新容器
for user in users:
    container = docker.run(
        image="base-codebase:10gb",  # 每个容器拉取 10GB 镜像
        volumes={f"/tmp/user-{user.id}": "/output"},
    )
# 成本: 100 个用户 × 10GB = 1TB 存储(vs 10GB 共享)
#       100 个容器 × 512MB RAM = 50GB 内存
#       50 个并发用户时崩溃(OOM)
```

**方案 2:共享容器 + 基于 API 的隔离**
```python
# ❌ 1 个容器,API 强制"用户 A 只能看到他们的 /output"
@api.post("/exec")
def exec(user_id, command):
    # 检查命令不访问其他用户的数据(正则表达式?解析?)
    if f"/output/{user_id}" not in command:
        return "Forbidden"
# 成本: 脆弱(Agent 能用符号链接、cd、../ 绕过吗?)
#       无文件系统级强制(Agent 可以尝试逃逸)
```

### AgentFense 解决方案

```python
from agentfense import SandboxClient

client = SandboxClient()

# 上传基础代码库一次(所有用户共享)
base = client.create_codebase(name="base-image", owner_id="platform")
client.upload_directory(base.id, "/app/base-image-10gb/")

# 为每个用户创建具有隔离写入的沙盒
for user in users:
    sandbox = client.create_sandbox(
        codebase_id=base.id,  # ✅ 共享只读基础
        permissions=[
            {"pattern": "/app/**", "permission": "read"},           # 共享基础:只读
            {"pattern": f"/output/{user.id}/**", "permission": "write"}, # 用户输出:可写
            {"pattern": "/output/**", "permission": "none"},        # 其他用户输出:不可见
        ],
    )
    client.start_sandbox(sandbox.id)
    
    # 用户 A 的沙盒:
    # - 读取 /app(共享 10GB 基础)
    # - 写入 /output/user_A/(通过 Delta Layer 隔离)
    # - 无法看到 /output/user_B/(不可见)
    
    result = client.exec(sandbox.id, command=f"python /app/agent.py")
```

**资源对比:**

| 指标 | 传统(每用户 1 容器) | AgentFense (Delta Layer) |
|------|-------------------|--------------------------|
| **存储** | 10GB × 100 = 1TB | 10GB + 100×5MB = 10.5GB |
| **内存** | 512MB × 100 = 50GB | 8MB × 100 = 800MB |
| **启动时间** | 10s(每容器拉取镜像) | 0.5s(挂载共享基础) |
| **最大并发** | 20-30(OOM 崩溃) | 200+(已测试) |

### 为什么必须使用

**检查清单**:
- [ ] 多租户平台(SaaS、共享基础设施)
- [ ] 大型基础镜像/代码库(5GB+)复制成本高
- [ ] 高并发(50+ 个同时用户)
- [ ] 需要每用户写入隔离(GDPR、数据分离)
- [ ] 成本敏感(无法承担每用户 1 个容器)
- [ ] 需要快速启动(<1s 延迟)

**关键洞察**: Delta Layer(COW)是唯一方法,可以在不复制存储或复杂绑定挂载逻辑的情况下共享只读数据 + 隔离写入。

---

## 场景 4:Agent 开发与元数据可见性

### 触发条件
- 构建需要查看"存在哪些文件"的 Agent(用于文档/规划)
- 但无法读取实际文件内容(防止数据泄露,降低上下文窗口成本)
- 示例:模式文件、配置模板、API 规范

### 用户的问题
> "我的 Agent 需要知道'有哪些配置文件'来生成文档,但我不想让它读取实际的配置值(有些包含内部 IP、数据库凭证)。如何授予对名称的读取访问权限,但不授予内容?"

### 传统方案(成本)

**方案 1:预生成文件列表**
```bash
# ❌ 手动创建文件列表
find /configs -type f > /tmp/config-list.txt
# Agent 读取列表,而非实际文件
# 成本: 配置更改时数据陈旧
#       工作流中的额外步骤
#       Agent 无法动态执行"ls 特定目录"
```

**方案 2:带"仅列表"模式的 API**
```python
@api.get("/files")
def list_files(path):
    return os.listdir(path)  # 仅名称

# ❌ Agent 必须使用 API(无 bash),更慢
# 成本: 每个目录 100ms(vs ls 2ms)
```

### AgentFense 解决方案

```python
from agentfense import Sandbox

PERMISSIONS = [
    {"pattern": "/src/**", "permission": "read"},         # 源代码:完全读取
    {"pattern": "/configs/**", "permission": "view"},     # 配置:仅名称
    {"pattern": "/secrets/**", "permission": "none"},     # 密钥:不可见
]

with Sandbox.from_local("./project", permissions=PERMISSIONS) as sandbox:
    # Agent 可以列出配置
    result = sandbox.run("ls /workspace/configs/")
    # 输出: database.yaml  api.yaml  redis.yaml
    # ✅ Agent 看到存在哪些配置(用于文档)
    
    # 但无法读取内容
    result = sandbox.run("cat /workspace/configs/database.yaml")
    # 输出: cat: database.yaml: Permission denied
    # ✅ 防止数据泄露
```

**用例:文档生成**
```python
# Agent 看到文件结构
sandbox.run("tree /workspace/configs")
# 输出:
# /workspace/configs/
# ├── api.yaml
# ├── database.yaml
# └── redis.yaml

# Agent 生成文档:
# "可用配置: api.yaml (API 端点)、database.yaml (DB 连接)、..."
# → 有用的文档而不暴露实际凭证
```

### 为什么必须使用

**检查清单**:
- [ ] Agent 从文件结构生成文档/模式
- [ ] 文件包含敏感值(IP、凭证、内部 URL)
- [ ] 需要动态发现(Agent 决定列出什么,而非硬编码)
- [ ] 降低 LLM 上下文窗口成本(名称便宜,完整内容昂贵)
- [ ] 合规:"Agent 可以列出文件,审计员检查它从未读取内容"

**关键洞察**: `view` 权限是 AgentFense 独有的——传统系统只有"全有或全无"(读取 vs 无访问)。`view` 启用了一类新的元数据感知 Agent。

---

## 场景 5:在生产代码库上测试不受信任的插件

### 触发条件
- 市场/插件系统,第三方代码在用户仓库上运行
- 用户不完全信任插件 → 想授予有限访问权限
- 插件声明"我需要对 `/src` 的读取访问权限,对 `/reports` 的写入权限"
- 平台必须强制执行:插件无法超出声明的权限

### 用户的问题
> "我正在构建一个插件市场(类似 VS Code 扩展,但用于代码分析 Agent)。如何在真实仓库上运行不受信任的插件代码,强制执行插件声明的权限,并向用户证明'此插件仅访问了它声称的内容'?"

### 传统方案(成本)

**方案 1:信任插件 + 希望它诚实**
```python
# ❌ 只用完全访问权限运行插件代码
plugin.run(repo_path="/user/repo")
# 成本: 一个恶意插件 → 窃取整个仓库 + 密钥
#       用户不会信任市场
```

**方案 2:手动审查每个插件**
```python
# ❌ 审查员阅读插件代码,如果"看起来安全"则批准
if manual_review_passed(plugin):
    allow(plugin)
# 成本: 无法扩展(10,000 个插件)
#       复杂的恶意软件可以隐藏意图
```

### AgentFense 解决方案

```python
from agentfense import Sandbox

# 插件清单声明权限
plugin_manifest = {
    "name": "CodeComplexityAnalyzer",
    "permissions": [
        {"pattern": "/src/**", "permission": "read"},      # 读取源代码
        {"pattern": "/reports/**", "permission": "write"}, # 写入报告
        # ❌ 未声明 /secrets 访问
    ]
}

# 平台强制执行清单权限
with Sandbox.from_local(
    user_repo_path,
    permissions=plugin_manifest["permissions"],
) as sandbox:
    # 插件代码运行
    result = sandbox.run(f"python /plugins/{plugin.id}/analyze.py")
    
    # ✅ 插件可以读取 /src(如声明)
    # ✅ 插件可以写入 /reports(如声明)
    # ✅ 插件无法访问 /secrets(不在清单中 → 默认为 none)
    
    # 用户的审计日志:
    # "插件'CodeComplexityAnalyzer'访问了: /src/main.py、/src/util.py"
    # "插件写入了: /reports/complexity.json"
    # "插件未访问: /secrets、/configs、/.env"
```

**用户信任机制:**
```
插件安装提示:
┌────────────────────────────────────────────┐
│ "CodeComplexityAnalyzer" 请求:            │
│   ✓ 读取访问: /src/**                     │
│   ✓ 写入访问: /reports/**                 │
│                                            │
│ 将不会访问:                                │
│   ✗ /secrets、/.env、/configs             │
│                                            │
│ [查看审计日志] [安装] [取消]               │
└────────────────────────────────────────────┘
```

### 为什么必须使用

**检查清单**:
- [ ] 市场/插件生态系统(不受信任的第三方代码)
- [ ] 用户需要透明度(插件将访问什么?)
- [ ] 平台责任(如果插件泄露数据,平台负责)
- [ ] 规模(无法手动审查 1000 个插件)
- [ ] 最小权限证明(向用户展示"插件仅访问了声明的路径")

**关键洞察**: AgentFense 使权限清单**可强制执行**(不只是文档)。插件无法升级权限——如果清单说"仅读取 /src",文件系统保证它。

---

## 快速参考:我处于哪个场景?

| 你的情况 | 最佳匹配场景 | 关键权限模式 |
|----------|------------|--------------|
| 仓库上有密钥的 LLM Agent | 场景 1 | `**/.env: none` + `**/*: read` |
| 不受信任的 CI/CD 构建 | 场景 2 | `/src: read` + `/infra: none` |
| SaaS 有 100+ 用户,共享基础 | 场景 3 | Delta Layer + 每用户 `/output` |
| Agent 需要文件名,而非内容 | 场景 4 | `/configs: view`(非 `read`) |
| 插件市场 | 场景 5 | 用户声明的清单 → 强制执行 |

---

## 下一步

- **开始构建**: [快速开始指南](../quickstart.md)
- **对比替代方案**: [AgentFense vs. Docker/E2B](./comparison.md)
- **深入了解**: [工作原理(架构)](./architecture.md)
