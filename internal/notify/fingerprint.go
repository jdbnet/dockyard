package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// PendingFingerprint hashes the pending update set for instant-mode change detection.
func PendingFingerprint(pending []PendingRow) string {
	if len(pending) == 0 {
		return ""
	}
	keys := make([]string, len(pending))
	for i, row := range pending {
		keys[i] = row.ContainerID + "|" + strings.TrimSpace(row.RemoteDigest)
	}
	sort.Strings(keys)
	sum := sha256.Sum256([]byte(strings.Join(keys, "\n")))
	return hex.EncodeToString(sum[:])
}
