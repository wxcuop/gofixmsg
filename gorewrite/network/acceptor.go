package network

import (
"net"
"fmt"
)

// Acceptor listens for incoming connections and dispatches them to handler.
type Acceptor struct {
Addr string
ln   net.Listener
}

func NewAcceptor(addr string) *Acceptor { return &Acceptor{Addr: addr} }

func (a *Acceptor) Start(handler func(net.Conn)) error {
ln, err := net.Listen("tcp", a.Addr)
if err != nil {
return fmt.Errorf("acceptor: listen: %w", err)
}
a.ln = ln
go func() {
for {
c, err := ln.Accept()
if err != nil {
return
}
go handler(c)
}
}()
return nil
}

func (a *Acceptor) AddrString() string { return a.ln.Addr().String() }

func (a *Acceptor) Stop() error {
if a.ln != nil {
return a.ln.Close()
}
return nil
}
