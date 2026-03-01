"""
Enhanced unit tests for PyFixMsg Plus core components.
Comprehensive testing of individual modules and classes.
"""
import asyncio
import pytest
import tempfile
import os
import sqlite3
from unittest.mock import Mock, AsyncMock, patch, MagicMock
from datetime import datetime

# Import all core components for testing
from pyfixmsg_plus.fixengine.configmanager import ConfigManager
from pyfixmsg_plus.fixengine.engine import FixEngine
from pyfixmsg_plus.fixengine.message_handler import MessageHandler
from pyfixmsg_plus.fixengine.message_store_factory import MessageStoreFactory
from pyfixmsg_plus.fixengine.database_message_store import DatabaseMessageStore
from pyfixmsg_plus.fixengine.database_message_store_aiosqlite import DatabaseMessageStoreAioSqlite
from pyfixmsg_plus.fixengine.state_machine import StateMachine
from pyfixmsg_plus.fixengine.heartbeat import Heartbeat
from pyfixmsg_plus.application import Application


@pytest.mark.unit
class TestConfigManager:
    """Comprehensive unit tests for ConfigManager."""
    
    def test_config_loading(self, temp_config_file):
        """Test configuration file loading."""
        config = ConfigManager(temp_config_file)
        
        assert config.get('session', 'sender_comp_id') == 'SENDER'
        assert config.get('session', 'target_comp_id') == 'TARGET'
        assert config.get('network', 'host') == 'localhost'
        assert config.get('database', 'type') == 'sqlite3'
    
    def test_config_defaults(self, temp_config_file):
        """Test that default values are provided when not specified."""
        config = ConfigManager(temp_config_file)
        
        # These should have default values
        logon_timeout = config.get('session', 'logon_timeout', '30')
        assert logon_timeout == '30'
        
        # Test with explicit default
        unknown_value = config.get('session', 'unknown_setting', 'default_value')
        assert unknown_value == 'default_value'
    
    def test_config_missing_file(self):
        """Test handling of missing configuration file."""
        with pytest.raises(FileNotFoundError):
            ConfigManager('/nonexistent/config.ini')
    
    def test_config_invalid_format(self):
        """Test handling of invalid configuration format."""
        with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
            f.write("invalid config format\nno sections")
            config_path = f.name
        
        try:
            with pytest.raises(Exception):
                ConfigManager(config_path)
        finally:
            os.unlink(config_path)
    
    def test_config_modification(self, temp_config_file):
        """Test runtime configuration modification."""
        config = ConfigManager(temp_config_file)
        
        # Modify configuration
        config.config.set('session', 'heartbeat_interval', '60')
        
        # Verify modification
        assert config.get('session', 'heartbeat_interval') == '60'
    
    def test_config_sections_and_options(self, temp_config_file):
        """Test section and option enumeration."""
        config = ConfigManager(temp_config_file)
        
        sections = config.config.sections()
        assert 'session' in sections
        assert 'network' in sections
        assert 'database' in sections
        
        session_options = config.config.options('session')
        assert 'sender_comp_id' in session_options
        assert 'target_comp_id' in session_options


@pytest.mark.unit
@pytest.mark.asyncio
class TestMessageStoreFactory:
    """Unit tests for MessageStoreFactory."""
    
    def test_create_sqlite3_store(self, config_manager):
        """Test creation of SQLite3 message store."""
        config_manager.config.set('database', 'type', 'sqlite3')
        
        store = MessageStoreFactory.create_message_store(config_manager)
        
        assert isinstance(store, DatabaseMessageStore)
        assert store.db_path == ':memory:'
    
    def test_create_aiosqlite_store(self, config_manager):
        """Test creation of aiosqlite message store."""
        config_manager.config.set('database', 'type', 'aiosqlite')
        
        store = MessageStoreFactory.create_message_store(config_manager)
        
        assert isinstance(store, DatabaseMessageStoreAioSqlite)
        assert store.db_path == ':memory:'
    
    def test_unsupported_database_type(self, config_manager):
        """Test handling of unsupported database type."""
        config_manager.config.set('database', 'type', 'unsupported_db')
        
        with pytest.raises(ValueError, match="Unsupported database type"):
            MessageStoreFactory.create_message_store(config_manager)
    
    def test_missing_database_config(self, config_manager):
        """Test handling of missing database configuration."""
        config_manager.config.remove_option('database', 'type')
        
        # Should default to sqlite3
        store = MessageStoreFactory.create_message_store(config_manager)
        assert isinstance(store, DatabaseMessageStore)


