package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

const (
	LabelComposeProject    = "com.docker.compose.project"
	LabelComposeService    = "com.docker.compose.service"
	LabelComposeWorkingDir = "com.docker.compose.project.working_dir"
)

type Client struct {
	cli *client.Client
}

type ContainerSummary struct {
	ID             string
	ShortID        string
	Name           string
	Image          string
	State          string
	Status         string
	ComposeProject string
	ComposeService string
	ComposeWorkDir string
	Health         string
	StartedAt      time.Time
	RestartCount   int
	Ports          []string
	Labels         map[string]string
}

type ImageSummary struct {
	ID        string
	ShortID   string
	RepoTags  []string
	Size      int64
	Created   time.Time
	Unused    bool
	Containers int
}

type VolumeSummary struct {
	Name       string
	Driver     string
	Mountpoint string
	Scope      string
	Unused     bool
	CreatedAt  time.Time
}

type NetworkSummary struct {
	ID         string
	Name       string
	Driver     string
	Scope      string
	Internal   bool
	Containers int
}

type InspectResult struct {
	ID      string
	ImageID string
	Raw     string
	Summary ContainerSummary
}

type LogOptions struct {
	Tail   string
	Follow bool
	Since  string
}

func NewClient(socket string) (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost("unix://"+socket),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

func (c *Client) ListContainers(ctx context.Context, all bool) ([]ContainerSummary, error) {
	opts := container.ListOptions{All: all}
	list, err := c.cli.ContainerList(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]ContainerSummary, 0, len(list))
	for _, ctr := range list {
		out = append(out, mapContainer(ctr))
	}
	return out, nil
}

// publicPortStrings formats published ports, deduplicating IPv4/IPv6 bindings
// (Docker lists 0.0.0.0:8080 and [::]:8080 as separate entries).
func publicPortStrings(ports []container.Port) []string {
	out := make([]string, 0, len(ports))
	seen := make(map[string]struct{})
	for _, p := range ports {
		if p.PublicPort == 0 {
			continue
		}
		key := fmt.Sprintf("%d:%d/%s", p.PublicPort, p.PrivatePort, p.Type)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func mapContainer(ctr container.Summary) ContainerSummary {
	name := ""
	if len(ctr.Names) > 0 {
		name = strings.TrimPrefix(ctr.Names[0], "/")
	}
	health := ""
	if ctr.Status != "" {
		lower := strings.ToLower(ctr.Status)
		switch {
		case strings.Contains(lower, "healthy"):
			health = "healthy"
		case strings.Contains(lower, "unhealthy"):
			health = "unhealthy"
		case strings.Contains(lower, "health: starting"):
			health = "starting"
		}
	}
	ports := publicPortStrings(ctr.Ports)
	shortID := ctr.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	return ContainerSummary{
		ID:             ctr.ID,
		ShortID:        shortID,
		Name:           name,
		Image:          ctr.Image,
		State:          ctr.State,
		Status:         ctr.Status,
		ComposeProject: ctr.Labels[LabelComposeProject],
		ComposeService: ctr.Labels[LabelComposeService],
		ComposeWorkDir: ctr.Labels[LabelComposeWorkingDir],
		Health:         health,
		Ports:          ports,
		Labels:         ctr.Labels,
	}
}

func (c *Client) InspectContainer(ctx context.Context, id string) (*InspectResult, error) {
	raw, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(raw, "", "  ")
	summary := ContainerSummary{
		ID:             raw.ID,
		ShortID:        raw.ID[:12],
		Name:           strings.TrimPrefix(raw.Name, "/"),
		Image:          raw.Config.Image,
		State:          raw.State.Status,
		Status:         raw.State.Status,
		ComposeProject: raw.Config.Labels[LabelComposeProject],
		ComposeService: raw.Config.Labels[LabelComposeService],
		ComposeWorkDir: raw.Config.Labels[LabelComposeWorkingDir],
		Health:         healthFromInspect(raw),
		RestartCount:   raw.RestartCount,
		Labels:         raw.Config.Labels,
	}
	if raw.State.StartedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, raw.State.StartedAt); err == nil {
			summary.StartedAt = t
		}
	}
	return &InspectResult{
		ID:      raw.ID,
		ImageID: raw.Image,
		Raw:     string(b),
		Summary: summary,
	}, nil
}

func healthFromInspect(raw container.InspectResponse) string {
	if raw.State.Health == nil {
		return ""
	}
	return raw.State.Health.Status
}

func normalizeImageID(id string) string {
	return strings.TrimPrefix(id, "sha256:")
}

