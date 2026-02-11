package tui

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"cli-agent/internal/app"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
	"github.com/muesli/reflow/wrap"
)

// Layout constants - minimal design
const (
	minWidth  = 60
	minHeight = 20
)

const (
	foldMaxLines  = 40
	foldMaxChars  = 6000
	resumeListMax = 20
	inputMinLines = 1
	inputMaxLines = 8
)

type MainModel struct {
	app          *app.Application
	mode         app.Mode
	messages     []Message
	title        string
	session      *app.Session
	input        textarea.Model
	inputHistory []string
	historyIndex int
	historyDraft string
	viewport     viewport.Model
	help         helpModel
	width        int
	height       int
	markdown     *MarkdownRenderer
	diffRenderer *DiffRenderer
	loading      bool
	loadingText  string
	spinnerPos   int
	modeIndex    int
	ready        bool

	progressCh chan app.ProgressEvent

	// Per-request progress events used to render an OpenCode-style timeline in
	// the assistant message once execution finishes.
	timelineEnabled  bool
	turnProgress     []app.ProgressEvent
	turnEvents       []app.ProgressEvent
	turnProgressFrom time.Time

	// When false, new messages won't auto-scroll the viewport; we'll show a
	// "new:N" indicator in the status bar instead.
	stickToBottom bool
	unseenCount   int

	expandedMessageIDs map[string]bool

	showBanner bool
	verbosity  string

	resumePickerActive bool
	resumeItems        []app.SessionSummary
	resumeIndex        int
	resumeLoadErr      string
	resumeLoadedAt     time.Time

	planDecisionActive bool
	planDecisionChoice int
	pendingPlanText    string
	pendingPlanContext string
}

type Message struct {
	ID        string
	Role      string
	Content   string
	Timestamp time.Time
	IsStatus  bool
	IsPlan    bool
	// For file edits
	IsFileEdit bool
	FilePath   string
	ChangeType string
	OldContent string
	NewContent string
}

type spinMsg struct{}

type aiResponseMsg struct {
	response string
	err      error
}

type progressUpdateMsg struct {
	event app.ProgressEvent
}

type progressDoneMsg struct{}

type titleMsg struct {
	title string
}

type resumeTickMsg struct{}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
var modes = []app.Mode{app.ModePlan, app.ModeCreate}

