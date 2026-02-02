package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cli-agent/internal/app"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Layout constants - minimal design
const (
	headerHeight = 9  // For ASCII banner
	statusHeight = 1
	inputHeight  = 4  // Input box with border
	minWidth     = 60
	minHeight    = 20
)

type MainModel struct {
	app          *app.Application
	mode         app.Mode
	messages     []Message
	input        textarea.Model
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
}

type Message struct {
	ID        string
	Role      string
	Content   string
	Timestamp time.Time
	// For file edits
	IsFileEdit   bool
	FilePath     string
	ChangeType   string
	OldContent   string
	NewContent   string
}

type spinMsg struct{}

type aiResponseMsg struct {
	response string
	err      error
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
var modes = []app.Mode{app.ModePlan, app.ModeCode, app.ModeDo}

func NewMainModel(application *app.Application, mode app.Mode) *MainModel {
	ta := textarea.New()
	ta.Placeholder = "message... (shift+tab: mode, enter: send)"
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

	m := &MainModel{
		app:          application,
		mode:         mode,
		modeIndex:    modeIndex,
		messages:     []Message{},
		input:        ta,
		help:         newHelpModel(),
		width:        100,
		height:       30,
		markdown:     NewMarkdownRenderer(),
		diffRenderer: NewDiffRenderer(),
		loading:      false,
		loadingText:  "thinking...",
		ready:        false,
	}

	m.messages = append(m.messages, Message{
		ID:        "welcome-1",
		Role:      "system",
		Content:   "eai v2 ready. enter to send, shift+tab to change mode.",
		Timestamp: time.Now(),
	})

	return m
}

func (m *MainModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *MainModel) chatAreaWidth() int {
	return m.width - 2
}

func (m *MainModel) chatAreaHeight() int {
	return m.height - headerHeight - statusHeight - inputHeight
}

func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		chatWidth := m.chatAreaWidth()
		chatHeight := m.chatAreaHeight()

		if !m.ready {
			m.viewport = viewport.New(chatWidth, chatHeight)
			m.viewport.Style = lipgloss.NewStyle()
			m.ready = true
		} else {
			m.viewport.Width = chatWidth
			m.viewport.Height = chatHeight
		}

		m.input.SetWidth(chatWidth - 4)
		m.help.SetWidth(m.width)
		m.updateViewport()
		return m, nil

	case tea.KeyMsg:
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
						Content:   "connected to minimax api.",
						Timestamp: time.Now(),
					})
				}
				m.input.Reset()
				m.updateViewport()
				m.viewport.GotoBottom()
				return m, nil
			}

			userMsg := Message{
				ID:        fmt.Sprintf("user-%d", time.Now().UnixNano()),
				Role:      "user",
				Content:   input,
				Timestamp: time.Now(),
			}
			m.messages = append(m.messages, userMsg)
			m.input.Reset()
			m.loading = true
			m.spinnerPos = 0
			m.loadingText = "thinking..."
			m.updateViewport()
			m.viewport.GotoBottom()
			return m, tea.Batch(m.sendMessage(input), m.spinTick())

		case key.Matches(msg, m.help.keys.Clear):
			m.messages = []Message{}
			m.messages = append(m.messages, Message{
				ID:        "welcome-1",
				Role:      "system",
				Content:   "chat cleared.",
				Timestamp: time.Now(),
			})
			m.updateViewport()
			return m, nil

		case msg.Type == tea.KeyPgUp:
			m.viewport.ViewUp()
			return m, nil

		case msg.Type == tea.KeyPgDown:
			m.viewport.ViewDown()
			return m, nil
		}

	case aiResponseMsg:
		m.loading = false
		if msg.err != nil {
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
				Role:      "error",
				Content:   fmt.Sprintf("error: %v", msg.err),
				Timestamp: time.Now(),
			})
		} else {
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("ai-%d", time.Now().UnixNano()),
				Role:      "assistant",
				Content:   msg.response,
				Timestamp: time.Now(),
			})
		}
		m.updateViewport()
		m.viewport.GotoBottom()
		return m, nil

	case spinMsg:
		m.spinnerPos = (m.spinnerPos + 1) % len(spinner)
		if m.loading {
			m.updateViewport()
			return m, m.spinTick()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *MainModel) updateViewport() {
	var b strings.Builder
	chatWidth := m.chatAreaWidth() - 2

	for _, msg := range m.messages {
		b.WriteString(m.renderMessage(msg, chatWidth))
		b.WriteString("\n")
	}

	// Loading spinner moved to input area, not in chat
	m.viewport.SetContent(b.String())
}

