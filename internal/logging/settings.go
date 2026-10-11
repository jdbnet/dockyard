package logging

import (
	"fmt"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

// SettingsView is the logging form with secrets removed.
type SettingsView struct {
	Enabled          bool   `json:"enabled"`
	URL              string `json:"url"`
	Type             string `json:"type"`
	Auth             string `json:"auth"`
	Username         string `json:"username"`
	PasswordSet      bool   `json:"password_set"`
	TokenSet         bool   `json:"token_set"`
	CollectByDefault bool   `json:"collect_by_default"`
}

// SettingsUpdate is a form submission. An empty password or token keeps the saved secret.
type SettingsUpdate struct {
	Enabled          bool   `json:"enabled"`
	URL              string `json:"url"`
	Type             string `json:"type"`
	Auth             string `json:"auth"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Token            string `json:"token"`
	CollectByDefault bool   `json:"collect_by_default"`
}

// Status is the shipping health shown on the settings page.
type Status struct {
	State          string    `json:"state"`
	LastSuccess    time.Time `json:"last_success,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	ErrorCount     int64     `json:"error_count"`
	BufferedBytes  int64     `json:"buffered_bytes"`
	Dropped        int64     `json:"dropped"`
	Followed       int       `json:"followed"`
	PauseCode      int       `json:"pause_code,omitempty"`
	NotCollectable []Skipped `json:"not_collectable"`
}

// PublicSettings hides secrets.
func PublicSettings(cfg config.LoggingConfig) SettingsView {
	return SettingsView{
		Enabled:          cfg.Enabled,
		URL:              cfg.URL,
		Type:             cfg.Type,
		Auth:             cfg.Auth,
		Username:         cfg.Username,
		PasswordSet:      cfg.Password != "",
		TokenSet:         cfg.Token != "",
		CollectByDefault: cfg.CollectByDefault,
	}
}

// Normalise checks the logging form and fills default type and auth.
func Normalise(cfg config.LoggingConfig) (config.LoggingConfig, error) {
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.Type = strings.ToLower(strings.TrimSpace(cfg.Type))
	if cfg.Type == "" {
		cfg.Type = "gologs"
	}
	if cfg.Type != "gologs" && cfg.Type != "loki" {
		return cfg, fmt.Errorf("type must be gologs or loki")
	}
	cfg.Auth = strings.ToLower(strings.TrimSpace(cfg.Auth))
	if cfg.Auth == "" {
		cfg.Auth = "none"
	}
	switch cfg.Auth {
	case "none", "basic", "bearer":
	default:
		return cfg, fmt.Errorf("auth must be none, basic, or bearer")
	}
	cfg.Username = strings.TrimSpace(cfg.Username)
	if cfg.Enabled && cfg.URL == "" {
		return cfg, fmt.Errorf("destination URL is required when logging is enabled")
	}
	return cfg, nil
}

// MergeUpdate applies a form submission onto the saved settings.
func MergeUpdate(current config.LoggingConfig, update SettingsUpdate) config.LoggingConfig {
	next := current
	next.Enabled = update.Enabled
	next.URL = update.URL
	next.Type = update.Type
	next.Auth = update.Auth
	next.Username = update.Username
	next.CollectByDefault = update.CollectByDefault
	if update.Password != "" {
		next.Password = update.Password
	}
	if update.Token != "" {
		next.Token = update.Token
	}
	return next
}
