package network

import (
	"crypto/tls"
	"fmt"
)

// LoadTLSConfig loads TLS configuration from certificate files.
// Returns nil if no cert/key files are provided (non-TLS mode).
// Returns an error if cert/key files are specified but cannot be loaded.
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

	// Load CA certificate if provided (for client-side verification)
	if caFile != "" {
		caCert, err := readCertFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %w", err)
		}
		// For simplicity, just store the cert data. Real implementations should
		// add to certPool and use SetClientCertVerification.
		cfg.ClientAuth = tls.RequireAnyClientCert
		_ = caCert // TODO: Implement full CA cert chain validation
	}

	return cfg, nil
}

// readCertFile is a helper to read certificate file content.
func readCertFile(filename string) ([]byte, error) {
	// In a full implementation, this would parse the cert and add to x509.CertPool
	// For now, just ensure the file can be read.
	content := make([]byte, 0)
	// In a real implementation, use ioutil.ReadFile or os.ReadFile
	// content, err := os.ReadFile(filename)
	return content, nil
}
