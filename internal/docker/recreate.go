package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
)

// PullImage pulls the given image reference.
func (c *Client) PullImage(ctx context.Context, ref string) error {
	rc, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	_, _ = io.Copy(io.Discard, rc)
	return nil
}

// RecreateContainer pulls the current image, removes the container, and recreates it with the same config.
func (c *Client) RecreateContainer(ctx context.Context, id string) (string, error) {
	insp, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}

	imageRef := insp.Config.Image
	if imageRef == "" {
		return "", fmt.Errorf("container has no image")
	}
	if err := c.PullImage(ctx, imageRef); err != nil {
		return "", fmt.Errorf("pull image: %w", err)
	}

	name := insp.Name
	wasRunning := insp.State.Running

	if wasRunning {
		timeout := 10
		_ = c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
	}

	removeOpts := container.RemoveOptions{Force: true}
	if err := c.cli.ContainerRemove(ctx, id, removeOpts); err != nil {
		return "", fmt.Errorf("remove: %w", err)
	}

	cfg := insp.Config
	cfg.Hostname = ""
	hostCfg := insp.HostConfig
	hostCfg.ContainerIDFile = ""

	netCfg := &network.NetworkingConfig{}
	if insp.NetworkSettings != nil {
		netCfg.EndpointsConfig = make(map[string]*network.EndpointSettings)
		for netName, ep := range insp.NetworkSettings.Networks {
			netCfg.EndpointsConfig[netName] = &network.EndpointSettings{
				IPAMConfig:          ep.IPAMConfig,
				Links:               ep.Links,
				Aliases:             ep.Aliases,
				NetworkID:           ep.NetworkID,
				EndpointID:          ep.EndpointID,
				Gateway:             ep.Gateway,
				IPAddress:           ep.IPAddress,
				IPPrefixLen:         ep.IPPrefixLen,
				IPv6Gateway:         ep.IPv6Gateway,
				GlobalIPv6Address:   ep.GlobalIPv6Address,
				GlobalIPv6PrefixLen: ep.GlobalIPv6PrefixLen,
				MacAddress:          ep.MacAddress,
				DriverOpts:          ep.DriverOpts,
			}
		}
	}

	createName := name
	if len(createName) > 0 && createName[0] == '/' {
		createName = createName[1:]
	}

	resp, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, createName)
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}

	if wasRunning {
		if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			return resp.ID, fmt.Errorf("start: %w", err)
		}
	}

	return resp.ID, nil
}

// ContainerComposeInfo returns compose metadata from container labels.
func ContainerComposeInfo(labels map[string]string) (project, service, workingDir string) {
	if labels == nil {
		return "", "", ""
	}
	return labels[LabelComposeProject], labels[LabelComposeService], labels[LabelComposeWorkingDir]
}

// ImageFromInspect extracts the configured image reference.
func ImageFromInspect(raw string) (string, error) {
	var insp container.InspectResponse
	if err := json.Unmarshal([]byte(raw), &insp); err != nil {
		return "", err
	}
	return insp.Config.Image, nil
}
