package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Docker.Socket != "/var/run/docker.sock" {
		t.Fatalf("unexpected socket: %s", cfg.Docker.Socket)
	}
	if cfg.Web.Port != 8080 {
		t.Fatalf("unexpected port: %d", cfg.Web.Port)
	}
	if cfg.Compose.StacksDir != "/opt/stacks" {
		t.Fatalf("unexpected stacks dir: %s", cfg.Compose.StacksDir)
	}
	if cfg.Updates.Interval.Duration != time.Hour {
		t.Fatalf("unexpected updates interval: %s", cfg.Updates.Interval.Duration)
	}
	if !cfg.Logging.CollectByDefault || cfg.Logging.Enabled || cfg.Logging.Type != "gologs" || cfg.Logging.Auth != "none" {
		t.Fatalf("unexpected logging defaults: %+v", cfg.Logging)
	}
}

func TestSaveLoggingPreservesOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("docker:\n  socket: /tmp/docker.sock\nauth:\n  username: admin\n  password: secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	err = cfg.SaveLogging(LoggingConfig{
		Enabled:          true,
		URL:              "http://logs.example/loki/api/v1/push",
		Type:             "loki",
		Auth:             "bearer",
		Token:            "tok",
		CollectByDefault: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "/tmp/docker.sock") || !strings.Contains(text, "secret") || !strings.Contains(text, "tok") {
		t.Fatalf("config lost data:\n%s", text)
	}
	again, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Docker.Socket != "/tmp/docker.sock" || again.Auth.Password != "secret" {
		t.Fatalf("reloaded config = %+v", again)
	}
	if again.Logging.Token != "tok" || again.Logging.CollectByDefault || again.Logging.Type != "loki" {
		t.Fatalf("logging = %+v", again.Logging)
	}
}

func TestSaveGeneralPreservesLogging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("logging:\n  token: hidden\n  type: gologs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Stats.Interval.Duration = 5 * time.Minute
	cfg.Auth.Password = "secret"
	if err := cfg.SaveGeneral(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "token: hidden") || !strings.Contains(text, "secret") {
		t.Fatalf("config lost data:\n%s", text)
	}
	if !strings.Contains(text, "interval: 5m\n") && !strings.Contains(text, "interval: 5m\r\n") {
		t.Fatalf("duration was not written as 5m:\n%s", text)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("DOCKYARD_WEB_PORT", "9000")
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Web.Port != 9000 {
		t.Fatalf("expected port 9000, got %d", cfg.Web.Port)
	}
	os.Unsetenv("DOCKYARD_WEB_PORT")
}

func TestUpdatesIntervalEnv(t *testing.T) {
	t.Setenv("DOCKYARD_UPDATES_INTERVAL", "15m")
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Updates.Interval.Duration != 15*time.Minute {
		t.Fatalf("expected 15m, got %s", cfg.Updates.Interval.Duration)
	}
}

func TestDefaultAutoUpdateSettings(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Updates.AutoUpdate {
		t.Fatal("autoupdate should default false")
	}
	if cfg.Updates.AutoUpdateInterval.Duration != 0 {
		t.Fatal("autoupdate_interval should default zero")
	}
}

func TestWebBindIsLoopback(t *testing.T) {
	cfg := defaultConfig()
	if !cfg.WebBindIsLoopback() {
		t.Fatal("default bind should be loopback")
	}
	cfg.Web.Bind = "0.0.0.0"
	if cfg.WebBindIsLoopback() {
		t.Fatal("0.0.0.0 is not loopback")
	}
	cfg.Web.Bind = "localhost"
	if !cfg.WebBindIsLoopback() {
		t.Fatal("localhost should be loopback")
	}
}

func TestAuthEnabled(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AuthEnabled() {
		t.Fatal("auth should be disabled by default")
	}
	cfg.Auth.Username = "admin"
	cfg.Auth.Password = "secret"
	if !cfg.AuthEnabled() {
		t.Fatal("auth should be enabled when credentials set")
	}
}
