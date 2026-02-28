package engine

import (
	"fmt"
	"strconv"

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

// RegisterDefaultHandlers registers Logon/Logout/Resend/TestRequest and basic admin handlers.
func RegisterDefaultHandlers(p *Processor, ctx *HandlerContext) {
	p.Register("A", func(m *fixmsg.FixMessage) error { // Logon
		// If ResetSeqNumFlag=Y reset seq nums to 1
		if v, _ := m.Get(141); v == "Y" {
			if ctx.Engine != nil && ctx.Store != nil {
				_ = ctx.Engine.SeqMgr.SetOutgoing(1)
				ctx.Engine.SeqMgr.SetIncoming(1)
			}
		}
		ctx.SM.OnEvent("logon_received")
		return nil
	})

	p.Register("5", func(m *fixmsg.FixMessage) error { // Logout
		// reply with Logout if we have an engine
		if ctx.Engine != nil {
			sender, _ := m.Get(56) // note: incoming Target becomes our Sender
			target, _ := m.Get(49)
			out := NewLogoutMessage(sender, target)
			_ = ctx.Engine.SendMessage(out)
		}
		ctx.SM.OnEvent("logout_received")
		return nil
	})

	p.Register("2", func(m *fixmsg.FixMessage) error { // ResendRequest
		b, _ := m.Get(7)
		e, _ := m.Get(16)
		if b == "" {
			return fmt.Errorf("ResendRequest missing BeginSeqNo")
		}
		bi, _ := strconv.Atoi(b)
		ei := 0
		if e != "" {
			ei, _ = strconv.Atoi(e)
		}
		if ctx == nil || ctx.Store == nil || ctx.Engine == nil {
			return nil
		}
		// iterate requested seqs and try to fetch from store; if missing, send SequenceReset GapFill to skip
		sender, _ := m.Get(56)
		target, _ := m.Get(49)
		for seq := bi; seq <= ei; seq++ {
			msg, err := ctx.Store.GetMessage("FIX.4.4", target, sender, seq)
			if err == nil && msg != nil {
				// replay stored message bytes
				if ctx.Engine != nil && ctx.Engine.Conn != nil {
					_, _ = ctx.Engine.Conn.Write(msg.Body)
				}
			} else {
				// missing: send SequenceReset as GapFill to advance
				sr := NewSequenceResetMessage(target, sender, seq+1, true)
				if ctx.Engine != nil {
					_ = ctx.Engine.SendMessage(sr)
				}
			}
		}
		return nil
	})

	p.Register("4", func(m *fixmsg.FixMessage) error { // SequenceReset
		// NewSeqNo tag 36
		if v, _ := m.Get(36); v != "" {
			n, _ := strconv.Atoi(v)
			if ctx.Engine != nil && ctx.Engine.SeqMgr != nil {
				_ = ctx.Engine.SeqMgr.SetOutgoing(n - 1)
			}
		}
		return nil
	})

	p.Register("1", func(m *fixmsg.FixMessage) error { // TestRequest
		// reply with Heartbeat containing TestReqID
		trid, _ := m.Get(112)
		sender, _ := m.Get(56)
		target, _ := m.Get(49)
		hb := NewHeartbeatMessage(sender, target)
		if trid != "" {
			hb.Set(112, trid)
			hb.SetLenAndChecksum()
		}
		if ctx.Engine != nil {
			_ = ctx.Engine.SendMessage(hb)
		}
		return nil
	})

	p.Register("0", func(m *fixmsg.FixMessage) error { // Heartbeat
		// no-op but could validate TestReqID presence
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

func NewSequenceResetMessage(sender, target string, newSeq int, gapFill bool) *fixmsg.FixMessage {
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
