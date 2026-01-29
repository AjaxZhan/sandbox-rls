"""Sandbox SDK - Python client for the Sandbox RLS service.

Example:
    >>> from sandbox_sdk import SandboxClient
    >>> 
    >>> client = SandboxClient(endpoint="localhost:9000")
    >>> 
    >>> # Create a codebase
    >>> codebase = client.create_codebase(name="my-project", owner_id="user_123")
    >>> 
    >>> # Upload a file
    >>> client.upload_file(codebase.id, "hello.py", b'print("Hello, World!")')
    >>> 
    >>> # Create a sandbox with permissions
    >>> sandbox = client.create_sandbox(
    ...     codebase_id=codebase.id,
    ...     permissions=[
    ...         {"pattern": "**/*.py", "permission": "read"},
    ...         {"pattern": "/docs/**", "permission": "write"},
    ...     ]
    ... )
    >>> 
    >>> # Start the sandbox
    >>> client.start_sandbox(sandbox.id)
    >>> 
    >>> # Execute a command
    >>> result = client.exec(sandbox.id, command="python hello.py")
    >>> print(result.stdout)  # Output: Hello, World!
    >>> 
    >>> # Clean up
    >>> client.destroy_sandbox(sandbox.id)
    >>> client.delete_codebase(codebase.id)
"""

from .client import SandboxClient
from .types import (
    Codebase,
    ExecResult,
    FileInfo,
    Permission,
    PatternType,
    PermissionRule,
    Sandbox,
    SandboxStatus,
    UploadResult,
)

__version__ = "0.1.0"
__all__ = [
    "SandboxClient",
    "Codebase",
    "ExecResult",
    "FileInfo",
    "Permission",
    "PatternType",
    "PermissionRule",
    "Sandbox",
    "SandboxStatus",
    "UploadResult",
]
