import datetime

from google.api import annotations_pb2 as _annotations_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf import duration_pb2 as _duration_pb2
import common_pb2 as _common_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RuntimeType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    RUNTIME_TYPE_UNSPECIFIED: _ClassVar[RuntimeType]
    RUNTIME_TYPE_BWRAP: _ClassVar[RuntimeType]
    RUNTIME_TYPE_DOCKER: _ClassVar[RuntimeType]
RUNTIME_TYPE_UNSPECIFIED: RuntimeType
RUNTIME_TYPE_BWRAP: RuntimeType
RUNTIME_TYPE_DOCKER: RuntimeType

class ResourceLimits(_message.Message):
    __slots__ = ("memory_bytes", "cpu_quota", "cpu_shares", "pids_limit")
    MEMORY_BYTES_FIELD_NUMBER: _ClassVar[int]
    CPU_QUOTA_FIELD_NUMBER: _ClassVar[int]
    CPU_SHARES_FIELD_NUMBER: _ClassVar[int]
    PIDS_LIMIT_FIELD_NUMBER: _ClassVar[int]
    memory_bytes: int
    cpu_quota: int
    cpu_shares: int
    pids_limit: int
    def __init__(self, memory_bytes: _Optional[int] = ..., cpu_quota: _Optional[int] = ..., cpu_shares: _Optional[int] = ..., pids_limit: _Optional[int] = ...) -> None: ...

