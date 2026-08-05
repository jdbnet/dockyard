package engine

import (
	"context"
	"time"

	"github.com/docker/docker/api/types/events"
)

func (e *Engine) handleDockerEvent(ctx context.Context, ev events.Message) {
	id := ev.Actor.ID
	action := string(ev.Action)
	evTime := time.Unix(ev.Time, 0)

	switch ev.Type {
	case events.ContainerEventType:
		if action == "start" {
			if e.tracker.record(id, evTime) {
				e.broadcast(Event{
					Type: "container", Action: "restart_loop", Resource: id,
					Message: "restart loop detected", Timestamp: evTime,
				})
			}
		}
		if err := e.refreshContainers(ctx); err != nil {
			return
		}
		e.broadcast(Event{
			Type: "container", Action: action, Resource: id,
			Timestamp: evTime,
		})
	case events.ImageEventType:
		_ = e.refreshImages(ctx)
		e.broadcast(Event{Type: "image", Action: action, Resource: id, Timestamp: evTime})
	case events.NetworkEventType:
		_ = e.refreshNetworks(ctx)
		e.broadcast(Event{Type: "network", Action: action, Resource: id, Timestamp: evTime})
	case events.VolumeEventType:
		_ = e.refreshVolumes(ctx)
		e.broadcast(Event{Type: "volume", Action: action, Resource: id, Timestamp: evTime})
	}
}

func (e *Engine) watchEventsLoop(ctx context.Context) {
	msgCh, errCh := e.docker.Events(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				time.Sleep(time.Second)
				msgCh, errCh = e.docker.Events(ctx)
				continue
			}
			return
		case ev, ok := <-msgCh:
			if !ok {
				return
			}
			e.handleDockerEvent(ctx, ev)
		}
	}
}
