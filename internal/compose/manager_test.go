package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateStack(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	content := "services:\n  app:\n    image: nginx:alpine\n"
	if err := m.Create("my-stack", content); err != nil {
		t.Fatalf("create stack: %v", err)
	}

	composePath := filepath.Join(dir, "my-stack", ComposeFileName)
	got, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read compose file: %v", err)
	}
	if string(got) != content {
		t.Fatalf("unexpected compose content: %q", string(got))
	}

	stacks, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(stacks) != 1 || stacks[0].Name != "my-stack" {
		t.Fatalf("expected created stack in list, got %+v", stacks)
	}
}

func TestCreateStackAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Create("dup", "services: {}\n"); err != nil {
		t.Fatal(err)
	}
	if err := m.Create("dup", "services: {}\n"); err == nil {
		t.Fatal("expected error creating duplicate stack")
	}
}
