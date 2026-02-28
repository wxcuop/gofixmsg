package network_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/network"
)

// TestLoadTLSConfigNoFiles returns nil when no cert file is provided.
func TestLoadTLSConfigNoFiles(t *testing.T) {
	cfg, err := network.LoadTLSConfig("", "", "")
	require.NoError(t, err)
	require.Nil(t, cfg)
}

// TestLoadTLSConfigMissingFiles returns error when cert file doesn't exist.
func TestLoadTLSConfigMissingFiles(t *testing.T) {
	cfg, err := network.LoadTLSConfig("/nonexistent/cert.pem", "/nonexistent/key.pem", "")
	require.Error(t, err)
	require.Nil(t, cfg)
	require.Contains(t, err.Error(), "failed to load certificate/key pair")
}

// TestLoadTLSConfigReturnsConfig validates that LoadTLSConfig structure is correct.
// (Full cert loading test would require actual cert files)
func TestLoadTLSConfigStructure(t *testing.T) {
	// When cert file is empty, should return nil
	cfg, err := network.LoadTLSConfig("", "", "")
	require.NoError(t, err)
	require.Nil(t, cfg)

	// When cert file is specified but doesn't exist, should error
	cfg, err = network.LoadTLSConfig("fake.pem", "fake.key", "")
	require.Error(t, err)
	require.Nil(t, cfg)
}
