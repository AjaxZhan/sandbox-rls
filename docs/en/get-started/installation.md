# Installation Guide

This guide covers complete installation of AgentFense server and Python SDK, including prerequisites, configuration, and verification.

## Overview

AgentFense has two components:

1. **Server** (Go) - gRPC/REST service for sandbox management
2. **Python SDK** - Client library for interacting with the server

You can install both on the same machine (development) or separate machines (production).

## Server Installation

### Prerequisites

| Component | Required | Version | Notes |
|-----------|----------|---------|-------|
| **Go** | Yes | 1.21+ | For building from source |
| **bubblewrap** (`bwrap`) | Recommended | Latest | For lightweight isolation |
| **Docker** | Optional | 20.10+ | For container isolation |
| **FUSE** | Yes | 2.9+ | Filesystem driver |
| **Linux** | Recommended | Kernel 4.18+ | macOS supported with limitations |

### Installing Prerequisites

#### Ubuntu / Debian

```bash
# Install bubblewrap and FUSE
sudo apt update
sudo apt install -y bubblewrap fuse3 libfuse3-dev

# Install Docker (optional)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER  # Add user to docker group
```

#### Fedora / RHEL / CentOS

```bash
# Install bubblewrap and FUSE
sudo dnf install -y bubblewrap fuse3 fuse3-devel

# Install Docker (optional)
sudo dnf install -y docker
sudo systemctl start docker
sudo usermod -aG docker $USER
```

#### macOS

```bash
# Install via Homebrew
brew install macfuse
brew install --cask docker

# Note: bwrap not available on macOS, use Docker runtime
```

### Installation Methods

#### Method 1: Binary Release (Recommended)

Download pre-built binaries from GitHub releases:

```bash
# Set version (check latest at https://github.com/AjaxZhan/AgentFense/releases)
VERSION=v0.1.0

# Download for Linux
wget https://github.com/AjaxZhan/AgentFense/releases/download/${VERSION}/agentfense-server-linux-amd64.tar.gz
tar -xzf agentfense-server-linux-amd64.tar.gz
cd agentfense-server

# Make executable
chmod +x agentfense-server

# Verify installation
./agentfense-server --version
```

Available binaries:
- `agentfense-server-linux-amd64.tar.gz` (Linux 64-bit)
- `agentfense-server-linux-arm64.tar.gz` (Linux ARM64)
- `agentfense-server-darwin-amd64.tar.gz` (macOS Intel)
- `agentfense-server-darwin-arm64.tar.gz` (macOS Apple Silicon)

#### Method 2: Build from Source

```bash
# Clone repository
git clone https://github.com/AjaxZhan/AgentFense.git
cd AgentFense

# Install dependencies
go mod tidy

# Build server
go build -o bin/agentfense-server ./cmd/agentfense-server

# Verify build
./bin/agentfense-server --version
```

**Build with optimizations:**

```bash
# Smaller binary, optimized for production
go build -ldflags="-s -w" -o bin/agentfense-server ./cmd/agentfense-server
```

#### Method 3: Docker

Run server in a container:

```bash
# Pull image
docker pull agentfense/agentfense-server:latest

# Run server (gRPC :9000, REST :8080)
docker run -d \
  --name agentfense-server \
  -p 9000:9000 \
  -p 8080:8080 \
  -v /var/lib/agentfense:/var/lib/agentfense \
  --privileged \
  agentfense/agentfense-server:latest

# Check logs
docker logs -f agentfense-server
```

**Notes:**
- `--privileged` required for FUSE mounts inside container
- Volume `/var/lib/agentfense` stores codebases and metadata (must persist)

### Configuration

AgentFense uses a YAML configuration file. Default location: `configs/agentfense-server.yaml`.

#### Minimal Configuration

Create `config.yaml`:

```yaml
server:
  grpc_addr: ":9000"      # gRPC server address
  http_addr: ":8080"      # REST gateway address

storage:
  codebase_path: "/var/lib/agentfense/codebases"  # Codebase storage (must be absolute for Docker)
  mount_path: "/var/lib/agentfense/mounts"        # FUSE mount points and deltas
  db_path: "/var/lib/agentfense/metadata.db"      # SQLite metadata database

runtime:
  type: "bwrap"           # Runtime: bwrap, docker, or mock
  default_timeout: "30s"  # Command timeout
  max_timeout: "10m"      # Maximum allowed timeout

logging:
  level: "info"           # Log level: debug, info, warn, error
  format: "json"          # Log format: json or text
```

