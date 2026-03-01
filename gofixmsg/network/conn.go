package network

import (
	"bufio"
	"net"
	"time"
)

// Conn wraps a net.Conn with Send and SetReadDeadline abstractions.
// This enables consistent handling of both raw TCP and TLS connections.
type Conn struct {
	underlying net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	bufSize    int
}

// NewConn creates a new Conn wrapper with 8192-byte buffers (matching Python implementation).
func NewConn(nc net.Conn) *Conn {
	const bufSize = 8192
	return &Conn{
		underlying: nc,
		reader:     bufio.NewReaderSize(nc, bufSize),
		writer:     bufio.NewWriterSize(nc, bufSize),
		bufSize:    bufSize,
	}
}

// Send writes data to the connection and flushes the buffer.
func (c *Conn) Send(data []byte) error {
	if _, err := c.writer.Write(data); err != nil {
		return err
	}
	return c.writer.Flush()
}

// Write writes data to the connection buffer (satisfies io.Writer).
// Data is buffered; call Flush or Send to ensure delivery.
func (c *Conn) Write(p []byte) (int, error) {
	return c.writer.Write(p)
}

// Flush flushes the write buffer.
func (c *Conn) Flush() error {
	return c.writer.Flush()
}

// SetDeadline sets both read and write deadlines on the underlying connection.
func (c *Conn) SetDeadline(t time.Time) error {
	return c.underlying.SetDeadline(t)
}

// SetReadDeadline sets the read deadline for the underlying connection.
func (c *Conn) SetReadDeadline(deadline time.Time) error {
	return c.underlying.SetReadDeadline(deadline)
}

// SetWriteDeadline sets the write deadline for the underlying connection.
func (c *Conn) SetWriteDeadline(deadline time.Time) error {
	return c.underlying.SetWriteDeadline(deadline)
}

// Read reads data from the buffered reader.
func (c *Conn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// ReadByte reads a single byte from the buffered reader.
func (c *Conn) ReadByte() (byte, error) {
	return c.reader.ReadByte()
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.underlying.Close()
}

// LocalAddr returns the local address of the underlying connection.
func (c *Conn) LocalAddr() net.Addr {
	return c.underlying.LocalAddr()
}

// RemoteAddr returns the remote address of the underlying connection.
func (c *Conn) RemoteAddr() net.Addr {
	return c.underlying.RemoteAddr()
}

// Underlying returns the underlying net.Conn for advanced operations if needed.
func (c *Conn) Underlying() net.Conn {
	return c.underlying
}
