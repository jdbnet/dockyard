package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"git.jdbnet.co.uk/jamie/dockyard/internal/compose"
	"git.jdbnet.co.uk/jamie/dockyard/internal/config"
	"git.jdbnet.co.uk/jamie/dockyard/internal/docker"
)

type Engine struct {
	cfg     *config.Config
	docker  *docker.Client
	stacks  *compose.Manager
	store   *store
	stats   *statsCache
	tracker *restartTracker
	subMu   sync.RWMutex
	subs    map[chan Event]struct{}
	cancel  context.CancelFunc
}

type dockerStatsJSON struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs  uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

func New(cfg *config.Config, dc *docker.Client) (*Engine, error) {
	stacks, err := compose.NewManager(cfg.Compose.StacksDir)
	if err != nil {
		return nil, fmt.Errorf("compose stacks: %w", err)
	}
	return &Engine{
		cfg:     cfg,
		docker:  dc,
		stacks:  stacks,
		store:   newStore(),
		stats:   newStatsCache(),
		tracker: newRestartTracker(cfg.RestartLoop.Threshold, cfg.RestartLoop.Window.Duration),
		subs:    make(map[chan Event]struct{}),
	}, nil
}

func (e *Engine) Run(ctx context.Context) error {
	ctx, e.cancel = context.WithCancel(ctx)
	go e.watchEventsLoop(ctx)
	go e.pollStats(ctx)
	go e.fallbackRefresh(ctx)

	if err := e.refreshAll(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

func (e *Engine) Shutdown() {
	if e.cancel != nil {
		e.cancel()
	}
}

func (e *Engine) Subscribe() <-chan Event {
	ch := make(chan Event, 64)
	e.subMu.Lock()
	e.subs[ch] = struct{}{}
	e.subMu.Unlock()
	return ch
}

func (e *Engine) Unsubscribe(ch <-chan Event) {
	e.subMu.Lock()
	if c, ok := any(ch).(chan Event); ok {
		delete(e.subs, c)
		close(c)
	}
	e.subMu.Unlock()
}

func (e *Engine) broadcast(ev Event) {
	e.subMu.RLock()
	defer e.subMu.RUnlock()
	for ch := range e.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (e *Engine) refreshAll(ctx context.Context) error {
	if err := e.refreshContainers(ctx); err != nil {
		return err
	}
	if err := e.refreshImages(ctx); err != nil {
		log.Printf("refresh images: %v", err)
	}
	if err := e.refreshVolumes(ctx); err != nil {
		log.Printf("refresh volumes: %v", err)
	}
	if err := e.refreshNetworks(ctx); err != nil {
		log.Printf("refresh networks: %v", err)
	}
	return nil
}

func (e *Engine) refreshContainers(ctx context.Context) error {
	list, err := e.docker.ListContainers(ctx, true)
	if err != nil {
		return err
	}
	out := make([]Container, 0, len(list))
	for _, c := range list {
		ctr := mapContainerSummary(c, e.tracker)
		if pt, ok := e.stats.latest(c.ID); ok {
			ctr.CPUPct = pt.CPUPct
			ctr.MemPct = pt.MemPct
			ctr.MemBytes = pt.MemBytes
		}
		if c.State == "running" {
			if insp, err := e.docker.InspectContainer(ctx, c.ID); err == nil {
				ctr.RestartCount = insp.Summary.RestartCount
				ctr.Health = insp.Summary.Health
				ctr.StartedAt = insp.Summary.StartedAt
				if !ctr.StartedAt.IsZero() {
					ctr.Uptime = formatUptime(time.Since(ctr.StartedAt))
				}
			}
		}
		out = append(out, ctr)
	}
	e.store.setContainers(out)
	e.broadcast(Event{Type: "containers", Action: "refresh", Message: "containers updated", Timestamp: time.Now()})
	return nil
}

func (e *Engine) refreshImages(ctx context.Context) error {
	list, err := e.docker.ListImages(ctx)
	if err != nil {
		return err
	}
	out := make([]Image, 0, len(list))
	for _, img := range list {
		out = append(out, Image{
			ID: img.ID, ShortID: img.ShortID, RepoTags: img.RepoTags,
			Size: img.Size, Created: img.Created, Unused: img.Unused, Containers: img.Containers,
		})
	}
	e.store.setImages(out)
	return nil
}

func (e *Engine) refreshVolumes(ctx context.Context) error {
	list, err := e.docker.ListVolumes(ctx)
	if err != nil {
		return err
	}
	out := make([]Volume, 0, len(list))
	for _, v := range list {
		out = append(out, Volume{
			Name: v.Name, Driver: v.Driver, Mountpoint: v.Mountpoint,
			Scope: v.Scope, Unused: v.Unused, CreatedAt: v.CreatedAt,
		})
	}
	e.store.setVolumes(out)
	return nil
}

func (e *Engine) refreshNetworks(ctx context.Context) error {
	list, err := e.docker.ListNetworks(ctx)
	if err != nil {
		return err
	}
	out := make([]Network, 0, len(list))
	for _, n := range list {
		out = append(out, Network{
			ID: n.ID, Name: n.Name, Driver: n.Driver,
			Scope: n.Scope, Internal: n.Internal, Containers: n.Containers,
		})
	}
	e.store.setNetworks(out)
	return nil
}

func (e *Engine) fallbackRefresh(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.Events.FallbackRefresh.Duration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.refreshAll(ctx); err != nil {
				log.Printf("fallback refresh: %v", err)
			}
		}
	}
}

func (e *Engine) pollStats(ctx context.Context) {
	ticker := time.NewTicker(e.cfg.Stats.Interval.Duration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.collectStats(ctx)
		}
	}
}

