import sys
import os
import pytest
sys.path.append(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..'))

from pyfixmsg_plus.fixengine.configmanager import ConfigManager

def test_config_manager_initialization():
    config_manager = ConfigManager()
    assert config_manager is not None
    assert isinstance(config_manager, ConfigManager)

def test_config_manager_singleton():
    cm1 = ConfigManager()
    cm2 = ConfigManager()
    assert cm1 is cm2  # They should be the same instance

def test_config_manager_load_config():
    config_manager = ConfigManager()
    config_manager.load_config()
    assert config_manager.get('FIX', 'sender_comp_id', 'SENDER') == 'SENDER'

if __name__ == "__main__":
    pytest.main()
