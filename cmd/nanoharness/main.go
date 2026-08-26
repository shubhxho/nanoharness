package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shubhxho/nanoharness/internal/harness"
)

var version = "dev"

type phase string

const (
	phaseIdle    phase = ""
	phaseGather  phase = "gather"
	phaseConfirm phase = "confirm"
	phaseSend    phase = "send"
)

type message struct {
	role, text string
	err        bool
}
type resultMsg struct {
	provider string
	text     string
	err      error
	cites    int
	elapsed  time.Duration
}
type contextMsg struct {
	query, kind string
	report      harness.Report
	err         error
}
type gatherMsg struct {
	packet  harness.Packet
	err     error
	elapsed time.Duration
}
type tickMsg time.Time

type app struct {
	input                        textarea.Model
	provider                     string
	models                       map[string]string
	messages                     []message
	status, auth                 string
	write, attach, busy, confirm bool
	super                        bool
	phase                        phase
	picker                       string
	pick                         int
	spin                         int
	scroll                       int
	started                      time.Time
	evidence                     []harness.Citation
	pending                      harness.Packet
	width, height                int
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func initialApp() app {
	input := textarea.New()
	input.Placeholder = "Ask the codebase — Enter sends through the harness · Ctrl+J newline"
	input.Focus()
	input.CharLimit = 12000
	input.SetWidth(80)
	input.SetHeight(3)
	input.ShowLineNumbers = false
	input.Prompt = "› "
	models := map[string]string{}
	for _, p := range harness.Profiles {
		models[p.ID] = p.Default
	}
	return app{
		input:    input,
		provider: "codex",
		models:   models,
		super:    true,
		attach:   true,
		status:   "superpower on · ready",
		auth:     harness.AuthStatus("codex"),
		messages: []message{{"nano", "Superpower send is on. Enter runs gather → confirm → send through the harness. Ctrl+J for newline · F5 toggles super · F1 help.", false}},
	}
}

func (m app) Init() tea.Cmd { return tea.Batch(textarea.Blink, tickCmd()) }

func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(max(20, msg.Width-8))
		return m, nil
	case tickMsg:
		if m.busy {
			m.spin = (m.spin + 1) % len(spinFrames)
			if !m.started.IsZero() {
				m.status = m.phaseStatus(time.Since(m.started))
			}
		}
		return m, tickCmd()
	case gatherMsg:
		if msg.err != nil {
			m.busy = false
			m.phase = phaseIdle
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
			m.status = "gather failed"
			return m, nil
		}
		m.pending = msg.packet
		if len(msg.packet.Citations) > 0 {
			m.evidence = msg.packet.Citations
		}
		if msg.packet.Confirm {
			m.busy = false
			m.confirm = true
			m.phase = phaseConfirm
			m.status = fmt.Sprintf("confirm · %s · gathered in %s", harness.Describe(msg.packet), msg.elapsed.Round(time.Millisecond))
			return m, nil
		}
		m.status = fmt.Sprintf("gathered in %s · %s", msg.elapsed.Round(time.Millisecond), harness.Describe(msg.packet))
		return m.dispatch(msg.packet)
	case resultMsg:
		m.busy = false
		m.confirm = false
		m.phase = phaseIdle
		m.pending = harness.Packet{}
		m.started = time.Time{}
		if msg.err != nil {
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
			m.status = "request failed"
		} else {
			m.messages = append(m.messages, message{msg.provider, msg.text, false})
			m.status = fmt.Sprintf("ready · %d cites · %s", msg.cites, msg.elapsed.Round(time.Millisecond))
		}
		if m.write {
			m.write = false
		}
		m.scroll = 0
		return m, nil
	case contextMsg:
		if msg.err != nil {
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
			return m, nil
		}
		m.evidence = harness.Top(msg.report.Citations, harness.AttachLimit)
		m.messages = append(m.messages, message{"context", fmt.Sprintf("LOCAL LEXICAL %s\nExact token/path evidence only; incomplete and not a dependency graph.\nquery: %s\n%s", strings.ToUpper(msg.kind), msg.query, summary(m.evidence)), false})
		m.status = fmt.Sprintf("%d citations ready", len(m.evidence))
		m.scroll = 0
		return m, nil
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.confirm {
			return m.updateConfirm(key)
		}
		if m.picker != "" {
			return m.updatePicker(key)
		}
		if m.busy {
			return m, nil
		}
		switch key {
		case "f1":
			m.messages = append(m.messages, message{"nano", "Enter send · Ctrl+J newline · F2 provider · F3 model · F4 attach · F5 super · Tab next · Ctrl+W write · /query · /research · /impact · /super on|off", false})
			return m, nil
		case "f2", "ctrl+p":
			m.picker = "provider"
			m.pick = providerIndex(m.provider)
			return m, nil
		case "f3":
			m.picker = "model"
			m.pick = modelIndex(m.provider, m.models[m.provider])
			return m, nil
		case "f4":
			m.attach = !m.attach
			m.status = map[bool]string{true: "context attachment on", false: "context attachment off"}[m.attach]
			return m, nil
		case "f5":
			m.super = !m.super
			if m.super {
				m.attach = true
				m.status = "superpower on"
			} else {
				m.status = "superpower off"
			}
			return m, nil
		case "tab":
			m.setProvider((providerIndex(m.provider) + 1) % len(harness.Profiles))
			return m, nil
		case "ctrl+w":
			if m.provider == "codex" {
				m.write = !m.write
				m.status = map[bool]string{true: "workspace write armed", false: "read-only"}[m.write]
			}
			return m, nil
		case "pgup", "ctrl+u":
			m.scroll += 3
			return m, nil
		case "pgdown", "ctrl+d":
			if m.scroll > 0 {
				m.scroll -= 3
				if m.scroll < 0 {
					m.scroll = 0
				}
			}
			return m, nil
		case "enter":
			return m.submit()
		case "ctrl+j":
			m.input.InsertString("\n")
			return m, nil
		}
	}
	if m.busy || m.confirm || m.picker != "" {
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m app) phaseStatus(elapsed time.Duration) string {
	frame := spinFrames[m.spin%len(spinFrames)]
	switch m.phase {
	case phaseGather:
		return fmt.Sprintf("%s gather · %s", frame, elapsed.Round(time.Millisecond))
	case phaseSend:
		return fmt.Sprintf("%s send via %s · %s", frame, m.provider, elapsed.Round(time.Millisecond))
	default:
		return m.status
	}
}

