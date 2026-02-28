package engine

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
)

// Phase 22: BodyLength Framing Tests
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

func (tp *testProcessor) Register(msgType string, fn func(*fixmsg.FixMessage) error) {}
