package engine_test

import (
"testing"

"github.com/stretchr/testify/require"
"github.com/wxcuop/pyfixmsg_plus/engine"
)

// TestMultiSessionEngine_Creation verifies the multi-session engine can be created.
func TestMultiSessionEngine_Creation(t *testing.T) {
mse := engine.NewMultiSessionEngine("127.0.0.1:0")
require.NotNil(t, mse)
require.NotNil(t, mse.Acceptor)
require.Equal(t, 0, mse.SessionCount())
}

// TestMultiSessionEngine_Start verifies acceptor can start and stop.
func TestMultiSessionEngine_Start(t *testing.T) {
mse := engine.NewMultiSessionEngine("127.0.0.1:0")
require.NoError(t, mse.Start())

addr := mse.Acceptor.AddrString()
require.NotEmpty(t, addr)

require.NoError(t, mse.Stop())
}

// TestMultiSessionEngine_GetSession verifies session retrieval works for non-existent sessions.
func TestMultiSessionEngine_GetSession(t *testing.T) {
mse := engine.NewMultiSessionEngine("127.0.0.1:0")
require.NoError(t, mse.Start())
defer mse.Stop()

// Non-existent session
sess, exists := mse.GetSession("nonexistent")
require.False(t, exists)
require.Nil(t, sess)
}

// TestMultiSessionEngine_SessionIDs verifies session ID listing.
func TestMultiSessionEngine_SessionIDs(t *testing.T) {
mse := engine.NewMultiSessionEngine("127.0.0.1:0")
require.NoError(t, mse.Start())
defer mse.Stop()

// No sessions initially
ids := mse.SessionIDs()
require.Empty(t, ids)
}
