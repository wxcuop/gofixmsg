package network

import (
	"crypto/tls"
	"net"
	"time"
)

// Initiator dials a remote address and establishes a connection.
type Initiator struct {
	Addr      string
	Timeout   time.Duration
	TLSConfig *tls.Config
}

// NewInitiator creates a new Initiator with default 5-second timeout.
func NewInitiator(addr string) *Initiator {
	return &Initiator{Addr: addr, Timeout: 5 * time.Second}
}

// WithTLS sets the TLS configuration for the connection.
func (i *Initiator) WithTLS(cfg *tls.Config) *Initiator {
	i.TLSConfig = cfg
	return i
}

// Connect establishes a connection to the remote address.
// Returns a Conn wrapper with 8192-byte buffering.
func (i *Initiator) Connect() (*Conn, error) {
	if i.Timeout == 0 {
		i.Timeout = 5 * time.Second
	}
	conn, err := net.DialTimeout("tcp", i.Addr, i.Timeout)
	if err != nil {
		return nil, err
	}
	if i.TLSConfig != nil {
		tlsConn := tls.Client(conn, i.TLSConfig)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, err
		}
		return NewConn(tlsConn), nil
	}
	return NewConn(conn), nil
}
