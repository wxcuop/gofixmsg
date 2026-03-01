package session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
	"github.com/wxcuop/pyfixmsg_plus/fixmsg/codec"
)

// ProcessorIface is the minimal interface Session requires from a processor.
type ProcessorIface interface {
	Process(*fixmsg.FixMessage) error
}

// Session manages the TCP connection, framing and dispatch to a Processor.
type Session struct {
	Conn      net.Conn
	Processor ProcessorIface
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	closed    bool
	// send queue
	sendCh chan []byte
	// OnMessage, if set, is called for each parsed inbound message instead of Processor.Process
	OnMessage func(*fixmsg.FixMessage)
	// OnClose, if set, is called once when the session is closed (by Stop or error)
	OnClose func()
	// internal flag to ensure OnClose is called only once
	onCloseCalled bool
}

func (s *Session) SetOnMessage(fn func(*fixmsg.FixMessage)) {
	s.OnMessage = fn
}

// SetOnClose registers a callback to be invoked once when the session closes.
func (s *Session) SetOnClose(fn func()) {
	s.OnClose = fn
}

func NewSession(conn net.Conn, p ProcessorIface) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{Conn: conn, Processor: p, ctx: ctx, cancel: cancel, sendCh: make(chan []byte, 128)}
}

// onCloseOnce invokes the OnClose callback at most once.
func (s *Session) onCloseOnce() {
	s.mu.Lock()
	if s.onCloseCalled {
		s.mu.Unlock()
		return
	}
	s.onCloseCalled = true
	fn := s.OnClose
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// abort performs a non-blocking shutdown from within read/write loops.
// It cancels the session context, closes the underlying connection and send channel,
// and invokes the OnClose callback exactly once.
func (s *Session) abort() {
	// ensure only one closer proceeds
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	// cancel context first so concurrent Send sees ctx.Done
	s.cancel()
	_ = s.Conn.Close()
	// close send channel to wake writeLoop
	select {
	case <-s.ctx.Done():
		// proceed
	default:
	}
	// closing sendCh; guard against panic by recover
	func() {
		defer func() { _ = recover() }()
		close(s.sendCh)
	}()
	// notify
	s.onCloseOnce()
}

// Start begins the read and write loops and returns immediately.
// Context returns the session's context for use in monitoring and cancellation.
func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) Start() {
	s.wg.Add(2)
	go s.readLoop()
	go s.writeLoop()
}

// Stop signals shutdown and waits for goroutines to finish.
func (s *Session) Stop() {
	// ensure we mark closed exactly once and still wait for goroutines
	s.mu.Lock()
	already := s.closed
	if !already {
		s.closed = true
	}
	s.mu.Unlock()
	if !already {
		s.cancel()
		_ = s.Conn.Close()
		// guard against double-close
		func() {
			defer func() { _ = recover() }()
			close(s.sendCh)
		}()
	}
	// wait for read/write goroutines to finish
	s.wg.Wait()
	// ensure OnClose is invoked once
	s.onCloseOnce()
}

// readLoop reads from the connection, assembles FIX frames using BodyLength (tag 9) framing,
// and dispatches complete frames sequentially to the Processor. It tolerates partial TCP reads.
func (s *Session) readLoop() {
	defer s.wg.Done()
	defer s.abort()
	buf := make([]byte, 0, 8192)
	tmp := make([]byte, 4096)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		_ = s.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := s.Conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			// try to extract complete frames using BodyLength framing
			for {
				end := findBodyLengthFrameEnd(buf)
				if end < 0 {
					break
				}
				frame := make([]byte, end)
				copy(frame, buf[:end])
				// consume
				buf = buf[end:]
				// dispatch SEQUENTIALLY to maintain exact message order (Phase 23 requirement)
				// let codec parse and hand to processor
				msg, err := codec.New(nil).Parse(frame)
				if err != nil {
					// parsing failed; ignore or log in real implementation
					continue
				}
				if s.OnMessage != nil {
					s.OnMessage(msg)
				} else {
					_ = s.Processor.Process(msg)
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return
			}
			// on timeout/temporary error, continue
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return
		}
	}
}

