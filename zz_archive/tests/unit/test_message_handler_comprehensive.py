"""
High-impact unit tests for MessageHandler and MessageProcessor.
This file targets the 313-line message handler for maximum Phase 2 coverage.
"""
import pytest
import asyncio
from unittest.mock import Mock, AsyncMock, patch, MagicMock

from pyfixmsg_plus.fixengine.message_handler import (
    MessageHandler, MessageProcessor, LogonHandler, 
    ExecutionReportHandler, NewOrderHandler, CancelOrderHandler,
    OrderCancelReplaceHandler, OrderCancelRejectHandler,
    NewOrderMultilegHandler, MultilegOrderCancelReplaceHandler,
    ResendRequestHandler, SequenceResetHandler, RejectHandler,
    LogoutHandler, HeartbeatHandler, logging_decorator
)
from pyfixmsg_plus.fixengine.message_handler import TestRequestHandler as TestRequestHandlerClass


@pytest.fixture
def mock_dependencies():
    """Create mock dependencies for message handlers."""
    mock_message_store = Mock()
    mock_state_machine = Mock()
    mock_state_machine.state = Mock()
    mock_state_machine.state.name = 'LOGGED_IN'
    
    mock_application = Mock()
    mock_application.fromAdmin = AsyncMock()
    mock_application.fromApp = AsyncMock()
    
    mock_engine = Mock()
    mock_engine.session_id = "TEST_SESSION"
    mock_engine.codec = Mock()
    mock_engine.mode = 'acceptor'  # Set proper mode
    mock_engine.send_logout_message = AsyncMock()
    mock_engine.send_reject_message = AsyncMock()
    mock_engine.send_heartbeat = AsyncMock()
    mock_engine.send_message = AsyncMock()
    mock_engine.fixmsg = Mock(return_value={'35': '0'})
    
    return {
        'message_store': mock_message_store,
        'state_machine': mock_state_machine,
        'application': mock_application,
        'engine': mock_engine
    }


@pytest.fixture
def sample_logon_message():
    """Create a sample FIX logon message."""
    class FixMessage(dict):
        """Mock FIX message that behaves like the real FixMessage."""
        def get(self, key, default=None):
            return super().get(str(key), default)
    
    return FixMessage({
        '8': 'FIX.4.4',      # BeginString
        '35': 'A',           # MsgType (Logon)
        '49': 'SENDER',      # SenderCompID
        '56': 'TARGET',      # TargetCompID
        '34': '1',           # MsgSeqNum
        '52': '20250726-12:00:00.000',  # SendingTime
        '98': '0',           # EncryptMethod
        '108': '30',         # HeartBtInt
        '141': 'N'           # ResetSeqNumFlag
    })


@pytest.fixture
def sample_heartbeat_message():
    """Create a sample FIX heartbeat message.""" 
    class FixMessage(dict):
        """Mock FIX message that behaves like the real FixMessage."""
        def get(self, key, default=None):
            return super().get(str(key), default)
    
    return FixMessage({
        '8': 'FIX.4.4',      # BeginString
        '35': '0',           # MsgType (Heartbeat)
        '49': 'SENDER',      # SenderCompID
        '56': 'TARGET',      # TargetCompID
        '34': '2',           # MsgSeqNum
        '52': '20250726-12:00:00.000',  # SendingTime
    })


