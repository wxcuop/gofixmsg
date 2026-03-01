package scheduler

import (
"encoding/json"
"log"
"sync"
"time"

"github.com/wxcuop/gofixmsg/config"
)

// ScheduleTask represents a single scheduled task from config
type ScheduleTask struct {
Time   string `json:"time"`   // HH:MM format
Action string `json:"action"` // start, stop/logout, reset, reset_start
}

// RuntimeScheduler manages config-driven scheduled actions
type RuntimeScheduler struct {
tasks    []ScheduleTask
mu       sync.RWMutex
done     chan struct{}
running  bool
handlers map[string]func() // action handlers: start, stop, reset, reset_start
}

// NewRuntimeScheduler creates a new scheduler (not started)
func NewRuntimeScheduler() *RuntimeScheduler {
return &RuntimeScheduler{
done:     make(chan struct{}),
handlers: make(map[string]func()),
}
}

// Load parses [Scheduler] schedules JSON from ConfigManager
func (s *RuntimeScheduler) Load(mgr *config.Manager) error {
schedulesJSON := mgr.Get("Scheduler", "schedules")
if schedulesJSON == "" {
s.mu.Lock()
s.tasks = []ScheduleTask{}
s.mu.Unlock()
return nil
}

var tasks []ScheduleTask
if err := json.Unmarshal([]byte(schedulesJSON), &tasks); err != nil {
return err
}

s.mu.Lock()
s.tasks = tasks
s.mu.Unlock()
return nil
}

// RegisterAction registers a handler for an action name
func (s *RuntimeScheduler) RegisterAction(name string, fn func()) {
s.mu.Lock()
defer s.mu.Unlock()
s.handlers[name] = fn
}

// Start begins the scheduler loop (runs in a goroutine)
func (s *RuntimeScheduler) Start() {
s.mu.Lock()
if s.running {
s.mu.Unlock()
return
}
s.running = true
s.mu.Unlock()

go s.run()
}

// Stop halts the scheduler
func (s *RuntimeScheduler) Stop() {
s.mu.Lock()
if !s.running {
s.mu.Unlock()
return
}
s.running = false
s.mu.Unlock()
close(s.done)
}

// run is the main scheduler loop (runs once per minute)
func (s *RuntimeScheduler) run() {
ticker := time.NewTicker(1 * time.Minute)
defer ticker.Stop()

for {
select {
case <-s.done:
return
case <-ticker.C:
s.checkAndExecute()
}
}
}

// checkAndExecute checks if any scheduled tasks should run now
func (s *RuntimeScheduler) checkAndExecute() {
s.mu.RLock()
tasks := s.tasks
handlers := s.handlers
s.mu.RUnlock()

	now := time.Now()
	nowMinute := now.Truncate(time.Minute)

for _, task := range tasks {
// Parse task time (HH:MM format)
t, err := time.Parse("15:04", task.Time)
if err != nil {
log.Printf("scheduler: invalid time format in task: %v", task)
continue
}

// Get task time for today
taskTime := time.Date(now.Year(), now.Month(), now.Day(),
t.Hour(), t.Minute(), 0, 0, now.Location())

		// Check if task is due in this minute or the immediately previous minute.
		// This tolerates scheduler tick jitter and HH:MM second truncation.
		delta := nowMinute.Sub(taskTime)
		if delta >= 0 && delta <= time.Minute {
handler, ok := handlers[task.Action]
if !ok {
log.Printf("scheduler: unknown action: %s", task.Action)
continue
}

// Execute handler
go func(f func()) {
defer func() {
if r := recover(); r != nil {
log.Printf("scheduler: handler panicked: %v", r)
}
}()
f()
}(handler)
}
}
}

// GetTasks returns a copy of currently loaded tasks (for testing)
func (s *RuntimeScheduler) GetTasks() []ScheduleTask {
s.mu.RLock()
defer s.mu.RUnlock()
tasks := make([]ScheduleTask, len(s.tasks))
copy(tasks, s.tasks)
return tasks
}

// ScheduleAfter runs f after duration d in a new goroutine (original simple scheduler)
func ScheduleAfter(d time.Duration, f func()) {
go func() { time.Sleep(d); f() }()
}
