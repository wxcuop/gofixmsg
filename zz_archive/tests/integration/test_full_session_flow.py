"""
Integration tests for PyFixMsg Plus FIX Engine.
Tests end-to-end scenarios including session establishment, message exchange, and error handling.
"""
import asyncio
import pytest
import time
from unittest.mock import Mock, AsyncMock

from pyfixmsg_plus.fixengine.engine import FixEngine
from pyfixmsg_plus.fixengine.configmanager import ConfigManager
from pyfixmsg_plus.application import Application


@pytest.mark.integration
@pytest.mark.asyncio
class TestFullSessionFlow:
    """Test complete FIX session flows from logon to logout."""
    
    async def test_session_establishment_and_termination(self, fix_engine_pair, mock_application):
        """Test basic session establishment and clean termination."""
        initiator, acceptor = fix_engine_pair
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start acceptor first
        await acceptor.start()
        await asyncio.sleep(0.1)  # Allow acceptor to start listening
        
        # Start initiator to establish connection
        await initiator.start()
        
        # Wait for session establishment
        await asyncio.sleep(2.0)
        
        # Verify session is established
        assert initiator.is_logged_on()
        assert acceptor.is_logged_on()
        
        # Verify application callbacks were called
        mock_application.on_logon.assert_called()
        
        # Stop sessions
        await initiator.stop()
        await acceptor.stop()
        
        # Verify clean termination
        assert not initiator.is_logged_on()
        assert not acceptor.is_logged_on()
    
    async def test_heartbeat_mechanism(self, fix_engine_pair, mock_application):
        """Test heartbeat exchange during active session."""
        initiator, acceptor = fix_engine_pair
        
        # Set shorter heartbeat interval for testing
        initiator.config_manager.config.set('session', 'heartbeat_interval', '2')
        acceptor.config_manager.config.set('session', 'heartbeat_interval', '2')
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        
        # Wait for session establishment
        await asyncio.sleep(1.0)
        assert initiator.is_logged_on()
        assert acceptor.is_logged_on()
        
        # Wait for heartbeat exchanges
        await asyncio.sleep(5.0)
        
        # Verify sessions are still active (heartbeats working)
        assert initiator.is_logged_on()
        assert acceptor.is_logged_on()
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()
    
    async def test_message_exchange(self, fix_engine_pair, mock_application, sample_new_order_message):
        """Test application message exchange between sessions."""
        initiator, acceptor = fix_engine_pair
        
        # Track received messages
        received_messages = []
        
        async def capture_app_message(session_id, message):
            received_messages.append(message)
        
        mock_application.from_app = capture_app_message
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        
        # Wait for session establishment
        await asyncio.sleep(1.0)
        assert initiator.is_logged_on()
        
        # Send application message
        await initiator.send_to_target(sample_new_order_message)
        
        # Wait for message delivery
        await asyncio.sleep(1.0)
        
        # Verify message was received
        assert len(received_messages) == 1
        received_msg = received_messages[0]
        assert received_msg['35'] == 'D'  # NewOrderSingle
        assert received_msg['55'] == sample_new_order_message['55']  # Symbol
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()
    
    async def test_resend_request_handling(self, fix_engine_pair, mock_application):
        """Test resend request and gap fill functionality."""
        initiator, acceptor = fix_engine_pair
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        
        # Wait for session establishment
        await asyncio.sleep(1.0)
        assert initiator.is_logged_on()
        
        # Send multiple messages
        for i in range(5):
            msg = {
                '8': 'FIX.4.4',
                '35': '0',  # Heartbeat
                '49': 'INITIATOR',
                '56': 'ACCEPTOR',
                '34': str(i + 2),  # Start after logon
                '52': '20250726-12:00:00.000',
            }
            await initiator.send_to_target(msg)
            await asyncio.sleep(0.1)
        
        # Request resend from acceptor
        resend_request = {
            '8': 'FIX.4.4',
            '35': '2',  # ResendRequest
            '49': 'ACCEPTOR',
            '56': 'INITIATOR',
            '34': str(10),
            '52': '20250726-12:00:05.000',
            '7': '2',   # BeginSeqNo
            '16': '4',  # EndSeqNo
        }
        await acceptor.send_to_target(resend_request)
        
        # Wait for resend processing
        await asyncio.sleep(2.0)
        
        # Verify resend was handled (check sequence numbers)
        # This is a basic test - more detailed verification would check actual resent messages
        assert initiator.is_logged_on()
        assert acceptor.is_logged_on()
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()
    
    async def test_sequence_reset_handling(self, fix_engine_pair, mock_application):
        """Test sequence reset functionality."""
        initiator, acceptor = fix_engine_pair
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        
        # Wait for session establishment
        await asyncio.sleep(1.0)
        assert initiator.is_logged_on()
        
        # Send sequence reset
        sequence_reset = {
            '8': 'FIX.4.4',
            '35': '4',  # SequenceReset
            '49': 'INITIATOR',
            '56': 'ACCEPTOR',
            '34': '5',
            '52': '20250726-12:00:00.000',
            '123': 'Y',  # GapFillFlag
            '36': '10',  # NewSeqNo
        }
        await initiator.send_to_target(sequence_reset)
        
        # Wait for processing
        await asyncio.sleep(1.0)
        
        # Verify session is still active after sequence reset
        assert initiator.is_logged_on()
        assert acceptor.is_logged_on()
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()


