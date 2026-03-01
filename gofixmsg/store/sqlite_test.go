package store_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/store"
)

func TestSQLiteStore_SaveGet(t *testing.T) {
	f, err := os.CreateTemp("", "fixstore-*.db")
	require.NoError(t, err)
	_ = f.Close()
	defer os.Remove(f.Name())

	st := store.NewSQLiteStore()
	require.NoError(t, st.Init(f.Name()))

	m := &store.Message{
		BeginString:  "FIX.4.4",
		SenderCompID: "S",
		TargetCompID: "T",
		MsgSeqNum:    1,
		MsgType:      "D",
		Body:         []byte("8=FIX.4.4\x0135=D\x01"),
		Created:      time.Now(),
	}
	require.NoError(t, st.SaveMessage(m))

	got, err := st.GetMessage("FIX.4.4", "S", "T", 1)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, m.MsgType, got.MsgType)
	require.Equal(t, m.MsgSeqNum, got.MsgSeqNum)

	require.NoError(t, st.SaveSessionSeq("S-T", 42, 10))
	outSeq, inSeq, err := st.GetSessionSeq("S-T")
	require.NoError(t, err)
	require.Equal(t, 42, outSeq)
	require.Equal(t, 10, inSeq)
}
