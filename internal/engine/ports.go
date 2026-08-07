package engine

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PortBinding maps a host port to a container and compose stack.
type PortBinding struct {
	HostPort       uint16 `json:"host_port"`
	ContainerPort  uint16 `json:"container_port"`
	Protocol       string `json:"protocol"`
	Binding        string `json:"binding"`
	ContainerID    string `json:"container_id"`
	ContainerName  string `json:"container_name"`
	ContainerState string `json:"container_state"`
	ComposeProject string `json:"compose_project"`
	ComposeService string `json:"compose_service"`
}

func (e *Engine) Ports(_ context.Context) ([]PortBinding, error) {
	containers := e.store.containersSnapshot()
	out := make([]PortBinding, 0)
	for _, c := range containers {
		for _, p := range c.Ports {
			host, container, proto, err := parsePortString(p)
			if err != nil {
				continue
			}
			out = append(out, PortBinding{
				HostPort:       host,
				ContainerPort:  container,
				Protocol:       proto,
				Binding:        p,
				ContainerID:    c.ID,
				ContainerName:  c.Name,
				ContainerState: c.State,
				ComposeProject: c.ComposeProject,
				ComposeService: c.ComposeService,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].HostPort != out[j].HostPort {
			return out[i].HostPort < out[j].HostPort
		}
		if out[i].ContainerName != out[j].ContainerName {
			return out[i].ContainerName < out[j].ContainerName
		}
		return out[i].ContainerPort < out[j].ContainerPort
	})
	return out, nil
}

func parsePortString(s string) (host, container uint16, proto string, err error) {
	slash := strings.LastIndex(s, "/")
	if slash < 0 {
		return 0, 0, "", fmt.Errorf("invalid port %q", s)
	}
	proto = s[slash+1:]
	pair := s[:slash]
	parts := strings.SplitN(pair, ":", 2)
	if len(parts) != 2 {
		return 0, 0, "", fmt.Errorf("invalid port %q", s)
	}
	h, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, 0, "", err
	}
	c, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return 0, 0, "", err
	}
	return uint16(h), uint16(c), proto, nil
}
