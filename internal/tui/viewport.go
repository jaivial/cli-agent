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
	foldMaxChars  = 32768
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
	loadingSince time.Time
	loadingEnded time.Time
	startupUntil time.Time
	animEnabled  bool
	modeIndex    int
	ready        bool

	progressCh   chan app.ProgressEvent
	activeCancel context.CancelFunc
	cancelQueued bool

	queuedRequests   []queuedRequest
	turnResponseDone bool
	turnProgressDone bool

	traceMsgIndex int

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

	modelPickerActive bool
	modelPickerIndex  int
	modelOptions      []string

	slashPopupIndex int
	slashPopupKey   string

	permissionsPickerActive bool
	permissionsPickerIndex  int
	permissionsOptions      []string
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

type queuedRequest struct {
	query    string
	display  string
	queuedAt time.Time
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
var modes = []app.Mode{app.ModeDo}

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
	// The TUI always runs the tool-driven agent loop (like `eai agent`).
	mode = app.ModeDo

	ta := textarea.New()
	ta.Placeholder = "message... (/ commands, esc cancel, enter send)"
	ta.Focus()
	ta.CharLimit = 8000
	ta.SetWidth(60)
	ta.SetHeight(1)
	ta.Prompt = " "
	ta.ShowLineNumbers = false

	// Minimal transparent styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
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
		startupUntil:       time.Now().Add(time.Millisecond * defaultStartupShimmer),
		animEnabled:        animationsEnabled(),
		stickToBottom:      true,
		expandedMessageIDs: make(map[string]bool),
		showBanner:         application != nil && application.Config.ShowBanner,
		verbosity:          verbosity,
		timelineEnabled:    timelineEnabled,
		modelOptions:       append([]string(nil), app.SupportedModels...),
		permissionsOptions: []string{app.PermissionsFullAccess, app.PermissionsDangerouslyFullAccess},
		traceMsgIndex:      -1,
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
		Content:   "eai ready. enter sends (queues while running). esc cancels. type / for commands.",
		Timestamp: time.Now(),
	})
	m.recomputeInputHeight()

	return m
}

