package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"cli-agent/internal/app"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type focusArea int

const (
	focusInput focusArea = iota
	focusChat
	focusTrace
)

type Message struct {
	ID        string
	Role      string
	Content   string
	Timestamp time.Time

	IsFileEdit bool
	FilePath   string
	ChangeType string
	OldContent string
	NewContent string
}

type TraceItem struct {
	ID        string
	Time      time.Time
	Kind      app.AgentEventKind
	ToolName  string
	Summary   string
	Detail    string
	Success   *bool
	Duration  int64
	Expanded  bool
	FilePath  string
	Change    string
	Old       string
	New       string
}

type spinMsg struct{}

type agentEventMsg struct{ ev app.AgentEvent }
type agentDoneMsg struct {
	state *app.AgentState
	err   error
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var modes = []app.Mode{app.ModePlan, app.ModeCode, app.ModeDo}

type MainModel struct {
	app      *app.Application
	mode     app.Mode
	modeIndex int

	theme Theme
	help  helpModel

	width  int
	height int
	ready  bool

	focus focusArea

	messages []Message
	input    textarea.Model
	chatVP   viewport.Model

	showTrace bool
	traceOnly bool
	trace      []TraceItem
	traceSel   int
	traceOff   int

	markdown     *MarkdownRenderer
	diffRenderer *DiffRenderer

	running     bool
	statusText  string
	spinnerPos  int
	maxLoops    int
	cancel      context.CancelFunc
	eventsCh    chan app.AgentEvent
	doneCh      chan agentDoneMsg

	lastTick time.Time
}

func NewMainModel(application *app.Application, mode app.Mode) *MainModel {
	ta := textarea.New()
	ta.Placeholder = "Ask, then press Enter. Tab switches focus."
	ta.Focus()
	ta.CharLimit = 8000
	ta.SetHeight(1)
	ta.Prompt = " "
	ta.ShowLineNumbers = false

	// Keep textarea styling minimal; we style the input container instead.
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()

	modeIndex := 0
	for i, m := range modes {
		if m == mode {
			modeIndex = i
			break
		}
	}

	t := NewTheme()
	m := &MainModel{
		app:          application,
		mode:         mode,
		modeIndex:    modeIndex,
		theme:        t,
		help:         newHelpModel(),
		width:        100,
		height:       30,
		focus:        focusInput,
		showTrace:    true,
		traceOnly:    false,
		trace:        nil,
		messages:     nil,
		input:        ta,
		markdown:     NewMarkdownRenderer(),
		diffRenderer: NewDiffRenderer(),
		running:      false,
		statusText:   "Ready",
		maxLoops:     30,
	}

	m.messages = append(m.messages, Message{
		ID:        "welcome-1",
		Role:      "system",
		Content:   "eai ready. Enter runs the agent. Ctrl+T toggles trace. Ctrl+C cancels a run.",
		Timestamp: time.Now(),
	})

	// Prefer a calmer default in small terminals.
	if os.Getenv("EAI_SHOW_TRACE") == "0" {
		m.showTrace = false
	}

	return m
}

func (m *MainModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.help.SetWidth(m.width)

		layout := m.computeLayout()
		if !m.ready {
			m.chatVP = viewport.New(layout.ChatW, layout.ChatH)
			m.chatVP.Style = lipgloss.NewStyle()
			m.ready = true
		} else {
			m.chatVP.Width = layout.ChatW
			m.chatVP.Height = layout.ChatH
		}
		m.input.SetWidth(max(10, layout.InputW))
		m.updateChatViewport()
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.help.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.help.keys.NextMode):
			m.modeIndex = (m.modeIndex + 1) % len(modes)
			m.mode = modes[m.modeIndex]
			return m, nil

		case key.Matches(msg, m.help.keys.ToggleTrace):
			if m.width < 100 {
				m.traceOnly = !m.traceOnly
				m.showTrace = true
				if m.traceOnly {
					m.focus = focusTrace
				} else {
					m.focus = focusInput
				}
			} else {
				m.showTrace = !m.showTrace
				if !m.showTrace && m.focus == focusTrace {
					m.focus = focusInput
				}
			}
			return m, nil

		case key.Matches(msg, m.help.keys.FocusNext):
			m.cycleFocus()
			return m, nil

		case key.Matches(msg, m.help.keys.Cancel):
			if m.running && m.cancel != nil {
				m.statusText = "Cancelling…"
				m.cancel()
			}
			return m, nil

		case key.Matches(msg, m.help.keys.Clear):
			m.messages = []Message{{
				ID:        "welcome-1",
				Role:      "system",
				Content:   "cleared.",
				Timestamp: time.Now(),
			}}
			m.trace = nil
			m.traceSel = 0
			m.traceOff = 0
			m.updateChatViewport()
			return m, nil

		case key.Matches(msg, m.help.keys.Enter):
			if m.focus == focusTrace {
				m.toggleTraceExpanded()
				return m, nil
			}
			return m, m.onEnter()

		case msg.Type == tea.KeyUp:
			if m.focus == focusChat {
				m.chatVP.LineUp(1)
				return m, nil
			}
			if m.focus == focusTrace {
				m.moveTrace(-1)
				return m, nil
			}
		case msg.Type == tea.KeyDown:
			if m.focus == focusChat {
				m.chatVP.LineDown(1)
				return m, nil
			}
			if m.focus == focusTrace {
				m.moveTrace(1)
				return m, nil
			}
		case msg.Type == tea.KeyPgUp:
			m.chatVP.ViewUp()
			return m, nil
		case msg.Type == tea.KeyPgDown:
			m.chatVP.ViewDown()
			return m, nil
		}

	case agentEventMsg:
		m.appendTraceEvent(msg.ev)
		if m.running {
			return m, m.waitAgentMsg()
		}
		return m, nil

	case agentDoneMsg:
		m.running = false
		m.statusText = "Ready"
		m.cancel = nil
		m.eventsCh = nil
		m.doneCh = nil

		if msg.err != nil {
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
				Role:      "error",
				Content:   fmt.Sprintf("error: %v", msg.err),
				Timestamp: time.Now(),
			})
		} else if msg.state != nil {
			out := strings.TrimSpace(msg.state.FinalOutput)
			if out == "" {
				out = "Done."
			}
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("ai-%d", time.Now().UnixNano()),
				Role:      "assistant",
				Content:   out,
				Timestamp: time.Now(),
			})
		}
		m.updateChatViewport()
		m.chatVP.GotoBottom()
		return m, nil

	case spinMsg:
		m.spinnerPos = (m.spinnerPos + 1) % len(spinnerFrames)
		if m.running {
			return m, m.spinTick()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.chatVP, cmd = m.chatVP.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *MainModel) View() string {
	if !m.ready {
		return "…"
	}

	layout := m.computeLayout()
	top := m.renderTopBar()
	main := m.renderMain(layout)
	input := m.renderInputArea(layout)
	footer := m.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Left, top, main, input, footer)
}