func (m app) updateConfirm(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "n", "N":
		m.confirm = false
		m.phase = phaseIdle
		m.pending = harness.Packet{}
		m.status = "send cancelled"
		m.messages = append(m.messages, message{"nano", "Cancelled. Nothing left the machine.", false})
		return m, nil
	case "enter", "y", "Y":
		packet := m.pending
		m.confirm = false
		return m.dispatch(packet)
	}
	return m, nil
}

func (m app) updatePicker(key string) (tea.Model, tea.Cmd) {
	count := len(harness.Profiles)
	if m.picker == "model" {
		p, _ := harness.Find(m.provider)
		count = len(p.Models)
	}
	switch key {
	case "esc":
		m.picker = ""
		return m, nil
	case "up", "k":
		if m.pick > 0 {
			m.pick--
		}
		return m, nil
	case "down", "j":
		if m.pick < count-1 {
			m.pick++
		}
		return m, nil
	case "enter":
		if m.picker == "provider" {
			m.setProvider(m.pick)
		} else {
			p, _ := harness.Find(m.provider)
			m.models[m.provider] = p.Models[m.pick]
		}
		m.picker = ""
		return m, nil
	}
	return m, nil
}

func (m *app) setProvider(index int) {
	m.provider = harness.Profiles[index].ID
	m.auth = harness.AuthStatus(m.provider)
	m.status = "provider: " + harness.Profiles[index].Label
}

func (m app) submit() (tea.Model, tea.Cmd) {
	prompt := strings.TrimSpace(m.input.Value())
	if prompt == "" || m.busy {
		return m, nil
	}
	m.input.Reset()
	if strings.HasPrefix(prompt, "/") {
		return m.command(prompt)
	}
	m.messages = append(m.messages, message{"you", prompt, false})
	m.scroll = 0
	m.busy = true
	m.phase = phaseGather
	m.started = time.Now()
	m.status = m.phaseStatus(0)
	cfg := m.config()
	return m, func() tea.Msg {
		start := time.Now()
		packet, err := harness.Gather(cfg, prompt)
		return gatherMsg{packet, err, time.Since(start)}
	}
}

func (m app) config() harness.Config {
	return harness.Config{
		Super:    m.super,
		Provider: m.provider,
		Model:    m.models[m.provider],
		Write:    m.write,
		Evidence: m.evidence,
		Attach:   m.attach,
		Limit:    harness.AttachLimit,
	}
}

