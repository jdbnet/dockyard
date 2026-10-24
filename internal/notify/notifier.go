package notify

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

type ReportFunc func() UpdateReport

type Notifier struct {
	mu                     sync.Mutex
	cfg                    config.NotificationsConfig
	status                 Status
	lastInstantFingerprint string
	lastDailySent          string
	report                 ReportFunc
	client                 *http.Client
	scheduleCancel         context.CancelFunc
}

func New(cfg config.NotificationsConfig, report ReportFunc) *Notifier {
	n := &Notifier{
		cfg:    cfg,
		report: report,
		client: &http.Client{Timeout: 30 * time.Second},
	}
	return n
}

func (n *Notifier) Status() Status {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := n.status
	if len(n.status.WebhookErrors) > 0 {
		out.WebhookErrors = map[string]string{}
		for k, v := range n.status.WebhookErrors {
			out.WebhookErrors[k] = v
		}
	}
	return out
}

func (n *Notifier) Update(cfg config.NotificationsConfig) {
	n.mu.Lock()
	n.cfg = cfg
	n.mu.Unlock()
	n.restartSchedule()
}

func (n *Notifier) restartSchedule() {
	n.mu.Lock()
	if n.scheduleCancel != nil {
		n.scheduleCancel()
		n.scheduleCancel = nil
	}
	n.mu.Unlock()

	n.mu.Lock()
	mode := n.cfg.Schedule.Mode
	if mode == "" {
		mode = ScheduleInstant
	}
	enabled := n.cfg.Enabled
	n.mu.Unlock()

	if !enabled || mode == ScheduleInstant {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	n.mu.Lock()
	n.scheduleCancel = cancel
	n.mu.Unlock()
	go n.runSchedule(ctx)
}

func (n *Notifier) RunSchedule(ctx context.Context) {
	n.mu.Lock()
	if n.scheduleCancel != nil {
		n.scheduleCancel()
	}
	child, cancel := context.WithCancel(ctx)
	n.scheduleCancel = cancel
	n.mu.Unlock()
	go n.runSchedule(child)
}

func (n *Notifier) runSchedule(ctx context.Context) {
	for {
		n.mu.Lock()
		cfg := n.cfg
		n.mu.Unlock()
		if !cfg.Enabled {
			return
		}
		switch cfg.Schedule.Mode {
		case ScheduleInterval:
			interval := cfg.Schedule.Interval.Duration
			if interval <= 0 {
				interval = 6 * time.Hour
			}
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				n.fireScheduled(ctx)
			}
		case ScheduleDaily:
			wait, ok := timeUntilDaily(cfg.Schedule.At)
			if !ok {
				log.Printf("notifications: invalid daily schedule")
				return
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				n.fireDaily(ctx, cfg.Schedule.At)
			}
		default:
			return
		}
	}
}

func (n *Notifier) fireDaily(ctx context.Context, at string) {
	today := time.Now().Format("2006-01-02")
	n.mu.Lock()
	if n.lastDailySent == today {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()

	report := n.report()
	if len(report.Pending) == 0 {
		return
	}
	if err := n.Notify(ctx, report); err != nil {
		log.Printf("notifications: %v", err)
		return
	}
	n.mu.Lock()
	n.lastDailySent = today
	n.mu.Unlock()
}

func (n *Notifier) fireScheduled(ctx context.Context) {
	report := n.report()
	if len(report.Pending) == 0 {
		return
	}
	if err := n.Notify(ctx, report); err != nil {
		log.Printf("notifications: %v", err)
	}
}

func timeUntilDaily(at string) (time.Duration, bool) {
	hour, minute, err := parseDailyAt(at)
	if err != nil {
		return 0, false
	}
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now), true
}

func (n *Notifier) MaybeNotifyInstant(ctx context.Context, report UpdateReport) {
	n.mu.Lock()
	cfg := n.cfg
	n.mu.Unlock()
	if !cfg.Enabled || cfg.Schedule.Mode != ScheduleInstant {
		return
	}
	if len(report.Pending) == 0 {
		return
	}
	fp := PendingFingerprint(report.Pending)
	n.mu.Lock()
	if fp == n.lastInstantFingerprint {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()
	if err := n.Notify(ctx, report); err != nil {
		log.Printf("notifications: %v", err)
		return
	}
	n.mu.Lock()
	n.lastInstantFingerprint = fp
	n.mu.Unlock()
}

func (n *Notifier) Notify(ctx context.Context, report UpdateReport) error {
	if len(report.Pending) == 0 {
		return nil
	}
	n.mu.Lock()
	cfg := n.cfg
	n.mu.Unlock()

	var firstErr error
	webhookErrors := map[string]string{}
	for _, wh := range cfg.Webhooks {
		if !wh.Enabled || strings.TrimSpace(wh.URL) == "" {
			continue
		}
		p, err := BuildPayload(wh, cfg.PublicURL, report)
		if err != nil {
			webhookErrors[wh.ID] = err.Error()
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := postWebhook(ctx, n.client, wh, p); err != nil {
			webhookErrors[wh.ID] = err.Error()
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
	}

	n.mu.Lock()
	n.status.LastNotifyAt = time.Now().UTC()
	n.status.WebhookErrors = webhookErrors
	if firstErr != nil {
		n.status.LastError = firstErr.Error()
	} else {
		n.status.LastError = ""
	}
	n.mu.Unlock()
	return firstErr
}

func (n *Notifier) Test(ctx context.Context, stored config.NotificationsConfig, req TestRequest) error {
	report := SampleReport()
	wh, ok := resolveTestWebhook(stored, req)
	if !ok {
		return fmt.Errorf("webhook not found")
	}
	if strings.TrimSpace(wh.URL) == "" {
		return fmt.Errorf("webhook URL is required")
	}
	p, err := BuildPayload(wh, stored.PublicURL, report)
	if err != nil {
		return err
	}
	return postWebhook(ctx, n.client, wh, p)
}

func resolveTestWebhook(stored config.NotificationsConfig, req TestRequest) (config.WebhookConfig, bool) {
	if req.Webhook != nil {
		wh := config.WebhookConfig{
			ID:              req.Webhook.ID,
			Name:            req.Webhook.Name,
			Type:            req.Webhook.Type,
			URL:             req.Webhook.URL,
			Auth:            req.Webhook.Auth,
			Username:        req.Webhook.Username,
			Token:           req.Webhook.Token,
			Enabled:         true,
			MessageTemplate: req.Webhook.MessageTemplate,
			TitleTemplate:   req.Webhook.TitleTemplate,
			CustomFields:    req.Webhook.CustomFields,
		}
		for _, prev := range stored.Webhooks {
			if prev.ID == wh.ID || (req.WebhookID != "" && prev.ID == req.WebhookID) {
				if wh.URL == "" {
					wh.URL = prev.URL
				}
				if wh.Token == "" {
					wh.Token = prev.Token
				}
				if wh.Auth == "" {
					wh.Auth = prev.Auth
				}
				if wh.Username == "" {
					wh.Username = prev.Username
				}
				if wh.Type == "" {
					wh.Type = prev.Type
				}
				if wh.MessageTemplate == "" {
					wh.MessageTemplate = prev.MessageTemplate
				}
				if wh.TitleTemplate == "" {
					wh.TitleTemplate = prev.TitleTemplate
				}
				if len(wh.CustomFields) == 0 {
					wh.CustomFields = prev.CustomFields
				}
				break
			}
		}
		return wh, wh.URL != ""
	}
	id := req.WebhookID
	for _, wh := range stored.Webhooks {
		if wh.ID == id {
			return wh, true
		}
	}
	return config.WebhookConfig{}, false
}