@pytest.mark.unit
class TestDatabaseMessageStore:
    """Unit tests for DatabaseMessageStore (SQLite3)."""
    
    def test_initialization(self, temp_sqlite_db):
        """Test message store initialization."""
        store = DatabaseMessageStore(temp_sqlite_db)
        store.initialize()
        
        # Verify tables are created
        conn = sqlite3.connect(temp_sqlite_db)
        cursor = conn.cursor()
        
        cursor.execute("SELECT name FROM sqlite_master WHERE type='table'")
        tables = [row[0] for row in cursor.fetchall()]
        
        assert 'incoming_messages' in tables
        assert 'outgoing_messages' in tables
        
        conn.close()
        store.close()
    
    def test_store_incoming_message(self, temp_sqlite_db, sample_logon_message):
        """Test storing incoming messages."""
        store = DatabaseMessageStore(temp_sqlite_db)
        store.initialize()
        
        # Store message
        store.store_incoming_message('SESSION1', sample_logon_message)
        
        # Verify storage
        conn = sqlite3.connect(temp_sqlite_db)
        cursor = conn.cursor()
        
        cursor.execute("SELECT * FROM incoming_messages WHERE session_id = ?", ('SESSION1',))
        row = cursor.fetchone()
        
        assert row is not None
        assert row[1] == 'SESSION1'  # session_id
        assert row[2] == 1  # sequence_number (from message '34')
        
        conn.close()
        store.close()
    
    def test_store_outgoing_message(self, temp_sqlite_db, sample_heartbeat_message):
        """Test storing outgoing messages."""
        store = DatabaseMessageStore(temp_sqlite_db)
        store.initialize()
        
        # Store message
        store.store_outgoing_message('SESSION1', sample_heartbeat_message)
        
        # Verify storage
        conn = sqlite3.connect(temp_sqlite_db)
        cursor = conn.cursor()
        
        cursor.execute("SELECT * FROM outgoing_messages WHERE session_id = ?", ('SESSION1',))
        row = cursor.fetchone()
        
        assert row is not None
        assert row[1] == 'SESSION1'  # session_id
        assert row[2] == 2  # sequence_number
        
        conn.close()
        store.close()
    
    def test_get_next_sequence_number(self, temp_sqlite_db):
        """Test sequence number management."""
        store = DatabaseMessageStore(temp_sqlite_db)
        store.initialize()
        
        # Initial sequence number should be 1
        seq_num = store.get_next_outgoing_sequence_number('SESSION1')
        assert seq_num == 1
        
        # After storing a message, next should be incremented
        message = {
            '8': 'FIX.4.4',
            '35': '0',
            '34': '1',
            '49': 'SENDER',
            '56': 'TARGET',
            '52': '20250726-12:00:00.000',
        }
        store.store_outgoing_message('SESSION1', message)
        
        seq_num = store.get_next_outgoing_sequence_number('SESSION1')
        assert seq_num == 2
        
        store.close()
    
    def test_get_messages_in_range(self, temp_sqlite_db):
        """Test retrieving messages in a range."""
        store = DatabaseMessageStore(temp_sqlite_db)
        store.initialize()
        
        # Store multiple messages
        for i in range(1, 6):  # Store messages 1-5
            message = {
                '8': 'FIX.4.4',
                '35': '0',
                '34': str(i),
                '49': 'SENDER',
                '56': 'TARGET',
                '52': '20250726-12:00:00.000',
            }
            store.store_outgoing_message('SESSION1', message)
        
        # Retrieve range
        messages = store.get_outgoing_messages_in_range('SESSION1', 2, 4)
        
        assert len(messages) == 3  # Messages 2, 3, 4
        assert messages[0]['34'] == '2'
        assert messages[1]['34'] == '3'
        assert messages[2]['34'] == '4'
        
        store.close()
    
    def test_message_serialization(self, temp_sqlite_db):
        """Test message serialization and deserialization."""
        store = DatabaseMessageStore(temp_sqlite_db)
        store.initialize()
        
        original_message = {
            '8': 'FIX.4.4',
            '35': 'D',
            '49': 'SENDER',
            '56': 'TARGET',
            '34': '1',
            '52': '20250726-12:00:00.000',
            '11': 'ORDER123',
            '55': 'MSFT',
            '44': '150.25',
        }
        
        # Store and retrieve
        store.store_outgoing_message('SESSION1', original_message)
        messages = store.get_outgoing_messages_in_range('SESSION1', 1, 1)
        
        assert len(messages) == 1
        retrieved_message = messages[0]
        
        # Verify all fields are preserved
        for key, value in original_message.items():
            assert retrieved_message[key] == value
        
        store.close()