func (e *Engine) collectStats(ctx context.Context) {
	containers := e.store.containersSnapshot()
	if len(containers) == 0 {
		list, err := e.docker.ListContainers(ctx, true)
		if err != nil {
			return
		}
		for _, c := range list {
			if c.State == "running" {
				containers = append(containers, Container{ID: c.ID, State: c.State})
			}
		}
	}
	for _, c := range containers {
		if c.State != "running" {
			continue
		}
		reader, err := e.docker.ContainerStats(ctx, c.ID)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			continue
		}
		var stats dockerStatsJSON
		if err := json.Unmarshal(body, &stats); err != nil {
			continue
		}
		cpuPct := calcCPUPct(stats)
		memPct := calcMemPct(stats)
		var netRx, netTx uint64
		for _, n := range stats.Networks {
			netRx += n.RxBytes
			netTx += n.TxBytes
		}
		e.stats.add(c.ID, e.cfg.Stats.BufferSize, StatPoint{
			Timestamp: time.Now(),
			CPUPct:    cpuPct,
			MemPct:    memPct,
			MemBytes:  stats.MemoryStats.Usage,
			NetRx:     netRx,
			NetTx:     netTx,
		})
	}
	if err := e.refreshContainers(ctx); err != nil {
		log.Printf("stats refresh containers: %v", err)
	}
}

func calcCPUPct(s dockerStatsJSON) float64 {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemUsage - s.PreCPUStats.SystemUsage)
	if systemDelta <= 0 || cpuDelta < 0 {
		return 0
	}
	cpus := float64(s.CPUStats.OnlineCPUs)
	if cpus == 0 {
		cpus = 1
	}
	return (cpuDelta / systemDelta) * cpus * 100.0
}

func calcMemPct(s dockerStatsJSON) float64 {
	if s.MemoryStats.Limit > 0 {
		return float64(s.MemoryStats.Usage) / float64(s.MemoryStats.Limit) * 100.0
	}
	return 0
}

// Public API

func (e *Engine) Containers(_ context.Context) ([]Container, error) {
	return e.store.containersSnapshot(), nil
}

func (e *Engine) ComposeProjects(_ context.Context) ([]ComposeProject, error) {
	return groupComposeProjects(e.store.containersSnapshot()), nil
}

func (e *Engine) resolveContainerID(id string) string {
	for _, c := range e.store.containersSnapshot() {
		if c.ID == id || c.ShortID == id || strings.EqualFold(c.Name, id) {
			return c.ID
		}
	}
	return id
}

func (e *Engine) ContainerStats(id string) StatsSeries {
	return e.stats.series(e.resolveContainerID(id))
}

func (e *Engine) ContainerLatestStats(id string) (StatPoint, bool) {
	return e.stats.latest(e.resolveContainerID(id))
}

func (e *Engine) Images(_ context.Context) ([]Image, error) {
	return e.store.imagesSnapshot(), nil
}

func (e *Engine) Volumes(_ context.Context) ([]Volume, error) {
	return e.store.volumesSnapshot(), nil
}

func (e *Engine) Networks(_ context.Context) ([]Network, error) {
	return e.store.networksSnapshot(), nil
}