@pytest.mark.unit
class TestMessageProcessor:
    """Test the MessageProcessor - the main message coordinator."""
    
    def test_message_processor_initialization(self, mock_dependencies):
        """Test MessageProcessor initialization."""
        processor = MessageProcessor(**mock_dependencies)
        
        assert processor.handlers == {}
        assert processor.message_store == mock_dependencies['message_store']
        assert processor.state_machine == mock_dependencies['state_machine']
        assert processor.application == mock_dependencies['application']
        assert processor.engine == mock_dependencies['engine']
        assert hasattr(processor, 'logger')
    
    def test_register_handler(self, mock_dependencies):
        """Test handler registration."""
        processor = MessageProcessor(**mock_dependencies)
        handler = LogonHandler(**mock_dependencies)
        
        processor.register_handler('A', handler)
        
        assert 'A' in processor.handlers
        assert processor.handlers['A'] == handler
        assert isinstance(processor.handlers['A'], LogonHandler)
    
    def test_register_multiple_handlers(self, mock_dependencies):
        """Test registering multiple message handlers."""
        processor = MessageProcessor(**mock_dependencies)
        
        handlers = {
            'A': LogonHandler(**mock_dependencies),
            '0': HeartbeatHandler(**mock_dependencies),
            '1': TestRequestHandlerClass(**mock_dependencies),
            '5': LogoutHandler(**mock_dependencies)
        }
        
        for msg_type, handler in handlers.items():
            processor.register_handler(msg_type, handler)
        
        assert len(processor.handlers) == 4
        for msg_type, handler in handlers.items():
            assert processor.handlers[msg_type] == handler
    
    @pytest.mark.asyncio
    async def test_process_message_with_registered_handler(self, mock_dependencies, sample_logon_message):
        """Test processing message with registered handler."""
        processor = MessageProcessor(**mock_dependencies)
        
        # Create mock handler with async handle method
        mock_handler = Mock()
        mock_handler.handle = AsyncMock()
        
        processor.register_handler('A', mock_handler)
        
        await processor.process_message(sample_logon_message)
        
        mock_handler.handle.assert_called_once_with(sample_logon_message)
    
    @pytest.mark.asyncio
    async def test_process_message_without_registered_handler(self, mock_dependencies):
        """Test processing message without registered handler."""
        processor = MessageProcessor(**mock_dependencies)
        mock_dependencies['state_machine'].state.name = 'ACTIVE'
        mock_dependencies['engine'].send_reject_message = AsyncMock()
        
        # Unknown admin message type
        class FixMessage(dict):
            def get(self, key, default=None):
                return super().get(str(key), default)
        
        unknown_message = FixMessage({
            '35': 'Z',  # Unknown admin message type
            '34': '1'   # MsgSeqNum
        })
        
        await processor.process_message(unknown_message)
        
        # Should send reject for unknown admin message
        mock_dependencies['engine'].send_reject_message.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_process_message_application_message(self, mock_dependencies):
        """Test processing unhandled application message.""" 
        processor = MessageProcessor(**mock_dependencies)
        mock_dependencies['state_machine'].state.name = 'ACTIVE'
        mock_dependencies['application'].onMessage = AsyncMock()
        mock_dependencies['engine'].send_reject_message = AsyncMock()
        
        # Use a lowercase type that should be treated as application message
        class FixMessage(dict):
            def get(self, key, default=None):
                return super().get(str(key), default)
        
        app_message = FixMessage({
            '35': 'ao',  # Multi-char lowercase - should be application message
            '34': '1'    # MsgSeqNum
        })
        
        await processor.process_message(app_message)
        
        # Should pass to application.onMessage for non-admin types
        mock_dependencies['application'].onMessage.assert_called_once_with(app_message)


@pytest.mark.unit
class TestBaseMessageHandler:
    """Test the base MessageHandler class."""
    
    def test_message_handler_initialization(self, mock_dependencies):
        """Test MessageHandler base class initialization."""
        handler = MessageHandler(**mock_dependencies)
        
        assert handler.message_store == mock_dependencies['message_store']
        assert handler.state_machine == mock_dependencies['state_machine']
        assert handler.application == mock_dependencies['application']
        assert handler.engine == mock_dependencies['engine']
        assert hasattr(handler, 'logger')
    
    @pytest.mark.asyncio
    async def test_message_handler_handle_not_implemented(self, mock_dependencies):
        """Test base MessageHandler raises NotImplementedError."""
        handler = MessageHandler(**mock_dependencies)
        
        with pytest.raises(NotImplementedError):
            await handler.handle({})


