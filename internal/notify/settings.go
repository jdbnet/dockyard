package notify

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

const (
	ScheduleInstant  = "instant"
	ScheduleInterval = "interval"
	ScheduleDaily    = "daily"
)

type WebhookView struct {
	ID              string               `json:"id"`
	Name            string               `json:"name"`
	Type            string               `json:"type"`
	Auth            string               `json:"auth"`
	Username        string               `json:"username"`
	URLSet          bool                 `json:"url_set"`
	TokenSet        bool                 `json:"token_set"`
	Enabled         bool                 `json:"enabled"`
	MessageTemplate string               `json:"message_template,omitempty"`
	TitleTemplate   string               `json:"title_template,omitempty"`
	CustomFields    []config.WebhookField `json:"custom_fields,omitempty"`
}

type ScheduleView struct {
	Mode     string `json:"mode"`
	Interval string `json:"interval"`
	At       string `json:"at"`
}

type SettingsView struct {
	Enabled   bool          `json:"enabled"`
	PublicURL string        `json:"public_url"`
	Schedule  ScheduleView  `json:"schedule"`
	Webhooks  []WebhookView `json:"webhooks"`
}

type WebhookUpdate struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Type            string                `json:"type"`
	URL             string                `json:"url"`
	Auth            string                `json:"auth"`
	Username        string                `json:"username"`
	Token           string                `json:"token"`
	Enabled         bool                  `json:"enabled"`
	MessageTemplate string                `json:"message_template"`
	TitleTemplate   string                `json:"title_template"`
	CustomFields    []config.WebhookField `json:"custom_fields"`
}

type ScheduleUpdate struct {
	Mode     string `json:"mode"`
	Interval string `json:"interval"`
	At       string `json:"at"`
}

type SettingsUpdate struct {
	Enabled   bool            `json:"enabled"`
	PublicURL string          `json:"public_url"`
	Schedule  ScheduleUpdate  `json:"schedule"`
	Webhooks  []WebhookUpdate `json:"webhooks"`
}

type TestRequest struct {
	WebhookID string          `json:"webhook_id"`
	Webhook   *WebhookUpdate  `json:"webhook"`
	Settings  *SettingsUpdate `json:"settings"`
}

type Status struct {
	LastNotifyAt  time.Time         `json:"last_notify_at,omitempty"`
	LastError     string            `json:"last_error,omitempty"`
	WebhookErrors map[string]string `json:"webhook_errors,omitempty"`
}

func PublicSettings(cfg config.NotificationsConfig) SettingsView {
	webhooks := make([]WebhookView, len(cfg.Webhooks))
	for i, wh := range cfg.Webhooks {
		webhooks[i] = WebhookView{
			ID:              wh.ID,
			Name:            wh.Name,
			Type:            wh.Type,
			Auth:            wh.Auth,
			Username:        wh.Username,
			URLSet:          strings.TrimSpace(wh.URL) != "",
			TokenSet:        strings.TrimSpace(wh.Token) != "",
			Enabled:         wh.Enabled,
			MessageTemplate: wh.MessageTemplate,
			TitleTemplate:   wh.TitleTemplate,
			CustomFields:    wh.CustomFields,
		}
	}
	interval := "6h"
	if cfg.Schedule.Interval.Duration > 0 {
		interval = config.FormatDuration(cfg.Schedule.Interval.Duration)
	}
	return SettingsView{
		Enabled:   cfg.Enabled,
		PublicURL: cfg.PublicURL,
		Schedule: ScheduleView{
			Mode:     cfg.Schedule.Mode,
			Interval: interval,
			At:       cfg.Schedule.At,
		},
		Webhooks: webhooks,
	}
}

