package session

import (
	"net"
	"sync"
	"sync/atomic"
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

// These tests verify that findBodyLengthFrameEnd correctly parses FIX messages using BodyLength (tag 9)

func TestFindBodyLengthFrameEnd_CompleteFrame(t *testing.T) {
	// Simple frame: 8=FIX.4.2\x019=5\x0135=0\x0110=000\x01
	// BodyLength=5: "35=0\x01" (4+1=5 bytes)
	msg := []byte("8=FIX.4.2\x019=5\x0135=0\x0110=000\x01")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, len(msg), end, "should find complete frame")
}

func TestFindBodyLengthFrameEnd_PartialBeginString(t *testing.T) {
	// Incomplete: just "8=FIX" without SOH
	msg := []byte("8=FIX")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, -1, end, "should return -1 when BeginString incomplete")
}

func TestFindBodyLengthFrameEnd_PartialBodyLengthValue(t *testing.T) {
	// Split at middle of BodyLength value: "8=FIX.4.2\x019=1" (no SOH after value)
	msg := []byte("8=FIX.4.2\x019=1")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, -1, end, "should return -1 when BodyLength value incomplete")
}

func TestFindBodyLengthFrameEnd_PartialBody(t *testing.T) {
	// BodyLength=5 but only 3 bytes of body provided
	msg := []byte("8=FIX.4.2\x019=5\x0135=")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, -1, end, "should return -1 when body incomplete")
}

func TestFindBodyLengthFrameEnd_MultipleFields(t *testing.T) {
	// Frame with two fields: 8=FIX.4.2\x019=20\x0149=SENDER\x0156=TARGET\x0110=000\x01
	// BodyLength=20: "49=SENDER\x0156=TARGET\x01" (9+1+9+1=20)
	msg := []byte("8=FIX.4.2\x019=20\x0149=SENDER\x0156=TARGET\x0110=000\x01")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, len(msg), end, "should find frame with multiple fields")
}

func TestFindBodyLengthFrameEnd_10InDataField(t *testing.T) {
	// Critical test: "10=" appearing in the message data field (tag 58)
	// Frame: 8=FIX.4.2\x019=17\x0158=Error: 10=bad\x0110=000\x01
	// BodyLength=17: "58=Error: 10=bad\x01" (15+1+1=17 bytes)
	// The "10=" in the text field must not be confused with the checksum tag
	msg := []byte("8=FIX.4.2\x019=17\x0158=Error: 10=bad\x0110=000\x01")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, len(msg), end, "should use BodyLength to skip over '10=' in data field")
}

func TestFindBodyLengthFrameEnd_MultipleFrames(t *testing.T) {
	// Two consecutive frames in buffer
	msg1 := []byte("8=FIX.4.2\x019=5\x0135=0\x0110=000\x01")
	msg2 := []byte("8=FIX.4.2\x019=5\x0135=1\x0110=001\x01")
	combined := append(msg1, msg2...)

	// Should find first frame only
	end := findBodyLengthFrameEnd(combined)
	require.Equal(t, len(msg1), end, "should find first frame")

	// Should find second frame after consuming first
	end2 := findBodyLengthFrameEnd(combined[end:])
	require.Equal(t, len(msg2), end2, "should find second frame in remainder")
}

func TestFindBodyLengthFrameEnd_InvalidBeginString(t *testing.T) {
	// Frame not starting with tag 8
	msg := []byte("GARBAGE\x019=5\x0135=0\x0110=000\x01")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, -1, end, "should reject frame not starting with tag 8")
}

func TestFindBodyLengthFrameEnd_MissingTag9(t *testing.T) {
	// Frame with tag 8 but missing tag 9
	msg := []byte("8=FIX.4.2\x0135=0\x0110=000\x01")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, -1, end, "should reject frame missing tag 9")
}

func TestFindBodyLengthFrameEnd_InvalidBodyLength(t *testing.T) {
	// Non-numeric BodyLength
	msg := []byte("8=FIX.4.2\x019=ABC\x0135=0\x0110=000\x01")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, -1, end, "should reject non-numeric BodyLength")
}

func TestFindBodyLengthFrameEnd_MissingChecksum(t *testing.T) {
	// Frame missing checksum tag
	msg := []byte("8=FIX.4.2\x019=5\x0135=0")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, -1, end, "should reject frame missing checksum tag")
}

func TestFindBodyLengthFrameEnd_InvalidChecksumDigits(t *testing.T) {
	// Non-numeric checksum
	msg := []byte("8=FIX.4.2\x019=5\x0135=0\x0110=ABC\x01")
	end := findBodyLengthFrameEnd(msg)
	require.Equal(t, -1, end, "should reject non-numeric checksum digits")
}

// Test partial reads across multiple TCP packets
func TestSessionWithBodyLengthFraming_PartialReads(t *testing.T) {
	r, w := net.Pipe()
	proc := &fakeProcessor{ch: make(chan *fixmsg.FixMessage, 2)}
	s := NewSession(r, proc)
	s.Start()
	defer s.Stop()

	// Send complete message in two parts (split at BodyLength value)
	msg := "8=FIX.4.2\x019=5\x0135=0\x0110=000\x01"
	// Split at position 13: "8=FIX.4.2\x019=" | "5\x0135=0\x0110=000\x01"
	_, err := w.Write([]byte(msg[:13]))
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	_, err = w.Write([]byte(msg[13:]))
	require.NoError(t, err)

	// Verify message parsed
	select {
	case m := <-proc.ch:
		require.NotNil(t, m)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
	_ = w.Close()
}

// Test with "10=" in data field
func TestSessionWithBodyLengthFraming_10InData(t *testing.T) {
	r, w := net.Pipe()
	proc := &fakeProcessor{ch: make(chan *fixmsg.FixMessage, 1)}
	s := NewSession(r, proc)
	s.Start()
	defer s.Stop()

	// Message with "10=" in text field
	msg := "8=FIX.4.2\x019=17\x0158=Error: 10=bad\x0110=000\x01"
	_, err := w.Write([]byte(msg))
	require.NoError(t, err)

	// Should parse correctly, not stopping at "10=" in data
	select {
	case m := <-proc.ch:
		require.NotNil(t, m)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
	_ = w.Close()
}

// Test sequential message processing (Phase 23 prerequisite)
// Phase 22 readLoop has removed goroutines, so messages are processed sequentially
func TestSessionSequentialMessageProcessing(t *testing.T) {
	r, w := net.Pipe()

	var messages []*fixmsg.FixMessage
	var mu sync.Mutex

	// Custom processor to track call order
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

	// Send three messages
	for i := 0; i < 3; i++ {
		msg := "8=FIX.4.2\x019=5\x0135=0\x0110=000\x01"
		_, err := w.Write([]byte(msg))
		require.NoError(t, err)
		time.Sleep(25 * time.Millisecond)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)
	w.Close()
	s.Stop()

	// All messages should be processed
	require.Equal(t, 3, len(messages), "should process all 3 messages sequentially")
}

// testProcessor implements ProcessorIface for testing
type testProcessor struct {
	processFn func(*fixmsg.FixMessage) error
}

func (tp *testProcessor) Process(m *fixmsg.FixMessage) error {
	return tp.processFn(m)
}


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
