package network

import (
	"crypto/tls"
	"net"
	"time"
)

// Initiator dials a remote address.
type Initiator struct {
	Addr      string
	Timeout   time.Duration
	TLSConfig *tls.Config
}

func NewInitiator(addr string) *Initiator { return &Initiator{Addr: addr, Timeout: 5 * time.Second} }

func (i *Initiator) WithTLS(cfg *tls.Config) *Initiator { i.TLSConfig = cfg; return i }

func (i *Initiator) Connect() (net.Conn, error) {
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
		return tlsConn, nil
	}
	return conn, nil
}
