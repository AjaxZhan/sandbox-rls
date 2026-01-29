import datetime

from google.api import annotations_pb2 as _annotations_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
import common_pb2 as _common_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Codebase(_message.Message):
    __slots__ = ("id", "name", "owner_id", "size", "file_count", "created_at", "updated_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    OWNER_ID_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    FILE_COUNT_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    owner_id: str
    size: int
    file_count: int
    created_at: _timestamp_pb2.Timestamp
    updated_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., owner_id: _Optional[str] = ..., size: _Optional[int] = ..., file_count: _Optional[int] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class CreateCodebaseRequest(_message.Message):
    __slots__ = ("name", "owner_id")
    NAME_FIELD_NUMBER: _ClassVar[int]
    OWNER_ID_FIELD_NUMBER: _ClassVar[int]
    name: str
    owner_id: str
    def __init__(self, name: _Optional[str] = ..., owner_id: _Optional[str] = ...) -> None: ...

class GetCodebaseRequest(_message.Message):
    __slots__ = ("codebase_id",)
    CODEBASE_ID_FIELD_NUMBER: _ClassVar[int]
    codebase_id: str
    def __init__(self, codebase_id: _Optional[str] = ...) -> None: ...

class ListCodebasesRequest(_message.Message):
    __slots__ = ("owner_id", "page_size", "page_token")
    OWNER_ID_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    owner_id: str
    page_size: int
    page_token: str
    def __init__(self, owner_id: _Optional[str] = ..., page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class ListCodebasesResponse(_message.Message):
    __slots__ = ("codebases", "next_page_token")
    CODEBASES_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    codebases: _containers.RepeatedCompositeFieldContainer[Codebase]
    next_page_token: str
    def __init__(self, codebases: _Optional[_Iterable[_Union[Codebase, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class DeleteCodebaseRequest(_message.Message):
    __slots__ = ("codebase_id",)
    CODEBASE_ID_FIELD_NUMBER: _ClassVar[int]
    codebase_id: str
    def __init__(self, codebase_id: _Optional[str] = ...) -> None: ...

class UploadChunk(_message.Message):
    __slots__ = ("metadata", "data")
    class Metadata(_message.Message):
        __slots__ = ("codebase_id", "file_path", "total_size")
        CODEBASE_ID_FIELD_NUMBER: _ClassVar[int]
        FILE_PATH_FIELD_NUMBER: _ClassVar[int]
        TOTAL_SIZE_FIELD_NUMBER: _ClassVar[int]
        codebase_id: str
        file_path: str
        total_size: int
        def __init__(self, codebase_id: _Optional[str] = ..., file_path: _Optional[str] = ..., total_size: _Optional[int] = ...) -> None: ...
    METADATA_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    metadata: UploadChunk.Metadata
    data: bytes
    def __init__(self, metadata: _Optional[_Union[UploadChunk.Metadata, _Mapping]] = ..., data: _Optional[bytes] = ...) -> None: ...

class UploadResult(_message.Message):
    __slots__ = ("codebase_id", "file_path", "size", "checksum")
    CODEBASE_ID_FIELD_NUMBER: _ClassVar[int]
    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    CHECKSUM_FIELD_NUMBER: _ClassVar[int]
    codebase_id: str
    file_path: str
    size: int
    checksum: str
    def __init__(self, codebase_id: _Optional[str] = ..., file_path: _Optional[str] = ..., size: _Optional[int] = ..., checksum: _Optional[str] = ...) -> None: ...

class DownloadFileRequest(_message.Message):
    __slots__ = ("codebase_id", "file_path")
    CODEBASE_ID_FIELD_NUMBER: _ClassVar[int]
    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    codebase_id: str
    file_path: str
    def __init__(self, codebase_id: _Optional[str] = ..., file_path: _Optional[str] = ...) -> None: ...

class FileChunk(_message.Message):
    __slots__ = ("data",)
    DATA_FIELD_NUMBER: _ClassVar[int]
    data: bytes
    def __init__(self, data: _Optional[bytes] = ...) -> None: ...

class ListFilesRequest(_message.Message):
    __slots__ = ("codebase_id", "path", "recursive")
    CODEBASE_ID_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    RECURSIVE_FIELD_NUMBER: _ClassVar[int]
    codebase_id: str
    path: str
    recursive: bool
    def __init__(self, codebase_id: _Optional[str] = ..., path: _Optional[str] = ..., recursive: bool = ...) -> None: ...

class ListFilesResponse(_message.Message):
    __slots__ = ("files",)
    FILES_FIELD_NUMBER: _ClassVar[int]
    files: _containers.RepeatedCompositeFieldContainer[FileInfo]
    def __init__(self, files: _Optional[_Iterable[_Union[FileInfo, _Mapping]]] = ...) -> None: ...

class FileInfo(_message.Message):
    __slots__ = ("path", "name", "is_dir", "size", "modified_at")
    PATH_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    IS_DIR_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    MODIFIED_AT_FIELD_NUMBER: _ClassVar[int]
    path: str
    name: str
    is_dir: bool
    size: int
    modified_at: _timestamp_pb2.Timestamp
    def __init__(self, path: _Optional[str] = ..., name: _Optional[str] = ..., is_dir: bool = ..., size: _Optional[int] = ..., modified_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...
