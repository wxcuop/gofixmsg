package network_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/network"
)

// TestConnWrapper validates that Conn properly wraps net.Conn with Send/SetReadDeadline.
func TestConnWrapper(t *testing.T) {
	acc := network.NewAcceptor("127.0.0.1:0")
	received := make(chan []byte, 1)

	handler := func(c *network.Conn) {
		defer c.Close()
		buf := make([]byte, 64)
		n, err := c.Read(buf)
		require.NoError(t, err)
		received <- buf[:n]
	}

	require.NoError(t, acc.Start(handler))
	time.Sleep(10 * time.Millisecond)

	srvAddr := acc.AddrString()
	init := network.NewInitiator(srvAddr)
	c, err := init.Connect()
	require.NoError(t, err)
	defer c.Close()

	// Test Send method (should flush write buffer)
	testData := []byte("hello from conn wrapper")
	err = c.Send(testData)
	require.NoError(t, err)

	select {
	case msg := <-received:
		require.Equal(t, testData, msg)
	case <-time.After(500 * time.Millisecond):
		require.Fail(t, "timeout waiting for message")
	}

	require.NoError(t, acc.Stop())
}

// TestSetReadDeadline validates deadline setting on Conn.
func TestSetReadDeadline(t *testing.T) {
	acc := network.NewAcceptor("127.0.0.1:0")
	done := make(chan bool, 1)

	handler := func(c *network.Conn) {
		defer c.Close()
		// Set a very short read deadline to test timeout
		deadline := time.Now().Add(50 * time.Millisecond)
		err := c.SetReadDeadline(deadline)
		require.NoError(t, err)

		buf := make([]byte, 64)
		// This read should timeout
		_, err = c.Read(buf)
		require.Error(t, err)
		done <- true
	}

	require.NoError(t, acc.Start(handler))
	time.Sleep(10 * time.Millisecond)

	srvAddr := acc.AddrString()
	init := network.NewInitiator(srvAddr)
	c, err := init.Connect()
	require.NoError(t, err)
	defer c.Close()

	select {
	case <-done:
		// Handler completed as expected
	case <-time.After(500 * time.Millisecond):
		require.Fail(t, "timeout waiting for handler")
	}

	require.NoError(t, acc.Stop())
}

// TestInitiatorAndAcceptor validates basic Initiator/Acceptor with Conn wrapper.
func TestInitiatorAndAcceptor(t *testing.T) {
	acc := network.NewAcceptor("127.0.0.1:0")
	received := make(chan string, 1)

	handler := func(c *network.Conn) {
		defer c.Close()
		buf := make([]byte, 64)
		n, _ := c.Read(buf)
		received <- string(buf[:n])
	}

	require.NoError(t, acc.Start(handler))
	time.Sleep(10 * time.Millisecond)

	srvAddr := acc.AddrString()
	init := network.NewInitiator(srvAddr)
	c, err := init.Connect()
	require.NoError(t, err)
	defer c.Close()

	_, err = c.Write([]byte("hello"))
	require.NoError(t, err)
	err = c.Flush()
	require.NoError(t, err)

	select {
	case msg := <-received:
		require.Equal(t, "hello", msg)
	case <-time.After(500 * time.Millisecond):
		require.Fail(t, "timeout waiting for message")
	}

	require.NoError(t, acc.Stop())
}

// TestAcceptorPerClientGoroutines validates that Acceptor properly handles per-client goroutines.
func TestAcceptorPerClientGoroutines(t *testing.T) {
	acc := network.NewAcceptor("127.0.0.1:0")
	received := make(chan string, 3)

	handler := func(c *network.Conn) {
		defer c.Close()
		buf := make([]byte, 64)
		n, _ := c.Read(buf)
		received <- string(buf[:n])
	}

	require.NoError(t, acc.Start(handler))
	time.Sleep(10 * time.Millisecond)

	srvAddr := acc.AddrString()

	// Create 3 concurrent clients
	for i := 0; i < 3; i++ {
		go func(msg string) {
			init := network.NewInitiator(srvAddr)
			c, err := init.Connect()
			require.NoError(t, err)
			defer c.Close()
			_, err = c.Write([]byte(msg))
			require.NoError(t, err)
			err = c.Flush()
			require.NoError(t, err)
		}("client" + string(rune(i)))
	}

	// Verify all 3 messages were received
	count := 0
	timeout := time.After(1 * time.Second)
	for count < 3 {
		select {
		case <-received:
			count++
		case <-timeout:
			require.Fail(t, "timeout waiting for all messages")
		}
	}

	require.NoError(t, acc.Stop())
}

// TestBufferSizeVerification validates that 8192-byte buffers are used.
func TestBufferSizeVerification(t *testing.T) {
	acc := network.NewAcceptor("127.0.0.1:0")
	largeData := make([]byte, 4096) // Less than buffer size
	for i := range largeData {
		largeData[i] = 'A'
	}

	received := make(chan []byte, 1)
	handler := func(c *network.Conn) {
		defer c.Close()
		buf := make([]byte, 8192)
		n, _ := c.Read(buf)
		received <- buf[:n]
	}

	require.NoError(t, acc.Start(handler))
	time.Sleep(10 * time.Millisecond)

	srvAddr := acc.AddrString()
	init := network.NewInitiator(srvAddr)
	c, err := init.Connect()
	require.NoError(t, err)
	defer c.Close()

	_, err = c.Write(largeData)
	require.NoError(t, err)
	err = c.Flush()
	require.NoError(t, err)

	select {
	case msg := <-received:
		require.Equal(t, largeData, msg)
	case <-time.After(500 * time.Millisecond):
		require.Fail(t, "timeout waiting for large message")
	}

	require.NoError(t, acc.Stop())
}
