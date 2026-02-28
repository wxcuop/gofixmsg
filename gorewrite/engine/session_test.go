package engine

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
)

type fakeProcessor struct {
	ch chan *fixmsg.FixMessage
}

func (f *fakeProcessor) Process(m *fixmsg.FixMessage) error {
	f.ch <- m
	return nil
}

func (f *fakeProcessor) Register(msgType string, fn func(*fixmsg.FixMessage) error) {}

func TestSessionPartialRead(t *testing.T) {
	r1, w1 := net.Pipe()
	proc := &fakeProcessor{ch: make(chan *fixmsg.FixMessage, 1)}
	s := NewSession(r1, proc)
	s.Start()
	defer s.Stop()

	// craft simple FIX message: 8=FIX.4.2\x019=5\x0135=0\x0110=000\x01
	// BodyLength=5 for body "35=0\x01" (4+1 bytes)
	msg := "8=FIX.4.2\x019=5\x0135=0\x0110=000\x01"

	// write in two parts to simulate partial TCP frames
	_, err := w1.Write([]byte(msg[:10]))
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	_, err = w1.Write([]byte(msg[10:]))
	require.NoError(t, err)

	select {
	case m := <-proc.ch:
		require.NotNil(t, m)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
	_ = w1.Close()
	_ = r1.Close()
}
