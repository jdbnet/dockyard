package logging

import (
	"strings"
	"unicode"
)

const (
	labelLogs       = "dockyard.logs"
	labelLogsPrefix = "dockyard.logs.label."
	maxLineBytes    = 256 << 10
)

// Target is a running container the shipper might follow.
type Target struct {
	ID             string
	Name           string
	Image          string
	ComposeProject string
	ComposeService string
	Labels         map[string]string
	LogDriver      string
	State          string
}

// Entry is one demultiplexed log line.
type Entry struct {
	Stream string
	Line   string
}

// Skipped is a container that is running but not being followed.
type Skipped struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// Decision is the collect / skip result for one container.
type Decision struct {
	Collect        bool
	Skip           bool
	NotCollectable string
}

// Decide applies the default, the dockyard.logs label, the self-container skip, and the log driver.
func Decide(t Target, collectByDefault bool, selfIDs []string) Decision {
	if t.State != "" && t.State != "running" {
		return Decision{Skip: true}
	}
	if isSelf(t.ID, selfIDs) && !labelIs(t.Labels, "true") {
		return Decision{Skip: true}
	}
	if !collectableDriver(t.LogDriver) {
		return Decision{NotCollectable: driverReason(t.LogDriver)}
	}
	if !wantLogs(t.Labels, collectByDefault) {
		return Decision{Skip: true}
	}
	return Decision{Collect: true}
}

func wantLogs(labels map[string]string, collectByDefault bool) bool {
	value, ok := labels[labelLogs]
	if !ok {
		return collectByDefault
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true
	case "false":
		return false
	default:
		return collectByDefault
	}
}

func labelIs(labels map[string]string, want string) bool {
	return strings.EqualFold(strings.TrimSpace(labels[labelLogs]), want)
}

func collectableDriver(driver string) bool {
	return driver == "json-file" || driver == "local"
}

func driverReason(driver string) string {
	if driver == "" {
		return "log driver unknown"
	}
	return "log driver " + driver
}

func isSelf(containerID string, selfIDs []string) bool {
	id := strings.ToLower(containerID)
	if len(id) < 12 {
		return false
	}
	for _, self := range selfIDs {
		self = strings.ToLower(strings.TrimSpace(self))
		if len(self) < 12 {
			continue
		}
		if id == self || strings.HasPrefix(id, self) || strings.HasPrefix(self, id) {
			return true
		}
	}
	return false
}

// StreamLabels builds the Loki label set for one stream.
// Built-in labels replace any dockyard.logs.label.* value with the same name.
func StreamLabels(host string, t Target, stream string) map[string]string {
	labels := map[string]string{}
	for key, value := range t.Labels {
		name, ok := strings.CutPrefix(key, labelLogsPrefix)
		if !ok || value == "" {
			continue
		}
		safe := sanitiseLabel(name)
		if safe == "" {
			continue
		}
		labels[safe] = value
	}
	put := func(key, value string) {
		if value == "" {
			return
		}
		labels[key] = value
	}
	put("host", host)
	put("container", strings.TrimPrefix(t.Name, "/"))
	put("image", t.Image)
	put("compose_project", t.ComposeProject)
	put("compose_service", t.ComposeService)
	put("stream", stream)
	return labels
}

func sanitiseLabel(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '/':
			b.WriteByte('_')
		}
	}
	safe := strings.Trim(b.String(), "_")
	if safe == "" {
		return ""
	}
	first, _ := utf8Decode(safe)
	if first != '_' && !unicode.IsLetter(first) {
		safe = "_" + safe
	}
	return safe
}

func utf8Decode(s string) (rune, int) {
	for _, r := range s {
		return r, 1
	}
	return 0, 0
}
