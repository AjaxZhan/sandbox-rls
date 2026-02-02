"""Async high-level Sandbox API for easy sandbox management.

This module provides an asynchronous user-friendly interface for creating and
managing sandboxes with minimal boilerplate.
"""

from datetime import timedelta
from pathlib import Path
from typing import AsyncIterator, Dict, List, Optional, Union

from .client import AsyncSandboxClient, AsyncSessionWrapper
from ..exceptions import SandboxError, SandboxNotRunningError
from ..presets import get_preset, get_preset_dicts
from ..types import (
    Codebase,
    ExecResult,
    Permission,
    PermissionRule,
    ResourceLimits,
    RuntimeType,
    Sandbox as SandboxInfo,
    SandboxStatus,
)
from ..utils import (
    generate_codebase_name,
    generate_owner_id,
    human_readable_size,
    walk_directory,
)


class AsyncSandbox:
    """Async high-level sandbox interface with context manager support.
    
    This class provides an asynchronous simplified API for working with sandboxes,
    handling the complexity of codebase creation, file upload, and
    sandbox lifecycle management automatically.
    
    Example:
        >>> # One-liner to create a sandbox from local directory
        >>> async with await AsyncSandbox.from_local("./my-project") as sandbox:
        ...     result = await sandbox.run("python main.py")
        ...     print(result.stdout)
        
        >>> # With custom configuration
        >>> async with await AsyncSandbox.from_local(
        ...     "./my-project",
        ...     preset="agent-safe",
        ...     runtime=RuntimeType.DOCKER,
        ...     image="python:3.11-slim",
        ... ) as sandbox:
        ...     async with sandbox.session() as session:
        ...         await session.exec("pip install -r requirements.txt")
        ...         result = await session.exec("pytest")
    """
    
    def __init__(
        self,
        client: AsyncSandboxClient,
        sandbox_info: SandboxInfo,
        codebase: Codebase,
        owns_client: bool = False,
        owns_codebase: bool = False,
    ):
        """Initialize an AsyncSandbox instance.
        
        Note: Use the class methods (from_local, from_codebase) instead
        of calling this constructor directly.
        
        Args:
            client: The AsyncSandboxClient instance.
            sandbox_info: The Sandbox info object.
            codebase: The associated Codebase.
            owns_client: Whether this Sandbox owns the client (should close it).
            owns_codebase: Whether this Sandbox owns the codebase (should delete it).
        """
        self._client = client
        self._sandbox_info = sandbox_info
        self._codebase = codebase
        self._owns_client = owns_client
        self._owns_codebase = owns_codebase
        self._destroyed = False
    
    @classmethod
    async def from_local(
        cls,
        path: str,
        preset: Optional[str] = "view-only",
        permissions: Optional[List[Union[PermissionRule, Dict]]] = None,
        runtime: RuntimeType = RuntimeType.BWRAP,
        image: Optional[str] = None,
        resources: Optional[ResourceLimits] = None,
        endpoint: str = "localhost:9000",
        secure: bool = False,
        owner_id: Optional[str] = None,
        codebase_name: Optional[str] = None,
        ignore_patterns: Optional[List[str]] = None,
        labels: Optional[Dict[str, str]] = None,
        auto_start: bool = True,
    ) -> "AsyncSandbox":
        """Create a sandbox from a local directory.
        
        This is the recommended way to create a sandbox. It automatically:
        1. Creates a codebase
        2. Uploads all files from the directory
        3. Creates a sandbox with the specified permissions
        4. Starts the sandbox (if auto_start is True)
        
        Args:
            path: Path to the local directory.
            preset: Permission preset name ("view-only", "agent-safe", "read-only", "full-access").
            permissions: Additional permission rules (added to preset).
            runtime: Runtime type (bwrap or docker).
            image: Docker image name (required for docker runtime).
            resources: Resource limits (memory, CPU, etc.).
            endpoint: Sandbox service endpoint.
            secure: Whether to use TLS.
            owner_id: Owner ID for the codebase (auto-generated if not provided).
            codebase_name: Name for the codebase (derived from path if not provided).
            ignore_patterns: Additional file patterns to ignore during upload.
            labels: Labels to attach to the sandbox.
            auto_start: Whether to automatically start the sandbox.
            
        Returns:
            An AsyncSandbox instance ready for use.
            
        Raises:
            ValueError: If the path doesn't exist or isn't a directory.
            SandboxError: If sandbox creation fails.
            
        Example:
            >>> sandbox = await AsyncSandbox.from_local("./my-project")
            >>> result = await sandbox.run("ls -la")
            >>> print(result.stdout)
            >>> await sandbox.destroy()
        """
        # Validate path
        dir_path = Path(path).resolve()
        if not dir_path.exists():
            raise ValueError(f"Path does not exist: {path}")
        if not dir_path.is_dir():
            raise ValueError(f"Path is not a directory: {path}")

        # Normalize preset: treat None as default
        if preset is None:
            preset = "view-only"
        
        # Create client
        client = AsyncSandboxClient(endpoint=endpoint, secure=secure)
        
        codebase = None
        sandbox_info = None
        try:
            # Generate defaults
            if owner_id is None:
                owner_id = generate_owner_id()
            if codebase_name is None:
                codebase_name = generate_codebase_name(path)
            
            # Create codebase
            codebase = await client.create_codebase(name=codebase_name, owner_id=owner_id)
            
            # Upload files
            file_count = 0
            total_size = 0
            for rel_path, content in walk_directory(str(dir_path), ignore_patterns):
                await client.upload_file(codebase.id, rel_path, content)
                file_count += 1
                total_size += len(content)
            
            # Build permissions: preset + custom
            perm_rules: List[Union[PermissionRule, Dict]] = []
            perm_rules.extend(get_preset_dicts(preset))
            
            if permissions:
                perm_rules.extend(permissions)
            
            # Create sandbox
            sandbox_info = await client.create_sandbox(
                codebase_id=codebase.id,
                permissions=perm_rules,
                runtime=runtime,
                image=image,
                resources=resources,
                labels=labels,
            )
            
            # Start sandbox if requested
            if auto_start:
                sandbox_info = await client.start_sandbox(sandbox_info.id)
            
            return cls(
                client=client,
                sandbox_info=sandbox_info,
                codebase=codebase,
                owns_client=True,
                owns_codebase=True,
            )
            
        except Exception:
            # Clean up sandbox if it was created
            if sandbox_info is not None:
                try:
                    await client.destroy_sandbox(sandbox_info.id)
                except Exception:
                    pass  # Best effort cleanup
            # Clean up codebase if it was created
            if codebase is not None:
                try:
                    await client.delete_codebase(codebase.id)
                except Exception:
                    pass  # Best effort cleanup
            await client.close()
            raise
    
    @classmethod
    async def from_codebase(
        cls,
        codebase_id: str,
        preset: Optional[str] = "view-only",
        permissions: Optional[List[Union[PermissionRule, Dict]]] = None,
        runtime: RuntimeType = RuntimeType.BWRAP,
        image: Optional[str] = None,
        resources: Optional[ResourceLimits] = None,
        endpoint: str = "localhost:9000",
        secure: bool = False,
        labels: Optional[Dict[str, str]] = None,
        auto_start: bool = True,
    ) -> "AsyncSandbox":
        """Create a sandbox from an existing codebase.
        
        Use this when you want to create multiple sandboxes from the same
        codebase, or when the codebase already exists.
        
        Args:
            codebase_id: ID of the existing codebase.
            preset: Permission preset name.
            permissions: Additional permission rules.
            runtime: Runtime type (bwrap or docker).
            image: Docker image name (for docker runtime).
            resources: Resource limits.
            endpoint: Sandbox service endpoint.
            secure: Whether to use TLS.
            labels: Labels to attach to the sandbox.
            auto_start: Whether to automatically start the sandbox.
            
        Returns:
            An AsyncSandbox instance ready for use.
        """
        client = AsyncSandboxClient(endpoint=endpoint, secure=secure)

        # Normalize preset: treat None as default
        if preset is None:
            preset = "view-only"
        
        sandbox_info = None
        try:
            # Get codebase info
            codebase = await client.get_codebase(codebase_id)
            
            # Build permissions
            perm_rules: List[Union[PermissionRule, Dict]] = []
            perm_rules.extend(get_preset_dicts(preset))
            if permissions:
                perm_rules.extend(permissions)
            
            # Create sandbox
            sandbox_info = await client.create_sandbox(
                codebase_id=codebase_id,
                permissions=perm_rules,
                runtime=runtime,
                image=image,
                resources=resources,
                labels=labels,
            )
            
            if auto_start:
                sandbox_info = await client.start_sandbox(sandbox_info.id)
            
            return cls(
                client=client,
                sandbox_info=sandbox_info,
                codebase=codebase,
                owns_client=True,
                owns_codebase=False,  # Don't delete existing codebase
            )
            
        except Exception:
            # Clean up sandbox if it was created
            if sandbox_info is not None:
                try:
                    await client.destroy_sandbox(sandbox_info.id)
                except Exception:
                    pass  # Best effort cleanup
            await client.close()
            raise
    
    @classmethod
    async def connect(
        cls,
        sandbox_id: str,
        endpoint: str = "localhost:9000",
        secure: bool = False,
    ) -> "AsyncSandbox":
        """Connect to an existing sandbox.
        
        Use this to reconnect to a sandbox that was created earlier.
        
        Args:
            sandbox_id: ID of the existing sandbox.
            endpoint: Sandbox service endpoint.
            secure: Whether to use TLS.
            
        Returns:
            An AsyncSandbox instance connected to the existing sandbox.
        """
        client = AsyncSandboxClient(endpoint=endpoint, secure=secure)
        
        try:
            # Get sandbox info
            sandbox_info = await client.get_sandbox(sandbox_id)
            
            # Get codebase info
            codebase = await client.get_codebase(sandbox_info.codebase_id)
            
            return cls(
                client=client,
                sandbox_info=sandbox_info,
                codebase=codebase,
                owns_client=True,
                owns_codebase=False,
            )
            
        except Exception as e:
            await client.close()
            raise
    
    # ============================================
    # Properties
    # ============================================
    
    @property
    def id(self) -> str:
        """Get the sandbox ID."""
        return self._sandbox_info.id
    
    @property
    def codebase_id(self) -> str:
        """Get the associated codebase ID."""
        return self._codebase.id
    
    @property
    def status(self) -> SandboxStatus:
        """Get the current sandbox status."""
        return self._sandbox_info.status
    
    @property
    def runtime(self) -> RuntimeType:
        """Get the runtime type."""
        return self._sandbox_info.runtime
    
    @property
    def info(self) -> SandboxInfo:
        """Get the full sandbox info object."""
        return self._sandbox_info
    
    @property
    def codebase(self) -> Codebase:
        """Get the associated codebase."""
        return self._codebase
    
    # ============================================
    # Lifecycle Methods
    # ============================================
    
    async def start(self) -> "AsyncSandbox":
        """Start the sandbox if it's not already running.
        
        Returns:
            Self for method chaining.
        """
        if self._sandbox_info.status != SandboxStatus.RUNNING:
            self._sandbox_info = await self._client.start_sandbox(self.id)
        return self
    
    async def stop(self) -> "AsyncSandbox":
        """Stop the sandbox.
        
        Returns:
            Self for method chaining.
        """
        if self._sandbox_info.status == SandboxStatus.RUNNING:
            self._sandbox_info = await self._client.stop_sandbox(self.id)
        return self
    
    async def refresh(self) -> "AsyncSandbox":
        """Refresh sandbox info from the server.
        
        Returns:
            Self for method chaining.
        """
        self._sandbox_info = await self._client.get_sandbox(self.id)
        return self
    
    async def destroy(self, delete_codebase: Optional[bool] = None) -> None:
        """Destroy the sandbox and optionally the codebase.
        
        Args:
            delete_codebase: Whether to delete the codebase.
                If None, deletes only if this Sandbox created it.
        """
        if self._destroyed:
            return
        
        try:
            # Destroy sandbox
            await self._client.destroy_sandbox(self.id)
            
            # Delete codebase if we own it
            should_delete = delete_codebase if delete_codebase is not None else self._owns_codebase
            if should_delete:
                await self._client.delete_codebase(self.codebase_id)
            
        finally:
            # Close client if we own it
            if self._owns_client:
                await self._client.close()
            
            self._destroyed = True
    
    # ============================================
    # Execution Methods
    # ============================================
    
    async def run(
        self,
        command: str,
        timeout: int = 60,
        env: Optional[Dict[str, str]] = None,
        workdir: Optional[str] = None,
        raise_on_error: bool = False,
    ) -> ExecResult:
        """Execute a command in the sandbox.
        
        This is a simplified wrapper around exec() with sensible defaults.
        
        Args:
            command: The command to execute.
            timeout: Timeout in seconds (default: 60).
            env: Environment variables.
            workdir: Working directory.
            raise_on_error: Whether to raise an exception on non-zero exit.
            
        Returns:
            The ExecResult with stdout, stderr, and exit code.
            
        Raises:
            SandboxNotRunningError: If the sandbox isn't running.
            CommandExecutionError: If raise_on_error is True and command fails.
            CommandTimeoutError: If the command times out.
            
        Example:
            >>> result = await sandbox.run("python --version")
            >>> print(result.stdout)
            Python 3.11.0
        """
        from ..exceptions import CommandExecutionError
        
        result = await self._client.exec(
            sandbox_id=self.id,
            command=command,
            timeout=timedelta(seconds=timeout),
            env=env,
            workdir=workdir,
        )
        
        if raise_on_error and result.exit_code != 0:
            raise CommandExecutionError(
                command=command,
                exit_code=result.exit_code,
                stdout=result.stdout,
                stderr=result.stderr,
            )
        
        return result
    
    async def exec(
        self,
        command: str,
        stdin: Optional[str] = None,
        env: Optional[Dict[str, str]] = None,
        workdir: Optional[str] = None,
        timeout: Optional[timedelta] = None,
    ) -> ExecResult:
        """Execute a command in the sandbox (full API).
        
        Args:
            command: The command to execute.
            stdin: Optional stdin input.
            env: Environment variables.
            workdir: Working directory.
            timeout: Timeout duration.
            
        Returns:
            The ExecResult with stdout, stderr, and exit code.
        """
        return await self._client.exec(
            sandbox_id=self.id,
            command=command,
            stdin=stdin,
            env=env,
            workdir=workdir,
            timeout=timeout,
        )
    
    async def exec_stream(
        self,
        command: str,
        stdin: Optional[str] = None,
        env: Optional[Dict[str, str]] = None,
        workdir: Optional[str] = None,
        timeout: Optional[timedelta] = None,
    ) -> AsyncIterator[bytes]:
        """Execute a command and stream the output.
        
        Args:
            command: The command to execute.
            stdin: Optional stdin input.
            env: Environment variables.
            workdir: Working directory.
            timeout: Timeout duration.
            
        Yields:
            Chunks of output data.
        """
        async for chunk in self._client.exec_stream(
            sandbox_id=self.id,
            command=command,
            stdin=stdin,
            env=env,
            workdir=workdir,
            timeout=timeout,
        ):
            yield chunk
    
    # ============================================
    # Session Methods
    # ============================================
    
    async def session(
        self,
        shell: str = "/bin/sh",
        env: Optional[Dict[str, str]] = None,
    ) -> AsyncSessionWrapper:
        """Create a new shell session.
        
        A session maintains a persistent shell process that preserves
        working directory, environment variables, and background processes.
        
        Args:
            shell: The shell binary to use.
            env: Initial environment variables.
            
        Returns:
            An AsyncSessionWrapper for the new session.
            
        Example:
            >>> async with sandbox.session() as session:
            ...     await session.exec("cd /workspace")
            ...     await session.exec("source venv/bin/activate")
            ...     result = await session.exec("python main.py")
        """
        return await self._client.create_session(
            sandbox_id=self.id,
            shell=shell,
            env=env,
        )
    
    # ============================================
    # File Operations
    # ============================================
    
    @staticmethod
    def _to_codebase_path(path: str) -> str:
        """Map a sandbox path (usually under /workspace) to a codebase path."""
        if path == "/workspace":
            return ""
        if path.startswith("/workspace/"):
            return path[len("/workspace/") :]
        if path.startswith("/"):
            return path[1:]
        return path
    
    async def read_file(self, path: str) -> str:
        """Read a file from the sandbox.
        
        Args:
            path: Path to the file in the sandbox.
            
        Returns:
            The file content as a string.
            
        Example:
            >>> content = await sandbox.read_file("/workspace/output.txt")
        """
        content = await self._client.download_file(self.codebase_id, self._to_codebase_path(path))
        return content.decode("utf-8")
    
    async def read_file_bytes(self, path: str) -> bytes:
        """Read a file as bytes from the sandbox.
        
        Args:
            path: Path to the file in the sandbox.
            
        Returns:
            The file content as bytes.
        """
        return await self._client.download_file(self.codebase_id, self._to_codebase_path(path))
    
    async def write_file(self, path: str, content: Union[str, bytes]) -> None:
        """Write a file to the sandbox.
        
        Args:
            path: Path where the file should be stored.
            content: The file content (string or bytes).
            
        Example:
            >>> await sandbox.write_file("/workspace/config.json", '{"debug": true}')
        """
        if isinstance(content, str):
            content = content.encode("utf-8")
        await self._client.upload_file(self.codebase_id, self._to_codebase_path(path), content)
    
    async def list_files(self, path: str = "", recursive: bool = False) -> List[str]:
        """List files in the sandbox.
        
        Args:
            path: Directory path (empty for root).
            recursive: Whether to list recursively.
            
        Returns:
            List of file paths.
        """
        files = await self._client.list_files(
            codebase_id=self.codebase_id,
            path=self._to_codebase_path(path),
            recursive=recursive,
        )
        return [f.path for f in files]
    
    # ============================================
    # Context Manager
    # ============================================
    
    async def __aenter__(self) -> "AsyncSandbox":
        """Enter async context manager."""
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb) -> None:
        """Exit async context manager, destroying the sandbox."""
        await self.destroy()
    
    def __repr__(self) -> str:
        return (
            f"AsyncSandbox(id={self.id!r}, status={self.status.value!r}, "
            f"runtime={self.runtime.value!r})"
        )
