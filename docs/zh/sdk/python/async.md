# 异步 API 指南

`AsyncSandbox` 和 `AsyncSandboxClient` 类提供完全兼容 async/await 的 API,用于并发沙盒执行。本指南涵盖异步模式、性能考虑和最佳实践。

---

## 目录

- [何时使用异步](#何时使用异步)
- [AsyncSandbox 概述](#asyncsandbox-概述)
- [创建异步沙盒](#创建异步沙盒)
- [并发执行模式](#并发执行模式)
- [异步 Session](#异步-session)
- [AsyncSandboxClient(低层)](#asyncsandboxclient低层)
- [性能对比](#性能对比)
- [最佳实践](#最佳实践)
- [从同步迁移](#从同步迁移)

---

## 何时使用异步

### 使用异步的情况:

✅ **高并发**:同时运行 10+ 个沙盒  
✅ **I/O 密集型工作负载**:大部分时间用于等待网络/磁盘  
✅ **异步框架集成**:FastAPI、aiohttp、基于 asyncio 的服务  
✅ **长时间运行的命令**:想要启动多个命令并等待所有完成  
✅ **资源效率**:需要处理 100+ 并发沙盒而不产生 100+ 线程  

### 保持同步的情况:

❌ **简单脚本**:顺序执行 1-5 个沙盒  
❌ **CPU 密集型工作负载**:异步无帮助(GIL 限制 Python 并行性)  
❌ **同步框架**:Flask、Django 或同步代码库  
❌ **团队不熟悉**:如果团队不熟悉,async/await 会增加复杂性  

---

## AsyncSandbox 概述

`AsyncSandbox` 镜像同步 `Sandbox` API,但使用 async/await:

| 同步 API | 异步 API | 区别 |
|----------|-----------|------------|
| `Sandbox.from_local()` | `await AsyncSandbox.from_local()` | 可等待的类方法 |
| `sandbox.run()` | `await sandbox.run()` | 可等待的方法 |
| `with sandbox:` | `async with sandbox:` | 异步上下文管理器 |
| `sandbox.session()` | `await sandbox.session()` | 返回 AsyncSessionWrapper |
| `with sandbox.session():` | `async with session:` | 异步上下文管理器 |

**关键区别**:所有 I/O 操作都是可等待的,允许事件循环在等待时处理其他任务。

---

## 创建异步沙盒

### `AsyncSandbox.from_local()`

```python
@classmethod
async def from_local(
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
) -> "AsyncSandbox"
```

**示例**:
```python
import asyncio
from agentfense import AsyncSandbox

async def main():
    # 创建异步沙盒
    async with await AsyncSandbox.from_local("./project") as sandbox:
        result = await sandbox.run("pytest")
        print(result.stdout)

asyncio.run(main())
```

### `AsyncSandbox.from_codebase()`

```python
async with await AsyncSandbox.from_codebase(
    codebase_id="cb_xyz123",
    preset="agent-safe",
) as sandbox:
    result = await sandbox.run("make test")
```

### `AsyncSandbox.connect()`

```python
# 重新连接到现有沙盒
sandbox = await AsyncSandbox.connect("sb_abc123")
result = await sandbox.run("whoami")
await sandbox.destroy()
```

---

## 并发执行模式

### 模式 1:并行运行多个沙盒

**问题**:在 10 个不同项目中执行相同命令。

```python
import asyncio
from agentfense import AsyncSandbox

async def test_project(project_path: str) -> tuple[str, bool]:
    """在项目中运行测试并返回 (名称, 成功)。"""
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        result = await sandbox.run("pytest", timeout=300)
        return project_path, result.exit_code == 0

async def test_all_projects():
    """并发测试 10 个项目。"""
    projects = [f"./project-{i}" for i in range(10)]
    
    # 全部并行运行
    tasks = [test_project(p) for p in projects]
    results = await asyncio.gather(*tasks)
    
    # 打印结果
    for project, success in results:
        status = "✓ 通过" if success else "✗ 失败"
        print(f"{status} {project}")

asyncio.run(test_all_projects())
```

**为什么使用异步**:不是 10 * 60s = 600s 顺序执行,而是在约 60s 内运行(全部并行)。

---

### 模式 2:使用不同命令扇出

**问题**:在同一代码库上并发运行不同的分析工具。

```python
async def run_analysis(project_path: str) -> dict:
    """并行运行多个分析工具。"""
    
    async def pylint_check(sandbox):
        result = await sandbox.run("pylint .", timeout=120)
        return "pylint", result.stdout
    
    async def mypy_check(sandbox):
        result = await sandbox.run("mypy .", timeout=120)
        return "mypy", result.stdout
    
    async def pytest_coverage(sandbox):
        result = await sandbox.run("pytest --cov", timeout=300)
        return "coverage", result.stdout
    
    # 创建沙盒一次
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        # 并发运行所有检查
        tasks = [
            pylint_check(sandbox),
            mypy_check(sandbox),
            pytest_coverage(sandbox),
        ]
        results = await asyncio.gather(*tasks)
        
        return {tool: output for tool, output in results}

# 使用
results = asyncio.run(run_analysis("./my-project"))
print(results["pylint"])
```

---

### 模式 3:并发文件操作

**问题**:从沙盒并行下载多个文件。

```python
async def download_outputs(sandbox: AsyncSandbox, file_paths: list[str]) -> dict[str, str]:
    """并发下载多个文件。"""
    
    async def download_one(path: str) -> tuple[str, str]:
        content = await sandbox.read_file(path)
        return path, content
    
    tasks = [download_one(path) for path in file_paths]
    results = await asyncio.gather(*tasks)
    
    return {path: content for path, content in results}

# 使用
async with await AsyncSandbox.from_local("./project") as sandbox:
    await sandbox.run("./generate-reports.sh")
    
    files_to_download = [
        "/workspace/output/report1.txt",
        "/workspace/output/report2.txt",
        "/workspace/output/report3.txt",
    ]
    
    contents = await download_outputs(sandbox, files_to_download)
    for path, content in contents.items():
        print(f"{path}: {len(content)} 字节")
```

---

### 模式 4:有依赖的流水线

**问题**:运行阶段 N 依赖阶段 N-1 的流程。

```python
async def ci_pipeline(project_path: str) -> bool:
    """在一个沙盒内顺序运行 CI 流水线阶段。"""
    async with await AsyncSandbox.from_local(
        project_path,
        preset="development",
        runtime=RuntimeType.DOCKER,
        image="python:3.11-slim",
    ) as sandbox:
        async with await sandbox.session() as session:
            # 阶段 1:安装依赖
            result = await session.exec("pip install -r requirements.txt")
            if result.exit_code != 0:
                print("依赖安装失败")
                return False
            
            # 阶段 2:Linting(在安装后运行)
            result = await session.exec("flake8 .")
            if result.exit_code != 0:
                print("Linting 失败")
                return False
            
            # 阶段 3:测试(在 linting 后运行)
            result = await session.exec("pytest --cov")
            if result.exit_code != 0:
                print("测试失败")
                return False
            
            # 阶段 4:构建(在测试后运行)
            result = await session.exec("python setup.py bdist_wheel")
            return result.exit_code == 0

# 使用
success = asyncio.run(ci_pipeline("./my-package"))
```

**注意**:这是顺序的(一个流水线内没有并行性),但你可以**并行运行多个流水线**:

```python
async def run_pipelines():
    projects = ["./project-A", "./project-B", "./project-C"]
    tasks = [ci_pipeline(p) for p in projects]
    results = await asyncio.gather(*tasks)  # 3 个流水线并行
    return results

results = asyncio.run(run_pipelines())
```

---

### 模式 5:带取消的超时

**问题**:运行多个沙盒,但如果任何一个失败或超时则取消所有。

```python
async def test_with_timeout(project_path: str, timeout: int) -> str:
    """使用全局超时运行测试。"""
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        result = await sandbox.run("pytest", timeout=timeout)
        if result.exit_code != 0:
            raise Exception(f"{project_path} 中的测试失败")
        return f"{project_path}: 通过"

async def test_all_with_global_timeout():
    """运行所有测试,但如果任何失败或全局超时则取消。"""
    projects = [f"./project-{i}" for i in range(5)]
    
    try:
        # 5 分钟全局超时
        results = await asyncio.wait_for(
            asyncio.gather(*[test_with_timeout(p, 60) for p in projects]),
            timeout=300
        )
        print("所有测试通过:", results)
    except asyncio.TimeoutError:
        print("超过全局超时!")
    except Exception as e:
        print(f"测试失败: {e}")

asyncio.run(test_all_with_global_timeout())
```

---

## 异步 Session

### 创建异步 Session

```python
async with await AsyncSandbox.from_local("./project") as sandbox:
    async with await sandbox.session() as session:
        await session.exec("cd /workspace")
        await session.exec("npm install")
        result = await session.exec("npm test")
```

### 在单独 Session 中并行命令

**问题**:并行运行独立的有状态工作流。

```python
async def workflow_A(sandbox: AsyncSandbox):
    async with await sandbox.session() as session:
        await session.exec("cd /workspace/backend")
        await session.exec("pytest backend/")

async def workflow_B(sandbox: AsyncSandbox):
    async with await sandbox.session() as session:
        await session.exec("cd /workspace/frontend")
        await session.exec("npm test")

# 并发运行两个工作流(不同 session)
async with await AsyncSandbox.from_local("./monorepo") as sandbox:
    await asyncio.gather(
        workflow_A(sandbox),
        workflow_B(sandbox),
    )
```

**重要**:每个工作流都有自己的 session(单独的 shell 进程),因此状态不会冲突。

---

## AsyncSandboxClient(低层)

对于细粒度控制,直接使用 `AsyncSandboxClient`。

### 示例:跨异步沙盒重用代码库

```python
from agentfense import AsyncSandboxClient, RuntimeType

async def multi_sandbox_analysis(project_path: str):
    """创建一个代码库,多个沙盒。"""
    async with AsyncSandboxClient(endpoint="localhost:9000") as client:
        # 创建并上传代码库一次
        codebase = await client.create_codebase(name="shared", owner_id="team")
        
        # 上传文件(同步 walk_directory,异步上传)
        from agentfense.utils import walk_directory
        for rel_path, content in walk_directory(project_path):
            await client.upload_file(codebase.id, rel_path, content)
        
        # 从同一代码库创建多个沙盒
        sandbox1 = await client.create_sandbox(
            codebase.id,
            permissions=[{"pattern": "**/*", "permission": "read"}],
            runtime=RuntimeType.BWRAP,
        )
        
        sandbox2 = await client.create_sandbox(
            codebase.id,
            permissions=[{"pattern": "**/*", "permission": "write"}],
            runtime=RuntimeType.DOCKER,
            image="python:3.11-slim",
        )
        
        # 启动两个沙盒
        await asyncio.gather(
            client.start_sandbox(sandbox1.id),
            client.start_sandbox(sandbox2.id),
        )
        
        # 并发运行命令
        results = await asyncio.gather(
            client.exec(sandbox1.id, command="pylint ."),
            client.exec(sandbox2.id, command="pytest --cov"),
        )
        
        # 清理
        await asyncio.gather(
            client.destroy_sandbox(sandbox1.id),
            client.destroy_sandbox(sandbox2.id),
        )
        await client.delete_codebase(codebase.id)
        
        return results

results = asyncio.run(multi_sandbox_analysis("./project"))
```

---

## 性能对比

### 基准:10 个沙盒运行 10 秒命令

**同步(顺序)**:
```python
import time
from agentfense import Sandbox

start = time.time()
results = []
for i in range(10):
    with Sandbox.from_local("./project") as sandbox:
        result = sandbox.run("sleep 10")
        results.append(result)
elapsed = time.time() - start
print(f"时间: {elapsed:.1f}s")  # 约 100s (10 * 10s)
```

**异步(并行)**:
```python
import time
import asyncio
from agentfense import AsyncSandbox

async def run_one():
    async with await AsyncSandbox.from_local("./project") as sandbox:
        return await sandbox.run("sleep 10")

async def main():
    start = time.time()
    tasks = [run_one() for _ in range(10)]
    results = await asyncio.gather(*tasks)
    elapsed = time.time() - start
    print(f"时间: {elapsed:.1f}s")  # 约 10s (全部并行)
    return results

asyncio.run(main())
```

**加速**:约 10 倍(10 个沙盒并发运行而非顺序)。

---

### 可扩展性:100 并发沙盒

异步可以处理 100+ 并发沙盒,内存开销最小:

```python
async def run_100_sandboxes():
    """并发运行 100 个沙盒。"""
    async def run_one(i: int):
        async with await AsyncSandbox.from_local(f"./project-{i}") as sandbox:
            return await sandbox.run("echo 'done'")
    
    tasks = [run_one(i) for i in range(100)]
    results = await asyncio.gather(*tasks)
    print(f"完成 {len(results)} 个沙盒")

asyncio.run(run_100_sandboxes())
```

**内存使用**:约 50-100 MB(异步开销与每个沙盒一个线程相比很小)。

---

## 最佳实践

### 1. 使用 `asyncio.gather()` 并行执行

```python
# ✓ 好:并行运行
tasks = [sandbox.run(cmd) for cmd in commands]
results = await asyncio.gather(*tasks)

# ✗ 差:顺序(违背异步目的)
results = []
for cmd in commands:
    result = await sandbox.run(cmd)
    results.append(result)
```

---

### 2. 在 `gather()` 中处理异常

```python
# ✓ 好:为每个任务捕获异常
tasks = [run_test(project) for project in projects]
results = await asyncio.gather(*tasks, return_exceptions=True)

for project, result in zip(projects, results):
    if isinstance(result, Exception):
        print(f"{project} 失败: {result}")
    else:
        print(f"{project} 成功")

# ✗ 差:一个失败取消所有
try:
    results = await asyncio.gather(*tasks)  # 第一个异常停止所有
except Exception as e:
    print(f"失败: {e}")  # 不知道哪个任务失败
```

---

### 3. 使用信号量限制并发

```python
async def run_with_limit(projects: list[str], max_concurrent: int = 5):
    """运行沙盒,最大并发限制。"""
    semaphore = asyncio.Semaphore(max_concurrent)
    
    async def run_one(project: str):
        async with semaphore:  # 一次只有 5 个
            async with await AsyncSandbox.from_local(project) as sandbox:
                return await sandbox.run("make test")
    
    tasks = [run_one(p) for p in projects]
    return await asyncio.gather(*tasks)

# 运行 100 个项目,但只有 5 个并发沙盒
results = asyncio.run(run_with_limit([f"./p-{i}" for i in range(100)], max_concurrent=5))
```

---

### 4. 正确关闭 Client

```python
# ✓ 好:使用上下文管理器
async with AsyncSandboxClient(endpoint="localhost:9000") as client:
    # ... 使用 client ...
# 自动关闭

# ✗ 差:手动关闭(容易忘记)
client = AsyncSandboxClient(endpoint="localhost:9000")
# ... 使用 client ...
await client.close()  # 如果发生异常容易忘记
```

---

### 5. 避免混合同步和异步

```python
# ✗ 差:异步函数中的阻塞调用
async def bad_example():
    async with await AsyncSandbox.from_local("./project") as sandbox:
        result = await sandbox.run("test.sh")
        
        # 异步上下文中的阻塞 I/O!
        with open("output.txt", "w") as f:  # 阻塞事件循环
            f.write(result.stdout)

# ✓ 好:使用异步文件 I/O
import aiofiles

async def good_example():
    async with await AsyncSandbox.from_local("./project") as sandbox:
        result = await sandbox.run("test.sh")
        
        async with aiofiles.open("output.txt", "w") as f:
            await f.write(result.stdout)
```

---

## 从同步迁移

将同步代码转换为异步很简单:

### 步骤 1:更改导入

```python
# 同步
from agentfense import Sandbox

# 异步
from agentfense import AsyncSandbox
```

---

### 步骤 2:添加 `async`/`await`

```python
# 同步
def run_tests(project_path: str):
    with Sandbox.from_local(project_path) as sandbox:
        result = sandbox.run("pytest")
        return result.exit_code == 0

# 异步
async def run_tests(project_path: str):
    async with await AsyncSandbox.from_local(project_path) as sandbox:
        result = await sandbox.run("pytest")
        return result.exit_code == 0
```

---

### 步骤 3:更新上下文管理器

```python
# 同步
with sandbox.session() as session:
    session.exec("cd /workspace")

# 异步
async with await sandbox.session() as session:
    await session.exec("cd /workspace")
```

---

### 步骤 4:在入口点使用 `asyncio.run()`

```python
# 同步
if __name__ == "__main__":
    success = run_tests("./project")
    print(success)

# 异步
import asyncio

if __name__ == "__main__":
    success = asyncio.run(run_tests("./project"))
    print(success)
```

---

## 另请参阅

- [高层 API 指南](./high-level.md) - 同步 API 参考
- [Sessions 指南](./sessions.md) - 有状态执行
- [Python asyncio 文档](https://docs.python.org/zh-cn/3/library/asyncio.html) - 官方 asyncio 指南
