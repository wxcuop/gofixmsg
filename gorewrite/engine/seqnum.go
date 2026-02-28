package engine

import (
	"fmt"
	"sync"

	"github.com/wxcuop/pyfixmsg_plus/store"
)

// SeqManager manages incoming and outgoing sequence numbers and persists them to store.
type SeqManager struct {
	mu    sync.Mutex
	in    int
	out   int
	store store.Store
	sid   string // session id used for storing seqs
}

func NewSeqManager(st store.Store, sessionID string) *SeqManager {
	m := &SeqManager{store: st, sid: sessionID}
	// load from store if present
	if st != nil {
		seq, err := st.GetSessionSeq(sessionID)
		if err == nil && seq > 0 {
			m.out = seq
		}
	}
	return m
}

func (s *SeqManager) Incoming() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.in
}

func (s *SeqManager) Outgoing() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out
}

func (s *SeqManager) SetIncoming(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.in = n
}

func (s *SeqManager) SetOutgoing(n int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out = n
	if s.store != nil {
		if err := s.store.SaveSessionSeq(s.sid, s.out); err != nil {
			return fmt.Errorf("seqmgr: persist out seq: %w", err)
		}
	}
	return nil
}

func (s *SeqManager) IncrementIncoming() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.in++
	return s.in
}

func (s *SeqManager) IncrementOutgoing() (int, error) {
	s.mu.Lock()
	s.out++
	v := s.out
	s.mu.Unlock()
	if s.store != nil {
		if err := s.store.SaveSessionSeq(s.sid, v); err != nil {
			return v, fmt.Errorf("seqmgr: persist out seq: %w", err)
		}
	}
	return v, nil
}
