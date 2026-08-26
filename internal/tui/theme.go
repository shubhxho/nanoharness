package tui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette used across the TUI.
var (
	colorBG      = lipgloss.Color("#1e1e2e")
	colorSurface = lipgloss.Color("#313244")
	colorBorder  = lipgloss.Color("#45475a")
	colorText    = lipgloss.Color("#cdd6f4")
	colorMuted   = lipgloss.Color("#9399b2")
	colorLav     = lipgloss.Color("#b4befe")
	colorTeal    = lipgloss.Color("#94e2d5")
	colorPeach   = lipgloss.Color("#fab387")
	colorYellow  = lipgloss.Color("#f9e2af")
	colorGreen   = lipgloss.Color("#a6e3a1")
	colorRed     = lipgloss.Color("#f38ba8")
)

func chipStyle(c lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c).Background(colorSurface).Bold(true).Padding(0, 1)
}

func roleStyle(role string) lipgloss.Style {
	c := colorPeach
	switch role {
	case "you":
		c = colorTeal
	case "error":
		c = colorRed
	case "context":
		c = colorYellow
	case "nano":
		c = colorLav
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true)
}

func messageBodyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorText)
}

func boxStyle(c lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(c).
		Padding(0, 1).
		MarginBottom(1)
}
