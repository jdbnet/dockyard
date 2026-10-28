package engine

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/docker"
	"github.com/jdbnet/dockyard/internal/logging"
)

func (e *Engine) pollAutoUpdates(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	e.autoUpdateTick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.autoUpdateTick(ctx)
		}
	}
}

func (e *Engine) autoUpdateTick(ctx context.Context) {
	e.autoMu.Lock()
	if e.autoResetNextCheck {
		e.autoNextCheck = map[string]time.Time{}
		e.autoResetNextCheck = false
	}
	e.autoMu.Unlock()

	list, err := e.docker.ListContainers(ctx, false)
	if err != nil {
		log.Printf("auto-update: list containers: %v", err)
		return
	}
	cfg := e.cfg.Updates
	now := time.Now()
	namesPresent := map[string]struct{}{}

	for _, c := range list {
		if c.State != "" && c.State != "running" {
			continue
		}
		if !checkableImageRef(c.Image) {
			continue
		}
		namesPresent[c.Name] = struct{}{}
		enabled, interval, _ := autoUpdatePolicy(c.Labels, cfg)
		_, intervalLabel := parseContainerIntervalLabel(containerIntervalLabelRaw(c.Labels))
		if !enabled && !intervalLabel {
			e.autoMu.Lock()
			delete(e.autoRows, c.Name)
			delete(e.autoNextCheck, c.Name)
			e.autoMu.Unlock()
			continue
		}
		self := logging.IsSelf(c.ID, e.selfIDs)
		hasDigests, digestErr := e.containerHasRepoDigests(ctx, c.ImageID)
		if digestErr != nil {
			log.Printf("auto-update: %s: %v", c.Name, digestErr)
			continue
		}
		if !autoLoopOwnsContainer(c.Labels, cfg, self, hasDigests) {
			e.autoMu.Lock()
			delete(e.autoRows, c.Name)
			delete(e.autoNextCheck, c.Name)
			e.autoMu.Unlock()
			continue
		}
		if !hasDigests {
			e.setAutoRow(c, ImageUpdate{
				ContainerID: c.ID,
				ShortID:     c.ShortID,
				Name:        c.Name,
				Image:       c.Image,
				Error:       "image has no registry digest",
				AutoUpdate:  enabled,
			})
			continue
		}
		if self && enabled {
			e.setAutoRow(c, ImageUpdate{
				ContainerID: c.ID,
				ShortID:     c.ShortID,
				Name:        c.Name,
				Image:       c.Image,
				Error:       "Dockyard does not auto-update its own container",
				AutoUpdate:  true,
			})
			continue
		}

		e.autoMu.Lock()
		next, due := e.autoNextCheck[c.Name]
		if next.IsZero() || now.After(next) {
			due = true
		}
		inFlight := e.autoInFlight[c.Name]
		e.autoMu.Unlock()

		if !due || inFlight {
			continue
		}
		cont := c
		go e.runAutoCheck(ctx, cont, interval, enabled)
	}

	e.autoMu.Lock()
	for name := range e.autoNextCheck {
		if _, ok := namesPresent[name]; !ok {
			delete(e.autoNextCheck, name)
			delete(e.autoInFlight, name)
			delete(e.autoLastFailedDigest, name)
			delete(e.autoRows, name)
		}
	}
	e.autoMu.Unlock()
}

func (e *Engine) containerHasRepoDigests(ctx context.Context, imageID string) (bool, error) {
	if imageID == "" {
		return false, nil
	}
	digests, err := e.docker.ImageRepoDigests(ctx, imageID)
	if err != nil {
		return false, err
	}
	return len(digests) > 0, nil
}

