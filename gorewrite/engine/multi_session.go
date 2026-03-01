package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/network"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// MultiSessionEngine manages multiple concurrent FIX sessions.
// It supports acceptor-mode operation where multiple initiators can connect
// to a single acceptor, each establishing its own independent session.
type MultiSessionEngine struct {
	// session map keyed by session ID
	sessions map[string]*FixEngine
	mu       sync.RWMutex

	// Acceptor listens for incoming connections
	Acceptor *network.Acceptor

	// Application callback interface (shared across all sessions)
	App Application

	// Per-session factories (for creating state machines and stores)
	StateMachineFactory func(string) *state.StateMachine
	StoreFactory        func(string) store.Store
}

// NewMultiSessionEngine creates a new multi-session engine for the given address.
func NewMultiSessionEngine(addr string) *MultiSessionEngine {
	return &MultiSessionEngine{
		sessions:   make(map[string]*FixEngine),
		Acceptor:   network.NewAcceptor(addr),
		App:        &NoOpApplication{},
		StateMachineFactory: NewDefaultStateMachine,
		StoreFactory:        NewDefaultStore,
	}
}

// SetApplication sets the application callback for all sessions.
func (m *MultiSessionEngine) SetApplication(app Application) {
	m.App = app
}

// SetStateMachineFactory sets the factory function for creating state machines per session.
func (m *MultiSessionEngine) SetStateMachineFactory(factory func(string) *state.StateMachine) {
	m.StateMachineFactory = factory
}

// SetStoreFactory sets the factory function for creating stores per session.
func (m *MultiSessionEngine) SetStoreFactory(factory func(string) store.Store) {
	m.StoreFactory = factory
}

// Start begins accepting incoming connections.
// Each connection creates a new session with a unique session ID.
func (m *MultiSessionEngine) Start() error {
	return m.Acceptor.Start(func(conn *network.Conn) {
		m.handleNewConnection(conn)
	})
}

// handleNewConnection processes a new incoming connection.
// It creates a session ID, initializes engine components, and starts message processing.
func (m *MultiSessionEngine) handleNewConnection(conn *network.Conn) {
	// Create session ID (will be finalized after logon contains BeginString, SenderCompID, TargetCompID)
	sessionID := generateSessionID("")

	// Create engine components
	sm := m.StateMachineFactory(sessionID)
	st := m.StoreFactory(sessionID)

	engine := NewFixEngine(nil)
	engine.SetApplication(m.App)
	engine.SetSessionID(sessionID)
	engine.SetupComponents(sm, st)

	// Attach connection and session (conn is already a wrapped network.Conn)
	s := NewSession(conn, engine.Proc)
	engine.Conn = conn.Underlying()

	if err := engine.AttachSession(s); err != nil {
		conn.Close()
		return
	}

	// Register this session in the multi-session map
	m.registerSession(sessionID, engine)

	// Start a monitor goroutine that waits for the session to close
	// and then unregisters it from the map
	go func() {
		// Wait for the session to close by polling Session.ctx
		<-s.ctx.Done()
		// Give the engine's OnClose a moment to complete detach/cleanup
		time.Sleep(10 * time.Millisecond)
		// Unregister from the multi-session map
		m.unregisterSession(sessionID)
	}()
}

// registerSession adds a session to the map.
func (m *MultiSessionEngine) registerSession(sessionID string, engine *FixEngine) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = engine
}

// unregisterSession removes a session from the map and closes it.
func (m *MultiSessionEngine) unregisterSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if engine, exists := m.sessions[sessionID]; exists {
		engine.Close()
		delete(m.sessions, sessionID)
	}
}

// GetSession retrieves a session by ID (read-only).
func (m *MultiSessionEngine) GetSession(sessionID string) (*FixEngine, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	engine, exists := m.sessions[sessionID]
	return engine, exists
}

// SessionCount returns the number of active sessions.
func (m *MultiSessionEngine) SessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Stop closes all active sessions and stops accepting new connections.
func (m *MultiSessionEngine) Stop() error {
	m.Acceptor.Stop()

	m.mu.Lock()
	defer m.mu.Unlock()

	for sessionID, engine := range m.sessions {
		engine.Close()
		delete(m.sessions, sessionID)
	}
	return nil
}

// generateSessionID creates a session ID in the format:
// "BeginString:SenderCompID:TargetCompID"
// If any component is empty, a temporary ID is used and updated after logon.
var sessionIDCounter int64 = 0
var sessionIDMu sync.Mutex

func generateSessionID(parts ...string) string {
	if len(parts) >= 3 && parts[0] != "" && parts[1] != "" && parts[2] != "" {
		return fmt.Sprintf("%s:%s:%s", parts[0], parts[1], parts[2])
	}
	// Return a temporary ID that will be updated on logon
	sessionIDMu.Lock()
	defer sessionIDMu.Unlock()
	sessionIDCounter++
	return fmt.Sprintf("temp-%d", sessionIDCounter)
}

// NewDefaultStateMachine creates a new state machine for a session.
func NewDefaultStateMachine(sessionID string) *state.StateMachine {
	return state.NewStateMachine()
}

// NewDefaultStore creates a new message store for a session.
func NewDefaultStore(sessionID string) store.Store {
	return store.NewSQLiteStore()
}

// BroadcastMessage sends a message to all active sessions (for admin messages).
func (m *MultiSessionEngine) BroadcastMessage(msg *fixmsg.FixMessage) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for _, engine := range m.sessions {
		if err := engine.HandleIncoming(msg); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// SessionIDs returns all active session IDs (for monitoring).
func (m *MultiSessionEngine) SessionIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
}
