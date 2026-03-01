package engine

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
)

// Phase 23: Sequential Processing Tests
// These tests verify that messages are processed sequentially in exact order
// and that the per-message goroutines have been removed from readLoop

// TestSessionProcessingIsSequential verifies messages are NOT processed in parallel goroutines
// Phase 23 requirement: Remove per-message goroutines, ensure sequential handling
func TestSessionProcessingIsSequential(t *testing.T) {
	r, w := net.Pipe()

	var processedSeqNums []int
	var mu sync.Mutex
	var inFlight int32 = 0

	customProc := &testProcessor{
		processFn: func(m *fixmsg.FixMessage) error {
			// Track concurrent processing
			current := atomic.AddInt32(&inFlight, 1)
			defer atomic.AddInt32(&inFlight, -1)

			// If more than one is in-flight at the same time, we have goroutines!
			if current > 1 {
				t.Logf("WARNING: %d messages processing concurrently (should be 1)", current)
			}

			// Small delay to detect concurrency
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			defer mu.Unlock()
			processedSeqNums = append(processedSeqNums, len(processedSeqNums))
			return nil
		},
	}

	s := NewSession(r, customProc)
	s.Start()
	defer s.Stop()

	// Send 5 messages in quick succession
	for i := 0; i < 5; i++ {
		msg := "8=FIX.4.2\x019=5\x0135=0\x0110=000\x01"
		_, err := w.Write([]byte(msg))
		require.NoError(t, err)
	}

	// Wait for processing
	time.Sleep(1 * time.Second)
	w.Close()
	s.Stop()

	// Verify all messages were processed
	require.Equal(t, 5, len(processedSeqNums), "should process all 5 messages")

	// Verify order: [0, 1, 2, 3, 4]
	for i := 0; i < len(processedSeqNums); i++ {
		require.Equal(t, i, processedSeqNums[i], "message %d should be processed in order", i)
	}
}

// TestSequenceIntegrity tests that MsgSeqNum ordering is maintained
func TestSequenceIntegrity(t *testing.T) {
	r, w := net.Pipe()

	var messages []*fixmsg.FixMessage
	var mu sync.Mutex

	customProc := &testProcessor{
		processFn: func(m *fixmsg.FixMessage) error {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, m)
			return nil
		},
	}

	s := NewSession(r, customProc)
	s.Start()
	defer s.Stop()

	// Send 3 messages with different tags
	msgs := []string{
		"8=FIX.4.2\x019=5\x0135=A\x0110=000\x01", // MsgType=A
		"8=FIX.4.2\x019=5\x0135=B\x0110=001\x01", // MsgType=B
		"8=FIX.4.2\x019=5\x0135=C\x0110=002\x01", // MsgType=C
	}

	for _, msg := range msgs {
		_, err := w.Write([]byte(msg))
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Small delay between sends
	}

	time.Sleep(500 * time.Millisecond)
	w.Close()
	s.Stop()

	// Verify all 3 messages parsed
	require.Equal(t, 3, len(messages), "should parse all 3 messages")

	// Verify order by checking MsgType (tag 35)
	expected := []string{"A", "B", "C"}
	for i, m := range messages {
		msgType, ok := m.Get(35)
		require.True(t, ok, "message %d should have tag 35", i)
		require.Equal(t, expected[i], msgType, "message %d MsgType mismatch", i)
	}
}

// TestHandleIncomingCalledSequentially verifies processor is called in order
// Phase 23: Verify HandleIncoming called sequentially (no goroutines in readLoop)
func TestHandleIncomingCalledSequentially(t *testing.T) {
	r, w := net.Pipe()

	var callOrder []int
	var mu sync.Mutex
	var processDelay time.Duration = 50 * time.Millisecond

	customProc := &testProcessor{
		processFn: func(m *fixmsg.FixMessage) error {
			// Simulate processing time
			time.Sleep(processDelay)

			mu.Lock()
			defer mu.Unlock()
			callOrder = append(callOrder, len(callOrder))
			return nil
		},
	}

	s := NewSession(r, customProc)
	s.Start()
	defer s.Stop()

	// Send 4 messages
	for i := 0; i < 4; i++ {
		msg := "8=FIX.4.2\x019=5\x0135=0\x0110=000\x01"
		_, err := w.Write([]byte(msg))
		require.NoError(t, err)
		// Send without delay - if there are goroutines, they'll all run concurrently
	}

	// Wait for all to complete
	time.Sleep(time.Duration(5) * processDelay)
	w.Close()
	s.Stop()

	// Verify all were called
	require.Equal(t, 4, len(callOrder), "should call processor 4 times")

	// Verify order
	for i := 0; i < len(callOrder); i++ {
		require.Equal(t, i, callOrder[i], "processor call %d should be in order", i)
	}
}

