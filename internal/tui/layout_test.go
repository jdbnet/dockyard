package tui

import (
	"strings"
	"testing"
)

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
	if got := m.logsMaxHOffset(); got != 35 {
		t.Fatalf("expected max horizontal offset 35, got %d", got)
	}
}

func TestDetailViewsKeepPanelBorder(t *testing.T) {
	cases := []model{
		{width: 40, height: 16, view: viewLogs, logLines: []string{"hello"}},
		{width: 40, height: 16, view: viewInspect, inspectText: "id: abc"},
		{width: 40, height: 16, view: viewImages, confirm: &confirmDialog{message: "Prune 2 unused image(s)?"}},
		{width: 40, height: 16, view: viewContainers, helpOpen: true},
	}
	for _, m := range cases {
		got := m.render()
		if !strings.Contains(got, "┌") || !strings.Contains(got, "└") {
			t.Fatalf("expected bordered panel, got:\n%s", got)
		}
	}
}

func TestNoticeClearIgnoresStaleTimer(t *testing.T) {
	var m model
	_ = m.pushNotice("saved", "")
	stale := m.noticeID
	_ = m.pushNotice("", "failed")
	if m.noticeID == stale {
		t.Fatal("expected a new notice id")
	}
	next, _ := m.Update(noticeClearMsg{id: stale})
	m = next.(model)
	if m.errMsg != "failed" || m.statusMsg != "" {
		t.Fatalf("stale timer cleared the current notice: status=%q err=%q", m.statusMsg, m.errMsg)
	}
	next, _ = m.Update(noticeClearMsg{id: m.noticeID})
	m = next.(model)
	if m.errMsg != "" || m.statusMsg != "" {
		t.Fatalf("current notice should clear, status=%q err=%q", m.statusMsg, m.errMsg)
	}
}
