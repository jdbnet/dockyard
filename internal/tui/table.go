package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const colSep = " │ "

type colSpec struct {
	header string
	min    int
	max    int // 0 = flexible (uses remaining terminal width)
}

func containerCols() []colSpec {
	return []colSpec{
		{header: "NAME", min: 12, max: 0},
		{header: "STACK", min: 10, max: 0},
		{header: "STATE", min: 8, max: 10},
		{header: "CPU", min: 6, max: 7},
		{header: "MEM", min: 8, max: 10},
		{header: "UPTIME", min: 6, max: 8},
		{header: "HEALTH", min: 8, max: 10},
	}
}

func imageCols() []colSpec {
	return []colSpec{
		{header: "ID", min: 12, max: 14},
		{header: "TAGS", min: 16, max: 0},
		{header: "SIZE", min: 8, max: 10},
		{header: "UNUSED", min: 8, max: 8},
		{header: "CTRS", min: 4, max: 6},
	}
}

func volumeCols() []colSpec {
	return []colSpec{
		{header: "NAME", min: 16, max: 0},
		{header: "DRIVER", min: 8, max: 12},
		{header: "SCOPE", min: 8, max: 10},
		{header: "UNUSED", min: 8, max: 8},
	}
}

func networkCols() []colSpec {
	return []colSpec{
		{header: "NAME", min: 16, max: 0},
		{header: "DRIVER", min: 8, max: 12},
		{header: "SCOPE", min: 8, max: 10},
		{header: "CTRS", min: 4, max: 6},
	}
}

func portCols() []colSpec {
	return []colSpec{
		{header: "HOST", min: 6, max: 8},
		{header: "CTR", min: 6, max: 8},
		{header: "PROTO", min: 5, max: 6},
		{header: "CONTAINER", min: 12, max: 0},
		{header: "STACK", min: 10, max: 0},
		{header: "SERVICE", min: 10, max: 0},
		{header: "STATE", min: 8, max: 10},
	}
}

func colsForView(v viewKind) []colSpec {
	switch v {
	case viewStacks:
		return stackCols()
	case viewImages:
		return imageCols()
	case viewVolumes:
		return volumeCols()
	case viewNetworks:
		return networkCols()
	case viewPorts:
		return portCols()
	default:
		return containerCols()
	}
}

func computeWidths(specs []colSpec, rows []rowItem, contentW int) []int {
	widths := make([]int, len(specs))
	for i, s := range specs {
		widths[i] = max(len(s.header), s.min)
		if s.max > 0 && widths[i] > s.max {
			widths[i] = s.max
		}
	}
	for _, row := range rows {
		for i, val := range row.cols {
			if i >= len(widths) {
				break
			}
			w := len(val)
			if w > widths[i] {
				widths[i] = w
			}
			if specs[i].max > 0 && widths[i] > specs[i].max {
				widths[i] = specs[i].max
			}
		}
	}

	if contentW > 0 {
		used := tableLineWidth(widths)
		if used < contentW {
			distributeExtra(widths, specs, contentW-used)
		}
	}
	return widths
}

func distributeExtra(widths []int, specs []colSpec, extra int) {
	if extra <= 0 {
		return
	}
	var flex []int
	total := 0
	for i, s := range specs {
		if s.max == 0 {
			flex = append(flex, i)
			total += widths[i]
		}
	}
	if len(flex) == 0 {
		widths[len(widths)-1] += extra
		return
	}
	if total == 0 {
		each := extra / len(flex)
		for _, i := range flex {
			widths[i] += each
		}
		widths[flex[len(flex)-1]] += extra - each*len(flex)
		return
	}
	remaining := extra
	for n, i := range flex {
		if n == len(flex)-1 {
			widths[i] += remaining
			continue
		}
		add := extra * widths[i] / total
		widths[i] += add
		remaining -= add
	}
}

func padCell(val string, width int) string {
	if width <= 0 {
		return val
	}
	if len(val) > width {
		if width <= 1 {
			return val[:width]
		}
		return val[:width-3] + "..."
	}
	return fmt.Sprintf("%-*s", width, val)
}

func formatTableRow(cols []string, widths []int) string {
	parts := make([]string, len(widths))
	for i := range widths {
		val := ""
		if i < len(cols) {
			val = cols[i]
		}
		parts[i] = padCell(val, widths[i])
	}
	return strings.Join(parts, colSep)
}

func tableLineWidth(widths []int) int {
	if len(widths) == 0 {
		return 0
	}
	n := 0
	for _, w := range widths {
		n += w
	}
	n += (len(widths) - 1) * len(colSep)
	return n
}

func formatStyledTableRow(view viewKind, specs []colSpec, cols []string, widths []int, row rowItem) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		val := ""
		if i < len(cols) {
			val = cols[i]
		}
		padded := padCell(val, w)
		parts[i] = styleTableCell(view, specs, i, val, row).Render(padded)
	}
	return strings.Join(parts, colSep)
}

func styleTableCell(view viewKind, specs []colSpec, col int, val string, row rowItem) lipgloss.Style {
	header := ""
	if col < len(specs) {
		header = specs[col].header
	}

	switch view {
	case viewContainers:
		switch col {
		case 0:
			return lipgloss.NewStyle().Foreground(colorText)
		case 1:
			return lipgloss.NewStyle().Foreground(colorInfo)
		case 2:
			if strings.Contains(val, "updating") {
				return lipgloss.NewStyle().Foreground(colorWarn)
			}
			return stateStyle(val)
		case 5:
			return lipgloss.NewStyle().Foreground(colorMuted)
		case 6:
			return healthStyle(val)
		default:
			return lipgloss.NewStyle().Foreground(colorText)
		}
	case viewStacks:
		switch header {
		case "SRC":
			return lipgloss.NewStyle().Foreground(colorInfo)
		case "PATH":
			return lipgloss.NewStyle().Foreground(colorMuted)
		default:
			return lipgloss.NewStyle().Foreground(colorText)
		}
	case viewImages:
		if header == "UNUSED" && val == "unused" {
			return lipgloss.NewStyle().Foreground(colorWarn)
		}
		return lipgloss.NewStyle().Foreground(colorText)
	case viewVolumes:
		if header == "UNUSED" && val == "unused" {
			return lipgloss.NewStyle().Foreground(colorWarn)
		}
		return lipgloss.NewStyle().Foreground(colorText)
	case viewPorts:
		if header == "STATE" {
			return stateStyle(val)
		}
		if header == "STACK" {
			return lipgloss.NewStyle().Foreground(colorInfo)
		}
		return lipgloss.NewStyle().Foreground(colorText)
	default:
		return lipgloss.NewStyle().Foreground(colorText)
	}
}

func tableWidth(widths []int) int {
	return tableLineWidth(widths) + rowMarkerWidth
}
