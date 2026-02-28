package network_test

import (
"io"
"net"
"testing"
"time"

"github.com/stretchr/testify/require"
"github.com/wxcuop/pyfixmsg_plus/network"
)

func TestInitiatorAndAcceptor(t *testing.T) {
acc := network.NewAcceptor("127.0.0.1:0")
var srvAddr string
received := make(chan string, 1)
reqHandler := func(c net.Conn) {
defer c.Close()
buf := make([]byte, 64)
n, _ := c.Read(buf)
received <- string(buf[:n])
}
require.NoError(t, acc.Start(reqHandler))
// give listener a moment
time.Sleep(10 * time.Millisecond)

srvAddr = acc.AddrString()
init := network.NewInitiator(srvAddr)
c, err := init.Connect()
require.NoError(t, err)
defer c.Close()
_, err = io.WriteString(c, "hello")
require.NoError(t, err)
select {
case msg := <-received:
require.Equal(t, "hello", msg)
case <-time.After(500 * time.Millisecond):
require.Fail(t, "timeout waiting for message")
}
_ = acc.Stop()
}