func looksLikeWebsiteHTMLRequest(query string) bool {
	s := strings.ToLower(strings.TrimSpace(query))
	if s == "" {
		return false
	}
	if !strings.Contains(s, "html") {
		return false
	}
	if !(strings.Contains(s, "website") || strings.Contains(s, "web site") || strings.Contains(s, "landing page") || strings.Contains(s, "site")) {
		return false
	}
	verbs := []string{"create", "build", "make", "generate", "write"}
	for _, v := range verbs {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

func NewMainModel(application *app.Application, mode app.Mode) *MainModel {
	ta := textarea.New()
	ta.Placeholder = "message... (up/down: history, shift+tab: mode, enter: send)"
	ta.Focus()
	ta.CharLimit = 8000
	ta.SetWidth(60)
	ta.SetHeight(1)
	ta.Prompt = " "
	ta.ShowLineNumbers = false

	// Minimal transparent styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()

	modeIndex := 0
	for i, m := range modes {
		if m == mode {
			modeIndex = i
			break
		}
	}

	verbosity := ""
	if application != nil {
		verbosity = strings.TrimSpace(application.Config.ChatVerbosity)
	}
	timelineEnabled := strings.TrimSpace(os.Getenv("EAI_TUI_TIMELINE")) != "0"

	m := &MainModel{
		app:                application,
		mode:               mode,
		modeIndex:          modeIndex,
		messages:           []Message{},
		title:              "New Chat",
		session:            nil,
		input:              ta,
		inputHistory:       make([]string, 0, 64),
		historyIndex:       -1,
		help:               newHelpModel(),
		width:              100,
		height:             30,
		markdown:           NewMarkdownRenderer(),
		diffRenderer:       NewDiffRenderer(),
		loading:            false,
		loadingText:        "thinking...",
		ready:              false,
		stickToBottom:      true,
		expandedMessageIDs: make(map[string]bool),
		showBanner:         application != nil && application.Config.ShowBanner,
		verbosity:          verbosity,
		timelineEnabled:    timelineEnabled,
	}
	if m.verbosity == "" {
		m.verbosity = "compact"
	}
	if m.app != nil {
		if history, err := m.app.LoadPromptHistory(""); err == nil && len(history) > 0 {
			m.inputHistory = append(m.inputHistory, history...)
		}
	}

	m.messages = append(m.messages, Message{
		ID:        "welcome-1",
		Role:      "system",
		Content:   "eai ready. enter to send, shift+tab to change mode. type /resume to load a previous session.",
		Timestamp: time.Now(),
	})
	m.recomputeInputHeight()

	return m
}

func (m *MainModel) Init() tea.Cmd {
	if m.resumePickerActive {
		return tea.Batch(textarea.Blink, m.resumeTick())
	}
	return textarea.Blink
}

func (m *MainModel) StartResumePicker() {
	m.openResumePicker()
}

func (m *MainModel) sessionWorkDir() string {
	if m.session != nil && strings.TrimSpace(m.session.WorkDir) != "" {
		return m.session.WorkDir
	}
	return ""
}

func (m *MainModel) ensureSession() error {
	if m.session != nil {
		return nil
	}
	if m.app == nil {
		return fmt.Errorf("application unavailable")
	}
	sess, err := m.app.CreateSession("")
	if err != nil {
		return err
	}
	m.session = sess
	if strings.TrimSpace(sess.Title) != "" {
		m.title = strings.TrimSpace(sess.Title)
	} else {
		m.title = "New Chat"
	}
	return nil
}

func (m *MainModel) persistSessionMessage(role, content string) {
	if m.session == nil || m.app == nil {
		return
	}
	_ = m.app.AppendSessionMessage(m.session.ID, role, content, m.mode, m.sessionWorkDir())
}

func (m *MainModel) submitUserRequest(query string, display string) tea.Cmd {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	display = strings.TrimSpace(display)
	if display == "" {
		display = query
	}

	// Any explicit user turn closes a previous plan decision prompt.
	m.planDecisionActive = false
	m.planDecisionChoice = planDecisionYes
	m.pendingPlanText = ""
	m.pendingPlanContext = ""

	// Start a fresh persisted session on first user message (no auto-resume).
	if err := m.ensureSession(); err != nil {
		m.messages = append(m.messages, Message{
			ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
			Role:      "error",
			Content:   fmt.Sprintf("session error: %v", err),
			Timestamp: time.Now(),
		})
		m.updateViewport()
		m.viewport.GotoBottom()
		return nil
	}

	userMsg := Message{
		ID:        fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Role:      "user",
		Content:   display,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, userMsg)
	m.persistSessionMessage("user", query)
	m.loading = true
	m.spinnerPos = 0
	m.loadingText = "thinking..."
	m.turnProgress = nil
	m.turnEvents = nil
	m.turnProgressFrom = time.Now()
	m.updateViewport()
	m.viewport.GotoBottom()
	m.stickToBottom = true
	m.unseenCount = 0

	// Stream step updates while the agent runs.
	m.progressCh = make(chan app.ProgressEvent, 256)
	cmds := []tea.Cmd{
		m.sendMessageWithProgress(query, m.progressCh),
		m.waitForProgress(m.progressCh),
		m.spinTick(),
	}

	// If we don't have a title yet, try to generate one asynchronously.
	if m.session != nil && strings.TrimSpace(m.session.Title) == "" {
		cmds = append(cmds, m.generateTitle())
	}
	return tea.Batch(cmds...)
}

func (m *MainModel) applySession(sess *app.Session, msgs []app.StoredMessage) {
	m.session = sess
	if sess != nil && strings.TrimSpace(sess.Title) != "" {
		m.title = strings.TrimSpace(sess.Title)
	} else {
		m.title = "New Chat"
	}
	m.messages = m.messages[:0]
	for _, sm := range msgs {
		role := strings.ToLower(strings.TrimSpace(sm.Role))
		if role == "" {
			role = "system"
		}
		m.messages = append(m.messages, Message{
			ID:        sm.ID,
			Role:      role,
			Content:   sm.Content,
			Timestamp: sm.CreatedAt,
		})
	}
	if len(m.messages) == 0 {
		resumeLabel := "session resumed."
		if sess != nil {
			if strings.TrimSpace(sess.Title) != "" {
				resumeLabel = "resumed: " + strings.TrimSpace(sess.Title)
			} else {
				resumeLabel = "resumed session " + sess.ID
			}
		}
		m.messages = append(m.messages, Message{
			ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
			Role:      "system",
			Content:   resumeLabel,
			Timestamp: time.Now(),
		})
	}
	m.rebuildInputHistoryFromMessages()
	m.turnProgress = nil
	m.turnProgressFrom = time.Time{}
	m.loading = false
	m.loadingText = "thinking..."
	m.updateViewport()
	m.viewport.GotoBottom()
	m.stickToBottom = true
	m.unseenCount = 0
}

func (m *MainModel) openResumePicker() {
	m.resumePickerActive = true
	m.resumeIndex = 0
	m.resumeItems = nil
	m.resumeLoadErr = ""
	m.resumeLoadedAt = time.Now()
	if m.app == nil {
		m.resumeLoadErr = "application unavailable"
		return
	}
	items, err := m.app.ListRecentSessions("", resumeListMax)
	if err != nil {
		m.resumeLoadErr = err.Error()
		return
	}
	m.resumeItems = items
	if len(m.resumeItems) == 0 {
		m.resumeLoadErr = "no previous sessions in this folder."
	}
}

func (m *MainModel) closeResumePicker() {
	m.resumePickerActive = false
	m.resumeLoadErr = ""
	m.resumeLoadedAt = time.Time{}
	m.input.Focus()
}

func (m *MainModel) resumeSelectedSession() error {
	if m.app == nil {
		return fmt.Errorf("application unavailable")
	}
	if m.resumeIndex < 0 || m.resumeIndex >= len(m.resumeItems) {
		return fmt.Errorf("no session selected")
	}
	selected := m.resumeItems[m.resumeIndex]
	sess, msgs, err := m.app.LoadSession("", selected.Session.ID)
	if err != nil {
		return err
	}
	m.applySession(sess, msgs)
	return nil
}

func (m *MainModel) resumeTick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return resumeTickMsg{}
	})
}

func (m *MainModel) chatAreaWidth() int {
	return m.width - 2
}

func (m *MainModel) statusBarHeight() int {
	h := lipgloss.Height(m.renderStatusBar())
	if h < 1 {
		return 1
	}
	return h
}

func (m *MainModel) inputAreaHeight() int {
	// Activity line + bordered textarea (top and bottom border + content lines).
	return 1 + 2 + m.input.Height()
}

func (m *MainModel) recomputeInputHeight() bool {
	contentWidth := m.chatAreaWidth() - 8
	if contentWidth < 10 {
		contentWidth = 10
	}
	text := m.input.Value()
	lines := strings.Split(text, "\n")
	needed := 0
	for _, line := range lines {
		if line == "" {
			needed++
			continue
		}
		runes := len([]rune(line))
		wrapped := runes / contentWidth
		if runes%contentWidth != 0 {
			wrapped++
		}
		if wrapped < 1 {
			wrapped = 1
		}
		needed += wrapped
	}
	if needed < inputMinLines {
		needed = inputMinLines
	}
	if needed > inputMaxLines {
		needed = inputMaxLines
	}
	if m.input.Height() == needed {
		return false
	}
	m.input.SetHeight(needed)
	return true
}

func (m *MainModel) resetInput() {
	m.historyIndex = -1
	m.historyDraft = ""
	m.input.Reset()
	if m.recomputeInputHeight() {
		m.applyLayout()
	}
}

