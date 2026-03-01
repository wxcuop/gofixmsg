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

func TestInitiatorAcceptor_TestRequestEscalation(t *testing.T) {
	acc := network.NewAcceptor("127.0.0.1:0")
	testReqCh := make(chan *fixmsg.FixMessage, 1)

	acceptorHandler := func(conn *network.Conn) {
		// send immediate Logon from acceptor to initiator
		im := handler.NewLogonMessage("SV", "CL")
		im.Set(34, "1")
		im.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
		im.SetLenAndChecksum()
		if b, err := im.ToWire(); err == nil {
			conn.Write(b)
			conn.Flush() // Must flush buffered writes
		}

		// server-side: just listen and capture TestRequest
		proc := handler.NewProcessor()
		server := &engine.FixEngine{}
		server.Proc = proc

		done := make(chan struct{})
		sess := session.NewSession(conn, proc)
		sess.SetOnClose(func() {
			close(done)
		})
		stServer := store.NewSQLiteStore()
		_ = stServer.Init(":memory:")
		server.SetupComponents(state.NewStateMachine(), stServer)
		// Register custom handlers AFTER SetupComponents to override defaults
		proc.Register("1", func(m *fixmsg.FixMessage) error {
			select {
			case testReqCh <- m:
			default:
			}
			return nil
		})
		_ = server.AttachSession(sess)
		<-done
		time.Sleep(100 * time.Millisecond)
	}

	err := acc.Start(acceptorHandler)
	require.NoError(t, err)
	defer acc.Stop()

	addr := acc.AddrString()
	init := network.NewInitiator(addr)
	initiator := engine.NewFixEngine(init)

	// fast heartbeat monitor: 50ms interval, 100ms timeout
	initiator.Monitor = engine.NewHeartbeatMonitor(initiator, 50*time.Millisecond, 100*time.Millisecond)

	st := store.NewSQLiteStore()
	require.NoError(t, st.Init(":memory:"))
	initiator.SetupComponents(state.NewStateMachine(), st)

	// connect and send a Logon from initiator
	require.NoError(t, initiator.Connect())
	defer initiator.Close()
	out := handler.NewLogonMessage("CL", "SV")
	out.Set(34, "1")
	out.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
	out.SetLenAndChecksum()
	require.NoError(t, initiator.SendMessage(out))

	// expect a TestRequest to be sent by initiator within ~200ms
	select {
	case m := <-testReqCh:
		mt, _ := m.Get(35)
		require.Equal(t, "1", mt)
	case <-time.After(1000 * time.Millisecond):
		t.Fatal("timeout waiting for TestRequest")
	}
}
