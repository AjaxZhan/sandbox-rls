"""Tests for agentfense types."""

import pytest
from agentfense.types import (
    Permission,
    PatternType,
    PermissionRule,
    Sandbox as SandboxInfo,
    SandboxStatus,
    Codebase,
    ExecResult,
    FileInfo,
)


class TestPermission:
    """Tests for Permission enum."""

    def test_permission_values(self):
        assert Permission.NONE.value == "none"
        assert Permission.VIEW.value == "view"
        assert Permission.READ.value == "read"
        assert Permission.WRITE.value == "write"

    def test_permission_from_string(self):
        assert Permission("none") == Permission.NONE
        assert Permission("read") == Permission.READ


class TestPatternType:
    """Tests for PatternType enum."""

    def test_pattern_type_values(self):
        assert PatternType.GLOB.value == "glob"
        assert PatternType.DIRECTORY.value == "directory"
        assert PatternType.FILE.value == "file"


class TestPermissionRule:
    """Tests for PermissionRule dataclass."""

    def test_create_permission_rule(self):
        rule = PermissionRule(
            pattern="**/*.py",
            permission=Permission.READ,
            type=PatternType.GLOB,
            priority=10,
        )
        assert rule.pattern == "**/*.py"
        assert rule.permission == Permission.READ
        assert rule.type == PatternType.GLOB
        assert rule.priority == 10

    def test_permission_rule_defaults(self):
        rule = PermissionRule(pattern="/docs/**")
        assert rule.permission == Permission.READ
        assert rule.type == PatternType.GLOB
        assert rule.priority == 0

    def test_to_dict(self):
        rule = PermissionRule(
            pattern="/src/**",
            permission=Permission.WRITE,
            type=PatternType.DIRECTORY,
            priority=5,
        )
        d = rule.to_dict()
        assert d["pattern"] == "/src/**"
        assert d["permission"] == "write"
        assert d["type"] == "directory"
        assert d["priority"] == 5


class TestSandbox:
    """Tests for Sandbox dataclass."""

    def test_create_sandbox(self):
        sandbox = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
            labels={"env": "test"},
        )
        assert sandbox.id == "sb_123"
        assert sandbox.codebase_id == "cb_456"
        assert sandbox.status == SandboxStatus.RUNNING
        assert sandbox.labels["env"] == "test"
        assert sandbox.permissions == []


class TestCodebase:
    """Tests for Codebase dataclass."""

    def test_create_codebase(self):
        codebase = Codebase(
            id="cb_123",
            name="my-project",
            owner_id="user_456",
            size=1024,
            file_count=10,
        )
        assert codebase.id == "cb_123"
        assert codebase.name == "my-project"
        assert codebase.owner_id == "user_456"
        assert codebase.size == 1024
        assert codebase.file_count == 10


class TestExecResult:
    """Tests for ExecResult dataclass."""

    def test_create_exec_result(self):
        result = ExecResult(
            stdout="hello world",
            stderr="",
            exit_code=0,
        )
        assert result.stdout == "hello world"
        assert result.stderr == ""
        assert result.exit_code == 0
        assert result.duration is None
        assert result.command is None

    def test_exec_result_with_command(self):
        result = ExecResult(
            stdout="output",
            stderr="error",
            exit_code=1,
            command="python main.py",
        )
        assert result.command == "python main.py"

    def test_success_property_true(self):
        result = ExecResult(stdout="ok", stderr="", exit_code=0)
        assert result.success is True

    def test_success_property_false(self):
        result = ExecResult(stdout="", stderr="error", exit_code=1)
        assert result.success is False

    def test_output_property_stdout_only(self):
        result = ExecResult(stdout="hello", stderr="", exit_code=0)
        assert result.output == "hello"

    def test_output_property_stderr_only(self):
        result = ExecResult(stdout="", stderr="error", exit_code=1)
        assert result.output == "error"

    def test_output_property_both(self):
        result = ExecResult(stdout="out", stderr="err", exit_code=0)
        assert result.output == "out\nerr"

    def test_output_property_empty(self):
        result = ExecResult(stdout="", stderr="", exit_code=0)
        assert result.output == ""

    def test_raise_on_error_success(self):
        result = ExecResult(stdout="ok", stderr="", exit_code=0, command="echo ok")
        # Should return self and not raise
        returned = result.raise_on_error()
        assert returned is result

    def test_raise_on_error_failure(self):
        from agentfense.exceptions import CommandExecutionError
        
        result = ExecResult(
            stdout="",
            stderr="command not found",
            exit_code=127,
            command="nonexistent_cmd",
        )
        with pytest.raises(CommandExecutionError) as exc_info:
            result.raise_on_error()
        
        assert exc_info.value.exit_code == 127
        assert exc_info.value.command == "nonexistent_cmd"
        assert "command not found" in exc_info.value.stderr

    def test_raise_on_error_without_command(self):
        from agentfense.exceptions import CommandExecutionError
        
        result = ExecResult(stdout="", stderr="error", exit_code=1)
        with pytest.raises(CommandExecutionError) as exc_info:
            result.raise_on_error()
        
        assert exc_info.value.command == "<unknown>"


class TestFileInfo:
    """Tests for FileInfo dataclass."""

    def test_create_file_info(self):
        info = FileInfo(
            path="/src/main.py",
            name="main.py",
            is_dir=False,
            size=256,
        )
        assert info.path == "/src/main.py"
        assert info.name == "main.py"
        assert not info.is_dir
        assert info.size == 256
