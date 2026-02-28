package engine

import (
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
)

// Application defines callbacks for FIX session lifecycle and message events.
// Applications implement this interface to hook into session management and message processing.
type Application interface {
	// OnCreate is called when a session is created (before logon).
	OnCreate(sessionID string)

	// OnLogon is called after a successful logon.
	OnLogon(sessionID string)

	// OnLogout is called after a logout.
	OnLogout(sessionID string)

	// ToAdmin is called just before an admin message (35=A/5/0/1/2/3/4) is sent.
	// The application may modify the message or return error to reject sending.
	ToAdmin(msg *fixmsg.FixMessage, sessionID string) error

	// FromAdmin is called after an admin message is received and parsed.
	// The application may reject processing by returning error.
	FromAdmin(msg *fixmsg.FixMessage, sessionID string) error

	// ToApp is called just before an application message (not admin) is sent.
	// The application may modify the message or return error to reject sending.
	ToApp(msg *fixmsg.FixMessage, sessionID string) error

	// FromApp is called after an application message is received and parsed.
	// The application may reject processing by returning error.
	FromApp(msg *fixmsg.FixMessage, sessionID string) error

	// OnReject is called when a message is rejected (e.g., invalid seqnum, parsing error).
	OnReject(msg *fixmsg.FixMessage, reason string, sessionID string)
}

// NoOpApplication is a default implementation of Application that does nothing.
// Use this when you don't need application callbacks.
type NoOpApplication struct{}

func (n *NoOpApplication) OnCreate(sessionID string)                                                  {}
func (n *NoOpApplication) OnLogon(sessionID string)                                                   {}
func (n *NoOpApplication) OnLogout(sessionID string)                                                  {}
func (n *NoOpApplication) ToAdmin(msg *fixmsg.FixMessage, sessionID string) error                     { return nil }
func (n *NoOpApplication) FromAdmin(msg *fixmsg.FixMessage, sessionID string) error                  { return nil }
func (n *NoOpApplication) ToApp(msg *fixmsg.FixMessage, sessionID string) error                      { return nil }
func (n *NoOpApplication) FromApp(msg *fixmsg.FixMessage, sessionID string) error                    { return nil }
func (n *NoOpApplication) OnReject(msg *fixmsg.FixMessage, reason string, sessionID string)          {}

// Compile-time check that NoOpApplication implements Application.
var _ Application = (*NoOpApplication)(nil)
