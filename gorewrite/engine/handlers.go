package engine

import (
	"fmt"
	"strconv"
	"time"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// isAdminMessageType checks if a message is an admin message based on MsgType (tag 35).
func isAdminMessageType(msgType string) bool {
	switch msgType {
	case "A", "5", "0", "1", "2", "3", "4":
		return true
	default:
		return false
	}
}

// Handlers have access to engine components via the context struct.
type HandlerContext struct {
	SM     *state.StateMachine
	Store  store.Store
	Engine *FixEngine
}

// RegisterDefaultHandlers registers Logon/Logout/Resend/TestRequest and basic admin handlers.
func RegisterDefaultHandlers(p *Processor, ctx *HandlerContext) {
	p.Register("A", func(m *fixmsg.FixMessage) error { // Logon
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.App != nil {
			if err := ctx.Engine.App.FromAdmin(m, ctx.Engine.sessionID); err != nil {
				if ctx.Engine.App != nil {
					ctx.Engine.App.OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.sessionID)
				}
				return err
			}
		}
		// If ResetSeqNumFlag=Y reset seq nums to 1
		if v, _ := m.Get(141); v == "Y" {
			if ctx.Engine != nil && ctx.Store != nil {
				_ = ctx.Engine.SeqMgr.SetOutgoing(1)
				ctx.Engine.SeqMgr.SetIncoming(1)
			}
		}
		_, _ = ctx.SM.OnEvent("logon_received")
		// Call OnLogon callback
		if ctx.Engine != nil && ctx.Engine.App != nil {
			ctx.Engine.App.OnLogon(ctx.Engine.sessionID)
		}
		return nil
	})

	p.Register("5", func(m *fixmsg.FixMessage) error { // Logout
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.App != nil {
			if err := ctx.Engine.App.FromAdmin(m, ctx.Engine.sessionID); err != nil {
				if ctx.Engine.App != nil {
					ctx.Engine.App.OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.sessionID)
				}
				return err
			}
		}
		// reply with Logout if we have an engine
		if ctx.Engine != nil {
			sender, _ := m.Get(56) // note: incoming Target becomes our Sender
			target, _ := m.Get(49)
			out := NewLogoutMessage(sender, target)
			// Call ToAdmin callback
			if ctx.Engine.App != nil {
				if err := ctx.Engine.App.ToAdmin(out, ctx.Engine.sessionID); err != nil {
					if ctx.Engine.App != nil {
						ctx.Engine.App.OnReject(out, fmt.Sprintf("ToAdmin rejected: %v", err), ctx.Engine.sessionID)
					}
					return err
				}
			}
			_ = ctx.Engine.SendMessage(out)
		}
		_, _ = ctx.SM.OnEvent("logout_received")
		// Call OnLogout callback
		if ctx.Engine != nil && ctx.Engine.App != nil {
			ctx.Engine.App.OnLogout(ctx.Engine.sessionID)
		}
		return nil
	})

	p.Register("2", func(m *fixmsg.FixMessage) error { // ResendRequest
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.App != nil {
			if err := ctx.Engine.App.FromAdmin(m, ctx.Engine.sessionID); err != nil {
				if ctx.Engine.App != nil {
					ctx.Engine.App.OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.sessionID)
				}
				return err
			}
		}
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
				// replay stored message bytes — enqueue via SessionSend if available
				if ctx.Engine != nil {
					_ = ctx.Engine.SessionSend(msg.Body)
				}
			} else {
				// missing: send SequenceReset as GapFill to advance
				sr := NewSequenceResetMessage(target, sender, seq+1, true)
				// Call ToAdmin callback
				if ctx.Engine.App != nil {
					if err := ctx.Engine.App.ToAdmin(sr, ctx.Engine.sessionID); err != nil {
						if ctx.Engine.App != nil {
							ctx.Engine.App.OnReject(sr, fmt.Sprintf("ToAdmin rejected: %v", err), ctx.Engine.sessionID)
						}
						continue
					}
				}
				if ctx.Engine != nil {
					_ = ctx.Engine.SendMessage(sr)
				}
			}
		}
		return nil
	})

	p.Register("4", func(m *fixmsg.FixMessage) error { // SequenceReset
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.App != nil {
			if err := ctx.Engine.App.FromAdmin(m, ctx.Engine.sessionID); err != nil {
				if ctx.Engine.App != nil {
					ctx.Engine.App.OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.sessionID)
				}
				return err
			}
		}
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
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.App != nil {
			if err := ctx.Engine.App.FromAdmin(m, ctx.Engine.sessionID); err != nil {
				if ctx.Engine.App != nil {
					ctx.Engine.App.OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.sessionID)
				}
				return err
			}
		}
		// reply with Heartbeat containing TestReqID
		trid, _ := m.Get(112)
		sender, _ := m.Get(56)
		target, _ := m.Get(49)
		hb := NewHeartbeatMessage(sender, target)
		if trid != "" {
			hb.Set(112, trid)
			// SendMessage will call SetLenAndChecksum via ToWire
		}
		// Call ToAdmin callback
		if ctx.Engine != nil && ctx.Engine.App != nil {
			if err := ctx.Engine.App.ToAdmin(hb, ctx.Engine.sessionID); err != nil {
				if ctx.Engine.App != nil {
					ctx.Engine.App.OnReject(hb, fmt.Sprintf("ToAdmin rejected: %v", err), ctx.Engine.sessionID)
				}
				return err
			}
		}
		if ctx.Engine != nil {
			_ = ctx.Engine.SendMessage(hb)
		}
		return nil
	})

	p.Register("0", func(m *fixmsg.FixMessage) error { // Heartbeat
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.App != nil {
			if err := ctx.Engine.App.FromAdmin(m, ctx.Engine.sessionID); err != nil {
				if ctx.Engine.App != nil {
					ctx.Engine.App.OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.sessionID)
				}
				return err
			}
		}
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

// simple send helper (backwards compatible)
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
	if msgType != "" && !isAdminMessageType(msgType) {
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
