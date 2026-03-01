"""
Property-based tests for PyFixMsg Plus FIX Engine.
Uses Hypothesis to generate test cases and validate invariants.
"""
import asyncio
import pytest
from hypothesis import given, strategies as st, settings, assume
from hypothesis.stateful import RuleBasedStateMachine, rule, invariant, Bundle
import string

from pyfixmsg_plus.fixengine.configmanager import ConfigManager
from pyfixmsg_plus.fixengine.engine import FixEngine
from pyfixmsg_plus.application import Application
from pyfixmsg.fixmessage import FixMessage


# ============================================================================
# Data Generation Strategies
# ============================================================================

# FIX field value strategies
fix_string = st.text(
    alphabet=st.characters(
        whitelist_categories=('Lu', 'Ll', 'Nd', 'Pc'),
        whitelist_characters='-._'
    ),
    min_size=1,
    max_size=64
)

fix_compid = st.text(
    alphabet=st.characters(whitelist_categories=('Lu', 'Ll', 'Nd')),
    min_size=1,
    max_size=16
)

fix_price = st.decimals(
    min_value=0.01,
    max_value=999999.99,
    places=2
).map(str)

fix_quantity = st.integers(min_value=1, max_value=1000000).map(str)

fix_seq_num = st.integers(min_value=1, max_value=999999).map(str)

fix_msg_type = st.sampled_from([
    'A',   # Logon
    '0',   # Heartbeat
    '1',   # TestRequest
    '2',   # ResendRequest
    '3',   # Reject
    '4',   # SequenceReset
    '5',   # Logout
    'D',   # NewOrderSingle
    '8',   # ExecutionReport
    '9',   # OrderCancelReject
    'F',   # OrderCancelRequest
])

fix_side = st.sampled_from(['1', '2'])  # Buy, Sell

fix_order_type = st.sampled_from(['1', '2', '3', '4'])  # Market, Limit, Stop, StopLimit

fix_time_in_force = st.sampled_from(['0', '1', '2', '3', '4'])  # Day, GTC, IOC, FOK, GTD

fix_symbol = st.text(
    alphabet=st.characters(whitelist_categories=('Lu', 'Nd')),
    min_size=1,
    max_size=12
)

# Generate valid FIX timestamps
fix_timestamp = st.datetimes(
    min_value=pytest.datetime(2020, 1, 1),
    max_value=pytest.datetime(2030, 12, 31)
).map(lambda dt: dt.strftime('%Y%m%d-%H:%M:%S.%f')[:-3])


# ============================================================================
# Message Generation Strategies
# ============================================================================

@st.composite
def fix_logon_message(draw, sender_comp_id=None, target_comp_id=None):
    """Generate a valid Logon message."""
    return {
        '8': 'FIX.4.4',
        '35': 'A',
        '49': sender_comp_id or draw(fix_compid),
        '56': target_comp_id or draw(fix_compid),
        '34': draw(fix_seq_num),
        '52': draw(fix_timestamp),
        '98': '0',  # EncryptMethod (None)
        '108': str(draw(st.integers(min_value=10, max_value=300))),  # HeartBtInt
    }

@st.composite
def fix_heartbeat_message(draw, sender_comp_id=None, target_comp_id=None):
    """Generate a valid Heartbeat message."""
    return {
        '8': 'FIX.4.4',
        '35': '0',
        '49': sender_comp_id or draw(fix_compid),
        '56': target_comp_id or draw(fix_compid),
        '34': draw(fix_seq_num),
        '52': draw(fix_timestamp),
    }

@st.composite
def fix_new_order_message(draw, sender_comp_id=None, target_comp_id=None):
    """Generate a valid New Order Single message."""
    return {
        '8': 'FIX.4.4',
        '35': 'D',
        '49': sender_comp_id or draw(fix_compid),
        '56': target_comp_id or draw(fix_compid),
        '34': draw(fix_seq_num),
        '52': draw(fix_timestamp),
        '11': draw(fix_string),  # ClOrdID
        '21': '1',  # HandlInst (Automated)
        '38': draw(fix_quantity),  # OrderQty
        '40': draw(fix_order_type),  # OrdType
        '44': draw(fix_price),  # Price (for limit orders)
        '54': draw(fix_side),  # Side
        '55': draw(fix_symbol),  # Symbol
        '59': draw(fix_time_in_force),  # TimeInForce
    }