func (m *MainModel) persistInputHistory() {
	if m.app == nil {
		return
	}
	_ = m.app.SavePromptHistory("", m.inputHistory)
}

func (m *MainModel) appendInputHistoryEntry(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}
	if n := len(m.inputHistory); n > 0 && m.inputHistory[n-1] == input {
		return false
	}
	m.inputHistory = append(m.inputHistory, input)
	const maxHistory = 200
	if len(m.inputHistory) > maxHistory {
		m.inputHistory = m.inputHistory[len(m.inputHistory)-maxHistory:]
	}
	return true
}

func (m *MainModel) pushInputHistory(input string) {
	if m.appendInputHistoryEntry(input) {
		m.persistInputHistory()
	}
	m.historyIndex = -1
	m.historyDraft = ""
}

func (m *MainModel) rebuildInputHistoryFromMessages() {
	changed := false
	for _, msg := range m.messages {
		if msg.Role != "user" {
			continue
		}
		text := strings.TrimSpace(msg.Content)
		if text == "" {
			continue
		}
		if m.appendInputHistoryEntry(text) {
			changed = true
		}
	}
	if changed {
		m.persistInputHistory()
	}
	m.historyIndex = -1
	m.historyDraft = ""
}

func (m *MainModel) setInputValue(value string) {
	m.input.SetValue(value)
	m.input.CursorEnd()
	if m.recomputeInputHeight() {
		wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
		m.applyLayout()
		m.updateViewport()
		if wasAtBottom {
			m.viewport.GotoBottom()
			m.stickToBottom = true
			m.unseenCount = 0
		}
	}
}

func (m *MainModel) browseInputHistory(direction int) bool {
	if len(m.inputHistory) == 0 {
		return false
	}
	if direction < 0 {
		if m.historyIndex == -1 {
			m.historyDraft = m.input.Value()
			m.historyIndex = len(m.inputHistory) - 1
		} else if m.historyIndex > 0 {
			m.historyIndex--
		}
		if m.historyIndex >= 0 && m.historyIndex < len(m.inputHistory) {
			m.setInputValue(m.inputHistory[m.historyIndex])
			return true
		}
		return false
	}
	if direction > 0 {
		if m.historyIndex == -1 {
			return false
		}
		if m.historyIndex < len(m.inputHistory)-1 {
			m.historyIndex++
			m.setInputValue(m.inputHistory[m.historyIndex])
			return true
		}
		draft := m.historyDraft
		m.historyIndex = -1
		m.historyDraft = ""
		m.setInputValue(draft)
		return true
	}
	return false
}

func (m *MainModel) headerHeight() int {
	h := lipgloss.Height(m.renderHeader())
	if h < 0 {
		return 0
	}
	return h
}

func (m *MainModel) chatAreaHeight() int {
	return m.height - m.headerHeight() - m.planDecisionHeight() - m.statusBarHeight() - m.inputAreaHeight()
}

func (m *MainModel) applyLayout() {
	chatWidth := m.chatAreaWidth()
	chatHeight := m.chatAreaHeight()
	if chatHeight < 3 {
		chatHeight = 3
	}
	scrollbarWidth := 1
	vpWidth := chatWidth - scrollbarWidth
	if vpWidth < 10 {
		vpWidth = chatWidth
		scrollbarWidth = 0
	}

	if !m.ready {
		m.viewport = viewport.New(vpWidth, chatHeight)
		m.viewport.Style = lipgloss.NewStyle()
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = chatHeight
	}

	m.input.SetWidth(chatWidth - 4)
	m.help.SetWidth(m.width)
}

func (m *MainModel) setStickinessAfterScroll() {
	if m.viewport.AtBottom() {
		m.stickToBottom = true
		m.unseenCount = 0
	} else {
		m.stickToBottom = false
	}
}

func (m *MainModel) toggleExpandLastAssistant() bool {
	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := m.messages[i]
		if msg.Role != "assistant" {
			continue
		}
		raw := strings.TrimSpace(msg.Content)
		if raw == "" {
			continue
		}
		lines := 1 + strings.Count(raw, "\n")
		if lines <= foldMaxLines && len(raw) <= foldMaxChars {
			continue
		}
		m.expandedMessageIDs[msg.ID] = !m.expandedMessageIDs[msg.ID]
		return true
	}
	return false
}

func (m *MainModel) renderScrollbar(height int) string {
	if height <= 0 {
		return ""
	}

	total := m.viewport.TotalLineCount()
	if total <= height || total <= 0 {
		lines := make([]string, height)
		for i := 0; i < height; i++ {
			lines[i] = " "
		}
		return strings.Join(lines, "\n")
	}

	scrollable := total - height
	if scrollable <= 0 {
		lines := make([]string, height)
		for i := 0; i < height; i++ {
			lines[i] = " "
		}
		return strings.Join(lines, "\n")
	}

	// Thumb size proportional to visible fraction.
	thumbH := int(math.Round(float64(height*height) / float64(total)))
	if thumbH < 1 {
		thumbH = 1
	}
	if thumbH > height {
		thumbH = height
	}

	frac := float64(m.viewport.YOffset) / float64(scrollable)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	thumbTop := int(math.Round(frac * float64(height-thumbH)))
	if thumbTop < 0 {
		thumbTop = 0
	}
	if thumbTop > height-thumbH {
		thumbTop = height - thumbH
	}

	track := lipgloss.NewStyle().Foreground(lipgloss.Color("#44475A")).Render("│")
	thumb := lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9")).Render("█")

	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		if i >= thumbTop && i < thumbTop+thumbH {
			lines = append(lines, thumb)
		} else {
			lines = append(lines, track)
		}
	}
	return strings.Join(lines, "\n")
}