@pytest.mark.unit
class TestLogonHandler:
    """Test LogonHandler - covers critical logon logic."""
    
    def test_logon_handler_initialization(self, mock_dependencies):
        """Test LogonHandler initialization."""
        handler = LogonHandler(**mock_dependencies)
        
        assert isinstance(handler, MessageHandler)
        assert hasattr(handler, 'logger')
    
    @pytest.mark.asyncio
    async def test_logon_handler_valid_message(self, mock_dependencies, sample_logon_message):
        """Test LogonHandler with valid logon message."""
        handler = LogonHandler(**mock_dependencies)
        
        # Mock engine dependencies for logon processing
        mock_dependencies['engine'].target = 'SENDER'
        mock_dependencies['engine'].sender = 'TARGET'
        mock_dependencies['engine'].heartbeat_interval = 30
        mock_dependencies['engine'].heartbeat = Mock()
        mock_dependencies['engine'].heartbeat.set_remote_interval = Mock()
        mock_dependencies['engine'].heartbeat.start = AsyncMock()
        mock_dependencies['engine'].reset_sequence_numbers = AsyncMock()
        mock_dependencies['message_store'].get_next_incoming_sequence_number = Mock(return_value=1)
        mock_dependencies['state_machine'].on_event = Mock()
        
        await handler.handle(sample_logon_message)
        
        # Should call state machine for acceptor logon
        mock_dependencies['state_machine'].on_event.assert_called_with('logon_received_valid')

    @pytest.mark.asyncio
    @pytest.mark.asyncio
    async def test_logon_handler_invalid_message_type(self, mock_dependencies):
        """Test LogonHandler rejects non-logon messages."""
        handler = LogonHandler(**mock_dependencies)
        
        # Non-logon message
        invalid_message = {
            '35': '0',  # Heartbeat, not Logon
            '34': '1'
        }
        
        # Mock logger to capture error
        handler.logger = Mock()
        
        await handler.handle(invalid_message)
        
        # Should log error for non-logon message
        handler.logger.error.assert_called_once()


@pytest.mark.unit
class TestSpecificMessageHandlers:
    """Test specific message handler implementations."""
    
    def test_all_handler_classes_exist(self, mock_dependencies):
        """Test that all handler classes can be instantiated."""
        handler_classes = [
            LogonHandler, TestRequestHandlerClass, ExecutionReportHandler,
            NewOrderHandler, CancelOrderHandler, OrderCancelReplaceHandler,
            OrderCancelRejectHandler, NewOrderMultilegHandler,
            MultilegOrderCancelReplaceHandler, ResendRequestHandler,
            SequenceResetHandler, RejectHandler, LogoutHandler, HeartbeatHandler
        ]
        
        for handler_class in handler_classes:
            handler = handler_class(**mock_dependencies)
            assert isinstance(handler, MessageHandler)
            assert hasattr(handler, 'handle')
            assert callable(handler.handle)
            print(f"✅ {handler_class.__name__} instantiated successfully")
    
    @pytest.mark.asyncio
    async def test_heartbeat_handler(self, mock_dependencies, sample_heartbeat_message):
        """Test HeartbeatHandler functionality."""
        handler = HeartbeatHandler(**mock_dependencies)
        mock_dependencies['application'].fromAdmin = AsyncMock()
        
        await handler.handle(sample_heartbeat_message)
        
        # Should call application.fromAdmin for heartbeat
        mock_dependencies['application'].fromAdmin.assert_called_once()
    
    @pytest.mark.asyncio
    async def test_test_request_handler(self, mock_dependencies):
        """Test TestRequestHandler functionality."""
        handler = TestRequestHandlerClass(**mock_dependencies)
        mock_dependencies['engine'].send_heartbeat = AsyncMock()
        mock_dependencies['engine'].send_reject_message = AsyncMock()
        
        class FixMessage(dict):
            def get(self, key, default=None):
                return super().get(str(key), default)
        
        test_request_message = FixMessage({
            '35': '1',        # TestRequest
            '112': 'TEST123', # TestReqID
            '34': '1'
        })
        
        await handler.handle(test_request_message)
        
        # Should send heartbeat response using send_message
        mock_dependencies['engine'].send_message.assert_called_once()


