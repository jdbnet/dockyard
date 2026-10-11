package logging

import "testing"

func TestResumeSkipsShippedLines(t *testing.T) {
	resume := NewResume(Cursor{Timestamp: 100, Count: 2})
	if resume.Keep(99) {
		t.Fatal("kept a line older than the cursor")
	}
	if resume.Keep(100) || resume.Keep(100) {
		t.Fatal("kept a line already shipped at the cursor timestamp")
	}
	if !resume.Keep(100) {
		t.Fatal("dropped a new line at the cursor timestamp")
	}
	if got := resume.Cursor(); got.Timestamp != 100 || got.Count != 3 {
		t.Fatalf("cursor = %+v", got)
	}
	if !resume.Keep(150) {
		t.Fatal("dropped a later line")
	}
	if got := resume.Cursor(); got.Timestamp != 150 || got.Count != 1 {
		t.Fatalf("cursor = %+v", got)
	}
}

func TestParseDockerLine(t *testing.T) {
	ts, text, ok := ParseDockerLine("2026-08-04T17:03:28.578394840Z hello")
	if !ok || text != "hello" || ts <= 0 {
		t.Fatalf("parse = %d %q %v", ts, text, ok)
	}
}
