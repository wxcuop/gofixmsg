package engine

import (
	"fmt"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
)

// Processor dispatches by MsgType to registered handlers.
type Processor struct {
	h map[string]func(*fixmsg.FixMessage) error
}

func NewProcessor() *Processor { return &Processor{h: make(map[string]func(*fixmsg.FixMessage) error)} }

func (p *Processor) Register(msgType string, fn func(*fixmsg.FixMessage) error) {
	p.h[msgType] = fn
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
