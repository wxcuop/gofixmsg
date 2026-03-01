package engine_test

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/engine"
	"github.com/wxcuop/gofixmsg/network"
)

func TestEngineConnect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	addr := ln.Addr().String()
	// accept in background
	go func() {
		c, _ := ln.Accept()
		defer c.Close()
		// wait a bit
		time.Sleep(10 * time.Millisecond)
	}()
	init := network.NewInitiator(addr)
	e := engine.NewFixEngine(init)
	require.NoError(t, e.Connect())
	defer e.Close()
}
