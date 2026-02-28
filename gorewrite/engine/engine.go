package engine

import (
"fmt"
"net"

"github.com/wxcuop/pyfixmsg_plus/network"
)

// FixEngine holds components needed for a session.
type FixEngine struct{
Initiator *network.Initiator
Conn net.Conn
}

func NewFixEngine(init *network.Initiator) *FixEngine { return &FixEngine{Initiator: init} }

func (e *FixEngine) Connect() error {
if e.Initiator == nil {
return fmt.Errorf("no initiator configured")
}
c, err := e.Initiator.Connect()
if err != nil { return err }
e.Conn = c
return nil
}

func (e *FixEngine) Close() error {
if e.Conn != nil { return e.Conn.Close() }
return nil
}