func (m *MainModel) onEnter() tea.Cmd {
	val := strings.TrimSpace(m.input.Value())
	if val == "" {
		return nil
	}

	// Keep /connect flow from previous UI.
	if strings.HasPrefix(val, "/connect") {
		cfg, _ := app.LoadConfig(app.DefaultConfigPath())
		wizard := NewSetupWizard(&cfg)
		p := tea.NewProgram(wizard)
		if _, err := p.Run(); err != nil {
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
				Role:      "error",
				Content:   fmt.Sprintf("connection error: %v", err),
				Timestamp: time.Now(),
			})
		} else if wizard.Saved() {
			newCfg := wizard.GetConfig()
			m.app.ReloadClient(newCfg)
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
				Role:      "system",
				Content:   "connected.",
				Timestamp: time.Now(),
			})
		}
		m.input.Reset()
		m.updateChatViewport()
		m.chatVP.GotoBottom()
		return nil
	}

	m.messages = append(m.messages, Message{
		ID:        fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Role:      "user",
		Content:   val,
		Timestamp: time.Now(),
	})
	m.input.Reset()
	m.updateChatViewport()
	m.chatVP.GotoBottom()

	if m.running {
		m.messages = append(m.messages, Message{
			ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
			Role:      "system",
			Content:   "already running (Ctrl+C to cancel).",
			Timestamp: time.Now(),
		})
		m.updateChatViewport()
		m.chatVP.GotoBottom()
		return nil
	}

	m.running = true
	m.statusText = "Running agent…"
	m.spinnerPos = 0
	m.traceOnly = false

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.eventsCh = make(chan app.AgentEvent, 256)
	m.doneCh = make(chan agentDoneMsg, 1)

	go func(task string, maxLoops int, events chan app.AgentEvent, done chan agentDoneMsg) {
		state, err := m.app.ExecuteAgentStream(ctx, task, maxLoops, func(ev app.AgentEvent) {
			select {
			case events <- ev:
			default:
				// Drop if UI can't keep up; trace is best-effort.
			}
		})
		done <- agentDoneMsg{state: state, err: err}
		close(events)
	}(val, m.maxLoops, m.eventsCh, m.doneCh)

	return tea.Batch(m.waitAgentMsg(), m.spinTick())
}