func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		wasReady := m.ready
		m.width = msg.Width
		m.height = msg.Height

		m.recomputeInputHeight()
		m.applyLayout()
		m.updateViewport()
		// Default to most recent messages on first render, but don't yank the
		// viewport if the user has scrolled up.
		if !wasReady || m.stickToBottom {
			m.viewport.GotoBottom()
			m.stickToBottom = true
			m.unseenCount = 0
		}
		return m, nil

	case tea.KeyMsg:
		if m.resumePickerActive {
			switch {
			case key.Matches(msg, m.help.keys.Quit):
				return m, tea.Quit
			case msg.Type == tea.KeyEsc:
				m.closeResumePicker()
				return m, nil
			case msg.String() == "up" || msg.String() == "k":
				if len(m.resumeItems) > 0 {
					m.resumeIndex--
					if m.resumeIndex < 0 {
						m.resumeIndex = len(m.resumeItems) - 1
					}
				}
				return m, m.resumeTick()
			case msg.String() == "down" || msg.String() == "j":
				if len(m.resumeItems) > 0 {
					m.resumeIndex = (m.resumeIndex + 1) % len(m.resumeItems)
				}
				return m, m.resumeTick()
			case msg.Type == tea.KeyEnter:
				if len(m.resumeItems) == 0 {
					m.closeResumePicker()
					return m, nil
				}
				if err := m.resumeSelectedSession(); err != nil {
					m.resumeLoadErr = err.Error()
					return m, m.resumeTick()
				}
				m.closeResumePicker()
				return m, nil
			default:
				return m, m.resumeTick()
			}
		}

		if m.planDecisionActive {
			switch msg.String() {
			case "up", "k":
				m.planDecisionChoice = planDecisionYes
				return m, nil
			case "down", "j":
				m.planDecisionChoice = planDecisionNo
				return m, nil
			}
			switch msg.Type {
			case tea.KeyEsc:
				m.planDecisionActive = false
				m.planDecisionChoice = planDecisionYes
				m.pendingPlanText = ""
				m.pendingPlanContext = ""
				m.applyLayout()
				m.updateViewport()
				if m.stickToBottom || m.viewport.AtBottom() {
					m.viewport.GotoBottom()
				}
				return m, nil
			case tea.KeyEnter:
				choice := m.planDecisionChoice
				planText := strings.TrimSpace(m.pendingPlanText)
				planContext := strings.TrimSpace(m.pendingPlanContext)
				m.planDecisionActive = false
				m.planDecisionChoice = planDecisionYes
				m.pendingPlanText = ""
				m.pendingPlanContext = ""
				m.applyLayout()
				m.updateViewport()
				if m.stickToBottom || m.viewport.AtBottom() {
					m.viewport.GotoBottom()
				}
				if choice == planDecisionNo {
					return m, nil
				}
				m.mode = app.ModeCreate
				for i, md := range modes {
					if md == app.ModeCreate {
						m.modeIndex = i
						break
					}
				}
				return m, m.submitUserRequest(buildPlanImplementationPrompt(planText, planContext), "Implement this approved plan")
			}
		}

		switch {
		case msg.Type == tea.KeyUp || key.Matches(msg, m.input.KeyMap.LinePrevious):
			if m.browseInputHistory(-1) {
				return m, nil
			}
		case msg.Type == tea.KeyDown || key.Matches(msg, m.input.KeyMap.LineNext):
			if m.browseInputHistory(1) {
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, m.help.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.help.keys.NextMode):
			m.modeIndex = (m.modeIndex + 1) % len(modes)
			m.mode = modes[m.modeIndex]
			// Don't add mode change to chat, just update the header
			return m, nil

		case key.Matches(msg, m.help.keys.Enter):
			if m.input.Value() == "" {
				return m, nil
			}

			input := strings.TrimSpace(m.input.Value())
			if strings.HasPrefix(input, "/connect") {
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
					// Reload the client with new config - no restart needed
					newCfg := wizard.GetConfig()
					m.app.ReloadClient(newCfg)
					m.messages = append(m.messages, Message{
						ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
						Role:      "system",
						Content:   "connected.",
						Timestamp: time.Now(),
					})
				}
				m.resetInput()
				m.updateViewport()
				m.viewport.GotoBottom()
				return m, nil
			}
			if strings.HasPrefix(strings.ToLower(input), "/resume") {
				sid := strings.TrimSpace(input[len("/resume"):])
				m.resetInput()
				if sid != "" {
					if sess, msgs, err := m.app.LoadSession("", sid); err == nil {
						m.applySession(sess, msgs)
						return m, nil
					}
					m.messages = append(m.messages, Message{
						ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
						Role:      "error",
						Content:   fmt.Sprintf("resume failed: session %q not found in this folder", sid),
						Timestamp: time.Now(),
					})
					m.updateViewport()
					m.viewport.GotoBottom()
					return m, nil
				}
				m.openResumePicker()
				return m, m.resumeTick()
			}
			m.pushInputHistory(input)
			m.resetInput()
			return m, m.submitUserRequest(input, input)

		case key.Matches(msg, m.help.keys.Clear):
			m.messages = []Message{}
			m.planDecisionActive = false
			m.planDecisionChoice = planDecisionYes
			m.pendingPlanText = ""
			if m.session != nil && m.app.Memory != nil {
				_ = m.app.Memory.ClearSessionMessages(m.session.ID)
				m.session.Title = ""
				_ = m.app.Memory.SaveSession(m.session)
				m.title = "New Chat"
			}
			m.messages = append(m.messages, Message{
				ID:        "welcome-1",
				Role:      "system",
				Content:   "chat cleared.",
				Timestamp: time.Now(),
			})
			m.applyLayout()
			m.updateViewport()
			m.viewport.GotoBottom()
			m.stickToBottom = true
			m.unseenCount = 0
			return m, nil

		case msg.Type == tea.KeyPgUp:
			m.viewport.ViewUp()
			m.setStickinessAfterScroll()
			return m, nil

		case msg.Type == tea.KeyPgDown:
			m.viewport.ViewDown()
			m.setStickinessAfterScroll()
			return m, nil

		case key.Matches(msg, m.help.keys.ToggleBanner):
			wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
			m.showBanner = !m.showBanner
			if m.app != nil {
				m.app.Config.ShowBanner = m.showBanner
				_ = app.SaveConfig(m.app.Config, app.DefaultConfigPath())
			}
			m.applyLayout()
			m.updateViewport()
			if wasAtBottom {
				m.viewport.GotoBottom()
				m.stickToBottom = true
				m.unseenCount = 0
			}
			return m, nil

		case key.Matches(msg, m.help.keys.ToggleVerbosity):
			switch m.verbosity {
			case "compact":
				m.verbosity = "balanced"
			case "balanced":
				m.verbosity = "detailed"
			default:
				m.verbosity = "compact"
			}
			if m.app != nil {
				m.app.Config.ChatVerbosity = m.verbosity
				_ = app.SaveConfig(m.app.Config, app.DefaultConfigPath())
			}
			return m, nil

		case key.Matches(msg, m.help.keys.Expand):
			wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
			if m.toggleExpandLastAssistant() {
				m.updateViewport()
				if wasAtBottom {
					m.viewport.GotoBottom()
				}
			}
			return m, nil
		}

	case aiResponseMsg:
		wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
		m.loading = false
		if msg.err != nil {
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
				Role:      "error",
				Content:   fmt.Sprintf("error: %v", msg.err),
				Timestamp: time.Now(),
			})
		} else {
			assistantContent := msg.response
			isPlan := false
			if display, planText, ok := buildPlanDisplayIfApplicable(m.mode, msg.response); ok {
				assistantContent = display
				isPlan = true
				m.planDecisionActive = true
				m.planDecisionChoice = planDecisionYes
				m.pendingPlanText = planText
				m.pendingPlanContext = buildPlanContextForExecution(m.turnEvents)
				m.applyLayout()
			}
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("ai-%d", time.Now().UnixNano()),
				Role:      "assistant",
				Content:   assistantContent,
				IsPlan:    isPlan,
				Timestamp: time.Now(),
			})
			m.persistSessionMessage("assistant", assistantContent)
		}
		m.turnProgress = nil
		m.turnEvents = nil
		m.turnProgressFrom = time.Time{}
		m.updateViewport()
		if wasAtBottom {
			m.viewport.GotoBottom()
			m.stickToBottom = true
			m.unseenCount = 0
		} else {
			m.unseenCount++
		}
		return m, nil

	case progressUpdateMsg:
		ev := msg.event
		if ev.At.IsZero() {
			ev.At = time.Now()
		}
		// Ignore straggler updates that arrive after the assistant response.
		if !m.loading {
			return m, m.waitForProgress(m.progressCh)
		}
		m.turnEvents = append(m.turnEvents, ev)
		if m.timelineEnabled && strings.TrimSpace(ev.Kind) != "" {
			m.turnProgress = append(m.turnProgress, ev)
		}
		kind := strings.ToLower(strings.TrimSpace(ev.Kind))
		showStatusLine := true
		if m.mode == app.ModePlan && kind == "reasoning" {
			showStatusLine = false
		}
		if showStatusLine {
			if line := strings.TrimSpace(FormatProgressEventForChat(ev)); line != "" {
				duplicate := false
				if n := len(m.messages); n > 0 {
					last := m.messages[n-1]
					duplicate = last.IsStatus && strings.TrimSpace(last.Content) == line
				}
				if !duplicate {
					role := "system"
					if strings.EqualFold(ev.ToolStatus, "error") || strings.EqualFold(ev.Kind, "error") {
						role = "error"
					}
					m.messages = append(m.messages, Message{
						ID:        fmt.Sprintf("status-%d", time.Now().UnixNano()),
						Role:      role,
						Content:   line,
						Timestamp: ev.At,
						IsStatus:  true,
					})
					m.updateViewport()
					if m.stickToBottom || m.viewport.AtBottom() {
						m.viewport.GotoBottom()
						m.stickToBottom = true
						m.unseenCount = 0
					}
				}
			}
		}
		txt := strings.TrimSpace(ev.Text)
		if txt != "" {
			m.loadingText = txt
		}
		return m, m.waitForProgress(m.progressCh)

	case progressDoneMsg:
		m.progressCh = nil
		return m, nil

	case titleMsg:
		if strings.TrimSpace(msg.title) != "" {
			m.title = strings.TrimSpace(msg.title)
			if m.session != nil && m.app.Memory != nil {
				m.session.Title = m.title
				_ = m.app.Memory.SaveSession(m.session)
			}
		}
		return m, nil

	case resumeTickMsg:
		if m.resumePickerActive {
			return m, m.resumeTick()
		}
		return m, nil

	case spinMsg:
		m.spinnerPos = (m.spinnerPos + 1) % len(spinner)
		if m.loading {
			return m, m.spinTick()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	if m.recomputeInputHeight() {
		wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
		m.applyLayout()
		m.updateViewport()
		if wasAtBottom {
			m.viewport.GotoBottom()
			m.stickToBottom = true
			m.unseenCount = 0
		}
	}

	// Only forward mouse events to the viewport so arrow keys don't scroll the
	// chat while the user edits input.
	if _, ok := msg.(tea.MouseMsg); ok {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
		m.setStickinessAfterScroll()
	}

	return m, tea.Batch(cmds...)
}

