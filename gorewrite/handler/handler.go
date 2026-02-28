package handler

import "fmt"

// MessageHandler handles a FIX message by type.
type MessageHandler interface {
Handle(msgType string, body []byte) error
}

// Processor dispatches messages to registered handlers.
type Processor struct{
h map[string]MessageHandler
}

func NewProcessor() *Processor { return &Processor{h: make(map[string]MessageHandler)} }

func (p *Processor) Register(msgType string, h MessageHandler) {
p.h[msgType] = h
}

func (p *Processor) Process(msgType string, body []byte) error {
if h, ok := p.h[msgType]; ok {
return h.Handle(msgType, body)
}
return fmt.Errorf("no handler for %s", msgType)
}
