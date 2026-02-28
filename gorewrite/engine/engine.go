package engine

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/network"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// FixEngine holds components needed for a session.
type FixEngine struct {
	Initiator *network.Initiator
	Conn      net.Conn
	Session   *Session
	Proc      *Processor
	SM        *state.StateMachine
	Store     store.Store
	SeqMgr    *SeqManager
}

func (e *FixEngine) SessionSend(b []byte) error {
	if e.Session == nil {
		if e.Conn == nil {
			return fmt.Errorf("no connection/session")
		}
		// fallback: direct write
		_, err := e.Conn.Write(b)
		return err
	}
	return e.Session.Send(b)
}

func NewFixEngine(init *network.Initiator) *FixEngine { return &FixEngine{Initiator: init} }

// SetupComponents wires state machine and store into the engine and registers handlers.
func (e *FixEngine) SetupComponents(sm *state.StateMachine, st store.Store) {
	e.SM = sm
	e.Store = st
	e.Proc = NewProcessor()
	// create sequence manager with a session id derived from initiator address if present
	sid := "default"
	if e.Initiator != nil && e.Initiator.Addr != "" {
		sid = e.Initiator.Addr
	}
	e.SeqMgr = NewSeqManager(st, sid)
	ctx := &HandlerContext{SM: sm, Store: st, Engine: e}
	RegisterDefaultHandlers(e.Proc, ctx)
}

func (e *FixEngine) Connect() error {
	if e.Initiator == nil {
		return fmt.Errorf("no initiator configured")
	}
	c, err := e.Initiator.Connect()
	if err != nil {
		return err
	}
	e.Conn = c
	// create session and wire message callback to engine/monitor
	s := NewSession(c, e.Proc)
	e.Session = s
	if e.Monitor == nil {
		e.Monitor = NewHeartbeatMonitor(e, 30*time.Second, 30*time.Second)
	}
	// On inbound message, pass to engine and notify monitor
	s.SetOnMessage(func(m *fixmsg.FixMessage) {
		_ = e.HandleIncoming(m)
		if e.Monitor != nil {
			e.Monitor.Seen()
		}
	})
	s.Start()
	// start monitor
	e.Monitor.Start(context.Background())
	return nil
}

func (e *FixEngine) Close() error {
	if e.Monitor != nil {
		e.Monitor.Stop()
	}
	if e.Session != nil {
		e.Session.Stop()
	}
	if e.Conn != nil {
		return e.Conn.Close()
	}
	return nil
}

// HandleIncoming processes an incoming FIX message using registered handlers.
func (e *FixEngine) HandleIncoming(m *fixmsg.FixMessage) error {
	if e.Proc == nil {
		return fmt.Errorf("processor not configured")
	}
	return e.Proc.Process(m)
}
