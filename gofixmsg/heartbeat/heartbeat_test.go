package heartbeat_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/gofixmsg/heartbeat"
)

func TestHeartbeat(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	count := 0
	h := heartbeat.New(10*time.Millisecond, func() {
		mu.Lock()
		count++
		mu.Unlock()
	})
	h.Start(ctx)
	defer h.Stop()
	time.Sleep(35 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, count, 2)
}
