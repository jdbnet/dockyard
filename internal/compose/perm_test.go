package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStackIsEditable_managed(t *testing.T) {
	s := Stack{Managed: true}
	if !s.IsEditable() {
		t.Fatal("managed stacks should be editable")
	}
}

func TestStackIsEditable_writableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(path, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Stack{ComposeFile: path}
	if !s.IsEditable() {
		t.Fatal("writable compose file should be editable")
	}
}

func TestStackIsEditable_readOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(path, []byte("services: {}\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	s := Stack{ComposeFile: path}
	if s.IsEditable() {
		t.Fatal("read-only compose file should not be editable")
	}
}