func (c *Client) ListImages(ctx context.Context) ([]ImageSummary, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	inUse := make(map[string]int)
	for _, ctr := range containers {
		if ctr.ImageID == "" {
			continue
		}
		inUse[normalizeImageID(ctr.ImageID)]++
	}

	list, err := c.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ImageSummary, 0, len(list))
	for _, img := range list {
		shortID := img.ID
		if strings.HasPrefix(shortID, "sha256:") {
			shortID = shortID[7:19]
		}
		created := time.Unix(img.Created, 0)
		count := inUse[normalizeImageID(img.ID)]
		out = append(out, ImageSummary{
			ID:         img.ID,
			ShortID:    shortID,
			RepoTags:   img.RepoTags,
			Size:       img.Size,
			Created:    created,
			Containers: count,
			Unused:     count == 0,
		})
	}
	return out, nil
}

func (c *Client) PruneUnusedImages(ctx context.Context) (PruneImagesReport, error) {
	images, err := c.ListImages(ctx)
	if err != nil {
		return PruneImagesReport{}, err
	}

	report := PruneImagesReport{}
	for _, img := range images {
		if !img.Unused {
			continue
		}
		report.Attempted++
		_, err := c.cli.ImageRemove(ctx, img.ID, image.RemoveOptions{Force: true, PruneChildren: true})
		if err != nil {
			label := imageLabel(img)
			report.Errors = append(report.Errors, fmt.Sprintf("%s (%s): %v", img.ShortID, label, err))
			continue
		}
		report.Deleted++
		report.SpaceReclaimed += uint64(img.Size)
	}
	return report, nil
}

func imageLabel(img ImageSummary) string {
	if len(img.RepoTags) == 0 {
		return "<none>"
	}
	return strings.Join(img.RepoTags, ", ")
}

type PruneImagesReport struct {
	Deleted        int
	SpaceReclaimed uint64
	Attempted      int
	Errors         []string
}

func (c *Client) ListVolumes(ctx context.Context) ([]VolumeSummary, error) {
	list, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]VolumeSummary, 0, len(list.Volumes))
	for _, vol := range list.Volumes {
		created := time.Time{}
		if vol.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, vol.CreatedAt); err == nil {
				created = t
			}
		}
		unused := vol.UsageData == nil || vol.UsageData.RefCount == 0
		out = append(out, VolumeSummary{
			Name:       vol.Name,
			Driver:     vol.Driver,
			Mountpoint: vol.Mountpoint,
			Scope:      vol.Scope,
			Unused:     unused,
			CreatedAt:  created,
		})
	}
	return out, nil
}

func (c *Client) ListNetworks(ctx context.Context) ([]NetworkSummary, error) {
	list, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]NetworkSummary, 0, len(list))
	for _, net := range list {
		out = append(out, NetworkSummary{
			ID:         net.ID[:12],
			Name:       net.Name,
			Driver:     net.Driver,
			Scope:      net.Scope,
			Internal:   net.Internal,
			Containers: len(net.Containers),
		})
	}
	return out, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
}

func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	_, err := c.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force})
	return err
}

func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	return c.cli.VolumeRemove(ctx, name, force)
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	return c.cli.NetworkRemove(ctx, id)
}

func (c *Client) ContainerLogs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       opts.Tail,
		Follow:     opts.Follow,
		Since:      opts.Since,
		Timestamps: true,
	})
}

func (c *Client) ContainerStats(ctx context.Context, id string) (io.ReadCloser, error) {
	resp, err := c.cli.ContainerStats(ctx, id, false)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (c *Client) Events(ctx context.Context) (<-chan events.Message, <-chan error) {
	msgCh := make(chan events.Message, 32)
	errCh := make(chan error, 1)
	go func() {
		defer close(msgCh)
		defer close(errCh)
		evCh, errChInternal := c.cli.Events(ctx, events.ListOptions{})
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errChInternal:
				if ok && err != nil {
					errCh <- err
				}
				return
			case ev, ok := <-evCh:
				if !ok {
					return
				}
				msgCh <- ev
			}
		}
	}()
	return msgCh, errCh
}

func (c *Client) RawClient() *client.Client {
	return c.cli
}

func (c *Client) Exec(ctx context.Context, id string, cmd []string) error {
	execResp, err := c.cli.ContainerExecCreate(ctx, id, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})
	if err != nil {
		return err
	}
	attach, err := c.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return err
	}
	defer attach.Close()
	_, err = io.Copy(io.Discard, attach.Reader)
	return err
}
