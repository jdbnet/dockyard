package config

import (
	"os"
	"path/filepath"
	"testing"
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
