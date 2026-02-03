# gRPC/Proto API 参考

Protocol Buffer 定义和 gRPC 服务规范。

## 概述

AgentFense 暴露两个 gRPC 服务：
- **SandboxService** - 沙盒生命周期和命令执行
- **CodebaseService** - 代码库和文件管理

所有服务也可通过 8080 端口的 REST API 访问（HTTP 网关）。

## 服务

### SandboxService

管理沙盒生命周期和执行。

#### CreateSandbox

创建新沙盒。

**请求：**
```protobuf
message CreateSandboxRequest {
  string codebase_id = 1;
  repeated PermissionRule permissions = 2;
  map<string, string> labels = 3;
  int64 expires_in = 4;  // 秒
  Runtime runtime = 5;
  string image = 6;  // Docker 镜像（如果 runtime=DOCKER）
  ResourceLimits resources = 7;
}
```

**响应：**
```protobuf
message Sandbox {
  string id = 1;
  string codebase_id = 2;
  SandboxStatus status = 3;
  repeated PermissionRule permissions = 4;
  map<string, string> labels = 5;
  int64 created_at = 6;
  int64 expires_at = 7;
  Runtime runtime = 8;
  string image = 9;
  ResourceLimits resources = 10;
}
```

#### GetSandbox

检索沙盒信息。

**请求：**
```protobuf
message GetSandboxRequest {
  string sandbox_id = 1;
}
```

#### ListSandboxes

列出所有沙盒，可选按代码库过滤。

**请求：**
```protobuf
message ListSandboxesRequest {
  string codebase_id = 1;  // 可选过滤器
}
```

#### StartSandbox

启动已停止的沙盒。

**请求：**
```protobuf
message StartSandboxRequest {
  string sandbox_id = 1;
}
```

#### StopSandbox

停止运行中的沙盒（不销毁）。

**请求：**
```protobuf
message StopSandboxRequest {
  string sandbox_id = 1;
}
```

#### DestroySandbox

永久销毁沙盒。

**请求：**
```protobuf
message DestroySandboxRequest {
  string sandbox_id = 1;
}
```

#### Exec

在沙盒中执行命令。

**请求：**
```protobuf
message ExecRequest {
  string sandbox_id = 1;
  string command = 2;
  string stdin = 3;
  map<string, string> env = 4;
  string workdir = 5;
  int64 timeout = 6;  // 秒
}
```

**响应：**
```protobuf
message ExecResponse {
  string stdout = 1;
  string stderr = 2;
  int32 exit_code = 3;
  int64 duration_ms = 4;
}
```

#### ExecStream

执行命令并流式输出。

**请求：** 与 Exec 相同

**响应：** 流式
```protobuf
message ExecStreamChunk {
  bytes data = 1;
}
```

### CodebaseService

管理代码库和文件。

#### CreateCodebase

创建新代码库。

**请求：**
```protobuf
message CreateCodebaseRequest {
  string name = 1;
  string owner_id = 2;
}
```

**响应：**
```protobuf
message Codebase {
  string id = 1;
  string name = 2;
  string owner_id = 3;
  int64 created_at = 4;
  int64 file_count = 5;
  int64 total_size = 6;
}
```

#### GetCodebase / ListCodebases / DeleteCodebase

代码库的标准 CRUD 操作。

#### UploadFile

上传文件到代码库。

**请求：** 流式
```protobuf
message UploadFileRequest {
  string codebase_id = 1;
  string file_path = 2;
  bytes chunk = 3;
}
```

**响应：**
```protobuf
message UploadFileResponse {
  int64 size = 1;
  string checksum = 2;
}
```

#### DownloadFile

从代码库下载文件。

**请求：**
```protobuf
message DownloadFileRequest {
  string codebase_id = 1;
  string file_path = 2;
}
```

**响应：** 字节流

#### ListFiles

列出代码库中的文件。

**请求：**
```protobuf
message ListFilesRequest {
  string codebase_id = 1;
  string path = 2;
  bool recursive = 3;
}
```

**响应：**
```protobuf
message ListFilesResponse {
  repeated FileInfo files = 1;
}

message FileInfo {
  string path = 1;
  int64 size = 2;
  int64 modified_at = 3;
  bool is_dir = 4;
}
```

## 类型

### PermissionRule

```protobuf
message PermissionRule {
  string pattern = 1;
  Permission permission = 2;
  PatternType type = 3;
  int32 priority = 4;
}
```

### Permission

```protobuf
enum Permission {
  PERMISSION_NONE = 0;
  PERMISSION_VIEW = 1;
  PERMISSION_READ = 2;
  PERMISSION_WRITE = 3;
}
```

### PatternType

```protobuf
enum PatternType {
  PATTERN_GLOB = 0;
  PATTERN_DIRECTORY = 1;
  PATTERN_FILE = 2;
}
```

### Runtime

```protobuf
enum Runtime {
  RUNTIME_BWRAP = 0;
  RUNTIME_DOCKER = 1;
}
```

### ResourceLimits

```protobuf
message ResourceLimits {
  int64 memory_bytes = 1;
  int64 cpu_quota = 2;
  int64 cpu_shares = 3;
  int32 pids_limit = 4;
}
```

### SandboxStatus

```protobuf
enum SandboxStatus {
  SANDBOX_PENDING = 0;
  SANDBOX_RUNNING = 1;
  SANDBOX_STOPPED = 2;
  SANDBOX_ERROR = 3;
}
```

## REST API

所有 gRPC 端点也可通过 REST 访问：

**基础 URL：** `http://localhost:8080`

**示例：**

```bash
# 创建沙盒
POST /v1/sandboxes
{
  "codebase_id": "cb_123",
  "permissions": [
    {"pattern": "**/*", "permission": "PERMISSION_READ"}
  ]
}

# 执行命令
POST /v1/sandboxes/{sandbox_id}/exec
{
  "command": "ls /workspace",
  "timeout": 30
}

# 上传文件（multipart）
POST /v1/codebases/{codebase_id}/files
```

## Proto 文件

完整的 proto 定义：
- [api/proto/sandbox.proto](https://github.com/AjaxZhan/AgentFense/blob/main/api/proto/sandbox.proto)
- [api/proto/codebase.proto](https://github.com/AjaxZhan/AgentFense/blob/main/api/proto/codebase.proto)
- [api/proto/common.proto](https://github.com/AjaxZhan/AgentFense/blob/main/api/proto/common.proto)
