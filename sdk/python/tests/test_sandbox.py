"""Tests for the high-level Sandbox API."""

import tempfile
import os
import inspect
from pathlib import Path
from unittest.mock import MagicMock, patch, PropertyMock

import pytest

from agentfense.sandbox import Sandbox
from agentfense.types import (
    Codebase,
    ExecResult,
    Permission,
    PermissionRule,
    ResourceLimits,
    RuntimeType,
    Sandbox as SandboxInfo,
    SandboxStatus,
)
from agentfense.exceptions import CommandExecutionError


class TestSandboxProperties:
    """Tests for Sandbox properties."""
    
    def test_sandbox_id_property(self):
        """Test that id property returns sandbox ID."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(
            id="cb_456",
            name="test",
            owner_id="user_1",
        )
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        assert sandbox.id == "sb_123"
    
    def test_sandbox_status_property(self):
        """Test that status property returns current status."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        assert sandbox.status == SandboxStatus.RUNNING
    
    def test_sandbox_runtime_property(self):
        """Test that runtime property returns runtime type."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
            runtime=RuntimeType.DOCKER,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        assert sandbox.runtime == RuntimeType.DOCKER


class TestSandboxLifecycle:
    """Tests for Sandbox lifecycle methods."""
    
    def test_start_calls_client(self):
        """Test that start() calls client.start_sandbox."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.PENDING,
        )
        mock_client.start_sandbox.return_value = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        result = sandbox.start()
        
        mock_client.start_sandbox.assert_called_once_with("sb_123")
        assert result is sandbox  # Returns self for chaining
        assert sandbox.status == SandboxStatus.RUNNING
    
    def test_stop_calls_client(self):
        """Test that stop() calls client.stop_sandbox."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        mock_client.stop_sandbox.return_value = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.STOPPED,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        sandbox.stop()
        
        mock_client.stop_sandbox.assert_called_once_with("sb_123")
    
    def test_destroy_calls_client(self):
        """Test that destroy() calls appropriate client methods."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
            owns_client=True,
            owns_codebase=True,
        )
        
        sandbox.destroy()
        
        mock_client.destroy_sandbox.assert_called_once_with("sb_123")
        mock_client.delete_codebase.assert_called_once_with("cb_456")
        mock_client.close.assert_called_once()
    
    def test_destroy_without_codebase_ownership(self):
        """Test that destroy() doesn't delete codebase if not owned."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
            owns_client=True,
            owns_codebase=False,
        )
        
        sandbox.destroy()
        
        mock_client.destroy_sandbox.assert_called_once()
        mock_client.delete_codebase.assert_not_called()


class TestSandboxExecution:
    """Tests for Sandbox execution methods."""
    
    def test_run_calls_exec(self):
        """Test that run() calls client.exec with correct parameters."""
        mock_client = MagicMock()
        mock_client.exec.return_value = ExecResult(
            stdout="Hello",
            stderr="",
            exit_code=0,
        )
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        result = sandbox.run("echo Hello", timeout=30)
        
        assert result.stdout == "Hello"
        assert result.exit_code == 0
        mock_client.exec.assert_called_once()
    
    def test_run_with_raise_on_error(self):
        """Test that run() raises on non-zero exit when requested."""
        mock_client = MagicMock()
        mock_client.exec.return_value = ExecResult(
            stdout="",
            stderr="Error!",
            exit_code=1,
        )
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        with pytest.raises(CommandExecutionError) as exc_info:
            sandbox.run("failing-command", raise_on_error=True)
        
        assert exc_info.value.exit_code == 1
    
    def test_session_creates_session_wrapper(self):
        """Test that session() creates a session through client."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        sandbox.session(shell="/bin/sh", env={"FOO": "bar"})
        
        mock_client.create_session.assert_called_once_with(
            sandbox_id="sb_123",
            shell="/bin/sh",
            env={"FOO": "bar"},
        )


