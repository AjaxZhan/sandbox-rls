from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Permission(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PERMISSION_UNSPECIFIED: _ClassVar[Permission]
    PERMISSION_NONE: _ClassVar[Permission]
    PERMISSION_VIEW: _ClassVar[Permission]
    PERMISSION_READ: _ClassVar[Permission]
    PERMISSION_WRITE: _ClassVar[Permission]

class PatternType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PATTERN_TYPE_UNSPECIFIED: _ClassVar[PatternType]
    PATTERN_TYPE_GLOB: _ClassVar[PatternType]
    PATTERN_TYPE_DIRECTORY: _ClassVar[PatternType]
    PATTERN_TYPE_FILE: _ClassVar[PatternType]

class SandboxStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SANDBOX_STATUS_UNSPECIFIED: _ClassVar[SandboxStatus]
    SANDBOX_STATUS_PENDING: _ClassVar[SandboxStatus]
    SANDBOX_STATUS_RUNNING: _ClassVar[SandboxStatus]
    SANDBOX_STATUS_STOPPED: _ClassVar[SandboxStatus]
    SANDBOX_STATUS_ERROR: _ClassVar[SandboxStatus]

class SessionStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SESSION_STATUS_UNSPECIFIED: _ClassVar[SessionStatus]
    SESSION_STATUS_ACTIVE: _ClassVar[SessionStatus]
    SESSION_STATUS_CLOSED: _ClassVar[SessionStatus]
PERMISSION_UNSPECIFIED: Permission
PERMISSION_NONE: Permission
PERMISSION_VIEW: Permission
PERMISSION_READ: Permission
PERMISSION_WRITE: Permission
PATTERN_TYPE_UNSPECIFIED: PatternType
PATTERN_TYPE_GLOB: PatternType
PATTERN_TYPE_DIRECTORY: PatternType
PATTERN_TYPE_FILE: PatternType
SANDBOX_STATUS_UNSPECIFIED: SandboxStatus
SANDBOX_STATUS_PENDING: SandboxStatus
SANDBOX_STATUS_RUNNING: SandboxStatus
SANDBOX_STATUS_STOPPED: SandboxStatus
SANDBOX_STATUS_ERROR: SandboxStatus
SESSION_STATUS_UNSPECIFIED: SessionStatus
SESSION_STATUS_ACTIVE: SessionStatus
SESSION_STATUS_CLOSED: SessionStatus

class PermissionRule(_message.Message):
    __slots__ = ("pattern", "type", "permission", "priority")
    PATTERN_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    PERMISSION_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    pattern: str
    type: PatternType
    permission: Permission
    priority: int
    def __init__(self, pattern: _Optional[str] = ..., type: _Optional[_Union[PatternType, str]] = ..., permission: _Optional[_Union[Permission, str]] = ..., priority: _Optional[int] = ...) -> None: ...

class Empty(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