func (m *MainModel) Init() tea.Cmd {
	if m.resumePickerActive {
		return tea.Batch(textarea.Blink, m.resumeTick())
	}
	if m.animEnabled {
		return tea.Batch(textarea.Blink, m.spinTick())
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

func (m *MainModel) maybeStartNextQueued() tea.Cmd {
	if m.loading || !m.turnResponseDone || !m.turnProgressDone || m.planDecisionActive {
		return nil
	}
	if len(m.queuedRequests) == 0 {
		return nil
	}
	next := m.queuedRequests[0]
	m.queuedRequests = m.queuedRequests[1:]
	return m.submitUserRequest(next.query, next.display)
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
	m.loadingSince = time.Now()
	m.loadingEnded = time.Time{}
	m.turnProgress = nil
	m.turnEvents = nil
	m.turnProgressFrom = time.Now()
	m.turnResponseDone = false
	m.turnProgressDone = false
	m.cancelQueued = false
	m.traceMsgIndex = -1
	if m.activeCancel != nil {
		// Defensive: ensure any previous cancel func doesn't linger.
		m.activeCancel()
		m.activeCancel = nil
	}

	m.startTurnTrace()
	m.updateTurnTrace(true)

	m.applyLayout()
	m.updateViewport()
	m.viewport.GotoBottom()
	m.stickToBottom = true
	m.unseenCount = 0

	// Stream step updates while the agent runs.
	m.progressCh = make(chan app.ProgressEvent, 256)
	ctx, cancel := context.WithCancel(context.Background())
	m.activeCancel = cancel
	cmds := []tea.Cmd{
		m.sendMessageWithProgress(ctx, query, m.progressCh),
		m.waitForProgress(m.progressCh),
	}
	if m.animEnabled {
		cmds = append(cmds, m.spinTick())
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
	m.loadingSince = time.Time{}
	m.loadingEnded = time.Time{}
	m.traceMsgIndex = -1
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

func (m *MainModel) openModelPicker() {
	m.modelPickerActive = true
	if len(m.modelOptions) == 0 {
		m.modelOptions = append([]string(nil), app.SupportedModels...)
	}

	current := app.DefaultModel
	if m.app != nil {
		if model := strings.TrimSpace(m.app.Config.Model); model != "" {
			current = model
		} else if m.app.Client != nil && strings.TrimSpace(m.app.Client.Model) != "" {
			current = m.app.Client.Model
		}
	}
	current = app.NormalizeModel(current)

	m.modelPickerIndex = 0
	for i, option := range m.modelOptions {
		if strings.EqualFold(strings.TrimSpace(option), current) {
			m.modelPickerIndex = i
			break
		}
	}
}

func (m *MainModel) closeModelPicker() {
	m.modelPickerActive = false
	m.input.Focus()
}

func (m *MainModel) selectModelAt(index int) string {
	if index < 0 || index >= len(m.modelOptions) {
		return ""
	}
	model := app.NormalizeModel(m.modelOptions[index])
	if m.app == nil {
		return model
	}
	cfg := m.app.Config
	cfg.Model = model
	cfg.BaseURL = app.NormalizeBaseURL(cfg.BaseURL)
	if err := app.SaveConfig(cfg, app.DefaultConfigPath()); err != nil {
		m.messages = append(m.messages, Message{
			ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
			Role:      "error",
			Content:   fmt.Sprintf("failed to save model: %v", err),
			Timestamp: time.Now(),
		})
		return ""
	}
	m.app.ReloadClient(cfg)
	return model
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
	height := 1 + 2 + m.input.Height()
	if m.modelPickerActive {
		if pickerHeight := lipgloss.Height(m.renderModelPicker()); pickerHeight > 0 {
			height += pickerHeight + 1
		}
	}
	if popupHeight := lipgloss.Height(m.renderSlashPopup()); popupHeight > 0 {
		height += popupHeight + 1
	}
	if pickerHeight := lipgloss.Height(m.renderPermissionsPicker()); pickerHeight > 0 {
		height += pickerHeight + 1
	}
	return height
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
	m.updateSlashPopupState()
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
	m.updateSlashPopupState()
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

	track := lipgloss.NewStyle().Foreground(lipgloss.Color(colorBorderSoft)).Render("│")
	thumb := lipgloss.NewStyle().Foreground(lipgloss.Color(colorScrollbar)).Render("█")

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
		if m.loading {
			m.updateTurnTrace(true)
		}
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

		if m.modelPickerActive {
			switch {
			case key.Matches(msg, m.help.keys.Quit):
				return m, tea.Quit
			}

			switch msg.String() {
			case "up", "k":
				if len(m.modelOptions) > 0 {
					m.modelPickerIndex--
					if m.modelPickerIndex < 0 {
						m.modelPickerIndex = len(m.modelOptions) - 1
					}
				}
				return m, nil
			case "down", "j":
				if len(m.modelOptions) > 0 {
					m.modelPickerIndex = (m.modelPickerIndex + 1) % len(m.modelOptions)
				}
				return m, nil
			}

			switch msg.Type {
			case tea.KeyEsc:
				wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
				m.closeModelPicker()
				m.applyLayout()
				m.updateViewport()
				if wasAtBottom {
					m.viewport.GotoBottom()
				}
				return m, nil
			case tea.KeyEnter:
				wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
				selected := m.selectModelAt(m.modelPickerIndex)
				m.closeModelPicker()
				if selected != "" {
					m.messages = append(m.messages, Message{
						ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
						Role:      "system",
						Content:   "model set to " + selected,
						Timestamp: time.Now(),
					})
				}
				m.applyLayout()
				m.updateViewport()
				if wasAtBottom {
					m.viewport.GotoBottom()
				}
				return m, nil
			default:
				return m, nil
			}
		}

		if m.permissionsPickerActive {
			switch {
			case key.Matches(msg, m.help.keys.Quit):
				return m, tea.Quit
			}

			switch msg.String() {
			case "up", "k":
				if len(m.permissionsOptions) > 0 {
					m.permissionsPickerIndex--
					if m.permissionsPickerIndex < 0 {
						m.permissionsPickerIndex = len(m.permissionsOptions) - 1
					}
				}
				return m, nil
			case "down", "j":
				if len(m.permissionsOptions) > 0 {
					m.permissionsPickerIndex = (m.permissionsPickerIndex + 1) % len(m.permissionsOptions)
				}
				return m, nil
			}

			switch msg.Type {
			case tea.KeyEsc:
				wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
				m.closePermissionsPicker()
				m.applyLayout()
				m.updateViewport()
				if wasAtBottom {
					m.viewport.GotoBottom()
				}
				return m, nil
			case tea.KeyEnter:
				wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
				selected := m.selectPermissionsAt(m.permissionsPickerIndex)
				m.closePermissionsPicker()
				if selected != "" {
					m.messages = append(m.messages, Message{
						ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
						Role:      "system",
						Content:   "permissions set to " + selected,
						Timestamp: time.Now(),
					})
				}
				m.applyLayout()
				m.updateViewport()
				if wasAtBottom {
					m.viewport.GotoBottom()
				}
				return m, nil
			default:
				return m, nil
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
				if cmd := m.maybeStartNextQueued(); cmd != nil {
					return m, cmd
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
					if cmd := m.maybeStartNextQueued(); cmd != nil {
						return m, cmd
					}
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

		if msg.Type == tea.KeyEsc && m.loading {
			if m.activeCancel != nil && !m.cancelQueued {
				m.cancelQueued = true
				m.loadingText = "canceling..."
				m.activeCancel()
				m.messages = append(m.messages, Message{
					ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
					Role:      "system",
					Content:   "cancel requested (esc).",
					Timestamp: time.Now(),
				})
				m.updateViewport()
				if m.stickToBottom || m.viewport.AtBottom() {
					m.viewport.GotoBottom()
					m.stickToBottom = true
					m.unseenCount = 0
				}
			}
			return m, nil
		}

		if items := m.slashPopupItems(); len(items) > 0 {
			switch msg.String() {
			case "up":
				m.slashPopupIndex--
				if m.slashPopupIndex < 0 {
					m.slashPopupIndex = len(items) - 1
				}
				return m, nil
			case "down":
				m.slashPopupIndex = (m.slashPopupIndex + 1) % len(items)
				return m, nil
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

		case key.Matches(msg, m.help.keys.Enter):
			if m.input.Value() == "" {
				return m, nil
			}

			input := strings.TrimSpace(m.input.Value())
			if items := m.slashPopupItems(); len(items) > 0 {
				idx := m.slashPopupIndex
				if idx < 0 || idx >= len(items) {
					idx = 0
				}
				insert := strings.TrimSpace(items[idx].InsertText)
				if insert != "" && !strings.EqualFold(strings.TrimSpace(input), insert) {
					m.setInputValue(insert)
					return m, nil
				}
			}

			if strings.EqualFold(strings.TrimSpace(input), "/permissions") {
				m.resetInput()
				m.openPermissionsPicker()
				m.applyLayout()
				m.updateViewport()
				if m.stickToBottom || m.viewport.AtBottom() {
					m.viewport.GotoBottom()
				}
				return m, nil
			}

			if handled, content, role := m.handlePermissionsCommand(input); handled {
				m.messages = append(m.messages, Message{
					ID:        fmt.Sprintf("%s-%d", role, time.Now().UnixNano()),
					Role:      role,
					Content:   content,
					Timestamp: time.Now(),
				})
				m.resetInput()
				m.updateViewport()
				m.viewport.GotoBottom()
				return m, nil
			}
			if strings.HasPrefix(input, "/connect") {
				if m.loading {
					m.messages = append(m.messages, Message{
						ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
						Role:      "system",
						Content:   "command unavailable while agent is running. press esc to cancel first.",
						Timestamp: time.Now(),
					})
					m.resetInput()
					m.updateViewport()
					if m.stickToBottom || m.viewport.AtBottom() {
						m.viewport.GotoBottom()
						m.stickToBottom = true
						m.unseenCount = 0
					}
					return m, nil
				}
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
						Content:   wizard.Summary() + " (applied without restart).",
						Timestamp: time.Now(),
					})
				}
				m.resetInput()
				m.updateViewport()
				m.viewport.GotoBottom()
				return m, nil
			}
			if strings.HasPrefix(strings.ToLower(input), "/model") {
				m.resetInput()
				m.openModelPicker()
				m.applyLayout()
				m.updateViewport()
				if m.stickToBottom || m.viewport.AtBottom() {
					m.viewport.GotoBottom()
				}
				return m, nil
			}
			if strings.HasPrefix(strings.ToLower(input), "/resume") {
				if m.loading {
					m.messages = append(m.messages, Message{
						ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
						Role:      "system",
						Content:   "command unavailable while agent is running. press esc to cancel first.",
						Timestamp: time.Now(),
					})
					m.resetInput()
					m.updateViewport()
					if m.stickToBottom || m.viewport.AtBottom() {
						m.viewport.GotoBottom()
						m.stickToBottom = true
						m.unseenCount = 0
					}
					return m, nil
				}
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

			if m.loading {
				// Avoid starting concurrent agent runs; queue user turns instead.
				m.pushInputHistory(input)
				m.queuedRequests = append(m.queuedRequests, queuedRequest{
					query:    input,
					display:  input,
					queuedAt: time.Now(),
				})
				preview := strings.TrimSpace(input)
				if i := strings.IndexByte(preview, '\n'); i >= 0 {
					preview = preview[:i]
				}
				if len(preview) > 80 {
					preview = preview[:77] + "..."
				}
				m.messages = append(m.messages, Message{
					ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
					Role:      "system",
					Content:   fmt.Sprintf("queued (%d): %s", len(m.queuedRequests), preview),
					Timestamp: time.Now(),
				})
				m.resetInput()
				m.updateViewport()
				if m.stickToBottom || m.viewport.AtBottom() {
					m.viewport.GotoBottom()
					m.stickToBottom = true
					m.unseenCount = 0
				}
				return m, nil
			}

			m.pushInputHistory(input)
			m.resetInput()
			return m, m.submitUserRequest(input, input)

		case key.Matches(msg, m.help.keys.Clear):
			m.messages = []Message{}
			m.planDecisionActive = false
			m.planDecisionChoice = planDecisionYes
			m.pendingPlanText = ""
			m.traceMsgIndex = -1
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
		canceled := m.cancelQueued
		m.loading = false
		m.loadingEnded = time.Now()
		m.turnResponseDone = true
		if m.activeCancel != nil {
			m.activeCancel()
			m.activeCancel = nil
		}
		if canceled && msg.err == nil {
			// The user requested cancellation; discard any late-arriving successful response.
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
				Role:      "system",
				Content:   "canceled.",
				Timestamp: time.Now(),
			})
		} else if msg.err != nil {
			errLower := strings.ToLower(msg.err.Error())
			if strings.Contains(errLower, "context canceled") || strings.Contains(errLower, "operation was canceled") {
				m.messages = append(m.messages, Message{
					ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
					Role:      "system",
					Content:   "canceled.",
					Timestamp: time.Now(),
				})
			} else {
				m.messages = append(m.messages, Message{
					ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
					Role:      "error",
					Content:   fmt.Sprintf("error: %v", msg.err),
					Timestamp: time.Now(),
				})
			}
		} else {
			assistantContent := msg.response
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("ai-%d", time.Now().UnixNano()),
				Role:      "assistant",
				Content:   assistantContent,
				Timestamp: time.Now(),
			})
			m.persistSessionMessage("assistant", assistantContent)
		}
		m.updateTurnTrace(false)
		m.turnProgress = nil
		m.turnEvents = nil
		m.turnProgressFrom = time.Time{}
		m.applyLayout()
		m.updateViewport()
		if wasAtBottom {
			m.viewport.GotoBottom()
			m.stickToBottom = true
			m.unseenCount = 0
		} else {
			m.unseenCount++
		}
		if cmd := m.maybeStartNextQueued(); cmd != nil {
			return m, cmd
		}
		return m, nil

	case progressUpdateMsg:
		ev := msg.event
		if ev.At.IsZero() {
			ev.At = time.Now()
		}
		m.turnEvents = append(m.turnEvents, ev)
		kind := strings.ToLower(strings.TrimSpace(ev.Kind))
		if kind == "file_edit" {
			path := strings.TrimSpace(ev.Path)
			if path == "" {
				path = "(unknown path)"
			}
			changeType := strings.ToLower(strings.TrimSpace(ev.ChangeType))
			if changeType == "" {
				changeType = "modify"
			}
			m.AddFileEdit(path, changeType, ev.OldContent, ev.NewContent)
		}
		if m.timelineEnabled && kind != "" && kind != "tool_output" {
			m.turnProgress = append(m.turnProgress, ev)
		}
		if kind != "tool_output" {
			txt := strings.TrimSpace(ev.Text)
			if txt != "" {
				m.loadingText = txt
			}
		}
		wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
		m.updateTurnTrace(true)
		m.updateViewport()
		if wasAtBottom {
			m.viewport.GotoBottom()
			m.stickToBottom = true
			m.unseenCount = 0
		}
		return m, m.waitForProgress(m.progressCh)

	case progressDoneMsg:
		m.progressCh = nil
		m.turnProgressDone = true
		if cmd := m.maybeStartNextQueued(); cmd != nil {
			return m, cmd
		}
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
		m.spinnerPos++
		if m.spinnerPos > 1_000_000 {
			m.spinnerPos = 0
		}
		if m.animEnabled && m.loading {
			wasAtBottom := m.stickToBottom || m.viewport.AtBottom()
			m.updateTurnTrace(true)
			m.updateViewport()
			if wasAtBottom {
				m.viewport.GotoBottom()
				m.stickToBottom = true
				m.unseenCount = 0
			}
		}
		if m.animEnabled && m.shouldKeepAnimating() {
			return m, m.spinTick()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.updateSlashPopupState()
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

	var prev *Message
	for i := range m.messages {
		msg := m.messages[i]
		rendered := strings.TrimRight(m.renderMessage(prev, msg, chatWidth), "\n")
		if strings.TrimSpace(rendered) == "" {
			continue
		}
		b.WriteString(rendered)
		b.WriteString("\n")
		prev = &m.messages[i]
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
	var roleColor string

	switch msg.Role {
	case "user":
		textColor = lipgloss.Color(colorFg)
		roleLabel = "you"
		roleColor = colorAccent
		alignRight = true // User messages right
	case "assistant":
		textColor = lipgloss.Color(colorFg)
		roleLabel = "eai"
		roleColor = colorAccent2
		alignRight = false // AI messages left
	case "system":
		textColor = lipgloss.Color(colorMuted)
		roleLabel = "sys"
		roleColor = colorMuted
		alignRight = false
	case "error":
		textColor = lipgloss.Color(colorError)
		roleLabel = "err"
		roleColor = colorError
		alignRight = false
	default:
		textColor = lipgloss.Color(colorFg)
		roleLabel = msg.Role
		roleColor = colorMuted
		alignRight = false
	}

	// Handle file edits with diff display
	if msg.IsFileEdit {
		return FormatEditMessage(msg.FilePath, msg.ChangeType, msg.OldContent, msg.NewContent)
	}

	// Live status lines render inline in the chat zone without bubble framing.
	if msg.IsStatus {
		content := strings.TrimRight(msg.Content, "\n")
		content = strings.TrimSpace(content)
		if content == "" {
			return ""
		}
		return content
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
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorAccent)).
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
			roleStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(roleColor)).Bold(true).Render(roleLabel)
			timeStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(timestamp)
			header := roleStyled + " " + timeStyled
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

	borderColor := lipgloss.Color(colorBorder)
	if msg.Role == "user" {
		borderColor = lipgloss.Color(colorAccent)
	}
	if msg.Role == "error" {
		borderColor = lipgloss.Color(colorError)
	}
	if msg.Role == "assistant" {
		borderColor = lipgloss.Color(colorBorder)
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
					Foreground(lipgloss.Color(colorMuted)).
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
		roleStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(roleColor)).Bold(true).Render(roleLabel)
		timeStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(timestamp)
		header := roleStyled + " " + timeStyled
		out.WriteString(alignStyle.Render(header))
		out.WriteString("\n")
	}
	out.WriteString(alignStyle.Render(bubble))
	return out.String()
}

func (m *MainModel) View() string {
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError)).
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
		Foreground(lipgloss.Color(colorBorderSoft)).
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
	queueInfo := ""
	if len(m.queuedRequests) > 0 {
		queueInfo = fmt.Sprintf("q:%d ", len(m.queuedRequests))
	}
	cancelInfo := ""
	if m.loading && m.cancelQueued {
		cancelInfo = "canceling "
	}
	rightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	rightText := newInfo + queueInfo + scrollInfo + cancelInfo + time.Now().Format("15:04")
	right := rightStyle.Render(rightText)

	brandStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorAccent)).
		Bold(true)
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorFg)).
		Bold(true)
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorMuted))

	sep := metaStyle.Render(" · ")
	brand := brandStyle.Render("eai")
	mode := metaStyle.Render("agent")
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
	return line1 + "\n" + line2
}

