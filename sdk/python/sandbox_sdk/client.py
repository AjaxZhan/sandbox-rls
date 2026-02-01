"""Sandbox SDK Client for interacting with the Sandbox service."""

from datetime import datetime, timedelta
from functools import wraps
from typing import Callable, Dict, Iterator, List, Optional, TypeVar, Union

import grpc
from google.protobuf.duration_pb2 import Duration

from ._gen import sandbox_pb2, sandbox_pb2_grpc
from ._gen import codebase_pb2, codebase_pb2_grpc
from ._gen import common_pb2
from .types import (
    Codebase,
    ExecResult,
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
    UploadResult,
)
from .exceptions import (
    SandboxError,
    SandboxNotFoundError,
    CodebaseNotFoundError,
    CommandTimeoutError,
    ConnectionError,
    PermissionDeniedError,
    SessionNotFoundError,
    from_grpc_error,
)

# Type variable for decorator
T = TypeVar("T")


def _handle_grpc_errors(context: str = "") -> Callable:
    """Decorator to convert gRPC errors to SDK exceptions.
    
    Args:
        context: Description of the operation for error messages.
    """
    def decorator(func: Callable[..., T]) -> Callable[..., T]:
        @wraps(func)
        def wrapper(*args, **kwargs) -> T:
            try:
                return func(*args, **kwargs)
            except grpc.RpcError as e:
                raise from_grpc_error(e, context)
        return wrapper
    return decorator


def _permission_to_proto(perm: Permission) -> common_pb2.Permission:
    """Convert Permission enum to protobuf."""
    mapping = {
        Permission.NONE: common_pb2.PERMISSION_NONE,
        Permission.VIEW: common_pb2.PERMISSION_VIEW,
        Permission.READ: common_pb2.PERMISSION_READ,
        Permission.WRITE: common_pb2.PERMISSION_WRITE,
    }
    return mapping.get(perm, common_pb2.PERMISSION_UNSPECIFIED)


def _proto_to_permission(perm: common_pb2.Permission) -> Permission:
    """Convert protobuf Permission to enum."""
    mapping = {
        common_pb2.PERMISSION_NONE: Permission.NONE,
        common_pb2.PERMISSION_VIEW: Permission.VIEW,
        common_pb2.PERMISSION_READ: Permission.READ,
        common_pb2.PERMISSION_WRITE: Permission.WRITE,
    }
    return mapping.get(perm, Permission.NONE)


def _pattern_type_to_proto(pt: PatternType) -> common_pb2.PatternType:
    """Convert PatternType enum to protobuf."""
    mapping = {
        PatternType.GLOB: common_pb2.PATTERN_TYPE_GLOB,
        PatternType.DIRECTORY: common_pb2.PATTERN_TYPE_DIRECTORY,
        PatternType.FILE: common_pb2.PATTERN_TYPE_FILE,
    }
    return mapping.get(pt, common_pb2.PATTERN_TYPE_UNSPECIFIED)


def _proto_to_pattern_type(pt: common_pb2.PatternType) -> PatternType:
    """Convert protobuf PatternType to enum."""
    mapping = {
        common_pb2.PATTERN_TYPE_GLOB: PatternType.GLOB,
        common_pb2.PATTERN_TYPE_DIRECTORY: PatternType.DIRECTORY,
        common_pb2.PATTERN_TYPE_FILE: PatternType.FILE,
    }
    return mapping.get(pt, PatternType.GLOB)


def _proto_to_status(status: common_pb2.SandboxStatus) -> SandboxStatus:
    """Convert protobuf SandboxStatus to enum."""
    mapping = {
        common_pb2.SANDBOX_STATUS_PENDING: SandboxStatus.PENDING,
        common_pb2.SANDBOX_STATUS_RUNNING: SandboxStatus.RUNNING,
        common_pb2.SANDBOX_STATUS_STOPPED: SandboxStatus.STOPPED,
        common_pb2.SANDBOX_STATUS_ERROR: SandboxStatus.ERROR,
    }
    return mapping.get(status, SandboxStatus.PENDING)


