package heartbeat_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/heartbeat"
)

func TestHeartbeat(t *testing.T) {
	ctx := context.Background()
	count := 0
	h := heartbeat.New(10*time.Millisecond, func() { count++ })
	h.Start(ctx)
	defer h.Stop()
	time.Sleep(35 * time.Millisecond)
	require.GreaterOrEqual(t, count, 2)
}
