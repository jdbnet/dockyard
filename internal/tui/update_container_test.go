package tui

import (
	"testing"

	"github.com/jdbnet/dockyard/internal/engine"
)

func TestContainerRowColsUpdating(t *testing.T) {
	row := rowItem{cols: []string{"web", "stack", "running", "1.2%", "10MB", "1h", "healthy"}}
	cols := containerRowCols(row, true, 0)
	if cols[2] != "⠋ updating" {
		t.Fatalf("expected updating state, got %q", cols[2])
	}
}

func TestPreviousImageLabel(t *testing.T) {
	img := &engine.PreviousImage{ShortID: "abc123", RepoTags: []string{"nginx:latest"}}
	if previousImageLabel(img) != "nginx:latest" {
		t.Fatalf("expected repo tag label")
	}
}
