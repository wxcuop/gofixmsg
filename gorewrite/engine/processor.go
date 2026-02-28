package engine

import (
	"fmt"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
)

// Processor dispatches by MsgType to registered handlers.
type Processor struct {
	h   map[string]func(*fixmsg.FixMessage) error
	app Application
}

func NewProcessor() *Processor { return &Processor{h: make(map[string]func(*fixmsg.FixMessage) error)} }

// SetApplication sets the application for FromApp callbacks.
func (p *Processor) SetApplication(app Application) {
	p.app = app
}

func (p *Processor) Register(msgType string, fn func(*fixmsg.FixMessage) error) {
	// automatically wrap non-admin handlers to call FromApp
	wrapped := func(m *fixmsg.FixMessage) error {
		if !isAdminMessageType(msgType) && p.app != nil {
			if err := p.app.FromApp(m, ""); err != nil {
				if p.app != nil {
					p.app.OnReject(m, fmt.Sprintf("FromApp rejected: %v", err), "")
				}
				return err
			}
		}
		// call the actual handler
		return fn(m)
	}
	p.h[msgType] = wrapped
}

// RegisterWithFromApp wraps a handler to automatically call FromApp for non-admin messages.
func (p *Processor) RegisterWithFromApp(msgType string, sessionID string, fn func(*fixmsg.FixMessage) error) {
	wrapped := func(m *fixmsg.FixMessage) error {
		// call FromApp for non-admin messages
		if !isAdminMessageType(msgType) && p.app != nil {
			if err := p.app.FromApp(m, sessionID); err != nil {
				if p.app != nil {
					p.app.OnReject(m, fmt.Sprintf("FromApp rejected: %v", err), sessionID)
				}
				return err
			}
		}
		// call the actual handler
		return fn(m)
	}
	p.Register(msgType, wrapped)
}

func (p *Processor) Process(m *fixmsg.FixMessage) error {
	mt, _ := m.Get(35)
	if mt == "" {
		return fmt.Errorf("missing MsgType")
	}
	if fn, ok := p.h[mt]; ok {
		return fn(m)
	}
	return fmt.Errorf("no handler for %s", mt)
}