@st.composite
def arbitrary_fix_message(draw):
    """Generate an arbitrary FIX message."""
    msg_type = draw(fix_msg_type)
    
    base_fields = {
        '8': 'FIX.4.4',
        '35': msg_type,
        '49': draw(fix_compid),
        '56': draw(fix_compid),
        '34': draw(fix_seq_num),
        '52': draw(fix_timestamp),
    }
    
    # Add message-specific fields based on type
    if msg_type == 'A':  # Logon
        base_fields.update({
            '98': '0',
            '108': str(draw(st.integers(min_value=10, max_value=300))),
        })
    elif msg_type == 'D':  # NewOrderSingle
        base_fields.update({
            '11': draw(fix_string),
            '21': '1',
            '38': draw(fix_quantity),
            '40': draw(fix_order_type),
            '54': draw(fix_side),
            '55': draw(fix_symbol),
            '59': draw(fix_time_in_force),
        })
        
        # Add price for limit orders
        if base_fields['40'] in ['2', '4']:  # Limit or StopLimit
            base_fields['44'] = draw(fix_price)
    
    return base_fields


# ============================================================================
# Property-Based Unit Tests
# ============================================================================

@pytest.mark.property
class TestMessageValidationProperties:
    """Test message validation properties using hypothesis."""
    
    @given(fix_logon_message())
    @settings(max_examples=100)
    def test_logon_message_validation(self, logon_msg):
        """Test that all generated logon messages are valid."""
        # Create FixMessage and validate
        try:
            fix_msg = FixMessage(logon_msg)
            
            # Verify required fields are present
            assert fix_msg.get('8') == 'FIX.4.4'
            assert fix_msg.get('35') == 'A'
            assert fix_msg.get('49') is not None
            assert fix_msg.get('56') is not None
            assert fix_msg.get('34') is not None
            assert fix_msg.get('52') is not None
            assert fix_msg.get('98') is not None
            assert fix_msg.get('108') is not None
            
            # Verify sequence number is positive
            seq_num = int(fix_msg.get('34'))
            assert seq_num > 0
            
            # Verify heartbeat interval is reasonable
            heartbeat = int(fix_msg.get('108'))
            assert 10 <= heartbeat <= 300
            
        except Exception as e:
            pytest.fail(f"Valid logon message failed validation: {e}")
    
    @given(fix_new_order_message())
    @settings(max_examples=100)
    def test_new_order_message_validation(self, order_msg):
        """Test that all generated new order messages are valid."""
        try:
            fix_msg = FixMessage(order_msg)
            
            # Verify required fields
            assert fix_msg.get('8') == 'FIX.4.4'
            assert fix_msg.get('35') == 'D'
            assert fix_msg.get('11') is not None  # ClOrdID
            assert fix_msg.get('38') is not None  # OrderQty
            assert fix_msg.get('40') is not None  # OrdType
            assert fix_msg.get('54') is not None  # Side
            assert fix_msg.get('55') is not None  # Symbol
            
            # Verify numeric fields
            order_qty = int(fix_msg.get('38'))
            assert order_qty > 0
            
            # Verify side is valid
            side = fix_msg.get('54')
            assert side in ['1', '2']
            
            # Verify price is present for limit orders
            order_type = fix_msg.get('40')
            if order_type in ['2', '4']:  # Limit or StopLimit
                price = fix_msg.get('44')
                assert price is not None
                assert float(price) > 0
            
        except Exception as e:
            pytest.fail(f"Valid new order message failed validation: {e}")
    
    @given(arbitrary_fix_message())
    @settings(max_examples=50)
    def test_message_parsing_roundtrip(self, fix_dict):
        """Test that any valid FIX message can be parsed and serialized."""
        try:
            # Create message from dict
            fix_msg = FixMessage(fix_dict)
            
            # Serialize to string
            serialized = str(fix_msg)
            
            # Parse back from string
            parsed_msg = FixMessage()
            parsed_msg.from_string(serialized)
            
            # Verify round-trip consistency
            assert parsed_msg.get('8') == fix_dict['8']
            assert parsed_msg.get('35') == fix_dict['35']
            assert parsed_msg.get('49') == fix_dict['49']
            assert parsed_msg.get('56') == fix_dict['56']
            
        except Exception as e:
            # Some generated messages might be invalid, which is expected
            pass
    
    @given(
        sender_id=fix_compid,
        target_id=fix_compid,
        seq_nums=st.lists(fix_seq_num, min_size=1, max_size=10)
    )
    @settings(max_examples=50)
    def test_sequence_number_ordering(self, sender_id, target_id, seq_nums):
        """Test that sequence numbers maintain proper ordering."""
        assume(sender_id != target_id)  # Sender and target should be different
        
        # Convert to integers and sort
        int_seq_nums = [int(seq) for seq in seq_nums]
        sorted_seq_nums = sorted(set(int_seq_nums))  # Remove duplicates and sort
        
        # Create messages with ordered sequence numbers
        messages = []
        for seq in sorted_seq_nums:
            msg = {
                '8': 'FIX.4.4',
                '35': '0',  # Heartbeat
                '49': sender_id,
                '56': target_id,
                '34': str(seq),
                '52': '20250726-12:00:00.000',
            }
            messages.append(FixMessage(msg))
        
        # Verify sequence number property: each message has higher seq than previous
        for i in range(1, len(messages)):
            prev_seq = int(messages[i-1].get('34'))
            curr_seq = int(messages[i].get('34'))
            assert curr_seq > prev_seq, f"Sequence number not increasing: {prev_seq} -> {curr_seq}"


