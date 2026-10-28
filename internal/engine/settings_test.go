package engine

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

func TestUpdateGeneralKeepsPasswordAndRejectsBadDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Auth.Password = "secret"
	e := &Engine{cfg: cfg, tracker: newRestartTracker(3, time.Minute)}
	view := e.GeneralSettings()
	if !view.AuthPasswordSet || view.AuthUsername != "" {
		t.Fatalf("view = %+v", view)
	}

	update := GeneralUpdate{
		DockerSocket:          "/var/run/docker.sock",
		StacksDir:             "/opt/stacks",
		WebEnabled:            true,
		WebPort:               8080,
		WebBind:               "127.0.0.1",
		AuthUsername:          "admin",
		StatsInterval:         "5s",
		StatsBufferSize:       60,
		EventsFallbackRefresh: "30s",
		RestartThreshold:      4,
		RestartWindow:         "10m",
		TUIRefreshRate:        "100ms",
		UpdatesInterval:       "1h",
		UpdatesAutoUpdate:     false,
	}
	saved, err := e.UpdateGeneral(update)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.AuthPasswordSet || cfg.Auth.Password != "secret" || cfg.Auth.Username != "admin" {
		t.Fatalf("password = %q user = %q set = %v", cfg.Auth.Password, cfg.Auth.Username, saved.AuthPasswordSet)
	}
	if cfg.RestartLoop.Threshold != 4 || cfg.RestartLoop.Window.Duration != 10*time.Minute {
		t.Fatalf("restart loop = %+v", cfg.RestartLoop)
	}

	update.StatsInterval = "nope"
	if _, err := e.UpdateGeneral(update); err == nil {
		t.Fatal("expected a duration error")
	}
	if cfg.Auth.Password != "secret" {
		t.Fatal("failed save changed the password")
	}
}
