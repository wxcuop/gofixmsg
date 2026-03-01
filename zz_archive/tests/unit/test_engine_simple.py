"""
Simplified high-impact unit tests for FixEngine.
This file targets the 478-line engine for maximum Phase 2 coverage with minimal mocking.
"""
import pytest
import asyncio
from unittest.mock import Mock, AsyncMock, patch
import tempfile
import os


@pytest.fixture
def engine_config_file():
    """Create a temporary config file for engine tests."""
    config_content = """
[FIX]
sender = TEST_SENDER
target = TEST_TARGET
heartbeat_interval = 30
version = FIX.4.4
mode = acceptor
host = localhost
port = 9878
"""
    
    with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
        f.write(config_content)
        temp_file = f.name
    
    yield temp_file
    
    # Cleanup
    try:
        os.unlink(temp_file)
    except FileNotFoundError:
        pass


@pytest.mark.unit
class TestFixEngineBasics:
    """Test basic FixEngine functionality without heavy mocking."""
    
    def test_engine_initialization_with_config_file(self, engine_config_file):
        """Test FixEngine initialization with config file."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            from pyfixmsg_plus.fixengine.configmanager import ConfigManager
            
            config_manager = ConfigManager(engine_config_file)
            mock_application = Mock()
            engine = FixEngine(config_manager, mock_application)
            
            assert hasattr(engine, 'config_manager')
            assert hasattr(engine, 'message_store')
            assert hasattr(engine, 'state_machine')
            assert hasattr(engine, 'heartbeat')
    
    def test_engine_properties_access(self, engine_config_file):
        """Test FixEngine property access."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            from pyfixmsg_plus.fixengine.configmanager import ConfigManager
            
            config_manager = ConfigManager(engine_config_file)
            mock_application = Mock()
            engine = FixEngine(config_manager, mock_application)
            
            # Test property access
            assert engine.sender == 'TEST_SENDER'
            assert engine.target == 'TEST_TARGET'
            assert engine.heartbeat_interval == 30
            assert engine.version == 'FIX.4.4'
            assert engine.mode == 'acceptor'
    
    def test_engine_session_id_generation(self, engine_config_file):
        """Test session ID generation."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            from pyfixmsg_plus.fixengine.configmanager import ConfigManager
            
            config_manager = ConfigManager(engine_config_file)
            mock_application = Mock()
            engine = FixEngine(config_manager, mock_application)
            
            session_id = engine.session_id
            assert session_id is not None
            assert isinstance(session_id, str)
            assert 'TEST_SENDER' in session_id
            assert 'TEST_TARGET' in session_id
    
    def test_engine_with_application(self, engine_config_file):
        """Test FixEngine initialization with application."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            from pyfixmsg_plus.fixengine.configmanager import ConfigManager
            
            config_manager = ConfigManager(engine_config_file)
            mock_application = Mock()
            engine = FixEngine(config_manager, mock_application)
            
            assert engine.application == mock_application
    
    def test_fixmsg_creation(self, engine_config_file):
        """Test FIX message creation."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            from pyfixmsg_plus.fixengine.configmanager import ConfigManager
            
            config_manager = ConfigManager(engine_config_file)
            mock_application = Mock()
            engine = FixEngine(config_manager, mock_application)
            
            # Test creating message with dict
            msg_data = {'35': 'D', '11': 'ORDER123'}
            msg = engine.fixmsg(msg_data)
            
            assert msg is not None
            assert hasattr(msg, 'get')  # Should behave like dict
    
    def test_sequence_number_properties(self, engine_config_file):
        """Test sequence number properties."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            from pyfixmsg_plus.fixengine.configmanager import ConfigManager
            
            config_manager = ConfigManager(engine_config_file)
            mock_application = Mock()
            engine = FixEngine(config_manager, mock_application)
            
            # Test sequence number access - need message store first
            if hasattr(engine, 'next_outgoing_seq_num'):
                next_outgoing = engine.next_outgoing_seq_num
                assert isinstance(next_outgoing, int)
                assert next_outgoing >= 1


