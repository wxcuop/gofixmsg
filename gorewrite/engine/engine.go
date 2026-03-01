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
	"github.com/wxcuop/pyfixmsg_plus/engine/handler"
	"github.com/wxcuop/pyfixmsg_plus/engine/session"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/heartbeat"
	"github.com/wxcuop/pyfixmsg_plus/network"
	"github.com/wxcuop/pyfixmsg_plus/scheduler"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// FixEngine holds components needed for a session.
type FixEngine struct {
	Initiator *network.Initiator
	Conn      net.Conn
	Session   *session.Session
	Proc      *handler.Processor
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
	// application callbacks
	App Application
	// session ID for callbacks
	sessionID string
	// scheduler for config-driven actions
	Scheduler *scheduler.RuntimeScheduler
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
	// default to NoOpApplication
	fe.App = &NoOpApplication{}
	return fe
}

// SetupComponents wires state machine and store into the engine and registers handlers.
// Also loads TLS configuration from config if ssl_* keys are present.
func (e *FixEngine) SetupComponents(sm *state.StateMachine, st store.Store) {
	e.SM = sm
	e.Store = st
	// Only create a new processor if one hasn't been set already
	if e.Proc == nil {
		e.Proc = handler.NewProcessor()
	}
	// set application on processor for FromApp callbacks
	e.Proc.SetApplication(e.App)
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
		// Load TLS configuration if ssl_* keys are present
		certFile := mgr.Get("", "ssl_cert_file")
		keyFile := mgr.Get("", "ssl_key_file")
		caFile := mgr.Get("", "ssl_ca_file")
		if certFile != "" {
			tlsCfg, err := network.LoadTLSConfig(certFile, keyFile, caFile)
			if err != nil {
				fmt.Printf("warning: failed to load TLS config: %v\n", err)
			} else if tlsCfg != nil {
				// Apply TLS config to Initiator if present
				if e.Initiator != nil {
					e.Initiator.WithTLS(tlsCfg)
				}
				// Note: Acceptor TLS is set separately via AcceptorWithTLS() if needed
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
	ctx := &handler.HandlerContext{SM: sm, Store: st, Engine: e}
	handler.RegisterDefaultHandlers(e.Proc, ctx)
	
	// Initialize scheduler and wire actions
	e.Scheduler = scheduler.NewRuntimeScheduler()
	e.wireSchedulerActions()
	
	// Load scheduler configuration
	if mgr != nil {
		if err := e.Scheduler.Load(mgr); err != nil {
			fmt.Printf("warning: failed to load scheduler config: %v\n", err)
		}
	}
	// Note: OnCreate callback should be called after session is attached, not here
}

// SetReconnectParams configures reconnect/backoff policy
func (e *FixEngine) SetReconnectParams(initial, max time.Duration, enable bool) {
	e.reconnectInitial = initial
	e.reconnectMax = max
	e.enableReconnect = enable
}

// SetApplication sets the application callback handler.
func (e *FixEngine) SetApplication(app Application) {
	if app != nil {
		e.App = app
	}
}

// SetSessionID sets the session ID for application callbacks.
func (e *FixEngine) SetSessionID(sid string) {
	e.sessionID = sid
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
	s := session.NewSession(c, e.Proc)
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
				s := session.NewSession(c, e.Proc)
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

// SendMessage sends a FIX message with automatic field setup (CompIDs, SeqNum, SendingTime).
func (e *FixEngine) SendMessage(m *fixmsg.FixMessage) error {
	// ensure Sender/Target CompIDs are present
	if !m.Contains(fixmsg.TagSenderCompID) {
		sender := e.SenderCompID
		if sender == "" {
			sender = "S"
		}
		m.Set(fixmsg.TagSenderCompID, sender)
	}
	if !m.Contains(fixmsg.TagTargetCompID) {
		target := e.TargetCompID
		if target == "" {
			target = "T"
		}
		m.Set(fixmsg.TagTargetCompID, target)
	}

	// stamp MsgSeqNum if missing
	if !m.Contains(fixmsg.TagMsgSeqNum) {
		seq := 1
		if e.SeqMgr != nil {
			if n, err := e.SeqMgr.IncrementOutgoing(); err == nil {
				seq = n
			}
		}
		m.Set(fixmsg.TagMsgSeqNum, strconv.Itoa(seq))
	}

	// stamp SendingTime if missing
	if !m.Contains(fixmsg.TagSendingTime) {
		m.Set(fixmsg.TagSendingTime, time.Now().UTC().Format("20060102-15:04:05.000"))
	}

	// Call ToApp callback for non-admin messages (admin callbacks are called by handlers/callers)
	msgType, _ := m.Get(35)
	if msgType != "" && !handler.IsAdminMessageType(msgType) {
		if e.App != nil {
			if err := e.App.ToApp(m, e.sessionID); err != nil {
				if e.App != nil {
					e.App.OnReject(m, fmt.Sprintf("ToApp rejected: %v", err), e.sessionID)
				}
				return err
			}
		}
	}

	b, err := m.ToWire()
	if err != nil {
		return err
	}
	// prefer SessionSend when session is present
	if e.Session != nil {
		return e.Session.Send(b)
	}
	if e.Conn == nil {
		return fmt.Errorf("no connection")
	}
	_, err = e.Conn.Write(b)
	return err
}

// EngineI interface implementations

// GetApp returns the application callback handler.
func (e *FixEngine) GetApp() handler.Application {
	// engine.Application and handler.Application have the same interface definition
	// so this is safe even though they're technically different types
	return e.App
}

// GetSessionID returns the session ID.
func (e *FixEngine) GetSessionID() string {
	return e.sessionID
}

// GetSeqMgr returns the sequence manager.
func (e *FixEngine) GetSeqMgr() handler.SeqMgrI {
	return e.SeqMgr
}

// Helper functions for creating FIX messages

// newHeartbeatMessage creates a minimal FIX heartbeat message (MsgType=0).
func newHeartbeatMessage(sender, target string) *fixmsg.FixMessage {
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		8:  "FIX.4.4",
		35: "0",
		49: sender,
		56: target,
	})
	m.SetLenAndChecksum()
	return m
}

// newSequenceResetMessage creates a SequenceReset message (MsgType=4).
func newSequenceResetMessage(sender, target string, newSeq int, gapFill bool) *fixmsg.FixMessage {
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		8:  "FIX.4.4",
		35: "4",
		49: sender,
		56: target,
		36: strconv.Itoa(newSeq),
	})
	if gapFill {
		m.Set(123, "Y")
	}
	m.SetLenAndChecksum()
	return m
}

