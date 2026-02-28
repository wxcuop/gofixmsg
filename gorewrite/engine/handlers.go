package engine

import (
	"fmt"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// Handlers have access to engine components via the context struct.
type HandlerContext struct {
	SM     *state.StateMachine
	Store  store.Store
	Engine *FixEngine
}

// RegisterDefaultHandlers registers simple Logon/Logout/Resend handlers.
func RegisterDefaultHandlers(p *Processor, ctx *HandlerContext) {
	p.Register("A", func(m *fixmsg.FixMessage) error { // Logon
		// on logon received, transition to Active
		ctx.SM.OnEvent("logon_received")
		return nil
	})

	p.Register("5", func(m *fixmsg.FixMessage) error { // Logout
		ctx.SM.OnEvent("logout_received")
		return nil
	})

	p.Register("2", func(m *fixmsg.FixMessage) error { // ResendRequest
		// naive behavior: extract BeginSeqNo (7) and EndSeqNo (16) and log/store
		b, _ := m.Get(7)
		e, _ := m.Get(16)
		_ = b
		_ = e
		// For now just acknowledge by returning nil
		return nil
	})

	p.Register("0", func(m *fixmsg.FixMessage) error { // Heartbeat
		// no-op
		return nil
	})
}

// Helper to build a minimal FixMessage for outgoing messages
func NewLogonMessage(sender, target string) *fixmsg.FixMessage {
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		8:  "FIX.4.4",
		35: "A",
		49: sender,
		56: target,
	})
	m.SetLenAndChecksum()
	return m
}

func NewLogoutMessage(sender, target string) *fixmsg.FixMessage {
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		8:  "FIX.4.4",
		35: "5",
		49: sender,
		56: target,
	})
	m.SetLenAndChecksum()
	return m
}

func NewHeartbeatMessage(sender, target string) *fixmsg.FixMessage {
	m := fixmsg.NewFixMessageFromMap(map[int]string{
		8:  "FIX.4.4",
		35: "0",
		49: sender,
		56: target,
	})
	m.SetLenAndChecksum()
	return m
}

// simple send helper
func (e *FixEngine) SendMessage(m *fixmsg.FixMessage) error {
	if e.Conn == nil {
		return fmt.Errorf("no connection")
	}
	b, err := m.ToWire()
	if err != nil {
		return err
	}
	_, err = e.Conn.Write(b)
	return err
}
