"""
Comprehensive unit tests for StateMachine: transitions, event handling, error scenarios, property-based edge cases.
"""
import pytest
from unittest.mock import Mock
from pyfixmsg_plus.fixengine.state_machine import (
    StateMachine, Disconnected, Connecting, LogonInProgress, Active, LogoutInProgress, AwaitingLogon
)

class TestStateMachineTransitions:
    def test_initial_state(self):
        sm = StateMachine(Disconnected())
        assert sm.state.name == "DISCONNECTED"

    def test_connecting_transition(self):
        sm = StateMachine(Disconnected())
        sm.on_event("initiator_connect_attempt")
        assert sm.state.name == "CONNECTING"

    def test_logon_in_progress_transition(self):
        sm = StateMachine(Connecting())
        sm.on_event("connection_established")
        assert sm.state.name == "LOGON_IN_PROGRESS"

    def test_active_transition(self):
        sm = StateMachine(LogonInProgress())
        sm.on_event("logon_successful")
        assert sm.state.name == "ACTIVE"

    def test_logout_transition(self):
        sm = StateMachine(Active())
        sm.on_event("logout_initiated")
        assert sm.state.name == "LOGOUT_IN_PROGRESS"

    def test_disconnect_from_active(self):
        sm = StateMachine(Active())
        sm.on_event("disconnect")
        assert sm.state.name == "DISCONNECTED"

    def test_disconnect_from_logout(self):
        sm = StateMachine(LogoutInProgress())
        sm.on_event("logout_confirmed")
        assert sm.state.name == "DISCONNECTED"

    def test_invalid_transition_stays_same(self):
        sm = StateMachine(Disconnected())
        # Invalid event should keep same state
        sm.on_event("invalid_event")
        assert sm.state.name == "DISCONNECTED"

    def test_connection_failed_from_connecting(self):
        sm = StateMachine(Connecting())
        sm.on_event("connection_failed")
        assert sm.state.name == "DISCONNECTED"

    def test_logon_failed_from_logon_in_progress(self):
        sm = StateMachine(LogonInProgress())
        sm.on_event("logon_failed")
        assert sm.state.name == "DISCONNECTED"

class TestStateMachineEvents:
    def test_event_handling_with_subscribers(self):
        sm = StateMachine(Disconnected())
        callback_mock = Mock()
        sm.subscribe(callback_mock)
        
        # Trigger state change
        sm.on_event("initiator_connect_attempt")
        assert sm.state.name == "CONNECTING"
        callback_mock.assert_called_once()

    def test_acceptor_events(self):
        sm = StateMachine(Disconnected())
        sm.on_event("client_accepted_awaiting_logon")
        assert sm.state.name == "AWAITING_LOGON"

    def test_awaiting_logon_to_active(self):
        sm = StateMachine(AwaitingLogon())
        sm.on_event("logon_received_valid")
        assert sm.state.name == "ACTIVE"

    def test_awaiting_logon_invalid(self):
        sm = StateMachine(AwaitingLogon())
        sm.on_event("invalid_logon_received")
        assert sm.state.name == "DISCONNECTED"

class TestStateMachinePropertyBased:
    def test_all_states_have_names(self):
        states = [Disconnected(), Connecting(), LogonInProgress(), Active(), LogoutInProgress(), AwaitingLogon()]
        for state in states:
            assert hasattr(state, "name")
            assert isinstance(state.name, str)
            assert len(state.name) > 0

    def test_state_machine_consistency(self):
        # Test that state machine maintains consistency
        sm = StateMachine(Disconnected())
        original_state = sm.state
        
        # Invalid events shouldn't change state
        sm.on_event("nonexistent_event")
        assert sm.state == original_state

# Property-based edge case tests can be added here
