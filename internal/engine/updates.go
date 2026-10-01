package engine

import (
	"context"
	"log"
	"strings"
	"time"
)

func (e *Engine) pollUpdates(ctx context.Context) {
	if err := e.scanUpdates(ctx); err != nil {
		log.Printf("updates scan: %v", err)
	}
	interval := e.cfg.Updates.Interval.Duration
	if interval <= 0 {
		interval = time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.scanUpdates(ctx); err != nil {
				log.Printf("updates scan: %v", err)
			}
		}
	}
}

func (e *Engine) ScanUpdates(ctx context.Context) error {
	return e.scanUpdates(ctx)
}

func (e *Engine) Updates() UpdatesReport {
	e.scanMu.Lock()
	defer e.scanMu.Unlock()
	return cloneUpdates(e.updates)
}

func (e *Engine) scanUpdates(ctx context.Context) error {
	e.scanMu.Lock()
	defer e.scanMu.Unlock()

	list, err := e.docker.ListContainers(ctx, false)
	if err != nil {
		return err
	}

	report := UpdatesReport{
		ScannedAt: time.Now(),
		Pending:   []ImageUpdate{},
		Failures:  []ImageUpdate{},
	}
	digestsByID := map[string]digestLookup{}
	remoteByRef := map[string]remoteDigest{}

	for _, c := range list {
		if c.State != "" && c.State != "running" {
			continue
		}
		if !checkableImageRef(c.Image) {
			continue
		}
		row := ImageUpdate{
			ContainerID: c.ID,
			ShortID:     c.ShortID,
			Name:        c.Name,
			Image:       c.Image,
		}
		if c.ImageID == "" {
			row.Error = "container has no image id"
			report.Failures = append(report.Failures, row)
			continue
		}
		inspected, ok := digestsByID[c.ImageID]
		if !ok {
			got, inspectErr := e.docker.ImageRepoDigests(ctx, c.ImageID)
			inspected = digestLookup{digests: got, err: inspectErr}
			digestsByID[c.ImageID] = inspected
		}
		if inspected.err != nil {
			row.Error = inspected.err.Error()
			report.Failures = append(report.Failures, row)
			continue
		}
		row.LocalDigest = localDigestForRef(c.Image, inspected.digests)

		remote, ok := remoteByRef[c.Image]
		if !ok {
			digest, remoteErr := e.docker.RemoteImageDigest(ctx, c.Image)
			remote = remoteDigest{digest: digest, err: remoteErr}
			remoteByRef[c.Image] = remote
		}
		if remote.err != nil {
			row.Error = remote.err.Error()
			report.Failures = append(report.Failures, row)
			continue
		}
		row.RemoteDigest = remote.digest
		if updateAvailable(remote.digest, inspected.digests) {
			report.Pending = append(report.Pending, row)
		}
	}

	e.updates = report
	e.broadcast(Event{Type: "updates", Action: "scan", Timestamp: report.ScannedAt})
	return nil
}

type digestLookup struct {
	digests []string
	err     error
}

type remoteDigest struct {
	digest string
	err    error
}

func cloneUpdates(in UpdatesReport) UpdatesReport {
	out := in
	out.Pending = append([]ImageUpdate{}, in.Pending...)
	out.Failures = append([]ImageUpdate{}, in.Failures...)
	return out
}

func checkableImageRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "@") || strings.HasPrefix(ref, "sha256:") {
		return false
	}
	name, tag, ok := splitImageRef(ref)
	if !ok || name == "" || name == "<none>" || tag == "<none>" {
		return false
	}
	return true
}

func splitImageRef(ref string) (name, tag string, ok bool) {
	lastSlash := strings.LastIndex(ref, "/")
	lastColon := strings.LastIndex(ref, ":")
	if lastColon > lastSlash {
		name = ref[:lastColon]
		tag = ref[lastColon+1:]
		if name == "" || tag == "" {
			return "", "", false
		}
		return name, tag, true
	}
	return ref, "latest", true
}

func localDigestForRef(ref string, repoDigests []string) string {
	name, _, ok := splitImageRef(ref)
	if !ok {
		name = ref
	}
	fallback := ""
	for _, rd := range repoDigests {
		digest, repo, ok := splitRepoDigest(rd)
		if !ok {
			continue
		}
		if fallback == "" {
			fallback = digest
		}
		if repo == name || strings.HasSuffix(repo, "/"+name) {
			return digest
		}
	}
	return fallback
}

func splitRepoDigest(rd string) (digest, repo string, ok bool) {
	at := strings.LastIndex(rd, "@")
	if at <= 0 || at == len(rd)-1 {
		return "", "", false
	}
	return rd[at+1:], rd[:at], true
}

func updateAvailable(remote string, repoDigests []string) bool {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return false
	}
	return !digestListed(remote, repoDigests)
}

func digestListed(remote string, repoDigests []string) bool {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return false
	}
	if !strings.Contains(remote, ":") {
		remote = "sha256:" + remote
	}
	for _, rd := range repoDigests {
		if rd == remote || strings.HasSuffix(rd, "@"+remote) {
			return true
		}
	}
	return false
}
