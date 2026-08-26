package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	local "github.com/shubhxho/nanoharness/internal/context"
	"github.com/shubhxho/nanoharness/internal/harness"
	"github.com/shubhxho/nanoharness/internal/providers"
)

var version = "dev"

type message struct {
	role, text string
	err        bool
}
type resultMsg struct {
	provider string
	text     string
	err      error
	cites    int
}
type contextMsg struct {
	query, kind string
	report      local.Report
	err         error
}
type gatherMsg struct {
	packet harness.Packet
	err    error
}
type tickMsg time.Time

type app struct {
	input                        textinput.Model
	provider                     string
	models                       map[string]string
	messages                     []message
	status, auth                 string
	write, attach, busy, confirm bool
	super                        bool
	picker                       string
	pick                         int
	spin                         int
	scroll                       int
	evidence                     []local.Citation
	pending                      harness.Packet
	width, height                int
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func initialApp() app {
	input := textinput.New()
	input.Placeholder = "Ask the codebase — Superpower gathers local evidence first"
	input.Prompt = "› "
	input.Focus()
	input.CharLimit = 10000
	input.Width = 80
	models := map[string]string{}
	for _, p := range providers.Profiles {
		models[p.ID] = p.Default
	}
	return app{
		input:    input,
		provider: "codex",
		models:   models,
		super:    true,
		attach:   true,
		status:   "superpower on · ready",
		auth:     providers.AuthStatus("codex"),
		messages: []message{{"nano", "Superpower is on. Every ask gathers local lexical citations, then leaves the machine only after you confirm. F1 help · F5 toggle super.", false}},
	}
}

func (m app) Init() tea.Cmd { return tea.Batch(textinput.Blink, tickCmd()) }

func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(20, msg.Width-8)
		return m, nil
	case tickMsg:
		if m.busy {
			m.spin = (m.spin + 1) % len(spinFrames)
		}
		return m, tickCmd()
	case gatherMsg:
		m.busy = false
		if msg.err != nil {
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
			m.status = "gather failed"
			return m, nil
		}
		m.pending = msg.packet
		if len(msg.packet.Citations) > 0 {
			m.evidence = msg.packet.Citations
		}
		if msg.packet.Confirm {
			m.confirm = true
			m.status = fmt.Sprintf("confirm · %d cites · write %t", msg.packet.CiteCount, m.write)
			m.messages = append(m.messages, message{"nano", fmt.Sprintf(
				"Confirm Superpower send → provider %s · model %s · workspace write %t · local citations leaving machine: %d.\nTerms: %s\n%s\nPress y / Enter to approve · Esc / n to cancel.",
				m.provider, displayModel(m.models[m.provider]), m.write, msg.packet.CiteCount, strings.Join(msg.packet.Terms, " "), summary(msg.packet.Citations),
			), false})
			return m, nil
		}
		return m.dispatch(msg.packet)
	case resultMsg:
		m.busy = false
		m.confirm = false
		m.pending = harness.Packet{}
		if msg.err != nil {
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
			m.status = "request failed"
		} else {
			m.messages = append(m.messages, message{msg.provider, msg.text, false})
			m.status = fmt.Sprintf("ready · last send used %d cites", msg.cites)
		}
		if m.write {
			m.write = false
		}
		return m, nil
	case contextMsg:
		if msg.err != nil {
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
			return m, nil
		}
		m.evidence = local.Top(msg.report.Citations, local.AttachLimit)
		m.messages = append(m.messages, message{"context", fmt.Sprintf("LOCAL LEXICAL %s\nExact token/path evidence only; incomplete and not a dependency graph.\nquery: %s\n%s", strings.ToUpper(msg.kind), msg.query, summary(m.evidence)), false})
		m.status = fmt.Sprintf("%d citations ready", len(m.evidence))
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
		switch key {
		case "f1":
			m.messages = append(m.messages, message{"nano", "F2 provider · F3 model · F4 attach · F5 superpower · Tab next · Ctrl+W write · /query TERMS · /research QUESTION · /impact SYMBOL · /super on|off · /context on|off|clear", false})
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
			m.setProvider((providerIndex(m.provider) + 1) % len(providers.Profiles))
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
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m app) updateConfirm(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "n", "N":
		m.confirm = false
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
	count := len(providers.Profiles)
	if m.picker == "model" {
		p, _ := providers.Find(m.provider)
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
			p, _ := providers.Find(m.provider)
			m.models[m.provider] = p.Models[m.pick]
		}
		m.picker = ""
		return m, nil
	}
	return m, nil
}