def _runtime_type_to_proto(rt: RuntimeType) -> sandbox_pb2.RuntimeType:
    """Convert RuntimeType enum to protobuf."""
    mapping = {
        RuntimeType.BWRAP: sandbox_pb2.RUNTIME_TYPE_BWRAP,
        RuntimeType.DOCKER: sandbox_pb2.RUNTIME_TYPE_DOCKER,
    }
    return mapping.get(rt, sandbox_pb2.RUNTIME_TYPE_UNSPECIFIED)


def _proto_to_runtime_type(rt: sandbox_pb2.RuntimeType) -> RuntimeType:
    """Convert protobuf RuntimeType to enum."""
    mapping = {
        sandbox_pb2.RUNTIME_TYPE_BWRAP: RuntimeType.BWRAP,
        sandbox_pb2.RUNTIME_TYPE_DOCKER: RuntimeType.DOCKER,
    }
    return mapping.get(rt, RuntimeType.BWRAP)


def _resource_limits_to_proto(
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


def _proto_to_resource_limits(
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


def _timestamp_to_datetime(ts) -> Optional[datetime]:
    """Convert protobuf Timestamp to datetime."""
    if ts is None or ts.seconds == 0:
        return None
    return datetime.fromtimestamp(ts.seconds + ts.nanos / 1e9)


def _duration_to_timedelta(d) -> Optional[timedelta]:
    """Convert protobuf Duration to timedelta."""
    if d is None:
        return None
    return timedelta(seconds=d.seconds, microseconds=d.nanos / 1000)


def _proto_to_sandbox(pb: sandbox_pb2.Sandbox) -> Sandbox:
    """Convert protobuf Sandbox to Sandbox dataclass."""
    permissions = [
        PermissionRule(
            pattern=p.pattern,
            permission=_proto_to_permission(p.permission),
            type=_proto_to_pattern_type(p.type),
            priority=p.priority,
        )
        for p in pb.permissions
    ]
    return Sandbox(
        id=pb.id,
        codebase_id=pb.codebase_id,
        status=_proto_to_status(pb.status),
        permissions=permissions,
        labels=dict(pb.labels),
        runtime=_proto_to_runtime_type(pb.runtime),
        image=pb.image if pb.image else None,
        resources=_proto_to_resource_limits(pb.resources) if pb.HasField("resources") else None,
        created_at=_timestamp_to_datetime(pb.created_at),
        started_at=_timestamp_to_datetime(pb.started_at),
        stopped_at=_timestamp_to_datetime(pb.stopped_at),
        expires_at=_timestamp_to_datetime(pb.expires_at),
    )


def _proto_to_codebase(pb: codebase_pb2.Codebase) -> Codebase:
    """Convert protobuf Codebase to Codebase dataclass."""
    return Codebase(
        id=pb.id,
        name=pb.name,
        owner_id=pb.owner_id,
        size=pb.size,
        file_count=pb.file_count,
        created_at=_timestamp_to_datetime(pb.created_at),
        updated_at=_timestamp_to_datetime(pb.updated_at),
    )


def _proto_to_file_info(pb: codebase_pb2.FileInfo) -> FileInfo:
    """Convert protobuf FileInfo to FileInfo dataclass."""
    return FileInfo(
        path=pb.path,
        name=pb.name,
        is_dir=pb.is_dir,
        size=pb.size,
        modified_at=_timestamp_to_datetime(pb.modified_at),
    )


def _proto_to_session_status(status: common_pb2.SessionStatus) -> SessionStatus:
    """Convert protobuf SessionStatus to enum."""
    mapping = {
        common_pb2.SESSION_STATUS_UNSPECIFIED: SessionStatus.UNKNOWN,
        common_pb2.SESSION_STATUS_ACTIVE: SessionStatus.ACTIVE,
        common_pb2.SESSION_STATUS_CLOSED: SessionStatus.CLOSED,
    }
    return mapping.get(status, SessionStatus.UNKNOWN)


def _proto_to_session(pb: sandbox_pb2.Session) -> Session:
    """Convert protobuf Session to Session dataclass."""
    return Session(
        id=pb.id,
        sandbox_id=pb.sandbox_id,
        status=_proto_to_session_status(pb.status),
        shell=pb.shell,
        created_at=_timestamp_to_datetime(pb.created_at),
        closed_at=_timestamp_to_datetime(pb.closed_at),
    )


class SessionWrapper:
    """Wrapper for a shell session with context manager support.
    
    A session maintains a persistent shell process that preserves
    working directory, environment variables, and background processes.
    
    Example:
        >>> with sandbox.session() as session:
        ...     session.exec("cd /workspace")
        ...     session.exec("npm install")
        ...     result = session.exec("npm test")
    """
    
    def __init__(self, client: "SandboxClient", session: Session):
        """Initialize the SessionWrapper.
        
        Args:
            client: The SandboxClient instance.
            session: The Session object.
        """
        self._client = client
        self._session = session
    
    @property
    def id(self) -> str:
        """Get the session ID."""
        return self._session.id
    
    @property
    def sandbox_id(self) -> str:
        """Get the sandbox ID."""
        return self._session.sandbox_id
    
    @property
    def status(self) -> SessionStatus:
        """Get the session status."""
        return self._session.status
    
    @property
    def shell(self) -> str:
        """Get the shell binary path."""
        return self._session.shell
    
    def exec(
        self,
        command: str,
        timeout: Optional[timedelta] = None,
    ) -> ExecResult:
        """Execute a command in the session.
        
        The command runs in the context of the persistent shell,
        so working directory and environment changes persist.
        
        Args:
            command: The command to execute.
            timeout: Optional timeout duration.
            
        Returns:
            The ExecResult with stdout, stderr, and exit code.
        """
        return self._client.session_exec(self.id, command, timeout)
    
    def close(self):
        """Close the session and clean up all child processes."""
        self._client.destroy_session(self.id)
    
    def __enter__(self) -> "SessionWrapper":
        """Enter context manager."""
        return self
    
    def __exit__(self, exc_type, exc_val, exc_tb):
        """Exit context manager, closing the session."""
        self.close()


class SandboxClient:
    """Client for interacting with the Sandbox service.
    
    Example:
        >>> client = SandboxClient(endpoint="localhost:9000")
        >>> sandbox = client.create_sandbox(
        ...     codebase_id="cb_123",
        ...     permissions=[
        ...         {"pattern": "/docs/**", "permission": "write"},
        ...         {"pattern": "**/*.py", "permission": "read"},
        ...     ]
        ... )
        >>> client.start_sandbox(sandbox.id)
        >>> result = client.exec(sandbox.id, command="ls -la /workspace")
        >>> print(result.stdout)
    """

    def __init__(self, endpoint: str = "localhost:9000", secure: bool = False):
        """Initialize the SandboxClient.
        
        Args:
            endpoint: The gRPC server endpoint (host:port).
            secure: Whether to use TLS for the connection.
        """
        if secure:
            self._channel = grpc.secure_channel(endpoint, grpc.ssl_channel_credentials())
        else:
            self._channel = grpc.insecure_channel(endpoint)
        
        self._sandbox_stub = sandbox_pb2_grpc.SandboxServiceStub(self._channel)
        self._codebase_stub = codebase_pb2_grpc.CodebaseServiceStub(self._channel)

    def close(self):
        """Close the gRPC channel."""
        self._channel.close()

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()

    # ============================================
    # Sandbox Operations
    # ============================================

    @_handle_grpc_errors("create sandbox")
    def create_sandbox(
        self,
        codebase_id: str,
        permissions: Optional[List[Union[PermissionRule, Dict]]] = None,
        labels: Optional[Dict[str, str]] = None,
        expires_in: Optional[timedelta] = None,
        runtime: RuntimeType = RuntimeType.BWRAP,
        image: Optional[str] = None,
        resources: Optional[ResourceLimits] = None,
    ) -> Sandbox:
        """Create a new sandbox.
        
        Args:
            codebase_id: The ID of the codebase to use.
            permissions: List of permission rules (PermissionRule or dict).
            labels: Optional labels for the sandbox.
            expires_in: Optional expiration duration.
            runtime: Runtime type (bwrap or docker). Default is bwrap.
            image: Docker image to use (required for docker runtime).
            resources: Resource limits (memory, CPU, processes).
            
        Returns:
            The created Sandbox object.
            
        Raises:
            SandboxError: If sandbox creation fails.
            InvalidConfigurationError: If configuration is invalid.
            
        Example:
            >>> sandbox = client.create_sandbox(
            ...     codebase_id="cb_123",
            ...     permissions=[{"pattern": "**/*", "permission": "read"}],
            ...     runtime=RuntimeType.DOCKER,
            ...     image="python:3.11-slim",
            ...     resources=ResourceLimits(memory_bytes=512*1024*1024),
            ... )
        """
        # Convert permissions to protobuf
        pb_permissions = []
        if permissions:
            for p in permissions:
                if isinstance(p, PermissionRule):
                    pb_permissions.append(common_pb2.PermissionRule(
                        pattern=p.pattern,
                        permission=_permission_to_proto(p.permission),
                        type=_pattern_type_to_proto(p.type),
                        priority=p.priority,
                    ))
                elif isinstance(p, dict):
                    perm = Permission(p.get("permission", "read"))
                    ptype = PatternType(p.get("type", "glob"))
                    pb_permissions.append(common_pb2.PermissionRule(
                        pattern=p["pattern"],
                        permission=_permission_to_proto(perm),
                        type=_pattern_type_to_proto(ptype),
                        priority=int(p.get("priority", 0)),
                    ))

        # Build request
        request = sandbox_pb2.CreateSandboxRequest(
            codebase_id=codebase_id,
            permissions=pb_permissions,
            labels=labels or {},
            runtime=_runtime_type_to_proto(runtime),
            image=image or "",
        )
        
        if expires_in:
            request.expires_in.CopyFrom(Duration(
                seconds=int(expires_in.total_seconds()),
                nanos=int((expires_in.total_seconds() % 1) * 1e9),
            ))
        
        if resources:
            pb_resources = _resource_limits_to_proto(resources)
            if pb_resources:
                request.resources.CopyFrom(pb_resources)

        response = self._sandbox_stub.CreateSandbox(request)
        return _proto_to_sandbox(response)

    @_handle_grpc_errors("get sandbox")
    def get_sandbox(self, sandbox_id: str) -> Sandbox:
        """Get information about a sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox.
            
        Returns:
            The Sandbox object.
            
        Raises:
            SandboxNotFoundError: If the sandbox doesn't exist.
        """
        request = sandbox_pb2.GetSandboxRequest(sandbox_id=sandbox_id)
        response = self._sandbox_stub.GetSandbox(request)
        return _proto_to_sandbox(response)

    @_handle_grpc_errors("list sandboxes")
    def list_sandboxes(self, codebase_id: Optional[str] = None) -> List[Sandbox]:
        """List all sandboxes.
        
        Args:
            codebase_id: Optional filter by codebase ID.
            
        Returns:
            List of Sandbox objects.
        """
        request = sandbox_pb2.ListSandboxesRequest(codebase_id=codebase_id or "")
        response = self._sandbox_stub.ListSandboxes(request)
        return [_proto_to_sandbox(sb) for sb in response.sandboxes]

    @_handle_grpc_errors("start sandbox")
    def start_sandbox(self, sandbox_id: str) -> Sandbox:
        """Start a pending sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox to start.
            
        Returns:
            The updated Sandbox object.
            
        Raises:
            SandboxNotFoundError: If the sandbox doesn't exist.
        """
        request = sandbox_pb2.StartSandboxRequest(sandbox_id=sandbox_id)
        response = self._sandbox_stub.StartSandbox(request)
        return _proto_to_sandbox(response)

    @_handle_grpc_errors("stop sandbox")
    def stop_sandbox(self, sandbox_id: str) -> Sandbox:
        """Stop a running sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox to stop.
            
        Returns:
            The updated Sandbox object.
            
        Raises:
            SandboxNotFoundError: If the sandbox doesn't exist.
        """
        request = sandbox_pb2.StopSandboxRequest(sandbox_id=sandbox_id)
        response = self._sandbox_stub.StopSandbox(request)
        return _proto_to_sandbox(response)

    @_handle_grpc_errors("destroy sandbox")
    def destroy_sandbox(self, sandbox_id: str) -> None:
        """Destroy a sandbox and release resources.
        
        Args:
            sandbox_id: The ID of the sandbox to destroy.
            
        Raises:
            SandboxNotFoundError: If the sandbox doesn't exist.
        """
        request = sandbox_pb2.DestroySandboxRequest(sandbox_id=sandbox_id)
        self._sandbox_stub.DestroySandbox(request)

    @_handle_grpc_errors("execute command")
    def exec(
        self,
        sandbox_id: str,
        command: str,
        stdin: Optional[str] = None,
        env: Optional[Dict[str, str]] = None,
        workdir: Optional[str] = None,
        timeout: Optional[timedelta] = None,
    ) -> ExecResult:
        """Execute a command in a sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox.
            command: The command to execute.
            stdin: Optional stdin input.
            env: Optional environment variables.
            workdir: Optional working directory.
            timeout: Optional timeout duration.
            
        Returns:
            The ExecResult with stdout, stderr, and exit code.
            
        Raises:
            SandboxNotFoundError: If the sandbox doesn't exist.
            SandboxNotRunningError: If the sandbox isn't running.
            CommandTimeoutError: If the command times out.
        """
        request = sandbox_pb2.ExecRequest(
            sandbox_id=sandbox_id,
            command=command,
            stdin=stdin or "",
            env=env or {},
            workdir=workdir or "",
        )
        
        if timeout:
            request.timeout.CopyFrom(Duration(
                seconds=int(timeout.total_seconds()),
                nanos=int((timeout.total_seconds() % 1) * 1e9),
            ))

        response = self._sandbox_stub.Exec(request)
        return ExecResult(
            stdout=response.stdout,
            stderr=response.stderr,
            exit_code=response.exit_code,
            duration=_duration_to_timedelta(response.duration),
        )

    @_handle_grpc_errors("execute command (stream)")
    def exec_stream(
        self,
        sandbox_id: str,
        command: str,
        stdin: Optional[str] = None,
        env: Optional[Dict[str, str]] = None,
        workdir: Optional[str] = None,
        timeout: Optional[timedelta] = None,
    ) -> Iterator[bytes]:
        """Execute a command and stream the output.
        
        Args:
            sandbox_id: The ID of the sandbox.
            command: The command to execute.
            stdin: Optional stdin input.
            env: Optional environment variables.
            workdir: Optional working directory.
            timeout: Optional timeout duration.
            
        Yields:
            Chunks of output data.
            
        Raises:
            SandboxNotFoundError: If the sandbox doesn't exist.
            SandboxNotRunningError: If the sandbox isn't running.
        """
        request = sandbox_pb2.ExecRequest(
            sandbox_id=sandbox_id,
            command=command,
            stdin=stdin or "",
            env=env or {},
            workdir=workdir or "",
        )
        
        if timeout:
            request.timeout.CopyFrom(Duration(
                seconds=int(timeout.total_seconds()),
                nanos=int((timeout.total_seconds() % 1) * 1e9),
            ))

        for response in self._sandbox_stub.ExecStream(request):
            yield response.data

    # ============================================
    # Session Operations
    # ============================================

    @_handle_grpc_errors("create session")
    def create_session(
        self,
        sandbox_id: str,
        shell: str = "/bin/bash",
        env: Optional[Dict[str, str]] = None,
    ) -> SessionWrapper:
        """Create a new shell session within a sandbox.
        
        A session maintains a persistent shell process that preserves
        working directory, environment variables, and background processes.
        
        Args:
            sandbox_id: The ID of the sandbox.
            shell: The shell binary to use (default: /bin/bash).
            env: Optional initial environment variables.
            
        Returns:
            A SessionWrapper object for the new session.
            
        Raises:
            SandboxNotFoundError: If the sandbox doesn't exist.
            SandboxNotRunningError: If the sandbox isn't running.
            
        Example:
            >>> session = client.create_session(sandbox_id)
            >>> session.exec("cd /workspace")
            >>> session.exec("npm install")
            >>> session.close()
        """
        request = sandbox_pb2.CreateSessionRequest(
            sandbox_id=sandbox_id,
            shell=shell,
            env=env or {},
        )
        response = self._sandbox_stub.CreateSession(request)
        session = _proto_to_session(response)
        return SessionWrapper(self, session)

    def session(
        self,
        sandbox_id: str,
        shell: str = "/bin/bash",
        env: Optional[Dict[str, str]] = None,
    ) -> SessionWrapper:
        """Create a session with context manager support (alias for create_session).
        
        Example:
            >>> with client.session(sandbox_id) as session:
            ...     session.exec("cd /workspace")
            ...     session.exec("npm install")
        """
        return self.create_session(sandbox_id, shell, env)

    @_handle_grpc_errors("get session")
    def get_session(self, session_id: str) -> Session:
        """Get information about a session.
        
        Args:
            session_id: The ID of the session.
            
        Returns:
            The Session object.
            
        Raises:
            SessionNotFoundError: If the session doesn't exist.
        """
        request = sandbox_pb2.GetSessionRequest(session_id=session_id)
        response = self._sandbox_stub.GetSession(request)
        return _proto_to_session(response)

    @_handle_grpc_errors("list sessions")
    def list_sessions(self, sandbox_id: str) -> List[Session]:
        """List all sessions for a sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox.
            
        Returns:
            List of Session objects.
        """
        request = sandbox_pb2.ListSessionsRequest(sandbox_id=sandbox_id)
        response = self._sandbox_stub.ListSessions(request)
        return [_proto_to_session(s) for s in response.sessions]

    @_handle_grpc_errors("destroy session")
    def destroy_session(self, session_id: str) -> None:
        """Destroy a session and clean up all child processes.
        
        Args:
            session_id: The ID of the session to destroy.
            
        Raises:
            SessionNotFoundError: If the session doesn't exist.
        """
        request = sandbox_pb2.DestroySessionRequest(session_id=session_id)
        self._sandbox_stub.DestroySession(request)

    @_handle_grpc_errors("session exec")
    def session_exec(
        self,
        session_id: str,
        command: str,
        timeout: Optional[timedelta] = None,
    ) -> ExecResult:
        """Execute a command within a session (stateful).
        
        The command runs in the context of the persistent shell,
        so working directory and environment changes persist.
        
        Args:
            session_id: The ID of the session.
            command: The command to execute.
            timeout: Optional timeout duration.
            
        Returns:
            The ExecResult with stdout, stderr, and exit code.
            
        Raises:
            SessionNotFoundError: If the session doesn't exist.
            SessionClosedError: If the session is closed.
            CommandTimeoutError: If the command times out.
        """
        request = sandbox_pb2.SessionExecRequest(
            session_id=session_id,
            command=command,
        )
        
        if timeout:
            request.timeout.CopyFrom(Duration(
                seconds=int(timeout.total_seconds()),
                nanos=int((timeout.total_seconds() % 1) * 1e9),
            ))

        response = self._sandbox_stub.SessionExec(request)
        return ExecResult(
            stdout=response.stdout,
            stderr=response.stderr,
            exit_code=response.exit_code,
            duration=_duration_to_timedelta(response.duration),
        )

    # ============================================
    # Codebase Operations
    # ============================================

    @_handle_grpc_errors("create codebase")
    def create_codebase(self, name: str, owner_id: str) -> Codebase:
        """Create a new codebase.
        
        Args:
            name: The name of the codebase.
            owner_id: The ID of the owner.
            
        Returns:
            The created Codebase object.
        """
        request = codebase_pb2.CreateCodebaseRequest(name=name, owner_id=owner_id)
        response = self._codebase_stub.CreateCodebase(request)
        return _proto_to_codebase(response)

    @_handle_grpc_errors("get codebase")
    def get_codebase(self, codebase_id: str) -> Codebase:
        """Get information about a codebase.
        
        Args:
            codebase_id: The ID of the codebase.
            
        Returns:
            The Codebase object.
            
        Raises:
            CodebaseNotFoundError: If the codebase doesn't exist.
        """
        request = codebase_pb2.GetCodebaseRequest(codebase_id=codebase_id)
        response = self._codebase_stub.GetCodebase(request)
        return _proto_to_codebase(response)

    @_handle_grpc_errors("list codebases")
    def list_codebases(self, owner_id: Optional[str] = None) -> List[Codebase]:
        """List all codebases.
        
        Args:
            owner_id: Optional filter by owner ID.
            
        Returns:
            List of Codebase objects.
        """
        request = codebase_pb2.ListCodebasesRequest(owner_id=owner_id or "")
        response = self._codebase_stub.ListCodebases(request)
        return [_proto_to_codebase(cb) for cb in response.codebases]

    @_handle_grpc_errors("delete codebase")
    def delete_codebase(self, codebase_id: str) -> None:
        """Delete a codebase.
        
        Args:
            codebase_id: The ID of the codebase to delete.
            
        Raises:
            CodebaseNotFoundError: If the codebase doesn't exist.
        """
        request = codebase_pb2.DeleteCodebaseRequest(codebase_id=codebase_id)
        self._codebase_stub.DeleteCodebase(request)

    @_handle_grpc_errors("upload file")
    def upload_file(
        self,
        codebase_id: str,
        file_path: str,
        content: bytes,
        chunk_size: int = 64 * 1024,
    ) -> UploadResult:
        """Upload a file to a codebase.
        
        Args:
            codebase_id: The ID of the codebase.
            file_path: The path where the file should be stored.
            content: The file content as bytes.
            chunk_size: Size of upload chunks (default 64KB).
            
        Returns:
            The UploadResult with file info.
            
        Raises:
            CodebaseNotFoundError: If the codebase doesn't exist.
            UploadError: If the upload fails.
        """
        def generate_chunks():
            # First send metadata
            yield codebase_pb2.UploadChunk(
                metadata=codebase_pb2.UploadChunk.Metadata(
                    codebase_id=codebase_id,
                    file_path=file_path,
                    total_size=len(content),
                )
            )
            # Then send data chunks
            for i in range(0, len(content), chunk_size):
                yield codebase_pb2.UploadChunk(data=content[i:i + chunk_size])

        response = self._codebase_stub.UploadFiles(generate_chunks())
        return UploadResult(
            codebase_id=response.codebase_id,
            file_path=response.file_path,
            size=response.size,
            checksum=response.checksum,
        )

    @_handle_grpc_errors("download file")
    def download_file(self, codebase_id: str, file_path: str) -> bytes:
        """Download a file from a codebase.
        
        Args:
            codebase_id: The ID of the codebase.
            file_path: The path of the file to download.
            
        Returns:
            The file content as bytes.
            
        Raises:
            CodebaseNotFoundError: If the codebase doesn't exist.
            FileNotFoundError: If the file doesn't exist.
        """
        request = codebase_pb2.DownloadFileRequest(
            codebase_id=codebase_id,
            file_path=file_path,
        )
        chunks = []
        for response in self._codebase_stub.DownloadFile(request):
            chunks.append(response.data)
        return b"".join(chunks)

    @_handle_grpc_errors("list files")
    def list_files(
        self,
        codebase_id: str,
        path: str = "",
        recursive: bool = False,
    ) -> List[FileInfo]:
        """List files in a codebase directory.
        
        Args:
            codebase_id: The ID of the codebase.
            path: The directory path (empty for root).
            recursive: Whether to list recursively.
            
        Returns:
            List of FileInfo objects.
            
        Raises:
            CodebaseNotFoundError: If the codebase doesn't exist.
        """
        request = codebase_pb2.ListFilesRequest(
            codebase_id=codebase_id,
            path=path,
            recursive=recursive,
        )
        response = self._codebase_stub.ListFiles(request)
        return [_proto_to_file_info(f) for f in response.files]
