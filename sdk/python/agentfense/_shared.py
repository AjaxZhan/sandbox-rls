"""Shared utility functions for sync and async clients.

This module contains conversion functions between protobuf messages and
Python dataclasses, used by both synchronous and asynchronous clients.
"""

from datetime import datetime, timedelta
from typing import Optional

from ._gen import sandbox_pb2
from ._gen import codebase_pb2
from ._gen import common_pb2
from .types import (
    Codebase,
    FileInfo,
    Permission,
    PatternType,
    PermissionRule,
    ResourceLimits,
    RuntimeType,
    Sandbox,
    SandboxStatus,
    Session,
    SessionStatus,
)


# ============================================
# Permission Conversions
# ============================================

def permission_to_proto(perm: Permission) -> common_pb2.Permission:
    """Convert Permission enum to protobuf."""
    mapping = {
        Permission.NONE: common_pb2.PERMISSION_NONE,
        Permission.VIEW: common_pb2.PERMISSION_VIEW,
        Permission.READ: common_pb2.PERMISSION_READ,
        Permission.WRITE: common_pb2.PERMISSION_WRITE,
    }
    return mapping.get(perm, common_pb2.PERMISSION_UNSPECIFIED)


def proto_to_permission(perm: common_pb2.Permission) -> Permission:
    """Convert protobuf Permission to enum."""
    mapping = {
        common_pb2.PERMISSION_NONE: Permission.NONE,
        common_pb2.PERMISSION_VIEW: Permission.VIEW,
        common_pb2.PERMISSION_READ: Permission.READ,
        common_pb2.PERMISSION_WRITE: Permission.WRITE,
    }
    return mapping.get(perm, Permission.NONE)


def pattern_type_to_proto(pt: PatternType) -> common_pb2.PatternType:
    """Convert PatternType enum to protobuf."""
    mapping = {
        PatternType.GLOB: common_pb2.PATTERN_TYPE_GLOB,
        PatternType.DIRECTORY: common_pb2.PATTERN_TYPE_DIRECTORY,
        PatternType.FILE: common_pb2.PATTERN_TYPE_FILE,
    }
    return mapping.get(pt, common_pb2.PATTERN_TYPE_UNSPECIFIED)


def proto_to_pattern_type(pt: common_pb2.PatternType) -> PatternType:
    """Convert protobuf PatternType to enum."""
    mapping = {
        common_pb2.PATTERN_TYPE_GLOB: PatternType.GLOB,
        common_pb2.PATTERN_TYPE_DIRECTORY: PatternType.DIRECTORY,
        common_pb2.PATTERN_TYPE_FILE: PatternType.FILE,
    }
    return mapping.get(pt, PatternType.GLOB)


# ============================================
# Status Conversions
# ============================================

def proto_to_status(status: common_pb2.SandboxStatus) -> SandboxStatus:
    """Convert protobuf SandboxStatus to enum."""
    mapping = {
        common_pb2.SANDBOX_STATUS_PENDING: SandboxStatus.PENDING,
        common_pb2.SANDBOX_STATUS_RUNNING: SandboxStatus.RUNNING,
        common_pb2.SANDBOX_STATUS_STOPPED: SandboxStatus.STOPPED,
        common_pb2.SANDBOX_STATUS_ERROR: SandboxStatus.ERROR,
    }
    return mapping.get(status, SandboxStatus.PENDING)


def proto_to_session_status(status: common_pb2.SessionStatus) -> SessionStatus:
    """Convert protobuf SessionStatus to enum."""
    mapping = {
        common_pb2.SESSION_STATUS_UNSPECIFIED: SessionStatus.UNKNOWN,
        common_pb2.SESSION_STATUS_ACTIVE: SessionStatus.ACTIVE,
        common_pb2.SESSION_STATUS_CLOSED: SessionStatus.CLOSED,
    }
    return mapping.get(status, SessionStatus.UNKNOWN)


# ============================================
# Runtime Conversions
# ============================================

def runtime_type_to_proto(rt: RuntimeType) -> sandbox_pb2.RuntimeType:
    """Convert RuntimeType enum to protobuf."""
    mapping = {
        RuntimeType.BWRAP: sandbox_pb2.RUNTIME_TYPE_BWRAP,
        RuntimeType.DOCKER: sandbox_pb2.RUNTIME_TYPE_DOCKER,
    }
    return mapping.get(rt, sandbox_pb2.RUNTIME_TYPE_UNSPECIFIED)


