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

	session := m.liveSession()
	mode := "READ ONLY"
	modeColor := colorPeach
	if session.Write {
		mode = "WRITE ARMED"
		modeColor = colorRed
	}
	super, superColor := "SUPER OFF", colorMuted
	if session.Super {
		super, superColor = "SUPERPOWER", colorYellow
	}
	phaseColor := colorMuted
	switch m.phase {
	case phaseGather:
		phaseColor = colorLav
	case phaseConfirm:
		phaseColor = colorYellow
	case phaseSend:
		phaseColor = colorGreen
	}
	model := displayModel(m.models[session.Provider])
	extras := ""
	if n := len(session.Evidence); n > 0 {
		extras += " " + chipStyle(colorTeal).Render(fmt.Sprintf("%d CITES", n))
	}
	if session.Continual.Autonomous {
		extras += " " + chipStyle(colorGreen).Render("AUTO")
	}
	if g := strings.TrimSpace(session.Continual.Goal); g != "" {
		extras += " " + chipStyle(colorYellow).Render("GOAL")
	}
	if m.term.Ghostty {
		extras += " " + chipStyle(colorTeal).Render("GHOSTTY")
	}
	header := lipgloss.NewStyle().Background(colorSurface).Padding(0, 1).Render(
		lipgloss.NewStyle().Foreground(colorBG).Background(colorLav).Bold(true).Padding(0, 1).Render("✦ nano "+Version) + " " +
			chipStyle(superColor).Render(super) + " " +
			chipStyle(phaseColor).Render(strings.ToUpper(string(m.phase))) + " " +
			chipStyle(colorLav).Render(session.Provider) + " " +
			chipStyle(colorMuted).Render(model) + " " +
			chipStyle(colorTeal).Render("● "+m.auth) + " " +
			chipStyle(modeColor).Render(mode) + extras,
	)
	pipe := lipgloss.NewStyle().Foreground(colorMuted).Render(
		" harness session · " + session.PipelineLine() + "  " +
			styledPipeline(m.phase, colorLav, colorYellow, colorGreen),
	)

	chatBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder).Padding(0, 1).Width(m.viewport.Width + 2).Render(m.viewport.View())
	body := chatBox
	if m.width >= 92 {
		inspector := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder).Padding(0, 1).Width(27).Render(m.inspectorView())
		body = lipgloss.JoinHorizontal(lipgloss.Top, chatBox, " ", inspector)
	}

	composerBorder := colorLav
	title := "ASK NANO · ENTER SENDS THROUGH HARNESS"
	inner := m.input.View()
	if session.Super {
		title = "ASK NANO · SUPERPOWER SEND"
	}
	if m.confirm {
		composerBorder = colorYellow
		title = "CONFIRM HARNESS SEND"
		inner = renderConfirm(session.Config, m.pending)
	} else if m.busy {
		inner = lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Render(m.spin.View() + " locked while " + string(m.phase) + " runs…")
	}
	composer := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(composerBorder).Padding(0, 1).Width(max(40, m.width-4)).Render(
		lipgloss.NewStyle().Foreground(composerBorder).Bold(true).Render(title) + "\n" + inner,
	)

	footer := lipgloss.NewStyle().Foreground(colorMuted).Render(" "+m.status) + "\n" + m.help.View(keys)
	view := lipgloss.JoinVertical(lipgloss.Left, header, pipe, body, composer, footer)
	if m.picking != "" {
		overlay := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorLav).Background(colorSurface).Padding(1, 2).Render(m.picker.View())
		view += "\n" + overlay
	}
	return lipgloss.NewStyle().Background(colorBG).Render(view)
}

func (m app) inspectorView() string {
	session := m.liveSession()
	model := displayModel(m.models[session.Provider])
	goal := strings.TrimSpace(session.Continual.Goal)
	if goal == "" {
		goal = "(none)"
	}
	auto := "off"
	if session.Continual.Autonomous {
		auto = fmt.Sprintf("on · turns %d", session.Continual.TurnLimit())
	}
	head := lipgloss.NewStyle().Foreground(colorLav).Bold(true).Render("INSPECTOR")
	label := lipgloss.NewStyle().Foreground(colorMuted).Bold(true)
	value := messageBodyStyle()
	body := head + "\n\n" +
		label.Render("PHASE") + "\n" + value.Render(string(m.phase)) + "\n\n" +
		label.Render("SUPER") + "\n" + value.Render(map[bool]string{true: "on", false: "off"}[session.Super]) + "\n\n" +
		label.Render("BACKEND") + "\n" + value.Render(session.Provider) + "\n\n" +
		label.Render("MODEL") + "\n" + value.Render(model) + "\n\n" +
		label.Render("GOAL") + "\n" + value.Render(clipText(goal, 120)) + "\n\n" +
		label.Render("AUTO") + "\n" + value.Render(auto) + "\n\n" +
		label.Render("GATES") + "\n" + value.Render(fmt.Sprintf("%d", len(session.Continual.Gates))) +
		" · " + label.Render("MEMORIES") + " " + value.Render(fmt.Sprintf("%d", len(session.Continual.Memories))) + "\n\n" +
		label.Render("CONTEXT") + "\n" + value.Render(fmt.Sprintf("attach %t · %d cites", session.Attach, len(session.Evidence))) + "\n\n" +
		label.Render("PIPELINE") + "\n" + value.Render(session.PipelineLine()) + "\n\n" +
		label.Render("TERMINAL") + "\n" + value.Render(m.term.Summary())
	if !m.focused && m.term.Ghostty {
		body += "\n" + lipgloss.NewStyle().Foreground(colorMuted).Render("(unfocused)")
	}
	body += "\n\n"
	if len(session.Continual.Gates) > 0 {
		body += label.Render("GATE LIST") + "\n" + value.Render(summaryGates(session.Continual.Gates)) + "\n\n"
	}
	if len(session.Evidence) > 0 {
		body += label.Render("EVIDENCE") + "\n" + value.Render(summary(harness.Top(session.Evidence, 5))) + "\n\n"
	}
	body += label.Render("SESSION") + "\n" + value.Render("via harness")
	return body
}

func summaryGates(gates []string) string {
	if len(gates) == 0 {
		return "(none)"
	}
	out := make([]string, len(gates))
	for i, g := range gates {
		out[i] = fmt.Sprintf("%02d  %s", i+1, clipText(g, 40))
	}
	return strings.Join(out, "\n")
}