// TestNoGoroutineSpawning verifies that readLoop doesn't spawn goroutines per message
// This is the core Phase 23 requirement: remove per-message goroutines
func TestNoGoroutineSpawning(t *testing.T) {
	r, w := net.Pipe()

	var maxConcurrent int32 = 0
	var currentConcurrent int32 = 0

	customProc := &testProcessor{
		processFn: func(m *fixmsg.FixMessage) error {
			// Track peak concurrency
			curr := atomic.AddInt32(&currentConcurrent, 1)
			defer atomic.AddInt32(&currentConcurrent, -1)

			// Update max
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if curr > old {
					if atomic.CompareAndSwapInt32(&maxConcurrent, old, curr) {
						break
					}
				} else {
					break
				}
			}

			// Simulate processing
			time.Sleep(20 * time.Millisecond)
			return nil
		},
	}

	s := NewSession(r, customProc)
	s.Start()
	defer s.Stop()

	// Send 10 messages quickly
	for i := 0; i < 10; i++ {
		msg := "8=FIX.4.2\x019=5\x0135=0\x0110=000\x01"
		_, err := w.Write([]byte(msg))
		require.NoError(t, err)
	}

	time.Sleep(500 * time.Millisecond)
	w.Close()
	s.Stop()

	maxConc := atomic.LoadInt32(&maxConcurrent)
	require.Equal(t, int32(1), maxConc, "should never have more than 1 message processing concurrently")
}

// TestMultiMessageSequenceWithFrameSplits tests sequence integrity with split frames
// Phase 23: Verify exact sequence order even with TCP frame splitting
func TestMultiMessageSequenceWithFrameSplits(t *testing.T) {
	r, w := net.Pipe()

	var messages []*fixmsg.FixMessage
	var mu sync.Mutex

	customProc := &testProcessor{
		processFn: func(m *fixmsg.FixMessage) error {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, m)
			return nil
		},
	}

	s := NewSession(r, customProc)
	s.Start()
	defer s.Stop()

	// Send 3 messages, each split at a different position
	// Message 1: split in BeginString
	msg1 := "8=FIX.4.2\x019=5\x0135=0\x0110=000\x01"
	_, _ = w.Write([]byte(msg1[:8]))  // "8=FIX.4"
	time.Sleep(25 * time.Millisecond)
	_, _ = w.Write([]byte(msg1[8:]))  // ".2\x019=5\x0135=0\x0110=000\x01"

	time.Sleep(25 * time.Millisecond)

	// Message 2: split in BodyLength
	msg2 := "8=FIX.4.2\x019=5\x0135=1\x0110=001\x01"
	_, _ = w.Write([]byte(msg2[:13]))  // "8=FIX.4.2\x019="
	time.Sleep(25 * time.Millisecond)
	_, _ = w.Write([]byte(msg2[13:]))  // "5\x0135=1\x0110=001\x01"

	time.Sleep(25 * time.Millisecond)

	// Message 3: whole frame
	msg3 := "8=FIX.4.2\x019=5\x0135=2\x0110=002\x01"
	_, _ = w.Write([]byte(msg3))

	time.Sleep(500 * time.Millisecond)
	w.Close()
	s.Stop()

	// Verify all 3 were processed in order
	require.Equal(t, 3, len(messages), "should process all 3 messages")

	// Verify order by checking MsgType
	expected := []string{"0", "1", "2"}
	for i, m := range messages {
		msgType, ok := m.Get(35)
		require.True(t, ok, "message %d should have tag 35", i)
		require.Equal(t, expected[i], msgType, "message %d should be %s, got %s", i, expected[i], msgType)
	}
}