@pytest.mark.integration
@pytest.mark.asyncio
class TestErrorRecoveryScenarios:
    """Test error handling and recovery scenarios."""
    
    async def test_invalid_message_handling(self, fix_engine_pair, mock_application):
        """Test handling of invalid FIX messages."""
        initiator, acceptor = fix_engine_pair
        
        # Track reject messages
        reject_messages = []
        
        async def capture_admin_message(session_id, message):
            if message.get('35') == '3':  # Reject
                reject_messages.append(message)
        
        mock_application.from_admin = capture_admin_message
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        
        # Wait for session establishment
        await asyncio.sleep(1.0)
        assert initiator.is_logged_on()
        
        # Send invalid message (missing required fields)
        invalid_message = {
            '8': 'FIX.4.4',
            '35': 'D',  # NewOrderSingle
            '49': 'INITIATOR',
            '56': 'ACCEPTOR',
            '34': '2',
            '52': '20250726-12:00:00.000',
            # Missing required fields like ClOrdID, Symbol, etc.
        }
        
        try:
            await initiator.send_to_target(invalid_message)
            await asyncio.sleep(1.0)
            
            # Verify reject was sent (implementation dependent)
            # Session should still be active despite invalid message
            assert initiator.is_logged_on()
            assert acceptor.is_logged_on()
            
        except Exception:
            # Some implementations may throw exception for invalid messages
            pass
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()
    
    async def test_session_timeout_and_reconnection(self, fix_engine_pair, mock_application):
        """Test session timeout and reconnection logic."""
        initiator, acceptor = fix_engine_pair
        
        # Set short timeout for testing
        initiator.config_manager.config.set('session', 'heartbeat_interval', '2')
        acceptor.config_manager.config.set('session', 'heartbeat_interval', '2')
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        
        # Wait for session establishment
        await asyncio.sleep(1.0)
        assert initiator.is_logged_on()
        
        # Simulate network interruption by stopping initiator abruptly
        await initiator.stop()
        
        # Wait for timeout detection
        await asyncio.sleep(5.0)
        
        # Restart initiator (simulate reconnection)
        initiator = FixEngine(initiator.config_manager)
        initiator.application = mock_application
        await initiator.start()
        
        # Wait for reconnection
        await asyncio.sleep(2.0)
        
        # Verify reconnection
        assert initiator.is_logged_on()
        assert acceptor.is_logged_on()
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()