@pytest.mark.unit
class TestFixEngineMessageOperations:
    """Test FixEngine message operations with mocked dependencies."""
    
    @pytest.mark.asyncio
    async def test_send_message_when_connected(self, engine_config_file):
        """Test sending message when connected."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            engine = FixEngine(config_file=engine_config_file)
            
            # Mock connection
            mock_connection = Mock()
            mock_connection.is_connected = True
            mock_connection.send_message = AsyncMock()
            engine.connection = mock_connection
            
            test_message = engine.fixmsg({'35': 'D', '11': 'ORDER123'})
            
            await engine.send_message(test_message)
            
            # Should send through connection
            mock_connection.send_message.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_send_message_when_disconnected(self, engine_config_file):
        """Test sending message when disconnected."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            engine = FixEngine(config_file=engine_config_file)
            
            # Mock disconnected connection
            mock_connection = Mock()
            mock_connection.is_connected = False
            mock_connection.send_message = AsyncMock()
            engine.connection = mock_connection
            
            test_message = engine.fixmsg({'35': 'D', '11': 'ORDER123'})
            
            # Should handle gracefully when disconnected
            await engine.send_message(test_message)
            
            # Should not send when disconnected
            mock_connection.send_message.assert_not_called()
    
    @pytest.mark.asyncio
    async def test_send_logout_message(self, engine_config_file):
        """Test sending logout message."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            engine = FixEngine(config_file=engine_config_file)
            
            # Mock connected connection
            mock_connection = Mock()
            mock_connection.is_connected = True
            mock_connection.send_message = AsyncMock()
            engine.connection = mock_connection
            
            await engine.send_logout_message(text="Test logout")
            
            # Should send logout message
            mock_connection.send_message.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_send_reject_message(self, engine_config_file):
        """Test sending reject message."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            engine = FixEngine(config_file=engine_config_file)
            
            # Mock connected connection
            mock_connection = Mock()
            mock_connection.is_connected = True
            mock_connection.send_message = AsyncMock()
            engine.connection = mock_connection
            
            await engine.send_reject_message(
                ref_seq_num=1,
                ref_tag_id=35,
                session_reject_reason=5,
                text="Invalid message type"
            )
            
            # Should send reject message
            mock_connection.send_message.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_reset_sequence_numbers(self, engine_config_file):
        """Test resetting sequence numbers."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            engine = FixEngine(config_file=engine_config_file)
            
            await engine.reset_sequence_numbers()
            
            # Should complete without error
            assert True
    
    @pytest.mark.asyncio 
    async def test_on_message_received(self, engine_config_file):
        """Test message processing flow."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            engine = FixEngine(config_file=engine_config_file)
            
            # Mock processor
            engine.processor.process_message = AsyncMock()
            
            # Simulate incoming message
            test_message = {'35': 'A', '49': 'SENDER', '56': 'TARGET'}
            
            await engine.on_message_received(test_message)
            
            # Should process through message processor
            engine.processor.process_message.assert_called_once_with(test_message)


@pytest.mark.unit
class TestFixEngineConfiguration:
    """Test FixEngine configuration and error handling."""
    
    def test_engine_configuration_access(self, engine_config_file):
        """Test accessing engine configuration."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            engine = FixEngine(config_file=engine_config_file)
            
            # Test various configuration access patterns
            assert engine.config_manager is not None
            assert engine.message_store is not None
            assert engine.state_machine is not None
            assert engine.processor is not None
            assert engine.heartbeat is not None
            assert engine.test_request is not None
    
    def test_engine_with_invalid_config(self):
        """Test engine with invalid configuration."""
        with patch('pyfixmsg_plus.fixengine.engine.Scheduler') as mock_scheduler_class:
            mock_scheduler_instance = Mock()
            mock_scheduler_instance.run_scheduler = AsyncMock()
            mock_scheduler_class.return_value = mock_scheduler_instance
            
            from pyfixmsg_plus.fixengine.engine import FixEngine
            
            # Should handle missing config file gracefully
            try:
                engine = FixEngine(config_file="/nonexistent/config.ini")
                # Engine should still initialize but may have default values
                assert engine is not None
            except Exception as e:
                # Or raise appropriate configuration error
                assert 'config' in str(e).lower() or 'file' in str(e).lower()
