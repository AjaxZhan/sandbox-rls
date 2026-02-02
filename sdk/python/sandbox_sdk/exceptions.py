"""Exception classes for the Sandbox SDK.

This module defines a hierarchy of exceptions that provide meaningful
error messages and context for SDK operations, replacing raw gRPC exceptions.
"""

from typing import Optional


class SandboxError(Exception):
    """Base exception for all Sandbox SDK errors.
    
    All SDK-specific exceptions inherit from this class, making it easy
    to catch any SDK error with a single except clause.
    
    Attributes:
        message: Human-readable error description.
        details: Optional additional error context.
        grpc_code: Original gRPC status code if applicable.
    """
    
    def __init__(
        self,
        message: str,
        details: Optional[str] = None,
        grpc_code: Optional[int] = None,
    ):
        self.message = message
        self.details = details
        self.grpc_code = grpc_code
        super().__init__(self._format_message())
    
    def _format_message(self) -> str:
        msg = self.message
        if self.details:
            msg += f" Details: {self.details}"
        return msg


class ConnectionError(SandboxError):
    """Failed to connect to the sandbox service.
    
    Raised when the SDK cannot establish a connection to the gRPC server.
    This could be due to network issues, server being down, or incorrect endpoint.
    """
    
    def __init__(
        self,
        endpoint: str,
        details: Optional[str] = None,
        grpc_code: Optional[int] = None,
    ):
        self.endpoint = endpoint
        super().__init__(
            f"Failed to connect to sandbox service at '{endpoint}'",
            details=details,
            grpc_code=grpc_code,
        )


class SandboxNotFoundError(SandboxError):
    """The specified sandbox does not exist.
    
    Raised when attempting to access a sandbox that doesn't exist or has been deleted.
    """
    
    def __init__(self, sandbox_id: str, details: Optional[str] = None):
        self.sandbox_id = sandbox_id
        super().__init__(
            f"Sandbox '{sandbox_id}' not found",
            details=details,
        )


class SandboxNotRunningError(SandboxError):
    """The sandbox is not in a running state.
    
    Raised when attempting to execute commands or create sessions
    on a sandbox that hasn't been started or has been stopped.
    """
    
    def __init__(
        self,
        sandbox_id: str,
        current_status: str,
        details: Optional[str] = None,
    ):
        self.sandbox_id = sandbox_id
        self.current_status = current_status
        super().__init__(
            f"Sandbox '{sandbox_id}' is not running (current status: {current_status})",
            details=details,
        )


class CodebaseError(SandboxError):
    """Error related to codebase operations.
    
    Raised for errors during codebase creation, file upload/download, etc.
    """
    pass


class CodebaseNotFoundError(CodebaseError):
    """The specified codebase does not exist.
    
    Raised when attempting to access a codebase that doesn't exist or has been deleted.
    """
    
    def __init__(self, codebase_id: str, details: Optional[str] = None):
        self.codebase_id = codebase_id
        super().__init__(
            f"Codebase '{codebase_id}' not found",
            details=details,
        )


class FileNotFoundError(CodebaseError):
    """The specified file does not exist in the codebase.
    
    Raised when attempting to download or access a file that doesn't exist.
    """
    
    def __init__(
        self,
        codebase_id: str,
        file_path: str,
        details: Optional[str] = None,
    ):
        self.codebase_id = codebase_id
        self.file_path = file_path
        super().__init__(
            f"File '{file_path}' not found in codebase '{codebase_id}'",
            details=details,
        )


class CommandTimeoutError(SandboxError):
    """Command execution timed out.
    
    Raised when a command takes longer than the specified timeout duration.
    The command may still be running in the sandbox.
    """
    
    def __init__(
        self,
        command: str,
        timeout_seconds: float,
        details: Optional[str] = None,
    ):
        self.command = command
        self.timeout_seconds = timeout_seconds
        super().__init__(
            f"Command timed out after {timeout_seconds}s: {command[:100]}...",
            details=details,
        )


