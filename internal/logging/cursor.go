package logging

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"
)

// Cursor is the last line shipped for one container.
// Count is how many lines at Timestamp have already been shipped.
type Cursor struct {
	Timestamp int64 `json:"timestamp"`
	Count     int   `json:"count"`
}

// Resume drops lines that have already been shipped.
type Resume struct {
	cur  Cursor
	seen int
}

func NewResume(cur Cursor) *Resume {
	return &Resume{cur: cur}
}

// Keep reports whether this timestamp should be shipped, and advances the cursor when it is.
func (r *Resume) Keep(ts int64) bool {
	if r.cur.Timestamp > 0 && ts < r.cur.Timestamp {
		return false
	}
	if r.cur.Timestamp > 0 && ts == r.cur.Timestamp {
		r.seen++
		if r.seen <= r.cur.Count {
			return false
		}
		r.cur.Count++
		return true
	}
	r.cur.Timestamp = ts
	r.cur.Count = 1
	r.seen = 1
	return true
}

func (r *Resume) Cursor() Cursor {
	return r.cur
}

// Later returns the cursor that is further through the log.
func Later(a, b Cursor) Cursor {
	if b.Timestamp > a.Timestamp || (b.Timestamp == a.Timestamp && b.Count > a.Count) {
		return b
	}
	return a
}

// SinceString is a Docker Logs Since value just before the cursor, so the follow overlaps and the filter can drop duplicates.
func SinceString(cur Cursor) string {
	if cur.Timestamp <= 0 {
		return ""
	}
	t := time.Unix(0, cur.Timestamp).Add(-time.Nanosecond).UTC()
	return t.Format(time.RFC3339Nano)
}

// ParseDockerLine splits a timestamped Docker log line.
func ParseDockerLine(line string) (int64, string, bool) {
	ts, text, ok := strings.Cut(line, " ")
	if !ok || ts == "" {
		return 0, line, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return 0, line, false
	}
	return parsed.UnixNano(), text, true
}

type cursorStore struct {
	mu   sync.Mutex
	path string
	mem  map[string]Cursor
	disk map[string]Cursor
}

func newCursorStore(path string) *cursorStore {
	return &cursorStore{
		path: path,
		mem:  map[string]Cursor{},
		disk: map[string]Cursor{},
	}
}

func (s *cursorStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var disk map[string]Cursor
	if err := json.Unmarshal(raw, &disk); err != nil {
		return err
	}
	if disk == nil {
		disk = map[string]Cursor{}
	}
	s.disk = disk
	s.mem = map[string]Cursor{}
	for id, cur := range disk {
		s.mem[id] = cur
	}
	return nil
}

func (s *cursorStore) Mem(id string) Cursor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mem[id]
}

func (s *cursorStore) SetMem(id string, cur Cursor) {
	s.mu.Lock()
	s.mem[id] = Later(s.mem[id], cur)
	s.mu.Unlock()
}

func (s *cursorStore) Overlay(records []Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range records {
		for id, cur := range rec.Cursors {
			s.mem[id] = Later(s.mem[id], cur)
		}
	}
}

// Commit persists cursors that will not be replayed from the spool.
func (s *cursorStore) Commit(cursors map[string]Cursor) error {
	if len(cursors) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, cur := range cursors {
		s.mem[id] = Later(s.mem[id], cur)
		s.disk[id] = Later(s.disk[id], cur)
	}
	return s.writeLocked()
}

func (s *cursorStore) writeLocked() error {
	if s.path == "" {
		return nil
	}
	raw, err := json.MarshalIndent(s.disk, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