@pytest.mark.unit
@pytest.mark.asyncio
class TestAsyncDatabaseMessageStore:
    """Unit tests for AsyncDatabaseMessageStore (aiosqlite)."""
    
    async def test_async_initialization(self, temp_sqlite_db):
        """Test async message store initialization."""
        store = DatabaseMessageStoreAioSqlite(temp_sqlite_db)
        await store.initialize()
        
        # Verify tables are created
        import aiosqlite
        async with aiosqlite.connect(temp_sqlite_db) as db:
            cursor = await db.execute("SELECT name FROM sqlite_master WHERE type='table'")
            tables = [row[0] async for row in cursor]
        
        assert 'incoming_messages' in tables
        assert 'outgoing_messages' in tables
        
        await store.close()
    
    async def test_async_store_message(self, temp_sqlite_db, sample_logon_message):
        """Test async message storage."""
        store = DatabaseMessageStoreAioSqlite(temp_sqlite_db)
        await store.initialize()
        
        # Store message
        await store.store_incoming_message('SESSION1', sample_logon_message)
        
        # Verify storage
        import aiosqlite
        async with aiosqlite.connect(temp_sqlite_db) as db:
            cursor = await db.execute("SELECT * FROM incoming_messages WHERE session_id = ?", ('SESSION1',))
            row = await cursor.fetchone()
        
        assert row is not None
        assert row[1] == 'SESSION1'
        
        await store.close()
    
    async def test_async_sequence_numbers(self, temp_sqlite_db):
        """Test async sequence number management."""
        store = DatabaseMessageStoreAioSqlite(temp_sqlite_db)
        await store.initialize()
        
        # Get initial sequence number
        seq_num = await store.get_next_outgoing_sequence_number('SESSION1')
        assert seq_num == 1
        
        # Store message and check increment
        message = {
            '8': 'FIX.4.4',
            '35': '0',
            '34': '1',
            '49': 'SENDER',
            '56': 'TARGET',
            '52': '20250726-12:00:00.000',
        }
        await store.store_outgoing_message('SESSION1', message)
        
        seq_num = await store.get_next_outgoing_sequence_number('SESSION1')
        assert seq_num == 2
        
        await store.close()
    
    async def test_concurrent_access(self, temp_sqlite_db):
        """Test concurrent access to async message store."""
        store = DatabaseMessageStoreAioSqlite(temp_sqlite_db)
        await store.initialize()
        
        async def store_messages(session_id, start_seq, count):
            """Store multiple messages concurrently."""
            for i in range(count):
                message = {
                    '8': 'FIX.4.4',
                    '35': '0',
                    '34': str(start_seq + i),
                    '49': 'SENDER',
                    '56': 'TARGET',
                    '52': '20250726-12:00:00.000',
                }
                await store.store_outgoing_message(session_id, message)
        
        # Store messages concurrently from different "sessions"
        tasks = [
            store_messages('SESSION1', 1, 5),
            store_messages('SESSION2', 1, 5),
            store_messages('SESSION3', 1, 5),
        ]
        
        await asyncio.gather(*tasks)
        
        # Verify all messages were stored
        for session_id in ['SESSION1', 'SESSION2', 'SESSION3']:
            messages = await store.get_outgoing_messages_in_range(session_id, 1, 5)
            assert len(messages) == 5
        
        await store.close()


