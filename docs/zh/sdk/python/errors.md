# 错误处理指南

AgentFense SDK 使用**语义化异常**提供清晰、可操作的错误信息。本指南涵盖所有异常类型、常见场景和错误处理最佳实践。

---

## 目录

- [异常层次结构](#异常层次结构)
- [核心异常](#核心异常)
- [命令异常](#命令异常)
- [沙盒异常](#沙盒异常)
- [代码库异常](#代码库异常)
- [Session 异常](#session-异常)
- [资源异常](#资源异常)
- [处理模式](#处理模式)
- [最佳实践](#最佳实践)

---

## 异常层次结构

所有 SDK 异常都继承自 `SandboxError`:

```
SandboxError (基类)
├── ConnectionError
├── SandboxNotFoundError
├── SandboxNotRunningError
├── CommandTimeoutError
├── CommandExecutionError
├── PermissionDeniedError
├── CodebaseError
│   ├── CodebaseNotFoundError
│   ├── FileNotFoundError
│   └── UploadError
├── SessionError
│   ├── SessionNotFoundError
│   └── SessionClosedError
├── ResourceLimitExceededError
└── InvalidConfigurationError
```

**优势**:
- 捕获所有 SDK 错误:`except SandboxError`
- 捕获特定错误:`except CommandTimeoutError`
- 丰富的错误上下文(退出码、stderr、文件路径等)

---

## 核心异常

### `SandboxError`

所有 SDK 错误的**基类异常**。

```python
from agentfense import SandboxError

try:
    with Sandbox.from_local("./project") as sandbox:
        sandbox.run("command")
except SandboxError as e:
    # 捕获任何 SDK 错误
    print(f"沙盒错误: {e}")
    print(f"详情: {e.details}")
    if e.grpc_code:
        print(f"gRPC 代码: {e.grpc_code}")
```

**属性**:
- `message` (str):人类可读的错误描述
- `details` (Optional[str]):额外上下文
- `grpc_code` (Optional[int]):原始 gRPC 状态码

**使用场景**:你想捕获**任何** SDK 错误。

---

### `ConnectionError`

**无法连接**到 AgentFense 服务器。

```python
from agentfense import ConnectionError, Sandbox

try:
    sandbox = Sandbox.from_local("./project", endpoint="invalid:9999")
except ConnectionError as e:
    print(f"无法连接到 {e.endpoint}")
    print(f"详情: {e.details}")
    # 使用不同端点重试或优雅失败
```

**属性**:
- `endpoint` (str):失败的端点
- `details` (str):连接错误详情

**常见原因**:
- 服务器未运行
- 错误的端点(host:port)
- 网络问题
- 防火墙阻止连接

**解决方案**:
1. 检查服务器是否运行:`./bin/agentfense-server`
2. 验证端点:默认为 `localhost:9000`
3. 检查网络连接

---

## 命令异常

### `CommandTimeoutError`

**命令超过超时**时长。

```python
from agentfense import CommandTimeoutError

try:
    result = sandbox.run("sleep 100", timeout=10)
except CommandTimeoutError as e:
    print(f"命令在 {e.timeout_seconds}s 后超时")
    print(f"命令: {e.command}")
    # 命令可能仍在沙盒中运行
```

**属性**:
- `command` (str):超时的命令
- `timeout_seconds` (float):超时时长
- `details` (str):额外上下文

**常见原因**:
- 长时间运行的操作(构建、测试、下载)
- 用户代码中的无限循环
- 没有超时的阻塞 I/O
- 进程挂起

**解决方案**:
1. 增加超时:`sandbox.run(cmd, timeout=300)`
2. 使用流式传输查看进度:`sandbox.exec_stream(cmd)`
3. 检查代码中的无限循环
4. 对后台进程使用 session

**示例:处理超时**
```python
try:
    result = sandbox.run("pytest", timeout=60)
except CommandTimeoutError:
    # 使用更长超时重试
    result = sandbox.run("pytest", timeout=300)
```

---

### `CommandExecutionError`

**命令以非零状态退出**且 `raise_on_error=True`。

```python
from agentfense import CommandExecutionError

try:
    result = sandbox.run("make build", raise_on_error=True)
except CommandExecutionError as e:
    print(f"命令失败: {e.command}")
    print(f"退出码: {e.exit_code}")
    print(f"Stdout: {e.stdout}")
    print(f"Stderr: {e.stderr}")
    
    # 检查特定错误类型
    if "ModuleNotFoundError" in e.stderr:
        print("缺少依赖 - 运行 pip install")
    elif e.exit_code == 127:
        print("命令未找到")
```

**属性**:
- `command` (str):失败的命令
- `exit_code` (int):非零退出码
- `stdout` (str):标准输出(错误消息中截断为 500 字符)
- `stderr` (str):标准错误(错误消息中截断为 500 字符)

**常见退出码**:
- `1`:通用错误
- `2`:shell 命令误用
- `126`:命令无法执行(权限问题)
- `127`:命令未找到
- `130`:脚本被 Ctrl+C 终止
- `137`:进程被杀死(SIGKILL),通常由于 OOM

**解决方案**:
1. 检查 stderr 获取错误详情
2. 验证依赖已安装
3. 检查文件权限
4. 确保命令在 PATH 中存在
5. 检查资源限制(内存、磁盘)

**示例:解析错误类型**
```python
try:
    sandbox.run("python script.py", raise_on_error=True)
except CommandExecutionError as e:
    if "SyntaxError" in e.stderr:
        print("Python 语法错误")
    elif "ImportError" in e.stderr:
        print("缺少导入")
    elif e.exit_code == 137:
        print("内存不足 - 增加资源")
    else:
        print(f"未知错误: {e.stderr}")
```

---

### `PermissionDeniedError`

**权限规则阻止了操作**。

```python
from agentfense import PermissionDeniedError

with Sandbox.from_local(
    "./project",
    preset="read-only",
) as sandbox:
    try:
        sandbox.run("echo 'data' > /workspace/output.txt")
    except PermissionDeniedError as e:
        print(f"权限拒绝: {e.operation}")
        if e.path:
            print(f"路径: {e.path}")
        # 需要更新权限
```

**属性**:
- `operation` (str):被拒绝的操作
- `path` (Optional[str]):文件路径(如果适用)
- `details` (str):额外上下文

**常见原因**:
- 尝试写入只读路径
- 访问具有 `none` 权限的文件
- 尝试对仅查看文件进行操作

**解决方案**:
1. 检查权限规则:`sandbox.info.permissions`
2. 使用适当的预设:`agent-safe`、`development`
3. 添加写入权限:`extend_preset("read-only", additions=[...])`
4. 生产前测试权限

---

## 沙盒异常

### `SandboxNotFoundError`

**沙盒不存在或已删除**。

```python
from agentfense import SandboxNotFoundError

try:
    sandbox = Sandbox.connect("sb_invalid_id")
except SandboxNotFoundError as e:
    print(f"沙盒未找到: {e.sandbox_id}")
    # 改为创建新沙盒
```

**属性**:
- `sandbox_id` (str):缺少的沙盒 ID
- `details` (str):额外上下文

**常见原因**:
- 错误的沙盒 ID
- 沙盒已销毁
- 沙盒过期(如果设置了过期时间)

---

### `SandboxNotRunningError`

**沙盒不在 RUNNING 状态**。

```python
from agentfense import SandboxNotRunningError, SandboxStatus

try:
    result = sandbox.run("ls")
except SandboxNotRunningError as e:
    print(f"沙盒未运行(状态: {e.current_status})")
    
    # 如果已停止则启动
    if e.current_status == "stopped":
        sandbox.start()
        result = sandbox.run("ls")
```

**属性**:
- `sandbox_id` (str):沙盒 ID
- `current_status` (str):当前状态("pending"、"stopped"、"error")
- `details` (str):额外上下文

**常见原因**:
- 忘记调用 `sandbox.start()`
- 沙盒已停止
- 沙盒处于错误状态

**解决方案**:
1. 检查状态:`sandbox.status`
2. 如需启动:`sandbox.start()`
3. 刷新状态:`sandbox.refresh()`

---

## 代码库异常

### `CodebaseNotFoundError`

**代码库不存在或已删除**。

```python
from agentfense import CodebaseNotFoundError

try:
    codebase = client.get_codebase("cb_invalid")
except CodebaseNotFoundError as e:
    print(f"代码库未找到: {e.codebase_id}")
```

**属性**:
- `codebase_id` (str):缺少的代码库 ID

---

### `FileNotFoundError`

**文件在代码库中不存在**。

```python
from agentfense import FileNotFoundError

try:
    content = client.download_file("cb_123", "nonexistent.txt")
except FileNotFoundError as e:
    print(f"文件未找到: {e.file_path}")
    print(f"代码库: {e.codebase_id}")
```

**属性**:
- `codebase_id` (str):代码库 ID
- `file_path` (str):缺少的文件路径

---

### `UploadError`

**文件上传失败**。

```python
from agentfense import UploadError

try:
    client.upload_file("cb_123", "large_file.zip", huge_content)
except UploadError as e:
    print(f"上传失败: {e.file_path}")
    print(f"详情: {e.details}")
```

**属性**:
- `file_path` (str):上传失败的文件
- `codebase_id` (str):目标代码库
- `details` (str):错误详情

**常见原因**:
- 网络中断
- 文件太大
- 服务器磁盘已满
- 无效的文件路径

---

## Session 异常

### `SessionNotFoundError`

**Session 不存在或已关闭**。

```python
from agentfense import SessionNotFoundError

try:
    session = client.get_session("session_invalid")
except SessionNotFoundError as e:
    print(f"Session 未找到: {e.session_id}")
```

**属性**:
- `session_id` (str):缺少的 session ID

---

### `SessionClosedError`

**尝试使用已关闭的 session**。

```python
from agentfense import SessionClosedError

session = sandbox.session()
session.close()

try:
    session.exec("ls")  # Session 已关闭
except SessionClosedError as e:
    print(f"Session 已关闭: {e.session_id}")
    # 创建新 session
    with sandbox.session() as new_session:
        new_session.exec("ls")
```

**属性**:
- `session_id` (str):关闭的 session ID

---

## 资源异常

### `ResourceLimitExceededError`

**沙盒超过资源限制**(内存、CPU、进程)。

```python
from agentfense import ResourceLimitExceededError

try:
    result = sandbox.run("./memory_hog.sh")
except ResourceLimitExceededError as e:
    print(f"超过资源限制: {e.resource_type}")
    print(f"限制: {e.limit}")
    # 增加资源或优化代码
```

**属性**:
- `resource_type` (str):"memory"、"cpu"、"pids" 等
- `limit` (str):限制值
- `details` (str):额外上下文

**常见原因**:
- 用户代码中的内存泄漏
- 生成了太多进程
- CPU 密集型操作与严格限制

**解决方案**:
1. 增加限制:`ResourceLimits(memory_bytes=1024*1024*1024)`
2. 优化代码
3. 对大数据使用流式传输
4. 分析代码找出瓶颈

---

### `InvalidConfigurationError`

**提供给 SDK 的配置无效**。

```python
from agentfense import InvalidConfigurationError

try:
    sandbox = Sandbox.from_local(
        "./project",
        runtime=RuntimeType.DOCKER,
        # 缺少 docker 所需的 'image'
    )
except InvalidConfigurationError as e:
    print(f"无效配置: {e.message}")
```

**常见原因**:
- 缺少必需参数
- 无效的枚举值
- 冲突的选项
- 格式错误的权限规则

---

## 处理模式

### 模式 1:特定错误处理

```python
from agentfense import (
    Sandbox,
    CommandTimeoutError,
    CommandExecutionError,
    PermissionDeniedError,
)

try:
    with Sandbox.from_local("./project") as sandbox:
        result = sandbox.run("make test", timeout=60, raise_on_error=True)
except CommandTimeoutError:
    print("测试超时 - 增加超时或优化测试")
except CommandExecutionError as e:
    print(f"测试失败(退出 {e.exit_code}): {e.stderr}")
except PermissionDeniedError:
    print("权限拒绝 - 检查沙盒权限")
except Exception as e:
    print(f"意外错误: {e}")
```

---

### 模式 2:使用回退重试

```python
from agentfense import CommandTimeoutError, CommandExecutionError

def run_with_retry(sandbox, command, max_retries=3):
    """使用重试逻辑运行命令。"""
    for attempt in range(max_retries):
        try:
            return sandbox.run(command, timeout=60, raise_on_error=True)
        except CommandTimeoutError:
            if attempt < max_retries - 1:
                print(f"超时(尝试 {attempt+1}),重试...")
                continue
            raise
        except CommandExecutionError as e:
            if e.exit_code == 137:  # OOM
                print("内存不足 - 无法重试")
                raise
            if attempt < max_retries - 1:
                print(f"失败(尝试 {attempt+1}),重试...")
                continue
            raise
```

---

### 模式 3:优雅降级

```python
from agentfense import PermissionDeniedError

def save_output(sandbox, content):
    """尝试多个输出位置。"""
    locations = ["/output/result.txt", "/tmp/result.txt", "/workspace/result.txt"]
    
    for location in locations:
        try:
            sandbox.write_file(location, content)
            print(f"保存到 {location}")
            return
        except PermissionDeniedError:
            continue
    
    print("警告:无法保存输出到任何位置")
```

---

### 模式 4:详细错误日志

```python
import logging
from agentfense import CommandExecutionError, SandboxError

logger = logging.getLogger(__name__)

try:
    result = sandbox.run("python script.py", raise_on_error=True)
except CommandExecutionError as e:
    logger.error(
        "命令执行失败",
        extra={
            "command": e.command,
            "exit_code": e.exit_code,
            "stdout": e.stdout[:200],
            "stderr": e.stderr[:200],
            "sandbox_id": sandbox.id,
        }
    )
    raise
except SandboxError as e:
    logger.error(
        "沙盒错误",
        extra={
            "error_type": type(e).__name__,
            "message": e.message,
            "details": e.details,
            "sandbox_id": sandbox.id if hasattr(sandbox, 'id') else None,
        }
    )
    raise
```

---

### 模式 5:带清理的上下文管理器

```python
from agentfense import SandboxError

class SafeSandbox:
    """带保证清理的包装器。"""
    
    def __init__(self, path):
        self.path = path
        self.sandbox = None
    
    def __enter__(self):
        try:
            self.sandbox = Sandbox.from_local(self.path)
            return self.sandbox
        except Exception as e:
            print(f"创建沙盒失败: {e}")
            raise
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.sandbox:
            try:
                self.sandbox.destroy()
            except SandboxError as e:
                print(f"警告:清理失败: {e}")
        
        # 不抑制异常
        return False

# 使用
with SafeSandbox("./project") as sandbox:
    sandbox.run("test.sh")
```

---

## 最佳实践

### 1. 先捕获特定异常

```python
# ✓ 好:从最具体到最不具体
try:
    sandbox.run("command", raise_on_error=True)
except CommandTimeoutError:
    # 处理超时
    pass
except CommandExecutionError:
    # 处理执行失败
    pass
except SandboxError:
    # 处理任何其他沙盒错误
    pass

# ✗ 差:先捕获基类异常(无法到达的代码)
try:
    sandbox.run("command")
except SandboxError:  # 捕获所有
    pass
except CommandTimeoutError:  # 永远不会到达!
    pass
```

---

### 2. 对关键命令使用 `raise_on_error`

```python
# ✓ 好:对关键错误快速失败
try:
    sandbox.run("pip install -r requirements.txt", raise_on_error=True)
    sandbox.run("pytest", raise_on_error=True)
except CommandExecutionError as e:
    print(f"关键命令失败: {e.stderr}")
    # 如果设置失败不要继续

# ✗ 差:手动退出码检查(冗长)
result = sandbox.run("pip install -r requirements.txt")
if result.exit_code != 0:
    print(f"失败: {result.stderr}")
    return
```

---

### 3. 提供可操作的错误消息

```python
from agentfense import PermissionDeniedError, CommandExecutionError

try:
    sandbox.run("echo 'test' > /secrets/file.txt")
except PermissionDeniedError:
    print("""
    权限拒绝:无法写入 /secrets/
    
    修复:改用输出目录:
        sandbox.run("echo 'test' > /output/file.txt")
    
    或:更新权限以允许写入访问:
        permissions = extend_preset("agent-safe", additions=[
            {"pattern": "/secrets/**", "permission": "write"}
        ])
    """)
```

---

### 4. 不要静默捕获所有异常

```python
# ✗ 差:静默失败
try:
    sandbox.run("command")
except:
    pass  # 出了什么问题?!

# ✓ 好:记录或重新引发
try:
    sandbox.run("command")
except SandboxError as e:
    logger.error(f"沙盒错误: {e}")
    raise
```

---

### 5. 即使出错也要清理资源

```python
# ✓ 好:上下文管理器确保清理
try:
    with Sandbox.from_local("./project") as sandbox:
        sandbox.run("command")
except SandboxError:
    pass  # 沙盒仍会被清理

# ✗ 风险:手动清理(可能被跳过)
sandbox = Sandbox.from_local("./project")
try:
    sandbox.run("command")
finally:
    sandbox.destroy()  # 更好,但上下文管理器更安全
```

---

## 另请参阅

- [高层 API 指南](./high-level.md) - Sandbox API 参考
- [权限指南](./permissions.md) - 理解 PermissionDeniedError
- [最佳实践](../best-practices.md) - SDK 通用建议
