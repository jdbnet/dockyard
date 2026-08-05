package engine

import "sync"

type ringBuffer struct {
	capacity int
	points   []StatPoint
	head     int
	size     int
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{capacity: capacity, points: make([]StatPoint, capacity)}
}

func (r *ringBuffer) add(p StatPoint) {
	r.points[r.head] = p
	r.head = (r.head + 1) % r.capacity
	if r.size < r.capacity {
		r.size++
	}
}

func (r *ringBuffer) snapshot() []StatPoint {
	if r.size == 0 {
		return nil
	}
	out := make([]StatPoint, r.size)
	if r.size < r.capacity {
		copy(out, r.points[:r.size])
		return out
	}
	copy(out, r.points[r.head:])
	copy(out[r.capacity-r.head:], r.points[:r.head])
	return out
}

func (r *ringBuffer) latest() (StatPoint, bool) {
	if r.size == 0 {
		return StatPoint{}, false
	}
	idx := (r.head - 1 + r.capacity) % r.capacity
	return r.points[idx], true
}

type statsCache struct {
	mu      sync.RWMutex
	buffers map[string]*ringBuffer
}

func newStatsCache() *statsCache {
	return &statsCache{buffers: make(map[string]*ringBuffer)}
}

func (s *statsCache) ensure(id string, capacity int) *ringBuffer {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.buffers[id]; ok {
		return b
	}
	b := newRingBuffer(capacity)
	s.buffers[id] = b
	return b
}

func (s *statsCache) add(id string, capacity int, p StatPoint) {
	b := s.ensure(id, capacity)
	s.mu.Lock()
	b.add(p)
	s.mu.Unlock()
}

func (s *statsCache) series(id string) StatsSeries {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.buffers[id]
	if !ok {
		return StatsSeries{ContainerID: id}
	}
	return StatsSeries{ContainerID: id, Points: b.snapshot()}
}

func (s *statsCache) latest(id string) (StatPoint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.buffers[id]
	if !ok {
		return StatPoint{}, false
	}
	return b.latest()
}

func (s *statsCache) remove(id string) {
	s.mu.Lock()
	delete(s.buffers, id)
	s.mu.Unlock()
}