def proto_to_runtime_type(rt: sandbox_pb2.RuntimeType) -> RuntimeType:
    """Convert protobuf RuntimeType to enum."""
    mapping = {
        sandbox_pb2.RUNTIME_TYPE_BWRAP: RuntimeType.BWRAP,
        sandbox_pb2.RUNTIME_TYPE_DOCKER: RuntimeType.DOCKER,
    }
    return mapping.get(rt, RuntimeType.BWRAP)


# ============================================
# Resource Limits Conversions
# ============================================

def resource_limits_to_proto(
    limits: Optional[ResourceLimits],
) -> Optional[sandbox_pb2.ResourceLimits]:
    """Convert ResourceLimits to protobuf."""
    if limits is None:
        return None
    return sandbox_pb2.ResourceLimits(
        memory_bytes=limits.memory_bytes or 0,
        cpu_quota=limits.cpu_quota or 0,
        cpu_shares=limits.cpu_shares or 0,
        pids_limit=limits.pids_limit or 0,
    )


def proto_to_resource_limits(
    pb: Optional[sandbox_pb2.ResourceLimits],
) -> Optional[ResourceLimits]:
    """Convert protobuf ResourceLimits to dataclass."""
    if pb is None:
        return None
    # Check if any limit is set
    if not any([pb.memory_bytes, pb.cpu_quota, pb.cpu_shares, pb.pids_limit]):
        return None
    return ResourceLimits(
        memory_bytes=pb.memory_bytes if pb.memory_bytes else None,
        cpu_quota=pb.cpu_quota if pb.cpu_quota else None,
        cpu_shares=pb.cpu_shares if pb.cpu_shares else None,
        pids_limit=pb.pids_limit if pb.pids_limit else None,
    )


# ============================================
# Time Conversions
# ============================================

def timestamp_to_datetime(ts) -> Optional[datetime]:
    """Convert protobuf Timestamp to datetime."""
    if ts is None or ts.seconds == 0:
        return None
    return datetime.fromtimestamp(ts.seconds + ts.nanos / 1e9)


def duration_to_timedelta(d) -> Optional[timedelta]:
    """Convert protobuf Duration to timedelta."""
    if d is None:
        return None
    return timedelta(seconds=d.seconds, microseconds=d.nanos / 1000)


# ============================================
# Complex Object Conversions
# ============================================

def proto_to_sandbox(pb: sandbox_pb2.Sandbox) -> Sandbox:
    """Convert protobuf Sandbox to Sandbox dataclass."""
    permissions = [
        PermissionRule(
            pattern=p.pattern,
            permission=proto_to_permission(p.permission),
            type=proto_to_pattern_type(p.type),
            priority=p.priority,
        )
        for p in pb.permissions
    ]
    return Sandbox(
        id=pb.id,
        codebase_id=pb.codebase_id,
        status=proto_to_status(pb.status),
        permissions=permissions,
        labels=dict(pb.labels),
        runtime=proto_to_runtime_type(pb.runtime),
        image=pb.image if pb.image else None,
        resources=proto_to_resource_limits(pb.resources) if pb.HasField("resources") else None,
        created_at=timestamp_to_datetime(pb.created_at),
        started_at=timestamp_to_datetime(pb.started_at),
        stopped_at=timestamp_to_datetime(pb.stopped_at),
        expires_at=timestamp_to_datetime(pb.expires_at),
    )


def proto_to_codebase(pb: codebase_pb2.Codebase) -> Codebase:
    """Convert protobuf Codebase to Codebase dataclass."""
    return Codebase(
        id=pb.id,
        name=pb.name,
        owner_id=pb.owner_id,
        size=pb.size,
        file_count=pb.file_count,
        created_at=timestamp_to_datetime(pb.created_at),
        updated_at=timestamp_to_datetime(pb.updated_at),
    )


def proto_to_file_info(pb: codebase_pb2.FileInfo) -> FileInfo:
    """Convert protobuf FileInfo to FileInfo dataclass."""
    return FileInfo(
        path=pb.path,
        name=pb.name,
        is_dir=pb.is_dir,
        size=pb.size,
        modified_at=timestamp_to_datetime(pb.modified_at),
    )


def proto_to_session(pb: sandbox_pb2.Session) -> Session:
    """Convert protobuf Session to Session dataclass."""
    return Session(
        id=pb.id,
        sandbox_id=pb.sandbox_id,
        status=proto_to_session_status(pb.status),
        shell=pb.shell,
        created_at=timestamp_to_datetime(pb.created_at),
        closed_at=timestamp_to_datetime(pb.closed_at),
    )