func (m *MainModel) updateViewport() {
	var b strings.Builder
	chatWidth := m.chatAreaWidth() - 2

	for i, msg := range m.messages {
		var prev *Message
		if i > 0 {
			prev = &m.messages[i-1]
		}
		b.WriteString(m.renderMessage(prev, msg, chatWidth))
		b.WriteString("\n")
	}
	if m.shouldRenderLaunchArt() {
		if art := strings.TrimSpace(m.renderLaunchArt(chatWidth)); art != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			for i := 0; i < m.launchArtTopPadding(); i++ {
				b.WriteString("\n")
			}
			b.WriteString(art)
			b.WriteString("\n")
		}
	}

	// Loading spinner moved to input area, not in chat
	m.viewport.SetContent(b.String())
}

func (m *MainModel) launchArtTopPadding() int {
	h := m.viewport.Height
	if h <= 0 {
		h = m.chatAreaHeight()
	}
	if h <= 10 {
		return 1
	}
	// Push the title down a bit so it feels more centered on first render.
	pad := h / 3
	if pad < 3 {
		pad = 3
	}
	if pad > 12 {
		pad = 12
	}
	return pad
}

func shouldShowMessageHeader(prev *Message, cur Message) bool {
	if prev == nil {
		return true
	}
	// Never show headers for system lines (keep them lightweight).
	if cur.Role == "system" {
		return false
	}
	if cur.Role != prev.Role {
		return true
	}
	// Show header if there's a meaningful time gap.
	if cur.Timestamp.Sub(prev.Timestamp) > 5*time.Minute {
		return true
	}
	return false
}

