"""Async Sandbox SDK Client for interacting with the Sandbox service.

This module provides an asynchronous version of SandboxClient using grpc.aio.
"""

from datetime import timedelta
from functools import wraps
from typing import AsyncIterator, Callable, Dict, List, Optional, TypeVar, Union

import grpc
import grpc.aio
from google.protobuf.duration_pb2 import Duration

from .._gen import sandbox_pb2, sandbox_pb2_grpc
from .._gen import codebase_pb2, codebase_pb2_grpc
from .._gen import common_pb2
from ..types import (
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
from ..exceptions import (
    SandboxError,
    SandboxNotFoundError,
    CodebaseNotFoundError,
    CommandTimeoutError,
    ConnectionError,
    PermissionDeniedError,
    SessionNotFoundError,
    from_grpc_error,
)
from .._shared import (
    permission_to_proto,
    pattern_type_to_proto,
    runtime_type_to_proto,
    resource_limits_to_proto,
    duration_to_timedelta,
    proto_to_sandbox,
    proto_to_codebase,
    proto_to_file_info,
    proto_to_session,
)

# Type variable for decorator
T = TypeVar("T")


def _handle_grpc_errors_async(context: str = "") -> Callable:
    """Decorator to convert gRPC errors to SDK exceptions (async version).
    
    Args:
        context: Description of the operation for error messages.
    """
    def decorator(func: Callable[..., T]) -> Callable[..., T]:
        @wraps(func)
        async def wrapper(*args, **kwargs) -> T:
            try:
                return await func(*args, **kwargs)
            except grpc.aio.AioRpcError as e:
                raise from_grpc_error(e, context)
        return wrapper
    return decorator


class AsyncSessionWrapper:
    """Async wrapper for a shell session with context manager support.
    
    A session maintains a persistent shell process that preserves
    working directory, environment variables, and background processes.
    
    Example:
        >>> async with sandbox.session() as session:
        ...     await session.exec("cd /workspace")
        ...     await session.exec("npm install")
        ...     result = await session.exec("npm test")
    """
    
    def __init__(self, client: "AsyncSandboxClient", session: Session):
        """Initialize the AsyncSessionWrapper.
        
        Args:
            client: The AsyncSandboxClient instance.
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
    
    async def exec(
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
        return await self._client.session_exec(self.id, command, timeout)
    
    async def close(self):
        """Close the session and clean up all child processes."""
        await self._client.destroy_session(self.id)
    
    async def __aenter__(self) -> "AsyncSessionWrapper":
        """Enter async context manager."""
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Exit async context manager, closing the session."""
        await self.close()


class AsyncSandboxClient:
    """Async client for interacting with the Sandbox service.
    
    Example:
        >>> client = AsyncSandboxClient(endpoint="localhost:9000")
        >>> sandbox = await client.create_sandbox(
        ...     codebase_id="cb_123",
        ...     permissions=[
        ...         {"pattern": "/docs/**", "permission": "write"},
        ...         {"pattern": "**/*.py", "permission": "read"},
        ...     ]
        ... )
        >>> await client.start_sandbox(sandbox.id)
        >>> result = await client.exec(sandbox.id, command="ls -la /workspace")
        >>> print(result.stdout)
    """

    def __init__(self, endpoint: str = "localhost:9000", secure: bool = False):
        """Initialize the AsyncSandboxClient.
        
        Args:
            endpoint: The gRPC server endpoint (host:port).
            secure: Whether to use TLS for the connection.
        """
        if secure:
            self._channel = grpc.aio.secure_channel(endpoint, grpc.ssl_channel_credentials())
        else:
            self._channel = grpc.aio.insecure_channel(endpoint)
        
        self._sandbox_stub = sandbox_pb2_grpc.SandboxServiceStub(self._channel)
        self._codebase_stub = codebase_pb2_grpc.CodebaseServiceStub(self._channel)

    async def close(self):
        """Close the gRPC channel."""
        await self._channel.close()

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.close()

    # ============================================
    # Sandbox Operations
    # ============================================

    @_handle_grpc_errors_async("create sandbox")
    async def create_sandbox(
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
        """
        # Convert permissions to protobuf
        pb_permissions = []
        if permissions:
            for p in permissions:
                if isinstance(p, PermissionRule):
                    pb_permissions.append(common_pb2.PermissionRule(
                        pattern=p.pattern,
                        permission=permission_to_proto(p.permission),
                        type=pattern_type_to_proto(p.type),
                        priority=p.priority,
                    ))
                elif isinstance(p, dict):
                    perm = Permission(p.get("permission", "read"))
                    ptype = PatternType(p.get("type", "glob"))
                    pb_permissions.append(common_pb2.PermissionRule(
                        pattern=p["pattern"],
                        permission=permission_to_proto(perm),
                        type=pattern_type_to_proto(ptype),
                        priority=int(p.get("priority", 0)),
                    ))

        # Build request
        request = sandbox_pb2.CreateSandboxRequest(
            codebase_id=codebase_id,
            permissions=pb_permissions,
            labels=labels or {},
            runtime=runtime_type_to_proto(runtime),
            image=image or "",
        )
        
        if expires_in:
            request.expires_in.CopyFrom(Duration(
                seconds=int(expires_in.total_seconds()),
                nanos=int((expires_in.total_seconds() % 1) * 1e9),
            ))
        
        if resources:
            pb_resources = resource_limits_to_proto(resources)
            if pb_resources:
                request.resources.CopyFrom(pb_resources)

        response = await self._sandbox_stub.CreateSandbox(request)
        return proto_to_sandbox(response)

    @_handle_grpc_errors_async("get sandbox")
    async def get_sandbox(self, sandbox_id: str) -> Sandbox:
        """Get information about a sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox.
            
        Returns:
            The Sandbox object.
        """
        request = sandbox_pb2.GetSandboxRequest(sandbox_id=sandbox_id)
        response = await self._sandbox_stub.GetSandbox(request)
        return proto_to_sandbox(response)

    @_handle_grpc_errors_async("list sandboxes")
    async def list_sandboxes(self, codebase_id: Optional[str] = None) -> List[Sandbox]:
        """List all sandboxes.
        
        Args:
            codebase_id: Optional filter by codebase ID.
            
        Returns:
            List of Sandbox objects.
        """
        request = sandbox_pb2.ListSandboxesRequest(codebase_id=codebase_id or "")
        response = await self._sandbox_stub.ListSandboxes(request)
        return [proto_to_sandbox(sb) for sb in response.sandboxes]

    @_handle_grpc_errors_async("start sandbox")
    async def start_sandbox(self, sandbox_id: str) -> Sandbox:
        """Start a pending sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox to start.
            
        Returns:
            The updated Sandbox object.
        """
        request = sandbox_pb2.StartSandboxRequest(sandbox_id=sandbox_id)
        response = await self._sandbox_stub.StartSandbox(request)
        return proto_to_sandbox(response)

    @_handle_grpc_errors_async("stop sandbox")
    async def stop_sandbox(self, sandbox_id: str) -> Sandbox:
        """Stop a running sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox to stop.
            
        Returns:
            The updated Sandbox object.
        """
        request = sandbox_pb2.StopSandboxRequest(sandbox_id=sandbox_id)
        response = await self._sandbox_stub.StopSandbox(request)
        return proto_to_sandbox(response)

    @_handle_grpc_errors_async("destroy sandbox")
    async def destroy_sandbox(self, sandbox_id: str) -> None:
        """Destroy a sandbox and release resources.
        
        Args:
            sandbox_id: The ID of the sandbox to destroy.
        """
        request = sandbox_pb2.DestroySandboxRequest(sandbox_id=sandbox_id)
        await self._sandbox_stub.DestroySandbox(request)

    @_handle_grpc_errors_async("execute command")
    async def exec(
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

        response = await self._sandbox_stub.Exec(request)
        return ExecResult(
            stdout=response.stdout,
            stderr=response.stderr,
            exit_code=response.exit_code,
            duration=duration_to_timedelta(response.duration),
            command=command,
        )

    async def exec_stream(
        self,
        sandbox_id: str,
        command: str,
        stdin: Optional[str] = None,
        env: Optional[Dict[str, str]] = None,
        workdir: Optional[str] = None,
        timeout: Optional[timedelta] = None,
    ) -> AsyncIterator[bytes]:
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

        try:
            async for response in self._sandbox_stub.ExecStream(request):
                yield response.data
        except grpc.aio.AioRpcError as e:
            raise from_grpc_error(e, "execute command (stream)")

    # ============================================
    # Session Operations
    # ============================================

    @_handle_grpc_errors_async("create session")
    async def create_session(
        self,
        sandbox_id: str,
        shell: str = "/bin/sh",
        env: Optional[Dict[str, str]] = None,
    ) -> AsyncSessionWrapper:
        """Create a new shell session within a sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox.
            shell: The shell binary to use (default: /bin/sh).
            env: Optional initial environment variables.
            
        Returns:
            An AsyncSessionWrapper object for the new session.
        """
        request = sandbox_pb2.CreateSessionRequest(
            sandbox_id=sandbox_id,
            shell=shell,
            env=env or {},
        )
        response = await self._sandbox_stub.CreateSession(request)
        session = proto_to_session(response)
        return AsyncSessionWrapper(self, session)

    async def session(
        self,
        sandbox_id: str,
        shell: str = "/bin/sh",
        env: Optional[Dict[str, str]] = None,
    ) -> AsyncSessionWrapper:
        """Create a session with context manager support (alias for create_session).
        
        Example:
            >>> async with client.session(sandbox_id) as session:
            ...     await session.exec("cd /workspace")
            ...     await session.exec("npm install")
        """
        return await self.create_session(sandbox_id, shell, env)

    @_handle_grpc_errors_async("get session")
    async def get_session(self, session_id: str) -> Session:
        """Get information about a session.
        
        Args:
            session_id: The ID of the session.
            
        Returns:
            The Session object.
        """
        request = sandbox_pb2.GetSessionRequest(session_id=session_id)
        response = await self._sandbox_stub.GetSession(request)
        return proto_to_session(response)

    @_handle_grpc_errors_async("list sessions")
    async def list_sessions(self, sandbox_id: str) -> List[Session]:
        """List all sessions for a sandbox.
        
        Args:
            sandbox_id: The ID of the sandbox.
            
        Returns:
            List of Session objects.
        """
        request = sandbox_pb2.ListSessionsRequest(sandbox_id=sandbox_id)
        response = await self._sandbox_stub.ListSessions(request)
        return [proto_to_session(s) for s in response.sessions]

    @_handle_grpc_errors_async("destroy session")
    async def destroy_session(self, session_id: str) -> None:
        """Destroy a session and clean up all child processes.
        
        Args:
            session_id: The ID of the session to destroy.
        """
        request = sandbox_pb2.DestroySessionRequest(session_id=session_id)
        await self._sandbox_stub.DestroySession(request)

    @_handle_grpc_errors_async("session exec")
    async def session_exec(
        self,
        session_id: str,
        command: str,
        timeout: Optional[timedelta] = None,
    ) -> ExecResult:
        """Execute a command within a session (stateful).
        
        Args:
            session_id: The ID of the session.
            command: The command to execute.
            timeout: Optional timeout duration.
            
        Returns:
            The ExecResult with stdout, stderr, and exit code.
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

        response = await self._sandbox_stub.SessionExec(request)
        return ExecResult(
            stdout=response.stdout,
            stderr=response.stderr,
            exit_code=response.exit_code,
            duration=duration_to_timedelta(response.duration),
            command=command,
        )

    # ============================================
    # Codebase Operations
    # ============================================

    @_handle_grpc_errors_async("create codebase")
    async def create_codebase(self, name: str, owner_id: str) -> Codebase:
        """Create a new codebase.
        
        Args:
            name: The name of the codebase.
            owner_id: The ID of the owner.
            
        Returns:
            The created Codebase object.
        """
        request = codebase_pb2.CreateCodebaseRequest(name=name, owner_id=owner_id)
        response = await self._codebase_stub.CreateCodebase(request)
        return proto_to_codebase(response)

    @_handle_grpc_errors_async("get codebase")
    async def get_codebase(self, codebase_id: str) -> Codebase:
        """Get information about a codebase.
        
        Args:
            codebase_id: The ID of the codebase.
            
        Returns:
            The Codebase object.
        """
        request = codebase_pb2.GetCodebaseRequest(codebase_id=codebase_id)
        response = await self._codebase_stub.GetCodebase(request)
        return proto_to_codebase(response)

    @_handle_grpc_errors_async("list codebases")
    async def list_codebases(self, owner_id: Optional[str] = None) -> List[Codebase]:
        """List all codebases.
        
        Args:
            owner_id: Optional filter by owner ID.
            
        Returns:
            List of Codebase objects.
        """
        request = codebase_pb2.ListCodebasesRequest(owner_id=owner_id or "")
        response = await self._codebase_stub.ListCodebases(request)
        return [proto_to_codebase(cb) for cb in response.codebases]

    @_handle_grpc_errors_async("delete codebase")
    async def delete_codebase(self, codebase_id: str) -> None:
        """Delete a codebase.
        
        Args:
            codebase_id: The ID of the codebase to delete.
        """
        request = codebase_pb2.DeleteCodebaseRequest(codebase_id=codebase_id)
        await self._codebase_stub.DeleteCodebase(request)

    @_handle_grpc_errors_async("upload file")
    async def upload_file(
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
        """
        async def generate_chunks():
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

        response = await self._codebase_stub.UploadFiles(generate_chunks())
        return UploadResult(
            codebase_id=response.codebase_id,
            file_path=response.file_path,
            size=response.size,
            checksum=response.checksum,
        )

    @_handle_grpc_errors_async("download file")
    async def download_file(self, codebase_id: str, file_path: str) -> bytes:
        """Download a file from a codebase.
        
        Args:
            codebase_id: The ID of the codebase.
            file_path: The path of the file to download.
            
        Returns:
            The file content as bytes.
        """
        request = codebase_pb2.DownloadFileRequest(
            codebase_id=codebase_id,
            file_path=file_path,
        )
        chunks = []
        async for response in self._codebase_stub.DownloadFile(request):
            chunks.append(response.data)
        return b"".join(chunks)

    @_handle_grpc_errors_async("list files")
    async def list_files(
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
        """
        request = codebase_pb2.ListFilesRequest(
            codebase_id=codebase_id,
            path=path,
            recursive=recursive,
        )
        response = await self._codebase_stub.ListFiles(request)
        return [proto_to_file_info(f) for f in response.files]
