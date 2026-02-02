"""Tests for Session functionality in the AgentFense Python SDK.

These tests verify the Session type definitions and basic functionality.
Note: Full integration tests require a running sandbox server.
"""

import pytest
from datetime import datetime, timedelta

from agentfense.types import Session, SessionStatus


class TestSessionType:
    """Tests for Session type."""

    def test_session_creation(self):
        """Test creating a Session object."""
        session = Session(
            id="sess_123",
            sandbox_id="sb_456",
            status=SessionStatus.ACTIVE,
            shell="/bin/bash",
            created_at=datetime.now(),
        )
        
        assert session.id == "sess_123"
        assert session.sandbox_id == "sb_456"
        assert session.status == SessionStatus.ACTIVE
        assert session.shell == "/bin/bash"
        assert session.created_at is not None
        assert session.closed_at is None

    def test_session_status_enum(self):
        """Test SessionStatus enum values."""
        assert SessionStatus.ACTIVE.value == "active"
        assert SessionStatus.CLOSED.value == "closed"

    def test_session_defaults(self):
        """Test Session default values."""
        session = Session(
            id="sess_123",
            sandbox_id="sb_456",
            status=SessionStatus.ACTIVE,
        )
        
        assert session.shell == "/bin/bash"
        assert session.created_at is None
        assert session.closed_at is None

    def test_session_with_closed_at(self):
        """Test Session with closed_at timestamp."""
        now = datetime.now()
        session = Session(
            id="sess_123",
            sandbox_id="sb_456",
            status=SessionStatus.CLOSED,
            shell="/bin/bash",
            created_at=now,
            closed_at=now,
        )
        
        assert session.status == SessionStatus.CLOSED
        assert session.closed_at is not None


class TestSessionWrapper:
    """Tests for SessionWrapper class."""
    
    def test_session_wrapper_properties(self):
        """Test SessionWrapper property access."""
        from agentfense.client import SessionWrapper
        
        # Create a mock client (we can't test actual calls without server)
        class MockClient:
            pass
        
        session = Session(
            id="sess_123",
            sandbox_id="sb_456",
            status=SessionStatus.ACTIVE,
            shell="/bin/sh",
        )
        
        wrapper = SessionWrapper(MockClient(), session)
        
        assert wrapper.id == "sess_123"
        assert wrapper.sandbox_id == "sb_456"
        assert wrapper.status == SessionStatus.ACTIVE
        assert wrapper.shell == "/bin/sh"

    def test_session_wrapper_context_manager(self):
        """Test SessionWrapper as context manager."""
        from agentfense.client import SessionWrapper
        
        class MockClient:
            def __init__(self):
                self.destroyed = False
            
            def destroy_session(self, session_id):
                self.destroyed = True
        
        mock_client = MockClient()
        session = Session(
            id="sess_123",
            sandbox_id="sb_456",
            status=SessionStatus.ACTIVE,
        )
        
        # Test context manager
        with SessionWrapper(mock_client, session) as wrapper:
            assert wrapper.id == "sess_123"
        
        # After exiting, session should be destroyed
        assert mock_client.destroyed is True

    def test_session_wrapper_context_manager_with_exception(self):
        """Test SessionWrapper cleans up even when exception occurs."""
        from agentfense.client import SessionWrapper
        
        class MockClient:
            def __init__(self):
                self.destroyed = False
            
            def destroy_session(self, session_id):
                self.destroyed = True
        
        mock_client = MockClient()
        session = Session(
            id="sess_123",
            sandbox_id="sb_456",
            status=SessionStatus.ACTIVE,
        )
        
        # Test context manager with exception
        try:
            with SessionWrapper(mock_client, session) as wrapper:
                raise ValueError("Test exception")
        except ValueError:
            pass
        
        # Session should still be destroyed
        assert mock_client.destroyed is True


class TestClientSessionMethods:
    """Tests for SandboxClient session methods (type checking only)."""
    
    def test_client_has_session_methods(self):
        """Test that SandboxClient has session methods."""
        from agentfense.client import SandboxClient
        
        # Check methods exist
        assert hasattr(SandboxClient, 'create_session')
        assert hasattr(SandboxClient, 'session')
        assert hasattr(SandboxClient, 'get_session')
        assert hasattr(SandboxClient, 'list_sessions')
        assert hasattr(SandboxClient, 'destroy_session')
        assert hasattr(SandboxClient, 'session_exec')
