package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/heartbeat"
)

// AttachSession attaches an existing Session to the engine and starts monitor and heartbeat sender.
func (e *FixEngine) AttachSession(s *Session) error {
	if s == nil {
		return fmt.Errorf("nil session")
	}
	e.Session = s
	// wire inbound message handling to engine and monitor
	s.SetOnMessage(func(m *fixmsg.FixMessage) {
		_ = e.HandleIncoming(m)
		if e.Monitor != nil {
			e.Monitor.Seen()
		}
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
			hb := NewHeartbeatMessage(sender, target)
			_ = e.SendMessage(hb)
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