#### Production Configuration

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
  
  # Docker runtime settings
  docker:
    host: ""                          # Empty = use default (DOCKER_HOST or unix:///var/run/docker.sock)
    default_image: "ubuntu:22.04"     # Default container image
    network_mode: "none"              # none, bridge, or host
    enable_networking: false          # Allow network access in sandboxes

sandbox:
  default_expiration: "24h"  # Auto-cleanup after 24 hours
  max_per_user: 50           # Max concurrent sandboxes per user
  cleanup_interval: "5m"     # Cleanup check interval

logging:
  level: "info"
  format: "json"
```

#### Configuration Options Reference

| Section | Key | Type | Default | Description |
|---------|-----|------|---------|-------------|
| `server` | `grpc_addr` | string | `:9000` | gRPC server bind address |
| | `http_addr` | string | `:8080` | REST gateway bind address |
| `storage` | `codebase_path` | string | - | Codebase storage directory (absolute path) |
| | `mount_path` | string | - | Mount point base directory (absolute path) |
| | `db_path` | string | - | Metadata database file path |
| `runtime` | `type` | string | `bwrap` | Runtime type: `bwrap`, `docker`, `mock` |
| | `bwrap_path` | string | `/usr/bin/bwrap` | Path to bwrap binary |
| | `default_timeout` | duration | `30s` | Default command timeout |
| | `max_timeout` | duration | `10m` | Maximum allowed timeout |
| `runtime.docker` | `host` | string | (auto) | Docker daemon socket |
| | `default_image` | string | `ubuntu:22.04` | Default container image |
| | `network_mode` | string | `none` | Network mode: `none`, `bridge`, `host` |
| | `enable_networking` | bool | `false` | Allow network in sandboxes |
| `sandbox` | `default_expiration` | duration | `24h` | Default sandbox TTL |
| | `max_per_user` | int | `10` | Max sandboxes per user |
| | `cleanup_interval` | duration | `5m` | Cleanup check interval |
| `logging` | `level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| | `format` | string | `json` | Log format: `json`, `text` |

### Starting the Server

#### Foreground (Development)

```bash
# Use default config
./agentfense-server -config configs/agentfense-server.yaml

# Override runtime
./agentfense-server -runtime docker

# Custom addresses
./agentfense-server -grpc-addr :9001 -http-addr :8081
```

#### Background (Production)

Using systemd:

Create `/etc/systemd/system/agentfense.service`:

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

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/agentfense

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable agentfense
sudo systemctl start agentfense

# Check status
sudo systemctl status agentfense

# View logs
sudo journalctl -u agentfense -f
```

Using Docker Compose:

Create `docker-compose.yml`:

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

Start:

```bash
docker-compose up -d
docker-compose logs -f
```

### Verifying Server Installation

#### Health Check

```bash
# REST API health endpoint
curl http://localhost:8080/health

# Expected output:
# {"status":"ok"}
```

#### Test gRPC Connection

Using `grpcurl`:

```bash
# Install grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# List services
grpcurl -plaintext localhost:9000 list

# Expected output includes:
# agentfense.v1.CodebaseService
# agentfense.v1.SandboxService
```

#### Create Test Sandbox

```bash
# Using REST API
curl -X POST http://localhost:8080/v1/codebases \
  -H "Content-Type: application/json" \
  -d '{"name": "test", "owner_id": "user_001"}'

# Expected: {"id": "cb_...", ...}
```

## Python SDK Installation

### Prerequisites

- **Python 3.8+** (Python 3.11+ recommended)
- **pip** or **poetry** package manager

### Installation Methods

#### Method 1: PyPI (Recommended)

```bash
pip install agentfense
```

#### Method 2: From Source

```bash
# Clone repository
git clone https://github.com/AjaxZhan/AgentFense.git
cd AgentFense/sdk/python

# Install in development mode
pip install -e .

# Or install with development dependencies
pip install -e ".[dev]"
```

#### Method 3: Using Poetry

```bash
cd AgentFense/sdk/python
poetry install
poetry shell
```

### Verifying SDK Installation

```bash
# Check version
python -c "from agentfense import __version__; print(__version__)"

# Test import
python -c "from agentfense import Sandbox, SandboxClient; print('SDK installed')"
```

### SDK Configuration

The SDK connects to the server via gRPC endpoint. Configure via:

#### Option 1: Constructor Parameter

```python
from agentfense import Sandbox, SandboxClient

