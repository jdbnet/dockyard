package engine

import (
	"fmt"
	"sync"
	"time"

	"github.com/jdbnet/dockyard/internal/docker"
)

type store struct {
	mu         sync.RWMutex
	containers []Container
	images     []Image
	volumes    []Volume
	networks   []Network
	inspect    map[string]string
}

func newStore() *store {
	return &store{inspect: make(map[string]string)}
}

func (s *store) setContainers(list []Container) {
	s.mu.Lock()
	s.containers = list
	s.mu.Unlock()
}

func (s *store) containersSnapshot() []Container {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Container, len(s.containers))
	copy(out, s.containers)
	return out
}

func (s *store) setImages(list []Image) {
	s.mu.Lock()
	s.images = list
	s.mu.Unlock()
}

func (s *store) imagesSnapshot() []Image {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Image, len(s.images))
	copy(out, s.images)
	return out
}

func (s *store) setVolumes(list []Volume) {
	s.mu.Lock()
	s.volumes = list
	s.mu.Unlock()
}

func (s *store) volumesSnapshot() []Volume {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Volume, len(s.volumes))
	copy(out, s.volumes)
	return out
}

func (s *store) setNetworks(list []Network) {
	s.mu.Lock()
	s.networks = list
	s.mu.Unlock()
}

func (s *store) networksSnapshot() []Network {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Network, len(s.networks))
	copy(out, s.networks)
	return out
}

func (s *store) setInspect(id, raw string) {
	s.mu.Lock()
	s.inspect[id] = raw
	s.mu.Unlock()
}

func (s *store) inspectSnapshot(id string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	raw, ok := s.inspect[id]
	return raw, ok
}

func mapContainerSummary(c docker.ContainerSummary, tracker *restartTracker) Container {
	uptime := ""
	if !c.StartedAt.IsZero() && c.State == "running" {
		uptime = formatUptime(time.Since(c.StartedAt))
	}
	loop := false
	if tracker != nil {
		loop = tracker.isLoop(c.ID, time.Now())
	}
	return Container{
		ID:             c.ID,
		ShortID:        c.ShortID,
		Name:           c.Name,
		Image:          c.Image,
		State:          c.State,
		Status:         c.Status,
		ComposeProject: c.ComposeProject,
		ComposeService: c.ComposeService,
		ComposeWorkDir: c.ComposeWorkDir,
		Health:         c.Health,
		StartedAt:      c.StartedAt,
		Uptime:         uptime,
		RestartCount:   c.RestartCount,
		RestartLoop:    loop,
		Ports:          c.Ports,
	}
}

func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := int(d.Hours()) / 24
	return fmt.Sprintf("%dd", days)
}
