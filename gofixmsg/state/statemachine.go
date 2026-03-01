package state

import (
	"fmt"
	"log/slog"
	"sync"
)

// Predefined state names
const (
	StateDisconnected     = "Disconnected"
	StateConnecting       = "Connecting"
	StateLogonInProgress  = "LogonInProgress"
	StateAwaitingLogon    = "AwaitingLogon"
	StateActive           = "Active"
	StateLogoutInProgress = "LogoutInProgress"
	StateReconnecting     = "Reconnecting"
)

// Predefined events
const (
	EventConnect             = "connect"
	EventConnected           = "connected"
	EventClientAccepted      = "client_accepted"
	EventLogonSent           = "logon_sent"
	EventLogonReceived       = "logon_received"
	EventLogonFailed         = "logon_failed"
	EventLogout              = "logout"
	EventLogoutComplete      = "logout_complete"
	EventLogoutReceived      = "logout_received"
	EventDisconnect          = "disconnect"
	EventDisconnected        = "disconnected"
	EventInitiateReconnect   = "initiate_reconnect"
	EventReconnectFailedMax  = "reconnect_failed_max_retries"
	EventGiveup              = "giveup"
)

// StateMachine manages session state transitions.
type StateMachine struct {
	mu    sync.Mutex
	state string
	subs  []func(string)
	// logger for state transitions
	logger *slog.Logger
}

// NewStateMachine returns a StateMachine starting in Disconnected.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		state:  StateDisconnected,
		logger: slog.Default(),
	}
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

// SetLogger sets the logger for state transitions.
func (s *StateMachine) SetLogger(logger *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if logger != nil {
		s.logger = logger
	}
}

// OnEvent applies an event string and returns the new state or error if undefined.
// Returns error if no transition is defined for the current state+event combination.
func (s *StateMachine) OnEvent(event string) (string, error) {
	s.mu.Lock()
	cur := s.state
	next := cur
	var transitionFound bool

	// per-state transitions
	switch cur {
	case StateDisconnected:
		if event == EventConnect {
			next = StateConnecting
			transitionFound = true
		}
	case StateConnecting:
		if event == EventConnected {
			next = StateAwaitingLogon
			transitionFound = true
		} else if event == EventClientAccepted {
			next = StateAwaitingLogon
			transitionFound = true
		}
	case StateAwaitingLogon:
		if event == EventLogonSent {
			next = StateLogonInProgress
			transitionFound = true
		} else if event == EventLogonReceived {
			next = StateActive
			transitionFound = true
		}
	case StateLogonInProgress:
		if event == EventLogonReceived {
			next = StateActive
			transitionFound = true
		} else if event == EventLogonFailed {
			next = StateReconnecting
			transitionFound = true
		}
	case StateActive:
		if event == EventLogout {
			next = StateLogoutInProgress
			transitionFound = true
		} else if event == EventDisconnect {
			next = StateReconnecting
			transitionFound = true
		}
	case StateLogoutInProgress:
		if event == EventLogoutComplete || event == EventLogoutReceived {
			next = StateDisconnected
			transitionFound = true
		}
	case StateReconnecting:
		if event == EventConnected {
			next = StateAwaitingLogon
			transitionFound = true
		} else if event == EventInitiateReconnect {
			// stay in reconnecting
			next = StateReconnecting
			transitionFound = true
		} else if event == EventReconnectFailedMax {
			next = StateDisconnected
			transitionFound = true
		} else if event == EventGiveup {
			next = StateDisconnected
			transitionFound = true
		}
	}

	// global events
	if event == EventDisconnected {
		next = StateDisconnected
		transitionFound = true
	}

	if next != cur {
		s.state = next
		// log the transition at info level
		s.logger.InfoContext(nil, "state transition",
			slog.String("from", cur),
			slog.String("to", next),
			slog.String("event", event))
		subs := append([]func(string){}, s.subs...)
		s.mu.Unlock()
		// notify without holding lock
		for _, cb := range subs {
			go cb(next)
		}
		return next, nil
	}

	// log undefined or no-op transition at debug level
	if !transitionFound {
		s.logger.DebugContext(nil, "undefined state transition",
			slog.String("state", cur),
			slog.String("event", event))
		s.mu.Unlock()
		return cur, fmt.Errorf("undefined transition: state=%s, event=%s", cur, event)
	}

	s.mu.Unlock()
	return cur, nil
}
