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
	"github.com/shubhxho/nanoharness/internal/terminal"
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
	history  []string
	histIdx  int
	status   string
	auth     string
	busy     bool
	confirm  bool
	showHelp bool
	picking  string // "", "provider", "model"
	phase    phase
	started  time.Time
	pending  harness.Packet
	term     terminal.Info
	focused  bool
	width    int
	height   int
	ready    bool
}

func initialApp(term terminal.Info) app {
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
	sp.Style = lipgloss.NewStyle().Foreground(colorLav)

	h := help.New()
	h.ShowAll = false

	models := map[string]string{}
	for _, p := range harness.Profiles {
		models[p.ID] = p.Default
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(colorLav).BorderForeground(colorLav)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(colorTeal)
	picker := list.New([]list.Item{}, delegate, 40, 12)
	picker.SetShowStatusBar(false)
	picker.SetFilteringEnabled(true)
	picker.SetShowHelp(false)
	picker.Styles.Title = lipgloss.NewStyle().Foreground(colorLav).Bold(true)

	welcome := fmt.Sprintf("nanoharness %s — Superpower Session is on.\nEnter → gather → confirm → send through harness.\n↑/↓ history · F1 help · F5 super · F6 status · Ctrl+N new session.", Version)
	if term.Ghostty {
		welcome += " Ghostty detected — truecolor + focus reporting enabled."
	}
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
		term:     term,
		focused:  true,
		messages: []message{{"nano", welcome, false}},
		histIdx:  -1,
	}
}

func (m app) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spin.Tick)
}

func (m app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.FocusMsg:
		m.focused = true
		if m.status == "unfocused" {
			m.status = "superpower on · ready"
		}
		return m, nil

	case tea.BlurMsg:
		m.focused = false
		if !m.busy && !m.confirm {
			m.status = "unfocused"
		}
		return m, nil

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
			m.status = fmt.Sprintf("ready · %d cites · %s · %s", msg.cites, msg.elapsed.Round(time.Millisecond), m.liveSession().PipelineLine())
		}
		m.refreshViewport()
		return m, nil

	case contextMsg:
		if msg.err != nil {
			m.messages = append(m.messages, message{"error", msg.err.Error(), true})
		} else {
			m.messages = append(m.messages, message{"context", fmt.Sprintf("LOCAL LEXICAL %s\nExact token/path evidence only.\nquery: %s\n%s", strings.ToUpper(msg.kind), msg.query, summary(m.session.Evidence)), false})
			m.status = fmt.Sprintf("%d citations ready · %s", len(m.session.Evidence), m.session.PipelineLine())
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
		case key.Matches(msg, keys.Status):
			m.messages = append(m.messages, message{"nano", m.liveSession().Status(Version), false})
			m.status = "status"
			m.refreshViewport()
			return m, nil
		case key.Matches(msg, keys.NewSession):
			return m.resetSession()
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
		case key.Matches(msg, keys.Clear):
			m.input.Reset()
			m.status = "composer cleared"
			return m, nil
		case msg.String() == "up":
			if strings.TrimSpace(m.input.Value()) == "" || m.histIdx >= 0 {
				return m.historyUp()
			}
		case msg.String() == "down":
			if m.histIdx >= 0 {
				return m.historyDown()
			}
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
	m.pushHistory(prompt)
	m.busy = true
	m.phase = phaseGather
	m.started = time.Now()
	m.status = m.phaseStatus(0)
	m.refreshViewport()
	session := m.liveSession()
	return m, tea.Batch(m.spin.Tick, func() tea.Msg {
		packet, elapsed, err := session.GatherTimed(prompt)
		return gatherMsg{packet, err, elapsed}
	})
}

func (m *app) pushHistory(prompt string) {
	if prompt == "" {
		return
	}
	if n := len(m.history); n > 0 && m.history[n-1] == prompt {
		m.histIdx = -1
		return
	}
	m.history = append(m.history, prompt)
	if len(m.history) > 50 {
		m.history = m.history[len(m.history)-50:]
	}
	m.histIdx = -1
}

func (m app) historyUp() (tea.Model, tea.Cmd) {
	if len(m.history) == 0 {
		return m, nil
	}
	if m.histIdx < 0 {
		m.histIdx = len(m.history) - 1
	} else if m.histIdx > 0 {
		m.histIdx--
	}
	m.input.SetValue(m.history[m.histIdx])
	m.input.CursorEnd()
	return m, nil
}

