package docker

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/registry"
)

func TestDockerHomeDirWithoutHOME(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("DOCKER_CONFIG", "")
	home := dockerHomeDir()
	if home == "" || home == "/" {
		t.Fatalf("home %q", home)
	}
	if !strings.HasSuffix(dockerConfigPath(), filepath.Join(".docker", "config.json")) {
		t.Fatal(dockerConfigPath())
	}
}

func TestAnnotateRegistryAuthMissingLogin(t *testing.T) {
	err := annotateRegistryAuth("ghcr.io/jdbnet/register:latest", "", os.ErrPermission)
	if err != os.ErrPermission {
		t.Fatalf("non-unauthorized error changed: %v", err)
	}
	denied := errors.New("unauthorized")
	wrapped := annotateRegistryAuth("ghcr.io/jdbnet/register:latest", "", denied)
	if wrapped == nil || !strings.Contains(wrapped.Error(), "no Docker credentials for ghcr.io") {
		t.Fatal(wrapped)
	}
	kept := annotateRegistryAuth("ghcr.io/jdbnet/register:latest", "abc", denied)
	if kept.Error() != "unauthorized" {
		t.Fatal(kept)
	}
}

func TestAuthForImageFromConfig(t *testing.T) {
	dir := t.TempDir()
	auth := base64.StdEncoding.EncodeToString([]byte("jamie:secret"))
	writeDockerConfig(t, dir, dockerConfigFile{
		Auths: map[string]dockerAuthEntry{
			"ghcr.io": {Auth: auth},
		},
	})
	t.Setenv("DOCKER_CONFIG", dir)

	encoded, err := encodedRegistryAuth("ghcr.io/jdbnet/register:latest")
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.DecodeAuthConfig(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "jamie" || got.Password != "secret" {
		t.Fatalf("got %+v", got)
	}
}

func TestAuthForImageMissingRegistry(t *testing.T) {
	dir := t.TempDir()
	writeDockerConfig(t, dir, dockerConfigFile{
		Auths: map[string]dockerAuthEntry{
			"ghcr.io": {Username: "jamie", Password: "secret"},
		},
	})
	t.Setenv("DOCKER_CONFIG", dir)

	encoded, err := encodedRegistryAuth("docker.io/library/nginx:latest")
	if err != nil {
		t.Fatal(err)
	}
	if encoded != "" {
		t.Fatal("expected no auth for a registry that is not logged in")
	}
}

func TestAuthForImageCredentialHelper(t *testing.T) {
	dir := t.TempDir()
	bin := t.TempDir()
	script := filepath.Join(bin, "docker-credential-dockyardtest")
	body := "#!/bin/sh\nprintf '%s' '{\"Username\":\"helperuser\",\"Secret\":\"helperpass\"}'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeDockerConfig(t, dir, dockerConfigFile{
		CredsStore: "dockyardtest",
		Auths: map[string]dockerAuthEntry{
			"ghcr.io": {},
		},
	})
	t.Setenv("DOCKER_CONFIG", dir)

	encoded, err := encodedRegistryAuth("ghcr.io/jdbnet/register:latest")
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.DecodeAuthConfig(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "helperuser" || got.Password != "helperpass" {
		t.Fatalf("got %+v", got)
	}
}

func TestRegistryHost(t *testing.T) {
	if got := registryHost("nginx:latest"); got != "docker.io" {
		t.Fatalf("got %q", got)
	}
	if got := registryHost("ghcr.io/jdbnet/register:latest"); got != "ghcr.io" {
		t.Fatalf("got %q", got)
	}
}

func writeDockerConfig(t *testing.T, dir string, cfg dockerConfigFile) {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
