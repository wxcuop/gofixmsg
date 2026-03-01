package handler

import (
	"fmt"
	"log/slog"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg/spec"
)

// Processor dispatches by MsgType to registered handlers.
type Processor struct {
	h              map[string]func(*fixmsg.FixMessage) error
	app            Application
	spec           *spec.FixSpec
	validateFn     func(*fixmsg.FixMessage, *spec.FixSpec) error
	logger         *slog.Logger
	getSessionIDFn func() string // callback to get current session ID for app callbacks
}

func (p *Processor) getLogger() *slog.Logger {
	if p.logger == nil {
		return slog.Default()
	}
	return p.logger
}

func NewProcessor() *Processor {
	return &Processor{
		h:      make(map[string]func(*fixmsg.FixMessage) error),
		logger: slog.Default(),
	}
}

// SetApplication sets the application for FromApp callbacks.
func (p *Processor) SetApplication(app Application) {
	p.app = app
}

// SetGetSessionIDFn sets the function to retrieve the current session ID for app callbacks.
func (p *Processor) SetGetSessionIDFn(fn func() string) {
	p.getSessionIDFn = fn
}

// SetSpec sets the FIX dictionary spec for validation.
func (p *Processor) SetSpec(s *spec.FixSpec) {
	p.spec = s
}

// SetValidateFunc sets the validation function
func (p *Processor) SetValidateFunc(fn func(*fixmsg.FixMessage, *spec.FixSpec) error) {
	p.validateFn = fn
}

// SetLogger sets the logger for the processor.
func (p *Processor) SetLogger(l *slog.Logger) {
	if l != nil {
		p.logger = l
	}
}

func (p *Processor) Register(msgType string, fn func(*fixmsg.FixMessage) error) {
	// automatically wrap non-admin handlers to call FromApp
	wrapped := func(m *fixmsg.FixMessage) error {
		if !IsAdminMessageType(msgType) && p.app != nil {
			sessionID := ""
			if p.getSessionIDFn != nil {
				sessionID = p.getSessionIDFn()
			}
			if err := p.app.FromApp(m, sessionID); err != nil {
				if p.app != nil {
					p.app.OnReject(m, fmt.Sprintf("FromApp rejected: %v", err), sessionID)
				}
				return err
			}
			// Call OnMessage after successful FromApp
			if p.app != nil {
				p.app.OnMessage(m, sessionID)
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
		if !IsAdminMessageType(msgType) && p.app != nil {
			if err := p.app.FromApp(m, sessionID); err != nil {
				if p.app != nil {
					p.app.OnReject(m, fmt.Sprintf("FromApp rejected: %v", err), sessionID)
				}
				return err
			}
			// Call OnMessage after successful FromApp
			if p.app != nil {
				p.app.OnMessage(m, sessionID)
			}
		}
		// call the actual handler
		return fn(m)
	}
	p.Register(msgType, wrapped)
}

func (p *Processor) Process(m *fixmsg.FixMessage) error {
	// Validate message structure and dictionary if provided
	var validateErr error
	if p.validateFn != nil {
		validateErr = p.validateFn(m, p.spec)
	}
	if validateErr != nil {
		p.getLogger().Error("validation failed", "msgType", m.FixFragment[35], "error", validateErr)
		if p.app != nil {
			p.app.OnReject(m, fmt.Sprintf("Validation failed: %v", validateErr), "")
		}
		return fmt.Errorf("validation failed: %w", validateErr)
	}

	mt, _ := m.Get(35)
	if mt == "" {
		return fmt.Errorf("missing MsgType")
	}
	if fn, ok := p.h[mt]; ok {
		return fn(m)
	}
	return fmt.Errorf("no handler for %s", mt)
}
