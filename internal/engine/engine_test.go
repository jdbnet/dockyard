package engine

import (
	"testing"
	"time"
)

func TestRingBuffer(t *testing.T) {
	rb := newRingBuffer(3)
	rb.add(StatPoint{CPUPct: 1})
	rb.add(StatPoint{CPUPct: 2})
	rb.add(StatPoint{CPUPct: 3})
	rb.add(StatPoint{CPUPct: 4})

	snap := rb.snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 points, got %d", len(snap))
	}
	if snap[0].CPUPct != 2 || snap[2].CPUPct != 4 {
		t.Fatalf("unexpected order: %+v", snap)
	}
}

func TestRestartTracker(t *testing.T) {
	rt := newRestartTracker(3, time.Minute)
	now := time.Now()
	if rt.record("c1", now) {
		t.Fatal("should not loop on first event")
	}
	if rt.record("c1", now.Add(10*time.Second)) {
		t.Fatal("should not loop on second event")
	}
	if !rt.record("c1", now.Add(20*time.Second)) {
		t.Fatal("should detect loop on third event")
	}
}

func TestFormatBytes(t *testing.T) {
	if FormatBytes(0) != "-" {
		t.Fatalf("expected dash for zero")
	}
	if FormatBytes(512) != "512 B" {
		t.Fatalf("got %q", FormatBytes(512))
	}
	if FormatBytes(1536) != "1.5 KB" {
		t.Fatalf("got %q", FormatBytes(1536))
	}
}

func TestGroupComposeProjects(t *testing.T) {
	containers := []Container{
		{Name: "b", ComposeProject: "stack", ComposeService: "api"},
		{Name: "a", ComposeProject: "stack", ComposeService: "web"},
		{Name: "solo", ComposeProject: ""},
	}
	projects := groupComposeProjects(containers)
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	var stack *ComposeProject
	for i := range projects {
		if projects[i].Name == "stack" {
			stack = &projects[i]
		}
	}
	if stack == nil || len(stack.Containers) != 2 {
		t.Fatalf("unexpected stack grouping: %+v", projects)
	}
}
