package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) backToList() (model, tea.Cmd) {
	prev := m.view
	if prev == viewLogs {
		m.stopLogFollow()
	}
	m.view = viewContainers
	m.logLines = nil
	m.inspectText = ""
	m.logViewport = 0
	m.filter = ""
	m.filterActive = false
	m.editMode = editNone
	if prev == viewLogs || prev == viewInspect {
		return m, tea.Batch(refreshRowsCmd(m.eng, viewContainers), tea.ClearScreen)
	}
	return m, refreshRowsCmd(m.eng, viewContainers)
}

func (m model) backToStacks() (model, tea.Cmd) {
	m.editMode = editNone
	m.view = viewStacks
	m.cursor = 0
	return m, tea.Batch(refreshRowsCmd(m.eng, viewStacks), tea.ClearScreen)
}

func scrollableLines(m model, raw []string) []string {
	if m.filter == "" {
		return raw
	}
	f := strings.ToLower(m.filter)
	var out []string
	for _, l := range raw {
		if strings.Contains(strings.ToLower(l), f) {
			out = append(out, l)
		}
	}
	return out
}

func (m model) contentHeight() int {
	h := m.height - 4
	if m.confirm != nil {
		h--
	}
	if m.helpOpen {
		h -= 8
	}
	if m.errMsg != "" || m.statusMsg != "" {
		h--
	}
	if h < 1 {
		return 1
	}
	return h
}

func (m model) scrollPageSize() int {
	return m.contentHeight()
}

func clampScroll(offset, total, visible int) int {
	if visible < 1 {
		visible = 1
	}
	maxOff := total - visible
	if maxOff < 0 {
		maxOff = 0
	}
	if offset > maxOff {
		return maxOff
	}
	if offset < 0 {
		return 0
	}
	return offset
}

func (m model) fillHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	lineStyle := lipgloss.NewStyle().Background(colorBg).Width(m.width)
	padded := make([]string, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		if lipgloss.Width(line) < m.width {
			line = line + strings.Repeat(" ", m.width-lipgloss.Width(line))
		}
		padded[i] = lineStyle.Render(truncateWidth(line, m.width))
	}
	return strings.Join(padded, "\n")
}

func truncateWidth(s string, w int) string {
	if w <= 0 {
		return s
	}
	for lipgloss.Width(s) > w {
		s = s[:len(s)-1]
	}
	return s
}
