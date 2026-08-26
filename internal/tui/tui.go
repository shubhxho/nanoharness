package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/shubhxho/nanoharness/internal/harness"
)

// Version is shown in the TUI header; set by cmd before Run.
var Version = "dev"

type phase string

const (
	phaseIdle    phase = "idle"
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

type pickItem struct{ title, desc, id string }

func (i pickItem) Title() string       { return i.title }
func (i pickItem) Description() string { return i.desc }
func (i pickItem) FilterValue() string { return i.title + " " + i.desc }

type keyMap struct {
	Send, Newline, Provider, Model, Attach, Super, Write, Help, Quit key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Send, k.Newline, k.Super, k.Help, k.Quit}
}
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Send, k.Newline, k.Provider, k.Model},
		{k.Attach, k.Super, k.Write, k.Help, k.Quit},
	}
}

var keys = keyMap{
	Send:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send via harness")),
	Newline:  key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "newline")),
	Provider: key.NewBinding(key.WithKeys("f2", "ctrl+p"), key.WithHelp("f2", "provider")),
	Model:    key.NewBinding(key.WithKeys("f3"), key.WithHelp("f3", "model")),
	Attach:   key.NewBinding(key.WithKeys("f4"), key.WithHelp("f4", "attach")),
	Super:    key.NewBinding(key.WithKeys("f5"), key.WithHelp("f5", "superpower")),
	Write:    key.NewBinding(key.WithKeys("ctrl+w"), key.WithHelp("ctrl+w", "arm write")),
	Help:     key.NewBinding(key.WithKeys("f1", "?"), key.WithHelp("f1", "help")),
	Quit:     key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
}

type app struct {
	input    textarea.Model
	viewport viewport.Model
	spin     spinner.Model
	help     help.Model
	picker   list.Model

	provider string
	models   map[string]string
	messages []message
	status   string
	auth     string
	write    bool
	attach   bool
	busy     bool
	confirm  bool
	super    bool
	showHelp bool
	picking  string // "", "provider", "model"
	phase    phase
	started  time.Time
	evidence []harness.Citation
	pending  harness.Packet
	width    int
	height   int
	ready    bool
}

func initialApp() app {
	ta := textarea.New()
	ta.Placeholder = "Ask the codebase — Enter sends through harness · Ctrl+J newline"
	ta.Focus()
	ta.CharLimit = 12000
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Prompt = "› "
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#b4befe"))

	h := help.New()
	h.ShowAll = false

	models := map[string]string{}
	for _, p := range harness.Profiles {
		models[p.ID] = p.Default
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("#b4befe")).BorderForeground(lipgloss.Color("#b4befe"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("#94e2d5"))
	picker := list.New([]list.Item{}, delegate, 40, 12)
	picker.SetShowStatusBar(false)
	picker.SetFilteringEnabled(true)
	picker.SetShowHelp(false)
	picker.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("#b4befe")).Bold(true)

	return app{
		input:    ta,
		spin:     sp,
		help:     h,
		picker:   picker,
		provider: "codex",
		models:   models,
		super:    true,
		attach:   true,
		phase:    phaseIdle,
		status:   "superpower on · ready",
		auth:     harness.AuthStatus("codex"),
		messages: []message{{"nano", fmt.Sprintf("nanoharness %s — Superpower send runs gather → confirm → send through the harness. F1 help · F5 super.", Version), false}},
	}
}

func (m app) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spin.Tick)
}

