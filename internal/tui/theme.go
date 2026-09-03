package tui

import "github.com/charmbracelet/lipgloss"

// 256-color foreground palette; backgrounds use the terminal default.
var (
	colorBorder  = lipgloss.Color("238")
	colorText    = lipgloss.Color("252")
	colorMuted   = lipgloss.Color("241")
	colorPrimary = lipgloss.Color("42")
	colorSuccess = lipgloss.Color("42")
	colorWarn    = lipgloss.Color("214")
	colorErr     = lipgloss.Color("203")
	colorInfo    = lipgloss.Color("39")
)

const rowMarkerWidth = 2 // "> " or "  "

var (
	styleHeader = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			Padding(0, 1)
	styleHeaderMuted = lipgloss.NewStyle().Foreground(colorMuted)
	styleStatus      = lipgloss.NewStyle().Foreground(colorMuted)
	styleErr         = lipgloss.NewStyle().Foreground(colorErr)
	styleAccent      = lipgloss.NewStyle().Foreground(colorPrimary)
	styleSelected    = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	styleMuted       = lipgloss.NewStyle().Foreground(colorMuted)
	styleHelp = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)
	styleWarn = lipgloss.NewStyle().Foreground(colorWarn)
	styleTablePanel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)
)

func stateStyle(state string) lipgloss.Style {
	switch state {
	case "running":
		return lipgloss.NewStyle().Foreground(colorSuccess)
	case "restarting", "paused":
		return lipgloss.NewStyle().Foreground(colorWarn)
	case "exited", "dead":
		return lipgloss.NewStyle().Foreground(colorMuted)
	default:
		return lipgloss.NewStyle().Foreground(colorText)
	}
}

func healthStyle(health string) lipgloss.Style {
	switch health {
	case "healthy":
		return lipgloss.NewStyle().Foreground(colorSuccess)
	case "LOOP", "unhealthy":
		return lipgloss.NewStyle().Foreground(colorWarn).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(colorMuted)
	}
}
