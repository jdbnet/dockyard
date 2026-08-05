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
