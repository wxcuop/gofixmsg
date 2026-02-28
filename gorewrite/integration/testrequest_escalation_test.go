package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/engine"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/network"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// TestInitiatorAcceptor_TestRequestEscalation verifies that an initiator will
// send a TestRequest when the acceptor stops sending heartbeats, and will
// close the session if no response is received within the TestReq timeout.
func TestInitiatorAcceptor_TestRequestEscalation(t *testing.T) {
	acc := network.NewAcceptor("127.0.0.1:0")
	testReqCh := make(chan *fixmsg.FixMessage, 1)

	handler := func(conn *network.Conn) {
		// send immediate Logon from acceptor to initiator
		im := engine.NewLogonMessage("SV", "CL")
		if b, err := im.ToWire(); err == nil {
			conn.Write(b)
			conn.Flush() // Must flush buffered writes
		}

		// processor that records TestRequest messages but does NOT reply
		proc := engine.NewProcessor()
		proc.Register("1", func(m *fixmsg.FixMessage) error {
			select {
			case testReqCh <- m:
			default:
			}
			return nil
		})
		proc.Register("A", func(m *fixmsg.FixMessage) error {
			// acceptor acknowledges logon but does not actively send heartbeats
			return nil
		})
		// start session to process inbound frames
		sess := engine.NewSession(conn, proc)
		sess.Start()
	}

	err := acc.Start(handler)
	require.NoError(t, err)
	defer acc.Stop()

	addr := acc.AddrString()
	init := network.NewInitiator(addr)
	initiator := engine.NewFixEngine(init)
	// configure comp ids for the initiator
	initiator.SenderCompID = "CL"
	initiator.TargetCompID = "SV"
	// small heartbeat interval and timeout for fast test
	initiator.Monitor = engine.NewHeartbeatMonitor(initiator, 20*time.Millisecond, 40*time.Millisecond)

	st := store.NewSQLiteStore()
	require.NoError(t, st.Init(":memory:"))
	initiator.SetupComponents(state.NewStateMachine(), st)

	// connect and send a Logon from initiator
	require.NoError(t, initiator.Connect())
	defer initiator.Close()
	out := engine.NewLogonMessage("CL", "SV")
	require.NoError(t, initiator.SendMessage(out))

	// expect a TestRequest to be sent by initiator within ~200ms
	select {
	case m := <-testReqCh:
		require.True(t, m.TagExact(35, "1", false))
		// verify comp ids are present
		s, _ := m.Get(49)
		gt, _ := m.Get(56)
		require.Equal(t, "CL", s)
		require.Equal(t, "SV", gt)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for TestRequest")
	}

	// wait for TestReq timeout + margin and then expect session is closed (SendMessage should fail)
	time.Sleep(80 * time.Millisecond)
	hb := engine.NewHeartbeatMessage("CL", "SV")
	require.Error(t, initiator.SendMessage(hb))
}