func (e *Engine) Inspect(ctx context.Context, id string) (string, error) {
	if raw, ok := e.store.inspectSnapshot(id); ok {
		return raw, nil
	}
	insp, err := e.docker.InspectContainer(ctx, id)
	if err != nil {
		return "", err
	}
	e.store.setInspect(id, insp.Raw)
	return insp.Raw, nil
}

func (e *Engine) Start(ctx context.Context, id string) error {
	if err := e.docker.StartContainer(ctx, id); err != nil {
		return err
	}
	e.broadcast(Event{Type: "container", Action: "start", Resource: id, Timestamp: time.Now()})
	return e.refreshContainers(ctx)
}

func (e *Engine) Stop(ctx context.Context, id string) error {
	if err := e.docker.StopContainer(ctx, id); err != nil {
		return err
	}
	e.broadcast(Event{Type: "container", Action: "stop", Resource: id, Timestamp: time.Now()})
	return e.refreshContainers(ctx)
}

func (e *Engine) Restart(ctx context.Context, id string) error {
	if err := e.docker.RestartContainer(ctx, id); err != nil {
		return err
	}
	e.broadcast(Event{Type: "container", Action: "restart", Resource: id, Timestamp: time.Now()})
	return e.refreshContainers(ctx)
}

func (e *Engine) Remove(ctx context.Context, id string, force bool) error {
	if err := e.docker.RemoveContainer(ctx, id, force); err != nil {
		return err
	}
	e.stats.remove(id)
	e.tracker.remove(id)
	e.broadcast(Event{Type: "container", Action: "remove", Resource: id, Timestamp: time.Now()})
	return e.refreshContainers(ctx)
}

func (e *Engine) RemoveImage(ctx context.Context, id string, force bool) error {
	if err := e.docker.RemoveImage(ctx, id, force); err != nil {
		return err
	}
	e.broadcast(Event{Type: "image", Action: "remove", Resource: id, Timestamp: time.Now()})
	return e.refreshImages(ctx)
}

type PruneImagesResult struct {
	Deleted        int      `json:"deleted"`
	SpaceReclaimed uint64   `json:"space_reclaimed"`
	Attempted      int      `json:"attempted"`
	Errors         []string `json:"errors,omitempty"`
}

func (e *Engine) PruneUnusedImages(ctx context.Context) (PruneImagesResult, error) {
	report, err := e.docker.PruneUnusedImages(ctx)
	if err != nil {
		return PruneImagesResult{}, err
	}
	if err := e.refreshImages(ctx); err != nil {
		return PruneImagesResult{}, err
	}
	e.broadcast(Event{Type: "images", Action: "prune", Timestamp: time.Now()})
	return PruneImagesResult{
		Deleted:        report.Deleted,
		SpaceReclaimed: report.SpaceReclaimed,
		Attempted:      report.Attempted,
		Errors:         report.Errors,
	}, nil
}

func (e *Engine) RemoveVolume(ctx context.Context, name string, force bool) error {
	if err := e.docker.RemoveVolume(ctx, name, force); err != nil {
		return err
	}
	return e.refreshVolumes(ctx)
}

func (e *Engine) RemoveNetwork(ctx context.Context, id string) error {
	if err := e.docker.RemoveNetwork(ctx, id); err != nil {
		return err
	}
	return e.refreshNetworks(ctx)
}

func (e *Engine) Logs(ctx context.Context, id string, opts docker.LogOptions) (io.ReadCloser, error) {
	return e.docker.ContainerLogs(ctx, id, opts)
}

func (e *Engine) LogLines(ctx context.Context, id string, opts docker.LogOptions) ([]string, error) {
	return e.docker.ReadLogLines(ctx, id, opts)
}

func (e *Engine) StreamLogs(ctx context.Context, id string, opts docker.LogOptions, emit func(line string) error) error {
	return e.docker.StreamLogLines(ctx, id, opts, emit)
}

func (e *Engine) Exec(ctx context.Context, id string, cmd []string) error {
	return e.docker.Exec(ctx, id, cmd)
}

func (e *Engine) ForceRefresh(ctx context.Context) error {
	return e.refreshAll(ctx)
}

func (e *Engine) Config() *config.Config {
	return e.cfg
}

func (e *Engine) DockerPing(ctx context.Context) error {
	return e.docker.Ping(ctx)
}

func FormatPct(v float64) string {
	return fmt.Sprintf("%.1f%%", v)
}

func FormatBytes(b uint64) string {
	if b == 0 {
		return "-"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
