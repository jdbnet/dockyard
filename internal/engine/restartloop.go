package engine

import (
	"sync"
	"time"
)

type restartTracker struct {
	mu        sync.Mutex
	threshold int
	window    time.Duration
	events    map[string][]time.Time
}

func newRestartTracker(threshold int, window time.Duration) *restartTracker {
	return &restartTracker{
		threshold: threshold,
		window:    window,
		events:    make(map[string][]time.Time),
	}
}

func (r *restartTracker) record(id string, t time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := t.Add(-r.window)
	list := r.events[id]
	filtered := list[:0]
	for _, ts := range list {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	filtered = append(filtered, t)
	r.events[id] = filtered
	return len(filtered) >= r.threshold
}

func (r *restartTracker) isLoop(id string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := now.Add(-r.window)
	count := 0
	for _, ts := range r.events[id] {
		if ts.After(cutoff) {
			count++
		}
	}
	return count >= r.threshold
}

func (r *restartTracker) remove(id string) {
	r.mu.Lock()
	delete(r.events, id)
	r.mu.Unlock()
}
