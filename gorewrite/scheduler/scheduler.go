package scheduler

import (
"time"
)

// ScheduleAfter runs f after duration d in a new goroutine.
func ScheduleAfter(d time.Duration, f func()) {
go func(){ time.Sleep(d); f() }()
}