class CommandExecutionError(SandboxError):
    """Command execution failed.
    
    Raised when a command exits with a non-zero status code and
    raise_on_error is enabled.
    """
    
    def __init__(
        self,
        command: str,
        exit_code: int,
        stdout: str = "",
        stderr: str = "",
    ):
        self.command = command
        self.exit_code = exit_code
        self.stdout = stdout
        self.stderr = stderr
        
        # Build error message
        details = None
        if stderr:
            details = stderr[:500]
        elif stdout:
            details = stdout[:500]
            
        super().__init__(
            f"Command failed with exit code {exit_code}: {command[:100]}",
            details=details,
        )


class PermissionDeniedError(SandboxError):
    """Permission denied for the requested operation.
    
    Raised when the sandbox's permission rules prevent access to a file or operation.
    """
    
    def __init__(
        self,
        operation: str,
        path: Optional[str] = None,
        details: Optional[str] = None,
    ):
        self.operation = operation
        self.path = path
        
        if path:
            message = f"Permission denied for {operation} on '{path}'"
        else:
            message = f"Permission denied for {operation}"
            
        super().__init__(message, details=details)


class SessionError(SandboxError):
    """Error related to session operations."""
    pass


class SessionNotFoundError(SessionError):
    """The specified session does not exist.
    
    Raised when attempting to access a session that doesn't exist or has been closed.
    """
    
    def __init__(self, session_id: str, details: Optional[str] = None):
        self.session_id = session_id
        super().__init__(
            f"Session '{session_id}' not found",
            details=details,
        )


class SessionClosedError(SessionError):
    """The session has been closed.
    
    Raised when attempting to execute commands in a closed session.
    """
    
    def __init__(self, session_id: str, details: Optional[str] = None):
        self.session_id = session_id
        super().__init__(
            f"Session '{session_id}' is closed",
            details=details,
        )


class ResourceLimitExceededError(SandboxError):
    """Resource limit exceeded.
    
    Raised when a sandbox exceeds its allocated resources (memory, CPU, etc.).
    """
    
    def __init__(
        self,
        resource_type: str,
        limit: str,
        details: Optional[str] = None,
    ):
        self.resource_type = resource_type
        self.limit = limit
        super().__init__(
            f"Resource limit exceeded: {resource_type} limit is {limit}",
            details=details,
        )


class InvalidConfigurationError(SandboxError):
    """Invalid configuration provided.
    
    Raised when the provided configuration is invalid or inconsistent.
    """
    pass


class UploadError(CodebaseError):
    """File upload failed.
    
    Raised when uploading a file to a codebase fails.
    """
    
    def __init__(
        self,
        file_path: str,
        codebase_id: str,
        details: Optional[str] = None,
    ):
        self.file_path = file_path
        self.codebase_id = codebase_id
        super().__init__(
            f"Failed to upload '{file_path}' to codebase '{codebase_id}'",
            details=details,
        )


def from_grpc_error(error, context: Optional[str] = None) -> SandboxError:
    """Convert a gRPC error to an appropriate SandboxError.
    
    Args:
        error: The gRPC RpcError exception.
        context: Optional context about the operation that failed.
        
    Returns:
        An appropriate SandboxError subclass.
    """
    import grpc
    
    code = error.code() if hasattr(error, 'code') else None
    details = error.details() if hasattr(error, 'details') else str(error)
    
    if context:
        details = f"{context}. {details}"
    
    grpc_code = code.value if code else None
    
    # Map gRPC codes to SDK exceptions
    if code == grpc.StatusCode.NOT_FOUND:
        return SandboxError(f"Resource not found", details=details, grpc_code=grpc_code)
    elif code == grpc.StatusCode.PERMISSION_DENIED:
        return PermissionDeniedError("operation", details=details)
    elif code == grpc.StatusCode.DEADLINE_EXCEEDED:
        return CommandTimeoutError("unknown", 0, details=details)
    elif code == grpc.StatusCode.UNAVAILABLE:
        return ConnectionError("unknown", details=details, grpc_code=grpc_code)
    elif code == grpc.StatusCode.INVALID_ARGUMENT:
        return InvalidConfigurationError(details or "Invalid argument")
    elif code == grpc.StatusCode.RESOURCE_EXHAUSTED:
        return ResourceLimitExceededError("unknown", "unknown", details=details)
    else:
        return SandboxError(
            f"Operation failed",
            details=details,
            grpc_code=grpc_code,
        )