func (m app) historyDown() (tea.Model, tea.Cmd) {
	if m.histIdx < 0 {
		return m, nil
	}
	if m.histIdx >= len(m.history)-1 {
		m.histIdx = -1
		m.input.Reset()
		return m, nil
	}
	m.histIdx++
	m.input.SetValue(m.history[m.histIdx])
	m.input.CursorEnd()
	return m, nil
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
		text, elapsed, err := session.SendTimed(packet)
		return resultMsg{provider, text, err, cites, elapsed}
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
		m.messages = append(m.messages, message{"nano", "/super · /goal · /memory · /gates · /memories · /evidence · /pipeline · /last · /auto · /gate · /status · /query · /research · /impact · /context · /provider · /model · /new · /exit", false})
	case "/exit":
		return m, tea.Quit
	case "/status", "/terminal":
		m.messages = append(m.messages, message{"nano", m.liveSession().Status(Version), false})
		m.status = "status"
	case "/pipeline":
		m.messages = append(m.messages, message{"nano", m.liveSession().PipelineLine(), false})
		m.status = "pipeline"
	case "/last":
		if t, ok := m.session.LastTurn(); ok {
			m.messages = append(m.messages, message{"nano", harness.FormatLastTurn(t), false})
		} else {
			m.messages = append(m.messages, message{"nano", "no turns yet — Enter sends through harness", false})
		}
		m.status = "last turn"
	case "/evidence":
		if len(m.session.Evidence) == 0 {
			m.messages = append(m.messages, message{"nano", "evidence: (none) — try /query or send with superpower on", false})
		} else {
			m.messages = append(m.messages, message{"nano", "evidence:\n" + summary(m.session.Evidence), false})
		}
		m.status = "evidence"
	case "/goal":
		if value == "" {
			g := strings.TrimSpace(m.session.Continual.Goal)
			if g == "" {
				g = "(none)"
			}
			m.messages = append(m.messages, message{"nano", "goal: " + g, false})
			m.status = "goal"
			break
		}
		m.session.WithGoal(value)
		m.status = "goal set"
	case "/memory":
		if value == "" {
			m.status = "usage: /memory NOTE"
			return m, nil
		}
		m.session.RememberNote(value)
		m.status = fmt.Sprintf("memory saved (%d)", len(m.session.Continual.Memories))
	case "/auto":
		switch value {
		case "", "on", "true", "1":
			m.session.WithAutonomous(true)
			m.status = "autonomous on (tools armed)"
		case "off", "false", "0":
			m.session.WithAutonomous(false)
			m.status = "autonomous off"
		default:
			m.status = "usage: /auto on|off"
			return m, nil
		}
	case "/gate":
		if value == "" {
			m.status = "usage: /gate COMMAND"
			return m, nil
		}
		m.session.WithGate(value)
		m.status = fmt.Sprintf("gate added (%d)", len(m.session.Continual.Gates))
	case "/gates":
		if len(m.session.Continual.Gates) == 0 {
			m.messages = append(m.messages, message{"nano", "gates: (none)", false})
		} else {
			m.messages = append(m.messages, message{"nano", "gates:\n" + summaryGates(m.session.Continual.Gates), false})
		}
		m.status = "gates"
	case "/memories":
		if len(m.session.Continual.Memories) == 0 {
			m.messages = append(m.messages, message{"nano", "memories: (none)", false})
		} else {
			var b strings.Builder
			b.WriteString("memories:\n")
			for i, note := range m.session.Continual.Memories {
				fmt.Fprintf(&b, "%02d  %s\n", i+1, clipText(note, 200))
			}
			m.messages = append(m.messages, message{"nano", strings.TrimRight(b.String(), "\n"), false})
		}
		m.status = "memories"
	case "/new":
		return m.resetSession()
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
			r, err := session.SearchRemember(value, mode)
			return contextMsg{value, kind, r, err}
		}
	default:
		m.status = "unknown command"
	}
	m.refreshViewport()
	return m, nil
}

func (m app) resetSession() (tea.Model, tea.Cmd) {
	m.session.Reset()
	m.pending = harness.Packet{}
	m.confirm = false
	m.busy = false
	m.phase = phaseIdle
	m.messages = []message{{"nano", fmt.Sprintf("new session — superpower on · provider %s · Enter sends through harness.", m.session.Provider), false}}
	m.status = m.session.PipelineLine()
	m.refreshViewport()
	return m, nil
}
