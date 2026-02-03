# 安装指南

本指南涵盖 AgentFense 服务端和 Python SDK 的完整安装,包括前置要求、配置和验证。

## 概述

AgentFense 有两个组件:

1. **服务端**(Go) - 用于沙盒管理的 gRPC/REST 服务
2. **Python SDK** - 与服务器交互的客户端库

您可以将两者安装在同一台机器上(开发)或不同机器上(生产)。

## 服务端安装

### 前置要求

| 组件 | 必需 | 版本 | 注意事项 |
|------|------|------|----------|
| **Go** | 是 | 1.21+ | 从源码构建用 |
| **bubblewrap** (`bwrap`) | 推荐 | 最新 | 用于轻量级隔离 |
| **Docker** | 可选 | 20.10+ | 用于容器隔离 |
| **FUSE** | 是 | 2.9+ | 文件系统驱动 |
| **Linux** | 推荐 | 内核 4.18+ | macOS 支持但有限制 |

### 安装前置依赖

#### Ubuntu / Debian

```bash
# 安装 bubblewrap 和 FUSE
sudo apt update
sudo apt install -y bubblewrap fuse3 libfuse3-dev

# 安装 Docker(可选)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER  # 将用户添加到 docker 组
```

#### Fedora / RHEL / CentOS

```bash
# 安装 bubblewrap 和 FUSE
sudo dnf install -y bubblewrap fuse3 fuse3-devel

# 安装 Docker(可选)
sudo dnf install -y docker
sudo systemctl start docker
sudo usermod -aG docker $USER
```

#### macOS

```bash
# 通过 Homebrew 安装
brew install macfuse
brew install --cask docker

# 注意:macOS 上不可用 bwrap,使用 Docker 运行时
```

### 安装方法

#### 方法 1:二进制发布(推荐)

从 GitHub releases 下载预构建的二进制文件:

```bash
# 设置版本(检查最新版本 https://github.com/AjaxZhan/AgentFense/releases)
VERSION=v0.1.0

# 为 Linux 下载
wget https://github.com/AjaxZhan/AgentFense/releases/download/${VERSION}/agentfense-server-linux-amd64.tar.gz
tar -xzf agentfense-server-linux-amd64.tar.gz
cd agentfense-server

# 设置可执行
chmod +x agentfense-server

# 验证安装
./agentfense-server --version
```

可用的二进制文件:
- `agentfense-server-linux-amd64.tar.gz`(Linux 64 位)
- `agentfense-server-linux-arm64.tar.gz`(Linux ARM64)
- `agentfense-server-darwin-amd64.tar.gz`(macOS Intel)
- `agentfense-server-darwin-arm64.tar.gz`(macOS Apple Silicon)

#### 方法 2:从源码构建

```bash
# 克隆仓库
git clone https://github.com/AjaxZhan/AgentFense.git
cd AgentFense

# 安装依赖
go mod tidy

# 构建服务端
go build -o bin/agentfense-server ./cmd/agentfense-server

# 验证构建
./bin/agentfense-server --version
```

**带优化的构建:**

```bash
# 更小的二进制文件,为生产环境优化
go build -ldflags="-s -w" -o bin/agentfense-server ./cmd/agentfense-server
```

#### 方法 3:Docker

在容器中运行服务器:

```bash
# 拉取镜像
docker pull agentfense/agentfense-server:latest

# 运行服务器(gRPC :9000,REST :8080)
docker run -d \
  --name agentfense-server \
  -p 9000:9000 \
  -p 8080:8080 \
  -v /var/lib/agentfense:/var/lib/agentfense \
  --privileged \
  agentfense/agentfense-server:latest

# 检查日志
docker logs -f agentfense-server
```

**注意:**
- 容器内 FUSE 挂载需要 `--privileged`
- 卷 `/var/lib/agentfense` 存储 codebase 和元数据(必须持久化)

### 配置

AgentFense 使用 YAML 配置文件。默认位置:`configs/agentfense-server.yaml`。

#### 最小配置

创建 `config.yaml`:

```yaml
server:
  grpc_addr: ":9000"      # gRPC 服务器地址
  http_addr: ":8080"      # REST 网关地址

storage:
  codebase_path: "/var/lib/agentfense/codebases"  # Codebase 存储(Docker 必须是绝对路径)
  mount_path: "/var/lib/agentfense/mounts"        # FUSE 挂载点和 delta
  db_path: "/var/lib/agentfense/metadata.db"      # SQLite 元数据库

runtime:
  type: "bwrap"           # 运行时:bwrap、docker 或 mock
  default_timeout: "30s"  # 命令超时
  max_timeout: "10m"      # 最大允许超时

logging:
  level: "info"           # 日志级别:debug、info、warn、error
  format: "json"          # 日志格式:json 或 text
```

#### 生产配置