func (e *Engine) runAutoCheck(ctx context.Context, c docker.ContainerSummary, interval time.Duration, autoApply bool) {
	defer func() {
		e.autoMu.Lock()
		e.autoNextCheck[c.Name] = time.Now().Add(interval)
		e.autoMu.Unlock()
	}()

	if err := e.beginContainerUpdate(c.Name); err != nil {
		return
	}
	defer e.endContainerUpdate(c.Name)

	cur, err := e.docker.InspectContainer(ctx, c.ID)
	if err != nil {
		e.recordAutoFailure(c, autoApply, err)
		return
	}
	if cur.ID != c.ID || cur.ImageID != c.ImageID {
		return
	}

	digests, err := e.docker.ImageRepoDigests(ctx, c.ImageID)
	if err != nil {
		e.recordAutoFailure(c, autoApply, err)
		return
	}
	local := localDigestForRef(c.Image, digests)
	remote, err := e.docker.RemoteManifestDigest(ctx, c.Image)
	if err != nil {
		e.recordAutoFailure(c, autoApply, err)
		return
	}
	if !updateAvailable(remote, digests) {
		e.autoMu.Lock()
		delete(e.autoRows, c.Name)
		delete(e.autoLastFailedDigest, c.Name)
		e.autoMu.Unlock()
		return
	}

	e.autoMu.Lock()
	if e.autoLastFailedDigest[c.Name] == remote {
		e.autoMu.Unlock()
		return
	}
	e.autoMu.Unlock()

	row := ImageUpdate{
		ContainerID:  c.ID,
		ShortID:      c.ShortID,
		Name:         c.Name,
		Image:        c.Image,
		LocalDigest:  local,
		RemoteDigest: remote,
		AutoUpdate:   autoApply,
	}
	if !autoApply {
		e.setAutoRow(c, row)
		e.broadcast(Event{Type: "updates", Action: "scan", Timestamp: time.Now()})
		if e.notifier != nil {
			e.notifier.MaybeNotifyInstant(ctx, toNotifyReport(e.Updates()))
		}
		return
	}

	previousImageID := cur.ImageID
	again, err := e.docker.InspectContainer(ctx, c.ID)
	if err != nil || again.ID != c.ID || again.ImageID != c.ImageID {
		return
	}
	newID, err := e.updateContainerByID(ctx, c.ID, cur)
	if err != nil {
		e.autoMu.Lock()
		e.autoLastFailedDigest[c.Name] = remote
		e.autoMu.Unlock()
		e.recordAutoFailure(c, true, err)
		return
	}
	if err := e.finishContainerUpdateAuto(ctx, previousImageID); err != nil {
		log.Printf("auto-update: %s: refresh: %v", c.Name, err)
	}

	after, err := e.docker.InspectContainer(ctx, newID)
	if err != nil {
		e.recordAutoFailure(c, true, err)
		return
	}
	newDigests, err := e.docker.ImageRepoDigests(ctx, after.ImageID)
	if err != nil {
		e.recordAutoFailure(c, true, err)
		return
	}
	if digestListed(remote, newDigests) {
		log.Printf("auto-update: %s (%s) %s -> %s", c.Name, c.Image, shortDigest(local), shortDigest(remote))
		e.autoMu.Lock()
		delete(e.autoRows, c.Name)
		delete(e.autoLastFailedDigest, c.Name)
		e.autoMu.Unlock()
		e.broadcast(Event{
			Type:      "updates",
			Action:    "auto_update",
			Resource:  after.ID,
			Message:   fmt.Sprintf("%s updated to %s", c.Name, shortDigest(remote)),
			Timestamp: time.Now(),
		})
		return
	}
	e.autoMu.Lock()
	e.autoLastFailedDigest[c.Name] = remote
	e.autoMu.Unlock()
	failRow := row
	failRow.Error = "updated container image does not list the remote digest"
	failRow.ContainerID = after.ID
	failRow.ShortID = after.Summary.ShortID
	e.setAutoRow(c, failRow)
	e.broadcast(Event{
		Type:      "updates",
		Action:    "auto_update_failed",
		Resource:  after.ID,
		Message:   fmt.Sprintf("%s: %s", c.Name, failRow.Error),
		Timestamp: time.Now(),
	})
}

func (e *Engine) recordAutoFailure(c docker.ContainerSummary, autoApply bool, err error) {
	log.Printf("auto-update: %s: %v", c.Name, err)
	row := ImageUpdate{
		ContainerID: c.ID,
		ShortID:     c.ShortID,
		Name:        c.Name,
		Image:       c.Image,
		Error:       err.Error(),
		AutoUpdate:  autoApply,
	}
	e.setAutoRow(c, row)
	e.broadcast(Event{
		Type:      "updates",
		Action:    "auto_update_failed",
		Resource:  c.ID,
		Message:   fmt.Sprintf("%s: %v", c.Name, err),
		Timestamp: time.Now(),
	})
}

func (e *Engine) setAutoRow(c docker.ContainerSummary, row ImageUpdate) {
	e.autoMu.Lock()
	e.autoRows[c.Name] = row
	e.autoMu.Unlock()
}

func mergeAutoRows(base UpdatesReport, autoRows map[string]ImageUpdate) UpdatesReport {
	out := base
	for _, row := range autoRows {
		if row.Error != "" {
			out.Failures = append(out.Failures, row)
		} else {
			out.Pending = append(out.Pending, row)
		}
	}
	return out
}

func shortDigest(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func (e *Engine) resetAutoUpdateSchedule() {
	e.autoMu.Lock()
	e.autoResetNextCheck = true
	e.autoMu.Unlock()
	go func() {
		if e.runCtx == nil {
			return
		}
		if err := e.scanUpdates(e.runCtx); err != nil {
			log.Printf("updates scan: %v", err)
		}
	}()
}

func (e *Engine) beginContainerUpdate(name string) error {
	e.autoMu.Lock()
	defer e.autoMu.Unlock()
	if e.autoInFlight[name] {
		return fmt.Errorf("update already in progress for %s", name)
	}
	e.autoInFlight[name] = true
	return nil
}

func (e *Engine) endContainerUpdate(name string) {
	e.autoMu.Lock()
	defer e.autoMu.Unlock()
	delete(e.autoInFlight, name)
}

func (e *Engine) finishContainerUpdateAuto(ctx context.Context, previousImageID string) error {
	if err := e.refreshAll(ctx); err != nil {
		return err
	}
	if img := e.removableImage(ctx, previousImageID); img != nil {
		if err := e.docker.RemoveImage(ctx, img.ID, false); err != nil {
			log.Printf("auto-update: remove previous image %s: %v", img.ShortID, err)
		}
	}
	return nil
}
