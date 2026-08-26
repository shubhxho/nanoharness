package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Version is shown in the TUI header; set by cmd before Run.
var Version = "dev"

// Run starts the Superpower TUI. All sends go through internal/harness.
func Run() error {
	p := tea.NewProgram(initialApp(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