func (m *MainModel) renderMessage(prev *Message, msg Message, width int) string {
	// Text-only colors, no backgrounds, no boxes
	var textColor lipgloss.Color
	var roleLabel string
	var alignRight bool

	switch msg.Role {
	case "user":
		textColor = lipgloss.Color("#8BE9FD") // Cyan for user
		roleLabel = "you"
		alignRight = true // User messages right
	case "assistant":
		textColor = lipgloss.Color("#50FA7B") // Green for AI
		roleLabel = "eai"
		alignRight = false // AI messages left
	case "system":
		textColor = lipgloss.Color("#6272A4") // Gray for system
		roleLabel = "sys"
		alignRight = false
	case "error":
		textColor = lipgloss.Color("#FF5555") // Red for errors
		roleLabel = "err"
		alignRight = false
	default:
		textColor = lipgloss.Color("#F8F8F2")
		roleLabel = msg.Role
		alignRight = false
	}

	// Handle file edits with diff display
	if msg.IsFileEdit {
		return FormatEditMessage(msg.FilePath, msg.ChangeType, msg.OldContent, msg.NewContent)
	}

	// Live status lines render inline in the chat zone without bubble framing.
	if msg.IsStatus {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			return ""
		}
		maxW := width - 2
		if maxW < 10 {
			maxW = width
		}
		content = wrap.String(content, maxW)
		lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
		if msg.Role == "error" {
			lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))
		}
		return lineStyle.Render(content)
	}

	// System messages: keep compact and centered.
	if msg.Role == "system" {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			return ""
		}
		maxW := width - 4
		if maxW < 10 {
			maxW = width
		}
		content = wrap.String(content, maxW)
		line := lipgloss.NewStyle().Foreground(textColor).Italic(true).Render(content)
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(line)
	}

	if msg.IsPlan {
		maxBubbleW := int(float64(width) * 0.86)
		if maxBubbleW < 32 {
			maxBubbleW = width
		}
		bubbleW := maxBubbleW
		if bubbleW > width {
			bubbleW = width
		}

		bubbleStyle := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#FFB86C")).
			Padding(0, 1).
			Width(bubbleW)
		contentW := bubbleW - bubbleStyle.GetHorizontalFrameSize()
		if contentW < 10 {
			contentW = width
		}

		content := renderPlanContent(msg.Content, contentW)
		if strings.TrimSpace(content) == "" {
			content = strings.TrimSpace(msg.Content)
		}
		content = strings.TrimRight(content, "\n")
		content = wrap.String(content, contentW)
		bubble := bubbleStyle.Render(content)

		align := lipgloss.Left
		if alignRight {
			align = lipgloss.Right
		}
		alignStyle := lipgloss.NewStyle().Width(width).Align(align)

		var out strings.Builder
		if shouldShowMessageHeader(prev, msg) {
			timestamp := msg.Timestamp.Format("15:04")
			headerText := fmt.Sprintf("%s %s", roleLabel, timestamp)
			header := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).Render(headerText)
			out.WriteString(alignStyle.Render(header))
			out.WriteString("\n")
		}
		out.WriteString(alignStyle.Render(bubble))
		return out.String()
	}

	// Bubble sizing: cap width for readability, but fall back to full width on
	// narrow terminals.
	maxBubbleW := int(float64(width) * 0.78)
	if maxBubbleW < 28 {
		maxBubbleW = width
	}
	bubbleW := maxBubbleW
	if bubbleW > width {
		bubbleW = width
	}

	borderColor := lipgloss.Color("#44475A")
	if msg.Role == "user" {
		borderColor = lipgloss.Color("#6272A4")
	}
	if msg.Role == "error" {
		borderColor = lipgloss.Color("#FF5555")
	}

	bubbleStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(bubbleW)

	contentW := bubbleW - bubbleStyle.GetHorizontalFrameSize()
	if contentW < 10 {
		contentW = width
	}

	var content string
	if msg.Role == "assistant" {
		raw := strings.TrimSpace(msg.Content)
		rawLines := 1 + strings.Count(raw, "\n")
		rawFoldable := rawLines > foldMaxLines || len(raw) > foldMaxChars

		content = m.markdown.Render(msg.Content, contentW)
		content = strings.TrimRight(content, "\n")
		// Wrap after markdown so ANSI sequences are handled.
		content = wrap.String(content, contentW)
		if rawFoldable && !m.expandedMessageIDs[msg.ID] {
			lines := strings.Split(content, "\n")
			if len(lines) > foldMaxLines {
				hidden := len(lines) - foldMaxLines
				content = strings.Join(lines[:foldMaxLines], "\n")
				hint := fmt.Sprintf("… (+%d lines, alt+e to expand)", hidden)
				hintStyled := lipgloss.NewStyle().
					Foreground(lipgloss.Color("#6272A4")).
					Italic(true).
					Render(hint)
				content = content + "\n" + hintStyled
			}
		}
	} else {
		content = strings.TrimSpace(msg.Content)
		content = wrap.String(content, contentW)
		content = lipgloss.NewStyle().Foreground(textColor).Render(content)
	}

	bubble := bubbleStyle.Render(content)

	align := lipgloss.Left
	if alignRight {
		align = lipgloss.Right
	}
	alignStyle := lipgloss.NewStyle().Width(width).Align(align)

	var out strings.Builder
	if shouldShowMessageHeader(prev, msg) {
		timestamp := msg.Timestamp.Format("15:04")
		headerText := fmt.Sprintf("%s %s", roleLabel, timestamp)
		header := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).Render(headerText)
		out.WriteString(alignStyle.Render(header))
		out.WriteString("\n")
	}
	out.WriteString(alignStyle.Render(bubble))
	return out.String()
}

