# gRPC/Proto API Reference

Protocol Buffer definitions and gRPC service specifications.

## Overview

AgentFense exposes two gRPC services:
- **SandboxService** - Sandbox lifecycle and command execution
- **CodebaseService** - Codebase and file management

All services are also available via REST API at port 8080 (HTTP gateway).

## Services

### SandboxService

Manages sandbox lifecycle and execution.

#### CreateSandbox

Creates a new sandbox.

**Request:**
```protobuf
message CreateSandboxRequest {
  string codebase_id = 1;
  repeated PermissionRule permissions = 2;
  map<string, string> labels = 3;
  int64 expires_in = 4;  // seconds
  Runtime runtime = 5;
  string image = 6;  // Docker image (if runtime=DOCKER)
  ResourceLimits resources = 7;
}
```

**Response:**
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

Retrieves sandbox information.

**Request:**
```protobuf
message GetSandboxRequest {
  string sandbox_id = 1;
}
```

#### ListSandboxes

Lists all sandboxes, optionally filtered by codebase.

**Request:**
```protobuf
message ListSandboxesRequest {
  string codebase_id = 1;  // optional filter
}
```

#### StartSandbox

Starts a stopped sandbox.

**Request:**
```protobuf
message StartSandboxRequest {
  string sandbox_id = 1;
}
```

#### StopSandbox

Stops a running sandbox (does not destroy).

**Request:**
```protobuf
message StopSandboxRequest {
  string sandbox_id = 1;
}
```

#### DestroySandbox

Permanently destroys a sandbox.

**Request:**
```protobuf
message DestroySandboxRequest {
  string sandbox_id = 1;
}
```

#### Exec

Executes a command in the sandbox.

**Request:**
```protobuf
message ExecRequest {
  string sandbox_id = 1;
  string command = 2;
  string stdin = 3;
  map<string, string> env = 4;
  string workdir = 5;
  int64 timeout = 6;  // seconds
}
```

**Response:**
```protobuf
message ExecResponse {
  string stdout = 1;
  string stderr = 2;
  int32 exit_code = 3;
  int64 duration_ms = 4;
}
```

#### ExecStream

Executes a command with streaming output.

**Request:** Same as Exec

**Response:** Stream of
```protobuf
message ExecStreamChunk {
  bytes data = 1;
}
```

### CodebaseService

Manages codebases and files.

#### CreateCodebase

Creates a new codebase.

**Request:**
```protobuf
message CreateCodebaseRequest {
  string name = 1;
  string owner_id = 2;
}
```

**Response:**
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

Standard CRUD operations for codebases.

#### UploadFile

Uploads a file to a codebase.

**Request:** Stream of
```protobuf
message UploadFileRequest {
  string codebase_id = 1;
  string file_path = 2;
  bytes chunk = 3;
}
```

**Response:**
```protobuf
message UploadFileResponse {
  int64 size = 1;
  string checksum = 2;
}
```

#### DownloadFile

Downloads a file from a codebase.

**Request:**
```protobuf
message DownloadFileRequest {
  string codebase_id = 1;
  string file_path = 2;
}
```

**Response:** Stream of bytes

#### ListFiles

Lists files in a codebase.

**Request:**
```protobuf
message ListFilesRequest {
  string codebase_id = 1;
  string path = 2;
  bool recursive = 3;
}
```

**Response:**
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

## Types

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

All gRPC endpoints are also available via REST:

**Base URL:** `http://localhost:8080`

**Examples:**

```bash
# Create sandbox
POST /v1/sandboxes
{
  "codebase_id": "cb_123",
  "permissions": [
    {"pattern": "**/*", "permission": "PERMISSION_READ"}
  ]
}

# Execute command
POST /v1/sandboxes/{sandbox_id}/exec
{
  "command": "ls /workspace",
  "timeout": 30
}

# Upload file (multipart)
POST /v1/codebases/{codebase_id}/files
```

## Proto Files

Full proto definitions:
- [api/proto/sandbox.proto](https://github.com/AjaxZhan/AgentFense/blob/main/api/proto/sandbox.proto)
- [api/proto/codebase.proto](https://github.com/AjaxZhan/AgentFense/blob/main/api/proto/codebase.proto)
- [api/proto/common.proto](https://github.com/AjaxZhan/AgentFense/blob/main/api/proto/common.proto)
