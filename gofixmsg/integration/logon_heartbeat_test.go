package integration

import (
	"github.com/wxcuop/gofixmsg/engine/handler"
	"github.com/wxcuop/gofixmsg/engine/session"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/engine"
	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/network"
	"github.com/wxcuop/gofixmsg/state"
	"github.com/wxcuop/gofixmsg/store"
)

func TestInitiatorAcceptor_LogonAndHeartbeat(t *testing.T) {
	acc := network.NewAcceptor("127.0.0.1:0")
	// handler factory
	acceptorHandler := func(conn *network.Conn) {
		// server-side processor and engine
		proc := handler.NewProcessor()
		server := &engine.FixEngine{}
		server.Proc = proc
		// send immediate Logon from acceptor to initiator (simpler for test)
		im := handler.NewLogonMessage("SV", "CL")
		im.Set(34, "1")
		im.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
		im.SetLenAndChecksum()
		if b, err := im.ToWire(); err == nil {
			conn.Write(b)
			conn.Flush() // Must flush buffered writes
		}
		// register reply to Logon (in case we need to respond again)
		proc.Register("A", func(m *fixmsg.FixMessage) error {
			sender, _ := m.Get(56)
			target, _ := m.Get(49)
			out := handler.NewLogonMessage(sender, target)
			out.Set(34, "2")
			out.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
			out.SetLenAndChecksum()
			_ = server.SendMessage(out)
			return nil
		})
		// create session
		done := make(chan struct{})
		sess := session.NewSession(conn, proc)
		sess.SetOnClose(func() {
			close(done)
		})
		// setup components for server and attach session
		stServer := store.NewSQLiteStore()
		_ = stServer.Init(":memory:")
		server.SetupComponents(state.NewStateMachine(), stServer)
		_ = server.AttachSession(sess)
		// start server-side heartbeat sender to exercise initiator receive path
		go func() {
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-sess.Context().Done():
					return
				case <-ticker.C:
					hb := handler.NewHeartbeatMessage("SV", "CL")
					err := server.SendMessage(hb)
					if err != nil {
						// could be session closing
						return
					}
				}
			}
		}()
		<-done
		time.Sleep(100 * time.Millisecond)
	}

	err := acc.Start(acceptorHandler)
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
	out := handler.NewLogonMessage("CL","SV")
	out.Set(34, "1")
	out.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
	out.SetLenAndChecksum()
	require.NoError(t, engineClient.SendMessage(out))

	// wait for logon reply
	select {
	case <-logonCh:
		// ok
	case <-time.After(1000 * time.Millisecond):
		t.Fatal("timeout waiting for logon reply")
	}

	// wait for a heartbeat to be received
	select {
	case <-hbCh:
		// ok
	case <-time.After(1000 * time.Millisecond):
		t.Fatal("timeout waiting for heartbeat")
	}
}
