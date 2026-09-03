package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExternalRegistryRememberForget(t *testing.T) {
	dir := t.TempDir()
	stacksDir := filepath.Join(dir, "stacks")
	projectDir := filepath.Join(dir, "external-proj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ComposeFileName), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := NewExternalRegistry(stacksDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Remember("my-stack", projectDir); err != nil {
		t.Fatal(err)
	}

	entries := reg.List()
	if len(entries) != 1 || entries[0].Project != "my-stack" || entries[0].Path != projectDir {
		t.Fatalf("unexpected entries: %+v", entries)
	}

	reg2, err := NewExternalRegistry(stacksDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reg2.List(); len(got) != 1 {
		t.Fatalf("expected persisted entry, got %+v", got)
	}

	if err := reg2.Forget("my-stack"); err != nil {
		t.Fatal(err)
	}
	if len(reg2.List()) != 0 {
		t.Fatalf("expected empty registry after forget")
	}
}

func TestExternalRegistryPrunesMissingComposeFile(t *testing.T) {
	dir := t.TempDir()
	stacksDir := filepath.Join(dir, "stacks")
	projectDir := filepath.Join(dir, "gone-proj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	composePath := filepath.Join(projectDir, ComposeFileName)
	if err := os.WriteFile(composePath, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := NewExternalRegistry(stacksDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Remember("gone", projectDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(composePath); err != nil {
		t.Fatal(err)
	}

	_, err = StackFromPath("gone", projectDir)
	if err == nil {
		t.Fatal("expected missing compose file error")
	}
}
