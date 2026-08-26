package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	local "github.com/shubhxho/nanoharness/internal/context"
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
}
type contextMsg struct {
	query, kind string
	report      local.Report
	err         error
}
type app struct {
	input                        textinput.Model
	provider                     string
	models                       map[string]string
	messages                     []message
	status, auth                 string
	write, attach, busy, confirm bool
	picker                       string
	pick                         int
	evidence                     []local.Citation
	width, height                int
}

func initialApp() app {
	input := textinput.New()
	input.Placeholder = "Ask about this codebase, or type /help"
	input.Prompt = "› "
	input.Focus()
	input.CharLimit = 10000
	input.Width = 80
	models := map[string]string{}
	for _, p := range providers.Profiles {
		models[p.ID] = p.Default
	}
	return app{input: input, provider: "codex", models: models, status: "ready", auth: providers.AuthStatus("codex"), messages: []message{{"nano", "Ready. F1 opens help. Local context is off until you enable it.", false}}}
}
func (m app) Init() tea.Cmd { return textinput.Blink }
func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(20, msg.Width-8)
		return m, nil
	case resultMsg:
		m.busy = false
		if msg.err != nil {
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
			m.status = "request failed"
		} else {
			m.messages = append(m.messages, message{msg.provider, msg.text, false})
			m.status = "ready"
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
		m.evidence = msg.report.Citations
		if len(m.evidence) > 8 {
			m.evidence = m.evidence[:8]
		}
		m.messages = append(m.messages, message{"context", fmt.Sprintf("LOCAL LEXICAL %s\nExact token/path evidence only; incomplete and not a dependency graph.\nquery: %s\n%s", strings.ToUpper(msg.kind), msg.query, summary(m.evidence)), false})
		m.status = fmt.Sprintf("%d citations ready", len(m.evidence))
		return m, nil
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.picker != "" {
			return m.updatePicker(key)
		}
		switch key {
		case "f1":
			m.messages = append(m.messages, message{"nano", "F2 provider · F3 model · F4 context · Tab next · Ctrl+W write · /query TERMS · /research QUESTION · /impact SYMBOL · /context on|off|clear", false})
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
		case "tab":
			m.setProvider((providerIndex(m.provider) + 1) % len(providers.Profiles))
			return m, nil
		case "ctrl+w":
			if m.provider == "codex" {
				m.write = !m.write
				m.status = map[bool]string{true: "workspace write armed", false: "read-only"}[m.write]
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
	wire := prompt
	count := 0
	if m.attach && len(m.evidence) > 0 {
		wire += "\n\n" + local.Render(m.evidence)
		count = len(m.evidence)
	}
	m.confirm = m.write || count > 0
	m.status = map[bool]string{true: fmt.Sprintf("confirm %d context cites / write access", count), false: "working…"}[m.confirm]
	if m.confirm {
		m.messages = append(m.messages, message{"nano", fmt.Sprintf("Confirm send: provider %s, model %s, workspace write: %t, local citations leaving machine: %d. Press y or Enter to approve; Esc/n cancels.", m.provider, m.models[m.provider], m.write, count), false})
		m.input.SetValue(wire)
		return m, nil
	}
	return m.start(wire)
}
func (m app) command(prompt string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(prompt)
	if len(fields) == 0 {
		return m, nil
	}
	value := strings.TrimSpace(strings.TrimPrefix(prompt, fields[0]))
	switch fields[0] {
	case "/help":
		m.messages = append(m.messages, message{"nano", "/query TERMS · /research QUESTION · /impact SYMBOL · /context on|off|clear|status · /provider NAME · /model NAME · /new · /exit", false})
	case "/exit":
		return m, tea.Quit
	case "/new":
		m.messages = nil
		m.evidence = nil
	case "/context":
		if value == "on" {
			m.attach = true
		} else if value == "off" {
			m.attach = false
		} else if value == "clear" {
			m.evidence = nil
		}
		m.status = fmt.Sprintf("context: %t · %d citations", m.attach, len(m.evidence))
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
		return m, func() tea.Msg {
			root, err := os.Getwd()
			if err != nil {
				return contextMsg{err: err}
			}
			r, err := local.Search(root, value)
			return contextMsg{value, kind, r, err}
		}
	default:
		m.status = "unknown command"
	}
	return m, nil
}
func (m app) start(wire string) (tea.Model, tea.Cmd) {
	m.busy = true
	m.confirm = false
	provider, model, write := m.provider, m.models[m.provider], m.write
	return m, func() tea.Msg {
		text, err := providers.Ask(provider, wire, model, write)
		return resultMsg{provider, text, err}
	}
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
	chip := func(s string, c lipgloss.Color) string {
		return lipgloss.NewStyle().Foreground(c).Background(surface).Bold(true).Padding(0, 1).Render(s)
	}
	mode := "READ ONLY"
	if m.write {
		mode = "WRITE ARMED"
	}
	model := m.models[m.provider]
	if model == "" {
		model = "vendor default"
	}
	header := lipgloss.NewStyle().Background(surface).Padding(0, 1).Render(lipgloss.NewStyle().Foreground(bg).Background(lav).Bold(true).Padding(0, 1).Render("✦ nano") + " " + chip(m.provider, lav) + " " + chip(model, muted) + " " + chip("● "+m.auth, teal) + " " + chip(mode, peach))
	var chat strings.Builder
	for _, msg := range m.messages {
		c := peach
		if msg.role == "you" {
			c = teal
		}
		if msg.err {
			c = red
		}
		chat.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render("▎ "+strings.ToUpper(msg.role)) + "\n")
		chat.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")).Render(msg.text) + "\n\n")
	}
	if m.busy {
		chat.WriteString(lipgloss.NewStyle().Foreground(lav).Italic(true).Render("● " + m.provider + " is thinking…"))
	}
	conversation := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(max(40, m.width-34)).Render(chat.String())
	inspector := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(27).Render(lipgloss.NewStyle().Foreground(lav).Bold(true).Render("INSPECTOR") + "\n\n" + "BACKEND\n" + m.provider + "\n\nMODEL\n" + model + "\n\nCONTEXT\n" + fmt.Sprintf("%t · %d cites", m.attach, len(m.evidence)) + "\n\nF1 help\nF2 provider\nF3 model\nF4 context")
	body := conversation
	if m.width >= 92 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, conversation, " ", inspector)
	}
	composer := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lav).Padding(0, 1).Render(lipgloss.NewStyle().Foreground(lav).Bold(true).Render("ASK NANO") + "\n" + m.input.View())
	footer := lipgloss.NewStyle().Foreground(muted).Render(" " + m.status + "   Enter send · F1 help · Ctrl+C quit")
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
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
	fmt.Println("nanoharness\n\nUSAGE:\n  nanoharness\n  nanoharness login <codex|openai|anthropic|claude> [--api-key]\n  nanoharness run [--provider ID] [--model ID] [--write] PROMPT\n  nanoharness context <index|query|research|impact> TERMS")
}
func runCLI(args []string) error {
	provider, model := "codex", ""
	write := false
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
		default:
			words = append(words, args[i])
		}
	}
	text, err := providers.Ask(provider, strings.Join(words, " "), model, write)
	if err == nil {
		fmt.Println(text)
	}
	return err
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
	r, err := local.Search(root, query)
	if err != nil {
		return err
	}
	label := "LOCAL LEXICAL CONTEXT"
	if mode == "research" {
		label = "LOCAL LEXICAL EVIDENCE PACKET"
	}
	if mode == "impact" {
		label = "POSSIBLE LEXICAL IMPACT"
	}
	fmt.Printf("%s v1 — exact token/path matching only; no embeddings or dependency graph.\nquery: %s\n\n", label, query)
	for i, c := range r.Citations {
		fmt.Printf("%02d %s:%d-%d score %d\n%s\n\n", i+1, c.Path, c.StartLine, c.EndLine, c.Score, c.Snippet)
	}
	return nil
}
