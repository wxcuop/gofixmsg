package engine_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/engine"
	"github.com/wxcuop/pyfixmsg_plus/store"
)

func TestSeqManager_Basic(t *testing.T) {
	st := store.NewSQLiteStore()
	f := t.TempDir() + "/seq.db"
	require.NoError(t, st.Init(f))

	m := engine.NewSeqManager(st, "S-T-127.0.0.1:1")
	// initial out may be zero
	v, err := m.IncrementOutgoing()
	require.NoError(t, err)
	require.Equal(t, 1, v)
	v2 := m.IncrementIncoming()
	require.Equal(t, 1, v2)

	// set outgoing and persist
	require.NoError(t, m.SetOutgoing(42))
	require.Equal(t, 42, m.Outgoing())
}