func (m app) dispatch(packet harness.Packet) (tea.Model, tea.Cmd) {
	m.busy = true
	m.phase = phaseSend
	m.started = time.Now()
	m.status = m.phaseStatus(0)
	cfg := m.config()
	cfg.Write = m.write
	cites := packet.CiteCount
	provider := cfg.Provider
	return m, func() tea.Msg {
		start := time.Now()
		text, err := harness.Send(cfg, packet)
		return resultMsg{provider, text, err, cites, time.Since(start)}
	}
}

func (m app) command(prompt string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(prompt)
	if len(fields) == 0 {
		return m, nil
	}
	value := strings.TrimSpace(strings.TrimPrefix(prompt, fields[0]))
	switch fields[0] {
	case "/help":
		m.messages = append(m.messages, message{"nano", "/super on|off · /query TERMS · /research QUESTION · /impact SYMBOL · /context on|off|clear|status · /provider NAME · /model NAME · /new · /exit", false})
	case "/exit":
		return m, tea.Quit
	case "/new":
		m.messages = nil
		m.evidence = nil
		m.pending = harness.Packet{}
		m.confirm = false
		m.phase = phaseIdle
	case "/super":
		switch value {
		case "", "on", "true", "1":
			m.super = true
			m.attach = true
		case "off", "false", "0":
			m.super = false
		case "status":
		default:
			m.status = "usage: /super on|off|status"
			return m, nil
		}
		m.status = fmt.Sprintf("superpower: %t · attach: %t · %d citations", m.super, m.attach, len(m.evidence))
	case "/context":
		switch value {
		case "on":
			m.attach = true
		case "off":
			m.attach = false
		case "clear":
			m.evidence = nil
		case "status", "":
		}
		m.status = fmt.Sprintf("context attach: %t · %d citations · super %t", m.attach, len(m.evidence), m.super)
	case "/provider":
		for i, p := range harness.Profiles {
			if p.ID == value {
				m.setProvider(i)
			}
		}
	case "/model":
		if value != "" {
			m.models[m.provider] = value
		}
	case "/query", "/research", "/impact":
		if value == "" {
			m.status = "context command needs terms"
			return m, nil
		}
		kind := strings.TrimPrefix(fields[0], "/")
		mode := harness.ModeQuery
		if kind == "research" {
			mode = harness.ModeResearch
		}
		if kind == "impact" {
			mode = harness.ModeImpact
		}
		return m, func() tea.Msg {
			r, err := harness.Search("", value, mode)
			return contextMsg{value, kind, r, err}
		}
	default:
		m.status = "unknown command"
	}
	return m, nil
}