```yaml
server:
  grpc_addr: ":9000"
  http_addr: ":8080"

storage:
  codebase_path: "/data/agentfense/codebases"
  mount_path: "/data/agentfense/mounts"
  db_path: "/data/agentfense/metadata.db"

runtime:
  type: "docker"
  default_timeout: "60s"
  max_timeout: "30m"
  bwrap_path: "/usr/bin/bwrap"
  
  # Docker 运行时设置
  docker:
    host: ""                          # 空 = 使用默认(DOCKER_HOST 或 unix:///var/run/docker.sock)
    default_image: "ubuntu:22.04"     # 默认容器镜像
    network_mode: "none"              # none、bridge 或 host
    enable_networking: false          # 允许沙盒中的网络访问

sandbox:
  default_expiration: "24h"  # 24 小时后自动清理
  max_per_user: 50           # 每个用户的最大并发沙盒数
  cleanup_interval: "5m"     # 清理检查间隔

logging:
  level: "info"
  format: "json"
```

#### 配置选项参考

| 部分 | 键 | 类型 | 默认值 | 描述 |
|------|---|------|--------|------|
| `server` | `grpc_addr` | string | `:9000` | gRPC 服务器绑定地址 |
| | `http_addr` | string | `:8080` | REST 网关绑定地址 |
| `storage` | `codebase_path` | string | - | Codebase 存储目录(绝对路径) |
| | `mount_path` | string | - | 挂载点基础目录(绝对路径) |
| | `db_path` | string | - | 元数据库文件路径 |
| `runtime` | `type` | string | `bwrap` | 运行时类型:`bwrap`、`docker`、`mock` |
| | `bwrap_path` | string | `/usr/bin/bwrap` | bwrap 二进制路径 |
| | `default_timeout` | duration | `30s` | 默认命令超时 |
| | `max_timeout` | duration | `10m` | 最大允许超时 |
| `runtime.docker` | `host` | string | (auto) | Docker 守护进程套接字 |
| | `default_image` | string | `ubuntu:22.04` | 默认容器镜像 |
| | `network_mode` | string | `none` | 网络模式:`none`、`bridge`、`host` |
| | `enable_networking` | bool | `false` | 允许沙盒中的网络 |
| `sandbox` | `default_expiration` | duration | `24h` | 默认沙盒 TTL |
| | `max_per_user` | int | `10` | 每个用户的最大沙盒数 |
| | `cleanup_interval` | duration | `5m` | 清理检查间隔 |
| `logging` | `level` | string | `info` | 日志级别:`debug`、`info`、`warn`、`error` |
| | `format` | string | `json` | 日志格式:`json`、`text` |

### 启动服务器

#### 前台(开发)

```bash
# 使用默认配置
./agentfense-server -config configs/agentfense-server.yaml

# 覆盖运行时
./agentfense-server -runtime docker

# 自定义地址
./agentfense-server -grpc-addr :9001 -http-addr :8081
```

#### 后台(生产)

使用 systemd:

创建 `/etc/systemd/system/agentfense.service`:

```ini
[Unit]
Description=AgentFense Sandbox Server
After=network.target

[Service]
Type=simple
User=agentfense
Group=agentfense
ExecStart=/usr/local/bin/agentfense-server -config /etc/agentfense/config.yaml
Restart=on-failure
RestartSec=5s

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/agentfense

[Install]
WantedBy=multi-user.target
```

启用并启动:

```bash
sudo systemctl daemon-reload
sudo systemctl enable agentfense
sudo systemctl start agentfense

# 检查状态
sudo systemctl status agentfense

# 查看日志
sudo journalctl -u agentfense -f
```

使用 Docker Compose:

创建 `docker-compose.yml`:

```yaml
version: '3.8'

services:
  agentfense-server:
    image: agentfense/agentfense-server:latest
    container_name: agentfense-server
    ports:
      - "9000:9000"
      - "8080:8080"
    volumes:
      - agentfense-data:/var/lib/agentfense
      - ./config.yaml:/etc/agentfense/config.yaml
    privileged: true
    restart: unless-stopped
    environment:
      - AGENTFENSE_CONFIG=/etc/agentfense/config.yaml

volumes:
  agentfense-data:
```

启动:

```bash
docker-compose up -d
docker-compose logs -f
```

### 验证服务器安装

#### 健康检查

```bash
# REST API 健康端点
curl http://localhost:8080/health

# 期望输出:
# {"status":"ok"}
```

#### 测试 gRPC 连接

使用 `grpcurl`:

```bash
# 安装 grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# 列出服务
grpcurl -plaintext localhost:9000 list

# 期望输出包括:
# agentfense.v1.CodebaseService
# agentfense.v1.SandboxService
```

#### 创建测试沙盒

```bash
# 使用 REST API
curl -X POST http://localhost:8080/v1/codebases \
  -H "Content-Type: application/json" \
  -d '{"name": "test", "owner_id": "user_001"}'

# 期望:{"id": "cb_...", ...}
```

## Python SDK 安装

### 前置要求

- **Python 3.8+**(推荐 Python 3.11+)
- **pip** 或 **poetry** 包管理器

### 安装方法

#### 方法 1:PyPI(推荐)

```bash
pip install agentfense
```

#### 方法 2:从源码

```bash
# 克隆仓库
git clone https://github.com/AjaxZhan/AgentFense.git
cd AgentFense/sdk/python

# 以开发模式安装
pip install -e .

# 或安装开发依赖
pip install -e ".[dev]"
```

