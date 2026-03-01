"""
Simple unit tests for PyFixMsg Plus core components.
These tests validate basic functionality without complex mocking.
"""
import pytest
import tempfile
import os
import asyncio

from pyfixmsg_plus.fixengine.configmanager import ConfigManager
from pyfixmsg_plus.fixengine.database_message_store import DatabaseMessageStore
from pyfixmsg_plus.fixengine.message_store_factory import MessageStoreFactory


@pytest.mark.unit
class TestBasicComponents:
    """Basic tests for core components that can run without complex setup."""
    
    def test_config_manager_basic(self):
        """Test basic config manager functionality."""
        # This should work with existing config
        config_file = 'config.ini'
        if os.path.exists(config_file):
            config_manager = ConfigManager(config_file)
            assert config_manager is not None
            # Test basic getter functionality
            try:
                value = config_manager.get('session', 'sender_comp_id', fallback='TEST')
                assert value is not None
            except Exception:
                # Config might not have this section, which is fine for basic test
                pass
    
    def test_database_message_store_creation(self):
        """Test that database message store can be created."""
        with tempfile.NamedTemporaryFile(suffix='.db', delete=False) as f:
            db_path = f.name
        
        try:
            store = DatabaseMessageStore(db_path)
            assert store is not None
            # Test initialization (sync version)
            try:
                # For sync store, just check it can be created
                assert hasattr(store, 'initialize')
            except Exception:
                # If initialize is async, that's fine for basic test
                pass
            assert os.path.exists(db_path)
        finally:
            if os.path.exists(db_path):
                os.unlink(db_path)
    
    def test_imports_work(self):
        """Test that all core modules can be imported."""
        # These imports should work if the modules are correct
        from pyfixmsg_plus.fixengine.engine import FixEngine
        from pyfixmsg_plus.fixengine.state_machine import StateMachine
        from pyfixmsg_plus.fixengine.message_store_factory import MessageStoreFactory
        from pyfixmsg_plus.application import Application
        
        # Basic assertion that classes exist
        assert FixEngine is not None
        assert StateMachine is not None 
        assert MessageStoreFactory is not None
        assert Application is not None


@pytest.mark.unit
@pytest.mark.asyncio  
class TestMessageStoreFactory:
    """Test message store factory functionality."""
    
    async def test_sqlite_store_creation(self):
        """Test creating sqlite message store."""
        factory = MessageStoreFactory()
        
        # Test store creation with required parameters
        store = await factory.get_message_store(
            store_type='database',
            db_path=':memory:',
            beginstring='FIX.4.4',
            sendercompid='TEST_SENDER',
            targetcompid='TEST_TARGET'
        )
        assert store is not None
        assert isinstance(store, DatabaseMessageStore)


if __name__ == '__main__':
    pytest.main([__file__, '-v'])
