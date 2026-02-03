# 常见问题

常见问题的解决方案。

## 安装问题

### 未安装 FUSE

**错误：** `fusermount: command not found`

**解决方案：**
```bash
# Ubuntu/Debian
sudo apt-get install fuse libfuse-dev

# CentOS/RHEL
sudo yum install fuse fuse-libs

# macOS
brew install macfuse
```

### 挂载时权限被拒绝

**错误：** `fuse: failed to open /dev/fuse: Permission denied`

**解决方案：**
```bash
# 将用户添加到 fuse 组
sudo usermod -a -G fuse $USER

# 重新加载组（或注销/登录）
newgrp fuse
```

## 运行时问题

### macOS Docker Desktop：view 权限不工作

**问题：** 在 macOS 的容器内，具有 `view` 权限的文件显示为"没有此文件"。

**原因：** Docker Desktop for Mac 中的 VirtioFS 限制。

**解决方法：**
- 使用 `read` 权限代替 `view`
- 使用 bwrap 运行时（需要 Linux）
- 使用原生 Linux 或 Linux 虚拟机

### 沙盒卡在 PENDING 状态

**症状：** `sandbox.start()` 挂起或超时

**可能原因：**
1. FUSE 挂载超时
2. Docker 镜像正在拉取
3. 资源耗尽

**解决方案：**
```python
# 检查日志
import logging
logging.basicConfig(level=logging.DEBUG)

# 增加超时（默认 30 秒）
sandbox = Sandbox.from_local(
    "./project",
    start_timeout=60  # 60 秒
)

# 检查服务器日志
docker logs agentfense-server
```

### 权限被拒绝错误

**错误：** 访问文件时 `Permission denied`

**诊断：**
```python
# 列出可见内容
files = sandbox.list_files(recursive=True)
print(files)

# 检查有效权限
# 具有 'none' 的文件根本不会出现
# 具有 'view' 的文件会出现但无法读取
```

**解决方案：**
1. 验证权限规则是否符合您的意图
2. 检查模式特定性（更具体的优先）
3. 使用 `full-access` 预设测试是否是权限问题

## 性能问题

### 高内存使用

**症状：** 内存随时间增长

**原因：**
- 并发沙盒过多
- Delta Layer 中的大文件
- FUSE 缓存积累

**解决方案：**
```python
# 限制并发沙盒
from agentfense import ResourceLimits

sandbox = Sandbox.from_local(
    "./project",
    resources=ResourceLimits(
        memory_bytes=256 * 1024 * 1024  # 256 MB
    )
)

# 正确清理
sandbox.destroy(delete_codebase=True)
```

### 文件操作缓慢

**症状：** 文件读取/写入很慢

**原因：**
- 大型 Delta Layer
- 网络延迟（如果服务器是远程的）
- FUSE 开销

**解决方案：**
1. 定期导出快照（减少 delta）
2. 尽可能使用本地服务器
3. 批量文件操作
4. 对大文件使用流式传输

## 获取帮助

如果您遇到此处未列出的问题：

1. 查看 [GitHub Issues](https://github.com/AjaxZhan/AgentFense/issues)
2. 启用调试日志
3. 收集服务器日志
4. 创建最小复现示例

**调试日志：**
```python
import logging
logging.basicConfig(
    level=logging.DEBUG,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
```
