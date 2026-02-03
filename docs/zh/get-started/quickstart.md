# 快速开始

在 **5 分钟内**开始使用 AgentFense。本指南展示如何创建第一个沙盒、执行带权限控制的命令并清理资源。

## 你将学到

- 安装 AgentFense 服务端和 Python SDK
- **一行代码**从本地目录创建沙盒
- 执行命令时自动强制权限
- 使用上下文管理器自动清理

## 前置要求

- **Python 3.8+** 已安装
- **Go 1.21+**(用于服务端)
- **Linux 或 macOS**(支持 Windows WSL2)
- **bubblewrap**(`bwrap`)或 **Docker** 用于隔离

## 步骤 1:安装服务端

### 方案 A:二进制发布(推荐)

```bash
# 下载并解压(将 VERSION 替换为最新版本)
wget https://github.com/AjaxZhan/AgentFense/releases/download/VERSION/agentfense-server-linux-amd64.tar.gz
tar -xzf agentfense-server-linux-amd64.tar.gz
cd agentfense-server

# 启动服务(gRPC :9000,REST :8080)
./agentfense-server -config config.yaml
```

### 方案 B:从源码构建

```bash
git clone https://github.com/AjaxZhan/AgentFense.git
cd AgentFense

# 安装依赖并构建
go mod tidy
go build -o bin/agentfense-server ./cmd/agentfense-server

# 启动服务
./bin/agentfense-server -config configs/agentfense-server.yaml
```

### 方案 C:Docker(快速测试)

```bash
docker run -d \
  -p 9000:9000 -p 8080:8080 \
  -v /tmp/agentfense:/var/lib/agentfense \
  --privileged \
  agentfense/agentfense-server:latest
```

**验证服务运行:**

```bash
curl http://localhost:8080/health
# 期望输出:{"status":"ok"}
```

## 步骤 2:安装 Python SDK

```bash
pip install agentfense
```

**验证安装:**

```bash
python -c "from agentfense import Sandbox; print('AgentFense SDK installed')"
```

## 步骤 3:创建第一个沙盒

创建测试项目:

```bash
mkdir my-project
cd my-project
echo "print('Hello from AgentFense')" > hello.py
echo "SECRET_KEY=my-secret" > .env
```

**运行第一个沙盒(一行代码):**

```python
from agentfense import Sandbox

# 一行代码:使用 "agent-safe" 预设创建沙盒
with Sandbox.from_local("./my-project") as sandbox:
    result = sandbox.run("python hello.py")
    print(result.stdout)  # 输出:Hello from AgentFense
```

**刚才发生了什么?**

1. `Sandbox.from_local()` 将文件上传到服务器
2. 使用 `agent-safe` 预设创建沙盒(隐藏 `.env`,允许读取所有文件,允许写入 `/output` 和 `/tmp`)
3. 启动沙盒
4. 在隔离环境内执行 `python hello.py`
5. 退出时自动清理

## 步骤 4:测试权限控制

尝试访问被隐藏的密钥文件:

```python
from agentfense import Sandbox

with Sandbox.from_local("./my-project") as sandbox:
    # 成功 - hello.py 是可读的
    result = sandbox.run("cat /workspace/hello.py")
    print(result.stdout)  # 输出:print('Hello from AgentFense')
    
    # 失败 - .env 被隐藏(权限:none)
    result = sandbox.run("cat /workspace/.env")
    print(result.exit_code)  # 输出:1(文件未找到)
    print(result.stderr)     # 输出:cat: /workspace/.env: No such file or directory
    
    # 即使 ls 也看不到它
    result = sandbox.run("ls -la /workspace/")
    print(result.stdout)  # 输出:hello.py(没有列出 .env)
```

**关键洞察:** `.env` 文件在**文件系统层面不可见**。即使 Agent 尝试访问也无法成功——无需额外逻辑。

## 步骤 5:自定义权限

定义自己的权限规则:

```python
from agentfense import Sandbox

# 自定义权限
PERMISSIONS = [
    {"pattern": "**/*", "permission": "read"},         # 默认:只读
    {"pattern": "/output/**", "permission": "write"},  # 允许写入 /output
    {"pattern": "**/.env*", "permission": "none"},     # 隐藏所有 .env 文件
    {"pattern": "/docs/**", "permission": "view"},     # 可列出但不可读
]

with Sandbox.from_local("./my-project", permissions=PERMISSIONS) as sandbox:
    # 读取任意文件
    result = sandbox.run("cat hello.py")
    
    # 写入 /output
    result = sandbox.run("echo 'result' > /workspace/output/result.txt")
    assert result.exit_code == 0
    
    # 不能写入其他位置
    result = sandbox.run("echo 'test' > /workspace/data.txt")
    assert result.exit_code != 0  # 权限拒绝
```