func (m app) View() string {
	if m.width > 0 && (m.width < 54 || m.height < 14) {
		return "\n  nano needs a terminal at least 54 × 14.\n"
	}
	bg := lipgloss.Color("#1e1e2e")
	surface := lipgloss.Color("#313244")
	lav := lipgloss.Color("#b4befe")
	teal := lipgloss.Color("#94e2d5")
	peach := lipgloss.Color("#fab387")
	muted := lipgloss.Color("#9399b2")
	red := lipgloss.Color("#f38ba8")
	yellow := lipgloss.Color("#f9e2af")
	green := lipgloss.Color("#a6e3a1")
	chip := func(s string, c lipgloss.Color) string {
		return lipgloss.NewStyle().Foreground(c).Background(surface).Bold(true).Padding(0, 1).Render(s)
	}
	mode := "READ ONLY"
	if m.write {
		mode = "WRITE ARMED"
	}
	super := "SUPER OFF"
	superColor := muted
	if m.super {
		super = "SUPERPOWER"
		superColor = yellow
	}
	model := displayModel(m.models[m.provider])
	phaseChip := chip("IDLE", muted)
	switch m.phase {
	case phaseGather:
		phaseChip = chip("GATHER", lav)
	case phaseConfirm:
		phaseChip = chip("CONFIRM", yellow)
	case phaseSend:
		phaseChip = chip("SEND", green)
	}
	header := lipgloss.NewStyle().Background(surface).Padding(0, 1).Render(
		lipgloss.NewStyle().Foreground(bg).Background(lav).Bold(true).Padding(0, 1).Render("✦ nano") + " " +
			chip(super, superColor) + " " + phaseChip + " " +
			chip(m.provider, lav) + " " +
			chip(model, muted) + " " +
			chip("● "+m.auth, teal) + " " +
			chip(mode, peach),
	)

	pipe := lipgloss.NewStyle().Foreground(muted).Render(" harness  gather → confirm → send")
	if m.phase != phaseIdle {
		pipe = lipgloss.NewStyle().Foreground(lav).Render(" harness  " + pipeline(m.phase))
	}

	visible := windowMessages(m.messages, m.scroll, chatBudget(m.height))
	var chat strings.Builder
	for _, msg := range visible {
		c := peach
		if msg.role == "you" {
			c = teal
		}
		if msg.err {
			c = red
		}
		if msg.role == "context" {
			c = yellow
		}
		chat.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render("▎ "+strings.ToUpper(msg.role)) + "\n")
		chat.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")).Render(clipText(msg.text, 2400)) + "\n\n")
	}
	if m.busy {
		label := "sending through harness…"
		if m.phase == phaseGather {
			label = "gathering local evidence…"
		}
		frame := spinFrames[m.spin%len(spinFrames)]
		elapsed := ""
		if !m.started.IsZero() {
			elapsed = " · " + time.Since(m.started).Round(time.Millisecond).String()
		}
		chat.WriteString(lipgloss.NewStyle().Foreground(lav).Italic(true).Render(frame + " " + label + elapsed))
	}

	convWidth := max(40, m.width-34)
	convHeight := max(6, m.height-14)
	conversation := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(convWidth).Height(convHeight).Render(chat.String())

	inspectorBody := lipgloss.NewStyle().Foreground(lav).Bold(true).Render("INSPECTOR") + "\n\n" +
		"PHASE\n" + string(m.phaseOrIdle()) + "\n\n" +
		"SUPER\n" + map[bool]string{true: "on", false: "off"}[m.super] + "\n\n" +
		"BACKEND\n" + m.provider + "\n\nMODEL\n" + model + "\n\n" +
		"CONTEXT\n" + fmt.Sprintf("attach %t · %d cites", m.attach, len(m.evidence)) + "\n\n"
	if len(m.evidence) > 0 {
		inspectorBody += "EVIDENCE\n" + summary(harness.Top(m.evidence, 5)) + "\n\n"
	}
	inspectorBody += "Enter send\nCtrl+J newline\nF5 super"
	inspector := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(27).Render(inspectorBody)
	body := conversation
	if m.width >= 92 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, conversation, " ", inspector)
	}

	composerBorder := lav
	composerTitle := "ASK NANO · ENTER SENDS THROUGH HARNESS"
	if m.super {
		composerTitle = "ASK NANO · SUPERPOWER SEND"
	}
	if m.confirm {
		composerBorder = yellow
		composerTitle = "CONFIRM HARNESS SEND"
	}
	composerInner := m.input.View()
	if m.confirm {
		composerInner = harness.ConfirmSummary(m.config(), m.pending)
	} else if m.busy {
		composerInner = lipgloss.NewStyle().Foreground(muted).Italic(true).Render("locked while " + string(m.phaseOrIdle()) + " runs…")
	}
	composer := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(composerBorder).Padding(0, 1).Width(max(40, m.width-4)).Render(
		lipgloss.NewStyle().Foreground(composerBorder).Bold(true).Render(composerTitle) + "\n" + composerInner,
	)
	footer := lipgloss.NewStyle().Foreground(muted).Render(" " + m.status + "   Enter send · Ctrl+J newline · F5 super · Ctrl+C quit")
	view := lipgloss.JoinVertical(lipgloss.Left, header, pipe, body, composer, footer)
	if m.picker != "" {
		view += "\n\n" + m.pickerView(lav, surface, muted)
	}
	return lipgloss.NewStyle().Background(bg).Render(view)
}

func (m app) phaseOrIdle() phase {
	if m.phase == phaseIdle {
		return "idle"
	}
	return m.phase
}

func pipeline(p phase) string {
	steps := []string{"gather", "confirm", "send"}
	var out []string
	for _, step := range steps {
		if string(p) == step {
			out = append(out, "["+step+"]")
		} else {
			out = append(out, step)
		}
	}
	return strings.Join(out, " → ")
}

func (m app) pickerView(lav, surface, muted lipgloss.Color) string {
	var rows []string
	if m.picker == "provider" {
		for i, p := range harness.Profiles {
			mark := "  "
			if i == m.pick {
				mark = "› "
			}
			rows = append(rows, mark+p.Label)
		}
	} else {
		p, _ := harness.Find(m.provider)
		for i, name := range p.Models {
			if name == "" {
				name = "vendor default"
			}
			mark := "  "
			if i == m.pick {
				mark = "› "
			}
			rows = append(rows, mark+name)
		}
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lav).Background(surface).Padding(1, 2).Render(strings.ToUpper(m.picker) + "\n\n" + strings.Join(rows, "\n") + "\n\n↑↓ navigate · Enter select · Esc cancel")
}

