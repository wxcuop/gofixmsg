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
		outSeq, inSeq, err := st.GetSessionSeq(sessionID)
		if err == nil {
			if outSeq > 0 {
				m.out = outSeq
			}
			if inSeq > 0 {
				m.in = inSeq
			}
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

func (s *SeqManager) SetIncoming(n int) error {
	s.mu.Lock()
	s.in = n
	vIn, vOut := s.in, s.out
	s.mu.Unlock()
	if s.store != nil {
		if err := s.store.SaveSessionSeq(s.sid, vOut, vIn); err != nil {
			return fmt.Errorf("seqmgr: persist in seq: %w", err)
		}
	}
	return nil
}

func (s *SeqManager) SetOutgoing(n int) error {
	s.mu.Lock()
	s.out = n
	vIn, vOut := s.in, s.out
	s.mu.Unlock()
	if s.store != nil {
		if err := s.store.SaveSessionSeq(s.sid, vOut, vIn); err != nil {
			return fmt.Errorf("seqmgr: persist out seq: %w", err)
		}
	}
	return nil
}

func (s *SeqManager) IncrementIncoming() (int, error) {
	s.mu.Lock()
	s.in++
	vIn, vOut := s.in, s.out
	s.mu.Unlock()
	if s.store != nil {
		if err := s.store.SaveSessionSeq(s.sid, vOut, vIn); err != nil {
			return vIn, fmt.Errorf("seqmgr: persist in seq: %w", err)
		}
	}
	return vIn, nil
}

func (s *SeqManager) IncrementOutgoing() (int, error) {
	s.mu.Lock()
	s.out++
	vIn, vOut := s.in, s.out
	s.mu.Unlock()
	if s.store != nil {
		if err := s.store.SaveSessionSeq(s.sid, vOut, vIn); err != nil {
			return vOut, fmt.Errorf("seqmgr: persist out seq: %w", err)
		}
	}
	return vOut, nil
}
