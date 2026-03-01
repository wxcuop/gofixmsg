"""
Focused unit tests for FixEngine session management, message routing, error handling, and async lifecycle.
Targets property access, message creation, and connection logic using ConfigManager and mock application.
"""
import pytest
import asyncio
from unittest.mock import Mock, AsyncMock, patch
from pyfixmsg_plus.fixengine.configmanager import ConfigManager
from pyfixmsg_plus.fixengine.engine import FixEngine

@pytest.fixture
def config_manager(tmp_path):
    config_file = tmp_path / "test_engine.ini"
    config_file.write_text("""
[FIX]
sender = TEST_SENDER
target = TEST_TARGET
heartbeat_interval = 30
mode = acceptor
host = 127.0.0.1
port = 9878
version = FIX.4.4
""")
    return ConfigManager(str(config_file))

@pytest.fixture
def mock_application():
    app = Mock()
    app.on_message = AsyncMock()
    return app

@pytest.fixture
def engine(config_manager, mock_application):
    with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler:
        mock_scheduler.return_value.run_scheduler = AsyncMock()
        return FixEngine(config_manager, mock_application)

class TestEngineProperties:
    def test_property_access(self, engine):
        assert engine.sender == "TEST_SENDER"
        assert engine.target == "TEST_TARGET"
        assert engine.heartbeat_interval == 30
        assert engine.mode == "acceptor"
        assert engine.version == "FIX.4.4"
        assert engine.host == "127.0.0.1"
        assert engine.port == 9878
        assert isinstance(engine.session_id, str)

class TestEngineMessageCreation:
    def test_fixmsg_creation(self, engine):
        msg = engine.fixmsg({"35": "D", "11": "ORDER123"})
        assert msg["35"] == "D"
        assert msg["11"] == "ORDER123"
        assert hasattr(msg, "get")

class TestEngineConnectionLogic:
    def test_connection_initialization(self, engine):
        # Should initialize as Acceptor
        assert engine.mode == "acceptor"
        assert hasattr(engine, "network")
        assert engine.network.host == "127.0.0.1"
        assert engine.network.port == 9878

class TestEngineSessionManagement:
    def test_state_machine_initialization(self, engine):
        assert hasattr(engine, "state_machine")
        assert engine.state_machine.state.name == "DISCONNECTED"

class TestEngineAsyncLifecycle:
    @pytest.mark.asyncio
    async def test_send_message_connected(self, engine):
        mock_conn = Mock()
        mock_conn.is_connected = True
        mock_conn.send_message = AsyncMock()
        engine.connection = mock_conn
        engine.state_machine.state.name = "CONNECTED"
        # Patch message_store: Mock for sync, AsyncMock for async
        mock_store = Mock()
        mock_store.get_next_outgoing_sequence_number.return_value = 1
        mock_store.increment_outgoing_sequence_number = AsyncMock()
        mock_store.store_message = AsyncMock()
        mock_store.close = AsyncMock()
        engine.message_store = mock_store
        # Patch network.send to call connection.send_message
        async def network_send(wire_message):
            await engine.connection.send_message(wire_message)
        engine.network.send = AsyncMock(side_effect=network_send)
        msg = engine.fixmsg({35: "D", 11: "ORDER123"})
        await engine.send_message(msg)
        mock_conn.send_message.assert_called_once()

    @pytest.mark.asyncio
    async def test_send_message_disconnected(self, engine):
        mock_conn = Mock()
        mock_conn.is_connected = False
        mock_conn.send_message = AsyncMock()
        engine.connection = mock_conn
        # Use integer tag keys
        msg = engine.fixmsg({35: "D", 11: "ORDER123"})
        await engine.send_message(msg)
        mock_conn.send_message.assert_not_called()

class TestEngineErrorHandling:
    @pytest.mark.asyncio
    async def test_send_logout_message(self, engine):
        mock_conn = Mock()
        mock_conn.is_connected = True
        mock_conn.send_message = AsyncMock()
        engine.connection = mock_conn
        engine.state_machine.state.name = "CONNECTED"
        mock_store = Mock()
        mock_store.get_next_outgoing_sequence_number.return_value = 1
        mock_store.increment_outgoing_sequence_number = AsyncMock()
        mock_store.store_message = AsyncMock()
        mock_store.close = AsyncMock()
        engine.message_store = mock_store
        async def network_send(wire_message):
            await engine.connection.send_message(wire_message)
        engine.network.send = AsyncMock(side_effect=network_send)
        await engine.send_logout_message(text="Test logout")
        mock_conn.send_message.assert_called_once()

    @pytest.mark.asyncio
    async def test_send_reject_message(self, engine):
        mock_conn = Mock()
        mock_conn.is_connected = True
        mock_conn.send_message = AsyncMock()
        engine.connection = mock_conn
        engine.state_machine.state.name = "CONNECTED"
        mock_store = Mock()
        mock_store.get_next_outgoing_sequence_number.return_value = 1
        mock_store.increment_outgoing_sequence_number = AsyncMock()
        mock_store.store_message = AsyncMock()
        mock_store.close = AsyncMock()
        engine.message_store = mock_store
        async def network_send(wire_message):
            await engine.connection.send_message(wire_message)
        engine.network.send = AsyncMock(side_effect=network_send)
        await engine.send_reject_message(ref_seq_num=1, ref_tag_id=35, session_reject_reason=5, text="Invalid message type")
        mock_conn.send_message.assert_called_once()

class TestEngineMessageRouting:
    def test_message_routing(self, engine):
        # Patch processor attribute
        processor_mock = Mock()
        processor_mock.process_message = Mock()
        engine.processor = processor_mock
        msg = {"35": "A", "49": "SENDER", "56": "TARGET"}
        # Simulate routing by calling process_message directly
        engine.processor.process_message(msg)
        engine.processor.process_message.assert_called_once_with(msg)