func (m *MainModel) waitAgentMsg() tea.Cmd {
	events := m.eventsCh
	done := m.doneCh
	return func() tea.Msg {
		if events == nil || done == nil {
			return nil
		}
		select {
		case ev, ok := <-events:
			if ok {
				return agentEventMsg{ev: ev}
			}
			// Events channel closed; wait for final result.
			return <-done
		case d := <-done:
			return d
		}
	}
}

func (m *MainModel) spinTick() tea.Cmd {
	// Reduced motion option.
	d := 90 * time.Millisecond
	if os.Getenv("EAI_REDUCE_MOTION") == "1" {
		d = 250 * time.Millisecond
	}
	return tea.Tick(d, func(_ time.Time) tea.Msg { return spinMsg{} })
}

func (m *MainModel) cycleFocus() {
	next := m.focus + 1
	if next > focusTrace {
		next = focusInput
	}
	// Skip trace focus if hidden.
	if next == focusTrace && (!m.showTrace || m.traceOnly && m.width >= 100) {
		next = focusInput
	}
	m.focus = next
	if m.focus == focusInput {
		m.input.Focus()
	} else {
		m.input.Blur()
	}
}

func (m *MainModel) updateChatViewport() {
	var b strings.Builder
	chatWidth := m.computeLayout().ChatW - 2
	if chatWidth < 20 {
		chatWidth = 20
	}
	for _, msg := range m.messages {
		b.WriteString(m.renderMessage(msg, chatWidth))
		b.WriteString("\n\n")
	}
	m.chatVP.SetContent(strings.TrimRight(b.String(), "\n"))
}

func (m *MainModel) renderMessage(msg Message, width int) string {
	if msg.IsFileEdit {
		return FormatEditMessage(msg.FilePath, msg.ChangeType, msg.OldContent, msg.NewContent)
	}

	var roleStyle lipgloss.Style
	roleLabel := "SYS"
	switch msg.Role {
	case "user":
		roleStyle = m.theme.RoleYou
		roleLabel = "YOU"
	case "assistant":
		roleStyle = m.theme.RoleAI
		roleLabel = "EAI"
	case "error":
		roleStyle = m.theme.RoleErr
		roleLabel = "ERR"
	default:
		roleStyle = m.theme.RoleSys
		roleLabel = "SYS"
	}

	head := roleStyle.Render(roleLabel)
	meta := m.theme.TopBarMeta.Render(msg.Timestamp.Format("15:04"))
	header := head + " " + meta

	var body string
	if msg.Role == "assistant" {
		body = m.markdown.Render(msg.Content, width)
	} else {
		body = lipgloss.NewStyle().Foreground(m.theme.TextPrimary).Width(width).Render(msg.Content)
	}

	return header + "\n" + body
}

func (m *MainModel) appendTraceEvent(ev app.AgentEvent) {
	item := TraceItem{
		ID:       fmt.Sprintf("tr-%d", time.Now().UnixNano()),
		Time:     ev.Timestamp,
		Kind:     ev.Kind,
		ToolName: ev.ToolName,
		Summary:  ev.Summary,
		Detail:   ev.Detail,
		Success:  ev.Success,
		Duration: ev.DurationMs,
		FilePath: ev.FilePath,
		Change:   ev.ChangeType,
		Old:      ev.OldContent,
		New:      ev.NewContent,
	}
	m.trace = append(m.trace, item)

	// Show diffs in the chat pane as well.
	if ev.Kind == app.AgentEventFileChange && ev.FilePath != "" {
		m.messages = append(m.messages, Message{
			ID:         fmt.Sprintf("edit-%d", time.Now().UnixNano()),
			Role:       "system",
			IsFileEdit: true,
			FilePath:   ev.FilePath,
			ChangeType: ev.ChangeType,
			OldContent: ev.OldContent,
			NewContent: ev.NewContent,
			Timestamp:  time.Now(),
		})
	}

	// Keep selection pinned to the latest event when the trace isn't focused.
	if m.focus != focusTrace {
		m.traceSel = len(m.trace) - 1
	}
	m.normalizeTraceScroll()
	m.updateChatViewport()
}

