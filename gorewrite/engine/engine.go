package engine

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
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
	// reconnect/backoff
	reconnectMu      sync.Mutex
	reconnectCtx     context.Context
	reconnectCancel  context.CancelFunc
	reconnectWG      sync.WaitGroup
	reconnectInitial time.Duration
	reconnectMax     time.Duration
	enableReconnect  bool // disable for tests by default
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

func NewFixEngine(init *network.Initiator) *FixEngine {
	// seed random for jitter
	rand.Seed(time.Now().UnixNano())
	fe := &FixEngine{Initiator: init}
	// default reconnect parameters
	fe.reconnectInitial = 1 * time.Second
	fe.reconnectMax = 30 * time.Second
	return fe
}

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

// SetReconnectParams configures reconnect/backoff policy
func (e *FixEngine) SetReconnectParams(initial, max time.Duration, enable bool) {
	e.reconnectInitial = initial
	e.reconnectMax = max
	e.enableReconnect = enable
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

// startReconnectLoop attempts to re-establish a connection with exponential backoff.
func (e *FixEngine) startReconnectLoop() {
	// ensure only one reconnect loop runs
	e.reconnectMu.Lock()
	if e.reconnectCtx != nil {
		e.reconnectMu.Unlock()
		return
	}
	e.reconnectCtx, e.reconnectCancel = context.WithCancel(context.Background())
	ctx := e.reconnectCtx
	e.reconnectWG.Add(1)
	e.reconnectMu.Unlock()
	go func() {
		defer e.reconnectWG.Done()
		backoff := e.reconnectInitial
		if backoff <= 0 {
			backoff = 1 * time.Second
		}
		max := e.reconnectMax
		if max <= 0 {
			max = 30 * time.Second
		}
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			c, err := e.Initiator.Connect()
			if err == nil {
				// attach new session
				e.Conn = c
				s := NewSession(c, e.Proc)
				_ = e.AttachSession(s)
				// stop reconnect loop
				e.reconnectMu.Lock()
				if e.reconnectCancel != nil {
					e.reconnectCancel()
				}
				e.reconnectCtx = nil
				e.reconnectCancel = nil
				e.reconnectMu.Unlock()
				return
			}
			// wait with jitter
			j := time.Duration(rand.Int63n(int64(backoff/2 + 1)))
			wait := backoff + j
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return
			}
			backoff *= 2
			if backoff > max {
				backoff = max
			}
		}
	}()
}

func (e *FixEngine) stopReconnect() {
	e.reconnectMu.Lock()
	if e.reconnectCancel != nil {
		e.reconnectCancel()
	}
	e.reconnectMu.Unlock()
	// wait for goroutine to finish
	e.reconnectWG.Wait()
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