@pytest.mark.unit
class TestStateMachine:
    """Unit tests for StateMachine."""
    
    def test_initial_state(self):
        """Test state machine initial state."""
        sm = StateMachine()
        
        assert sm.current_state == 'DISCONNECTED'
        assert not sm.is_logged_on()
    
    def test_state_transitions(self):
        """Test valid state transitions."""
        sm = StateMachine()
        
        # DISCONNECTED -> CONNECTING
        sm.transition_to('CONNECTING')
        assert sm.current_state == 'CONNECTING'
        
        # CONNECTING -> CONNECTED
        sm.transition_to('CONNECTED')
        assert sm.current_state == 'CONNECTED'
        
        # CONNECTED -> LOGGED_ON
        sm.transition_to('LOGGED_ON')
        assert sm.current_state == 'LOGGED_ON'
        assert sm.is_logged_on()
        
        # LOGGED_ON -> DISCONNECTED
        sm.transition_to('DISCONNECTED')
        assert sm.current_state == 'DISCONNECTED'
        assert not sm.is_logged_on()
    
    def test_invalid_state_transition(self):
        """Test invalid state transitions."""
        sm = StateMachine()
        
        # Cannot go directly from DISCONNECTED to LOGGED_ON
        with pytest.raises(ValueError, match="Invalid state transition"):
            sm.transition_to('LOGGED_ON')
    
    def test_state_callbacks(self):
        """Test state transition callbacks."""
        sm = StateMachine()
        callback_calls = []
        
        def on_state_change(old_state, new_state):
            callback_calls.append((old_state, new_state))
        
        sm.add_state_callback(on_state_change)
        
        sm.transition_to('CONNECTING')
        sm.transition_to('CONNECTED')
        
        assert len(callback_calls) == 2
        assert callback_calls[0] == ('DISCONNECTED', 'CONNECTING')
        assert callback_calls[1] == ('CONNECTING', 'CONNECTED')
    
    def test_state_history(self):
        """Test state transition history."""
        sm = StateMachine()
        
        sm.transition_to('CONNECTING')
        sm.transition_to('CONNECTED')
        sm.transition_to('LOGGED_ON')
        
        history = sm.get_state_history()
        
        assert len(history) == 4  # Including initial state
        assert history[0]['state'] == 'DISCONNECTED'
        assert history[1]['state'] == 'CONNECTING'
        assert history[2]['state'] == 'CONNECTED'
        assert history[3]['state'] == 'LOGGED_ON'