func (m *MainModel) renderLoadingDots(alpha float64) string {
	alpha = clamp01(alpha)
	active := 0
	if m.spinnerPos >= 0 {
		frames := 2000 / defaultAnimTick
		if frames < 3 {
			frames = 3
		}
		step := frames / 3
		if step < 1 {
			step = 1
		}
		active = (m.spinnerPos / step) % 3
	}

	base := blendHex(colorSubtle, colorMuted, alpha)
	hi := blendHex(colorSubtle, colorAccent, alpha)
	mid := blendHex(base, hi, 0.45)

	colors := [3]string{base, base, base}
	colors[active] = hi
	colors[(active+1)%3] = mid

	var b strings.Builder
	for i := 0; i < 3; i++ {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colors[i])).Render("●"))
	}
	return b.String()
}

func (m *MainModel) renderInputThinkingLine(width int) string {
	if !m.loading {
		return ""
	}
	if width < 20 {
		width = 20
	}

	label := shimmerText("Thinking", m.spinnerPos, colorMuted, colorAccent2)
	label = lipgloss.NewStyle().Italic(true).Render(label)
	dots := m.renderLoadingDots(1)

	gap := width - lipgloss.Width(label) - lipgloss.Width(dots)
	if gap < 1 {
		gap = 1
	}
	line := label + strings.Repeat(" ", gap) + dots
	return line
}

