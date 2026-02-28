package scheduler_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wxcuop/pyfixmsg_plus/scheduler"
)

func TestScheduleAfter(t *testing.T) {
	called := make(chan struct{}, 1)
	scheduler.ScheduleAfter(10*time.Millisecond, func() { called <- struct{}{} })
	select {
	case <-called:
		require.True(t, true)
	case <-time.After(100 * time.Millisecond):
		require.Fail(t, "scheduled function not called")
	}
}