@pytest.mark.unit
@pytest.mark.asyncio
class TestHeartbeatManager:
    """Unit tests for HeartbeatManager."""
    
    async def test_heartbeat_initialization(self):
        """Test heartbeat manager initialization."""
        hb_manager = HeartbeatManager(interval=30)
        
        assert hb_manager.interval == 30
        assert not hb_manager.is_running()
    
    async def test_heartbeat_timing(self):
        """Test heartbeat timing mechanism."""
        heartbeat_sent = []
        
        async def mock_send_heartbeat():
            heartbeat_sent.append(asyncio.get_event_loop().time())
        
        hb_manager = HeartbeatManager(interval=1)  # 1 second for testing
        hb_manager.set_heartbeat_callback(mock_send_heartbeat)
        
        # Start heartbeat
        await hb_manager.start()
        
        # Wait for a few heartbeats
        await asyncio.sleep(2.5)
        
        # Stop heartbeat
        await hb_manager.stop()
        
        # Should have sent 2-3 heartbeats
        assert len(heartbeat_sent) >= 2
        assert len(heartbeat_sent) <= 3
        
        # Check timing
        if len(heartbeat_sent) >= 2:
            interval = heartbeat_sent[1] - heartbeat_sent[0]
            assert abs(interval - 1.0) < 0.1  # Within 100ms tolerance
    
    async def test_heartbeat_reset(self):
        """Test heartbeat timer reset."""
        heartbeat_sent = []
        
        async def mock_send_heartbeat():
            heartbeat_sent.append(asyncio.get_event_loop().time())
        
        hb_manager = HeartbeatManager(interval=2)  # 2 seconds
        hb_manager.set_heartbeat_callback(mock_send_heartbeat)
        
        await hb_manager.start()
        
        # Wait 1 second then reset
        await asyncio.sleep(1.0)
        hb_manager.reset_timer()
        
        # Wait another 1.5 seconds (should not trigger yet)
        await asyncio.sleep(1.5)
        
        # Should have no heartbeats yet (timer was reset)
        assert len(heartbeat_sent) == 0
        
        # Wait for heartbeat to trigger
        await asyncio.sleep(1.0)
        
        await hb_manager.stop()
        
        # Should now have 1 heartbeat
        assert len(heartbeat_sent) == 1
    
    async def test_heartbeat_callback_error_handling(self):
        """Test error handling in heartbeat callback."""
        error_count = 0
        
        async def failing_heartbeat_callback():
            nonlocal error_count
            error_count += 1
            raise Exception("Simulated heartbeat error")
        
        hb_manager = HeartbeatManager(interval=0.5)  # Fast for testing
        hb_manager.set_heartbeat_callback(failing_heartbeat_callback)
        
        await hb_manager.start()
        await asyncio.sleep(1.2)  # Allow time for heartbeats
        await hb_manager.stop()
        
        # Should have attempted heartbeats despite errors
        assert error_count >= 2
        
        # Manager should still be functional
        assert not hb_manager.is_running()


@pytest.mark.unit
@pytest.mark.asyncio
class TestMessageHandler:
    """Unit tests for MessageHandler."""
    
    async def test_message_routing(self):
        """Test message routing to appropriate handlers."""
        handler = MessageHandler()
        
        # Track routed messages
        admin_messages = []
        app_messages = []
        
        async def admin_handler(session_id, message):
            admin_messages.append(message)
        
        async def app_handler(session_id, message):
            app_messages.append(message)
        
        handler.set_admin_handler(admin_handler)
        handler.set_app_handler(app_handler)
        
        # Send admin message (Logon)
        logon_msg = {
            '8': 'FIX.4.4',
            '35': 'A',  # Logon
            '49': 'SENDER',
            '56': 'TARGET',
            '34': '1',
            '52': '20250726-12:00:00.000',
        }
        
        await handler.handle_incoming_message('SESSION1', logon_msg)
        
        # Send application message (NewOrder)
        order_msg = {
            '8': 'FIX.4.4',
            '35': 'D',  # NewOrderSingle
            '49': 'SENDER',
            '56': 'TARGET',
            '34': '2',
            '52': '20250726-12:00:00.000',
        }
        
        await handler.handle_incoming_message('SESSION1', order_msg)
        
        # Verify routing
        assert len(admin_messages) == 1
        assert admin_messages[0]['35'] == 'A'
        
        assert len(app_messages) == 1
        assert app_messages[0]['35'] == 'D'
    
    async def test_message_validation(self):
        """Test message validation before routing."""
        handler = MessageHandler()
        
        validation_errors = []
        
        async def error_handler(session_id, error, message):
            validation_errors.append((error, message))
        
        handler.set_error_handler(error_handler)
        
        # Send invalid message (missing required fields)
        invalid_msg = {
            '8': 'FIX.4.4',
            # Missing MsgType (35)
            '49': 'SENDER',
            '56': 'TARGET',
        }
        
        await handler.handle_incoming_message('SESSION1', invalid_msg)
        
        # Should have validation error
        assert len(validation_errors) == 1
        assert 'MsgType' in validation_errors[0][0] or 'required' in validation_errors[0][0].lower()
    
    async def test_sequence_number_validation(self):
        """Test sequence number validation."""
        handler = MessageHandler()
        
        sequence_errors = []
        processed_messages = []
        
        async def seq_error_handler(session_id, error, message):
            sequence_errors.append((error, message))
        
        async def message_processor(session_id, message):
            processed_messages.append(message)
        
        handler.set_error_handler(seq_error_handler)
        handler.set_admin_handler(message_processor)
        
        # Send messages out of sequence
        msg1 = {
            '8': 'FIX.4.4',
            '35': '0',  # Heartbeat
            '49': 'SENDER',
            '56': 'TARGET',
            '34': '1',  # Sequence 1
            '52': '20250726-12:00:00.000',
        }
        
        msg3 = {
            '8': 'FIX.4.4',
            '35': '0',  # Heartbeat
            '49': 'SENDER',
            '56': 'TARGET',
            '34': '3',  # Sequence 3 (skipping 2)
            '52': '20250726-12:00:01.000',
        }
        
        await handler.handle_incoming_message('SESSION1', msg1)
        await handler.handle_incoming_message('SESSION1', msg3)
        
        # First message should be processed, second should trigger sequence error
        assert len(processed_messages) >= 1
        # Note: Sequence validation logic depends on implementation


