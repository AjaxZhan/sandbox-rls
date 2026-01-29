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
    """Represents a sandbox instance."""
    id: str
    codebase_id: str
    status: SandboxStatus
    permissions: List[PermissionRule] = field(default_factory=list)
    labels: Dict[str, str] = field(default_factory=dict)
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
    """Result of command execution."""
    stdout: str
    stderr: str
    exit_code: int
    duration: Optional[timedelta] = None


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