func (m *MainModel) moveTrace(delta int) {
	if len(m.trace) == 0 {
		return
	}
	m.traceSel += delta
	if m.traceSel < 0 {
		m.traceSel = 0
	}
	if m.traceSel >= len(m.trace) {
		m.traceSel = len(m.trace) - 1
	}
	m.normalizeTraceScroll()
}

func (m *MainModel) toggleTraceExpanded() {
	if len(m.trace) == 0 || m.traceSel < 0 || m.traceSel >= len(m.trace) {
		return
	}
	m.trace[m.traceSel].Expanded = !m.trace[m.traceSel].Expanded
}

func (m *MainModel) normalizeTraceScroll() {
	layout := m.computeLayout()
	visible := layout.TraceListH
	if visible <= 0 {
		visible = 1
	}
	if m.traceSel < m.traceOff {
		m.traceOff = m.traceSel
	}
	if m.traceSel >= m.traceOff+visible {
		m.traceOff = m.traceSel - visible + 1
	}
	if m.traceOff < 0 {
		m.traceOff = 0
	}
	maxOff := len(m.trace) - visible
	if maxOff < 0 {
		maxOff = 0
	}
	if m.traceOff > maxOff {
		m.traceOff = maxOff
	}
}

type layoutInfo struct {
	TopH int
	FootH int

	MainH int

	ChatW int
	ChatH int

	TraceW int
	TraceH int
	TraceListH int
	TraceDetailH int

	InputH int
	InputW int
}

func (m *MainModel) computeLayout() layoutInfo {
	top := 1
	foot := 1
	inputH := 3
	mainH := m.height - top - foot - inputH
	if mainH < 3 {
		mainH = 3
	}

	// When narrow, allow trace-only toggle.
	if m.traceOnly {
		return layoutInfo{
			TopH: top, FootH: foot, MainH: mainH,
			ChatW: m.width, ChatH: mainH,
			TraceW: m.width, TraceH: mainH,
			TraceListH: max(1, mainH-4),
			TraceDetailH: 3,
			InputH: inputH,
			InputW: max(10, m.width-4),
		}
	}

	showTrace := m.showTrace && m.width >= 100
	chatW := m.width
	traceW := 0
	if showTrace {
		gap := 1
		chatW = int(float64(m.width-gap) * 0.62)
		if chatW < 50 {
			chatW = 50
		}
		traceW = m.width - gap - chatW
		if traceW < 32 {
			traceW = 32
			chatW = m.width - gap - traceW
		}
	}

	traceListH := max(1, mainH-5)
	traceDetailH := 4
	if mainH < 10 {
		traceListH = mainH - 2
		traceDetailH = 0
	}

	return layoutInfo{
		TopH: top, FootH: foot, MainH: mainH,
		ChatW: chatW, ChatH: mainH,
		TraceW: traceW, TraceH: mainH,
		TraceListH: traceListH,
		TraceDetailH: traceDetailH,
		InputH: inputH,
		InputW: chatW - 4,
	}
}

func (m *MainModel) renderTopBar() string {
	left := m.theme.TopBarTitle.Render("eai") + " " + m.theme.TopBarBadge.Render(strings.ToUpper(string(m.mode)))
	status := m.statusText
	if m.running {
		status = spinnerFrames[m.spinnerPos] + " " + m.statusText
		status = m.theme.Spinner.Render(status)
	} else {
		status = m.theme.TopBarMeta.Render(status)
	}
	right := m.theme.TopBarMeta.Render(time.Now().Format("15:04"))

	// Compose with spacing.
	gap1 := m.width - lipgloss.Width(left) - lipgloss.Width(status) - lipgloss.Width(right)
	if gap1 < 2 {
		gap1 = 2
	}
	// Split the gap into two parts.
	a := gap1 / 2
	b := gap1 - a
	return m.theme.TopBar.Render(left + strings.Repeat(" ", a) + status + strings.Repeat(" ", b) + right)
}

func (m *MainModel) renderFooter() string {
	hints := "Tab focus  Ctrl+T trace  Ctrl+C cancel  Shift+Tab mode  q quit"
	if m.width < 80 {
		hints = "Tab focus  Ctrl+T trace  Ctrl+C cancel  q quit"
	}
	return m.theme.Footer.Width(m.width).Render(hints)
}

