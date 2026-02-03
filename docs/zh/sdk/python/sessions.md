# Sessions 指南

Sessions 在 AgentFense 沙盒中提供**有状态的命令执行**。本指南解释什么是 session、为什么它们必不可少,以及如何有效地使用它们。

---

## 目录

- [什么是 Session?](#什么是-session)
- [为什么使用 Session?](#为什么使用-session)
- [创建 Session](#创建-session)
- [Session 状态管理](#session-状态管理)
- [常见用例](#常见用例)
- [Session vs 直接执行](#session-vs-直接执行)
- [最佳实践](#最佳实践)
- [故障排除](#故障排除)

---

## 什么是 Session?

**Session** 是在沙盒内运行的持久化 shell 进程,它在多个命令执行之间**保留状态**。

### 没有 Session(直接执行)

```python
sandbox.run("cd /workspace/backend")  # Shell 进程 #1
sandbox.run("source venv/bin/activate")  # Shell 进程 #2 (全新开始)
sandbox.run("python manage.py test")  # Shell 进程 #3 (全新开始)
# ❌ 每个命令在新 shell 中运行 - 没有状态保留
```

**结果**:`cd` 和 `source` 命令对后续命令**没有影响**,因为每个 `run()` 都启动一个全新的 shell。

---

### 使用 Session(有状态执行)

```python
with sandbox.session() as session:
    session.exec("cd /workspace/backend")  # Shell 进程持续
    session.exec("source venv/bin/activate")  # 同一 shell,virtualenv 已激活
    session.exec("python manage.py test")  # 同一 shell,仍在 backend/ 中使用 venv
# ✓ 所有命令在同一 shell 中运行 - 状态持续
```

**结果**:每个命令都建立在前一个状态之上。最终的 `python` 命令在 `/workspace/backend/` 中运行并激活了 virtualenv。

---

## 为什么使用 Session?

当你需要以下功能时,Session 是**必需的**:

1. **更改工作目录(`cd`)**
   ```python
   with sandbox.session() as session:
       session.exec("cd /workspace/src")
       session.exec("pytest")  # 在 /workspace/src 中运行
   ```

2. **设置环境变量(`export`)**
   ```python
   with sandbox.session() as session:
       session.exec("export DEBUG=1")
       session.exec("python app.py")  # DEBUG=1 已设置
   ```

3. **激活虚拟环境(`source`)**
   ```python
   with sandbox.session() as session:
       session.exec("source venv/bin/activate")
       session.exec("pip list")  # 显示 venv 包
   ```

4. **运行后台进程**
   ```python
   with sandbox.session() as session:
       session.exec("redis-server &")  # 启动后台服务器
       session.exec("python test.py")  # 可以连接到 redis
   ```

5. **具有依赖关系的多步工作流**
   ```python
   with sandbox.session() as session:
       session.exec("npm install")  # 安装依赖
       session.exec("npm run build")  # 使用已安装的依赖
       session.exec("npm test")  # 使用构建产物
   ```

---

## 创建 Session

### 同步 API

```python
from agentfense import Sandbox

with Sandbox.from_local("./project") as sandbox:
    with sandbox.session() as session:
        result = session.exec("pwd")
        print(result.stdout)  # /workspace
```

---

### 异步 API

```python
from agentfense import AsyncSandbox

async with await AsyncSandbox.from_local("./project") as sandbox:
    async with await sandbox.session() as session:
        result = await session.exec("pwd")
        print(result.stdout)  # /workspace
```

---

### Session 参数

```python
def session(
    self,
    shell: str = "/bin/sh",
    env: Optional[Dict[str, str]] = None,
) -> SessionWrapper
```

| 参数 | 类型 | 默认值 | 描述 |
|-----------|------|---------|-------------|
| `shell` | `str` | `"/bin/sh"` | 要使用的 shell 二进制文件(`/bin/sh`、`/bin/bash`、`/bin/zsh` 等) |
| `env` | `Dict[str, str]` | `None` | 初始环境变量 |

**示例:自定义 Shell**
```python
with sandbox.session(shell="/bin/bash") as session:
    # Bash 特定语法(数组等)
    session.exec("declare -a arr=(1 2 3)")
```

**示例:初始环境**
```python
with sandbox.session(env={"DEBUG": "1", "ENV": "test"}) as session:
    result = session.exec("printenv DEBUG")
    print(result.stdout)  # 1
```

---

## Session 状态管理

### 工作目录

当前工作目录(`cwd`)在命令之间持续:

```python
with sandbox.session() as session:
    session.exec("cd /workspace/backend")
    result = session.exec("pwd")
    print(result.stdout)  # /workspace/backend
    
    session.exec("cd ../frontend")
    result = session.exec("pwd")
    print(result.stdout)  # /workspace/frontend
```

---

### 环境变量

使用 `export` 设置的环境变量持续:

```python
with sandbox.session() as session:
    session.exec("export API_URL=http://localhost:8000")
    session.exec("export DB_NAME=testdb")
    
    result = session.exec("printenv API_URL")
    print(result.stdout)  # http://localhost:8000
```

---

### 别名和函数

Shell 别名和函数被保留:

```python
with sandbox.session(shell="/bin/bash") as session:
    session.exec("alias ll='ls -la'")
    result = session.exec("ll")
    print(result.stdout)  # 运行 ls -la
    
    session.exec("myfunction() { echo 'Hello from function'; }")
    result = session.exec("myfunction")
    print(result.stdout)  # Hello from function
```

---

### 后台进程

后台进程(`&`)继续运行:

```python
with sandbox.session() as session:
    # 启动后台服务器
    session.exec("python -m http.server 8000 &")
    
    # 等待服务器启动
    import time
    time.sleep(2)
    
    # 服务器仍在运行
    result = session.exec("curl http://localhost:8000")
    print(result.stdout)  # HTML 目录列表
```

**重要**:当 session 关闭时,后台进程会被杀死。

---

## 常见用例

### 用例 1:Python 虚拟环境

```python
with sandbox.session() as session:
    # 创建 virtualenv
    session.exec("python -m venv venv")
    
    # 激活它
    session.exec("source venv/bin/activate")
    
    # 安装依赖
    session.exec("pip install pytest requests")
    
    # 运行测试(使用 venv 的 pytest)
    result = session.exec("pytest tests/")
    print(result.exit_code)
```

---

### 用例 2:Node.js 项目设置

```python
with sandbox.session() as session:
    # 安装依赖
    session.exec("cd /workspace")
    session.exec("npm install")
    
    # 运行 linter
    lint_result = session.exec("npm run lint")
    
    # 运行测试
    test_result = session.exec("npm test")
    
    # 构建生产包
    if test_result.exit_code == 0:
        session.exec("npm run build")
```

---

### 用例 3:测试的数据库设置

```python
with sandbox.session() as session:
    # 在后台启动数据库
    session.exec("mysqld --datadir=/tmp/mysql &")
    
    # 等待启动
    import time
    time.sleep(3)
    
    # 运行迁移
    session.exec("python manage.py migrate")
    
    # 运行测试
    result = session.exec("python manage.py test")
```

---

### 用例 4:多阶段 CI 流水线

```python
def ci_pipeline(sandbox):
    """在一个 session 中运行完整的 CI 流水线。"""
    with sandbox.session() as session:
        # 阶段 1:设置
        session.exec("cd /workspace")
        session.exec("export CI=true")
        
        # 阶段 2:安装
        result = session.exec("pip install -r requirements.txt")
        if result.exit_code != 0:
            return False, "安装失败"
        
        # 阶段 3:Lint
        result = session.exec("flake8 .")
        if result.exit_code != 0:
            return False, "Linting 失败"
        
        # 阶段 4:测试
        result = session.exec("pytest --cov")
        if result.exit_code != 0:
            return False, "测试失败"
        
        # 阶段 5:构建
        result = session.exec("python setup.py bdist_wheel")
        if result.exit_code != 0:
            return False, "构建失败"
        
        return True, "流水线成功"

# 使用
with Sandbox.from_local("./project") as sandbox:
    success, message = ci_pipeline(sandbox)
    print(message)
```

---

### 用例 5:交互式调试

```python
with sandbox.session() as session:
    # 设置环境
    session.exec("cd /workspace")
    session.exec("source .env")
    
    # 运行调试器命令
    session.exec("python -m pdb main.py")
    session.exec("break main.py:42")
    session.exec("continue")
    
    # 检查变量
    result = session.exec("print(local_vars)")
    print(result.stdout)
```

---

## Session vs 直接执行

### 何时使用直接执行(`run()` / `exec()`)

✅ **一次性命令**:单个独立命令  
✅ **不需要状态**:命令不依赖之前的状态  
✅ **最大隔离**:每个命令都重新开始  
✅ **简单性**:无需管理 session 生命周期  

**示例**:
```python
# 这些是独立的 - 不需要 session
sandbox.run("python --version")
sandbox.run("ls -la")
sandbox.run("cat README.md")
```

---

### 何时使用 Session(`session()`)

✅ **工作目录更改**:需要 `cd` 持续  
✅ **环境设置**:需要 `export`、`source`、`alias`  
✅ **Virtualenv/工具**:激活 Python venv、Node.js NVM 等  
✅ **后台服务**:启动服务器,针对它运行测试  
✅ **多步工作流**:每一步建立在前一步之上  

**示例**:
```python
# 这些相互依赖 - 需要 session
with sandbox.session() as session:
    session.exec("cd backend/")
    session.exec("source venv/bin/activate")
    session.exec("pytest")
```

---

### 性能对比

| 方面 | 直接执行 | Session |
|--------|------------------|---------|
| **启动开销** | 每个命令约 10-50ms | 一次约 50ms,然后每个命令 5-20ms |
| **状态隔离** | 完全(每次新 shell) | 共享(同一 shell) |
| **最适合** | 独立命令 | 依赖工作流 |

**建议**:对于 3+ 个依赖命令使用 session。对于 1-2 个独立命令,使用直接执行。

---

## 最佳实践

### 1. 始终使用上下文管理器

```python
# ✓ 好:自动清理
with sandbox.session() as session:
    session.exec("command")
# Session 自动关闭

# ✗ 差:手动清理(容易忘记)
session = sandbox.session()
session.exec("command")
session.close()  # 容易忘记
```

---

### 2. 检查退出码

```python
with sandbox.session() as session:
    result = session.exec("make build")
    if result.exit_code != 0:
        print(f"构建失败: {result.stderr}")
        return  # 失败时提前退出
    
    # 仅在构建成功时继续
    session.exec("make test")
```

---

### 3. 尽早设置环境

```python
# ✓ 好:在 session 创建时设置环境
with sandbox.session(env={"CI": "1", "DEBUG": "0"}) as session:
    session.exec("pytest")

# ✗ 效率较低:为每个命令设置环境
with sandbox.session() as session:
    session.exec("export CI=1")  # 额外命令
    session.exec("pytest")
```

---

### 4. 尽可能使用绝对路径

```python
# ✓ 好:绝对路径(明确)
with sandbox.session() as session:
    session.exec("python /workspace/scripts/test.py")

# ✗ 风险:相对路径(取决于 cwd)
with sandbox.session() as session:
    session.exec("python scripts/test.py")  # 如果 cwd != /workspace 则失败
```

---

### 5. 限制 Session 范围

```python
# ✓ 好:每个逻辑工作流一个 session
with sandbox.session() as session:
    # 后端测试
    session.exec("cd /workspace/backend")
    session.exec("pytest")

with sandbox.session() as session:
    # 前端测试(全新 session)
    session.exec("cd /workspace/frontend")
    session.exec("npm test")

# ✗ 差:为不相关的工作流重用 session
with sandbox.session() as session:
    session.exec("cd /workspace/backend")
    session.exec("pytest")
    session.exec("cd /workspace/frontend")  # 交叉污染风险
    session.exec("npm test")
```

---

## 故障排除

### 问题:`cd` 不持续

**症状**:
```python
sandbox.run("cd /workspace/src")
result = sandbox.run("pwd")
print(result.stdout)  # /workspace (不是 /workspace/src)
```

**解决方案**:使用 session:
```python
with sandbox.session() as session:
    session.exec("cd /workspace/src")
    result = session.exec("pwd")
    print(result.stdout)  # /workspace/src
```

---

### 问题:环境变量未设置

**症状**:
```python
sandbox.run("export DEBUG=1")
result = sandbox.run("printenv DEBUG")
print(result.stdout)  # (空)
```

**解决方案**:使用 session 或传递 `env`:
```python
# 选项 1:Session
with sandbox.session() as session:
    session.exec("export DEBUG=1")
    result = session.exec("printenv DEBUG")
    print(result.stdout)  # 1

# 选项 2:传递 env 参数
result = sandbox.run("printenv DEBUG", env={"DEBUG": "1"})
print(result.stdout)  # 1
```

---

### 问题:后台进程终止

**症状**:
```python
with sandbox.session() as session:
    session.exec("redis-server &")
# Session 关闭,redis-server 被杀死
```

**解决方案**:只要需要后台进程,就保持 session 打开:
```python
with sandbox.session() as session:
    session.exec("redis-server &")
    time.sleep(2)  # 等待启动
    
    # 在 session 打开时使用服务器
    result = session.exec("redis-cli ping")
    print(result.stdout)  # PONG
# 现在 session 关闭并杀死后台进程
```

---

### 问题:Session 在交互式命令上挂起

**症状**:
```python
with sandbox.session() as session:
    session.exec("python")  # 挂起等待输入
```

**解决方案**:避免交互式命令或使用 stdin:
```python
# 选项 1:非交互式标志
session.exec("python -c 'print(1+1)'")

# 选项 2:使用 stdin(session 中不支持,使用直接 exec)
result = sandbox.exec("python", stdin="print(1+1)\nexit()\n")
```

---

## 另请参阅

- [高层 API 指南](./high-level.md) - `Sandbox` 类参考
- [异步 API 指南](./async.md) - 异步 session 使用
- [最佳实践](../best-practices.md) - SDK 通用建议