func (m *MainModel) View() string {
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Render(fmt.Sprintf("resize to %dx%d (current: %dx%d)", minWidth, minHeight, m.width, m.height))
	}

	if !m.ready {
		return "..."
	}

	if m.resumePickerActive {
		header := m.renderHeader()
		picker := m.renderResumePicker()
		statusBar := m.renderStatusBar()

		parts := make([]string, 0, 3)
		if header != "" {
			parts = append(parts, header)
		}
		parts = append(parts, picker, statusBar)
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}

	// Build minimal layout
	header := m.renderHeader()
	chatHeight := m.chatAreaHeight()
	chatArea := lipgloss.JoinHorizontal(
		lipgloss.Left,
		m.viewport.View(),
		m.renderScrollbar(chatHeight),
	)
	planDecision := m.renderPlanDecision()
	statusBar := m.renderStatusBar()
	inputArea := m.renderInputArea()

	parts := make([]string, 0, 5)
	if header != "" {
		parts = append(parts, header)
	}
	parts = append(parts, chatArea)
	if planDecision != "" {
		parts = append(parts, planDecision)
	}
	parts = append(parts, statusBar, inputArea)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *MainModel) renderHeader() string {
	if !m.showBanner {
		return ""
	}

	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#44475A")).
		Render(strings.Repeat("─", m.width))
	return lipgloss.JoinVertical(lipgloss.Left, GetBanner(m.width), separator)
}

func (m *MainModel) renderStatusBar() string {
	scrollInfo := ""
	if m.viewport.TotalLineCount() > m.viewport.Height {
		scrollInfo = fmt.Sprintf("%d%% ", int(m.viewport.ScrollPercent()*100))
	}

	newInfo := ""
	if !m.stickToBottom && m.unseenCount > 0 {
		newInfo = fmt.Sprintf("new:%d ", m.unseenCount)
	}
	rightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	rightText := newInfo + scrollInfo + time.Now().Format("15:04")
	right := rightStyle.Render(rightText)

	brandStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB86C")).
		Bold(true)
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F8F8F2")).
		Bold(true)
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4"))

	sep := metaStyle.Render(" · ")
	brand := brandStyle.Render("eai")
	mode := metaStyle.Render(string(m.mode))
	verb := metaStyle.Render(m.verbosity)

	title := strings.TrimSpace(m.title)
	if title == "" {
		title = "New Chat"
	}

	// Truncate the title so the status bar never overflows the right side.
	maxLeft := m.width - lipgloss.Width(right)
	if maxLeft < 0 {
		maxLeft = 0
	}
	fixedW := lipgloss.Width(brand) + lipgloss.Width(sep)*3 + lipgloss.Width(mode) + lipgloss.Width(verb)
	titleW := maxLeft - fixedW
	if titleW < 4 {
		title = ""
	} else {
		title = truncate.StringWithTail(title, uint(titleW), "…")
	}

	left := brand
	if title != "" {
		left += sep + titleStyle.Render(title)
	}
	left += sep + mode + sep + verb

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	middle := strings.Repeat(" ", gap)
	line1 := left + middle + right

	model := ""
	if m.app != nil {
		model = strings.TrimSpace(m.app.Config.Model)
		if model == "" && m.app.Client != nil {
			model = strings.TrimSpace(m.app.Client.Model)
		}
	}
	if model == "" {
		return line1
	}

	line2Text := "model: " + model
	if m.width > 0 {
		line2Text = truncate.StringWithTail(line2Text, uint(m.width), "…")
	}
	line2 := rightStyle.Render(line2Text)
	if !m.loading {
		return line1 + "\n" + line2
	}

	loadingText := strings.TrimSpace(m.loadingText)
	if loadingText == "" {
		loadingText = "thinking..."
	}
	cubes := m.renderLoadingCubes(10)
	cubeWidth := lipgloss.Width(cubes)
	maxText := m.width - cubeWidth - 2
	if maxText < 8 {
		maxText = 8
	}
	loadingText = truncate.StringWithTail(loadingText, uint(maxText), "…")
	loadingLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#BD93F9")).
		Render(loadingText + "  " + cubes)
	return line1 + "\n" + line2 + "\n" + loadingLine
}

func (m *MainModel) renderLoadingCubes(count int) string {
	if count <= 0 {
		count = 10
	}
	if count == 1 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9")).Render("■")
	}

	active := m.spinnerPos % count
	steps := []string{
		"#BD93F9",
		"#A884EC",
		"#8C73D9",
		"#6D5FB6",
		"#4C496E",
	}

	var b strings.Builder
	for i := 0; i < count; i++ {
		dist := i - active
		if dist < 0 {
			dist = -dist
		}
		if dist > count-dist {
			dist = count - dist
		}
		colorIndex := len(steps) - 1
		if dist < len(steps) {
			colorIndex = dist
		}
		cube := lipgloss.NewStyle().Foreground(lipgloss.Color(steps[colorIndex])).Render("■")
		b.WriteString(cube)
		if i < count-1 {
			b.WriteString(" ")
		}
	}
	return b.String()
}

func (m *MainModel) renderInputArea() string {
	var result strings.Builder

	// Reserve one spacer line above input for stable layout; live thinking and
	// reasoning appear in the chat area as status lines.
	result.WriteString("\n")

	// Input box with thin border
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#44475A")).
		Padding(0, 1).
		Width(m.chatAreaWidth() - 2).
		Render(m.input.View())

	result.WriteString(inputBox)

	return result.String()
}

