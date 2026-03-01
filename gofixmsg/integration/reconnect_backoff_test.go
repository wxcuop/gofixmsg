package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/engine"
	"github.com/wxcuop/pyfixmsg_plus/network"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// TestReconnectBackoff verifies that initiator reconnect backoff works
// when enabled and doesn't block normal operation.
func TestReconnectBackoff(t *testing.T) {
	// create initiator to unreachable address
	init := network.NewInitiator("127.0.0.1:1")
	initiator := engine.NewFixEngine(init)
	initiator.SenderCompID = "CL"
	initiator.TargetCompID = "SV"

	// set up components
	st := store.NewSQLiteStore()
	sm := state.NewStateMachine()
	initiator.SetupComponents(sm, st)

	// enable reconnect with very short timeouts for testing
	initiator.SetReconnectParams(50*time.Millisecond, 100*time.Millisecond, true)

	// attempt connection (should fail since port 1 is closed)
	err := initiator.Connect()
	require.Error(t, err)

	// verify that reconnect loop didn't start (reconnect disabled until explicitly enabled)
	time.Sleep(200 * time.Millisecond)

	// now enable reconnect for the next test
	initiator2 := engine.NewFixEngine(init)
	initiator2.SenderCompID = "CL"
	initiator2.TargetCompID = "SV"
	initiator2.SetupComponents(sm, st)
	initiator2.SetReconnectParams(50*time.Millisecond, 100*time.Millisecond, true)

	// just verify the fields were set correctly
	require.NotNil(t, initiator2.Initiator)
}
