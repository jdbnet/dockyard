package logging

import (
	"context"
	"sync"

	"github.com/jdbnet/dockyard/internal/docker"
)

type dockerSource struct {
	client  *docker.Client
	mu      sync.Mutex
	drivers map[string]string
}

// NewDockerSource follows containers through the local Docker client.
func NewDockerSource(client *docker.Client) Source {
	return &dockerSource{client: client, drivers: map[string]string{}}
}

func (d *dockerSource) Targets(ctx context.Context) ([]Target, error) {
	list, err := d.client.ListContainers(ctx, true)
	if err != nil {
		return nil, err
	}
	out := make([]Target, 0, len(list))
	for _, ctr := range list {
		if ctr.State != "running" {
			continue
		}
		out = append(out, Target{
			ID:             ctr.ID,
			Name:           ctr.Name,
			Image:          ctr.Image,
			ComposeProject: ctr.ComposeProject,
			ComposeService: ctr.ComposeService,
			Labels:         ctr.Labels,
			LogDriver:      d.logDriver(ctx, ctr.ID),
			State:          ctr.State,
		})
	}
	return out, nil
}

func (d *dockerSource) logDriver(ctx context.Context, id string) string {
	d.mu.Lock()
	if driver, ok := d.drivers[id]; ok {
		d.mu.Unlock()
		return driver
	}
	d.mu.Unlock()

	insp, err := d.client.InspectContainer(ctx, id)
	if err != nil || insp == nil {
		return ""
	}
	driver := insp.Summary.LogDriver
	if driver == "" {
		return ""
	}
	d.mu.Lock()
	d.drivers[id] = driver
	d.mu.Unlock()
	return driver
}

func (d *dockerSource) Follow(ctx context.Context, id, since string, emit func(Entry) error) error {
	return d.client.StreamLogEntries(ctx, id, docker.LogOptions{Follow: true, Since: since}, func(entry docker.LogEntry) error {
		return emit(Entry{Stream: entry.Stream, Line: entry.Line})
	})
}