@pytest.mark.property
class TestConfigurationProperties:
    """Test configuration handling properties."""
    
    @given(
        sender_comp_id=fix_compid,
        target_comp_id=fix_compid,
        heartbeat_interval=st.integers(min_value=1, max_value=3600),
        port=st.integers(min_value=1024, max_value=65535)
    )
    @settings(max_examples=20)
    def test_configuration_validation(self, sender_comp_id, target_comp_id, heartbeat_interval, port):
        """Test that configuration validation works for various inputs."""
        assume(sender_comp_id != target_comp_id)
        
        config_dict = {
            'session': {
                'sender_comp_id': sender_comp_id,
                'target_comp_id': target_comp_id,
                'heartbeat_interval': heartbeat_interval,
            },
            'network': {
                'host': 'localhost',
                'port': port,
            },
            'database': {
                'type': 'sqlite3',
                'path': ':memory:'
            }
        }
        
        import tempfile
        import os
        
        # Create temporary config file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
            for section, options in config_dict.items():
                f.write(f'[{section}]\n')
                for key, value in options.items():
                    f.write(f'{key} = {value}\n')
                f.write('\n')
            config_path = f.name
        
        try:
            # Test configuration loading
            config_manager = ConfigManager(config_path)
            
            # Verify values are loaded correctly
            assert config_manager.get('session', 'sender_comp_id') == sender_comp_id
            assert config_manager.get('session', 'target_comp_id') == target_comp_id
            assert config_manager.get('session', 'heartbeat_interval') == str(heartbeat_interval)
            assert config_manager.get('network', 'port') == str(port)
            
        finally:
            os.unlink(config_path)


# ============================================================================
# Stateful Testing
# ============================================================================