func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.ready = true
		m.refreshViewport()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		if m.busy && !m.started.IsZero() {
			m.status = m.phaseStatus(time.Since(m.started))
		}
		return m, cmd

	case gatherMsg:
		if msg.err != nil {
			m.busy = false
			m.phase = phaseIdle
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
			m.status = "gather failed"
			m.refreshViewport()
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
			m.status = fmt.Sprintf("confirm · %s · %s", harness.Describe(msg.packet), msg.elapsed.Round(time.Millisecond))
			m.refreshViewport()
			return m, nil
		}
		m.status = fmt.Sprintf("gathered · %s · %s", harness.Describe(msg.packet), msg.elapsed.Round(time.Millisecond))
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
		m.refreshViewport()
		return m, nil

	case contextMsg:
		if msg.err != nil {
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
		} else {
			m.evidence = harness.Top(msg.report.Citations, harness.AttachLimit)
			m.messages = append(m.messages, message{"context", fmt.Sprintf("LOCAL LEXICAL %s\nExact token/path evidence only.\nquery: %s\n%s", strings.ToUpper(msg.kind), msg.query, summary(m.evidence)), false})
			m.status = fmt.Sprintf("%d citations ready", len(m.evidence))
		}
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		if m.picking != "" {
			return m.updatePicker(msg)
		}
		if m.confirm {
			return m.updateConfirm(msg)
		}
		if m.busy {
			return m, m.spin.Tick
		}
		switch {
		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			m.help.ShowAll = m.showHelp
			return m, nil
		case key.Matches(msg, keys.Provider):
			return m.openPicker("provider")
		case key.Matches(msg, keys.Model):
			return m.openPicker("model")
		case key.Matches(msg, keys.Attach):
			m.attach = !m.attach
			m.status = map[bool]string{true: "context attachment on", false: "context attachment off"}[m.attach]
			return m, nil
		case key.Matches(msg, keys.Super):
			m.super = !m.super
			if m.super {
				m.attach = true
				m.status = "superpower on"
			} else {
				m.status = "superpower off"
			}
			return m, nil
		case key.Matches(msg, keys.Write):
			if m.provider == "codex" {
				m.write = !m.write
				m.status = map[bool]string{true: "workspace write armed", false: "read-only"}[m.write]
			}
			return m, nil
		case msg.String() == "tab":
			m.setProvider((providerIndex(m.provider) + 1) % len(harness.Profiles))
			return m, nil
		case key.Matches(msg, keys.Send):
			return m.submit()
		case key.Matches(msg, keys.Newline):
			m.input.InsertString("\n")
			return m, nil
		case msg.String() == "pgup":
			m.viewport.HalfViewUp()
			return m, nil
		case msg.String() == "pgdown":
			m.viewport.HalfViewDown()
			return m, nil
		}
	}

	if !m.busy && !m.confirm && m.picking == "" {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.ready && !m.confirm && m.picking == "" {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *app) layout() {
	m.help.Width = m.width
	composerH := 5
	headerH := 3
	footerH := 2
	if m.showHelp {
		footerH = 4
	}
	chatW := max(40, m.width-34)
	if m.width < 92 {
		chatW = max(40, m.width-4)
	}
	chatH := max(6, m.height-headerH-composerH-footerH-1)
	m.viewport.Width = chatW
	m.viewport.Height = chatH
	m.input.SetWidth(max(20, m.width-8))
	m.picker.SetSize(min(56, max(36, m.width-10)), min(16, max(8, m.height/2)))
}

func (m *app) refreshViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(renderChat(m.messages, m.busy, m.phase, m.spin.View(), m.started))
	m.viewport.GotoBottom()
}

func (m app) phaseStatus(elapsed time.Duration) string {
	switch m.phase {
	case phaseGather:
		return fmt.Sprintf("%s gather · %s", m.spin.View(), elapsed.Round(time.Millisecond))
	case phaseSend:
		return fmt.Sprintf("%s send via %s · %s", m.spin.View(), m.provider, elapsed.Round(time.Millisecond))
	default:
		return m.status
	}
}

func (m app) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		m.confirm = false
		m.phase = phaseIdle
		m.pending = harness.Packet{}
		m.status = "send cancelled"
		m.messages = append(m.messages, message{"nano", "Cancelled. Nothing left the machine.", false})
		m.refreshViewport()
		return m, nil
	case "enter", "y", "Y":
		packet := m.pending
		m.confirm = false
		return m.dispatch(packet)
	}
	return m, nil
}

func (m app) openPicker(kind string) (tea.Model, tea.Cmd) {
	items := []list.Item{}
	title := "Provider"
	if kind == "provider" {
		for _, p := range harness.Profiles {
			items = append(items, pickItem{title: p.Label, desc: p.ID, id: p.ID})
		}
	} else {
		title = "Model"
		p, _ := harness.Find(m.provider)
		for _, name := range p.Models {
			label := name
			if label == "" {
				label = "vendor default"
			}
			items = append(items, pickItem{title: label, desc: m.provider, id: name})
		}
	}
	m.picker.Title = title
	m.picker.SetItems(items)
	m.picking = kind
	return m, nil
}

func (m app) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.picking = ""
		return m, nil
	case "enter":
		if item, ok := m.picker.SelectedItem().(pickItem); ok {
			if m.picking == "provider" {
				for i, p := range harness.Profiles {
					if p.ID == item.id {
						m.setProvider(i)
						break
					}
				}
			} else {
				m.models[m.provider] = item.id
				m.status = "model: " + displayModel(item.id)
			}
		}
		m.picking = ""
		return m, nil
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	return m, cmd
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
	m.busy = true
	m.phase = phaseGather
	m.started = time.Now()
	m.status = m.phaseStatus(0)
	m.refreshViewport()
	cfg := m.config()
	return m, tea.Batch(m.spin.Tick, func() tea.Msg {
		start := time.Now()
		packet, err := harness.Gather(cfg, prompt)
		return gatherMsg{packet, err, time.Since(start)}
	})
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
	m.refreshViewport()
	cfg := m.config()
	cfg.Write = m.write
	cites := packet.CiteCount
	provider := cfg.Provider
	return m, tea.Batch(m.spin.Tick, func() tea.Msg {
		start := time.Now()
		text, err := harness.Send(cfg, packet)
		return resultMsg{provider, text, err, cites, time.Since(start)}
	})
}

