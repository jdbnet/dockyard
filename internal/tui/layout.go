package tui

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func clearScreen() tea.Cmd {
	return func() tea.Msg { return tea.ClearScreen() }
}

func (m model) backToList() (model, tea.Cmd) {
	prev := m.view
	if prev == viewLogs {
		m.stopLogFollow()
	}
	m.view = viewContainers
	m.logLines = nil
	m.inspectText = ""
	m.logViewport = 0
	m.logHOffset = 0
	m.filter = ""
	m.filterActive = false
	m.editMode = editNone
	if prev == viewLogs || prev == viewInspect {
		return m, tea.Batch(refreshRowsCmd(m.eng, viewContainers), clearScreen())
	}
	return m, refreshRowsCmd(m.eng, viewContainers)
}

func (m model) backToStacks() (model, tea.Cmd) {
	m.editMode = editNone
	m.view = viewStacks
	m.cursor = 0
	return m, tea.Batch(refreshRowsCmd(m.eng, viewStacks), clearScreen())
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
	h := m.height - 2 // header + footer
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

func skipDisplayWidth(s string, offset int) string {
	if offset <= 0 {
		return s
	}
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		w := lipgloss.Width(string(r))
		if offset < w {
			break
		}
		offset -= w
		s = s[size:]
	}
	return s
}

func sliceWidth(s string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	return truncateWidth(skipDisplayWidth(s, offset), width)
}

func maxDisplayWidth(lines []string) int {
	maxW := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func horizontalScrollStep(viewportWidth int) int {
	step := viewportWidth / 4
	if step < 4 {
		return 4
	}
	return step
}

func (m model) panelBox(outerH int) (innerW, innerH int) {
	frameX, frameY := styleTablePanel.GetFrameSize()
	return max(1, m.width-frameX), max(1, outerH-frameY)
}

func (m model) logContentWidth() int {
	innerW, _ := m.panelBox(m.contentHeight())
	return innerW
}

// detailVisibleLines is the scrollable area inside the bordered panel, under the one-line hint.
func (m model) detailVisibleLines() int {
	_, innerH := m.panelBox(m.contentHeight())
	visible := innerH - 1
	if visible < 1 {
		return 1
	}
	return visible
}

func (m model) renderPanel(outerH int, content string) string {
	innerW, innerH := m.panelBox(outerH)
	return styleTablePanel.Width(m.width).Height(outerH).Render(fitBox(content, innerW, innerH))
}

func fitBox(content string, width, height int) string {
	raw := strings.Split(content, "\n")
	for len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	if len(raw) > height {
		raw = raw[:height]
	}
	out := make([]string, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(raw) {
			line = raw[i]
		}
		line = truncateWidth(line, width)
		if gap := width - lipgloss.Width(line); gap > 0 {
			line += strings.Repeat(" ", gap)
		}
		out[i] = line
	}
	return strings.Join(out, "\n")
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
