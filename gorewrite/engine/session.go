package engine

import (
	"bytes"
	"context"
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
	sendCh    chan []byte
}

func NewSession(conn net.Conn, p ProcessorIface) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{Conn: conn, Processor: p, ctx: ctx, cancel: cancel, sendCh: make(chan []byte, 128)}
}

// Start begins the read and write loops and returns immediately.
func (s *Session) Start() {
	s.wg.Add(2)
	go s.readLoop()
	go s.writeLoop()
}

// Stop signals shutdown and waits for goroutines to finish.
func (s *Session) Stop() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	_ = s.Conn.Close()
	close(s.sendCh)
	s.wg.Wait()
}

// readLoop reads from the connection, assembles FIX frames by looking for the checksum (10=nnn<SOH>) tag,
// and dispatches complete frames to the Processor. It tolerates partial TCP reads.
func (s *Session) readLoop() {
	defer s.wg.Done()
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
			// try to extract complete frames
			for {
				end := findChecksumFrameEnd(buf)
				if end < 0 {
					break
				}
				frame := make([]byte, end)
				copy(frame, buf[:end])
				// consume
				buf = buf[end:]
				// dispatch in goroutine to avoid blocking read loop
				f := frame
				s.wg.Add(1)
				go func(fb []byte) {
					defer s.wg.Done()
					// let codec parse and hand to processor
					msg, err := codec.New(nil).Parse(fb)
					if err != nil {
						// parsing failed; ignore or log in real implementation
						return
					}
					_ = s.Processor.Process(msg)
				}(f)
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

// findChecksumFrameEnd locates the end index (exclusive) of the first complete FIX frame found in buf,
// returning -1 if none found. A complete frame ends with tag 10=NNN<SOH> where NNN are three digits.
func findChecksumFrameEnd(buf []byte) int {
	// look for pattern "10=\d\d\d\x01"
	idx := bytes.Index(buf, []byte("10="))
	if idx < 0 {
		return -1
	}
	// ensure we have at least 6 bytes after idx: "10=" + 3 digits + '\x01'
	if len(buf) < idx+6 {
		return -1
	}
	// verify digits and trailing SOH
	if buf[idx+3] < '0' || buf[idx+3] > '9' || buf[idx+4] < '0' || buf[idx+4] > '9' || buf[idx+5] < '0' || buf[idx+5] > '9' {
		return -1
	}
	if buf[idx+6-1] != 0x01 { // idx+5 is third digit, idx+6-1 == idx+5, but keep readable
		// actually need idx+6-1 == idx+5 already checked; re-evaluate: trailing SOH is at idx+6
	}
	// trailing SOH should be at idx+6
	if buf[idx+6] != 0x01 {
		return -1
	}
	return idx + 7 // end index exclusive: position after trailing SOH (idx .. idx+6 were start..SOH)
}