func (m app) command(prompt string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(prompt)
	if len(fields) == 0 {
		return m, nil
	}
	value := strings.TrimSpace(strings.TrimPrefix(prompt, fields[0]))
	switch fields[0] {
	case "/help":
		m.showHelp = true
		m.help.ShowAll = true
		m.messages = append(m.messages, message{"nano", "/super on|off · /query TERMS · /research QUESTION · /impact SYMBOL · /context on|off|clear · /provider NAME · /model NAME · /new · /exit", false})
	case "/exit":
		return m, tea.Quit
	case "/new":
		m.messages = nil
		m.evidence = nil
		m.pending = harness.Packet{}
		m.confirm = false
		m.phase = phaseIdle
		m.refreshViewport()
	case "/super":
		switch value {
		case "", "on", "true", "1":
			m.super, m.attach = true, true
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
	m.refreshViewport()
	return m, nil
}

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

	mode := "READ ONLY"
	if m.write {
		mode = "WRITE ARMED"
	}
	super, superColor := "SUPER OFF", muted
	if m.super {
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
	model := displayModel(m.models[m.provider])
	header := lipgloss.NewStyle().Background(surface).Padding(0, 1).Render(
		lipgloss.NewStyle().Foreground(bg).Background(lav).Bold(true).Padding(0, 1).Render("✦ nano "+Version) + " " +
			chip(super, superColor) + " " + chip(strings.ToUpper(string(m.phase)), phaseColor) + " " +
			chip(m.provider, lav) + " " + chip(model, muted) + " " +
			chip("● "+m.auth, teal) + " " + chip(mode, peach),
	)
	pipe := lipgloss.NewStyle().Foreground(muted).Render(" harness  " + pipeline(m.phase))

	chatBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(m.viewport.Width + 2).Render(m.viewport.View())
	inspector := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#45475a")).Padding(0, 1).Width(27).Render(m.inspectorView(lav))
	body := chatBox
	if m.width >= 92 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, chatBox, " ", inspector)
	}

	composerBorder := lav
	title := "ASK NANO · ENTER SENDS THROUGH HARNESS"
	inner := m.input.View()
	if m.super {
		title = "ASK NANO · SUPERPOWER SEND"
	}
	if m.confirm {
		composerBorder = yellow
		title = "CONFIRM HARNESS SEND"
		inner = harness.ConfirmSummary(m.config(), m.pending)
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
	model := displayModel(m.models[m.provider])
	body := lipgloss.NewStyle().Foreground(lav).Bold(true).Render("INSPECTOR") + "\n\n" +
		"PHASE\n" + string(m.phase) + "\n\n" +
		"SUPER\n" + map[bool]string{true: "on", false: "off"}[m.super] + "\n\n" +
		"BACKEND\n" + m.provider + "\n\nMODEL\n" + model + "\n\n" +
		"CONTEXT\n" + fmt.Sprintf("attach %t · %d cites", m.attach, len(m.evidence)) + "\n\n"
	if len(m.evidence) > 0 {
		body += "EVIDENCE\n" + summary(harness.Top(m.evidence, 5)) + "\n\n"
	}
	body += "bubbles\nviewport · list\nspinner · help"
	return body
}

func renderChat(messages []message, busy bool, p phase, spin string, started time.Time) string {
	peach := lipgloss.Color("#fab387")
	teal := lipgloss.Color("#94e2d5")
	red := lipgloss.Color("#f38ba8")
	yellow := lipgloss.Color("#f9e2af")
	lav := lipgloss.Color("#b4befe")
	var b strings.Builder
	for _, msg := range messages {
		c := peach
		switch {
		case msg.err:
			c = red
		case msg.role == "you":
			c = teal
		case msg.role == "context":
			c = yellow
		}
		b.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render("▎ "+strings.ToUpper(msg.role)) + "\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")).Render(clipText(msg.text, 4000)) + "\n\n")
	}
	if busy {
		label := "sending through harness…"
		if p == phaseGather {
			label = "gathering local evidence…"
		}
		elapsed := ""
		if !started.IsZero() {
			elapsed = " · " + time.Since(started).Round(time.Millisecond).String()
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lav).Italic(true).Render(spin + " " + label + elapsed))
	}
	return b.String()
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

func providerIndex(id string) int {
	for i, p := range harness.Profiles {
		if p.ID == id {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

// Run starts the Superpower TUI. All sends go through internal/harness.
func Run() error {
	p := tea.NewProgram(initialApp(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
