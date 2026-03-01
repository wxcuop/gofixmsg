package scheduler

import (
"sync"
"testing"
"time"

"github.com/stretchr/testify/assert"
)

// TestRuntimeScheduler_EmptySchedules tests handling of empty schedules
func TestRuntimeScheduler_EmptySchedules(t *testing.T) {
sched := NewRuntimeScheduler()
loaded := sched.GetTasks()
assert.Equal(t, 0, len(loaded))
}

// TestRuntimeScheduler_ManualTaskLoad tests manual task loading
func TestRuntimeScheduler_ManualTaskLoad(t *testing.T) {
sched := NewRuntimeScheduler()

// Manually set tasks
tasks := []ScheduleTask{
{Time: "09:30", Action: "start"},
{Time: "16:00", Action: "stop/logout"},
{Time: "11:00", Action: "reset"},
{Time: "14:00", Action: "reset_start"},
}
sched.mu.Lock()
sched.tasks = tasks
sched.mu.Unlock()

loaded := sched.GetTasks()
assert.Equal(t, 4, len(loaded))
assert.Equal(t, "09:30", loaded[0].Time)
assert.Equal(t, "start", loaded[0].Action)
assert.Equal(t, "16:00", loaded[1].Time)
assert.Equal(t, "stop/logout", loaded[1].Action)
assert.Equal(t, "11:00", loaded[2].Time)
assert.Equal(t, "reset", loaded[2].Action)
assert.Equal(t, "14:00", loaded[3].Time)
assert.Equal(t, "reset_start", loaded[3].Action)
}

// TestRuntimeScheduler_ActionExecution tests action dispatch
func TestRuntimeScheduler_ActionExecution(t *testing.T) {
sched := NewRuntimeScheduler()

callOrder := []string{}
mu := sync.Mutex{}

// Register handlers
sched.RegisterAction("start", func() {
mu.Lock()
callOrder = append(callOrder, "start")
mu.Unlock()
})

sched.RegisterAction("stop/logout", func() {
mu.Lock()
callOrder = append(callOrder, "stop/logout")
mu.Unlock()
})

sched.RegisterAction("reset", func() {
mu.Lock()
callOrder = append(callOrder, "reset")
mu.Unlock()
})

// Manually set tasks to execute at "now-ish" time for testing
now := time.Now()
taskTime := now.Add(-30 * time.Second) // 30 seconds ago
taskTimeStr := taskTime.Format("15:04")

sched.mu.Lock()
sched.tasks = []ScheduleTask{
{Time: taskTimeStr, Action: "start"},
{Time: taskTimeStr, Action: "stop/logout"},
{Time: taskTimeStr, Action: "reset"},
}
sched.mu.Unlock()

// Execute the check manually (simulating what run() does)
sched.checkAndExecute()

// Give handlers time to execute
time.Sleep(100 * time.Millisecond)

// Verify all handlers were called
mu.Lock()
defer mu.Unlock()
assert.Equal(t, 3, len(callOrder))
assert.Contains(t, callOrder, "start")
assert.Contains(t, callOrder, "stop/logout")
assert.Contains(t, callOrder, "reset")
}

// TestRuntimeScheduler_UnknownActionIgnored tests that unknown actions are ignored gracefully
func TestRuntimeScheduler_UnknownActionIgnored(t *testing.T) {
sched := NewRuntimeScheduler()

callCount := 0
mu := sync.Mutex{}

sched.RegisterAction("start", func() {
mu.Lock()
callCount++
mu.Unlock()
})

now := time.Now()
taskTime := now.Add(-30 * time.Second)
taskTimeStr := taskTime.Format("15:04")

sched.mu.Lock()
sched.tasks = []ScheduleTask{
{Time: taskTimeStr, Action: "unknown_action"},
{Time: taskTimeStr, Action: "start"},
}
sched.mu.Unlock()

// Execute check
sched.checkAndExecute()
time.Sleep(100 * time.Millisecond)

// Only the known action should execute
mu.Lock()
defer mu.Unlock()
assert.Equal(t, 1, callCount)
}

// TestRuntimeScheduler_ActionPanicHandling tests that panicking handlers don't crash scheduler
func TestRuntimeScheduler_ActionPanicHandling(t *testing.T) {
sched := NewRuntimeScheduler()

safeCallCount := 0
mu := sync.Mutex{}

// Register a handler that panics
sched.RegisterAction("panic_action", func() {
panic("intentional panic for testing")
})

// Register a safe handler after the panic
sched.RegisterAction("safe_action", func() {
mu.Lock()
safeCallCount++
mu.Unlock()
})

now := time.Now()
taskTime := now.Add(-30 * time.Second)
taskTimeStr := taskTime.Format("15:04")

sched.mu.Lock()
sched.tasks = []ScheduleTask{
{Time: taskTimeStr, Action: "panic_action"},
{Time: taskTimeStr, Action: "safe_action"},
}
sched.mu.Unlock()

// Execute check - should not panic
defer func() {
if r := recover(); r != nil {
t.Errorf("checkAndExecute should not propagate panic, but got: %v", r)
}
}()

sched.checkAndExecute()
time.Sleep(100 * time.Millisecond)

mu.Lock()
defer mu.Unlock()
// Panic action should not prevent safe action from running
assert.Equal(t, 1, safeCallCount, "safe action should execute despite panic in previous action")
}

// TestRuntimeScheduler_NoActionOutsideWindow tests that actions only run within the 1-minute window
func TestRuntimeScheduler_NoActionOutsideWindow(t *testing.T) {
sched := NewRuntimeScheduler()

callCount := 0
mu := sync.Mutex{}

sched.RegisterAction("test_action", func() {
mu.Lock()
callCount++
mu.Unlock()
})

// Schedule task for 2 hours from now (definitely outside window)
futureTime := time.Now().Add(2 * time.Hour)
futureTimeStr := futureTime.Format("15:04")

sched.mu.Lock()
sched.tasks = []ScheduleTask{
{Time: futureTimeStr, Action: "test_action"},
}
sched.mu.Unlock()

// Execute check
sched.checkAndExecute()
time.Sleep(50 * time.Millisecond)

mu.Lock()
defer mu.Unlock()
assert.Equal(t, 0, callCount, "action should not run outside 1-minute window")
}

// TestScheduleAfter tests the original simple scheduler function
func TestScheduleAfter(t *testing.T) {
executed := false
mu := sync.Mutex{}

ScheduleAfter(50*time.Millisecond, func() {
mu.Lock()
executed = true
mu.Unlock()
})

time.Sleep(150 * time.Millisecond)

mu.Lock()
defer mu.Unlock()
assert.True(t, executed, "ScheduleAfter should execute after delay")
}

// Compile-time check that RuntimeScheduler can be used as expected
var _ = (*RuntimeScheduler)(nil)
