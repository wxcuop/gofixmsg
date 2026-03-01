package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/wxcuop/gofixmsg/engine/session"
	"github.com/wxcuop/gofixmsg/fixmsg"
	"github.com/wxcuop/gofixmsg/heartbeat"
)

// AttachSession attaches an existing Session to the engine and starts monitor and heartbeat sender.
func (e *FixEngine) AttachSession(s *session.Session) error {
	if s == nil {
		return fmt.Errorf("nil session")
	}
	e.Session = s
	if e.Logger != nil {
		s.Logger = e.Logger
	}
	// Call OnCreate callback
	if e.App != nil {
		e.App.OnCreate(e.sessionID)
	}
	// wire inbound message handling to engine and monitor
	s.SetOnMessage(func(m *fixmsg.FixMessage) {
		if err := e.HandleIncoming(m); err != nil {
			e.Logger.Error("failed to handle incoming message", "error", err)
		}
		if e.Monitor != nil {
			e.Monitor.Seen()
		}
	})
	// Chain with any existing OnClose callback (e.g. set by tests before AttachSession)
	existingOnClose := s.OnClose
	// on session close, detach and optionally start reconnect loop in a goroutine
	s.SetOnClose(func() {
		if existingOnClose != nil {
			existingOnClose()
		}
		go func() {
			// ensure we stop monitor/hb sender and clear session
			e.DetachSession()
			// if this engine has an Initiator configured and reconnect enabled, start reconnect loop
			if e.enableReconnect && e.Initiator != nil {
				e.startReconnectLoop()
			}
		}()
	})
	s.Start()
	// ensure monitor present
	if e.Monitor == nil {
		e.Monitor = NewHeartbeatMonitor(e, e.heartbeatInterval, e.heartbeatInterval)
	}
	if e.Monitor != nil {
		e.Monitor.Start(context.Background())
	}
	// heartbeat sender
	if e.hbSender == nil {
		interval := e.heartbeatInterval
		if interval <= 0 {
			interval = 30 * time.Second
		}
		e.hbSender = heartbeat.New(interval, func() {
			// send a minimal heartbeat using configured comp ids
			sender := e.SenderCompID
			target := e.TargetCompID
			if sender == "" {
				sender = "S"
			}
			if target == "" {
				target = "T"
			}
			hb := newHeartbeatMessage(sender, target)
			if err := e.SendMessage(hb); err != nil {
				e.Logger.Error("failed to send heartbeat", "error", err)
			}
		})
		e.hbSender.Start(context.Background())
	}
	return nil
}

// DetachSession stops heartbeat and monitor and detaches session
func (e *FixEngine) DetachSession() {
	if e.hbSender != nil {
		e.hbSender.Stop()
	}
	if e.Monitor != nil {
		e.Monitor.Stop()
	}
	if e.Session != nil {
		e.Session.Stop()
		e.Session = nil
	}
}
