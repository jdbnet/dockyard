package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jdbnet/dockyard/internal/compose"
	"github.com/jdbnet/dockyard/internal/config"
	"github.com/jdbnet/dockyard/internal/docker"
)

func TestStacksKeepsExternalAfterContainersGone(t *testing.T) {
	dir := t.TempDir()
	stacksDir := filepath.Join(dir, "stacks")
	projectDir := filepath.Join(dir, "external")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, compose.ComposeFileName), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Compose: config.ComposeConfig{StacksDir: stacksDir}}
	dc, err := docker.NewClient("/var/run/docker.sock")
	if err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfg, dc)
	if err != nil {
		t.Fatal(err)
	}

	if err := eng.external.Remember("my-external", projectDir); err != nil {
		t.Fatal(err)
	}

	stacks, err := eng.Stacks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stacks) != 1 {
		t.Fatalf("expected 1 external stack, got %d", len(stacks))
	}
	if stacks[0].Name != "my-external" || stacks[0].Managed {
		t.Fatalf("unexpected stack: %+v", stacks[0])
	}
	if stacks[0].ContainerCount != 0 || stacks[0].RunningCount != 0 {
		t.Fatalf("expected zero containers, got %+v", stacks[0])
	}
}

func TestStacksPrunesExternalWhenComposeFileRemoved(t *testing.T) {
	dir := t.TempDir()
	stacksDir := filepath.Join(dir, "stacks")
	projectDir := filepath.Join(dir, "external")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	composePath := filepath.Join(projectDir, compose.ComposeFileName)
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Compose: config.ComposeConfig{StacksDir: stacksDir}}
	dc, err := docker.NewClient("/var/run/docker.sock")
	if err != nil {
		t.Fatal(err)
	}
	eng, err := New(cfg, dc)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.external.Remember("gone", projectDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(composePath); err != nil {
		t.Fatal(err)
	}

	stacks, err := eng.Stacks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stacks) != 0 {
		t.Fatalf("expected stack pruned, got %+v", stacks)
	}
	if len(eng.external.List()) != 0 {
		t.Fatalf("expected registry entry removed")
	}
}