func (m *MainModel) renderModelPicker() string {
	if !m.modelPickerActive || len(m.modelOptions) == 0 {
		return ""
	}

	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent)).Render("select model")
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render("up/down to navigate, enter to select, esc to cancel")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(hint)
	b.WriteString("\n")

	for i, model := range m.modelOptions {
		line := "  " + model
		if i == m.modelPickerIndex {
			line = "> " + model
			line = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent2)).Bold(true).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color(colorFg)).Render(line)
		}
		b.WriteString(line)
		if i < len(m.modelOptions)-1 {
			b.WriteString("\n")
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(0, 1).
		Width(m.chatAreaWidth() - 2).
		Render(b.String())
}

func (m *MainModel) renderInputArea() string {
	var result strings.Builder

	if picker := m.renderModelPicker(); picker != "" {
		result.WriteString(picker)
		result.WriteString("\n")
	}

	if popup := m.renderSlashPopup(); popup != "" {
		result.WriteString(popup)
		result.WriteString("\n")
	}

	if picker := m.renderPermissionsPicker(); picker != "" {
		result.WriteString(picker)
		result.WriteString("\n")
	}

	// Activity line (always reserved). While running, show an animated thinking
	// indicator just above the input box.
	activityW := m.chatAreaWidth() - 2
	if activityW < 10 {
		activityW = m.chatAreaWidth()
	}
	result.WriteString(m.renderInputThinkingLine(activityW))
	result.WriteString("\n")

	// Input box with thin border
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
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

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	selectStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent2)).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFg))
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorError))

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
			BorderForeground(lipgloss.Color(colorBorder)).
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
		BorderForeground(lipgloss.Color(colorBorder)).
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
	return tea.Tick(time.Millisecond*time.Duration(defaultAnimTick), func(_ time.Time) tea.Msg {
		return spinMsg{}
	})
}

