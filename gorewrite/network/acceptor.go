package network

import (
"crypto/tls"
"fmt"
"net"
)

// Acceptor listens for incoming connections and dispatches them to handler.
type Acceptor struct {
Addr string
ln   net.Listener
TLSConfig *tls.Config
}

func NewAcceptor(addr string) *Acceptor { return &Acceptor{Addr: addr} }

func (a *Acceptor) WithTLS(cfg *tls.Config) *Acceptor { a.TLSConfig = cfg; return a }

func (a *Acceptor) Start(handler func(net.Conn)) error {
ln, err := net.Listen("tcp", a.Addr)
if err != nil {
return fmt.Errorf("acceptor: listen: %w", err)
}
a.ln = ln
if a.TLSConfig != nil {
a.ln = tls.NewListener(ln, a.TLSConfig)
}
go func() {
for {
c, err := a.ln.Accept()
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