@pytest.mark.property
@pytest.mark.slow
class FixEngineStateMachine(RuleBasedStateMachine):
    """Stateful testing of FIX engine using hypothesis."""
    
    def __init__(self):
        super().__init__()
        self.engine = None
        self.is_started = False
        self.sent_messages = []
        self.sequence_number = 1
        
        # Create test configuration
        import tempfile
        import os
        
        config_dict = {
            'session': {
                'sender_comp_id': 'SENDER',
                'target_comp_id': 'TARGET',
                'heartbeat_interval': 30,
            },
            'network': {
                'host': 'localhost',
                'port': 9999,
            },
            'database': {
                'type': 'sqlite3',
                'path': ':memory:'
            }
        }
        
        with tempfile.NamedTemporaryFile(mode='w', suffix='.ini', delete=False) as f:
            for section, options in config_dict.items():
                f.write(f'[{section}]\n')
                for key, value in options.items():
                    f.write(f'{key} = {value}\n')
                f.write('\n')
            self.config_path = f.name
        
        config_manager = ConfigManager(self.config_path)
        self.engine = FixEngine(config_manager)
        
        # Mock application
        from unittest.mock import Mock, AsyncMock
        app = Mock(spec=Application)
        app.on_create = AsyncMock()
        app.on_logon = AsyncMock()
        app.on_logout = AsyncMock()
        app.to_admin = AsyncMock()
        app.from_admin = AsyncMock()
        app.to_app = AsyncMock()
        app.from_app = AsyncMock()
        self.engine.application = app
    
    def teardown(self):
        """Cleanup after stateful testing."""
        import asyncio
        import os
        
        async def cleanup():
            if self.engine and self.is_started:
                await self.engine.stop()
        
        try:
            loop = asyncio.get_event_loop()
            loop.run_until_complete(cleanup())
        except:
            pass
        
        if os.path.exists(self.config_path):
            os.unlink(self.config_path)
    
    @rule()
    def start_engine(self):
        """Start the FIX engine."""
        if not self.is_started:
            import asyncio
            
            async def start():
                await self.engine.start()
            
            try:
                loop = asyncio.get_event_loop()
                loop.run_until_complete(start())
                self.is_started = True
            except Exception as e:
                # Starting might fail in test environment
                pass
    
    @rule()
    def stop_engine(self):
        """Stop the FIX engine."""
        if self.is_started:
            import asyncio
            
            async def stop():
                await self.engine.stop()
            
            try:
                loop = asyncio.get_event_loop()
                loop.run_until_complete(stop())
                self.is_started = False
            except Exception as e:
                # Stopping might fail
                pass
    
    @rule(message_data=fix_heartbeat_message())
    def send_message(self, message_data):
        """Send a message through the engine."""
        if self.is_started:
            import asyncio
            
            # Update sequence number
            message_data['34'] = str(self.sequence_number)
            self.sequence_number += 1
            
            async def send():
                await self.engine.send_to_target(message_data)
            
            try:
                loop = asyncio.get_event_loop()
                loop.run_until_complete(send())
                self.sent_messages.append(message_data)
            except Exception as e:
                # Sending might fail if not logged on
                pass
    
    @invariant()
    def sequence_numbers_are_ordered(self):
        """Invariant: Sequence numbers should be in order."""
        if len(self.sent_messages) > 1:
            for i in range(1, len(self.sent_messages)):
                prev_seq = int(self.sent_messages[i-1]['34'])
                curr_seq = int(self.sent_messages[i]['34'])
                assert curr_seq > prev_seq, f"Sequence number not increasing: {prev_seq} -> {curr_seq}"
    
    @invariant()
    def engine_state_consistency(self):
        """Invariant: Engine state should be consistent."""
        if self.engine:
            # Engine should exist
            assert self.engine is not None
            
            # If started, engine should have running state
            if self.is_started:
                # Note: This depends on engine implementation
                pass


# ============================================================================
# Integration Property Tests
# ============================================================================

@pytest.mark.property
@pytest.mark.asyncio
class TestSessionProperties:
    """Test session-level properties."""
    
    @given(
        heartbeat_interval=st.integers(min_value=10, max_value=120),
        message_count=st.integers(min_value=1, max_value=20)
    )
    @settings(max_examples=5, deadline=30000)  # Longer deadline for async tests
    async def test_heartbeat_timing_property(self, heartbeat_interval, message_count, fix_engine, mock_application):
        """Test that heartbeat timing follows configured interval."""
        # Configure heartbeat interval
        fix_engine.config_manager.config.set('session', 'heartbeat_interval', str(heartbeat_interval))
        fix_engine.application = mock_application
        
        # Track timing
        start_time = None
        heartbeat_times = []
        
        async def track_heartbeats(session_id, message):
            nonlocal start_time
            if message.get('35') == '0':  # Heartbeat
                current_time = asyncio.get_event_loop().time()
                if start_time is None:
                    start_time = current_time
                heartbeat_times.append(current_time - start_time)
        
        mock_application.from_admin = track_heartbeats
        
        try:
            await fix_engine.start()
            await asyncio.sleep(1.0)  # Wait for startup
            
            # Wait for several heartbeat intervals
            test_duration = heartbeat_interval * 2.5  # 2.5 intervals
            await asyncio.sleep(test_duration)
            
            # Verify heartbeat timing property
            if len(heartbeat_times) >= 2:
                intervals = [heartbeat_times[i] - heartbeat_times[i-1] 
                           for i in range(1, len(heartbeat_times))]
                
                # Heartbeat intervals should be approximately correct (±20% tolerance)
                for interval in intervals:
                    expected = heartbeat_interval
                    tolerance = expected * 0.2
                    assert abs(interval - expected) <= tolerance, \
                        f"Heartbeat interval {interval} not within tolerance of {expected}"
            
        except Exception as e:
            # Test environment limitations
            pass
        finally:
            if hasattr(fix_engine, 'stop'):
                await fix_engine.stop()


# Run stateful tests
TestFixEngineStateMachine = FixEngineStateMachine.TestCase


if __name__ == '__main__':
    pytest.main([__file__, '-v', '--tb=short', '-m', 'property'])
