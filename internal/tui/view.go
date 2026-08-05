package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorBg      = lipgloss.Color("#0f1117")
	colorSurface = lipgloss.Color("#161b22")
	colorAccent  = lipgloss.Color("#1ebe8a")
	colorWarn    = lipgloss.Color("#f59e0b")
	colorErr     = lipgloss.Color("#ef4444")
	colorMuted   = lipgloss.Color("#6b7280")
)

var (
	styleBase = lipgloss.NewStyle().Background(colorBg).Foreground(lipgloss.Color("#e5e7eb"))
	styleHeader = lipgloss.NewStyle().
			Background(colorSurface).
			Foreground(colorAccent).
			Bold(true).
			Padding(0, 1)
	styleStatus = lipgloss.NewStyle().Foreground(colorMuted)
	styleErr    = lipgloss.NewStyle().Foreground(colorErr)
	styleAccent = lipgloss.NewStyle().Foreground(colorAccent)
	styleSelected = lipgloss.NewStyle().
			Background(colorSurface).
			Foreground(colorAccent)
	styleHelp = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2).
			Background(colorSurface)
	styleWarn = lipgloss.NewStyle().Foreground(colorWarn)
)

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder
	bodyHeight := m.contentHeight()

	title := viewTitle(m)
	if m.editMode == editCompose {
		title = "compose · " + m.stackName
	} else if m.editMode == editNewStackName {
		title = "new stack"
	}
	b.WriteString(styleHeader.Width(m.width).Render(" dockyard · " + title))
	b.WriteString("\n")

	var body string
	switch {
	case m.helpOpen:
		body = styleHelp.Render(helpText())
		body = m.fillHeight(body, bodyHeight)
	case m.confirm != nil:
		body = m.fillHeight(styleWarn.Render(fmt.Sprintf("Confirm: %s [y/N]", m.confirm.message)), bodyHeight)
	case m.editMode == editNewStackName:
		body = m.fillHeight(m.nameInput.View(), bodyHeight)
	case m.editMode == editCompose:
		body = m.renderComposeEditor(bodyHeight)
	case m.view == viewLogs:
		body = m.renderLogs(bodyHeight)
	case m.view == viewInspect:
		body = m.renderInspect(bodyHeight)
	default:
		body = m.renderTable(bodyHeight)
	}
	b.WriteString(body)

	b.WriteString("\n")
	if m.commandMode {
		b.WriteString(styleAccent.Render(":" + m.commandInput + "█"))
	} else if m.filterActive {
		b.WriteString(styleAccent.Render("/" + m.filter + "█"))
	} else {
		b.WriteString(styleStatus.Render(statusBar(m)))
	}

	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(styleErr.Render(m.errMsg))
	} else if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(styleStatus.Render(m.statusMsg))
	}

	return styleBase.Width(m.width).Render(b.String())
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
	} else if sel := m.selected(); sel != nil && sel.id != "" {
		if !sel.stack.Managed {
			hint = "read-only (external stack)  esc back"
		}
	}
	b.WriteString(styleMuted().Render("  " + hint))
	b.WriteString("\n")
	b.WriteString(m.composeTA.View())
	return m.fillHeight(b.String(), bodyHeight)
}

func (m model) renderTable(bodyHeight int) string {
	rows := m.filteredRows()
	specs := colsForView(m.view)
	widths := computeWidths(specs, rows, m.width)

	var b strings.Builder
	headers := make([]string, len(specs))
	for i, s := range specs {
		headers[i] = s.header
	}

	b.WriteString(styleAccent.Render("  " + formatTableRow(headers, widths)))
	b.WriteString("\n")

	b.WriteString(styleMuted().Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	if len(rows) == 0 {
		b.WriteString(styleMuted().Render("(empty)"))
		return m.fillHeight(b.String(), bodyHeight)
	}

	visible := bodyHeight - 2
	if visible < 1 {
		visible = 1
	}
	start := 0
	if m.cursor >= visible {
		start = m.cursor - visible + 1
	}

	for i, r := range rows {
		if i < start {
			continue
		}
		if i-start >= visible {
			break
		}
		line := formatTableRow(r.cols, widths)
		marker := "  "
		if i == m.cursor {
			marker = "> "
			b.WriteString(styleSelected.Render(marker + line))
		} else {
			b.WriteString(marker + line)
		}
		b.WriteString("\n")
	}

	return m.fillHeight(b.String(), bodyHeight)
}

func (m model) renderLogs(bodyHeight int) string {
	lines := scrollableLines(m, m.logLines)
	visible := m.logsVisibleLines()
	offset := clampScroll(m.logViewport, len(lines), visible)

	var b strings.Builder
	tsFlag, asFlag := logsStatusFlags(m.logShowTS, m.logAutoScroll)
	if len(lines) == 0 {
		b.WriteString(styleMuted().Render("  logs - streaming…"))
	} else {
		b.WriteString(styleMuted().Render(fmt.Sprintf(
			"  logs (%d lines) [ts:%s autoscroll:%s] - t ts  s autoscroll  g/G top/bottom  j/k scroll  / filter  esc back",
			len(lines), tsFlag, asFlag,
		)))
	}
	b.WriteString("\n")

	if len(lines) == 0 {
		return m.fillHeight(b.String(), bodyHeight)
	}

	start := offset
	for i := start; i < len(lines) && i-start < visible; i++ {
		b.WriteString(truncateWidth(formatLogLine(lines[i], m.logShowTS), m.width))
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
	b.WriteString(styleMuted().Render("  inspect - j/k/pgup/pgdn scroll, esc back"))
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
		return "t timestamps  s autoscroll  g/G top/bottom  j/k scroll  / filter  esc/q back"
	case viewInspect:
		return "j/k/pgup/pgdn scroll  esc/q back"
	case viewStacks:
		return "e edit  u update  s up  S down  n new  x delete  j/k nav  esc/q back"
	case viewImages:
		return "x remove  P prune unused  j/k nav  R refresh  :cmd  /filter  ? help  q quit"
	}
	return ":cmd  /filter  j/k  d inspect  l logs  u update  s/S/r  x remove  c stacks  R refresh  ? help  q quit"
}

func helpText() string {
	return `Dockyard TUI - keybindings

  :containers :stacks :images :volumes :networks  Jump to view
  /           Filter current view
  j/k         Navigate
  d / Enter   Inspect container (containers view)
  l           Logs (follow, autoscroll)
  t           Toggle timestamps (logs view)
  s           Toggle autoscroll (logs view)
  g / G       Top / bottom (logs view)
  u           Update image (pull + recreate)
  s S r       Start / Stop / Restart
  x           Remove (confirm)
  P           Prune unused images (images view)
  c           Stacks view (compose files)
  n           New stack (stacks view)
  e           Edit compose file (stacks view)
  R           Force refresh
  q           Quit`
}

func styleMuted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorMuted)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
