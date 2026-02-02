"""Tests for async Sandbox SDK API.

These tests verify that the async API classes are properly structured and importable.
Full integration tests would require a running sandbox server.
"""

import pytest
from datetime import timedelta
import inspect
from unittest.mock import MagicMock, patch

# Test imports work
from sandbox_rls import (
    AsyncSandbox,
    AsyncSandboxClient,
    AsyncSessionWrapper,
)
from sandbox_rls._async import (
    AsyncSandbox as AsyncSandboxDirect,
    AsyncSandboxClient as AsyncSandboxClientDirect,
    AsyncSessionWrapper as AsyncSessionWrapperDirect,
)


class TestAsyncImports:
    """Verify all async classes can be imported."""

    def test_import_from_main_module(self):
        """Async classes should be importable from main module."""
        assert AsyncSandbox is not None
        assert AsyncSandboxClient is not None
        assert AsyncSessionWrapper is not None

    def test_import_from_async_module(self):
        """Async classes should be importable from _async module."""
        assert AsyncSandboxDirect is not None
        assert AsyncSandboxClientDirect is not None
        assert AsyncSessionWrapperDirect is not None

    def test_classes_are_same(self):
        """Imports from both paths should be the same class."""
        assert AsyncSandbox is AsyncSandboxDirect
        assert AsyncSandboxClient is AsyncSandboxClientDirect
        assert AsyncSessionWrapper is AsyncSessionWrapperDirect


class TestAsyncSandboxClientStructure:
    """Test AsyncSandboxClient has expected methods."""

    def test_has_required_methods(self):
        """AsyncSandboxClient should have all required async methods."""
        import inspect
        
        # These should all be coroutine functions
        async_methods = [
            'create_sandbox',
            'get_sandbox',
            'list_sandboxes',
            'start_sandbox',
            'stop_sandbox',
            'destroy_sandbox',
            'exec',
            'create_session',
            'get_session',
            'list_sessions',
            'destroy_session',
            'session_exec',
            'create_codebase',
            'get_codebase',
            'list_codebases',
            'delete_codebase',
            'upload_file',
            'download_file',
            'list_files',
            'close',
        ]
        
        for method_name in async_methods:
            method = getattr(AsyncSandboxClient, method_name, None)
            assert method is not None, f"Missing method: {method_name}"
            # Check it's a coroutine function (async def)
            assert inspect.iscoroutinefunction(method), f"{method_name} should be async"

    def test_exec_stream_is_async_generator(self):
        """exec_stream should be an async generator function."""
        import inspect
        method = getattr(AsyncSandboxClient, 'exec_stream', None)
        assert method is not None
        assert inspect.isasyncgenfunction(method), "exec_stream should be async generator"


class TestAsyncSandboxStructure:
    """Test AsyncSandbox has expected methods."""

    def test_has_required_class_methods(self):
        """AsyncSandbox should have all required async class methods."""
        import inspect
        
        async_class_methods = [
            'from_local',
            'from_codebase',
            'connect',
        ]
        
        for method_name in async_class_methods:
            method = getattr(AsyncSandbox, method_name, None)
            assert method is not None, f"Missing class method: {method_name}"
            assert inspect.iscoroutinefunction(method), f"{method_name} should be async"

    def test_has_required_instance_methods(self):
        """AsyncSandbox should have all required async instance methods."""
        import inspect
        
        async_methods = [
            'start',
            'stop',
            'refresh',
            'destroy',
            'run',
            'exec',
            'session',
            'read_file',
            'read_file_bytes',
            'write_file',
            'list_files',
        ]
        
        for method_name in async_methods:
            method = getattr(AsyncSandbox, method_name, None)
            assert method is not None, f"Missing method: {method_name}"
            assert inspect.iscoroutinefunction(method), f"{method_name} should be async"

    def test_exec_stream_is_async_generator(self):
        """exec_stream should be an async generator function."""
        import inspect
        method = getattr(AsyncSandbox, 'exec_stream', None)
        assert method is not None
        assert inspect.isasyncgenfunction(method), "exec_stream should be async generator"

    def test_has_context_manager_methods(self):
        """AsyncSandbox should support async context manager."""
        assert hasattr(AsyncSandbox, '__aenter__')
        assert hasattr(AsyncSandbox, '__aexit__')


class TestAsyncSandboxPresetDefaults:
    """Tests for AsyncSandbox default preset behavior."""
    
    def test_from_local_default_preset_is_view_only(self):
        """AsyncSandbox.from_local should default to view-only when preset not passed."""
        sig = inspect.signature(AsyncSandbox.from_local)
        assert sig.parameters["preset"].default == "view-only"
    
    def test_from_codebase_default_preset_is_view_only(self):
        """AsyncSandbox.from_codebase should default to view-only when preset not passed."""
        sig = inspect.signature(AsyncSandbox.from_codebase)
        assert sig.parameters["preset"].default == "view-only"


