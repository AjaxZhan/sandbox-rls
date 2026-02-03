# 性能

性能特征、基准测试和优化技巧。

## 基准测试

### 系统规格

所有基准测试运行环境：
- **CPU：** 2 vCPU（Intel Xeon）
- **RAM：** 4 GB
- **磁盘：** SSD
- **操作系统：** Ubuntu 22.04 LTS
- **运行时：** Docker

### 并发性

| 指标 | 值 |
|--------|-------|
| 最大并发沙盒 | 120+（已测试） |
| 推荐最大值（2 核/4GB） | 50-80 |
| 每个沙盒内存 | ~5 MB |
| 每个沙盒进程 | 2（FUSE + 运行时） |

### 延迟

| 操作 | 延迟（p50） | 延迟（p99） |
|-----------|---------------|---------------|
| 创建沙盒 | 150 ms | 300 ms |
| 启动沙盒（bwrap） | 80 ms | 150 ms |
| 启动沙盒（Docker） | 250 ms | 500 ms |
| 执行命令 | 20 ms | 50 ms |
| 文件读取（1KB） | 0.5 ms | 2 ms |
| 文件读取（1MB） | 15 ms | 30 ms |

### 吞吐量

| 操作 | 吞吐量 |
|-----------|------------|
| 命令执行/秒 | 1000+ |
| 文件读取/秒（小文件） | 5000+ |
| 文件写入/秒（小文件） | 3000+ |

### FUSE 开销

与原生文件系统相比：

| 操作 | 开销 |
|-----------|----------|
| 读取（小文件） | +3% |
| 读取（大文件） | +2% |
| 写入（小文件） | +5% |
| 写入（大文件） | +4% |
| 元数据（stat、readdir） | +2% |

## 优化技巧

### 1. 选择合适的运行时

```python
# 快速启动，低开销（仅 Linux）
runtime=RuntimeType.BWRAP

# 完全隔离，跨平台
runtime=RuntimeType.DOCKER
```

**建议：** 开发和高吞吐量场景使用 bwrap。生产隔离使用 Docker。

### 2. 重用代码库

```python
# 不好：每次都创建新代码库
for i in range(100):
    with Sandbox.from_local("./project") as sb:
        sb.run("test")

# 好：创建一次代码库，重用
client = SandboxClient()
codebase = client.create_codebase("project", "user")
# ... 上传文件一次 ...

for i in range(100):
    sandbox = client.create_sandbox(codebase.id, ...)
    # ... 使用沙盒 ...
    client.destroy_sandbox(sandbox.id)
```

### 3. 批量操作

```python
# 不好：多次往返
sandbox.exec("mkdir /workspace/output")
sandbox.exec("touch /workspace/output/file1.txt")
sandbox.exec("touch /workspace/output/file2.txt")

# 好：单个命令
sandbox.exec("mkdir -p /workspace/output && touch /workspace/output/file{1,2}.txt")

# 更好：使用会话
with sandbox.session() as session:
    session.exec("mkdir -p /workspace/output")
    session.exec("cd /workspace/output")
    session.exec("touch file1.txt file2.txt")
```

### 4. 限制资源使用

```python
from agentfense import ResourceLimits

sandbox = Sandbox.from_local(
    "./project",
    resources=ResourceLimits(
        memory_bytes=256 * 1024 * 1024,  # 256 MB
        cpu_quota=50000,  # 一个核心的 50%
        pids_limit=100,
    )
)
```

### 5. 正确清理

```python
# 完成后始终销毁沙盒
sandbox.destroy(delete_codebase=True)

# 或使用上下文管理器（自动清理）
with Sandbox.from_local("./project") as sandbox:
    # ... 使用沙盒 ...
    pass  # 自动清理
```

### 6. 使用异步实现并发

```python
import asyncio
from agentfense import AsyncSandbox

async def run_tests():
    # 并发运行 10 个沙盒
    tasks = []
    for i in range(10):
        async with await AsyncSandbox.from_local("./project") as sb:
            task = sb.run("pytest")
            tasks.append(task)
    
    results = await asyncio.gather(*tasks)
    return results

asyncio.run(run_tests())
```

## 容量规划

### 单服务器建议

| 服务器规格 | 并发沙盒 | 备注 |
|-------------|----------------------|-------|
| 2 核，4 GB | 50-80 | 入门级 |
| 4 核，8 GB | 150-200 | 推荐 |
| 8 核，16 GB | 400+ | 高容量 |

### 多服务器设置

对于 > 500 并发沙盒：
1. 部署多个 AgentFense 服务器
2. 使用负载均衡器（轮询或最少连接）
3. 共享代码库存储（NFS、S3 等）

## 监控

要跟踪的关键指标：

```python
# 服务器指标
- 活动沙盒数量
- FUSE 挂载数量
- 内存使用
- CPU 使用
- 磁盘 I/O

# 应用指标
- 沙盒创建速率
- 命令执行延迟
- 文件操作延迟
- 错误率
```

## 已知瓶颈

1. **内存：** 每个沙盒使用约 5 MB。高并发需要更多 RAM。
2. **FUSE 挂载：** 每个沙盒需要一个挂载。Linux 有限制（约 10 万个挂载）。
3. **文件描述符：** 如果看到"打开文件过多"，增加 `ulimit -n`。
4. **Delta Layer：** 大型 delta 会减慢读取速度。定期导出快照。

## 未来改进

计划中的性能增强：
- 沙盒池化（预热沙盒）
- 共享 FUSE 挂载（减少每个沙盒的开销）
- 批量 API（一次调用创建多个沙盒）
- 缓存层（减少重复文件读取）
