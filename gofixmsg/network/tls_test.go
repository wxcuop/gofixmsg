package network_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/network"
)

// Helper to generate a self-signed cert and key for testing
func generateTestCert(t *testing.T, dir string) (certFile, keyFile string) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	certFile = filepath.Join(dir, "cert.pem")
	certOut, err := os.Create(certFile)
	require.NoError(t, err)
	defer certOut.Close()
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	require.NoError(t, err)

	keyFile = filepath.Join(dir, "key.pem")
	keyOut, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	require.NoError(t, err)
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	err = pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	require.NoError(t, err)

	return certFile, keyFile
}

func TestLoadTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	certFile, keyFile := generateTestCert(t, tmpDir)

	t.Run("ValidNoCA", func(t *testing.T) {
		cfg, err := network.LoadTLSConfig(certFile, keyFile, "")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Len(t, cfg.Certificates, 1)
	})

	t.Run("ValidWithCA", func(t *testing.T) {
		// Use the same cert as CA for simplicity in testing logic path
		cfg, err := network.LoadTLSConfig(certFile, keyFile, certFile)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.RootCAs)
		require.NotNil(t, cfg.ClientCAs)
		require.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	})

	t.Run("MissingCert", func(t *testing.T) {
		cfg, err := network.LoadTLSConfig("/nonexistent/cert.pem", keyFile, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load certificate/key pair")
		require.Nil(t, cfg)
	})

	t.Run("MissingCA", func(t *testing.T) {
		cfg, err := network.LoadTLSConfig(certFile, keyFile, "/nonexistent/ca.pem")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read CA certificate file")
		require.Nil(t, cfg)
	})

	t.Run("InvalidCA", func(t *testing.T) {
		invalidCA := filepath.Join(tmpDir, "invalid_ca.pem")
		err := os.WriteFile(invalidCA, []byte("NOT A PEM CERTIFICATE"), 0644)
		require.NoError(t, err)

		cfg, err := network.LoadTLSConfig(certFile, keyFile, invalidCA)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse CA certificate")
		require.Nil(t, cfg)
	})
}

func TestLoadTLSConfigNoFiles(t *testing.T) {
	cfg, err := network.LoadTLSConfig("", "", "")
	require.NoError(t, err)
	require.Nil(t, cfg)
}