# High-level API
sandbox = Sandbox.from_local("./project", endpoint="localhost:9000")

# Low-level API
client = SandboxClient(endpoint="localhost:9000")
```

#### Option 2: Environment Variable

```bash
# Set environment variable
export AGENTFENSE_ENDPOINT=localhost:9000

# SDK uses it automatically
python your_script.py
```

```python
from agentfense import Sandbox

# Uses AGENTFENSE_ENDPOINT if set
sandbox = Sandbox.from_local("./project")
```

## Testing the Installation

Create a complete test:

```python
# test_installation.py
from agentfense import Sandbox

def test_agentfense():
    print("Testing AgentFense installation...")
    
    # Create test project
    import tempfile
    import os
    
    with tempfile.TemporaryDirectory() as tmpdir:
        # Write test file
        test_file = os.path.join(tmpdir, "hello.py")
        with open(test_file, "w") as f:
            f.write("print('Hello from AgentFense')")
        
        # Create sandbox
        with Sandbox.from_local(tmpdir, endpoint="localhost:9000") as sandbox:
            result = sandbox.run("python /workspace/hello.py")
            
            assert result.exit_code == 0, f"Command failed: {result.stderr}"
            assert "Hello from AgentFense" in result.stdout, f"Unexpected output: {result.stdout}"
            
            print(f"✅ Test passed! Output: {result.stdout.strip()}")

if __name__ == "__main__":
    test_agentfense()
```

Run the test:

```bash
python test_installation.py
```

Expected output:

```
Testing AgentFense installation...
✅ Test passed! Output: Hello from AgentFense
```

## Troubleshooting

### Server Issues

#### Port Already in Use

**Error:** `bind: address already in use`

**Solution:**

```bash
# Check what's using the port
sudo lsof -i :9000

# Kill the process or change port in config
./agentfense-server -grpc-addr :9001
```

#### Permission Denied (FUSE)

**Error:** `fusermount: failed to open /etc/fuse.conf: Permission denied`

**Solution:**

```bash
# Add user to fuse group
sudo usermod -aG fuse $USER
newgrp fuse

# Or run server with sudo (not recommended for production)
sudo ./agentfense-server
```

#### Bubblewrap Not Found

**Error:** `exec: "bwrap": executable file not found in $PATH`

**Solution:**

```bash
# Install bubblewrap
sudo apt install bubblewrap  # Debian/Ubuntu
sudo dnf install bubblewrap  # Fedora/RHEL

# Or switch to Docker runtime
./agentfense-server -runtime docker
```

#### Docker Permission Denied

**Error:** `permission denied while trying to connect to Docker daemon`

**Solution:**

```bash
# Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Or set DOCKER_HOST environment variable
export DOCKER_HOST=unix:///var/run/docker.sock
```

### SDK Issues

#### Connection Refused

**Error:** `ConnectionError: failed to connect to localhost:9000`

**Solution:**

```bash
# Check server is running
curl http://localhost:8080/health

# Check firewall allows port 9000
sudo ufw allow 9000/tcp  # Ubuntu/Debian
sudo firewall-cmd --add-port=9000/tcp --permanent  # RHEL/CentOS
```

#### Import Error

**Error:** `ModuleNotFoundError: No module named 'agentfense'`

**Solution:**

```bash
# Reinstall SDK
pip uninstall agentfense
pip install agentfense

# Or check Python path
python -c "import sys; print(sys.path)"
```

## Uninstallation

### Server

```bash
# Stop systemd service (if using)
sudo systemctl stop agentfense
sudo systemctl disable agentfense
sudo rm /etc/systemd/system/agentfense.service

# Remove data
sudo rm -rf /var/lib/agentfense

# Remove binary
sudo rm /usr/local/bin/agentfense-server
```

### Docker

```bash
# Stop and remove container
docker stop agentfense-server
docker rm agentfense-server

# Remove image
docker rmi agentfense/agentfense-server:latest

# Remove volumes
docker volume rm agentfense_agentfense-data
```

### Python SDK

```bash
pip uninstall agentfense
```

## Next Steps

- **[Quick Start](quickstart.md)** - Create your first sandbox
- **[Core Concepts](concepts.md)** - Understand architecture
- **[Security Model](../security/index.md)** - Learn about permissions
- **[Best Practices](../security/best-practices.md)** - Production deployment