#### 方法 3:使用 Poetry

```bash
cd AgentFense/sdk/python
poetry install
poetry shell
```

### 验证 SDK 安装

```bash
# 检查版本
python -c "from agentfense import __version__; print(__version__)"

# 测试导入
python -c "from agentfense import Sandbox, SandboxClient; print('SDK installed')"
```

### SDK 配置

SDK 通过 gRPC 端点连接到服务器。配置方式:

#### 选项 1:构造函数参数

```python
from agentfense import Sandbox, SandboxClient

# 高层 API
sandbox = Sandbox.from_local("./project", endpoint="localhost:9000")

# 低层 API
client = SandboxClient(endpoint="localhost:9000")
```

#### 选项 2:环境变量

```bash
# 设置环境变量
export AGENTFENSE_ENDPOINT=localhost:9000

# SDK 自动使用
python your_script.py
```

```python
from agentfense import Sandbox

# 如果设置了 AGENTFENSE_ENDPOINT 则使用它
sandbox = Sandbox.from_local("./project")
```

## 测试安装

创建完整测试:

```python
# test_installation.py
from agentfense import Sandbox

def test_agentfense():
    print("测试 AgentFense 安装...")
    
    # 创建测试项目
    import tempfile
    import os
    
    with tempfile.TemporaryDirectory() as tmpdir:
        # 写入测试文件
        test_file = os.path.join(tmpdir, "hello.py")
        with open(test_file, "w") as f:
            f.write("print('Hello from AgentFense')")
        
        # 创建沙盒
        with Sandbox.from_local(tmpdir, endpoint="localhost:9000") as sandbox:
            result = sandbox.run("python /workspace/hello.py")
            
            assert result.exit_code == 0, f"命令失败: {result.stderr}"
            assert "Hello from AgentFense" in result.stdout, f"意外输出: {result.stdout}"
            
            print(f"✅ 测试通过! 输出: {result.stdout.strip()}")

if __name__ == "__main__":
    test_agentfense()
```

运行测试:

```bash
python test_installation.py
```

期望输出:

```
测试 AgentFense 安装...
✅ 测试通过! 输出: Hello from AgentFense
```

## 故障排查

### 服务器问题

#### 端口已被占用

**错误:** `bind: address already in use`

**解决方案:**

```bash
# 检查什么在使用端口
sudo lsof -i :9000

# 杀死进程或在配置中更改端口
./agentfense-server -grpc-addr :9001
```

#### 权限拒绝(FUSE)

**错误:** `fusermount: failed to open /etc/fuse.conf: Permission denied`

**解决方案:**

```bash
# 将用户添加到 fuse 组
sudo usermod -aG fuse $USER
newgrp fuse

# 或用 sudo 运行服务器(生产环境不推荐)
sudo ./agentfense-server
```

#### 未找到 Bubblewrap

**错误:** `exec: "bwrap": executable file not found in $PATH`

**解决方案:**

```bash
# 安装 bubblewrap
sudo apt install bubblewrap  # Debian/Ubuntu
sudo dnf install bubblewrap  # Fedora/RHEL

# 或切换到 Docker 运行时
./agentfense-server -runtime docker
```

#### Docker 权限拒绝

**错误:** `permission denied while trying to connect to Docker daemon`

**解决方案:**

```bash
# 将用户添加到 docker 组
sudo usermod -aG docker $USER
newgrp docker

# 或设置 DOCKER_HOST 环境变量
export DOCKER_HOST=unix:///var/run/docker.sock
```

### SDK 问题

#### 连接被拒绝

**错误:** `ConnectionError: failed to connect to localhost:9000`

**解决方案:**

```bash
# 检查服务器是否运行
curl http://localhost:8080/health

# 检查防火墙是否允许端口 9000
sudo ufw allow 9000/tcp  # Ubuntu/Debian
sudo firewall-cmd --add-port=9000/tcp --permanent  # RHEL/CentOS
```

#### 导入错误

**错误:** `ModuleNotFoundError: No module named 'agentfense'`

**解决方案:**

```bash
# 重新安装 SDK
pip uninstall agentfense
pip install agentfense

# 或检查 Python 路径
python -c "import sys; print(sys.path)"
```

## 卸载

### 服务器

```bash
# 停止 systemd 服务(如果使用)
sudo systemctl stop agentfense
sudo systemctl disable agentfense
sudo rm /etc/systemd/system/agentfense.service

# 删除数据
sudo rm -rf /var/lib/agentfense

# 删除二进制文件
sudo rm /usr/local/bin/agentfense-server
```

### Docker

```bash
# 停止并删除容器
docker stop agentfense-server
docker rm agentfense-server

# 删除镜像
docker rmi agentfense/agentfense-server:latest

# 删除卷
docker volume rm agentfense_agentfense-data
```

### Python SDK

```bash
pip uninstall agentfense
```

## 下一步

- **[快速开始](quickstart.md)** - 创建第一个沙盒
- **[核心概念](concepts.md)** - 理解架构
- **[安全模型](../security/index.md)** - 了解权限
- **[最佳实践](../security/best-practices.md)** - 生产部署
