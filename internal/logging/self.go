package logging

import (
	"os"
	"regexp"
	"strings"
)

var containerIDPattern = regexp.MustCompile(`(?:docker|containerd)[-/]([0-9a-f]{64})`)

// OwnContainerIDs returns identifiers for the container this process is running in.
// Docker's default hostname is the short container id, and the cgroup path carries the full id.
func OwnContainerIDs() []string {
	seen := map[string]struct{}{}
	add := func(id string) {
		id = strings.ToLower(strings.TrimSpace(id))
		if !hexID(id) {
			return
		}
		seen[id] = struct{}{}
	}
	if host, err := os.Hostname(); err == nil {
		add(host)
	}
	for _, path := range []string{"/proc/self/cgroup", "/proc/1/cgroup", "/proc/self/mountinfo"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, match := range containerIDPattern.FindAllStringSubmatch(string(raw), -1) {
			add(match[1])
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out
}

func hexID(id string) bool {
	if len(id) != 12 && len(id) != 64 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
