package engine_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/engine"
	"github.com/wxcuop/gofixmsg/store"
)

func TestSeqManager_Basic(t *testing.T) {
	st := store.NewSQLiteStore()
	f := t.TempDir() + "/seq.db"
	require.NoError(t, st.Init(f))
	defer st.Close()

	m := engine.NewSeqManager(st, "S-T-127.0.0.1:1")
	// initial out may be zero
	v, err := m.IncrementOutgoing()
	require.NoError(t, err)
	require.Equal(t, 1, v)
	v2, err := m.IncrementIncoming()
	require.NoError(t, err)
	require.Equal(t, 1, v2)

	// set outgoing and persist
	require.NoError(t, m.SetOutgoing(42))
	require.Equal(t, 42, m.Outgoing())
}

func TestSeqManager_Persistence(t *testing.T) {
	st := store.NewSQLiteStore()
	f := t.TempDir() + "/persist.db"
	require.NoError(t, st.Init(f))
	defer st.Close()

	sid := "session-123"
	m := engine.NewSeqManager(st, sid)
	
	// Advance sequences
	require.NoError(t, m.SetOutgoing(100))
	require.NoError(t, m.SetIncoming(50))
	
	// Re-load SeqManager with same store and session ID
	m2 := engine.NewSeqManager(st, sid)
	require.Equal(t, 100, m2.Outgoing(), "Outgoing seq should be persisted")
	require.Equal(t, 50, m2.Incoming(), "Incoming seq should be persisted")
	
	// Increment and verify
	v, _ := m2.IncrementOutgoing()
	require.Equal(t, 101, v)
	v2, _ := m2.IncrementIncoming()
	require.Equal(t, 51, v2)
	
	// Re-load again
	m3 := engine.NewSeqManager(st, sid)
	require.Equal(t, 101, m3.Outgoing())
	require.Equal(t, 51, m3.Incoming())
}
