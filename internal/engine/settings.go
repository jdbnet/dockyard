package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

// GeneralSettings is the settings form with the login password removed.
type GeneralSettings struct {
	DockerSocket          string `json:"docker_socket"`
	StacksDir             string `json:"stacks_dir"`
	WebEnabled            bool   `json:"web_enabled"`
	WebPort               int    `json:"web_port"`
	WebBind               string `json:"web_bind"`
	AuthUsername          string `json:"auth_username"`
	AuthPasswordSet       bool   `json:"auth_password_set"`
	StatsInterval         string `json:"stats_interval"`
	StatsBufferSize       int    `json:"stats_buffer_size"`
	EventsFallbackRefresh string `json:"events_fallback_refresh"`
	RestartThreshold      int    `json:"restart_threshold"`
	RestartWindow         string `json:"restart_window"`
	TUIRefreshRate        string `json:"tui_refresh_rate"`
	UpdatesInterval       string `json:"updates_interval"`
}

// GeneralUpdate is a settings form submission. An empty auth password keeps the saved one.
type GeneralUpdate struct {
	DockerSocket          string `json:"docker_socket"`
	StacksDir             string `json:"stacks_dir"`
	WebEnabled            bool   `json:"web_enabled"`
	WebPort               int    `json:"web_port"`
	WebBind               string `json:"web_bind"`
	AuthUsername          string `json:"auth_username"`
	AuthPassword          string `json:"auth_password"`
	StatsInterval         string `json:"stats_interval"`
	StatsBufferSize       int    `json:"stats_buffer_size"`
	EventsFallbackRefresh string `json:"events_fallback_refresh"`
	RestartThreshold      int    `json:"restart_threshold"`
	RestartWindow         string `json:"restart_window"`
	TUIRefreshRate        string `json:"tui_refresh_rate"`
	UpdatesInterval       string `json:"updates_interval"`
}

func PublicGeneral(cfg *config.Config) GeneralSettings {
	return GeneralSettings{
		DockerSocket:          cfg.Docker.Socket,
		StacksDir:             cfg.Compose.StacksDir,
		WebEnabled:            cfg.Web.Enabled,
		WebPort:               cfg.Web.Port,
		WebBind:               cfg.Web.Bind,
		AuthUsername:          cfg.Auth.Username,
		AuthPasswordSet:       cfg.Auth.Password != "",
		StatsInterval:         config.FormatDuration(cfg.Stats.Interval.Duration),
		StatsBufferSize:       cfg.Stats.BufferSize,
		EventsFallbackRefresh: config.FormatDuration(cfg.Events.FallbackRefresh.Duration),
		RestartThreshold:      cfg.RestartLoop.Threshold,
		RestartWindow:         config.FormatDuration(cfg.RestartLoop.Window.Duration),
		TUIRefreshRate:        config.FormatDuration(cfg.TUI.RefreshRate.Duration),
		UpdatesInterval:       config.FormatDuration(cfg.Updates.Interval.Duration),
	}
}

func (e *Engine) GeneralSettings() GeneralSettings {
	return PublicGeneral(e.cfg)
}

func (e *Engine) UpdateGeneral(update GeneralUpdate) (GeneralSettings, error) {
	statsInterval, err := parseDuration("stats interval", update.StatsInterval)
	if err != nil {
		return GeneralSettings{}, err
	}
	fallback, err := parseDuration("event refresh", update.EventsFallbackRefresh)
	if err != nil {
		return GeneralSettings{}, err
	}
	restartWindow, err := parseDuration("restart window", update.RestartWindow)
	if err != nil {
		return GeneralSettings{}, err
	}
	tuiRate, err := parseDuration("TUI refresh", update.TUIRefreshRate)
	if err != nil {
		return GeneralSettings{}, err
	}
	updatesInterval, err := parseDuration("update scan interval", update.UpdatesInterval)
	if err != nil {
		return GeneralSettings{}, err
	}

	socket := strings.TrimSpace(update.DockerSocket)
	stacks := strings.TrimSpace(update.StacksDir)
	bind := strings.TrimSpace(update.WebBind)
	username := strings.TrimSpace(update.AuthUsername)
	switch {
	case socket == "":
		return GeneralSettings{}, fmt.Errorf("Docker socket is required")
	case stacks == "":
		return GeneralSettings{}, fmt.Errorf("stacks directory is required")
	case bind == "":
		return GeneralSettings{}, fmt.Errorf("web bind address is required")
	case update.WebPort < 1 || update.WebPort > 65535:
		return GeneralSettings{}, fmt.Errorf("web port must be between 1 and 65535")
	case update.StatsBufferSize < 1:
		return GeneralSettings{}, fmt.Errorf("stats buffer size must be at least 1")
	case update.RestartThreshold < 1:
		return GeneralSettings{}, fmt.Errorf("restart threshold must be at least 1")
	case username != "" && update.AuthPassword == "" && e.cfg.Auth.Password == "":
		return GeneralSettings{}, fmt.Errorf("password is required when a username is set")
	}

	e.cfg.Docker.Socket = socket
	e.cfg.Compose.StacksDir = stacks
	e.cfg.Web.Enabled = update.WebEnabled
	e.cfg.Web.Port = update.WebPort
	e.cfg.Web.Bind = bind
	e.cfg.Auth.Username = username
	if update.AuthPassword != "" {
		e.cfg.Auth.Password = update.AuthPassword
	}
	e.cfg.Stats.Interval.Duration = statsInterval
	e.cfg.Stats.BufferSize = update.StatsBufferSize
	e.cfg.Events.FallbackRefresh.Duration = fallback
	e.cfg.RestartLoop.Threshold = update.RestartThreshold
	e.cfg.RestartLoop.Window.Duration = restartWindow
	e.cfg.TUI.RefreshRate.Duration = tuiRate
	e.cfg.Updates.Interval.Duration = updatesInterval
	e.tracker.configure(update.RestartThreshold, restartWindow)

	if err := e.cfg.SaveGeneral(); err != nil {
		return GeneralSettings{}, err
	}
	return PublicGeneral(e.cfg), nil
}

func parseDuration(name, value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a duration such as 5s or 5m", name)
	}
	return parsed, nil
}