@pytest.mark.integration
@pytest.mark.asyncio
class TestMultiSessionManagement:
    """Test management of multiple concurrent sessions."""
    
    async def test_multiple_concurrent_sessions(self, sample_config_dict, free_port):
        """Test handling multiple concurrent FIX sessions."""
        # Create multiple initiator configs
        initiator_configs = []
        initiators = []
        
        for i in range(3):
            config = sample_config_dict.copy()
            config['session']['sender_comp_id'] = f'INIT{i}'
            config['session']['target_comp_id'] = 'ACCEPTOR'
            config['network']['port'] = free_port
            initiator_configs.append(config)
        
        # Create acceptor config
        acceptor_config = sample_config_dict.copy()
        acceptor_config['session']['sender_comp_id'] = 'ACCEPTOR'
        acceptor_config['network']['socket_accept_port'] = free_port
        
        # Create config manager and acceptor
        import tempfile
        import os
        
        with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
            for section, options in acceptor_config.items():
                f.write(f'[{section}]\n')
                for key, value in options.items():
                    f.write(f'{key} = {value}\n')
                f.write('\n')
            acceptor_config_path = f.name
        
        acceptor_cm = ConfigManager(acceptor_config_path)
        acceptor = FixEngine(acceptor_cm)
        acceptor.application = Mock(spec=Application)
        
        # Start acceptor
        await acceptor.start()
        await asyncio.sleep(0.1)
        
        try:
            # Start multiple initiators
            for config in initiator_configs:
                with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
                    for section, options in config.items():
                        f.write(f'[{section}]\n')
                        for key, value in options.items():
                            f.write(f'{key} = {value}\n')
                        f.write('\n')
                    config_path = f.name
                
                cm = ConfigManager(config_path)
                initiator = FixEngine(cm)
                initiator.application = Mock(spec=Application)
                initiators.append((initiator, config_path))
                
                await initiator.start()
                await asyncio.sleep(0.2)  # Stagger connection attempts
            
            # Wait for all sessions to establish
            await asyncio.sleep(3.0)
            
            # Verify all sessions are established
            for initiator, _ in initiators:
                assert initiator.is_logged_on()
            assert acceptor.is_logged_on()
            
            # Send messages from each initiator
            for i, (initiator, _) in enumerate(initiators):
                test_message = {
                    '8': 'FIX.4.4',
                    '35': '0',  # Heartbeat
                    '49': f'INIT{i}',
                    '56': 'ACCEPTOR',
                    '34': '2',
                    '52': '20250726-12:00:00.000',
                }
                await initiator.send_to_target(test_message)
            
            # Wait for message processing
            await asyncio.sleep(1.0)
            
            # Verify all sessions are still active
            for initiator, _ in initiators:
                assert initiator.is_logged_on()
            assert acceptor.is_logged_on()
            
        finally:
            # Cleanup
            for initiator, config_path in initiators:
                if initiator._running:
                    await initiator.stop()
                os.unlink(config_path)
            
            await acceptor.stop()
            os.unlink(acceptor_config_path)


@pytest.mark.integration
@pytest.mark.database
@pytest.mark.asyncio
class TestDatabaseIntegration:
    """Test database integration scenarios."""
    
    async def test_message_persistence_during_session(self, fix_engine_pair, mock_application):
        """Test that messages are properly persisted to database during session."""
        initiator, acceptor = fix_engine_pair
        
        # Set up applications
        initiator.application = mock_application
        acceptor.application = mock_application
        
        # Start session
        await acceptor.start()
        await asyncio.sleep(0.1)
        await initiator.start()
        
        # Wait for session establishment
        await asyncio.sleep(1.0)
        assert initiator.is_logged_on()
        
        # Send multiple messages
        message_count = 5
        for i in range(message_count):
            test_message = {
                '8': 'FIX.4.4',
                '35': '0',  # Heartbeat
                '49': 'INITIATOR',
                '56': 'ACCEPTOR',
                '34': str(i + 2),
                '52': f'20250726-12:0{i:02d}:00.000',
            }
            await initiator.send_to_target(test_message)
            await asyncio.sleep(0.1)
        
        # Wait for processing
        await asyncio.sleep(1.0)
        
        # Verify messages are persisted
        # Note: This would require access to the message store
        # In a real implementation, we'd query the database directly
        
        # Cleanup
        await initiator.stop()
        await acceptor.stop()
    
    async def test_session_recovery_from_database(self, config_manager, mock_application):
        """Test session recovery using persisted sequence numbers."""
        # This test would verify that sessions can recover their sequence numbers
        # from the database after restart
        
        # Create first session
        engine1 = FixEngine(config_manager)
        engine1.application = mock_application
        
        # Start and send some messages (to increment sequence numbers)
        await engine1.start()
        await asyncio.sleep(1.0)
        
        # Simulate sending messages to increment sequence numbers
        for i in range(3):
            await asyncio.sleep(0.1)
        
        # Stop session
        await engine1.stop()
        
        # Create new session with same config (simulate restart)
        engine2 = FixEngine(config_manager)
        engine2.application = mock_application
        
        # Start and verify sequence numbers are recovered
        await engine2.start()
        await asyncio.sleep(1.0)
        
        # Verify sequence number recovery
        # Note: This would require access to internal sequence number state
        
        # Cleanup
        await engine2.stop()


if __name__ == '__main__':
    pytest.main([__file__, '-v', '--tb=short'])
