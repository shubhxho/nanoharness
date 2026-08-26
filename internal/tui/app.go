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

type app struct {
	input    textarea.Model
	viewport viewport.Model
	spin     spinner.Model
	help     help.Model
	picker   list.Model
	session  *harness.Session

	models   map[string]string
	messages []message
	status   string
	auth     string
	busy     bool
	confirm  bool
	showHelp bool
	picking  string // "", "provider", "model"
	phase    phase
	started  time.Time
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
		session:  harness.NewSession("codex"),
		models:   models,
		phase:    phaseIdle,
		status:   "superpower on · ready",
		auth:     harness.NewSession("codex").Auth(),
		messages: []message{{"nano", fmt.Sprintf("nanoharness %s — Superpower send runs gather → confirm → send through the harness Session. F1 help · F5 super.", Version), false}},
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
			m.session.Remember(msg.packet.Citations)
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
		if m.session.Write {
			m.session.WithWrite(false)
		}
		m.refreshViewport()
		return m, nil

	case contextMsg:
		if msg.err != nil {
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
		} else {
			m.session.Remember(msg.report.Citations)
			m.messages = append(m.messages, message{"context", fmt.Sprintf("LOCAL LEXICAL %s\nExact token/path evidence only.\nquery: %s\n%s", strings.ToUpper(msg.kind), msg.query, summary(m.session.Evidence)), false})
			m.status = fmt.Sprintf("%d citations ready", len(m.session.Evidence))
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
			m.session.WithAttach(!m.session.Attach)
			m.status = map[bool]string{true: "context attachment on", false: "context attachment off"}[m.session.Attach]
			return m, nil
		case key.Matches(msg, keys.Super):
			m.session.WithSuper(!m.session.Super)
			m.status = map[bool]string{true: "superpower on", false: "superpower off"}[m.session.Super]
			return m, nil
		case key.Matches(msg, keys.Write):
			if m.session.Provider == "codex" {
				m.session.WithWrite(!m.session.Write)
				m.status = map[bool]string{true: "workspace write armed", false: "read-only"}[m.session.Write]
			}
			return m, nil
		case msg.String() == "tab":
			m.setProvider((providerIndex(m.session.Provider) + 1) % len(harness.Profiles))
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
		return fmt.Sprintf("%s send via %s · %s", m.spin.View(), m.session.Provider, elapsed.Round(time.Millisecond))
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
		p, _ := harness.Find(m.session.Provider)
		for _, name := range p.Models {
			label := name
			if label == "" {
				label = "vendor default"
			}
			items = append(items, pickItem{title: label, desc: m.session.Provider, id: name})
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
				m.models[m.session.Provider] = item.id
				m.session.WithModel(item.id)
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
	m.session.Provider = harness.Profiles[index].ID
	m.session.WithModel(m.models[m.session.Provider])
	m.auth = m.session.Auth()
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
	session := m.liveSession()
	return m, tea.Batch(m.spin.Tick, func() tea.Msg {
		start := time.Now()
		packet, err := session.Gather(prompt)
		return gatherMsg{packet, err, time.Since(start)}
	})
}

func (m app) liveSession() *harness.Session {
	s := m.session
	if s == nil {
		s = harness.NewSession("codex")
	}
	s.WithModel(m.models[s.Provider])
	return s
}

func (m app) dispatch(packet harness.Packet) (tea.Model, tea.Cmd) {
	m.busy = true
	m.phase = phaseSend
	m.started = time.Now()
	m.status = m.phaseStatus(0)
	m.refreshViewport()
	session := m.liveSession()
	cites := packet.CiteCount
	provider := session.Provider
	return m, tea.Batch(m.spin.Tick, func() tea.Msg {
		start := time.Now()
		text, err := session.Send(packet)
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
		m.session.WithEvidence(nil)
		m.pending = harness.Packet{}
		m.confirm = false
		m.phase = phaseIdle
		m.refreshViewport()
	case "/super":
		switch value {
		case "", "on", "true", "1":
			m.session.WithSuper(true)
		case "off", "false", "0":
			m.session.WithSuper(false)
		case "status":
		default:
			m.status = "usage: /super on|off|status"
			return m, nil
		}
		m.status = fmt.Sprintf("superpower: %t · attach: %t · %d citations", m.session.Super, m.session.Attach, len(m.session.Evidence))
	case "/context":
		switch value {
		case "on":
			m.session.WithAttach(true)
		case "off":
			m.session.WithAttach(false)
		case "clear":
			m.session.WithEvidence(nil)
		}
		m.status = fmt.Sprintf("context attach: %t · %d citations · super %t", m.session.Attach, len(m.session.Evidence), m.session.Super)
	case "/provider":
		for i, p := range harness.Profiles {
			if p.ID == value {
				m.setProvider(i)
			}
		}
	case "/model":
		if value != "" {
			m.models[m.session.Provider] = value
			m.session.WithModel(value)
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
		session := m.liveSession()
		return m, func() tea.Msg {
			r, err := session.Search(value, mode)
			return contextMsg{value, kind, r, err}
		}
	default:
		m.status = "unknown command"
	}
	m.refreshViewport()
	return m, nil
}
