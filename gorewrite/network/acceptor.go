package network

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
)

// Acceptor listens for incoming connections and dispatches them to a handler.
// Each accepted connection runs in its own goroutine with proper cleanup.
type Acceptor struct {
	Addr      string
	ln        net.Listener
	TLSConfig *tls.Config
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewAcceptor creates a new Acceptor for the given address.
func NewAcceptor(addr string) *Acceptor {
	return &Acceptor{
		Addr:   addr,
		stopCh: make(chan struct{}),
	}
}

// WithTLS sets the TLS configuration for the acceptor.
func (a *Acceptor) WithTLS(cfg *tls.Config) *Acceptor {
	a.TLSConfig = cfg
	return a
}

// Start begins accepting connections on the listener.
// Each connection is wrapped in a Conn with 8192-byte buffers and passed to the handler.
// The handler is called in its own goroutine per client.
func (a *Acceptor) Start(handler func(*Conn)) error {
	ln, err := net.Listen("tcp", a.Addr)
	if err != nil {
		return fmt.Errorf("acceptor: listen: %w", err)
	}
	a.ln = ln

	// Wrap with TLS if configured
	if a.TLSConfig != nil {
		a.ln = tls.NewListener(ln, a.TLSConfig)
	}

	// Start acceptance loop in a goroutine
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.acceptLoop(handler)
	}()

	return nil
}

// acceptLoop continuously accepts connections until stopped.
func (a *Acceptor) acceptLoop(handler func(*Conn)) {
	for {
		select {
		case <-a.stopCh:
			return
		default:
		}

		c, err := a.ln.Accept()
		if err != nil {
			// Accept error (likely due to listener being closed)
			return
		}

		// Wrap connection and handle in per-client goroutine
		a.wg.Add(1)
		go func(conn net.Conn) {
			defer a.wg.Done()
			wrapped := NewConn(conn)
			handler(wrapped)
			wrapped.Close()
		}(c)
	}
}

// AddrString returns the listener's address string.
func (a *Acceptor) AddrString() string {
	if a.ln != nil {
		return a.ln.Addr().String()
	}
	return a.Addr
}

// Stop gracefully stops the acceptor and waits for all goroutines to finish.
func (a *Acceptor) Stop() error {
	close(a.stopCh)
	if a.ln != nil {
		a.ln.Close()
	}
	a.wg.Wait()
	return nil
}
