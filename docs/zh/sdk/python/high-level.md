# 高层 API 指南

`Sandbox` 类提供了最简单的方式来创建和管理沙盒,并自动清理资源。本指南涵盖所有方法、参数和使用模式。

---

## 目录

- [创建沙盒](#创建沙盒)
  - [from_local()](#from_local)
  - [from_codebase()](#from_codebase)
  - [connect()](#connect)
- [生命周期管理](#生命周期管理)
- [执行命令](#执行命令)
- [使用 Session](#使用-session)
- [文件操作](#文件操作)
- [属性](#属性)
- [完整示例](#完整示例)

---

## 创建沙盒

### `Sandbox.from_local()`

从本地目录创建沙盒。这是大多数用例的**推荐**方法。

#### 方法签名

```python
@classmethod
def from_local(
    cls,
    path: str,
    preset: Optional[str] = "view-only",
    permissions: Optional[List[Union[PermissionRule, Dict]]] = None,
    runtime: RuntimeType = RuntimeType.BWRAP,
    image: Optional[str] = None,
    resources: Optional[ResourceLimits] = None,
    endpoint: str = "localhost:9000",
    secure: bool = False,
    owner_id: Optional[str] = None,
    codebase_name: Optional[str] = None,
    ignore_patterns: Optional[List[str]] = None,
    labels: Optional[Dict[str, str]] = None,
    auto_start: bool = True,
) -> "Sandbox"
```

#### 参数

| 参数 | 类型 | 默认值 | 描述 |
|-----------|------|---------|-------------|
| `path` | `str` | **必需** | 包含文件的本地目录路径 |
| `preset` | `str` | `"view-only"` | 权限预设名称(`"view-only"`, `"read-only"`, `"agent-safe"`, `"development"`, `"full-access"`) |
| `permissions` | `List[PermissionRule \| Dict]` | `None` | 额外的权限规则(与预设合并) |
| `runtime` | `RuntimeType` | `RuntimeType.BWRAP` | 运行时类型(`BWRAP` 或 `DOCKER`) |
| `image` | `str` | `None` | Docker 镜像名称(Docker 运行时必需) |
| `resources` | `ResourceLimits` | `None` | 资源限制(内存、CPU、PIDs) |
| `endpoint` | `str` | `"localhost:9000"` | AgentFense 服务器 gRPC 端点 |
| `secure` | `bool` | `False` | 连接使用 TLS |
| `owner_id` | `str` | 自动生成 | 代码库的所有者标识符 |
| `codebase_name` | `str` | 从路径派生 | 代码库名称 |
| `ignore_patterns` | `List[str]` | `None` | 上传时跳过的额外 gitignore 风格模式 |
| `labels` | `Dict[str, str]` | `None` | 用于组织/过滤的键值标签 |
| `auto_start` | `bool` | `True` | 是否自动启动沙盒 |

#### 返回值

可用的 `Sandbox` 实例(如果 `auto_start=True` 则为运行状态)。

#### 异常

- `ValueError`:如果 `path` 不存在或不是目录
- `SandboxError`:如果沙盒创建或启动失败
- `ConnectionError`:如果无法连接到 AgentFense 服务器

#### 执行过程

1. **验证路径**:检查目录是否存在
2. **创建代码库**:在服务器上分配存储
3. **上传文件**:递归遍历目录并上传所有文件(遵守 `.gitignore`、`.sandboxignore` 和 `ignore_patterns`)
4. **合并权限**:将预设与自定义 `permissions` 组合
5. **创建沙盒**:配置运行时和权限规则
6. **启动沙盒**(如果 `auto_start=True`):挂载 FUSE 文件系统并启动隔离
7. **返回**:`Sandbox` 实例(使用上下文管理器自动清理)

#### 基本示例

```python
from agentfense import Sandbox

# 最简单的用法:上传当前目录,仅查看访问
with Sandbox.from_local(".") as sandbox:
    result = sandbox.run("ls -la")
    print(result.stdout)
```

#### 示例:自定义权限

```python
from agentfense import Sandbox, PermissionRule, Permission

with Sandbox.from_local(
    "./my-project",
    preset="read-only",  # 从只读基础开始
    permissions=[
        # 添加对输出目录的写入访问
        PermissionRule(pattern="/output/**", permission=Permission.WRITE),
        # 屏蔽对机密的访问
        PermissionRule(pattern="/secrets/**", permission=Permission.NONE),
    ]
) as sandbox:
    result = sandbox.run("python process.py")
```

#### 示例:Docker 运行时和资源限制

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

with Sandbox.from_local(
    "./backend",
    preset="agent-safe",
    runtime=RuntimeType.DOCKER,
    image="python:3.11-slim",
    resources=ResourceLimits(
        memory_bytes=512 * 1024 * 1024,  # 512 MB
        cpu_quota=50000,  # 50% 的一个 CPU 核心
        pids_limit=100,   # 最多 100 个进程
    ),
    labels={
        "team": "backend",
        "env": "testing",
    }
) as sandbox:
    # 沙盒在 Docker 容器中隔离,强制执行限制
    result = sandbox.run("pytest", timeout=300)
    print(f"测试在 {result.duration} 内完成")
```

#### 示例:忽略模式

```python
# 上传时跳过大文件和构建产物
with Sandbox.from_local(
    "./project",
    ignore_patterns=[
        "*.log",
        "dist/**",
        "build/**",
        "*.mp4",
        "*.zip",
    ]
) as sandbox:
    result = sandbox.run("make test")
```

#### 重要说明

- **自动清理**:上下文管理器(`with` 语句)在退出时自动销毁沙盒并删除代码库
- **默认预设**:如果 `preset=None`,默认为 `"view-only"`(最安全选项)
- **文件大小限制**:默认跳过大于 50 MB 的文件(在 `walk_directory()` 中配置)
- **Gitignore 支持**:自动遵守根目录中的 `.gitignore` 和 `.sandboxignore`
- **Owner ID**:如果未提供,基于当前用户自动生成

---

### `Sandbox.from_codebase()`

从**现有**代码库创建沙盒。当你想在多个沙盒中使用不同权限重用相同文件时使用此方法。

#### 方法签名

```python
@classmethod
def from_codebase(
    cls,
    codebase_id: str,
    preset: Optional[str] = "view-only",
    permissions: Optional[List[Union[PermissionRule, Dict]]] = None,
    runtime: RuntimeType = RuntimeType.BWRAP,
    image: Optional[str] = None,
    resources: Optional[ResourceLimits] = None,
    endpoint: str = "localhost:9000",
    secure: bool = False,
    labels: Optional[Dict[str, str]] = None,
    auto_start: bool = True,
) -> "Sandbox"
```

#### 参数

与 `from_local()` 相同,除了:
- **`codebase_id`**(必需):要使用的现有代码库的 ID
- 没有 `path`、`owner_id`、`codebase_name`、`ignore_patterns`(代码库已存在)

#### 返回值

`Sandbox` 实例(**不**拥有代码库,清理时不会删除它)。

#### 用例

- **从相同代码创建多个沙盒**:在相同文件上测试不同的权限集
- **共享仓库**:多个用户访问同一代码库
- **性能**:跳过重复运行的文件重新上传

#### 示例:两个具有不同权限的沙盒

```python
from agentfense import Sandbox, SandboxClient

# 首先,手动创建代码库
client = SandboxClient()
codebase = client.create_codebase(name="shared-repo", owner_id="team_A")

# 上传文件一次
for file_path, content in walk_directory("./project"):
    client.upload_file(codebase.id, file_path, content)

# 创建只读沙盒
with Sandbox.from_codebase(
    codebase_id=codebase.id,
    preset="read-only",
    labels={"purpose": "analysis"},
) as sandbox1:
    result1 = sandbox1.run("pylint .")

# 创建可写沙盒(单独执行)
with Sandbox.from_codebase(
    codebase_id=codebase.id,
    preset="development",
    labels={"purpose": "testing"},
) as sandbox2:
    result2 = sandbox2.run("pytest --cov")

# 两个沙盒销毁后代码库仍然存在
client.delete_codebase(codebase.id)  # 手动清理
```

---

### `Sandbox.connect()`

连接到**现有的、正在运行的**沙盒。用于在断开连接后重新连接或访问由另一个进程创建的沙盒。

#### 方法签名

```python
@classmethod
def connect(
    cls,
    sandbox_id: str,
    endpoint: str = "localhost:9000",
    secure: bool = False,
) -> "Sandbox"
```

#### 参数

| 参数 | 类型 | 默认值 | 描述 |
|-----------|------|---------|-------------|
| `sandbox_id` | `str` | **必需** | 现有沙盒的 ID |
| `endpoint` | `str` | `"localhost:9000"` | AgentFense 服务器端点 |
| `secure` | `bool` | `False` | 连接使用 TLS |

#### 返回值

连接到现有沙盒的 `Sandbox` 实例(**不**拥有沙盒,清理时不会销毁它)。

#### 示例:网络问题后重新连接

```python
from agentfense import Sandbox

# 原始创建
sandbox = Sandbox.from_local("./project", auto_start=True)
sandbox_id = sandbox.id
print(f"沙盒已创建: {sandbox_id}")

# ... 网络中断或进程重启 ...

# 重新连接到同一沙盒
sandbox = Sandbox.connect(sandbox_id)
result = sandbox.run("whoami")
print(result.stdout)

# 手动清理(因为我们没有创建它)
sandbox.destroy()
```

#### 示例:访问由另一个进程创建的沙盒

```python
# 进程 A 创建长期存在的沙盒
sandbox = Sandbox.from_local("./data", auto_start=True)
print(f"沙盒 ID: {sandbox.id}")  # 共享此 ID

# 进程 B 连接到同一沙盒
sandbox_id = "sb_abc123"  # 来自进程 A
sandbox = Sandbox.connect(sandbox_id)
result = sandbox.run("cat /workspace/data.csv")
```

---

## 生命周期管理

### `start()`

启动处于 `PENDING` 或 `STOPPED` 状态的沙盒。

```python
def start(self) -> "Sandbox"
```

**返回值**:Self(用于方法链)

**示例**:
```python
sandbox = Sandbox.from_local("./project", auto_start=False)
print(f"状态: {sandbox.status}")  # PENDING

sandbox.start()
print(f"状态: {sandbox.status}")  # RUNNING

result = sandbox.run("python main.py")
```

---

### `stop()`

停止正在运行的沙盒(但不销毁它)。你可以稍后使用 `start()` 重新启动它。

```python
def stop(self) -> "Sandbox"
```

**返回值**:Self(用于方法链)

**示例**:
```python
with Sandbox.from_local("./project") as sandbox:
    sandbox.run("setup.sh")
    
    # 暂停执行
    sandbox.stop()
    print("沙盒已停止,但未销毁")
    
    # 恢复
    sandbox.start()
    sandbox.run("main.sh")
```

---

### `refresh()`

从服务器刷新沙盒元数据(更新 `status`、`started_at` 等)。

```python
def refresh(self) -> "Sandbox"
```

**返回值**:Self(用于方法链)

**示例**:
```python
sandbox = Sandbox.connect("sb_abc123")
print(f"状态: {sandbox.status}")  # 缓存值

sandbox.refresh()
print(f"状态: {sandbox.status}")  # 从服务器获取的新值
```

---

### `destroy()`

销毁沙盒并可选地删除代码库。**不可逆**。

```python
def destroy(self, delete_codebase: Optional[bool] = None) -> None
```

**参数**:
- `delete_codebase`:如果为 `True`,删除代码库。如果为 `False`,保留它。如果为 `None`,仅在此 `Sandbox` 创建了它时删除(默认)。

**示例**:
```python
# 自动清理(推荐)
with Sandbox.from_local("./project") as sandbox:
    sandbox.run("test.sh")
# 沙盒 + 代码库在此处自动销毁

# 手动清理
sandbox = Sandbox.from_local("./project")
try:
    sandbox.run("test.sh")
finally:
    sandbox.destroy()  # 显式清理

# 保留代码库以供重用
sandbox = Sandbox.from_local("./project")
sandbox.run("test.sh")
sandbox.destroy(delete_codebase=False)  # 沙盒销毁,代码库保留
```

---

## 执行命令

### `run()`

使用简化的参数和合理的默认值执行命令。大多数用例的**推荐**方法。

#### 方法签名

```python
def run(
    self,
    command: str,
    timeout: int = 60,
    env: Optional[Dict[str, str]] = None,
    workdir: Optional[str] = None,
    raise_on_error: bool = False,
) -> ExecResult
```

#### 参数

| 参数 | 类型 | 默认值 | 描述 |
|-----------|------|---------|-------------|
| `command` | `str` | **必需** | 要执行的 shell 命令 |
| `timeout` | `int` | `60` | 超时时间(以**秒**为单位,不是 timedelta) |
| `env` | `Dict[str, str]` | `None` | 要设置的环境变量 |
| `workdir` | `str` | `None` | 工作目录(默认:`/workspace`) |
| `raise_on_error` | `bool` | `False` | 如果退出码 != 0 则引发异常 |

#### 返回值

`ExecResult`,包含:
- `stdout`: str
- `stderr`: str
- `exit_code`: int
- `duration`: timedelta
- `command`: str(用于调试)

#### 示例:基本执行

```python
result = sandbox.run("python --version")
print(f"输出: {result.stdout}")
print(f"退出码: {result.exit_code}")
print(f"持续时间: {result.duration}")
```

#### 示例:使用环境变量

```python
result = sandbox.run(
    "python app.py",
    env={
        "DEBUG": "1",
        "API_KEY": "test-key",
    },
    timeout=120,
)
```

#### 示例:错误处理

```python
from agentfense.exceptions import CommandExecutionError

# 选项 1:手动检查退出码
result = sandbox.run("make build")
if result.exit_code != 0:
    print(f"构建失败: {result.stderr}")

# 选项 2:失败时引发异常
try:
    result = sandbox.run("make build", raise_on_error=True)
except CommandExecutionError as e:
    print(f"构建失败,退出码 {e.exit_code}")
    print(f"错误: {e.stderr}")
```

---

### `exec()`

以完全控制执行命令(支持 stdin、timedelta 超时)。需要高级功能时使用。

#### 方法签名

```python
def exec(
    self,
    command: str,
    stdin: Optional[str] = None,
    env: Optional[Dict[str, str]] = None,
    workdir: Optional[str] = None,
    timeout: Optional[timedelta] = None,
) -> ExecResult
```

#### 参数

与 `run()` 相同,除了:
- **`stdin`**:传递给命令的可选输入
- **`timeout`**:`timedelta` 对象(不是 int 秒)

#### 示例:使用 Stdin

```python
from datetime import timedelta

result = sandbox.exec(
    command="python analyze.py",
    stdin="line1\nline2\nline3",
    timeout=timedelta(minutes=5),
)
```

---

### `exec_stream()`

执行命令并实时流式传输输出。用于你想要实时更新的长时间运行命令。

#### 方法签名

```python
def exec_stream(
    self,
    command: str,
    stdin: Optional[str] = None,
    env: Optional[Dict[str, str]] = None,
    workdir: Optional[str] = None,
    timeout: Optional[timedelta] = None,
) -> Iterator[bytes]
```

#### 返回值

生成输出块(bytes)的迭代器。

#### 示例:实时输出

```python
for chunk in sandbox.exec_stream("./long-running-task.sh"):
    print(chunk.decode(), end="", flush=True)
```

#### 示例:保存到文件

```python
with open("output.log", "wb") as f:
    for chunk in sandbox.exec_stream("pytest --verbose"):
        f.write(chunk)
```

---

## 使用 Session

详细信息请参阅 [Sessions 指南](./sessions.md)。

### `session()`

创建有状态的 shell session。在 session 中执行的命令保留 `cwd`、环境变量和后台进程。

```python
def session(
    self,
    shell: str = "/bin/sh",
    env: Optional[Dict[str, str]] = None,
) -> SessionWrapper
```

**示例**:
```python
with sandbox.session() as session:
    session.exec("cd /workspace/backend")
    session.exec("source venv/bin/activate")
    session.exec("export DEBUG=1")
    result = session.exec("python manage.py test")
```

---

## 文件操作

### `read_file()`

从沙盒读取文件作为 UTF-8 字符串。

```python
def read_file(self, path: str) -> str
```

**示例**:
```python
content = sandbox.read_file("/workspace/output.txt")
print(content)
```

---

### `read_file_bytes()`

读取文件作为原始字节(用于二进制文件)。

```python
def read_file_bytes(self, path: str) -> bytes
```

**示例**:
```python
image_data = sandbox.read_file_bytes("/workspace/chart.png")
with open("local_chart.png", "wb") as f:
    f.write(image_data)
```

---

### `write_file()`

将文件写入沙盒。

```python
def write_file(self, path: str, content: Union[str, bytes]) -> None
```

**示例**:
```python
# 写入文本文件
sandbox.write_file("/workspace/config.json", '{"debug": true}')

# 写入二进制文件
sandbox.write_file("/workspace/data.bin", b"\x00\x01\x02\x03")
```

---

### `list_files()`

列出目录中的文件。

```python
def list_files(self, path: str = "", recursive: bool = False) -> List[str]
```

**示例**:
```python
# 列出根目录
files = sandbox.list_files()
print(files)  # ['main.py', 'README.md', 'src/']

# 递归列出
all_files = sandbox.list_files(recursive=True)
print(all_files)  # ['main.py', 'README.md', 'src/utils.py', 'src/tests/test_utils.py']
```

---

## 属性

### `id`

```python
@property
def id(self) -> str
```

唯一沙盒标识符。

```python
print(f"沙盒 ID: {sandbox.id}")  # sb_abc123def456
```

---

### `codebase_id`

```python
@property
def codebase_id(self) -> str
```

关联代码库的 ID。

```python
print(f"代码库 ID: {sandbox.codebase_id}")  # cb_xyz789
```

---

### `status`

```python
@property
def status(self) -> SandboxStatus
```

当前状态:`PENDING`、`RUNNING`、`STOPPED` 或 `ERROR`。

```python
from agentfense import SandboxStatus

if sandbox.status == SandboxStatus.RUNNING:
    sandbox.run("echo 'ready'")
```

---

### `runtime`

```python
@property
def runtime(self) -> RuntimeType
```

运行时类型(`BWRAP` 或 `DOCKER`)。

```python
print(f"使用运行时: {sandbox.runtime.value}")  # bwrap
```

---

### `info`

```python
@property
def info(self) -> SandboxInfo
```

完整的沙盒元数据(包括时间戳、标签、资源等)。

```python
print(f"创建于: {sandbox.info.created_at}")
print(f"标签: {sandbox.info.labels}")
```

---

### `codebase`

```python
@property
def codebase(self) -> Codebase
```

完整的代码库元数据。

```python
print(f"代码库名称: {sandbox.codebase.name}")
print(f"文件数: {sandbox.codebase.file_count}")
print(f"总大小: {sandbox.codebase.size} 字节")
```

---

## 完整示例

### 示例 1:CI/CD 测试运行器

```python
from agentfense import Sandbox, RuntimeType, ResourceLimits

def run_ci_tests(project_path: str) -> bool:
    """在具有资源限制的隔离沙盒中运行 CI 测试。"""
    with Sandbox.from_local(
        project_path,
        preset="development",
        runtime=RuntimeType.DOCKER,
        image="python:3.11-slim",
        resources=ResourceLimits(
            memory_bytes=1024 * 1024 * 1024,  # 1 GB
            pids_limit=200,
        ),
        labels={"ci": "true", "env": "test"},
    ) as sandbox:
        # 安装依赖
        install_result = sandbox.run("pip install -r requirements.txt", timeout=300)
        if install_result.exit_code != 0:
            print(f"依赖安装失败: {install_result.stderr}")
            return False
        
        # 运行测试
        test_result = sandbox.run("pytest --cov --cov-report=xml", timeout=600)
        
        # 下载覆盖率报告
        if test_result.exit_code == 0:
            coverage_xml = sandbox.read_file("/workspace/coverage.xml")
            with open("coverage.xml", "w") as f:
                f.write(coverage_xml)
        
        return test_result.exit_code == 0

# 使用
success = run_ci_tests("./my-project")
print(f"测试 {'通过' if success else '失败'}")
```

### 示例 2:AI Agent 与输出收集

```python
from agentfense import Sandbox, PermissionRule, Permission

def run_agent_task(code_path: str, task: str) -> dict:
    """运行 AI Agent 任务并收集结果。"""
    with Sandbox.from_local(
        code_path,
        preset="agent-safe",  # 屏蔽机密,允许读取 + 写入到 /output
        permissions=[
            PermissionRule(pattern="/logs/**", permission=Permission.WRITE),
        ],
    ) as sandbox:
        # 运行 agent
        with sandbox.session() as session:
            session.exec("cd /workspace")
            session.exec(f"export TASK='{task}'")
            result = session.exec("python agent.py", timeout=300)
        
        # 收集输出
        outputs = {}
        for file_path in sandbox.list_files("/output", recursive=True):
            full_path = f"/workspace/output/{file_path}"
            outputs[file_path] = sandbox.read_file(full_path)
        
        # 收集日志
        logs = []
        for log_file in sandbox.list_files("/logs", recursive=True):
            full_path = f"/workspace/logs/{log_file}"
            logs.append(sandbox.read_file(full_path))
        
        return {
            "exit_code": result.exit_code,
            "stdout": result.stdout,
            "stderr": result.stderr,
            "duration": result.duration,
            "outputs": outputs,
            "logs": logs,
        }

# 使用
results = run_agent_task("./agent-code", "analyze repository")
print(f"Agent 在 {results['duration']} 内完成")
print(f"生成了 {len(results['outputs'])} 个输出文件")
```

### 示例 3:多阶段构建过程

```python
from agentfense import Sandbox

def multi_stage_build(source_path: str):
    """运行保留状态的多阶段构建。"""
    with Sandbox.from_local(
        source_path,
        preset="development",
        runtime=RuntimeType.DOCKER,
        image="node:18",
    ) as sandbox:
        with sandbox.session() as session:
            # 阶段 1:安装依赖
            print("安装依赖...")
            result = session.exec("npm install")
            if result.exit_code != 0:
                raise Exception(f"安装失败: {result.stderr}")
            
            # 阶段 2:运行 linter
            print("运行 linter...")
            session.exec("npm run lint")
            
            # 阶段 3:运行测试
            print("运行测试...")
            test_result = session.exec("npm test")
            
            # 阶段 4:构建生产包
            print("构建中...")
            session.exec("npm run build")
            
            # 阶段 5:收集产物
            print("收集产物...")
            build_files = sandbox.list_files("/workspace/dist", recursive=True)
            
            for file_path in build_files:
                content = sandbox.read_file_bytes(f"/workspace/dist/{file_path}")
                local_path = f"./build/{file_path}"
                os.makedirs(os.path.dirname(local_path), exist_ok=True)
                with open(local_path, "wb") as f:
                    f.write(content)
            
            print(f"构建完成! 生成了 {len(build_files)} 个文件。")
            return test_result.exit_code == 0

# 使用
success = multi_stage_build("./frontend")
```

---

## 最佳实践

1. **始终使用上下文管理器**(`with` 语句)以确保清理
2. **使用预设**作为基础,用 `permissions` 扩展以满足自定义需求
3. **设置适当的超时**以防止命令挂起
4. **使用 session**进行依赖状态的多步工作流
5. **检查退出码**或对关键命令使用 `raise_on_error=True`
6. **标记你的沙盒**以便更轻松地调试和过滤
7. **重用代码库**当使用不同配置运行相同代码时
8. **监控资源使用**在生产环境中(设置 `resources` 参数)

---

## 另请参阅

- [异步 API 指南](./async.md) - 并发沙盒执行
- [Sessions 指南](./sessions.md) - 有状态命令执行
- [权限指南](./permissions.md) - 细粒度访问控制
- [错误处理指南](./errors.md) - 异常处理模式
