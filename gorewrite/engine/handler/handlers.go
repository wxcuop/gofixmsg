package handler

import (
	"fmt"
	"strconv"
	"time"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/state"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

// Application defines callbacks for FIX session lifecycle and message events.
// This is copied here to avoid circular imports between engine and handler packages.
type Application interface {
	OnCreate(sessionID string)
	OnLogon(sessionID string)
	OnLogout(sessionID string)
	ToAdmin(msg *fixmsg.FixMessage, sessionID string) error
	FromAdmin(msg *fixmsg.FixMessage, sessionID string) error
	ToApp(msg *fixmsg.FixMessage, sessionID string) error
	FromApp(msg *fixmsg.FixMessage, sessionID string) error
	OnReject(msg *fixmsg.FixMessage, reason string, sessionID string)
}

// EngineI defines the interface for engine operations that handlers need.
// This avoids circular dependencies between engine and handler packages.
type EngineI interface {
	SendMessage(*fixmsg.FixMessage) error
	SessionSend([]byte) error
	GetApp() Application
	GetSessionID() string
	GetSeqMgr() SeqMgrI
}

// SeqMgrI defines the interface for sequence manager.
type SeqMgrI interface {
	Incoming() int
	Outgoing() int
	SetIncoming(int)
	SetOutgoing(int) error
	IncrementIncoming() int
	IncrementOutgoing() (int, error)
}

// IsAdminMessageType checks if a message is an admin message based on MsgType (tag 35).
func IsAdminMessageType(msgType string) bool {
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
	Engine EngineI
}

// RegisterDefaultHandlers registers Logon/Logout/Resend/TestRequest and basic admin handlers.
func RegisterDefaultHandlers(p *Processor, ctx *HandlerContext) {
	p.Register("A", func(m *fixmsg.FixMessage) error { // Logon
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
			if err := ctx.Engine.GetApp().FromAdmin(m, ctx.Engine.GetSessionID()); err != nil {
				if ctx.Engine.GetApp() != nil {
					ctx.Engine.GetApp().OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.GetSessionID())
				}
				return err
			}
		}
		// If ResetSeqNumFlag=Y reset seq nums to 1
		if v, _ := m.Get(141); v == "Y" {
			if ctx.Engine != nil && ctx.Store != nil {
				_ = ctx.Engine.GetSeqMgr().SetOutgoing(1)
				ctx.Engine.GetSeqMgr().SetIncoming(1)
			}
		}
		_, _ = ctx.SM.OnEvent("logon_received")
		// Call OnLogon callback
		if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
			ctx.Engine.GetApp().OnLogon(ctx.Engine.GetSessionID())
		}
		return nil
	})

	p.Register("5", func(m *fixmsg.FixMessage) error { // Logout
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
			if err := ctx.Engine.GetApp().FromAdmin(m, ctx.Engine.GetSessionID()); err != nil {
				if ctx.Engine.GetApp() != nil {
					ctx.Engine.GetApp().OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.GetSessionID())
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
			if ctx.Engine.GetApp() != nil {
				if err := ctx.Engine.GetApp().ToAdmin(out, ctx.Engine.GetSessionID()); err != nil {
					if ctx.Engine.GetApp() != nil {
						ctx.Engine.GetApp().OnReject(out, fmt.Sprintf("ToAdmin rejected: %v", err), ctx.Engine.GetSessionID())
					}
					return err
				}
			}
			_ = ctx.Engine.SendMessage(out)
		}
		_, _ = ctx.SM.OnEvent("logout_received")
		// Call OnLogout callback
		if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
			ctx.Engine.GetApp().OnLogout(ctx.Engine.GetSessionID())
		}
		return nil
	})

	p.Register("2", func(m *fixmsg.FixMessage) error { // ResendRequest
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
			if err := ctx.Engine.GetApp().FromAdmin(m, ctx.Engine.GetSessionID()); err != nil {
				if ctx.Engine.GetApp() != nil {
					ctx.Engine.GetApp().OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.GetSessionID())
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
		if ei == 0 {
			// FIX standard: 0 means infinity (all messages subsequent to BeginSeqNo)
			if ctx.Engine != nil && ctx.Engine.GetSeqMgr() != nil {
				ei = ctx.Engine.GetSeqMgr().Outgoing()
			}
		}
		if ctx == nil || ctx.Store == nil || ctx.Engine == nil {
			return nil
		}
		// iterate requested seqs and try to fetch from store; if missing, send SequenceReset GapFill to skip
		sender, _ := m.Get(56)
		target, _ := m.Get(49)
		for seq := bi; seq <= ei; seq++ {
			msg, err := ctx.Store.GetMessage("FIX.4.4", sender, target, seq)
			if err == nil && msg != nil {
				// Parse stored message to add PossDupFlag and OrigSendingTime
				parsedMsg := fixmsg.NewFixMessage()
				parseErr := parsedMsg.LoadFix(msg.Body)
				if parseErr == nil {
					// Don't replay certain admin messages (e.g., Logon, Logout, ResendRequest, Heartbeat, TestRequest, SequenceReset)
					msgType, _ := parsedMsg.Get(35)
					if msgType == "A" || msgType == "5" || msgType == "2" || msgType == "0" || msgType == "1" || msgType == "4" {
						// Admin messages should be replaced by a SequenceReset GapFill
						sr := NewSequenceResetMessage(target, sender, seq+1, true)
						// Ensure it uses the original sequence number
						sr.Set(34, strconv.Itoa(seq))
						sr.Set(43, "Y")
						if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
							if err := ctx.Engine.GetApp().ToAdmin(sr, ctx.Engine.GetSessionID()); err != nil {
								ctx.Engine.GetApp().OnReject(sr, fmt.Sprintf("ToAdmin rejected: %v", err), ctx.Engine.GetSessionID())
								continue
							}
						}
						if ctx.Engine != nil {
							b, _ := sr.ToWire()
							_ = ctx.Engine.SessionSend(b)
						}
						continue
					}

					// Set PossDupFlag=Y
					parsedMsg.Set(43, "Y")
					// Copy SendingTime to OrigSendingTime if missing
					if origTime, ok := parsedMsg.Get(52); ok && !parsedMsg.Contains(122) {
						parsedMsg.Set(122, origTime)
					}
					// Update SendingTime to now
					parsedMsg.Set(52, time.Now().UTC().Format("20060102-15:04:05.000"))

					// Re-serialize and send
					if ctx.Engine != nil {
						b, err := parsedMsg.ToWire()
						if err == nil {
							_ = ctx.Engine.SessionSend(b)
						}
					}
				} else {
					fmt.Println("Parse error:", parseErr)
					// Fallback to GapFill if parsing fails
					sr := NewSequenceResetMessage(target, sender, seq+1, true)
					sr.Set(34, strconv.Itoa(seq))
					sr.Set(43, "Y")
					if ctx.Engine != nil {
						b, _ := sr.ToWire()
						_ = ctx.Engine.SessionSend(b)
					}
				}
			} else {
				// missing: send SequenceReset as GapFill to advance
				sr := NewSequenceResetMessage(target, sender, seq+1, true)
				// SequenceReset (GapFill) uses the original sequence number we are skipping
				sr.Set(34, strconv.Itoa(seq))
				sr.Set(43, "Y")
				// Call ToAdmin callback
				if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
					if err := ctx.Engine.GetApp().ToAdmin(sr, ctx.Engine.GetSessionID()); err != nil {
						if ctx.Engine.GetApp() != nil {
							ctx.Engine.GetApp().OnReject(sr, fmt.Sprintf("ToAdmin rejected: %v", err), ctx.Engine.GetSessionID())
						}
						continue
					}
				}
				if ctx.Engine != nil {
					b, _ := sr.ToWire()
					_ = ctx.Engine.SessionSend(b)
				}
			}
		}

		return nil
	})

	p.Register("4", func(m *fixmsg.FixMessage) error { // SequenceReset
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
			if err := ctx.Engine.GetApp().FromAdmin(m, ctx.Engine.GetSessionID()); err != nil {
				if ctx.Engine.GetApp() != nil {
					ctx.Engine.GetApp().OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.GetSessionID())
				}
				return err
			}
		}
		// NewSeqNo tag 36
		if v, _ := m.Get(36); v != "" {
			n, _ := strconv.Atoi(v)
			if ctx.Engine != nil && ctx.Engine.GetSeqMgr() != nil {
				_ = ctx.Engine.GetSeqMgr().SetOutgoing(n - 1)
			}
		}
		return nil
	})

	p.Register("1", func(m *fixmsg.FixMessage) error { // TestRequest
		// Call FromAdmin callback
		if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
			if err := ctx.Engine.GetApp().FromAdmin(m, ctx.Engine.GetSessionID()); err != nil {
				if ctx.Engine.GetApp() != nil {
					ctx.Engine.GetApp().OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.GetSessionID())
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
		if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
			if err := ctx.Engine.GetApp().ToAdmin(hb, ctx.Engine.GetSessionID()); err != nil {
				if ctx.Engine.GetApp() != nil {
					ctx.Engine.GetApp().OnReject(hb, fmt.Sprintf("ToAdmin rejected: %v", err), ctx.Engine.GetSessionID())
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
		if ctx.Engine != nil && ctx.Engine.GetApp() != nil {
			if err := ctx.Engine.GetApp().FromAdmin(m, ctx.Engine.GetSessionID()); err != nil {
				if ctx.Engine.GetApp() != nil {
					ctx.Engine.GetApp().OnReject(m, fmt.Sprintf("FromAdmin rejected: %v", err), ctx.Engine.GetSessionID())
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

// NewHeartbeatMessage creates a minimal FIX heartbeat message (MsgType=0).
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

// NewSequenceResetMessage creates a SequenceReset message (MsgType=4).
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
