package notify

import "time"

// PendingRow is one container with an available image update.
type PendingRow struct {
	ContainerID  string `json:"container_id"`
	ShortID      string `json:"short_id"`
	Name         string `json:"name"`
	Image        string `json:"image"`
	LocalDigest  string `json:"local_digest"`
	RemoteDigest string `json:"remote_digest"`
}

// UpdateReport is the pending-update snapshot sent to webhooks.
type UpdateReport struct {
	ScannedAt time.Time
	Pending   []PendingRow
}
