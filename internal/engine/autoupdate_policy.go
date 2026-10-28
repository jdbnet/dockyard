package engine

import (
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
	"github.com/jdbnet/dockyard/internal/docker"
)

const (
	LabelAutoUpdate = "dockyard.autoupdate"
	LabelInterval   = "dockyard.interval"
	minAutoInterval = 10 * time.Second
)

// autoUpdatePolicy decides whether auto-apply is enabled, the check interval, and
// whether dockyard.autoupdate explicitly set the enabled flag.
func autoUpdatePolicy(labels map[string]string, cfg config.UpdatesConfig) (enabled bool, interval time.Duration, explicit bool) {
	if labels == nil {
		labels = map[string]string{}
	}
	switch parseTristateLabel(labels[LabelAutoUpdate]) {
	case tristateTrue:
		enabled = true
		explicit = true
	case tristateFalse:
		enabled = false
		explicit = true
	default:
		enabled = cfg.AutoUpdate
		explicit = false
	}
	if d, ok := parseContainerIntervalLabel(labels[LabelInterval]); ok {
		interval = d
		return enabled, interval, explicit
	}
	if enabled && cfg.AutoUpdateInterval.Duration > 0 {
		interval = cfg.AutoUpdateInterval.Duration
		return enabled, interval, explicit
	}
	interval = cfg.Interval.Duration
	if interval <= 0 {
		interval = time.Hour
	}
	return enabled, interval, explicit
}

type tristate int

const (
	tristateUnset tristate = iota
	tristateTrue
	tristateFalse
)

func parseTristateLabel(value string) tristate {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return tristateTrue
	case "false":
		return tristateFalse
	default:
		return tristateUnset
	}
}

func parseContainerIntervalLabel(raw string) (time.Duration, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < minAutoInterval {
		return 0, false
	}
	return d, true
}

func containerIntervalLabelRaw(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	return strings.TrimSpace(labels[LabelInterval])
}

// autoLoopOwnsContainer reports whether the auto-update loop owns this container
// instead of the hourly scanUpdates pass.
func autoLoopOwnsContainer(labels map[string]string, cfg config.UpdatesConfig, selfContainer bool, hasRepoDigests bool) bool {
	enabled, _, explicit := autoUpdatePolicy(labels, cfg)
	_, intervalOK := parseContainerIntervalLabel(labels[LabelInterval])
	if selfContainer {
		return explicit && enabled
	}
	if !hasRepoDigests {
		return explicit && enabled
	}
	return enabled || intervalOK
}

func mapContainerAutoFields(labels map[string]string, cfg config.UpdatesConfig) (autoUpdate bool, updateInterval string) {
	enabled, _, _ := autoUpdatePolicy(labels, cfg)
	if d, ok := parseContainerIntervalLabel(labels[LabelInterval]); ok {
		updateInterval = config.FormatDuration(d)
	}
	return enabled, updateInterval
}

func formatContainerSummary(c docker.ContainerSummary, tracker *restartTracker, updates config.UpdatesConfig) Container {
	out := mapContainerSummary(c, tracker)
	autoUpdate, interval := mapContainerAutoFields(c.Labels, updates)
	out.AutoUpdate = autoUpdate
	out.UpdateInterval = interval
	return out
}
