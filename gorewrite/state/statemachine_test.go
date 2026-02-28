package state_test

import (
"sync"
"testing"
"time"

"github.com/stretchr/testify/require"
"github.com/wxcuop/pyfixmsg_plus/state"
)

func TestStateMachine_Transitions(t *testing.T) {
sm := state.NewStateMachine()
require.Equal(t, state.StateDisconnected, sm.State())

// Disconnected -> Connecting
ns := sm.OnEvent("connect")
require.Equal(t, state.StateConnecting, ns)
// Connecting -> AwaitingLogon
ns = sm.OnEvent("connected")
require.Equal(t, state.StateAwaitingLogon, ns)
// AwaitingLogon -> LogonInProgress
ns = sm.OnEvent("logon_sent")
require.Equal(t, state.StateLogonInProgress, ns)
// LogonInProgress -> Active
ns = sm.OnEvent("logon_received")
require.Equal(t, state.StateActive, ns)
// Active -> LogoutInProgress
ns = sm.OnEvent("logout")
require.Equal(t, state.StateLogoutInProgress, ns)
// LogoutInProgress -> Disconnected
ns = sm.OnEvent("logout_complete")
require.Equal(t, state.StateDisconnected, ns)
}

func TestStateMachine_GlobalDisconnected(t *testing.T) {
sm := state.NewStateMachine()
sm.OnEvent("connect")
require.Equal(t, state.StateConnecting, sm.State())
// global disconnected event
sm.OnEvent("disconnected")
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
sm.OnEvent("connect")
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
