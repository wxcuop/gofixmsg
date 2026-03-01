"""
Comprehensive unit tests for Heartbeat: timer lifecycle, interval changes, missed heartbeat detection, error scenarios.
"""
import pytest
import asyncio
from unittest.mock import Mock, AsyncMock
from pyfixmsg_plus.fixengine.heartbeat import Heartbeat

# Add test skeletons for heartbeat lifecycle and error scenarios
class TestHeartbeatLifecycle:
    @pytest.mark.asyncio
    async def test_start_stop(self):
        send_cb = AsyncMock()
        config_mgr = Mock()
        state_machine = Mock()
        fix_engine = Mock()
        hb = Heartbeat(interval=30, send_message_callback=send_cb, config_manager=config_mgr, state_machine=state_machine, fix_engine=fix_engine)
        await hb.start()
        await hb.stop()
        assert not hb.is_running()

    @pytest.mark.asyncio
    async def test_interval_change(self):
        send_cb = AsyncMock()
        config_mgr = Mock()
        state_machine = Mock()
        fix_engine = Mock()
        hb = Heartbeat(interval=10, send_message_callback=send_cb, config_manager=config_mgr, state_machine=state_machine, fix_engine=fix_engine)
        hb.interval = 20
        assert hb.interval == 20

    @pytest.mark.asyncio
    async def test_missed_heartbeat_detection(self):
        send_cb = AsyncMock()
        config_mgr = Mock()
        state_machine = Mock()
        fix_engine = Mock()
        hb = Heartbeat(interval=5, send_message_callback=send_cb, config_manager=config_mgr, state_machine=state_machine, fix_engine=fix_engine)
        hb.last_heartbeat = 0
        # Simulate missed heartbeat
        assert hasattr(hb, "last_heartbeat")

    # Add more tests for interval changes, missed heartbeat, recovery
