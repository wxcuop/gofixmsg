package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wxcuop/pyfixmsg_plus/fixmsg"
)

// HeartbeatMonitor monitors inbound traffic and sends TestRequest when peer misses heartbeats.
type HeartbeatMonitor struct {
	mu             sync.Mutex
	Interval       time.Duration
	TestReqTimeout time.Duration
	engine         *FixEngine
	lastSeen       time.Time
	testSent       time.Time
	outstanding    bool
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

func NewHeartbeatMonitor(e *FixEngine, interval, testReqTimeout time.Duration) *HeartbeatMonitor {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if testReqTimeout <= 0 {
		testReqTimeout = interval
	}
	return &HeartbeatMonitor{Interval: interval, TestReqTimeout: testReqTimeout, engine: e}
}

func (h *HeartbeatMonitor) Seen() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastSeen = time.Now()
	h.outstanding = false
}

func (h *HeartbeatMonitor) Start(ctx context.Context) {
	h.mu.Lock()
	if h.ctx != nil {
		h.mu.Unlock()
		return
	}
	h.ctx, h.cancel = context.WithCancel(ctx)
	h.lastSeen = time.Now()
	h.mu.Unlock()

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		t := time.NewTicker(h.Interval)
		defer t.Stop()
		for {
			select {
			case <-h.ctx.Done():
				return
			case <-t.C:
				h.mu.Lock()
				ls := h.lastSeen
				out := h.outstanding
				h.mu.Unlock()
				if time.Since(ls) > h.Interval {
					if !out {
						// send TestRequest
						id := fmt.Sprintf("TR-%d", time.Now().UnixNano())
						// determine comp ids from engine if available
						sender := "S"
						target := "T"
						if h.engine != nil {
							if h.engine.SenderCompID != "" {
								sender = h.engine.SenderCompID
							}
							if h.engine.TargetCompID != "" {
								target = h.engine.TargetCompID
							}
						}

						tr := fixmsg.NewFixMessageFromMap(map[int]string{
							8:   "FIX.4.4",
							35:  "1",
							49:  sender,
							56:  target,
							112: id,
						})
						tr.SetLenAndChecksum()
						if h.engine != nil {
							if err := h.engine.SendMessage(tr); err != nil {
								h.engine.Logger.Error("failed to send test request", "error", err)
							}
						}
						h.mu.Lock()
						h.testSent = time.Now()
						h.outstanding = true
						h.mu.Unlock()
					} else {
						// already outstanding — check timeout
						h.mu.Lock()
						ts := h.testSent
						h.mu.Unlock()
						if time.Since(ts) > h.TestReqTimeout {
							// timeout exceeded: close session
							if h.engine != nil && h.engine.Session != nil {
								h.engine.Session.Stop()
							}
						}
					}
				}
			}
		}
	}()
}

func (h *HeartbeatMonitor) Stop() {
	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
	}
	h.mu.Unlock()
	h.wg.Wait()
}