func Normalise(cfg config.NotificationsConfig) (config.NotificationsConfig, error) {
	cfg.PublicURL = strings.TrimRight(strings.TrimSpace(cfg.PublicURL), "/")
	cfg.Schedule.Mode = strings.ToLower(strings.TrimSpace(cfg.Schedule.Mode))
	if cfg.Schedule.Mode == "" {
		cfg.Schedule.Mode = ScheduleInstant
	}
	switch cfg.Schedule.Mode {
	case ScheduleInstant, ScheduleInterval, ScheduleDaily:
	default:
		return cfg, fmt.Errorf("schedule mode must be instant, interval, or daily")
	}
	if cfg.Schedule.Mode == ScheduleInterval {
		if cfg.Schedule.Interval.Duration <= 0 {
			cfg.Schedule.Interval.Duration = 6 * time.Hour
		}
	}
	if cfg.Schedule.Mode == ScheduleDaily {
		cfg.Schedule.At = strings.TrimSpace(cfg.Schedule.At)
		if parts := strings.Split(cfg.Schedule.At, ":"); len(parts) == 3 {
			cfg.Schedule.At = parts[0] + ":" + parts[1]
		}
		if cfg.Schedule.At == "" {
			cfg.Schedule.At = "09:00"
		}
		if _, _, err := parseDailyAt(cfg.Schedule.At); err != nil {
			return cfg, fmt.Errorf("schedule.at: %w", err)
		}
	}
	for i := range cfg.Webhooks {
		wh, err := normaliseWebhook(cfg.Webhooks[i])
		if err != nil {
			return cfg, err
		}
		cfg.Webhooks[i] = wh
	}
	if cfg.Enabled {
		enabled := 0
		for _, wh := range cfg.Webhooks {
			if wh.Enabled && strings.TrimSpace(wh.URL) != "" {
				enabled++
			}
		}
		if enabled == 0 {
			return cfg, fmt.Errorf("at least one enabled webhook with a URL is required when notifications are enabled")
		}
	}
	return cfg, nil
}

func normaliseWebhook(wh config.WebhookConfig) (config.WebhookConfig, error) {
	wh.Name = strings.TrimSpace(wh.Name)
	wh.Type = strings.ToLower(strings.TrimSpace(wh.Type))
	wh.URL = strings.TrimSpace(wh.URL)
	wh.Auth = strings.ToLower(strings.TrimSpace(wh.Auth))
	if wh.Auth == "" {
		wh.Auth = "none"
	}
	switch wh.Type {
	case "discord", "slack", "teams", "custom":
	default:
		return wh, fmt.Errorf("webhook type must be discord, slack, teams, or custom")
	}
	switch wh.Auth {
	case "none", "basic", "bearer":
	default:
		return wh, fmt.Errorf("webhook auth must be none, basic, or bearer")
	}
	if wh.Enabled && wh.URL == "" {
		return wh, fmt.Errorf("webhook URL is required when enabled")
	}
	if wh.Type == "custom" && wh.Enabled {
		hasKey := false
		for _, field := range wh.CustomFields {
			if strings.TrimSpace(field.Key) != "" {
				hasKey = true
				break
			}
		}
		if !hasKey {
			return wh, fmt.Errorf("custom webhook requires at least one field with a key")
		}
	}
	return wh, nil
}

func MergeUpdate(current config.NotificationsConfig, update SettingsUpdate) config.NotificationsConfig {
	next := current
	next.Enabled = update.Enabled
	next.PublicURL = update.PublicURL
	next.Schedule.Mode = update.Schedule.Mode
	if d, err := time.ParseDuration(strings.TrimSpace(update.Schedule.Interval)); err == nil && d > 0 {
		next.Schedule.Interval = config.Duration{Duration: d}
	}
	next.Schedule.At = update.Schedule.At

	byID := map[string]config.WebhookConfig{}
	for _, wh := range current.Webhooks {
		byID[wh.ID] = wh
	}
	next.Webhooks = make([]config.WebhookConfig, 0, len(update.Webhooks))
	for _, row := range update.Webhooks {
		wh := config.WebhookConfig{
			ID:              row.ID,
			Name:            row.Name,
			Type:            row.Type,
			URL:             row.URL,
			Auth:            row.Auth,
			Username:        row.Username,
			Token:           row.Token,
			Enabled:         row.Enabled,
			MessageTemplate: row.MessageTemplate,
			TitleTemplate:   row.TitleTemplate,
			CustomFields:    row.CustomFields,
		}
		if prev, ok := byID[row.ID]; ok {
			if wh.URL == "" {
				wh.URL = prev.URL
			}
			if wh.Token == "" {
				wh.Token = prev.Token
			}
			if wh.ID == "" {
				wh.ID = prev.ID
			}
		}
		if wh.ID == "" {
			wh.ID = newWebhookID()
		}
		next.Webhooks = append(next.Webhooks, wh)
	}
	return next
}

func newWebhookID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func parseDailyAt(at string) (hour int, minute int, err error) {
	parts := strings.Split(at, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected HH:MM")
	}
	h, err := parseInt(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("invalid hour")
	}
	m, err := parseInt(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid minute")
	}
	return h, m, nil
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
