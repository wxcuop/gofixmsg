package engine

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/wxcuop/pyfixmsg_plus/config"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/heartbeat"
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
	Monitor   *HeartbeatMonitor
	// configured CompIDs for outgoing admin messages
	SenderCompID string
	TargetCompID string
	// heartbeat sender
	hbSender          *heartbeat.Heartbeat
	heartbeatInterval time.Duration
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
	// read heartbeat interval and comp ids from config manager if present
	mgr := config.GetManager()
	intervalSec := 30
	if mgr != nil {
		if v := mgr.Get("", "heartbeat_interval"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				intervalSec = n
			}
		}
		// only set comp ids if not already configured on engine
		if e.SenderCompID == "" {
			if v := mgr.Get("", "sender_comp_id"); v != "" {
				e.SenderCompID = v
			} else {
				e.SenderCompID = "S"
			}
		}
		if e.TargetCompID == "" {
			if v := mgr.Get("", "target_comp_id"); v != "" {
				e.TargetCompID = v
			} else {
				e.TargetCompID = "T"
			}
		}
	}
	// ensure defaults
	if e.SenderCompID == "" {
		e.SenderCompID = "S"
	}
	if e.TargetCompID == "" {
		e.TargetCompID = "T"
	}
	e.heartbeatInterval = time.Duration(intervalSec) * time.Second
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
	// create session and attach
	s := NewSession(c, e.Proc)
	return e.AttachSession(s)
}

func (e *FixEngine) Close() error {
	// stop heartbeat and monitor and session
	e.DetachSession()
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
