package state

import "sync"

// Predefined state names
const (
StateDisconnected    = "Disconnected"
StateConnecting      = "Connecting"
StateLogonInProgress = "LogonInProgress"
StateAwaitingLogon   = "AwaitingLogon"
StateActive          = "Active"
StateLogoutInProgress = "LogoutInProgress"
StateReconnecting    = "Reconnecting"
)

// StateMachine manages session state transitions.
type StateMachine struct {
mu    sync.Mutex
state string
subs  []func(string)
}

// NewStateMachine returns a StateMachine starting in Disconnected.
func NewStateMachine() *StateMachine {
return &StateMachine{state: StateDisconnected}
}

// State returns the current state.
func (s *StateMachine) State() string {
s.mu.Lock()
defer s.mu.Unlock()
return s.state
}

// Register adds a subscriber callback that is invoked whenever the state changes.
func (s *StateMachine) Register(cb func(string)) {
s.mu.Lock()
defer s.mu.Unlock()
s.subs = append(s.subs, cb)
}

// OnEvent applies an event string and returns the new state. If no transition
// is defined for the current state+event, the state remains unchanged.
func (s *StateMachine) OnEvent(event string) string {
s.mu.Lock()
cur := s.state
next := cur
// per-state transitions
switch cur {
case StateDisconnected:
if event == "connect" {
next = StateConnecting
}
case StateConnecting:
if event == "connected" {
next = StateAwaitingLogon
}
case StateAwaitingLogon:
if event == "logon_sent" {
next = StateLogonInProgress
} else if event == "logon_received" {
next = StateActive
}
case StateLogonInProgress:
if event == "logon_received" {
next = StateActive
} else if event == "logon_failed" {
next = StateReconnecting
}
case StateActive:
if event == "logout" {
next = StateLogoutInProgress
} else if event == "disconnect" {
next = StateReconnecting
}
case StateLogoutInProgress:
if event == "logout_complete" || event == "logout_received" {
next = StateDisconnected
}
case StateReconnecting:
if event == "connected" {
next = StateAwaitingLogon
} else if event == "giveup" {
next = StateDisconnected
}
}
// global events
if event == "disconnected" {
next = StateDisconnected
}

if next != cur {
s.state = next
subs := append([]func(string){}, s.subs...)
s.mu.Unlock()
// notify without holding lock
for _, cb := range subs {
go cb(next)
}
return next
}
s.mu.Unlock()
return cur
}
