"""Utility functions for the Sandbox SDK.

This module provides helper functions for common tasks like
walking directories, parsing ignore files, and formatting output.
"""

import fnmatch
import os
from pathlib import Path
from typing import Iterator, List, Optional, Set, Tuple


# Default patterns to always ignore
DEFAULT_IGNORE_PATTERNS = [
    ".git",
    ".git/**",
    "__pycache__",
    "__pycache__/**",
    "*.pyc",
    "*.pyo",
    ".DS_Store",
    "Thumbs.db",
    "node_modules",
    "node_modules/**",
    ".venv",
    ".venv/**",
    "venv",
    "venv/**",
    ".env",
    "*.egg-info",
    "*.egg-info/**",
    ".tox",
    ".tox/**",
    ".pytest_cache",
    ".pytest_cache/**",
    ".mypy_cache",
    ".mypy_cache/**",
    ".ruff_cache",
    ".ruff_cache/**",
]


def parse_ignore_file(path: str) -> List[str]:
    """Parse a .gitignore or .sandboxignore file.
    
    Args:
        path: Path to the ignore file.
        
    Returns:
        List of ignore patterns.
        
    Note:
        - Empty lines and comments (starting with #) are skipped.
        - Patterns are returned as-is (gitignore syntax).
    """
    patterns = []
    ignore_path = Path(path)
    
    if not ignore_path.exists():
        return patterns
    
    with open(ignore_path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            # Skip empty lines and comments
            if not line or line.startswith("#"):
                continue
            patterns.append(line)
    
    return patterns


def _normalize_pattern(pattern: str) -> str:
    """Normalize a gitignore pattern for fnmatch.
    
    Args:
        pattern: A gitignore-style pattern.
        
    Returns:
        A pattern suitable for fnmatch.
    """
    # Remove leading slash (gitignore root anchor)
    if pattern.startswith("/"):
        pattern = pattern[1:]
    
    # Handle negation (we don't support this yet, just strip it)
    if pattern.startswith("!"):
        return ""
    
    # Handle directory-only patterns (trailing /)
    if pattern.endswith("/"):
        pattern = pattern[:-1]
    
    return pattern


def _should_ignore(
    rel_path: str,
    patterns: List[str],
    is_dir: bool = False,
) -> bool:
    """Check if a path should be ignored based on patterns.
    
    Args:
        rel_path: Relative path to check.
        patterns: List of ignore patterns.
        is_dir: Whether the path is a directory.
        
    Returns:
        True if the path should be ignored.
    """
    # Normalize path separators
    rel_path = rel_path.replace(os.sep, "/")
    
    for pattern in patterns:
        normalized = _normalize_pattern(pattern)
        if not normalized:
            continue
        
        # Check if pattern matches the path or any parent
        if fnmatch.fnmatch(rel_path, normalized):
            return True
        
        # Also check just the filename
        filename = os.path.basename(rel_path)
        if fnmatch.fnmatch(filename, normalized):
            return True
        
        # For patterns with **, check against full path
        if "**" in normalized:
            if fnmatch.fnmatch(rel_path, normalized):
                return True
    
    return False


def walk_directory(
    path: str,
    ignore_patterns: Optional[List[str]] = None,
    include_default_ignores: bool = True,
    max_file_size: int = 50 * 1024 * 1024,  # 50 MB
) -> Iterator[Tuple[str, bytes]]:
    """Recursively walk a directory and yield file paths with content.
    
    Args:
        path: Root directory path.
        ignore_patterns: Additional patterns to ignore (gitignore syntax).
        include_default_ignores: Whether to include default ignore patterns.
        max_file_size: Maximum file size to read (larger files are skipped).
        
    Yields:
        Tuples of (relative_path, file_content).
        
    Example:
        >>> for rel_path, content in walk_directory("./my-project"):
        ...     print(f"File: {rel_path}, Size: {len(content)} bytes")
    """
    root_path = Path(path).resolve()
    
    # Build ignore patterns
    patterns: List[str] = []
    if include_default_ignores:
        patterns.extend(DEFAULT_IGNORE_PATTERNS)
    if ignore_patterns:
        patterns.extend(ignore_patterns)
    
    # Check for .gitignore and .sandboxignore in root
    for ignore_file in [".gitignore", ".sandboxignore"]:
        ignore_path = root_path / ignore_file
        if ignore_path.exists():
            patterns.extend(parse_ignore_file(str(ignore_path)))
    
    # Track ignored directories to skip their contents
    ignored_dirs: Set[Path] = set()
    
    for current_path in root_path.rglob("*"):
        # Skip directories themselves (we only yield files)
        if current_path.is_dir():
            rel_path = str(current_path.relative_to(root_path))
            if _should_ignore(rel_path, patterns, is_dir=True):
                ignored_dirs.add(current_path)
            continue
        
        # Skip if parent is ignored
        if any(current_path.is_relative_to(d) for d in ignored_dirs):
            continue
        
        # Get relative path
        rel_path = str(current_path.relative_to(root_path))
        
        # Check if file should be ignored
        if _should_ignore(rel_path, patterns):
            continue
        
        # Skip files that are too large
        try:
            size = current_path.stat().st_size
            if size > max_file_size:
                continue
        except OSError:
            continue
        
        # Read file content
        try:
            content = current_path.read_bytes()
            yield rel_path, content
        except (IOError, PermissionError):
            # Skip files we can't read
            continue


def human_readable_size(size: int) -> str:
    """Format a file size in human-readable form.
    
    Args:
        size: Size in bytes.
        
    Returns:
        Human-readable string like "1.5 MB".
        
    Example:
        >>> human_readable_size(1536)
        '1.5 KB'
        >>> human_readable_size(1048576)
        '1.0 MB'
    """
    for unit in ["B", "KB", "MB", "GB", "TB"]:
        if abs(size) < 1024.0:
            return f"{size:.1f} {unit}"
        size /= 1024.0
    return f"{size:.1f} PB"


def generate_codebase_name(path: str) -> str:
    """Generate a codebase name from a directory path.
    
    Args:
        path: Directory path.
        
    Returns:
        A sanitized name suitable for a codebase.
    """
    dir_name = Path(path).resolve().name
    # Sanitize: replace spaces and special chars
    name = "".join(c if c.isalnum() or c in "-_" else "-" for c in dir_name)
    # Remove consecutive dashes
    while "--" in name:
        name = name.replace("--", "-")
    # Trim dashes from ends
    return name.strip("-") or "unnamed-project"


def generate_owner_id() -> str:
    """Generate a default owner ID.
    
    Returns:
        A default owner ID based on the current user.
    """
    import getpass
    import hashlib
    
    username = getpass.getuser()
    # Create a simple hash-based ID
    hash_input = f"sandbox-sdk-{username}".encode()
    short_hash = hashlib.sha256(hash_input).hexdigest()[:8]
    return f"user_{short_hash}"


def count_files(path: str, ignore_patterns: Optional[List[str]] = None) -> Tuple[int, int]:
    """Count files and total size in a directory.
    
    Args:
        path: Root directory path.
        ignore_patterns: Patterns to ignore.
        
    Returns:
        Tuple of (file_count, total_size_bytes).
    """
    file_count = 0
    total_size = 0
    
    for _, content in walk_directory(path, ignore_patterns):
        file_count += 1
        total_size += len(content)
    
    return file_count, total_size
