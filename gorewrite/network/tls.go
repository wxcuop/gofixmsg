package network

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// LoadTLSConfig loads TLS configuration from certificate files.
// Returns nil if no cert/key files are provided (non-TLS mode).
// Returns an error if cert/key files are specified but cannot be loaded.
// caFile is used to configure both RootCAs (for client cert verification) and ClientCAs (for server cert verification).
func LoadTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	// No TLS config if no cert file
	if certFile == "" {
		return nil, nil
	}

	cfg := &tls.Config{}

	// Load server certificate and key
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate/key pair: %w", err)
	}
	cfg.Certificates = []tls.Certificate{cert}

	// Load CA certificate if provided (for peer verification)
	if caFile != "" {
		caCertData, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate file: %w", err)
		}

		// Parse CA certificate and add to cert pool
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCertData) {
			return nil, fmt.Errorf("failed to parse CA certificate from %s", caFile)
		}

		// Configure for both client and server verification modes
		// RootCAs: used for verifying peer certs when in client mode (initiator)
		cfg.RootCAs = caCertPool
		// ClientCAs: used for verifying client certs when in server mode (acceptor)
		cfg.ClientCAs = caCertPool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return cfg, nil
}