class TestSandboxFileOperations:
    """Tests for Sandbox file operations."""
    
    def test_read_file_returns_string(self):
        """Test that read_file returns decoded string."""
        mock_client = MagicMock()
        mock_client.download_file.return_value = b"Hello World"
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        content = sandbox.read_file("/workspace/test.txt")
        
        assert content == "Hello World"
        mock_client.download_file.assert_called_once_with("cb_456", "test.txt")
    
    def test_write_file_encodes_string(self):
        """Test that write_file encodes string to bytes."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        sandbox.write_file("/workspace/test.txt", "Hello")
        
        mock_client.upload_file.assert_called_once_with(
            "cb_456",
            "test.txt",
            b"Hello",
        )
    
    def test_list_files_returns_paths(self):
        """Test that list_files returns file paths."""
        mock_client = MagicMock()
        from agentfense.types import FileInfo
        mock_client.list_files.return_value = [
            FileInfo(path="/a.txt", name="a.txt", is_dir=False, size=100),
            FileInfo(path="/b.txt", name="b.txt", is_dir=False, size=200),
        ]
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        files = sandbox.list_files(recursive=True)
        
        assert files == ["/a.txt", "/b.txt"]


class TestSandboxContextManager:
    """Tests for Sandbox context manager support."""
    
    def test_context_manager_destroys_on_exit(self):
        """Test that context manager calls destroy on exit."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
            owns_client=True,
            owns_codebase=True,
        )
        
        with sandbox:
            pass
        
        mock_client.destroy_sandbox.assert_called_once()
    
    def test_context_manager_destroys_on_exception(self):
        """Test that context manager cleans up even on exception."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
            owns_client=True,
        )
        
        with pytest.raises(RuntimeError):
            with sandbox:
                raise RuntimeError("Test error")
        
        mock_client.destroy_sandbox.assert_called_once()


class TestSandboxFromLocal:
    """Tests for Sandbox.from_local class method."""
    
    def test_from_local_validates_path_exists(self):
        """Test that from_local raises for non-existent path."""
        with pytest.raises(ValueError) as exc_info:
            Sandbox.from_local("/nonexistent/path/12345")
        
        assert "does not exist" in str(exc_info.value)
    
    def test_from_local_validates_path_is_directory(self):
        """Test that from_local raises for file path."""
        with tempfile.NamedTemporaryFile() as f:
            with pytest.raises(ValueError) as exc_info:
                Sandbox.from_local(f.name)
            
            assert "not a directory" in str(exc_info.value)


class TestSandboxPresetDefaults:
    """Tests for default preset behavior."""
    
    def test_from_local_default_preset_is_view_only(self):
        """Sandbox.from_local should default to view-only when preset not passed."""
        sig = inspect.signature(Sandbox.from_local)
        assert sig.parameters["preset"].default == "view-only"
    
    def test_from_codebase_default_preset_is_view_only(self):
        """Sandbox.from_codebase should default to view-only when preset not passed."""
        sig = inspect.signature(Sandbox.from_codebase)
        assert sig.parameters["preset"].default == "view-only"
    
    @patch("agentfense.sandbox.SandboxClient")
    def test_from_local_creates_resources(self, mock_client_class):
        """Test that from_local creates codebase and sandbox."""
        # Set up mocks
        mock_client = MagicMock()
        mock_client_class.return_value = mock_client
        
        mock_codebase = Codebase(id="cb_123", name="test", owner_id="user_1")
        mock_client.create_codebase.return_value = mock_codebase
        
        mock_sandbox_info = SandboxInfo(
            id="sb_456",
            codebase_id="cb_123",
            status=SandboxStatus.PENDING,
        )
        mock_client.create_sandbox.return_value = mock_sandbox_info
        mock_client.start_sandbox.return_value = SandboxInfo(
            id="sb_456",
            codebase_id="cb_123",
            status=SandboxStatus.RUNNING,
        )
        
        # Create a temp directory with a file
        with tempfile.TemporaryDirectory() as tmpdir:
            test_file = Path(tmpdir) / "test.py"
            test_file.write_text("print('hello')")
            
            sandbox = Sandbox.from_local(
                tmpdir,
                preset="read-only",
                endpoint="localhost:9000",
            )
            
            # Verify calls
            mock_client.create_codebase.assert_called_once()
            mock_client.upload_file.assert_called()
            mock_client.create_sandbox.assert_called_once()
            mock_client.start_sandbox.assert_called_once()
            
            # Clean up
            sandbox._destroyed = True  # Prevent actual cleanup

    @patch("agentfense.sandbox.get_preset_dicts")
    @patch("agentfense.sandbox.SandboxClient")
    def test_from_local_preset_none_defaults_to_view_only(self, mock_client_class, mock_get_preset_dicts):
        """Passing preset=None should behave like default view-only."""
        # Set up mocks
        mock_client = MagicMock()
        mock_client_class.return_value = mock_client
        mock_get_preset_dicts.return_value = []
        
        mock_codebase = Codebase(id="cb_123", name="test", owner_id="user_1")
        mock_client.create_codebase.return_value = mock_codebase
        
        mock_sandbox_info = SandboxInfo(
            id="sb_456",
            codebase_id="cb_123",
            status=SandboxStatus.PENDING,
        )
        mock_client.create_sandbox.return_value = mock_sandbox_info
        mock_client.start_sandbox.return_value = SandboxInfo(
            id="sb_456",
            codebase_id="cb_123",
            status=SandboxStatus.RUNNING,
        )
        
        with tempfile.TemporaryDirectory() as tmpdir:
            test_file = Path(tmpdir) / "test.py"
            test_file.write_text("print('hello')")
            
            sandbox = Sandbox.from_local(tmpdir, preset=None)  # type: ignore[arg-type]
            
            mock_get_preset_dicts.assert_called_once_with("view-only")
            
            # Clean up
            sandbox._destroyed = True  # Prevent actual cleanup


class TestSandboxRepr:
    """Tests for Sandbox string representation."""
    
    def test_repr_includes_key_info(self):
        """Test that repr includes id, status, and runtime."""
        mock_client = MagicMock()
        sandbox_info = SandboxInfo(
            id="sb_123",
            codebase_id="cb_456",
            status=SandboxStatus.RUNNING,
            runtime=RuntimeType.DOCKER,
        )
        codebase = Codebase(id="cb_456", name="test", owner_id="user_1")
        
        sandbox = Sandbox(
            client=mock_client,
            sandbox_info=sandbox_info,
            codebase=codebase,
        )
        
        repr_str = repr(sandbox)
        
        assert "sb_123" in repr_str
        assert "running" in repr_str
        assert "docker" in repr_str
