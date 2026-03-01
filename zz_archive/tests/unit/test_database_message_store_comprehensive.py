"""
High-impact unit tests for DatabaseMessageStore.
This file targets the 185-line database message store for maximum Phase 2 coverage.
"""
import pytest
import asyncio
import sqlite3
from unittest.mock import Mock, AsyncMock, patch, MagicMock
from pathlib import Path
import tempfile
import os

from pyfixmsg_plus.fixengine.database_message_store import DatabaseMessageStore


@pytest.fixture
def temp_db_path():
    """Create a temporary database file for testing."""
    with tempfile.NamedTemporaryFile(suffix='.db', delete=False) as tmp:
        db_path = tmp.name
    yield db_path
    # Cleanup
    if os.path.exists(db_path):
        os.unlink(db_path)


@pytest.fixture
def mock_config_manager():
    """Create mock configuration manager - not used by actual DatabaseMessageStore."""
    config = Mock()
    config.get.return_value = 'test_session'
    return config


@pytest.fixture
def sample_fix_message():
    """Create a sample FIX message for testing."""
    class FixMessage(dict):
        """Mock FIX message that behaves like the real FixMessage."""
        def get(self, key, default=None):
            return super().get(str(key), default)
            
        def __str__(self):
            return f"FIX_MESSAGE: {dict(self)}"
    
    return FixMessage({
        '8': 'FIX.4.4',      # BeginString
        '35': 'D',           # MsgType (New Order Single)
        '49': 'SENDER',      # SenderCompID
        '56': 'TARGET',      # TargetCompID
        '34': '1',           # MsgSeqNum
        '52': '20250726-12:00:00.000',  # SendingTime
        '55': 'AAPL',        # Symbol
        '54': '1',           # Side (Buy)
        '38': '100',         # OrderQty
        '40': '2',           # OrdType (Limit)
        '44': '150.00'       # Price
    })


@pytest.mark.unit
class TestDatabaseMessageStoreInitialization:
    """Test DatabaseMessageStore initialization and setup."""
    
    def test_initialization_with_valid_path(self, temp_db_path, mock_config_manager):
        """Test successful initialization with valid database path."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        
        assert store.db_path == temp_db_path
        assert store.beginstring == 'FIX.4.4'
        assert store.sendercompid == 'SENDER'
        assert store.targetcompid == 'TARGET'
        assert hasattr(store, 'logger')
        assert store.incoming_seqnum == 1
        assert store.outgoing_seqnum == 1
    
    @pytest.mark.asyncio
    async def test_initialization_creates_database(self, temp_db_path, mock_config_manager):
        """Test that initialization creates the database and tables."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Verify database file exists
        assert os.path.exists(temp_db_path)
        
        # Verify tables were created
        conn = sqlite3.connect(temp_db_path)
        cursor = conn.cursor()
        
        # Check if tables exist
        cursor.execute("SELECT name FROM sqlite_master WHERE type='table'")
        tables = [row[0] for row in cursor.fetchall()]
        
        expected_tables = ['messages', 'sessions']
        for table in expected_tables:
            assert table in tables
        
        conn.close()
        await store.close()
    
    def test_initialization_with_invalid_path(self, mock_config_manager):
        """Test handling of invalid database path."""
        invalid_path = "/invalid/path/database.db"
        
        with pytest.raises(Exception):  # Should raise some exception for invalid path
            store = DatabaseMessageStore(invalid_path, 'FIX.4.4', 'SENDER', 'TARGET')


@pytest.mark.unit
class TestDatabaseMessageStoreSequenceNumbers:
    """Test sequence number management functionality."""
    
    @pytest.mark.asyncio
    async def test_get_next_outgoing_sequence_number(self, temp_db_path, mock_config_manager):
        """Test getting next outgoing sequence number."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # First call should return 1
        seq_num = store.get_next_outgoing_sequence_number()
        assert seq_num == 1
        
        # Increment and check
        await store.increment_outgoing_sequence_number()
        seq_num = store.get_next_outgoing_sequence_number()
        assert seq_num == 2
        
        # Third call should return 3
        await store.increment_outgoing_sequence_number()
        seq_num = store.get_next_outgoing_sequence_number()
        assert seq_num == 3
        
        await store.close()
    
    @pytest.mark.asyncio
    async def test_get_next_incoming_sequence_number(self, temp_db_path, mock_config_manager):
        """Test getting next incoming sequence number."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # First call should return 1
        seq_num = store.get_next_incoming_sequence_number()
        assert seq_num == 1
        
        # Should increment
        await store.increment_incoming_sequence_number()
        seq_num = store.get_next_incoming_sequence_number()
        assert seq_num == 2
        
        await store.close()
    
    @pytest.mark.asyncio
    async def test_set_outgoing_sequence_number(self, temp_db_path, mock_config_manager):
        """Test setting outgoing sequence number."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Set to specific value
        await store.set_outgoing_sequence_number(100)
        
        # Next call should return 100
        seq_num = store.get_next_outgoing_sequence_number()
        assert seq_num == 100
        
        await store.close()
    
    @pytest.mark.asyncio
    async def test_set_incoming_sequence_number(self, temp_db_path, mock_config_manager):
        """Test setting incoming sequence number."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Set to specific value
        await store.set_incoming_sequence_number(50)
        
        # Next call should return 50
        seq_num = store.get_next_incoming_sequence_number()
        assert seq_num == 50
        
        # After increment, should return 51
        await store.increment_incoming_sequence_number()
        seq_num = store.get_next_incoming_sequence_number()
        assert seq_num == 51
        
        await store.close()