func (m *MainModel) renderResumePicker() string {
	maxWidth := m.chatAreaWidth() - 4
	if maxWidth < 30 {
		maxWidth = m.chatAreaWidth()
	}
	totalHeight := m.height - m.headerHeight() - m.statusBarHeight()
	if totalHeight < 8 {
		totalHeight = 8
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB86C"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	selectStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555"))

	var b strings.Builder
	b.WriteString(titleStyle.Render("Resume Session"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/↓ select  •  enter resume  •  esc cancel"))
	b.WriteString("\n\n")

	if strings.TrimSpace(m.resumeLoadErr) != "" {
		b.WriteString(errorStyle.Render(m.resumeLoadErr))
		b.WriteString("\n")
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#44475A")).
			Padding(1, 1).
			Width(m.chatAreaWidth()).
			Height(totalHeight).
			Render(b.String())
	}

	now := time.Now()
	itemBlockLines := 2
	maxVisible := (totalHeight - 4) / itemBlockLines
	if maxVisible < 1 {
		maxVisible = 1
	}
	start := 0
	if len(m.resumeItems) > maxVisible && m.resumeIndex >= maxVisible {
		start = m.resumeIndex - maxVisible + 1
	}
	end := len(m.resumeItems)
	if end > start+maxVisible {
		end = start + maxVisible
	}

	for i := start; i < end; i++ {
		item := m.resumeItems[i]
		selected := i == m.resumeIndex
		prefix := "  "
		titleRenderer := rowStyle
		if selected {
			prefix = "› "
			titleRenderer = selectStyle
		}

		title := strings.TrimSpace(item.Session.Title)
		if title == "" {
			title = "Untitled Session"
		}
		title = truncate.StringWithTail(title, uint(maxWidth-4), "…")

		workDuration := item.WorkDuration
		if !m.resumeLoadedAt.IsZero() && !item.LastActivity.IsZero() {
			// Keep work duration "live" while picker is open.
			if m.resumeLoadedAt.After(item.LastActivity) && m.resumeLoadedAt.Sub(item.LastActivity) <= 15*time.Minute {
				workDuration += now.Sub(m.resumeLoadedAt)
			}
		}

		lastSeen := "unknown"
		if !item.LastActivity.IsZero() {
			lastSeen = formatRelativeTime(now, item.LastActivity)
		}

		meta := fmt.Sprintf("%s  •  worked %s  •  %d msgs  •  %s",
			shortSessionID(item.Session.ID),
			formatWorkDuration(workDuration),
			item.MessageCount,
			lastSeen,
		)
		meta = truncate.StringWithTail(meta, uint(maxWidth-4), "…")

		b.WriteString(titleRenderer.Render(prefix + title))
		b.WriteString("\n")
		b.WriteString(metaStyle.Render("  " + meta))
		b.WriteString("\n")
	}

	if len(m.resumeItems) > end {
		b.WriteString(hintStyle.Render(fmt.Sprintf("… %d more sessions", len(m.resumeItems)-end)))
		b.WriteString("\n")
	}

	content := strings.TrimRight(b.String(), "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#44475A")).
		Padding(1, 1).
		Width(m.chatAreaWidth()).
		Height(totalHeight).
		Render(content)
}

func shortSessionID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 10 {
		return id
	}
	return id[:10]
}

func formatWorkDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	if d < time.Minute {
		return "<1m"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func formatRelativeTime(now time.Time, t time.Time) string {
	if t.IsZero() {
		return "inactive"
	}
	if now.Before(t) {
		return "just now"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("2006-01-02 15:04")
	}
}

func (m *MainModel) spinTick() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(_ time.Time) tea.Msg {
		return spinMsg{}
	})
}

func (m *MainModel) sendMessageWithProgress(query string, progressCh chan<- app.ProgressEvent) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		cb := func(app.ProgressEvent) {}
		if progressCh != nil {
			cb = func(ev app.ProgressEvent) {
				progressCh <- ev
			}
		}

		sid := ""
		if m.session != nil {
			sid = m.session.ID
		}
		response, err := m.app.ExecuteChatInSessionWithProgressEvents(ctx, sid, m.mode, query, cb)
		if progressCh != nil {
			close(progressCh)
		}
		if err != nil {
			return aiResponseMsg{err: err}
		}
		return aiResponseMsg{response: response}
	}
}

func (m *MainModel) waitForProgress(ch <-chan app.ProgressEvent) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return progressDoneMsg{}
		}
		return progressUpdateMsg{event: ev}
	}
}

func (m *MainModel) generateTitle() tea.Cmd {
	return func() tea.Msg {
		if m.session == nil || m.app == nil || m.app.Memory == nil {
			return titleMsg{}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		msgs, err := m.app.Memory.LoadMessages(m.session.ID)
		if err != nil {
			return titleMsg{}
		}
		title, _ := m.app.GenerateChatTitle(ctx, msgs)
		return titleMsg{title: title}
	}
}

// AddFileEdit adds a file edit message with diff
func (m *MainModel) AddFileEdit(filePath, changeType, oldContent, newContent string) {
	m.messages = append(m.messages, Message{
		ID:         fmt.Sprintf("edit-%d", time.Now().UnixNano()),
		Role:       "system",
		IsFileEdit: true,
		FilePath:   filePath,
		ChangeType: changeType,
		OldContent: oldContent,
		NewContent: newContent,
		Timestamp:  time.Now(),
	})
	m.updateViewport()
	m.viewport.GotoBottom()
}

// Color constants - minimal palette
const (
	colorFg      = "#F8F8F2"
	colorFgMuted = "#6272A4"
	colorAccent  = "#FFB86C"
	colorUser    = "#8BE9FD"
	colorAI      = "#50FA7B"
	colorError   = "#FF5555"
	colorBorder  = "#44475A"
)
