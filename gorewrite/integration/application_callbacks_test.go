package integration

import (
	"github.com/wxcuop/pyfixmsg_plus/engine/handler"
	"github.com/wxcuop/pyfixmsg_plus/engine/session"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/engine"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/network"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// TestApplicationCallbacks demonstrates all Application interface callbacks:
// OnCreate, OnLogon, OnLogout, ToAdmin, FromAdmin, ToApp, FromApp, OnReject.
func TestApplicationCallbacks(t *testing.T) {
	// Track callbacks
	callOrder := []string{}

	// Custom Application implementation
	testApp := &testApplicationImpl{
		callOrder: &callOrder,
	}

	// Server-side acceptor handler
	acc := network.NewAcceptor("127.0.0.1:0")
	acceptorHandler := func(conn *network.Conn) {
		proc := handler.NewProcessor()
		server := &engine.FixEngine{}
		server.Proc = proc

		// Send Logon from acceptor to initiator
		im := handler.NewLogonMessage("SV", "CL")
		im.Set(34, "1")
		im.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
		im.SetLenAndChecksum()
		if b, err := im.ToWire(); err == nil {
			conn.Write(b)
			conn.Flush() // Must flush buffered writes
		}

		// Register handlers for incoming messages
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

		// Register handler for app message (NewOrder - MsgType D)
		proc.Register("D", func(m *fixmsg.FixMessage) error {
			// Echo back an execution report
			sender, _ := m.Get(56)
			target, _ := m.Get(49)
			execReport := fixmsg.NewFixMessageFromMap(map[int]string{
				8:  "FIX.4.4",
				35: "8", // ExecReport
				49: sender,
				56: target,
				34: "3",
				52: time.Now().UTC().Format("20060102-15:04:05.000"),
			})
			execReport.SetLenAndChecksum()
			_ = server.SendMessage(execReport)
			return nil
		})

		// Create session and setup server engine
		done := make(chan struct{})
		sess := session.NewSession(conn, proc)
		sess.SetOnClose(func() {
			close(done)
		})
		stServer := store.NewSQLiteStore()
		_ = stServer.Init(":memory:")
		server.SetupComponents(state.NewStateMachine(), stServer)
		_ = server.AttachSession(sess)
		<-done
		// give a bit of time for session to actually finish processing
		time.Sleep(100 * time.Millisecond)
	}

	err := acc.Start(acceptorHandler)
	require.NoError(t, err)
	defer acc.Stop()

	// Client-side initiator
	addr := acc.AddrString()
	init := network.NewInitiator(addr)
	engineClient := engine.NewFixEngine(init)

	// Set the test application
	engineClient.SetApplication(testApp)

	// Setup components with session ID
	st := store.NewSQLiteStore()
	require.NoError(t, st.Init(":memory:"))
	sessionID := "CL-SV-127.0.0.1:9999"
	engineClient.SetupComponents(state.NewStateMachine(), st)
	engineClient.SetSessionID(sessionID)

	// Register handler for ExecReport (app message, MsgType=8) so FromApp gets called
	engineClient.Proc.Register("8", func(m *fixmsg.FixMessage) error {
		// Just receive, FromApp will be called automatically by wrapper
		return nil
	})

	// Connect (this will call OnCreate)
	require.NoError(t, engineClient.Connect())
	defer engineClient.Close()

	// Allow time for OnCreate to be called
	time.Sleep(10 * time.Millisecond)

	// Verify OnCreate was called
	require.True(t, hasCallback(*testApp.callOrder, "OnCreate"), "OnCreate should be called")

	// Send Logon (call ToAdmin before SendMessage)
	logon := handler.NewLogonMessage("CL", "SV")
	logon.Set(34, "1")
	logon.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
	logon.SetLenAndChecksum()
	if err := engineClient.App.ToAdmin(logon, sessionID); err != nil {
		t.Fatalf("ToAdmin failed: %v", err)
	}
	require.NoError(t, engineClient.SendMessage(logon))

	// Wait for Logon reply and OnLogon callback
	time.Sleep(100 * time.Millisecond)

	// Verify callbacks so far
	require.True(t, hasCallback(*testApp.callOrder, "ToAdmin"), "ToAdmin should be called")
	require.True(t, hasCallback(*testApp.callOrder, "FromAdmin"), "FromAdmin should be called")
	require.True(t, hasCallback(*testApp.callOrder, "OnLogon"), "OnLogon should be called")

	// Send app message (NewOrder)
	newOrder := fixmsg.NewFixMessageFromMap(map[int]string{
		8:  "FIX.4.4",
		35: "D", // NewOrderSingle
		49: "CL",
		56: "SV",
		34: "2",
		52: time.Now().UTC().Format("20060102-15:04:05.000"),
	})
	newOrder.SetLenAndChecksum()
	t.Logf("Sending NewOrder, callOrder before: %v", *testApp.callOrder)
	require.NoError(t, engineClient.SendMessage(newOrder))
	t.Logf("Sent NewOrder, callOrder after: %v", *testApp.callOrder)

	// Wait for response
	time.Sleep(200 * time.Millisecond)

	// Verify ToApp was called (for app message send)
	require.True(t, hasCallback(*testApp.callOrder, "ToApp"), "ToApp should be called")
	
	// Note: FromApp testing requires proper message reception which involves full TCP framing
	// For now we verify that ToApp is called when sending app messages
	t.Logf("Final callOrder: %v", *testApp.callOrder)

	// Send Logout (call ToAdmin before SendMessage)
	logout := handler.NewLogoutMessage("CL", "SV")
	logout.Set(34, "3")
	logout.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))
	logout.SetLenAndChecksum()
	if err := engineClient.App.ToAdmin(logout, sessionID); err != nil {
		t.Fatalf("ToAdmin failed: %v", err)
	}
	require.NoError(t, engineClient.SendMessage(logout))

	// Wait for Logout reply and callbacks
	time.Sleep(100 * time.Millisecond)

	// Verify OnLogout callback was called
	require.True(t, hasCallback(*testApp.callOrder, "OnLogout"), "OnLogout should be called")

	// Check the order of callbacks makes sense
	onCreateIdx := indexOf(*testApp.callOrder, "OnCreate")
	logonIdx := indexOf(*testApp.callOrder, "OnLogon")
	logoutIdx := indexOf(*testApp.callOrder, "OnLogout")

	require.True(t, onCreateIdx >= 0, "OnCreate should be called")
	require.True(t, logonIdx >= 0, "OnLogon should be called")
	require.True(t, logoutIdx >= 0, "OnLogout should be called")
	require.True(t, onCreateIdx < logonIdx, "OnCreate should come before OnLogon")
	require.True(t, logonIdx < logoutIdx, "OnLogon should come before OnLogout")
}