func (m *MainModel) renderInputArea(l layoutInfo) string {
	box := m.theme.InputBox
	if m.focus == focusInput {
		box = m.theme.InputBoxF
	}
	// Always render input even in trace-only mode so the user can switch back easily.
	w := l.ChatW
	if m.traceOnly {
		w = m.width
	}
	inner := m.input.View()
	return box.Width(max(10, w-2)).Render(inner)
}

func (m *MainModel) renderMain(l layoutInfo) string {
	chatPane := m.renderChatPane(l)

	if m.traceOnly {
		tracePane := m.renderTracePane(l)
		return tracePane
	}

	if m.showTrace && l.TraceW > 0 {
		tracePane := m.renderTracePane(l)
		sep := m.theme.PaneDivider.Render("│")
		return lipgloss.JoinHorizontal(lipgloss.Top, chatPane, sep, tracePane)
	}
	return chatPane
}

func (m *MainModel) renderChatPane(l layoutInfo) string {
	title := "Chat"
	if m.focus == focusChat {
		title = m.theme.PaneTitleF.Render(title)
	} else {
		title = m.theme.PaneTitle.Render(title)
	}
	content := m.chatVP.View()
	box := m.theme.Pane
	if m.focus == focusChat {
		box = m.theme.PaneFocused
	}
	return box.Width(l.ChatW).Height(l.ChatH).Render(title + "\n" + content)
}

func (m *MainModel) renderTracePane(l layoutInfo) string {
	titleText := fmt.Sprintf("Trace (%d)", len(m.trace))
	var title string
	box := m.theme.Pane
	if m.focus == focusTrace {
		box = m.theme.PaneFocused
		title = m.theme.PaneTitleF.Render(titleText)
	} else {
		title = m.theme.PaneTitle.Render(titleText)
	}

	list := m.renderTraceList(l)
	detail := m.renderTraceDetail(l)

	var content string
	if l.TraceDetailH > 0 {
		content = title + "\n" + list + "\n" + detail
	} else {
		content = title + "\n" + list
	}

	w := l.TraceW
	h := l.TraceH
	if m.traceOnly {
		w = m.width
		h = l.MainH
	}
	return box.Width(w).Height(h).Render(content)
}

func (m *MainModel) renderTraceList(l layoutInfo) string {
	if len(m.trace) == 0 {
		return m.theme.TraceNeutral.Render("No events yet.")
	}

	start := m.traceOff
	end := start + l.TraceListH
	if end > len(m.trace) {
		end = len(m.trace)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		it := m.trace[i]
		prefix := "  "
		lineStyle := m.theme.TraceNeutral
		if it.Success != nil {
			if *it.Success {
				lineStyle = m.theme.TraceOK
			} else {
				lineStyle = m.theme.TraceERR
			}
		}
		if i == m.traceSel {
			prefix = "> "
			lineStyle = m.theme.TraceSel
		}
		s := it.Summary
		if strings.TrimSpace(s) == "" {
			s = string(it.Kind)
		}
		// Keep list lines compact.
		s = truncateRunes(oneLineTUI(s), max(12, l.TraceW-10))
		b.WriteString(lineStyle.Render(prefix + s))
		if i != end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m *MainModel) renderTraceDetail(l layoutInfo) string {
	if len(m.trace) == 0 || m.traceSel < 0 || m.traceSel >= len(m.trace) {
		return ""
	}
	it := m.trace[m.traceSel]
	if !it.Expanded {
		return m.theme.TraceNeutral.Render("Enter: expand")
	}

	detail := strings.TrimSpace(it.Detail)
	if detail == "" && it.FilePath != "" {
		detail = fmt.Sprintf("%s %s", it.Change, it.FilePath)
	}
	if detail == "" {
		detail = "(no detail)"
	}
	return m.theme.TraceNeutral.Render(truncateForBox(detail, max(10, l.TraceW-6), l.TraceDetailH))
}

func truncateForBox(s string, width int, height int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = truncateRunes(lines[i], width)
	}
	if height > 0 && len(lines) > height {
		lines = append(lines[:max(1, height-1)], "[truncated]")
	}
	return strings.Join(lines, "\n")
}

func truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	if maxRunes <= 1 {
		return string(r[:maxRunes])
	}
	return string(r[:maxRunes-1]) + "…"
}

func oneLineTUI(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
