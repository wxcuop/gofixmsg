package integration_test

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/engine"
	"github.com/wxcuop/pyfixmsg_plus/network"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

func TestEngineStoreIntegration(t *testing.T) {
	f, err := os.CreateTemp("", "fixstore-integ-*.db")
	require.NoError(t, err)
	_ = f.Close()
	defer os.Remove(f.Name())

	st := store.NewSQLiteStore()
	require.NoError(t, st.Init(f.Name()))

	// start an acceptor to receive connection from engine
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	addr := ln.Addr().String()
	go func() {
		c, _ := ln.Accept()
		defer c.Close()
		// keep connection open briefly
		time.Sleep(50 * time.Millisecond)
	}()

	init := network.NewInitiator(addr)
	e := engine.NewFixEngine(init)
	require.NoError(t, e.Connect())
	defer e.Close()

	m := &store.Message{
		BeginString:  "FIX.4.4",
		SenderCompID: "S",
		TargetCompID: "T",
		MsgSeqNum:    99,
		MsgType:      "0",
		Body:         []byte("8=FIX.4.4\x0135=0\x01"),
		Created:      time.Now(),
	}

	require.NoError(t, st.SaveMessage(m))
	got, err := st.GetMessage(m.BeginString, m.SenderCompID, m.TargetCompID, m.MsgSeqNum)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, m.MsgSeqNum, got.MsgSeqNum)
}
