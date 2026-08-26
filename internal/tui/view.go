package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/shubhxho/nanoharness/internal/harness"
)

func (m app) View() string {
	if !m.ready {
		return "\n  starting nanoharness…\n"
	}
	if m.width < 54 || m.height < 14 {
		return "\n  nano needs a terminal at least 54 × 14.\n"
	}

	bg := lipgloss.Color("#1e1e2e")
	surface := lipgloss.Color("#313244")
	lav := lipgloss.Color("#b4befe")
	teal := lipgloss.Color("#94e2d5")
	peach := lipgloss.Color("#fab387")
	muted := lipgloss.Color("#9399b2")
	yellow := lipgloss.Color("#f9e2af")
	green := lipgloss.Color("#a6e3a1")
	chip := func(s string, c lipgloss.Color) string {
		return lipgloss.NewStyle().Foreground(c).Background(surface).Bold(true).Padding(0, 1).Render(s)
	}

	session := m.liveSession()
	mode := "READ ONLY"
	if session.Write {
		mode = "WRITE ARMED"
	}
	super, superColor := "SUPER OFF", muted
	if session.Super {
		super, superColor = "SUPERPOWER", yellow
	}
	phaseColor := muted
	switch m.phase {
	case phaseGather:
		phaseColor = lav
	case phaseConfirm:
		phaseColor = yellow
	case phaseSend:
		phaseColor = green
	}
	model := displayModel(m.models[session.Provider])
	header := lipgloss.NewStyle().Background(surface).Padding(0, 1).Render(
		lipgloss.NewStyle().Foreground(bg).Background(lav).Bold(true).Padding(0, 1).Render("✦ nano "+Version) + " " +
			chip(super, superColor) + " " + chip(strings.ToUpper(string(m.phase)), phaseColor) + " " +
			chip(session.Provider, lav) + " " + chip(model, muted) + " " +
			chip("● "+m.auth, teal) + " " + chip(mode, peach),
	)
	pipe := lipgloss.NewStyle().Foreground(muted).Render(" harness session  " + pipeline(m.phase))

	chatBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(m.viewport.Width + 2).Render(m.viewport.View())
	inspector := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(27).Render(m.inspectorView(lav))
	body := chatBox
	if m.width >= 92 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, chatBox, " ", inspector)
	}

	composerBorder := lav
	title := "ASK NANO · ENTER SENDS THROUGH HARNESS"
	inner := m.input.View()
	if session.Super {
		title = "ASK NANO · SUPERPOWER SEND"
	}
	if m.confirm {
		composerBorder = yellow
		title = "CONFIRM HARNESS SEND"
		inner = harness.ConfirmSummary(session.Config, m.pending)
	} else if m.busy {
		inner = lipgloss.NewStyle().Foreground(muted).Italic(true).Render(m.spin.View() + " locked while " + string(m.phase) + " runs…")
	}
	composer := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(composerBorder).Padding(0, 1).Width(max(40, m.width-4)).Render(
		lipgloss.NewStyle().Foreground(composerBorder).Bold(true).Render(title) + "\n" + inner,
	)

	footer := lipgloss.NewStyle().Foreground(muted).Render(" "+m.status) + "\n" + m.help.View(keys)
	view := lipgloss.JoinVertical(lipgloss.Left, header, pipe, body, composer, footer)
	if m.picking != "" {
		overlay := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lav).Background(surface).Padding(1, 2).Render(m.picker.View())
		view += "\n" + overlay
	}
	return lipgloss.NewStyle().Background(bg).Render(view)
}

func (m app) inspectorView(lav lipgloss.Color) string {
	session := m.liveSession()
	model := displayModel(m.models[session.Provider])
	body := lipgloss.NewStyle().Foreground(lav).Bold(true).Render("INSPECTOR") + "\n\n" +
		"PHASE\n" + string(m.phase) + "\n\n" +
		"SUPER\n" + map[bool]string{true: "on", false: "off"}[session.Super] + "\n\n" +
		"BACKEND\n" + session.Provider + "\n\nMODEL\n" + model + "\n\n" +
		"CONTEXT\n" + fmt.Sprintf("attach %t · %d cites", session.Attach, len(session.Evidence)) + "\n\n"
	if len(session.Evidence) > 0 {
		body += "EVIDENCE\n" + summary(harness.Top(session.Evidence, 5)) + "\n\n"
	}
	body += "session\nvia harness"
	return body
}