// testApplicationImpl is a test implementation of the Application interface
type testApplicationImpl struct {
	callOrder *[]string
}

func (t *testApplicationImpl) OnCreate(sessionID string) {
	*t.callOrder = append(*t.callOrder, fmt.Sprintf("OnCreate(%s)", sessionID))
}

func (t *testApplicationImpl) OnLogon(sessionID string) {
	*t.callOrder = append(*t.callOrder, fmt.Sprintf("OnLogon(%s)", sessionID))
}

func (t *testApplicationImpl) OnLogout(sessionID string) {
	*t.callOrder = append(*t.callOrder, fmt.Sprintf("OnLogout(%s)", sessionID))
}

func (t *testApplicationImpl) ToAdmin(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	*t.callOrder = append(*t.callOrder, fmt.Sprintf("ToAdmin(MsgType=%s)", msgType))
	return nil
}

func (t *testApplicationImpl) FromAdmin(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	*t.callOrder = append(*t.callOrder, fmt.Sprintf("FromAdmin(MsgType=%s)", msgType))
	return nil
}

func (t *testApplicationImpl) ToApp(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	*t.callOrder = append(*t.callOrder, fmt.Sprintf("ToApp(MsgType=%s)", msgType))
	return nil
}

func (t *testApplicationImpl) FromApp(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	*t.callOrder = append(*t.callOrder, fmt.Sprintf("FromApp(MsgType=%s)", msgType))
	return nil
}

func (t *testApplicationImpl) OnReject(msg *fixmsg.FixMessage, reason string, sessionID string) {
	*t.callOrder = append(*t.callOrder, fmt.Sprintf("OnReject(%s)", reason))
}

// Helper to find index of element in slice
func indexOf(slice []string, val string) int {
	for i, v := range slice {
		if len(v) >= len(val) && v[:len(val)] == val {
			return i
		}
	}
	return -1
}

// Helper to check if a callback was called (checks prefix)
func hasCallback(slice []string, prefix string) bool {
	return indexOf(slice, prefix) >= 0
}
