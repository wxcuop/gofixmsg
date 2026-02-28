package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/network"
	"github.com/wxcuop/pyfixmsg_plus/engine"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
)

func TestInitiatorAcceptor_LogonAndHeartbeat(t *testing.T) {
	acc := network.NewAcceptor("127.0.0.1:0")
	// handler factory
	handler := func(conn net.Conn) {
		// server-side processor and engine
		proc := engine.NewProcessor()
		server := &engine.FixEngine{}
		// register reply to Logon
		proc.Register("A", func(m *fixmsg.FixMessage) error {
			sender, _ := m.Get(56)
			target, _ := m.Get(49)
			out := engine.NewLogonMessage(sender, target)
			b, _ := out.ToWire()
			_ = server.SessionSend(b)
			return nil
		})
		// create session
		sess := engine.NewSession(conn, proc)
		server.Session = sess
		// monitor
		server.Monitor = engine.NewHeartbeatMonitor(server, 20*time.Millisecond, 40*time.Millisecond)
		sess.SetOnMessage(func(m *fixmsg.FixMessage) { _ = server.HandleIncoming(m); server.Monitor.Seen() })
		sess.Start()
		server.Monitor.Start(context.Background())
	}

	err := acc.Start(handler)
	require.NoError(t, err)
	defer acc.Stop()

	addr := acc.AddrString()
	init := network.NewInitiator(addr)
	engineClient := engine.NewFixEngine(init)
	// small heartbeat for test
	engineClient.Monitor = engine.NewHeartbeatMonitor(engineClient, 20*time.Millisecond, 40*time.Millisecond)
	// setup components with in-memory sqlite store
	st := store.NewSQLiteStore()
	require.NoError(t, st.Init(":memory:"))
	engineClient.SetupComponents(state.NewStateMachine(), st)

	// channels to observe events
	logonCh := make(chan struct{}, 1)
	hbCh := make(chan struct{}, 1)

	// register handlers to observe
	engineClient.Proc.Register("A", func(m *fixmsg.FixMessage) error {
		select { case logonCh <- struct{}{}: default: }
		return nil
	})
	engineClient.Proc.Register("0", func(m *fixmsg.FixMessage) error {
		select { case hbCh <- struct{}{}: default: }
		return nil
	})

	// connect the initiator (starts session and monitor)
	require.NoError(t, engineClient.Connect())
	defer engineClient.Close()

	// send Logon from initiator
	out := engine.NewLogonMessage("CL","SV")
	require.NoError(t, engineClient.SendMessage(out))

	// wait for logon reply
	select {
	case <-logonCh:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for logon reply")
	}

	// wait for a heartbeat to be received
	select {
	case <-hbCh:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for heartbeat")
	}
}
