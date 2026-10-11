package logging

import (
	"encoding/json"
	"os"
	"sync"
)

const defaultSpoolBytes = 32 << 20

// Record is one push body waiting on disk.
type Record struct {
	Payload json.RawMessage   `json:"payload"`
	Cursors map[string]Cursor `json:"cursors"`
}

type spool struct {
	mu      sync.Mutex
	path    string
	max     int
	recs    []Record
	dropped int64
}

func newSpool(path string, maxBytes int) *spool {
	if maxBytes <= 0 {
		maxBytes = defaultSpoolBytes
	}
	return &spool{path: path, max: maxBytes}
}

func (s *spool) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	var recs []Record
	if err := json.Unmarshal(raw, &recs); err != nil {
		return err
	}
	s.recs = recs
	return nil
}

func (s *spool) Snapshot() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, len(s.recs))
	copy(out, s.recs)
	return out
}

// Append stores a record. When the spool is over its cap, the oldest records are returned so their cursors can be committed.
func (s *spool) Append(rec Record) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var dropped []Record
	for len(s.recs) > 0 && s.sizeLocked()+len(rec.Payload) > s.max {
		dropped = append(dropped, s.recs[0])
		s.recs = s.recs[1:]
		s.dropped++
	}
	if len(rec.Payload) > s.max {
		s.dropped++
		dropped = append(dropped, rec)
		return dropped, s.flushLocked()
	}
	s.recs = append(s.recs, rec)
	return dropped, s.flushLocked()
}

func (s *spool) Peek() (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.recs) == 0 {
		return Record{}, false
	}
	return s.recs[0], true
}

func (s *spool) Ack() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.recs) == 0 {
		return nil
	}
	s.recs = s.recs[1:]
	return s.flushLocked()
}

func (s *spool) Size() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int64(s.sizeLocked())
}

func (s *spool) sizeLocked() int {
	n := 0
	for _, rec := range s.recs {
		n += len(rec.Payload)
	}
	return n
}

func (s *spool) flushLocked() error {
	if s.path == "" {
		return nil
	}
	raw, err := json.Marshal(s.recs)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
