package engine

import "time"

type Container struct {
	ID             string   `json:"id"`
	ShortID        string   `json:"short_id"`
	Name           string   `json:"name"`
	Image          string   `json:"image"`
	State          string   `json:"state"`
	Status         string   `json:"status"`
	ComposeProject string   `json:"compose_project"`
	ComposeService string   `json:"compose_service"`
	ComposeWorkDir string   `json:"compose_working_dir"`
	Health         string   `json:"health"`
	StartedAt      time.Time `json:"started_at"`
	Uptime         string   `json:"uptime"`
	RestartCount   int      `json:"restart_count"`
	RestartLoop    bool     `json:"restart_loop"`
	CPUPct         float64  `json:"cpu_pct"`
	MemPct         float64  `json:"mem_pct"`
	MemBytes       uint64   `json:"mem_bytes"`
	Ports          []string `json:"ports"`
}

type ComposeProject struct {
	Name       string      `json:"name"`
	Containers []Container `json:"containers"`
}

type Image struct {
	ID         string    `json:"id"`
	ShortID    string    `json:"short_id"`
	RepoTags   []string  `json:"repo_tags"`
	Size       int64     `json:"size"`
	Created    time.Time `json:"created"`
	Unused     bool      `json:"unused"`
	Containers int       `json:"containers"`
}

type Volume struct {
	Name       string    `json:"name"`
	Driver     string    `json:"driver"`
	Mountpoint string    `json:"mountpoint"`
	Scope      string    `json:"scope"`
	Unused     bool      `json:"unused"`
	CreatedAt  time.Time `json:"created_at"`
}

type Network struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Scope      string `json:"scope"`
	Internal   bool   `json:"internal"`
	Containers int    `json:"containers"`
}

type StatPoint struct {
	Timestamp time.Time `json:"timestamp"`
	CPUPct    float64   `json:"cpu_pct"`
	MemPct    float64   `json:"mem_pct"`
	MemBytes  uint64    `json:"mem_bytes"`
	NetRx     uint64    `json:"net_rx"`
	NetTx     uint64    `json:"net_tx"`
}

type StatsSeries struct {
	ContainerID string      `json:"container_id"`
	Points      []StatPoint `json:"points"`
}

type Event struct {
	Type      string    `json:"type"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

const StandaloneProject = "(standalone)"
