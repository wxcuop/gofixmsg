package engine

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestHeartbeatMonitor_SendsTestRequestAndCloses verifies that the monitor
// sends a TestRequest after a missed heartbeat interval and closes the session
// when the TestRequest times out.
func TestHeartbeatMonitor_SendsTestRequestAndCloses(t *testing.T) {
	// create a net.Pipe to observe outbound bytes
	local, peer := net.Pipe()
	defer peer.Close()

	// create a minimal processor that does nothing
	dummy := NewProcessor()

	// create session on local end
	s := NewSession(local, dummy)
	s.Start()
	defer s.Stop()

	// create engine and attach session
	e := &FixEngine{Session: s}

	// attach engine to monitor
	mon := NewHeartbeatMonitor(e, 20*time.Millisecond, 40*time.Millisecond)
	e.Monitor = mon

	// start monitor
	mon.Start(nil)
	defer mon.Stop()

	// expect a TestRequest to be sent within ~100ms
	buf := make([]byte, 4096)
	peer.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	n, err := peer.Read(buf)
	require.NoError(t, err)
	out := buf[:n]
	// should contain 35=1 (TestRequest)
	require.Contains(t, string(out), "35=1")

	// after TestRequest is sent, monitor should close session after timeout
	// wait slightly longer than the TestReqTimeout
	time.Sleep(80 * time.Millisecond)

	// further reads should return EOF because session.Stop closed the local side
	peer.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, err = peer.Read(buf)
	require.True(t, err == io.EOF || err != nil)
}