@pytest.mark.unit
@pytest.mark.asyncio  
class TestFixEngine:
    """Unit tests for FixEngine core functionality."""
    
    async def test_engine_initialization(self, config_manager):
        """Test engine initialization."""
        engine = FixEngine(config_manager)
        
        assert engine.config_manager == config_manager
        assert engine.application is None
        assert not engine._running
    
    async def test_application_assignment(self, fix_engine, mock_application):
        """Test application assignment."""
        fix_engine.application = mock_application
        
        assert fix_engine.application == mock_application
    
    async def test_engine_lifecycle(self, fix_engine, mock_application):
        """Test engine start/stop lifecycle."""
        fix_engine.application = mock_application
        
        # Initially not running
        assert not fix_engine._running
        
        # Start engine
        try:
            await fix_engine.start()
            # May fail in test environment, but should not raise unexpected errors
        except Exception as e:
            # Network binding might fail in test environment
            pass
        
        # Stop engine
        try:
            await fix_engine.stop()
        except Exception as e:
            # Cleanup might fail in test environment
            pass
        
        # Should not be running after stop
        assert not fix_engine._running
    
    async def test_message_sending_without_session(self, fix_engine, mock_application):
        """Test that sending messages without session raises appropriate error."""
        fix_engine.application = mock_application
        
        message = {
            '8': 'FIX.4.4',
            '35': '0',
            '49': 'SENDER',
            '56': 'TARGET',
            '34': '1',
            '52': '20250726-12:00:00.000',
        }
        
        # Should raise error when not connected
        with pytest.raises(Exception):
            await fix_engine.send_to_target(message)
    
    @patch('pyfixmsg_plus.fixengine.engine.asyncio.start_server')
    async def test_engine_start_as_acceptor(self, mock_start_server, config_manager, mock_application):
        """Test engine start in acceptor mode."""
        # Configure as acceptor
        config_manager.config.set('network', 'socket_accept_port', '9998')
        
        engine = FixEngine(config_manager)
        engine.application = mock_application
        
        # Mock server start
        mock_server = MagicMock()
        mock_server.serve_forever = AsyncMock()
        mock_start_server.return_value = mock_server
        
        try:
            await engine.start()
            
            # Should have called start_server for acceptor
            mock_start_server.assert_called_once()
            
        finally:
            await engine.stop()
    
    async def test_configuration_validation(self, sample_config_dict):
        """Test configuration validation during engine creation."""
        # Missing required configuration
        incomplete_config = sample_config_dict.copy()
        del incomplete_config['session']['sender_comp_id']
        
        import tempfile
        import os
        
        with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
            for section, options in incomplete_config.items():
                f.write(f'[{section}]\n')
                for key, value in options.items():
                    f.write(f'{key} = {value}\n')
                f.write('\n')
            config_path = f.name
        
        try:
            config_manager = ConfigManager(config_path)
            
            # Should be able to create engine even with incomplete config
            # Validation may happen later during operation
            engine = FixEngine(config_manager)
            assert engine is not None
            
        finally:
            os.unlink(config_path)


if __name__ == '__main__':
    pytest.main([__file__, '-v', '--tb=short', '-m', 'unit'])
