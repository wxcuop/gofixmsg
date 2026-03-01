package idgen_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/idgen"
)

func TestNumericEncodeDecode(t *testing.T) {
	g := idgen.NewNumericGenerator()
	s := g.Encode(12345)
	n, err := g.Decode(s)
	require.NoError(t, err)
	require.Equal(t, 12345, n)
}

func TestYMDEncodeDecode(t *testing.T) {
	fixed := time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC)
	g := idgen.NewYMDGenerator(func() time.Time { return fixed })
	s := g.Encode(42)
	require.Len(t, s, 8+g.Width)
	n, err := g.Decode(s)
	require.NoError(t, err)
	require.Equal(t, 42, n)
}