// writeLoop consumes sendCh and writes frames to the underlying connection.
func (s *Session) writeLoop() {
	defer s.wg.Done()
	defer s.abort()
	for {
		select {
		case <-s.ctx.Done():
			return
		case b, ok := <-s.sendCh:
			if !ok {
				return
			}
			_ = s.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_, err := s.Conn.Write(b)
			if err != nil {
				_ = s.Conn.Close()
				return
			}
		}
	}
}

// Send enqueues a raw FIX frame (already with checksum) to the send queue.
func (s *Session) Send(b []byte) error {
	select {
	case <-s.ctx.Done():
		return io.ErrClosedPipe
	case s.sendCh <- b:
		return nil
	}
}

// findBodyLengthFrameEnd implements robust FIX message framing using BodyLength (tag 9).
// A complete FIX frame has this structure:
//   8=<BeginString>\x019=<BodyLength>\x01...(BodyLength bytes)...10=<CheckSum>\x01
//
// The function returns the byte index (exclusive) of the first complete frame, or -1 if incomplete.
// It handles:
//   - Partial reads (frame split across TCP packets)
//   - BodyLength value split across packets
//   - Data fields containing "10=" or other FIX sequences (uses BodyLength byte count)
//   - Checksum validation (tag 10 must appear after exactly BodyLength body bytes)
func findBodyLengthFrameEnd(buf []byte) int {
	// Step 1: Verify we have BeginString (tag 8)
	if len(buf) < 3 || !bytes.HasPrefix(buf, []byte("8=")) {
		return -1
	}
	
	// Step 2: Find the SOH after BeginString value (skip until first SOH)
	beginStringEnd := bytes.IndexByte(buf, 0x01)
	if beginStringEnd < 0 {
		return -1
	}
	
	// Step 3: Parse BodyLength (tag 9)
	bodyLengthStart := beginStringEnd + 1
	if bodyLengthStart+3 > len(buf) || !bytes.HasPrefix(buf[bodyLengthStart:], []byte("9=")) {
		return -1
	}
	
	// Step 4: Extract BodyLength value (digits between "9=" and next SOH)
	bodyLengthValStart := bodyLengthStart + 2
	bodyLengthValEnd := bytes.IndexByte(buf[bodyLengthValStart:], 0x01)
	if bodyLengthValEnd < 0 {
		return -1
	}
	bodyLengthValEnd += bodyLengthValStart
	
	// Parse the numeric value
	bodyLengthStr := string(buf[bodyLengthValStart:bodyLengthValEnd])
	var bodyLength int
	_, err := fmt.Sscanf(bodyLengthStr, "%d", &bodyLength)
	if err != nil || bodyLength < 0 {
		return -1
	}
	
	// Step 5: Calculate where the message body ends
	// Body starts after the SOH that ends tag 9's value
	bodyStart := bodyLengthValEnd + 1
	bodyEnd := bodyStart + bodyLength
	
	// Step 6: Verify we have enough bytes for body + checksum tag "10=NNN\x01" (7 bytes minimum)
	if len(buf) < bodyEnd+7 {
		return -1
	}
	
	// Step 7: Verify tag 10 is at the expected position (after BodyLength bytes)
	if !bytes.HasPrefix(buf[bodyEnd:], []byte("10=")) {
		return -1
	}
	
	// Step 8: Verify checksum digits (at bodyEnd+3, bodyEnd+4, bodyEnd+5) and SOH (at bodyEnd+6)
	if buf[bodyEnd+3] < '0' || buf[bodyEnd+3] > '9' ||
		buf[bodyEnd+4] < '0' || buf[bodyEnd+4] > '9' ||
		buf[bodyEnd+5] < '0' || buf[bodyEnd+5] > '9' ||
		buf[bodyEnd+6] != 0x01 {
		return -1
	}
	
	// Return the index after the final SOH
	return bodyEnd + 7 // "10=NNN\x01"
}
