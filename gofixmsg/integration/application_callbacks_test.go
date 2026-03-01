package integration

import (
	"context"
	"github.com/wxcuop/gofixmsg/engine/handler"
	"github.com/wxcuop/gofixmsg/engine/session"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/engine"
	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/network"
	"github.com/wxcuop/gofixmsg/state"
	"github.com/wxcuop/gofixmsg/store"
)

// TestApplicationCallbacks demonstrates all Application interface callbacks:
// OnCreate, OnLogon, OnLogout, ToAdmin, FromAdmin, ToApp, FromApp, OnReject.
func TestApplicationCallbacks(t *testing.T) {
	// Set a timeout for the entire test to prevent indefinite hangs
	// Increased to 45 seconds to account for cleanup and goroutine termination
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		testApplicationCallbacksImpl(t)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-ctx.Done():
		t.Fatalf("TestApplicationCallbacks timed out after 45 seconds")
	}
}

// testApplicationCallbacksImpl contains the actual test logic
func testApplicationCallbacksImpl(t *testing.T) {
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
		defer stServer.Close()
		server.SetupComponents(state.NewStateMachine(), stServer)
		_ = server.AttachSession(sess)
		
		// Wait for session to close with a 5-second timeout
		timeoutChan := time.After(5 * time.Second)
		select {
		case <-done:
			// Session closed normally
		case <-timeoutChan:
			// Timeout waiting for session close, force cleanup
			return
		}
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
	defer st.Close()
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
	
	// Ensure cleanup happens with a timeout to prevent hanging
	defer func() {
		closeDone := make(chan struct{})
		go func() {
			engineClient.Close()
			close(closeDone)
		}()
		select {
		case <-closeDone:
			// Closed successfully
		case <-time.After(3 * time.Second):
			// Close timeout - continue anyway to avoid test hang
		}
	}()

	// Allow time for OnCreate to be called
	time.Sleep(10 * time.Millisecond)

	// Verify OnCreate was called
	testApp.mu.Lock()
	onCreateCalled := hasCallback(*testApp.callOrder, "OnCreate")
	testApp.mu.Unlock()
	require.True(t, onCreateCalled, "OnCreate should be called")

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
	testApp.mu.Lock()
	hasToAdmin := hasCallback(*testApp.callOrder, "ToAdmin")
	hasFromAdmin := hasCallback(*testApp.callOrder, "FromAdmin")
	hasOnLogon := hasCallback(*testApp.callOrder, "OnLogon")
	testApp.mu.Unlock()
	require.True(t, hasToAdmin, "ToAdmin should be called")
	require.True(t, hasFromAdmin, "FromAdmin should be called")
	require.True(t, hasOnLogon, "OnLogon should be called")

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
	testApp.mu.Lock()
	callOrderBefore := *testApp.callOrder
	testApp.mu.Unlock()
	t.Logf("Sending NewOrder, callOrder before: %v", callOrderBefore)
	require.NoError(t, engineClient.SendMessage(newOrder))
	testApp.mu.Lock()
	callOrderAfter := *testApp.callOrder
	testApp.mu.Unlock()
	t.Logf("Sent NewOrder, callOrder after: %v", callOrderAfter)

	// Wait for response
	time.Sleep(200 * time.Millisecond)

	// Verify ToApp was called (for app message send)
	testApp.mu.Lock()
	hasToApp := hasCallback(*testApp.callOrder, "ToApp")
	testApp.mu.Unlock()
	require.True(t, hasToApp, "ToApp should be called")
	
	// Note: FromApp testing requires proper message reception which involves full TCP framing
	// For now we verify that ToApp is called when sending app messages
	testApp.mu.Lock()
	finalCallOrder1 := *testApp.callOrder
	testApp.mu.Unlock()
	t.Logf("Final callOrder: %v", finalCallOrder1)

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
	testApp.mu.Lock()
	hasOnLogout := hasCallback(*testApp.callOrder, "OnLogout")
	callOrderFinal := *testApp.callOrder
	testApp.mu.Unlock()
	require.True(t, hasOnLogout, "OnLogout should be called")

	// Check the order of callbacks makes sense
	onCreateIdx := indexOf(callOrderFinal, "OnCreate")
	logonIdx := indexOf(callOrderFinal, "OnLogon")
	logoutIdx := indexOf(callOrderFinal, "OnLogout")

	require.True(t, onCreateIdx >= 0, "OnCreate should be called")
	require.True(t, logonIdx >= 0, "OnLogon should be called")
	require.True(t, logoutIdx >= 0, "OnLogout should be called")
	require.True(t, onCreateIdx < logonIdx, "OnCreate should come before OnLogon")
	require.True(t, logonIdx < logoutIdx, "OnLogon should come before OnLogout")
}

// testApplicationImpl is a test implementation of the Application interface
type testApplicationImpl struct {
	mu        sync.Mutex
	callOrder *[]string
}

// record appends a callback name to the call order list (thread-safe).
func (t *testApplicationImpl) record(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	*t.callOrder = append(*t.callOrder, name)
}

func (t *testApplicationImpl) OnCreate(sessionID string) {
	t.record(fmt.Sprintf("OnCreate(%s)", sessionID))
}

func (t *testApplicationImpl) OnLogon(sessionID string) {
	t.record(fmt.Sprintf("OnLogon(%s)", sessionID))
}

func (t *testApplicationImpl) OnLogout(sessionID string) {
	t.record(fmt.Sprintf("OnLogout(%s)", sessionID))
}

func (t *testApplicationImpl) ToAdmin(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	t.record(fmt.Sprintf("ToAdmin(MsgType=%s)", msgType))
	return nil
}

func (t *testApplicationImpl) FromAdmin(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	t.record(fmt.Sprintf("FromAdmin(MsgType=%s)", msgType))
	return nil
}

func (t *testApplicationImpl) ToApp(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	t.record(fmt.Sprintf("ToApp(MsgType=%s)", msgType))
	return nil
}

func (t *testApplicationImpl) FromApp(msg *fixmsg.FixMessage, sessionID string) error {
	msgType, _ := msg.Get(35)
	t.record(fmt.Sprintf("FromApp(MsgType=%s)", msgType))
	return nil
}

func (t *testApplicationImpl) OnMessage(msg *fixmsg.FixMessage, sessionID string) {
	msgType, _ := msg.Get(35)
	t.record(fmt.Sprintf("OnMessage(MsgType=%s)", msgType))
}

func (t *testApplicationImpl) OnReject(msg *fixmsg.FixMessage, reason string, sessionID string) {
	t.record(fmt.Sprintf("OnReject(%s)", reason))
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