func providerIndex(id string) int {
	for i, p := range harness.Profiles {
		if p.ID == id {
			return i
		}
	}
	return 0
}

func modelIndex(id, model string) int {
	p, _ := harness.Find(id)
	for i, x := range p.Models {
		if x == model {
			return i
		}
	}
	return 0
}

func displayModel(model string) string {
	if model == "" {
		return "vendor default"
	}
	return model
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func chatBudget(height int) int {
	n := height - 16
	if n < 4 {
		return 4
	}
	if n > 40 {
		return 40
	}
	return n
}

func windowMessages(all []message, scroll, budget int) []message {
	if len(all) == 0 {
		return nil
	}
	end := len(all) - scroll
	if end < 1 {
		end = 1
	}
	if end > len(all) {
		end = len(all)
	}
	start := end - budget
	if start < 0 {
		start = 0
	}
	return all[start:end]
}

func clipText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n… truncated …"
}

func summary(c []harness.Citation) string {
	if len(c) == 0 {
		return "No local matches."
	}
	out := make([]string, len(c))
	for i, x := range c {
		out[i] = fmt.Sprintf("%02d  %s:%d-%d", i+1, x.Path, x.StartLine, x.EndLine)
	}
	return strings.Join(out, "\n")
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "tui" {
		p := tea.NewProgram(initialApp(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	var err error
	switch args[0] {
	case "login":
		if len(args) < 2 {
			err = fmt.Errorf("login needs a provider")
		} else {
			err = harness.Login(args[1], len(args) > 2 && args[2] == "--api-key")
		}
	case "run":
		err = runCLI(args[1:])
	case "context":
		err = contextCLI(args[1:])
	case "version", "--version", "-V":
		fmt.Println("nanoharness", version)
	case "help", "--help", "-h":
		usage()
	default:
		err = fmt.Errorf("unknown command: %s", args[0])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`nanoharness — Superpower terminal harness

USAGE:
  nanoharness
  nanoharness login <codex|openai|anthropic|claude> [--api-key]
  nanoharness run [--provider ID] [--model ID] [--write] [--super|--no-super] PROMPT
  nanoharness context <index|query|research|impact> TERMS

TUI Enter and run both go through harness gather → confirm/send.
Pass --no-super to skip local citation gather.`)
}

func runCLI(args []string) error {
	provider, model := "codex", ""
	write, super := false, true
	var words []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider":
			i++
			if i < len(args) {
				provider = args[i]
			}
		case "--model":
			i++
			if i < len(args) {
				model = args[i]
			}
		case "--write":
			write = true
		case "--super":
			super = true
		case "--no-super":
			super = false
		default:
			words = append(words, args[i])
		}
	}
	prompt := strings.Join(words, " ")
	cfg := harness.Config{Super: super, Provider: provider, Model: model, Write: write, Attach: super}

	fmt.Fprintln(os.Stderr, "# harness: gather…")
	start := time.Now()
	packet, err := harness.Gather(cfg, prompt)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "# harness: %s · %s\n", harness.Describe(packet), time.Since(start).Round(time.Millisecond))
	fmt.Fprintln(os.Stderr, "# harness: send…")
	start = time.Now()
	text, err := harness.Send(cfg, packet)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "# harness: sent via %s in %s\n", provider, time.Since(start).Round(time.Millisecond))
	fmt.Println(text)
	return nil
}

func contextCLI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("context needs a mode")
	}
	mode := args[0]
	if mode == "index" {
		root, _ := os.Getwd()
		r, err := harness.Index("")
		if err == nil {
			fmt.Printf("LOCAL LEXICAL CONTEXT v1\nroot: %s\nscanned: %d bytes · skipped: %d\n", root, r.ScannedBytes, r.Skipped)
		}
		return err
	}
	query := strings.Join(args[1:], " ")
	if query == "" {
		return fmt.Errorf("context %s needs terms", mode)
	}
	searchMode, err := harness.ParseMode(mode)
	if err != nil {
		return fmt.Errorf("context mode must be index, query, research, or impact")
	}
	r, err := harness.Search("", query, searchMode)
	if err != nil {
		return err
	}
	fmt.Print(harness.FormatReport(harness.ModeLabel(searchMode), query, searchMode, r))
	return nil
}
