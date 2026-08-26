package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/shubhxho/nanoharness/internal/terminal"
)

// Version is shown in the TUI header; set by cmd before Run.
var Version = "dev"

// Run starts the Superpower TUI. All sends go through internal/harness.
func Run() error {
	term := terminal.Detect()
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
	if term.Ghostty {
		opts = append(opts, tea.WithReportFocus())
	}
	p := tea.NewProgram(initialApp(term), opts...)
	_, err := p.Run()
	return err
}