@pytest.mark.asyncio
class TestAsyncSandboxPresetNone:
    """Tests for AsyncSandbox preset=None behavior."""
    
    @patch("sandbox_rls._async.sandbox.get_preset_dicts")
    @patch("sandbox_rls._async.sandbox.AsyncSandboxClient")
    async def test_from_local_preset_none_defaults_to_view_only(
        self, mock_client_class, mock_get_preset_dicts, tmp_path
    ):
        """Passing preset=None should behave like default view-only (async)."""
        mock_client = MagicMock()
        mock_client_class.return_value = mock_client
        mock_get_preset_dicts.return_value = []
        
        # Async mocks for client methods
        from sandbox_rls.types import Codebase, Sandbox as SandboxInfo, SandboxStatus
        mock_client.create_codebase = MagicMock(return_value=Codebase(id="cb_123", name="t", owner_id="u"))
        mock_client.upload_file = MagicMock()
        mock_client.create_sandbox = MagicMock(
            return_value=SandboxInfo(id="sb_456", codebase_id="cb_123", status=SandboxStatus.PENDING)
        )
        mock_client.start_sandbox = MagicMock(
            return_value=SandboxInfo(id="sb_456", codebase_id="cb_123", status=SandboxStatus.RUNNING)
        )
        
        async def _awaitable(x):
            return x
        
        # Make the async client methods awaitable
        mock_client.create_codebase.side_effect = lambda *a, **k: _awaitable(mock_client.create_codebase.return_value)
        mock_client.upload_file.side_effect = lambda *a, **k: _awaitable(None)
        mock_client.create_sandbox.side_effect = lambda *a, **k: _awaitable(mock_client.create_sandbox.return_value)
        mock_client.start_sandbox.side_effect = lambda *a, **k: _awaitable(mock_client.start_sandbox.return_value)
        mock_client.destroy_sandbox.side_effect = lambda *a, **k: _awaitable(None)
        mock_client.delete_codebase.side_effect = lambda *a, **k: _awaitable(None)
        mock_client.close.side_effect = lambda *a, **k: _awaitable(None)
        
        (tmp_path / "test.py").write_text("print('hello')")
        
        sandbox = await AsyncSandbox.from_local(str(tmp_path), preset=None)  # type: ignore[arg-type]
        try:
            mock_get_preset_dicts.assert_called_once_with("view-only")
        finally:
            sandbox._destroyed = True


class TestAsyncSessionWrapperStructure:
    """Test AsyncSessionWrapper has expected methods."""

    def test_has_required_methods(self):
        """AsyncSessionWrapper should have all required async methods."""
        import inspect
        
        async_methods = [
            'exec',
            'close',
        ]
        
        for method_name in async_methods:
            method = getattr(AsyncSessionWrapper, method_name, None)
            assert method is not None, f"Missing method: {method_name}"
            assert inspect.iscoroutinefunction(method), f"{method_name} should be async"

    def test_has_context_manager_methods(self):
        """AsyncSessionWrapper should support async context manager."""
        assert hasattr(AsyncSessionWrapper, '__aenter__')
        assert hasattr(AsyncSessionWrapper, '__aexit__')

    def test_has_properties(self):
        """AsyncSessionWrapper should have expected properties."""
        properties = ['id', 'sandbox_id', 'status', 'shell']
        for prop_name in properties:
            assert hasattr(AsyncSessionWrapper, prop_name), f"Missing property: {prop_name}"


@pytest.mark.asyncio
class TestAsyncClientInit:
    """Test AsyncSandboxClient initialization."""

    async def test_create_client(self):
        """AsyncSandboxClient can be instantiated."""
        client = AsyncSandboxClient(endpoint="localhost:9000")
        assert client is not None
        await client.close()

    async def test_client_context_manager(self):
        """AsyncSandboxClient works as async context manager."""
        async with AsyncSandboxClient(endpoint="localhost:9000") as client:
            assert client is not None


class TestExecResultIntegration:
    """Test ExecResult works well with async API types."""

    def test_exec_result_chain_with_raise(self):
        """ExecResult.raise_on_error() returns self for chaining."""
        from sandbox_rls import ExecResult
        
        result = ExecResult(
            stdout="Success",
            stderr="",
            exit_code=0,
            command="echo Success",
        )
        
        # Chain should work
        chained = result.raise_on_error()
        assert chained is result
        assert chained.stdout == "Success"
        assert chained.success is True

    def test_exec_result_output_combination(self):
        """ExecResult.output combines stdout and stderr correctly."""
        from sandbox_rls import ExecResult
        
        result = ExecResult(
            stdout="Line 1",
            stderr="Warning: something",
            exit_code=0,
            command="some_cmd",
        )
        
        assert "Line 1" in result.output
        assert "Warning" in result.output
