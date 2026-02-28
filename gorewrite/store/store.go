package store

import "time"

// Message represents a persisted FIX message.
type Message struct {
BeginString  string
SenderCompID string
TargetCompID string
MsgSeqNum    int
MsgType      string
Body         []byte
Created      time.Time
}

// Store defines the persistent message store interface used by the engine.
// Implementations must be safe for concurrent use by multiple goroutines.
type Store interface {
// Init opens / creates the data store at path.
Init(path string) error
// SaveMessage persists a single message.
SaveMessage(m *Message) error
// GetMessage retrieves a previously saved message by (beginstring, sender, target, seq).
GetMessage(begin, sender, target string, seq int) (*Message, error)
// SaveSessionSeq stores the last known sequence number for a session id.
SaveSessionSeq(sessionID string, seq int) error
// GetSessionSeq returns the last stored sequence number for a session id (0 if none).
GetSessionSeq(sessionID string) (int, error)
}
