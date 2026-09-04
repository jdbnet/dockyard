package tui

import "testing"

func TestSliceWidth(t *testing.T) {
	line := "0123456789abcdef"
	if got := sliceWidth(line, 0, 8); got != "01234567" {
		t.Fatalf("expected first 8 chars, got %q", got)
	}
	if got := sliceWidth(line, 8, 8); got != "89abcdef" {
		t.Fatalf("expected offset slice, got %q", got)
	}
	if got := sliceWidth(line, 20, 8); got != "" {
		t.Fatalf("expected empty slice past end, got %q", got)
	}
}

func TestLogsMaxHOffset(t *testing.T) {
	m := model{
		view:     viewLogs,
		width:    10,
		logLines: []string{"short", "this line is much wider than the viewport"},
	}
	if got := m.logsMaxHOffset(); got != 31 {
		t.Fatalf("expected max horizontal offset 31, got %d", got)
	}
}