// wireSchedulerActions registers scheduler action handlers to engine methods
func (e *FixEngine) wireSchedulerActions() {
	if e.Scheduler == nil {
		return
	}

	// start: Connect to FIX peer
	e.Scheduler.RegisterAction("start", func() {
		if err := e.Connect(); err != nil {
			fmt.Printf("scheduler: error connecting: %v\n", err)
		}
	})

	// stop/logout: Logout and close connection
	e.Scheduler.RegisterAction("stop/logout", func() {
		if err := e.Close(); err != nil {
			fmt.Printf("scheduler: error closing: %v\n", err)
		}
	})

	// reset: Reset sequence numbers
	e.Scheduler.RegisterAction("reset", func() {
		if e.SeqMgr != nil {
			e.SeqMgr.SetIncoming(1)
			if err := e.SeqMgr.SetOutgoing(1); err != nil {
				fmt.Printf("scheduler: error resetting sequence: %v\n", err)
			}
		}
	})

	// reset_start: Reset sequence and connect
	e.Scheduler.RegisterAction("reset_start", func() {
		if e.SeqMgr != nil {
			e.SeqMgr.SetIncoming(1)
			if err := e.SeqMgr.SetOutgoing(1); err != nil {
				fmt.Printf("scheduler: error resetting sequence: %v\n", err)
			}
		}
		if err := e.Connect(); err != nil {
			fmt.Printf("scheduler: error connecting after reset: %v\n", err)
		}
	})
}

// StartScheduler begins the runtime scheduler
func (e *FixEngine) StartScheduler() {
	if e.Scheduler != nil {
		e.Scheduler.Start()
	}
}

// StopScheduler halts the runtime scheduler
func (e *FixEngine) StopScheduler() {
	if e.Scheduler != nil {
		e.Scheduler.Stop()
	}
}
