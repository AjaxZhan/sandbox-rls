"""AgentFense - Python client for the AgentFense service.

This SDK provides both low-level and high-level APIs for interacting
with the Sandbox service, with support for both synchronous and asynchronous
operations.

High-Level API (Recommended):
    Use the Sandbox class for the simplest experience:
    
    >>> from agentfense import Sandbox
    >>> 
    >>> # One-liner to create and run commands in a sandbox
    >>> with Sandbox.from_local("./my-project") as sandbox:
    ...     result = sandbox.run("python main.py")
    ...     print(result.stdout)
    >>> 
    >>> # With Docker and resource limits
    >>> with Sandbox.from_local(
    ...     "./my-project",
    ...     preset="agent-safe",
    ...     runtime=RuntimeType.DOCKER,
    ...     image="python:3.11-slim",
    ...     resources=ResourceLimits(memory_bytes=512*1024*1024),
    ... ) as sandbox:
    ...     result = sandbox.run("pytest")

Async API:
    Use AsyncSandbox for async/await operations:
    
    >>> from agentfense import AsyncSandbox
    >>> 
    >>> async with await AsyncSandbox.from_local("./my-project") as sandbox:
    ...     result = await sandbox.run("python main.py")
    ...     print(result.stdout)

Low-Level API:
    Use SandboxClient for fine-grained control:
    
    >>> from agentfense import SandboxClient
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

from .client import SandboxClient, SessionWrapper
from .sandbox import Sandbox
from ._async import AsyncSandbox, AsyncSandboxClient, AsyncSessionWrapper
from .types import (
    Codebase,
    ExecResult,
    FileInfo,
    Permission,
    PatternType,
    PermissionRule,
    ResourceLimits,
    RuntimeType,
    Sandbox as SandboxInfo,
    SandboxStatus,
    Session,
    SessionStatus,
    UploadResult,
)
from .presets import (
    PRESETS,
    PRESET_AGENT_SAFE,
    PRESET_READ_ONLY,
    PRESET_FULL_ACCESS,
    PRESET_DEVELOPMENT,
    PRESET_VIEW_ONLY,
    get_preset,
    get_preset_dicts,
    extend_preset,
    list_presets,
    register_preset,
)
from .exceptions import (
    SandboxError,
    SandboxNotFoundError,
    SandboxNotRunningError,
    CodebaseError,
    CodebaseNotFoundError,
    FileNotFoundError,
    CommandTimeoutError,
    CommandExecutionError,
    PermissionDeniedError,
    SessionError,
    SessionNotFoundError,
    SessionClosedError,
    ResourceLimitExceededError,
    InvalidConfigurationError,
    UploadError,
    ConnectionError,
)
from .utils import (
    walk_directory,
    parse_ignore_file,
    human_readable_size,
    generate_codebase_name,
    generate_owner_id,
    count_files,
)

__version__ = "0.2.0"
__all__ = [
    # High-level API (sync)
    "Sandbox",
    # High-level API (async)
    "AsyncSandbox",
    # Low-level client (sync)
    "SandboxClient",
    "SessionWrapper",
    # Low-level client (async)
    "AsyncSandboxClient",
    "AsyncSessionWrapper",
    # Types
    "Codebase",
    "ExecResult",
    "FileInfo",
    "Permission",
    "PatternType",
    "PermissionRule",
    "ResourceLimits",
    "RuntimeType",
    "SandboxInfo",
    "SandboxStatus",
    "Session",
    "SessionStatus",
    "UploadResult",
    # Presets
    "PRESETS",
    "PRESET_AGENT_SAFE",
    "PRESET_READ_ONLY",
    "PRESET_FULL_ACCESS",
    "PRESET_DEVELOPMENT",
    "PRESET_VIEW_ONLY",
    "get_preset",
    "get_preset_dicts",
    "extend_preset",
    "list_presets",
    "register_preset",
    # Exceptions
    "SandboxError",
    "SandboxNotFoundError",
    "SandboxNotRunningError",
    "CodebaseError",
    "CodebaseNotFoundError",
    "FileNotFoundError",
    "CommandTimeoutError",
    "CommandExecutionError",
    "PermissionDeniedError",
    "SessionError",
    "SessionNotFoundError",
    "SessionClosedError",
    "ResourceLimitExceededError",
    "InvalidConfigurationError",
    "UploadError",
    "ConnectionError",
    # Utilities
    "walk_directory",
    "parse_ignore_file",
    "human_readable_size",
    "generate_codebase_name",
    "generate_owner_id",
    "count_files",
]