@pytest.mark.unit
class TestLoggingDecorator:
    """Test the logging decorator functionality."""
    
    @pytest.mark.asyncio
    async def test_logging_decorator_with_logger(self, mock_dependencies):
        """Test logging decorator with proper logger."""
        # Create a test handler with the decorator
        class TestHandler(MessageHandler):
            @logging_decorator
            async def handle(self, message):
                return "handled"
        
        handler = TestHandler(**mock_dependencies)
        handler.logger = Mock()
        
        class FixMessage(dict):
            def get(self, key, default=None):
                return super().get(str(key), default)
        
        test_message = FixMessage({
            '35': 'A',
            '34': '1'
        })
        
        result = await handler.handle(test_message)
        
        assert result == "handled"
        # Should have called logger.debug twice (before and after)
        assert handler.logger.debug.call_count == 2
    
    @pytest.mark.asyncio
    async def test_logging_decorator_without_logger(self, mock_dependencies, capsys):
        """Test logging decorator fallback when no logger."""
        # Create a test handler with the decorator
        class TestHandler(MessageHandler):
            @logging_decorator
            async def handle(self, message):
                return "handled"
        
        handler = TestHandler(**mock_dependencies)
        handler.logger = None  # No logger
        
        class FixMessage(dict):
            def get(self, key, default=None):
                return super().get(str(key), default)
        
        test_message = FixMessage({
            '35': 'A',
            '34': '1'
        })
        
        result = await handler.handle(test_message)
        
        assert result == "handled"
        # Should have used print fallback
        captured = capsys.readouterr()
        assert "Fallback Logging" in captured.out


@pytest.mark.unit
class TestMessageHandlerIntegration:
    """Test integration between MessageProcessor and handlers."""
    
    @pytest.mark.asyncio
    async def test_full_message_processing_flow(self, mock_dependencies, sample_logon_message):
        """Test complete message processing flow."""
        processor = MessageProcessor(**mock_dependencies)

        # Register handlers
        logon_handler = LogonHandler(**mock_dependencies)
        heartbeat_handler = HeartbeatHandler(**mock_dependencies)

        processor.register_handler('A', logon_handler)
        processor.register_handler('0', heartbeat_handler)

        # Mock engine dependencies for logon processing
        mock_dependencies['engine'].target = 'SENDER'
        mock_dependencies['engine'].sender = 'TARGET'
        mock_dependencies['engine'].heartbeat_interval = 30
        mock_dependencies['engine'].heartbeat = Mock()
        mock_dependencies['engine'].heartbeat.set_remote_interval = Mock()
        mock_dependencies['engine'].heartbeat.start = AsyncMock()
        mock_dependencies['message_store'].get_next_incoming_sequence_number = Mock(return_value=1)
        mock_dependencies['state_machine'].on_event = Mock()

        # Process logon message
        await processor.process_message(sample_logon_message)

        # Should have called the logon handler and state machine
        mock_dependencies['state_machine'].on_event.assert_called_with('logon_received_valid')
        
        assert len(processor.handlers) == 2
        assert 'A' in processor.handlers
        assert '0' in processor.handlers


# Coverage targeting summary:
"""
MESSAGE HANDLER COVERAGE STRATEGY:

✅ HIGH-IMPACT TARGETS:
- MessageProcessor: Core coordinator (~50 lines)
- Base MessageHandler: Foundation class (~15 lines)  
- LogonHandler: Critical authentication (~80 lines)
- HeartbeatHandler: Session management (~30 lines)
- TestRequestHandler: Network keep-alive (~25 lines)
- Logging decorator: Cross-cutting concern (~20 lines)

✅ COVERAGE TECHNIQUES:
- Mock all dependencies (message_store, state_machine, application, engine)
- Test initialization, registration, and message processing
- Cover both success and error paths
- Test decorator functionality
- Integration testing between processor and handlers

✅ EXPECTED IMPACT:
- Current coverage: 23% (72/313 lines)
- Target coverage: 60%+ (190+ lines)
- Coverage gain: +37% for message_handler.py
- Overall project impact: +3-5% total coverage

✅ NEXT STEPS:
- Add these tests to test_runner.py
- Measure actual coverage impact
- Target database_message_store.py next (185 lines, 29% coverage)
"""
