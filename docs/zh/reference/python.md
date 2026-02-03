# Python API 参考

从 Python SDK 源代码自动生成的 API 文档。

## 高层 API

### Sandbox

::: agentfense.sandbox.Sandbox
    options:
      show_root_heading: true
      show_source: false
      members:
        - from_local
        - from_codebase
        - connect
        - run
        - exec
        - exec_stream
        - session
        - read_file
        - read_file_bytes
        - write_file
        - list_files
        - start
        - stop
        - refresh
        - destroy

### AsyncSandbox

::: agentfense._async.sandbox.AsyncSandbox
    options:
      show_root_heading: true
      show_source: false
      members:
        - from_local
        - from_codebase
        - connect
        - run
        - exec
        - exec_stream
        - session
        - read_file
        - read_file_bytes
        - write_file
        - list_files
        - start
        - stop
        - refresh
        - destroy

## 低层 API

### SandboxClient

::: agentfense.client.SandboxClient
    options:
      show_root_heading: true
      show_source: false
      members:
        - create_sandbox
        - get_sandbox
        - list_sandboxes
        - start_sandbox
        - stop_sandbox
        - destroy_sandbox
        - exec
        - exec_stream
        - session
        - create_session
        - session_exec
        - get_session
        - list_sessions
        - destroy_session
        - create_codebase
        - get_codebase
        - list_codebases
        - delete_codebase
        - upload_file
        - download_file
        - list_files

### AsyncSandboxClient

::: agentfense._async.client.AsyncSandboxClient
    options:
      show_root_heading: true
      show_source: false

## 类型

### Permission

::: agentfense.types.Permission
    options:
      show_root_heading: true
      members:
        - NONE
        - VIEW
        - READ
        - WRITE

### PatternType

::: agentfense.types.PatternType
    options:
      show_root_heading: true
      members:
        - GLOB
        - DIRECTORY
        - FILE

### RuntimeType

::: agentfense.types.RuntimeType
    options:
      show_root_heading: true
      members:
        - BWRAP
        - DOCKER

### PermissionRule

::: agentfense.types.PermissionRule
    options:
      show_root_heading: true
      show_source: false

### ResourceLimits

::: agentfense.types.ResourceLimits
    options:
      show_root_heading: true
      show_source: false

### ExecResult

::: agentfense.types.ExecResult
    options:
      show_root_heading: true
      show_source: false

## 异常

### SandboxError

::: agentfense.exceptions.SandboxError
    options:
      show_root_heading: true
      show_source: false

### 具体异常

::: agentfense.exceptions.SandboxNotFoundError
::: agentfense.exceptions.SandboxNotRunningError
::: agentfense.exceptions.CommandTimeoutError
::: agentfense.exceptions.CommandExecutionError
::: agentfense.exceptions.PermissionDeniedError
::: agentfense.exceptions.SessionError
::: agentfense.exceptions.CodebaseError
::: agentfense.exceptions.ConnectionError

## 预设

### 预设函数

::: agentfense.presets.list_presets
    options:
      show_root_heading: true
      show_source: false

::: agentfense.presets.get_preset
    options:
      show_root_heading: true
      show_source: false

::: agentfense.presets.get_preset_dicts
    options:
      show_root_heading: true
      show_source: false

::: agentfense.presets.extend_preset
    options:
      show_root_heading: true
      show_source: false

::: agentfense.presets.register_preset
    options:
      show_root_heading: true
      show_source: false

## 工具函数

::: agentfense.utils
    options:
      show_root_heading: true
      show_source: false
      members:
        - walk_directory
        - parse_ignore_file
        - human_readable_size
        - generate_codebase_name
        - generate_owner_id
        - count_files
