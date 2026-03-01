package state_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/state"
)

func TestStateMachine_Transitions(t *testing.T) {
	sm := state.NewStateMachine()
	require.Equal(t, state.StateDisconnected, sm.State())

	// Disconnected -> Connecting
	ns, err := sm.OnEvent(state.EventConnect)
	require.NoError(t, err)
	require.Equal(t, state.StateConnecting, ns)
	// Connecting -> AwaitingLogon
	ns, err = sm.OnEvent(state.EventConnected)
	require.NoError(t, err)
	require.Equal(t, state.StateAwaitingLogon, ns)
	// AwaitingLogon -> LogonInProgress
	ns, err = sm.OnEvent(state.EventLogonSent)
	require.NoError(t, err)
	require.Equal(t, state.StateLogonInProgress, ns)
	// LogonInProgress -> Active
	ns, err = sm.OnEvent(state.EventLogonReceived)
	require.NoError(t, err)
	require.Equal(t, state.StateActive, ns)
	// Active -> LogoutInProgress
	ns, err = sm.OnEvent(state.EventLogout)
	require.NoError(t, err)
	require.Equal(t, state.StateLogoutInProgress, ns)
	// LogoutInProgress -> Disconnected
	ns, err = sm.OnEvent(state.EventLogoutComplete)
	require.NoError(t, err)
	require.Equal(t, state.StateDisconnected, ns)
}

func TestStateMachine_GlobalDisconnected(t *testing.T) {
	sm := state.NewStateMachine()
	_, err := sm.OnEvent(state.EventConnect)
	require.NoError(t, err)
	require.Equal(t, state.StateConnecting, sm.State())
	// global disconnected event
	_, err = sm.OnEvent(state.EventDisconnected)
	require.NoError(t, err)
	require.Equal(t, state.StateDisconnected, sm.State())
}

func TestStateMachine_SubscriberNotified(t *testing.T) {
	sm := state.NewStateMachine()
	var wg sync.WaitGroup
	wg.Add(1)
	var got string
	sm.Register(func(s string) {
		got = s
		wg.Done()
	})
	_, err := sm.OnEvent(state.EventConnect)
	require.NoError(t, err)
	// wait for notification
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()
	select {
	case <-waitCh:
		require.Equal(t, state.StateConnecting, got)
	case <-time.After(1 * time.Second):
		require.Fail(t, "timeout waiting for subscriber")
	}
}

func TestStateMachine_UndefinedTransition(t *testing.T) {
	sm := state.NewStateMachine()
	// Try an undefined transition
	_, err := sm.OnEvent("invalid_event")
	require.Error(t, err)
	// State should remain unchanged
	require.Equal(t, state.StateDisconnected, sm.State())
}

func TestStateMachine_NewEvents(t *testing.T) {
	sm := state.NewStateMachine()
	// Test client_accepted event
	_, err := sm.OnEvent(state.EventConnect)
	require.NoError(t, err)
	ns, err := sm.OnEvent(state.EventClientAccepted)
	require.NoError(t, err)
	require.Equal(t, state.StateAwaitingLogon, ns)

	// Test initiate_reconnect
	sm = state.NewStateMachine()
	_, _ = sm.OnEvent(state.EventConnect)
	_, _ = sm.OnEvent(state.EventConnected)
	_, _ = sm.OnEvent(state.EventLogonSent)
	_, _ = sm.OnEvent(state.EventLogonFailed)
	require.Equal(t, state.StateReconnecting, sm.State())

	// initiate_reconnect should stay in Reconnecting
	ns, err = sm.OnEvent(state.EventInitiateReconnect)
	require.NoError(t, err)
	require.Equal(t, state.StateReconnecting, ns)

	// reconnect_failed_max_retries should go to Disconnected
	ns, err = sm.OnEvent(state.EventReconnectFailedMax)
	require.NoError(t, err)
	require.Equal(t, state.StateDisconnected, ns)
}
