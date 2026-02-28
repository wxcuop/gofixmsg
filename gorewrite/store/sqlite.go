package store

import (
"database/sql"
"fmt"
"time"

_ "modernc.org/sqlite"
)

// SQLiteStore implements Store backed by SQLite (modernc.org/sqlite driver).
type SQLiteStore struct {
db *sql.DB
}

func NewSQLiteStore() *SQLiteStore { return &SQLiteStore{} }

func (s *SQLiteStore) Init(path string) error {
db, err := sql.Open("sqlite", path)
if err != nil {
return fmt.Errorf("sqlite: open: %w", err)
}
// set some pragmas for reasonable defaults
if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;"); err != nil {
_ = db.Close()
return fmt.Errorf("sqlite: pragmas: %w", err)
}
// create tables
schema := `
CREATE TABLE IF NOT EXISTS messages (
  beginstring TEXT NOT NULL,
  sendercompid TEXT NOT NULL,
  targetcompid TEXT NOT NULL,
  msgseqnum INTEGER NOT NULL,
  msgtype TEXT,
  body BLOB,
  created INTEGER,
  PRIMARY KEY (beginstring, sendercompid, targetcompid, msgseqnum)
);
CREATE TABLE IF NOT EXISTS sessions (
  sessionid TEXT PRIMARY KEY,
  seq INTEGER NOT NULL
);
`
if _, err := db.Exec(schema); err != nil {
_ = db.Close()
return fmt.Errorf("sqlite: create schema: %w", err)
}

s.db = db
return nil
}

func (s *SQLiteStore) SaveMessage(m *Message) error {
if s.db == nil {
return fmt.Errorf("sqlite: not initialized")
}
_, err := s.db.Exec(`INSERT OR REPLACE INTO messages(beginstring,sendercompid,targetcompid,msgseqnum,msgtype,body,created) VALUES(?,?,?,?,?,?,?)`,
m.BeginString, m.SenderCompID, m.TargetCompID, m.MsgSeqNum, m.MsgType, m.Body, m.Created.Unix())
if err != nil {
return fmt.Errorf("sqlite: save message: %w", err)
}
return nil
}

func (s *SQLiteStore) GetMessage(begin, sender, target string, seq int) (*Message, error) {
if s.db == nil {
return nil, fmt.Errorf("sqlite: not initialized")
}
row := s.db.QueryRow(`SELECT msgtype,body,created FROM messages WHERE beginstring=? AND sendercompid=? AND targetcompid=? AND msgseqnum=?`, begin, sender, target, seq)
var msgType string
var body []byte
var createdInt int64
if err := row.Scan(&msgType, &body, &createdInt); err != nil {
if err == sql.ErrNoRows {
return nil, nil
}
return nil, fmt.Errorf("sqlite: get message: %w", err)
}
return &Message{
BeginString:  begin,
SenderCompID: sender,
TargetCompID: target,
MsgSeqNum:    seq,
MsgType:      msgType,
Body:         body,
Created:      time.Unix(createdInt, 0),
}, nil
}

func (s *SQLiteStore) SaveSessionSeq(sessionID string, seq int) error {
if s.db == nil {
return fmt.Errorf("sqlite: not initialized")
}
_, err := s.db.Exec(`INSERT OR REPLACE INTO sessions(sessionid,seq) VALUES(?,?)`, sessionID, seq)
if err != nil {
return fmt.Errorf("sqlite: save session seq: %w", err)
}
return nil
}

func (s *SQLiteStore) GetSessionSeq(sessionID string) (int, error) {
if s.db == nil {
return 0, fmt.Errorf("sqlite: not initialized")
}
row := s.db.QueryRow(`SELECT seq FROM sessions WHERE sessionid=?`, sessionID)
var seq int
if err := row.Scan(&seq); err != nil {
if err == sql.ErrNoRows {
return 0, nil
}
return 0, fmt.Errorf("sqlite: get session seq: %w", err)
}
return seq, nil
}
