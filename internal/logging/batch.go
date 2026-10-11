package logging

import (
	"sync"
	"time"
)

const (
	defaultBatchBytes = 1 << 20
	defaultBatchWait  = 2 * time.Second
)

type batchLine struct {
	ts   int64
	text string
}

type group struct {
	labels map[string]string
	lines  []batchLine
}

// Item is one line waiting to be batched.
type Item struct {
	ContainerID string
	Cursor      Cursor
	Labels      map[string]string
	TS          int64
	Text        string
}

// Batch is one Loki push body and the cursors it covers.
type Batch struct {
	Payload []byte
	Cursors map[string]Cursor
}

// Batcher groups lines until the payload reaches maxBytes or the wait elapses.
type Batcher struct {
	mu      sync.Mutex
	max     int
	wait    time.Duration
	groups  []*group
	index   map[string]int
	cursors map[string]Cursor
	first   time.Time
}

func newBatcher(maxBytes int, wait time.Duration) *Batcher {
	if maxBytes <= 0 {
		maxBytes = defaultBatchBytes
	}
	if wait <= 0 {
		wait = defaultBatchWait
	}
	return &Batcher{
		max:     maxBytes,
		wait:    wait,
		index:   map[string]int{},
		cursors: map[string]Cursor{},
	}
}

// Add appends a line. When the current batch cannot fit it, Add returns that batch and starts a new one.
func (b *Batcher) Add(now time.Time, item Item) *Batch {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.groups) == 0 {
		b.first = now
		b.appendLocked(item)
		return nil
	}
	saved := cloneCursors(b.cursors)
	b.appendLocked(item)
	if b.sizeLocked() <= b.max {
		return nil
	}
	b.popLine(item)
	b.cursors = saved
	ready := b.takeLocked()
	b.first = now
	b.appendLocked(item)
	return ready
}

// FlushDue returns the batch once it has been open for the wait, otherwise nil.
func (b *Batcher) FlushDue(now time.Time) *Batch {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.groups) == 0 || b.first.IsZero() || now.Sub(b.first) < b.wait {
		return nil
	}
	return b.takeLocked()
}

// Flush returns whatever is buffered.
func (b *Batcher) Flush() *Batch {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.groups) == 0 {
		return nil
	}
	return b.takeLocked()
}

func (b *Batcher) appendLocked(item Item) {
	key := labelKey(item.Labels)
	i, ok := b.index[key]
	if !ok {
		i = len(b.groups)
		b.index[key] = i
		b.groups = append(b.groups, &group{labels: cloneLabels(item.Labels)})
	}
	b.groups[i].lines = append(b.groups[i].lines, batchLine{ts: item.TS, text: item.Text})
	if item.ContainerID != "" {
		b.cursors[item.ContainerID] = Later(b.cursors[item.ContainerID], item.Cursor)
	}
}

func (b *Batcher) takeLocked() *Batch {
	payload, err := encodeStreams(b.groups)
	groups := b.groups
	cursors := b.cursors
	b.groups = nil
	b.index = map[string]int{}
	b.cursors = map[string]Cursor{}
	b.first = time.Time{}
	if err != nil || len(groups) == 0 {
		return nil
	}
	return &Batch{Payload: payload, Cursors: cursors}
}

func (b *Batcher) popLine(item Item) {
	key := labelKey(item.Labels)
	i, ok := b.index[key]
	if !ok {
		return
	}
	g := b.groups[i]
	if len(g.lines) > 0 {
		g.lines = g.lines[:len(g.lines)-1]
	}
	if len(g.lines) > 0 {
		return
	}
	b.groups = append(b.groups[:i], b.groups[i+1:]...)
	b.index = map[string]int{}
	for n, group := range b.groups {
		b.index[labelKey(group.labels)] = n
	}
}

func (b *Batcher) sizeLocked() int {
	payload, err := encodeStreams(b.groups)
	if err != nil {
		return 0
	}
	return len(payload)
}

func cloneCursors(in map[string]Cursor) map[string]Cursor {
	out := make(map[string]Cursor, len(in))
	for id, cur := range in {
		out[id] = cur
	}
	return out
}

func cloneLabels(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