class Sandbox(_message.Message):
    __slots__ = ("id", "codebase_id", "permissions", "status", "labels", "created_at", "started_at", "stopped_at", "expires_at", "runtime", "image", "resources")
    class LabelsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    CODEBASE_ID_FIELD_NUMBER: _ClassVar[int]
    PERMISSIONS_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    STOPPED_AT_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_AT_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    RESOURCES_FIELD_NUMBER: _ClassVar[int]
    id: str
    codebase_id: str
    permissions: _containers.RepeatedCompositeFieldContainer[_common_pb2.PermissionRule]
    status: _common_pb2.SandboxStatus
    labels: _containers.ScalarMap[str, str]
    created_at: _timestamp_pb2.Timestamp
    started_at: _timestamp_pb2.Timestamp
    stopped_at: _timestamp_pb2.Timestamp
    expires_at: _timestamp_pb2.Timestamp
    runtime: RuntimeType
    image: str
    resources: ResourceLimits
    def __init__(self, id: _Optional[str] = ..., codebase_id: _Optional[str] = ..., permissions: _Optional[_Iterable[_Union[_common_pb2.PermissionRule, _Mapping]]] = ..., status: _Optional[_Union[_common_pb2.SandboxStatus, str]] = ..., labels: _Optional[_Mapping[str, str]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., started_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., stopped_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., expires_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., runtime: _Optional[_Union[RuntimeType, str]] = ..., image: _Optional[str] = ..., resources: _Optional[_Union[ResourceLimits, _Mapping]] = ...) -> None: ...

class CreateSandboxRequest(_message.Message):
    __slots__ = ("codebase_id", "permissions", "expires_in", "labels", "runtime", "image", "resources")
    class LabelsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CODEBASE_ID_FIELD_NUMBER: _ClassVar[int]
    PERMISSIONS_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_IN_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    IMAGE_FIELD_NUMBER: _ClassVar[int]
    RESOURCES_FIELD_NUMBER: _ClassVar[int]
    codebase_id: str
    permissions: _containers.RepeatedCompositeFieldContainer[_common_pb2.PermissionRule]
    expires_in: _duration_pb2.Duration
    labels: _containers.ScalarMap[str, str]
    runtime: RuntimeType
    image: str
    resources: ResourceLimits
    def __init__(self, codebase_id: _Optional[str] = ..., permissions: _Optional[_Iterable[_Union[_common_pb2.PermissionRule, _Mapping]]] = ..., expires_in: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ..., labels: _Optional[_Mapping[str, str]] = ..., runtime: _Optional[_Union[RuntimeType, str]] = ..., image: _Optional[str] = ..., resources: _Optional[_Union[ResourceLimits, _Mapping]] = ...) -> None: ...

class GetSandboxRequest(_message.Message):
    __slots__ = ("sandbox_id",)
    SANDBOX_ID_FIELD_NUMBER: _ClassVar[int]
    sandbox_id: str
    def __init__(self, sandbox_id: _Optional[str] = ...) -> None: ...

class ListSandboxesRequest(_message.Message):
    __slots__ = ("codebase_id", "page_size", "page_token")
    CODEBASE_ID_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    codebase_id: str
    page_size: int
    page_token: str
    def __init__(self, codebase_id: _Optional[str] = ..., page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class ListSandboxesResponse(_message.Message):
    __slots__ = ("sandboxes", "next_page_token")
    SANDBOXES_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    sandboxes: _containers.RepeatedCompositeFieldContainer[Sandbox]
    next_page_token: str
    def __init__(self, sandboxes: _Optional[_Iterable[_Union[Sandbox, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class StartSandboxRequest(_message.Message):
    __slots__ = ("sandbox_id",)
    SANDBOX_ID_FIELD_NUMBER: _ClassVar[int]
    sandbox_id: str
    def __init__(self, sandbox_id: _Optional[str] = ...) -> None: ...

class StopSandboxRequest(_message.Message):
    __slots__ = ("sandbox_id",)
    SANDBOX_ID_FIELD_NUMBER: _ClassVar[int]
    sandbox_id: str
    def __init__(self, sandbox_id: _Optional[str] = ...) -> None: ...

class DestroySandboxRequest(_message.Message):
    __slots__ = ("sandbox_id",)
    SANDBOX_ID_FIELD_NUMBER: _ClassVar[int]
    sandbox_id: str
    def __init__(self, sandbox_id: _Optional[str] = ...) -> None: ...

class ExecRequest(_message.Message):
    __slots__ = ("sandbox_id", "command", "stdin", "env", "workdir", "timeout")
    class EnvEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SANDBOX_ID_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    STDIN_FIELD_NUMBER: _ClassVar[int]
    ENV_FIELD_NUMBER: _ClassVar[int]
    WORKDIR_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_FIELD_NUMBER: _ClassVar[int]
    sandbox_id: str
    command: str
    stdin: str
    env: _containers.ScalarMap[str, str]
    workdir: str
    timeout: _duration_pb2.Duration
    def __init__(self, sandbox_id: _Optional[str] = ..., command: _Optional[str] = ..., stdin: _Optional[str] = ..., env: _Optional[_Mapping[str, str]] = ..., workdir: _Optional[str] = ..., timeout: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class ExecResult(_message.Message):
    __slots__ = ("stdout", "stderr", "exit_code", "duration")
    STDOUT_FIELD_NUMBER: _ClassVar[int]
    STDERR_FIELD_NUMBER: _ClassVar[int]
    EXIT_CODE_FIELD_NUMBER: _ClassVar[int]
    DURATION_FIELD_NUMBER: _ClassVar[int]
    stdout: str
    stderr: str
    exit_code: int
    duration: _duration_pb2.Duration
    def __init__(self, stdout: _Optional[str] = ..., stderr: _Optional[str] = ..., exit_code: _Optional[int] = ..., duration: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...

class ExecOutput(_message.Message):
    __slots__ = ("type", "data")
    class OutputType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        OUTPUT_TYPE_UNSPECIFIED: _ClassVar[ExecOutput.OutputType]
        OUTPUT_TYPE_STDOUT: _ClassVar[ExecOutput.OutputType]
        OUTPUT_TYPE_STDERR: _ClassVar[ExecOutput.OutputType]
    OUTPUT_TYPE_UNSPECIFIED: ExecOutput.OutputType
    OUTPUT_TYPE_STDOUT: ExecOutput.OutputType
    OUTPUT_TYPE_STDERR: ExecOutput.OutputType
    TYPE_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    type: ExecOutput.OutputType
    data: bytes
    def __init__(self, type: _Optional[_Union[ExecOutput.OutputType, str]] = ..., data: _Optional[bytes] = ...) -> None: ...

class Session(_message.Message):
    __slots__ = ("id", "sandbox_id", "status", "shell", "created_at", "closed_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    SANDBOX_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    SHELL_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    CLOSED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    sandbox_id: str
    status: _common_pb2.SessionStatus
    shell: str
    created_at: _timestamp_pb2.Timestamp
    closed_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., sandbox_id: _Optional[str] = ..., status: _Optional[_Union[_common_pb2.SessionStatus, str]] = ..., shell: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., closed_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateSessionRequest(_message.Message):
    __slots__ = ("sandbox_id", "shell", "env")
    class EnvEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    SANDBOX_ID_FIELD_NUMBER: _ClassVar[int]
    SHELL_FIELD_NUMBER: _ClassVar[int]
    ENV_FIELD_NUMBER: _ClassVar[int]
    sandbox_id: str
    shell: str
    env: _containers.ScalarMap[str, str]
    def __init__(self, sandbox_id: _Optional[str] = ..., shell: _Optional[str] = ..., env: _Optional[_Mapping[str, str]] = ...) -> None: ...

class GetSessionRequest(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class ListSessionsRequest(_message.Message):
    __slots__ = ("sandbox_id",)
    SANDBOX_ID_FIELD_NUMBER: _ClassVar[int]
    sandbox_id: str
    def __init__(self, sandbox_id: _Optional[str] = ...) -> None: ...

class ListSessionsResponse(_message.Message):
    __slots__ = ("sessions",)
    SESSIONS_FIELD_NUMBER: _ClassVar[int]
    sessions: _containers.RepeatedCompositeFieldContainer[Session]
    def __init__(self, sessions: _Optional[_Iterable[_Union[Session, _Mapping]]] = ...) -> None: ...

class DestroySessionRequest(_message.Message):
    __slots__ = ("session_id",)
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    def __init__(self, session_id: _Optional[str] = ...) -> None: ...

class SessionExecRequest(_message.Message):
    __slots__ = ("session_id", "command", "timeout")
    SESSION_ID_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_FIELD_NUMBER: _ClassVar[int]
    session_id: str
    command: str
    timeout: _duration_pb2.Duration
    def __init__(self, session_id: _Optional[str] = ..., command: _Optional[str] = ..., timeout: _Optional[_Union[datetime.timedelta, _duration_pb2.Duration, _Mapping]] = ...) -> None: ...