func (m *MainModel) renderMessage(msg Message, width int) string {
	// Text-only colors, no backgrounds, no boxes
	var textColor lipgloss.Color
	var roleLabel string
	var alignRight bool

	switch msg.Role {
	case "user":
		textColor = lipgloss.Color("#8BE9FD") // Cyan for user
		roleLabel = "you"
		alignRight = false // User messages left
	case "assistant":
		textColor = lipgloss.Color("#50FA7B") // Green for AI
		roleLabel = "eai"
		alignRight = true // AI messages right
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

	// Header with same color as message text
	timestamp := msg.Timestamp.Format("15:04")
	headerText := fmt.Sprintf("%s %s", roleLabel, timestamp)
	header := lipgloss.NewStyle().
		Foreground(textColor).
		Render(headerText)

	// Content styling - just color, no width/box
	var content string
	if msg.Role == "assistant" {
		content = m.markdown.Render(msg.Content, width)
	} else {
		content = lipgloss.NewStyle().
			Foreground(textColor).
			Render(msg.Content)
	}

	// Align right for AI messages
	if alignRight {
		headerStyle := lipgloss.NewStyle().Width(width).Align(lipgloss.Right)
		contentStyle := lipgloss.NewStyle().Width(width).Align(lipgloss.Right)
		return headerStyle.Render(header) + "\n" + contentStyle.Render(content)
	}

	return header + "\n" + content
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

	// Build minimal layout
	header := m.renderHeader()
	chatArea := m.viewport.View()
	statusBar := m.renderStatusBar()
	inputArea := m.renderInputArea()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		chatArea,
		statusBar,
		inputArea,
	)
}

func (m *MainModel) renderHeader() string {
	// Minimal ASCII banner
	banner := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB86C")).
		Bold(true).
		Render(eaiTextASCII)

	// Mode indicator
	modeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#BD93F9"))

	modeIndicator := modeStyle.Render(fmt.Sprintf("[%s]", string(m.mode)))

	// Separator line
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#44475A")).
		Render(strings.Repeat("─", m.width))

	return lipgloss.JoinVertical(lipgloss.Left,
		banner,
		modeIndicator,
		separator,
	)
}

func (m *MainModel) renderStatusBar() string {
	leftStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4"))

	rightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4"))

	left := leftStyle.Render(fmt.Sprintf("eai | %s", string(m.mode)))

	scrollInfo := ""
	if m.viewport.TotalLineCount() > m.viewport.Height {
		scrollInfo = fmt.Sprintf("%d%% ", int(m.viewport.ScrollPercent()*100))
	}

	right := rightStyle.Render(scrollInfo + time.Now().Format("15:04"))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	middle := strings.Repeat(" ", gap)

	return left + middle + right
}

func (m *MainModel) renderInputArea() string {
	var result strings.Builder

	// Show loading spinner above input when thinking
	if m.loading {
		sp := spinner[m.spinnerPos]
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#BD93F9"))
		result.WriteString(loadingStyle.Render(fmt.Sprintf("%s %s", sp, m.loadingText)))
		result.WriteString("\n")
	}

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

func (m *MainModel) spinTick() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(_ time.Time) tea.Msg {
		return spinMsg{}
	})
}

func (m *MainModel) sendMessage(query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		response, err := m.app.ExecuteChat(ctx, m.mode, query)
		if err != nil {
			return aiResponseMsg{err: err}
		}
		return aiResponseMsg{response: response}
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
