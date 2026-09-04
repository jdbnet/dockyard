package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	w := m.width
	bodyHeight := m.contentHeight()

	title := viewTitle(m)
	if m.editMode == editCompose {
		title = "compose · " + m.stackName
	} else if m.editMode == editNewStackName {
		title = "new stack"
	}

	header := m.renderHeader(w, title)

	var bodyLines []string
	switch {
	case m.helpOpen:
		body := styleHelp.Render(helpText())
		bodyLines = strings.Split(m.fillHeight(body, bodyHeight), "\n")
	case m.confirm != nil:
		bodyLines = strings.Split(
			m.fillHeight(styleWarn.Render(fmt.Sprintf("Confirm: %s [y/N]", m.confirm.message)), bodyHeight),
			"\n",
		)
	case m.editMode == editNewStackName:
		bodyLines = strings.Split(m.fillHeight(m.nameInput.View(), bodyHeight), "\n")
	case m.editMode == editCompose:
		bodyLines = strings.Split(m.renderComposeEditor(bodyHeight), "\n")
	case m.view == viewLogs:
		bodyLines = strings.Split(m.renderLogs(bodyHeight), "\n")
	case m.view == viewInspect:
		bodyLines = strings.Split(m.renderInspect(bodyHeight), "\n")
	default:
		panel := strings.TrimRight(m.renderTablePanel(bodyHeight), "\n")
		bodyLines = strings.Split(panel, "\n")
	}

	var footer string
	if m.commandMode {
		footer = m.renderCommandInput(w)
	} else if m.filterActive {
		footer = styleAccent.Width(w).Padding(0, 1).Render("/" + m.filter + "█")
	} else {
		footer = styleStatus.Width(w).Padding(0, 1).Render(statusBar(m))
	}

	out := make([]string, 0, m.height)
	out = append(out, header)
	out = append(out, bodyLines...)
	out = append(out, footer)

	if m.errMsg != "" {
		out = append(out, styleErr.Width(w).Padding(0, 1).Render(m.errMsg))
	} else if m.statusMsg != "" {
		out = append(out, styleStatus.Width(w).Padding(0, 1).Render(m.statusMsg))
	}

	for len(out) < m.height {
		out = append(out, strings.Repeat(" ", w))
	}

	return strings.Join(out, "\n")
}

func viewTitle(m model) string {
	switch m.view {
	case viewStacks:
		return "stacks"
	case viewImages:
		return "images"
	case viewVolumes:
		return "volumes"
	case viewNetworks:
		return "networks"
	case viewPorts:
		return "ports"
	case viewLogs:
		return "logs"
	case viewInspect:
		return "inspect"
	default:
		return "containers"
	}
}

func (m model) renderComposeEditor(bodyHeight int) string {
	var b strings.Builder
	hint := "ctrl+s save  esc back"
	if m.stackNew {
		hint += "  (new stack)"
	} else if !m.stackEditable {
		hint = "read-only  esc back"
	}
	b.WriteString(styleMuted.Render("  " + hint))
	b.WriteString("\n")
	b.WriteString(m.composeTA.View())
	return m.fillHeight(b.String(), bodyHeight)
}

