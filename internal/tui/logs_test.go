package tui

import "testing"

func TestFormatLogLine(t *testing.T) {
	line := `2026-08-04T17:03:28.578394840Z GET /health`
	got := formatLogLine(line, false)
	if got != "GET /health" {
		t.Fatalf("expected message only, got %q", got)
	}
	if formatLogLine(line, true) != line {
		t.Fatal("expected full line with timestamps on")
	}
}

func TestLogsMaxScrollAtBottom(t *testing.T) {
	m := model{
		view:        viewLogs,
		height:      24,
		width:       80,
		logLines:    []string{"a", "b", "c", "d", "e"},
		logAutoScroll: true,
	}
	m.logViewport = m.logsMaxScroll()
	if !m.logsAtBottom() {
		t.Fatalf("expected bottom scroll at %d", m.logViewport)
	}
}
