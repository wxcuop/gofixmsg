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
		server.Proc = proc
		// send immediate Logon from acceptor to initiator (simpler for test)
		im := engine.NewLogonMessage("SV", "CL")
		if b, err := im.ToWire(); err == nil {
			_, _ = conn.Write(b)
		}
		// register reply to Logon (in case we need to respond again)
		proc.Register("A", func(m *fixmsg.FixMessage) error {
			sender, _ := m.Get(56)
			target, _ := m.Get(49)
			out := engine.NewLogonMessage(sender, target)
			_ = server.SendMessage(out)
			return nil
		})
		// create session
		sess := engine.NewSession(conn, proc)
		// setup components for server and attach session
		stServer := store.NewSQLiteStore()
		_ = stServer.Init(":memory:")
		server.SetupComponents(state.NewStateMachine(), stServer)
		_ = server.AttachSession(sess)
		// start server-side heartbeat sender to exercise initiator receive path
		go func() {
			t := time.NewTicker(20 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-context.Background().Done():
					return
				case <-t.C:
					hb := engine.NewHeartbeatMessage("SV", "CL")
					b, _ := hb.ToWire()
					_ = server.SessionSend(b)
				}
			}
		}()
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