func (m model) renderTablePanel(outerH int) string {
	rows := m.filteredRows()
	specs := colsForView(m.view)
	w := m.width

	panel := styleTablePanel
	frameX := panel.GetHorizontalFrameSize()
	frameY := panel.GetVerticalFrameSize()
	innerW := max(1, w-frameX)
	innerH := max(1, outerH-frameY)

	contentW := innerW - rowMarkerWidth
	widths := computeWidths(specs, rows, contentW)

	headers := make([]string, len(specs))
	for i, s := range specs {
		headers[i] = s.header
	}

	headerText := strings.Repeat(" ", rowMarkerWidth) + formatTableRow(headers, widths)
	dividerText := strings.Repeat(" ", rowMarkerWidth) + strings.Repeat("─", tableLineWidth(widths))

	var lines []string
	if innerH >= 1 {
		lines = append(lines, styleMuted.Bold(true).Render(headerText))
	}
	if innerH >= 2 {
		lines = append(lines, lipgloss.NewStyle().Foreground(colorBorder).Render(dividerText))
	}

	if len(rows) == 0 {
		if innerH >= 3 {
			lines = append(lines, styleMuted.Render(strings.Repeat(" ", rowMarkerWidth)+"(empty)"))
		}
		for len(lines) < innerH {
			lines = append(lines, "")
		}
		return panel.Width(w).Render(strings.Join(lines, "\n"))
	}

	dataSlots := innerH - len(lines)
	start := 0
	if dataSlots > 0 && m.cursor >= dataSlots {
		start = m.cursor - dataSlots + 1
	}

	rowCount := 0
	for i, r := range rows {
		if rowCount >= dataSlots {
			break
		}
		if i < start {
			continue
		}
		cols := containerRowCols(r, m.view == viewContainers && m.updating[r.id], m.spinnerTick)
		line := formatStyledTableRow(m.view, specs, cols, widths, r)
		if i == m.cursor {
			line = styleSelected.Render("> ") + line
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
		rowCount++
	}

	for len(lines) < innerH {
		lines = append(lines, "")
	}

	return panel.Width(w).Render(strings.Join(lines, "\n"))
}

func (m model) renderLogs(bodyHeight int) string {
	lines := scrollableLines(m, m.logLines)
	visible := m.logsVisibleLines()
	offset := clampScroll(m.logViewport, len(lines), visible)
	hOffset := m.clampLogHOffset()

	var b strings.Builder
	tsFlag, asFlag := logsStatusFlags(m.logShowTS, m.logAutoScroll)
	if len(lines) == 0 {
		b.WriteString(styleMuted.Render("  logs - streaming…"))
	} else {
		b.WriteString(styleMuted.Render(fmt.Sprintf(
			"  logs (%d lines) [ts:%s autoscroll:%s] - t ts  s autoscroll  g/G top/bottom  ↑↓←→ scroll  / filter  esc back",
			len(lines), tsFlag, asFlag,
		)))
	}
	b.WriteString("\n")

	if len(lines) == 0 {
		return m.fillHeight(b.String(), bodyHeight)
	}

	start := offset
	for i := start; i < len(lines) && i-start < visible; i++ {
		b.WriteString(sliceWidth(formatLogLine(lines[i], m.logShowTS), hOffset, m.width))
		b.WriteString("\n")
	}

	return m.fillHeight(b.String(), bodyHeight)
}

func (m model) renderInspect(bodyHeight int) string {
	lines := strings.Split(m.inspectText, "\n")
	visible := bodyHeight - 1
	if visible < 1 {
		visible = 1
	}
	offset := clampScroll(m.logViewport, len(lines), visible)

	var b strings.Builder
	b.WriteString(styleMuted.Render("  inspect - j/k/pgup/pgdn scroll, esc back"))
	b.WriteString("\n")

	start := offset
	for i := start; i < len(lines) && i-start < visible; i++ {
		b.WriteString(truncateWidth(lines[i], m.width))
		b.WriteString("\n")
	}

	return m.fillHeight(b.String(), bodyHeight)
}

func statusBar(m model) string {
	if m.editMode == editCompose {
		return "ctrl+s save  esc back"
	}
	if m.editMode == editNewStackName {
		return "enter confirm name  esc cancel"
	}
	switch m.view {
	case viewLogs:
		return "t timestamps  s autoscroll  g/G top/bottom  ↑↓←→ scroll  / filter  esc/q back"
	case viewInspect:
		return "j/k/pgup/pgdn scroll  esc/q back"
	case viewStacks:
		return "e edit  u update  s up  S down  n new  x delete  j/k nav  esc/q back"
	case viewImages:
		return "x remove  P prune unused  j/k nav  R refresh  :cmd  /filter  ? help  q quit"
	case viewPorts:
		return "d inspect  j/k nav  R refresh  :cmd  /filter  ? help  q quit"
	}
	return ":cmd  /filter  j/k  d inspect  l logs  u update  s/S/r  x remove  c stacks  p ports  R refresh  ? help  q quit"
}

func (m model) renderHeader(w int, title string) string {
	left := " dockyard" + styleHeaderMuted.Render("/"+title)
	frame := styleHeader.Padding(0, 1)
	if m.version == "" {
		return frame.Width(w).Render(left)
	}
	ver := styleHeaderMuted.Render(m.version)
	innerW := w - frame.GetHorizontalFrameSize()
	gap := innerW - lipgloss.Width(left) - lipgloss.Width(ver)
	if gap < 1 {
		gap = 1
	}
	return frame.Width(w).Render(left + strings.Repeat(" ", gap) + ver)
}

func helpText() string {
	return `Dockyard TUI - keybindings

  :containers :stacks :images :volumes :networks :ports  Jump to view
  /           Filter current view
  tab         Autocomplete : command
  j/k         Navigate
  d / Enter   Inspect container (containers view)
  l           Logs (follow, autoscroll)
  t           Toggle timestamps (logs view)
  s           Toggle autoscroll (logs view)
  g / G       Top / bottom (logs view)
  ↑↓←→        Scroll logs (j/k/h/l also work)
  H / L       Start / end of line (logs view)
  u           Update image (pull + recreate)
  s S r       Start / Stop / Restart
  x           Remove (confirm)
  P           Prune unused images (images view)
  c           Stacks view (compose files)
  p           Ports view
  n           New stack (stacks view)
  e           Edit compose file (stacks view)
  R           Force refresh
  q           Quit`
}

func (m model) renderCommandInput(w int) string {
	input := m.commandInput
	suffix := execSuggestion(input)
	if suffix == "" {
		_, suffix = commandSuggestion(input)
	}
	line := styleAccent.Render(":" + input + "█") + styleMuted.Render(suffix)
	return lipgloss.NewStyle().Width(w).Padding(0, 1).Render(line)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