func (m *app) setProvider(index int) {
	m.provider = providers.Profiles[index].ID
	m.auth = providers.AuthStatus(m.provider)
	m.status = "provider: " + providers.Profiles[index].Label
}

func (m app) submit() (tea.Model, tea.Cmd) {
	prompt := strings.TrimSpace(m.input.Value())
	if prompt == "" || m.busy {
		return m, nil
	}
	m.input.SetValue("")
	if strings.HasPrefix(prompt, "/") {
		return m.command(prompt)
	}
	m.messages = append(m.messages, message{"you", prompt, false})
	m.scroll = 0
	m.busy = true
	m.status = "superpower gathering local evidence…"
	cfg := m.config()
	return m, func() tea.Msg {
		packet, err := harness.Gather(cfg, prompt)
		return gatherMsg{packet, err}
	}
}

func (m app) config() harness.Config {
	return harness.Config{
		Super:    m.super,
		Provider: m.provider,
		Model:    m.models[m.provider],
		Write:    m.write,
		Evidence: m.evidence,
		Attach:   m.attach && !m.super,
		Limit:    local.AttachLimit,
	}
}

func (m app) dispatch(packet harness.Packet) (tea.Model, tea.Cmd) {
	m.busy = true
	m.status = "working…"
	cfg := m.config()
	cfg.Write = m.write
	cites := packet.CiteCount
	return m, func() tea.Msg {
		text, err := harness.Send(cfg, packet)
		return resultMsg{cfg.Provider, text, err, cites}
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
	case "/super":
		switch value {
		case "", "on", "true", "1":
			m.super = true
			m.attach = true
		case "off", "false", "0":
			m.super = false
		case "status":
			// fall through to status line
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
			// status only
		}
		m.status = fmt.Sprintf("context attach: %t · %d citations · super %t", m.attach, len(m.evidence), m.super)
	case "/provider":
		for i, p := range providers.Profiles {
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
		mode := local.ModeQuery
		if kind == "research" {
			mode = local.ModeResearch
		}
		if kind == "impact" {
			mode = local.ModeImpact
		}
		return m, func() tea.Msg {
			root, err := os.Getwd()
			if err != nil {
				return contextMsg{err: err}
			}
			r, err := local.SearchMode(root, value, mode)
			return contextMsg{value, kind, r, err}
		}
	default:
		m.status = "unknown command"
	}
	return m, nil
}

func (m app) View() string {
	if m.width > 0 && (m.width < 54 || m.height < 12) {
		return "\n  nano needs a terminal at least 54 × 12.\n"
	}
	bg := lipgloss.Color("#1e1e2e")
	surface := lipgloss.Color("#313244")
	lav := lipgloss.Color("#b4befe")
	teal := lipgloss.Color("#94e2d5")
	peach := lipgloss.Color("#fab387")
	muted := lipgloss.Color("#9399b2")
	red := lipgloss.Color("#f38ba8")
	yellow := lipgloss.Color("#f9e2af")
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
	header := lipgloss.NewStyle().Background(surface).Padding(0, 1).Render(
		lipgloss.NewStyle().Foreground(bg).Background(lav).Bold(true).Padding(0, 1).Render("✦ nano") + " " +
			chip(super, superColor) + " " +
			chip(m.provider, lav) + " " +
			chip(model, muted) + " " +
			chip("● "+m.auth, teal) + " " +
			chip(mode, peach),
	)
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
		label := m.provider + " is thinking…"
		if strings.Contains(m.status, "gather") {
			label = "Superpower gathering local evidence…"
		}
		frame := spinFrames[m.spin%len(spinFrames)]
		chat.WriteString(lipgloss.NewStyle().Foreground(lav).Italic(true).Render(frame + " " + label))
	}
	if m.confirm {
		chat.WriteString(lipgloss.NewStyle().Foreground(yellow).Bold(true).Render("\n⚠ awaiting confirmation (y/Enter · n/Esc)"))
	}
	conversation := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(max(40, m.width-34)).Height(max(6, m.height-10)).Render(chat.String())
	inspectorBody := lipgloss.NewStyle().Foreground(lav).Bold(true).Render("INSPECTOR") + "\n\n" +
		"SUPER\n" + map[bool]string{true: "on", false: "off"}[m.super] + "\n\n" +
		"BACKEND\n" + m.provider + "\n\nMODEL\n" + model + "\n\n" +
		"CONTEXT\n" + fmt.Sprintf("attach %t · %d cites", m.attach, len(m.evidence)) + "\n\n"
	if len(m.evidence) > 0 {
		inspectorBody += "EVIDENCE\n" + summary(local.Top(m.evidence, 5)) + "\n\n"
	}
	inspectorBody += "F1 help\nF2 provider\nF3 model\nF4 attach\nF5 super"
	inspector := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(27).Render(inspectorBody)
	body := conversation
	if m.width >= 92 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, conversation, " ", inspector)
	}
	composerTitle := "ASK NANO"
	if m.super {
		composerTitle = "ASK NANO · SUPERPOWER"
	}
	composer := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lav).Padding(0, 1).Render(lipgloss.NewStyle().Foreground(lav).Bold(true).Render(composerTitle) + "\n" + m.input.View())
	footer := lipgloss.NewStyle().Foreground(muted).Render(" " + m.status + "   Enter send · F5 super · PgUp/PgDn scroll · Ctrl+C quit")
	view := lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", composer, footer)
	if m.picker != "" {
		view += "\n\n" + m.pickerView(lav, surface, muted)
	}
	return lipgloss.NewStyle().Background(bg).Render(view)
}