func (m *MainModel) shouldKeepAnimating() bool {
	now := time.Now()
	if m.loading {
		return true
	}
	if !m.startupUntil.IsZero() && now.Before(m.startupUntil) && m.shouldRenderLaunchArt() {
		return true
	}
	return false
}

func (m *MainModel) sendMessageWithProgress(ctx context.Context, query string, progressCh chan<- app.ProgressEvent) tea.Cmd {
	return func() tea.Msg {
		cb := func(app.ProgressEvent) {}
		if progressCh != nil {
			cb = func(ev app.ProgressEvent) {
				progressCh <- ev
			}
		}

		sid := ""
		workDir := ""
		if m.session != nil {
			sid = m.session.ID
			workDir = m.sessionWorkDir()
		}
		response, err := m.app.ExecuteAgentTaskInSessionWithProgressEvents(ctx, sid, workDir, query, cb)
		if progressCh != nil {
			close(progressCh)
		}
		if err != nil {
			return aiResponseMsg{err: err}
		}
		return aiResponseMsg{response: response}
	}
}

func (m *MainModel) handlePermissionsCommand(input string) (handled bool, content string, role string) {
	trimmed := strings.TrimSpace(input)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "/permissions") {
		return false, "", ""
	}

	if m.app == nil {
		return true, "permissions command unavailable: app is not initialized", "error"
	}

	arg := strings.TrimSpace(trimmed[len("/permissions"):])
	if arg == "" {
		return true, renderPermissionsStatus(m.app.Config.Permissions), "system"
	}

	mode, ok := app.ParsePermissionsMode(arg)
	if !ok {
		return true, fmt.Sprintf("invalid permissions mode %q. use: /permissions full-access|dangerously-full-access", arg), "error"
	}

	m.app.Config.Permissions = mode
	if err := app.SaveConfig(m.app.Config, app.DefaultConfigPath()); err != nil {
		return true, fmt.Sprintf("permissions updated in-memory (%s), but failed to save config: %v\n\n%s", mode, err, renderPermissionsStatus(mode)), "error"
	}
	return true, fmt.Sprintf("permissions updated to %s\n\n%s", mode, renderPermissionsStatus(mode)), "system"
}

func renderPermissionsStatus(desired string) string {
	desiredMode := app.NormalizePermissionsMode(desired)
	effectiveMode, isRoot := app.EffectivePermissionsMode(desiredMode)

	var b strings.Builder
	b.WriteString("permissions status:\n")
	b.WriteString("- desired: ")
	b.WriteString(desiredMode)
	b.WriteString("\n")
	b.WriteString("- effective: ")
	b.WriteString(effectiveMode)
	b.WriteString("\n")
	if isRoot {
		b.WriteString("- running as root: yes\n")
	} else {
		b.WriteString("- running as root: no\n")
	}
	if desiredMode == app.PermissionsDangerouslyFullAccess && !isRoot {
		b.WriteString("- note: dangerously-full-access requires launching eai as root (example: sudo -E eai)\n")
	}
	return strings.TrimSpace(b.String())
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
}

// Color/theme constants live in theme.go.
