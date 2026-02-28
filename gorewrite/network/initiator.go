package network

import (
"net"
"time"
)

// Initiator dials a remote address.
type Initiator struct {
Addr    string
Timeout time.Duration
}

func NewInitiator(addr string) *Initiator { return &Initiator{Addr: addr, Timeout: 5 * time.Second} }

func (i *Initiator) Connect() (net.Conn, error) {
if i.Timeout == 0 {
i.Timeout = 5 * time.Second
}
return net.DialTimeout("tcp", i.Addr, i.Timeout)
}
