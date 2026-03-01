"""
Test fixtures and utilities for PyFixMsg Plus testing framework.
Provides common fixtures, mock objects, and test utilities.
"""
import asyncio
import tempfile
import sqlite3
import os
import socket
from contextlib import contextmanager
from typing import Dict, Any, Optional, AsyncGenerator
from unittest.mock import Mock, AsyncMock
import pytest
import pytest_asyncio
import aiosqlite
from faker import Faker

from pyfixmsg_plus.fixengine.configmanager import ConfigManager
from pyfixmsg_plus.fixengine.engine import FixEngine
from pyfixmsg_plus.fixengine.message_store_factory import MessageStoreFactory
from pyfixmsg_plus.application import Application

fake = Faker()

# ============================================================================
# Configuration Fixtures
# ============================================================================

@pytest.fixture
def sample_config_dict():
    """Basic configuration dictionary for testing."""
    return {
        'database': {
            'type': 'sqlite3',
            'path': ':memory:'
        },
        'session': {
            'sender_comp_id': 'SENDER',
            'target_comp_id': 'TARGET',
            'heartbeat_interval': 30,
            'logon_timeout': 30
        },
        'network': {
            'host': 'localhost',
            'port': 9999,
            'socket_accept_port': 9998
        },
        'logging': {
            'level': 'INFO',
            'format': 'structured'
        }
    }

@pytest.fixture
def temp_config_file(sample_config_dict):
    """Create a temporary config file for testing."""
    with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
        # Convert dict to INI format
        for section, options in sample_config_dict.items():
            f.write(f'[{section}]\n')
            for key, value in options.items():
                f.write(f'{key} = {value}\n')
            f.write('\n')
        config_path = f.name
    
    yield config_path
    os.unlink(config_path)

@pytest.fixture
def config_manager(temp_config_file):
    """ConfigManager instance with test configuration."""
    return ConfigManager(temp_config_file)

# ============================================================================
# Database Fixtures
# ============================================================================

@pytest.fixture
def temp_sqlite_db():
    """Create a temporary SQLite database for testing."""
    with tempfile.NamedTemporaryFile(suffix='.db', delete=False) as f:
        db_path = f.name
    
    yield db_path
    if os.path.exists(db_path):
        os.unlink(db_path)

@pytest.fixture
def memory_sqlite_db():
    """In-memory SQLite database for fast testing."""
    return ':memory:'

@pytest_asyncio.fixture
async def async_db_connection(temp_sqlite_db):
    """Async database connection for aiosqlite testing."""
    async with aiosqlite.connect(temp_sqlite_db) as db:
        yield db

@pytest.fixture
def sync_db_connection(temp_sqlite_db):
    """Sync database connection for sqlite3 testing."""
    conn = sqlite3.connect(temp_sqlite_db)
    yield conn
    conn.close()

# ============================================================================
# Message Store Fixtures
# ============================================================================

@pytest_asyncio.fixture
async def async_message_store(config_manager, async_db_connection):
    """Async message store for testing."""
    store = MessageStoreFactory.create_message_store(config_manager)
    await store.initialize()
    yield store
    await store.close()

@pytest.fixture
def sync_message_store(config_manager, sync_db_connection):
    """Sync message store for testing."""
    # Temporarily modify config to use sync store
    original_type = config_manager.get('database', 'type')
    config_manager.config.set('database', 'type', 'sqlite3')
    
    store = MessageStoreFactory.create_message_store(config_manager)
    store.initialize()
    yield store
    store.close()
    
    # Restore original config
    config_manager.config.set('database', 'type', original_type)

# ============================================================================
# Network Fixtures
# ============================================================================