## 步骤 6:使用会话(持久化 Shell)

对于多步骤工作流,使用会话保留工作目录和环境:

```python
from agentfense import Sandbox

with Sandbox.from_local("./my-project") as sandbox:
    # 创建持久化会话
    with sandbox.session() as session:
        session.exec("cd /workspace")
        session.exec("export DEBUG=1")
        session.exec("source venv/bin/activate")  # 如果 venv 存在
        
        # 所有之前的状态都被保留
        result = session.exec("python hello.py")
        print(result.stdout)
```

## 步骤 7:Docker 运行时 + 资源限制

获得更强隔离和资源控制:

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./my-project",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB
        pids_limit=100,                  # 最多 100 个进程
    ),
) as sandbox:
    result = sandbox.run("python hello.py")
    print(result.stdout)
```

## 完整示例:带权限控制的 AI Agent

构建一个执行 bash 命令的安全 AI Agent:

```python
from anthropic import Anthropic
from agentfense import Sandbox
import re

client = Anthropic()

PERMISSIONS = [
    {"pattern": "**/*", "permission": "read"},
    {"pattern": "/output/**", "permission": "write"},
    {"pattern": "**/.env*", "permission": "none"},
]

def extract_bash_commands(text: str) -> list:
    """从 ```bash``` 代码块提取命令"""
    return re.findall(r'```bash\n(.*?)```', text, re.DOTALL)

with Sandbox.from_local("./my-project", permissions=PERMISSIONS) as sandbox:
    response = client.messages.create(
        model="claude-sonnet-4-20250514",
        messages=[{"role": "user", "content": "列出所有 Python 文件"}],
        system="在 ```bash``` 代码块中输出 bash 命令。"
    )
    
    commands = extract_bash_commands(response.content[0].text)
    for cmd in commands:
        result = sandbox.run(cmd.strip(), timeout=10)
        print(f"$ {cmd}\n{result.stdout}")
```

即使 Agent 生成 `cat .env` 命令,它也**无法访问 `.env`**——文件在文件系统层面不可见。

## 下一步

现在你已有一个可工作的沙盒,接下来探索:

- **[核心概念](concepts.md)** - 理解 Codebase、权限、会话和 Delta Layer
- **[安装指南](installation.md)** - 详细的服务端设置、依赖、配置
- **[安全模型](../security/index.md)** - AgentFense 如何强制权限
- **[权限系统](../security/permissions.md)** - 深入四级权限
- **[最佳实践](../security/best-practices.md)** - 生产部署和安全模式

## 故障排查

### 服务器连接被拒绝

**问题:** `ConnectionError: failed to connect to localhost:9000`

**解决方案:** 确保服务器正在运行:`./agentfense-server -config config.yaml`

### 命令因权限拒绝退出

**问题:** 写文件时出现 `Permission denied` 错误

**解决方案:** 检查权限规则——除非明确授予 `write` 权限,否则文件默认为只读。使用 `extend_preset()` 或添加显式写权限规则。

### .env 文件仍然可见

**问题:** 密钥文件出现在 `ls` 输出中

**解决方案:** 确保模式正确匹配。对所有目录使用 `**/.env*`,对特定文件使用 `/path/to/.env`。优先级规则生效——更具体的模式会覆盖通用模式。

### macOS 上 Docker 运行时失败

**问题:** `view` 权限无法正常工作

**解决方案:** 这是 macOS 上 Docker Desktop 的已知限制(VirtioFS 问题)。使用 `read` 代替 `view`,或在 Linux 上使用 `bwrap` 运行时。

## 快速参考

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

# 基本用法
with Sandbox.from_local("./path") as sb:
    result = sb.run("command")

# 自定义权限
with Sandbox.from_local("./path", permissions=[...]) as sb:
    result = sb.run("command")

# Docker + 资源限制
with Sandbox.from_local(
    "./path",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(memory_bytes=512*1024*1024),
) as sb:
    result = sb.run("command")

# 会话(持久化 shell)
with sandbox.session() as session:
    session.exec("cd /workspace")
    result = session.exec("command")

# 错误处理
result = sandbox.run("command", timeout=30, raise_on_error=True)
```