@pytest.mark.unit
class TestDatabaseMessageStoreMessages:
    """Test message storage and retrieval."""
    
    @pytest.mark.asyncio
    async def test_store_and_get_message(self, temp_db_path, mock_config_manager, sample_fix_message):
        """Test storing and retrieving messages."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Store message
        seq_num = 1
        await store.store_message('FIX.4.4', 'SENDER', 'TARGET', seq_num, str(sample_fix_message))
        
        # Retrieve message
        retrieved_message = await store.get_message('FIX.4.4', 'SENDER', 'TARGET', seq_num)
        
        assert retrieved_message is not None
        assert 'AAPL' in retrieved_message  # Check message content
        
        await store.close()
    
    @pytest.mark.asyncio
    async def test_get_message_not_found(self, temp_db_path, mock_config_manager):
        """Test retrieving non-existent message."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Try to get non-existent message
        retrieved_message = await store.get_message('FIX.4.4', 'SENDER', 'TARGET', 999)
        
        assert retrieved_message is None
        
        await store.close()
    
    @pytest.mark.asyncio
    async def test_store_multiple_messages(self, temp_db_path, mock_config_manager, sample_fix_message):
        """Test storing multiple messages."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Store multiple messages
        for i in range(1, 6):  # Store messages 1-5
            msg = sample_fix_message.copy()
            msg['34'] = str(i)  # Different sequence numbers
            await store.store_message('FIX.4.4', 'SENDER', 'TARGET', i, str(msg))
        
        # Verify all messages can be retrieved
        for i in range(1, 6):
            retrieved = await store.get_message('FIX.4.4', 'SENDER', 'TARGET', i)
            assert retrieved is not None
            assert f"'34': '{i}'" in retrieved
        
        await store.close()


@pytest.mark.unit  
class TestDatabaseMessageStoreReset:
    """Test reset functionality."""
    
    @pytest.mark.asyncio
    async def test_reset_sequence_numbers(self, temp_db_path, mock_config_manager):
        """Test resetting sequence numbers."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Set sequence numbers to non-default values
        await store.set_outgoing_sequence_number(50)
        await store.set_incoming_sequence_number(75)
        
        # Reset
        await store.reset_sequence_numbers()
        
        # Should be back to 1
        assert store.get_next_outgoing_sequence_number() == 1
        assert store.get_next_incoming_sequence_number() == 1
        
        await store.close()
    
    @pytest.mark.asyncio
    async def test_is_new_session(self, temp_db_path, mock_config_manager):
        """Test new session detection."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Should start as new session
        assert store.is_new_session() == True
        
        # After incrementing, should no longer be new
        await store.increment_outgoing_sequence_number()
        assert store.is_new_session() == False
        
        # After reset, should be new again
        await store.reset_sequence_numbers()
        assert store.is_new_session() == True
        
        await store.close()


@pytest.mark.unit
class TestDatabaseMessageStoreErrorHandling:
    """Test error handling and edge cases."""
    
    def test_database_connection_error_handling(self, mock_config_manager):
        """Test handling of database connection errors."""
        # Use an invalid path that should cause connection issues
        invalid_path = "/nonexistent/readonly/path/database.db"
        
        with pytest.raises(Exception):  # Should raise exception due to invalid path
            store = DatabaseMessageStore(invalid_path, 'FIX.4.4', 'SENDER', 'TARGET')
    
    @pytest.mark.asyncio
    async def test_invalid_sequence_number_types(self, temp_db_path, mock_config_manager):
        """Test handling of invalid sequence number types."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Test with string that can't be converted to int
        await store.set_outgoing_sequence_number("invalid")  # Should handle gracefully
        
        # Should remain unchanged from error case
        assert store.get_next_outgoing_sequence_number() == 1
        
        await store.close()
    
    @pytest.mark.asyncio
    async def test_current_outgoing_sequence_number(self, temp_db_path, mock_config_manager):
        """Test current outgoing sequence number functionality."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Should start at 0 (no messages sent yet)
        assert store.get_current_outgoing_sequence_number() == 0
        
        # After incrementing
        await store.increment_outgoing_sequence_number()
        assert store.get_current_outgoing_sequence_number() == 1
        
        await store.close()


@pytest.mark.unit
class TestDatabaseMessageStoreIntegration:
    """Test integration scenarios and complex workflows."""
    
    @pytest.mark.asyncio
    async def test_complete_message_flow(self, temp_db_path, mock_config_manager, sample_fix_message):
        """Test complete message storage and sequence number flow."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Simulate a complete FIX session workflow
        
        # 1. Send initial messages
        for i in range(1, 4):
            seq_num = store.get_next_outgoing_sequence_number()
            assert seq_num == i
            
            msg = sample_fix_message.copy()
            msg['34'] = str(seq_num)
            await store.store_message('FIX.4.4', 'SENDER', 'TARGET', seq_num, str(msg))
            await store.increment_outgoing_sequence_number()
        
        # 2. Receive messages
        for i in range(1, 3):
            incoming_seq = store.get_next_incoming_sequence_number()
            assert incoming_seq == i
            
            msg = sample_fix_message.copy()
            msg['34'] = str(incoming_seq)
            await store.store_message('FIX.4.4', 'TARGET', 'SENDER', incoming_seq, str(msg))
            await store.increment_incoming_sequence_number()
        
        # 3. Verify stored messages
        assert await store.get_message('FIX.4.4', 'SENDER', 'TARGET', 1) is not None
        assert await store.get_message('FIX.4.4', 'SENDER', 'TARGET', 2) is not None
        assert await store.get_message('FIX.4.4', 'SENDER', 'TARGET', 3) is not None
        
        assert await store.get_message('FIX.4.4', 'TARGET', 'SENDER', 1) is not None
        assert await store.get_message('FIX.4.4', 'TARGET', 'SENDER', 2) is not None
        
        await store.close()
    
    @pytest.mark.asyncio
    async def test_concurrent_access_simulation(self, temp_db_path, mock_config_manager, sample_fix_message):
        """Test behavior under simulated concurrent access."""
        store = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store.initialize()
        
        # Simulate multiple operations with proper increments
        seq_numbers = []
        for _ in range(5):
            seq_num = store.get_next_outgoing_sequence_number()
            seq_numbers.append(seq_num)
            await store.increment_outgoing_sequence_number()
        
        # Should be sequential
        expected = list(range(1, 6))
        assert seq_numbers == expected
        
        await store.close()
    
    @pytest.mark.asyncio
    async def test_session_persistence(self, temp_db_path, mock_config_manager, sample_fix_message):
        """Test session persistence across close/open cycles."""
        # First session
        store1 = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store1.initialize()
        
        # Send some messages
        for i in range(3):
            seq_num = store1.get_next_outgoing_sequence_number()
            await store1.store_message('FIX.4.4', 'SENDER', 'TARGET', seq_num, f"Message {seq_num}")
            await store1.increment_outgoing_sequence_number()
        
        await store1.close()
        
        # Second session - should load persisted state
        store2 = DatabaseMessageStore(temp_db_path, 'FIX.4.4', 'SENDER', 'TARGET')
        await store2.initialize()
        
        # Should resume from where we left off
        next_seq = store2.get_next_outgoing_sequence_number()
        assert next_seq == 4  # Should be 4 since we sent 1,2,3
        
        # Should not be a new session
        assert store2.is_new_session() == False
        
        await store2.close()


# Coverage targeting summary:
"""
DATABASE MESSAGE STORE COVERAGE STRATEGY:

✅ HIGH-IMPACT TARGETS:
- Initialization and database setup (~30 lines)
- Sequence number management (~40 lines) 
- Outgoing message storage/retrieval (~35 lines)
- Incoming message storage/retrieval (~25 lines)
- Reset functionality (~15 lines)
- Error handling and edge cases (~20 lines)
- Integration scenarios (~20 lines)

✅ COVERAGE TECHNIQUES:
- Real database operations with temporary files
- Mock configuration management
- Test both success and error paths
- Async reset functionality testing
- Edge case and error condition testing
- Integration workflow testing

✅ EXPECTED IMPACT:
- Current coverage: 29% (54/185 lines)
- Target coverage: 75%+ (140+ lines)
- Coverage gain: +46% for database_message_store.py
- Overall project impact: +4-6% total coverage

✅ NEXT STEPS:
- Add these tests to test_runner.py
- Measure actual coverage impact  
- Continue with other high-impact targets
"""
