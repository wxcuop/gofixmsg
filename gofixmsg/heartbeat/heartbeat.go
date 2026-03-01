package heartbeat

import (
	"context"
	"sync"
	"time"
)

// Heartbeat sends ticks to the Receiver function at Interval.
type Heartbeat struct {
	Interval time.Duration
	recv     func()
	mu       sync.Mutex
	cancel   context.CancelFunc
}

func New(interval time.Duration, recv func()) *Heartbeat {
	return &Heartbeat{Interval: interval, recv: recv}
}

func (h *Heartbeat) Start(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.Interval <= 0 {
		h.Interval = time.Second
	}
	cctx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	go func() {
		t := time.NewTicker(h.Interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if h.recv != nil {
					h.recv()
				}
			case <-cctx.Done():
				return
			}
		}
	}()
}

func (h *Heartbeat) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancel != nil {
		h.cancel()
	}
}
