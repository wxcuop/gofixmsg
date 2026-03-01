package session

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/fixmsg"
)

func TestSessionSendQueue(t *testing.T) {
	r, w := net.Pipe()
	proc := &fakeProcessor{ch: make(chan *fixmsg.FixMessage, 1)}
	s := NewSession(r, proc)
	s.Start()
	defer s.Stop()

	payload := []byte("8=FIX.4.2\x019=12\x0135=0\x0110=000\x01")

	// send via queue
	err := s.Send(payload)
	require.NoError(t, err)

	// read from peer
	buf := make([]byte, 1024)
	n, err := w.Read(buf)
	require.NoError(t, err)
	require.Equal(t, payload, buf[:n])

	_ = w.Close()
}
