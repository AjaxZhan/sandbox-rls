"""Type definitions for the Sandbox SDK."""

from dataclasses import dataclass, field
from datetime import datetime, timedelta
from enum import Enum
from typing import Dict, List, Optional


class Permission(str, Enum):
    """Permission levels for file access."""
    NONE = "none"
    VIEW = "view"
    READ = "read"
    WRITE = "write"


class PatternType(str, Enum):
    """Types of permission patterns."""
    GLOB = "glob"
    DIRECTORY = "directory"
    FILE = "file"


class SandboxStatus(str, Enum):
    """Status of a sandbox."""
    PENDING = "pending"
    RUNNING = "running"
    STOPPED = "stopped"
    ERROR = "error"


class SessionStatus(str, Enum):
    """Status of a shell session."""
    UNKNOWN = "unknown"  # Unspecified or unknown status
    ACTIVE = "active"
    CLOSED = "closed"


class RuntimeType(str, Enum):
    """Runtime type for sandbox execution.
    
    Determines the isolation mechanism used for the sandbox.
    """
    BWRAP = "bwrap"    # Bubblewrap - lightweight namespace isolation
    DOCKER = "docker"  # Docker container - stronger isolation


@dataclass
class ResourceLimits:
    """Resource constraints for a sandbox.
    
    Defines limits on CPU, memory, and process count that the sandbox
    can use. These limits are enforced by the runtime (Docker or bwrap).
    
    Attributes:
        memory_bytes: Maximum memory in bytes (e.g., 512*1024*1024 for 512MB).
        cpu_quota: CPU quota in microseconds per 100ms period.
        cpu_shares: CPU shares (relative weight, default 1024).
        pids_limit: Maximum number of processes/threads.
        
    Example:
        >>> limits = ResourceLimits(
        ...     memory_bytes=512 * 1024 * 1024,  # 512 MB
        ...     pids_limit=100,
        ... )
    """
    memory_bytes: Optional[int] = None
    cpu_quota: Optional[int] = None
    cpu_shares: Optional[int] = None
    pids_limit: Optional[int] = None
    
    def to_dict(self) -> Dict:
        """Convert to dictionary, excluding None values."""
        return {
            k: v for k, v in {
                "memory_bytes": self.memory_bytes,
                "cpu_quota": self.cpu_quota,
                "cpu_shares": self.cpu_shares,
                "pids_limit": self.pids_limit,
            }.items() if v is not None
        }
    
    @classmethod
    def from_dict(cls, data: Dict) -> "ResourceLimits":
        """Create ResourceLimits from a dictionary."""
        return cls(
            memory_bytes=data.get("memory_bytes"),
            cpu_quota=data.get("cpu_quota"),
            cpu_shares=data.get("cpu_shares"),
            pids_limit=data.get("pids_limit"),
        )


@dataclass
class PermissionRule:
    """A rule defining file permissions."""
    pattern: str
    permission: Permission = Permission.READ
    type: PatternType = PatternType.GLOB
    priority: int = 0

    def to_dict(self) -> Dict:
        """Convert to dictionary for API calls."""
        return {
            "pattern": self.pattern,
            "permission": self.permission.value,
            "type": self.type.value,
            "priority": self.priority,
        }


@dataclass
class Sandbox:
    """Represents a sandbox instance.
    
    Attributes:
        id: Unique identifier for the sandbox.
        codebase_id: ID of the associated codebase.
        status: Current status (pending, running, stopped, error).
        permissions: List of permission rules for file access.
        labels: User-defined labels for organization.
        runtime: Runtime type (bwrap or docker).
        image: Docker image name (for docker runtime).
        resources: Resource limits (memory, CPU, etc.).
        created_at: When the sandbox was created.
        started_at: When the sandbox was started.
        stopped_at: When the sandbox was stopped.
        expires_at: When the sandbox will expire.
    """
    id: str
    codebase_id: str
    status: SandboxStatus
    permissions: List[PermissionRule] = field(default_factory=list)
    labels: Dict[str, str] = field(default_factory=dict)
    runtime: RuntimeType = RuntimeType.BWRAP
    image: Optional[str] = None
    resources: Optional[ResourceLimits] = None
    created_at: Optional[datetime] = None
    started_at: Optional[datetime] = None
    stopped_at: Optional[datetime] = None
    expires_at: Optional[datetime] = None


@dataclass
class Codebase:
    """Represents a codebase (file storage)."""
    id: str
    name: str
    owner_id: str
    size: int = 0
    file_count: int = 0
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None


@dataclass
class ExecResult:
    """Result of command execution.
    
    Attributes:
        stdout: Standard output from the command.
        stderr: Standard error from the command.
        exit_code: Exit code of the command (0 typically means success).
        duration: How long the command took to execute.
        command: The command that was executed (for debugging).
        
    Example:
        >>> result = sandbox.run("python --version")
        >>> if result.success:
        ...     print(result.stdout)
        >>> else:
        ...     print(f"Failed: {result.output}")
        
        >>> # Chain with raise_on_error for fail-fast behavior
        >>> result = sandbox.run("make build").raise_on_error()
    """
    stdout: str
    stderr: str
    exit_code: int
    duration: Optional[timedelta] = None
    command: Optional[str] = None
    
    @property
    def success(self) -> bool:
        """True if the command exited with code 0."""
        return self.exit_code == 0
    
    @property
    def output(self) -> str:
        """Combined stdout and stderr output.
        
        Useful when you want all output regardless of stream.
        """
        parts = []
        if self.stdout:
            parts.append(self.stdout)
        if self.stderr:
            parts.append(self.stderr)
        return "\n".join(parts) if parts else ""
    
    def raise_on_error(self) -> "ExecResult":
        """Raise CommandExecutionError if the command failed.
        
        Returns self for method chaining, allowing patterns like:
            result = sandbox.run("make").raise_on_error()
            
        Raises:
            CommandExecutionError: If exit_code is non-zero.
            
        Returns:
            Self, for method chaining.
        """
        if not self.success:
            from .exceptions import CommandExecutionError
            raise CommandExecutionError(
                command=self.command or "<unknown>",
                exit_code=self.exit_code,
                stdout=self.stdout,
                stderr=self.stderr,
            )
        return self


@dataclass
class FileInfo:
    """Information about a file in a codebase."""
    path: str
    name: str
    is_dir: bool
    size: int
    modified_at: Optional[datetime] = None


@dataclass
class UploadResult:
    """Result of file upload."""
    codebase_id: str
    file_path: str
    size: int
    checksum: str


@dataclass
class Session:
    """Represents a shell session within a sandbox.
    
    A session maintains a persistent shell process that preserves
    working directory, environment variables, and background processes
    across multiple command executions.
    """
    id: str
    sandbox_id: str
    status: SessionStatus
    shell: str = "/bin/bash"
    created_at: Optional[datetime] = None
    closed_at: Optional[datetime] = None