func (m app) pickerView(lav, surface, muted lipgloss.Color) string {
	var rows []string
	if m.picker == "provider" {
		for i, p := range providers.Profiles {
			mark := "  "
			if i == m.pick {
				mark = "› "
			}
			rows = append(rows, mark+p.Label)
		}
	} else {
		p, _ := providers.Find(m.provider)
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
	for i, p := range providers.Profiles {
		if p.ID == id {
			return i
		}
	}
	return 0
}

func modelIndex(id, model string) int {
	p, _ := providers.Find(id)
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
	n := height - 12
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

func summary(c []local.Citation) string {
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
			err = providers.Login(args[1], len(args) > 2 && args[2] == "--api-key")
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

Superpower is on by default for TUI and run. Pass --no-super to send a bare
prompt. Local citations are gathered and attached through one harness path.`)
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
	text, packet, err := harness.Run(cfg, prompt)
	if err != nil {
		return err
	}
	if packet.CiteCount > 0 || packet.Gathered {
		fmt.Fprintf(os.Stderr, "# harness: attached %d local citations (super=%t terms=%q)\n", packet.CiteCount, super, strings.Join(packet.Terms, " "))
	}
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
		r, err := local.Search(root, "index")
		if err == nil {
			fmt.Printf("LOCAL LEXICAL CONTEXT v1\nroot: %s\nscanned: %d bytes · skipped: %d\n", root, r.ScannedBytes, r.Skipped)
		}
		return err
	}
	query := strings.Join(args[1:], " ")
	if query == "" {
		return fmt.Errorf("context %s needs terms", mode)
	}
	root, _ := os.Getwd()
	searchMode := local.ModeQuery
	label := "LOCAL LEXICAL CONTEXT"
	switch mode {
	case "research":
		searchMode = local.ModeResearch
		label = "LOCAL LEXICAL EVIDENCE PACKET"
	case "impact":
		searchMode = local.ModeImpact
		label = "POSSIBLE LEXICAL IMPACT"
	case "query":
		// default
	default:
		return fmt.Errorf("context mode must be index, query, research, or impact")
	}
	r, err := local.SearchMode(root, query, searchMode)
	if err != nil {
		return err
	}
	fmt.Printf("%s v1 — exact token/path matching only; no embeddings or dependency graph.\nquery: %s\nmode: %s\n\n", label, query, searchMode)
	for i, c := range r.Citations {
		fmt.Printf("%02d %s:%d-%d score %d\n%s\n\n", i+1, c.Path, c.StartLine, c.EndLine, c.Score, c.Snippet)
	}
	return nil
}