@pytest.fixture
def free_port():
    """Find a free port for testing."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(('', 0))
        s.listen(1)
        port = s.getsockname()[1]
    return port

@pytest.fixture
def test_host():
    """Test host for network operations."""
    return 'localhost'

@pytest.fixture
def network_config(test_host, free_port):
    """Network configuration for testing."""
    return {
        'host': test_host,
        'port': free_port,
        'socket_accept_port': free_port + 1
    }

# ============================================================================
# FIX Engine Fixtures
# ============================================================================

@pytest_asyncio.fixture
async def fix_engine(config_manager):
    """FIX Engine instance for testing."""
    app = Mock(spec=Application)
    app.on_create = AsyncMock()
    app.on_logon = AsyncMock()
    app.on_logout = AsyncMock()
    app.to_admin = AsyncMock()
    app.from_admin = AsyncMock()
    app.to_app = AsyncMock()
    app.from_app = AsyncMock()
    
    engine = FixEngine(config_manager, app)
    yield engine
    if engine._running:
        await engine.stop()

@pytest_asyncio.fixture
async def fix_engine_pair(sample_config_dict, free_port):
    """Pair of FIX engines for testing interactions."""
    # Create initiator config
    initiator_config = sample_config_dict.copy()
    initiator_config['session']['sender_comp_id'] = 'INITIATOR'
    initiator_config['session']['target_comp_id'] = 'ACCEPTOR'
    initiator_config['network']['port'] = free_port
    
    # Create acceptor config  
    acceptor_config = sample_config_dict.copy()
    acceptor_config['session']['sender_comp_id'] = 'ACCEPTOR'
    acceptor_config['session']['target_comp_id'] = 'INITIATOR'
    acceptor_config['network']['socket_accept_port'] = free_port
    
    # Create config files
    with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
        for section, options in initiator_config.items():
            f.write(f'[{section}]\n')
            for key, value in options.items():
                f.write(f'{key} = {value}\n')
            f.write('\n')
        initiator_config_path = f.name
    
    with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
        for section, options in acceptor_config.items():
            f.write(f'[{section}]\n')
            for key, value in options.items():
                f.write(f'{key} = {value}\n')
            f.write('\n')
        acceptor_config_path = f.name
    
    initiator_cm = ConfigManager(initiator_config_path)
    acceptor_cm = ConfigManager(acceptor_config_path)
    
    # Create mock applications for the engines
    initiator_app = Mock(spec=Application)
    initiator_app.on_create = AsyncMock()
    initiator_app.on_logon = AsyncMock()
    initiator_app.on_logout = AsyncMock()
    initiator_app.to_admin = AsyncMock()
    initiator_app.from_admin = AsyncMock()
    initiator_app.to_app = AsyncMock()
    initiator_app.from_app = AsyncMock()
    
    acceptor_app = Mock(spec=Application)
    acceptor_app.on_create = AsyncMock()
    acceptor_app.on_logon = AsyncMock()
    acceptor_app.on_logout = AsyncMock()
    acceptor_app.to_admin = AsyncMock()
    acceptor_app.from_admin = AsyncMock()
    acceptor_app.to_app = AsyncMock()
    acceptor_app.from_app = AsyncMock()
    
    initiator = FixEngine(initiator_cm, initiator_app)
    acceptor = FixEngine(acceptor_cm, acceptor_app)
    
    yield initiator, acceptor
    
    # Cleanup
    if initiator._running:
        await initiator.stop()
    if acceptor._running:
        await acceptor.stop()
    
    os.unlink(initiator_config_path)
    os.unlink(acceptor_config_path)

# ============================================================================
# Application Fixtures
# ============================================================================

@pytest.fixture
def mock_application():
    """Mock application for testing."""
    app = Mock(spec=Application)
    app.on_create = AsyncMock()
    app.on_logon = AsyncMock()
    app.on_logout = AsyncMock()
    app.to_admin = AsyncMock()
    app.from_admin = AsyncMock()
    app.to_app = AsyncMock()
    app.from_app = AsyncMock()
    return app

# ============================================================================
# FIX Message Fixtures
# ============================================================================

@pytest.fixture
def sample_logon_message():
    """Sample FIX Logon message."""
    return {
        '8': 'FIX.4.4',      # BeginString
        '35': 'A',           # MsgType (Logon)
        '49': 'SENDER',      # SenderCompID
        '56': 'TARGET',      # TargetCompID
        '34': '1',           # MsgSeqNum
        '52': '20250726-12:00:00.000',  # SendingTime
        '98': '0',           # EncryptMethod
        '108': '30',         # HeartBtInt
    }

@pytest.fixture
def sample_heartbeat_message():
    """Sample FIX Heartbeat message."""
    return {
        '8': 'FIX.4.4',      # BeginString
        '35': '0',           # MsgType (Heartbeat)
        '49': 'SENDER',      # SenderCompID
        '56': 'TARGET',      # TargetCompID
        '34': '2',           # MsgSeqNum
        '52': '20250726-12:00:30.000',  # SendingTime
    }

@pytest.fixture
def sample_new_order_message():
    """Sample FIX New Order Single message."""
    return {
        '8': 'FIX.4.4',      # BeginString
        '35': 'D',           # MsgType (NewOrderSingle)
        '49': 'SENDER',      # SenderCompID
        '56': 'TARGET',      # TargetCompID
        '34': '3',           # MsgSeqNum
        '52': '20250726-12:01:00.000',  # SendingTime
        '11': 'ORDER123',    # ClOrdID
        '21': '1',           # HandlInst
        '38': '100',         # OrderQty
        '40': '2',           # OrdType (Limit)
        '44': '50.25',       # Price
        '54': '1',           # Side (Buy)
        '55': 'MSFT',        # Symbol
        '59': '0',           # TimeInForce (DAY)
    }

# ============================================================================
# Performance Testing Fixtures
# ============================================================================

@pytest.fixture
def performance_metrics():
    """Container for performance metrics."""
    return {
        'start_time': None,
        'end_time': None,
        'message_count': 0,
        'error_count': 0,
        'latencies': []
    }

@pytest.fixture
def load_test_config():
    """Configuration for load testing."""
    return {
        'concurrent_sessions': 10,
        'messages_per_session': 1000,
        'message_rate_per_second': 100,
        'test_duration_seconds': 60
    }

# ============================================================================
# Chaos Testing Fixtures
# ============================================================================

@pytest.fixture
def network_failure_simulator():
    """Simulator for network failures."""
    class NetworkFailureSimulator:
        def __init__(self):
            self.failures_enabled = False
            self.failure_rate = 0.1
            self.failure_duration = 1.0
        
        async def simulate_network_partition(self, duration=5.0):
            """Simulate network partition for specified duration."""
            self.failures_enabled = True
            await asyncio.sleep(duration)
            self.failures_enabled = False
        
        async def simulate_packet_loss(self, loss_rate=0.1, duration=10.0):
            """Simulate packet loss for specified duration."""
            original_rate = self.failure_rate
            self.failure_rate = loss_rate
            self.failures_enabled = True
            await asyncio.sleep(duration)
            self.failures_enabled = False
            self.failure_rate = original_rate
        
        def should_fail_connection(self):
            """Determine if connection should fail."""
            import random
            return self.failures_enabled and random.random() < self.failure_rate
    
    return NetworkFailureSimulator()

# ============================================================================
# Property-Based Testing Fixtures
# ============================================================================

@pytest.fixture
def fix_message_generator():
    """Generator for property-based testing of FIX messages."""
    from hypothesis import strategies as st
    
    # FIX field value strategies
    sender_comp_id = st.text(min_size=1, max_size=16, alphabet=st.characters(whitelist_categories=('Lu', 'Ll', 'Nd')))
    target_comp_id = st.text(min_size=1, max_size=16, alphabet=st.characters(whitelist_categories=('Lu', 'Ll', 'Nd')))
    msg_seq_num = st.integers(min_value=1, max_value=999999)
    symbol = st.text(min_size=1, max_size=6, alphabet=st.characters(whitelist_categories=('Lu', 'Nd')))
    price = st.decimals(min_value=0.01, max_value=9999.99, places=2)
    quantity = st.integers(min_value=1, max_value=1000000)
    
    return {
        'sender_comp_id': sender_comp_id,
        'target_comp_id': target_comp_id,
        'msg_seq_num': msg_seq_num,
        'symbol': symbol,
        'price': price,
        'quantity': quantity
    }

# ============================================================================
# Test Data Factories
# ============================================================================

class FixMessageFactory:
    """Factory for creating FIX messages with realistic test data."""
    
    @staticmethod
    def create_logon(sender_comp_id=None, target_comp_id=None, seq_num=1):
        """Create a Logon message."""
        return {
            '8': 'FIX.4.4',
            '35': 'A',
            '49': sender_comp_id or fake.company()[:8].upper(),
            '56': target_comp_id or fake.company()[:8].upper(),
            '34': str(seq_num),
            '52': fake.date_time().strftime('%Y%m%d-%H:%M:%S.%f')[:-3],
            '98': '0',
            '108': '30',
        }
    
    @staticmethod
    def create_heartbeat(sender_comp_id=None, target_comp_id=None, seq_num=2):
        """Create a Heartbeat message."""
        return {
            '8': 'FIX.4.4',
            '35': '0',
            '49': sender_comp_id or fake.company()[:8].upper(),
            '56': target_comp_id or fake.company()[:8].upper(),
            '34': str(seq_num),
            '52': fake.date_time().strftime('%Y%m%d-%H:%M:%S.%f')[:-3],
        }
    
    @staticmethod
    def create_new_order(sender_comp_id=None, target_comp_id=None, seq_num=3):
        """Create a New Order Single message."""
        return {
            '8': 'FIX.4.4',
            '35': 'D',
            '49': sender_comp_id or fake.company()[:8].upper(),
            '56': target_comp_id or fake.company()[:8].upper(),
            '34': str(seq_num),
            '52': fake.date_time().strftime('%Y%m%d-%H:%M:%S.%f')[:-3],
            '11': fake.uuid4()[:12].upper(),
            '21': '1',
            '38': str(fake.random_int(min=1, max=10000)),
            '40': '2',
            '44': str(fake.pydecimal(left_digits=4, right_digits=2, positive=True)),
            '54': str(fake.random_int(min=1, max=2)),
            '55': fake.random_element(['MSFT', 'AAPL', 'GOOGL', 'AMZN', 'TSLA']),
            '59': '0',
        }

@pytest.fixture
def fix_message_factory():
    """FIX message factory for test data generation."""
    return FixMessageFactory

# ============================================================================
# Async Testing Utilities
# ============================================================================

@pytest.fixture
def event_loop():
    """Create an event loop for async testing."""
    loop = asyncio.new_event_loop()
    yield loop
    loop.close()

@contextmanager
def timeout_context(seconds=30):
    """Context manager for test timeouts."""
    import signal
    
    def timeout_handler(signum, frame):
        raise TimeoutError(f"Test timed out after {seconds} seconds")
    
    # Set the signal handler and alarm
    old_handler = signal.signal(signal.SIGALRM, timeout_handler)
    signal.alarm(seconds)
    
    try:
        yield
    finally:
        # Reset the alarm and handler
        signal.alarm(0)
        signal.signal(signal.SIGALRM, old_handler)

# ============================================================================
# Cleanup Utilities
# ============================================================================

@pytest.fixture(autouse=True)
def cleanup_temp_files():
    """Automatically cleanup temporary files after each test."""
    temp_files = []
    
    def register_temp_file(filepath):
        temp_files.append(filepath)
    
    yield register_temp_file
    
    # Cleanup
    for filepath in temp_files:
        if os.path.exists(filepath):
            try:
                os.unlink(filepath)
            except OSError:
                pass  # File might be locked or already deleted
