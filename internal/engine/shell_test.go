package engine

import (
	"context"
	"testing"

	"github.com/jdbnet/dockyard/internal/docker"
)

type stubSession struct{}

func (stubSession) Read([]byte) (int, error)  { return 0, nil }
func (stubSession) Write([]byte) (int, error) { return 0, nil }
func (stubSession) Size() (uint, uint)       { return 80, 24 }

func TestShellRejectsStoppedContainer(t *testing.T) {
	eng := &Engine{store: newStore()}
	eng.store.setContainers([]Container{{
		ID:    "sha256:abc",
		Name:  "web",
		State: "exited",
	}})

	err := eng.Shell(context.Background(), "sha256:abc", stubSession{})
	if err == nil || err.Error() != `container "web" is not running` {
		t.Fatalf("expected not running error, got %v", err)
	}
}

func TestShellRejectsMissingContainer(t *testing.T) {
	eng := &Engine{store: newStore()}
	err := eng.Shell(context.Background(), "missing", stubSession{})
	if err == nil || err.Error() != "container not found" {
		t.Fatalf("expected not found error, got %v", err)
	}
}

var _ docker.TerminalSession = stubSession{}
