package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cli-agent/internal/app"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the main TUI application state
type Model struct {
	app         *app.Application
	mode        app.Mode
	messages    []Message
	input       textarea.Model
	loading     bool
	help        helpModel
	windowWidth int
	windowHeight int
	markdown    *MarkdownRenderer
	loadingSpinner int
}

// Message represents a single chat message
type Message struct {
	ID        string
	Role      string
	Content   string
	Timestamp time.Time
}

// New creates a new TUI application
func New(application *app.Application, mode app.Mode) *Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here... (Shift+Enter for new line)"
	ta.Focus()
	ta.CharLimit = 8000
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.Prompt = "▍ "

	// Customize textarea styles
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	return &Model{
		app:            application,
		mode:           mode,
		messages:       []Message{},
		input:          ta,
		loading:        false,
		help:           newHelpModel(),
		windowWidth:    80,
		windowHeight:   24,
		markdown:       NewMarkdownRenderer(),
		loadingSpinner: 0,
	}
}

// Init initializes the TUI
func (m *Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles UI updates
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.input.SetWidth(msg.Width - 8)
		m.help.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.help.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.help.keys.Enter):
			if m.input.Value() == "" {
				return m, nil
			}

			// Add user message
			userMsg := Message{
				ID:        fmt.Sprintf("user-%d", time.Now().UnixNano()),
				Role:      "user",
				Content:   strings.TrimSpace(m.input.Value()),
				Timestamp: time.Now(),
			}
			m.messages = append(m.messages, userMsg)
			query := m.input.Value()
			m.input.Reset()
			m.loading = true
			m.loadingSpinner = 0

			// Send to AI
			return m, tea.Batch(m.sendToAI(query), m.spinCmd())

		case key.Matches(msg, m.help.keys.Clear):
			m.messages = []Message{}
			return m, nil
		}

	case aiResponseMsg:
		m.loading = false
		if msg.err != nil {
			errorMsg := Message{
				ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
				Role:      "error",
				Content:   fmt.Sprintf("Error: %v", msg.err),
				Timestamp: time.Now(),
			}
			m.messages = append(m.messages, errorMsg)
		} else {
			aiMsg := Message{
				ID:        fmt.Sprintf("ai-%d", time.Now().UnixNano()),
				Role:      "assistant",
				Content:   msg.response,
				Timestamp: time.Now(),
			}
			m.messages = append(m.messages, aiMsg)
		}
		return m, nil

	case spinMsg:
		if m.loading {
			m.loadingSpinner = (m.loadingSpinner + 1) % 4
			return m, m.spinCmd()
		}
	}

	// Update input
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the TUI
func (m *Model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Chat history
	b.WriteString(m.renderMessages())
	b.WriteString("\n")

	// Input area
	b.WriteString(m.renderInput())
	b.WriteString("\n")

	// Loading indicator
	if m.loading {
		spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spinnerChar := spinnerChars[m.loadingSpinner%len(spinnerChars)]
		b.WriteString(loadingStyle.Render(fmt.Sprintf("%s Thinking...", spinnerChar)))
		b.WriteString("\n")
	}

	// Help
	b.WriteString(m.help.View())

	return b.String()
}

// sendToAI sends a query to the AI and returns a response
func (m *Model) sendToAI(query string) tea.Cmd {
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

// spinCmd creates a command to animate the loading spinner
func (m *Model) spinCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(_ time.Time) tea.Msg {
		return spinMsg{}
	})
}

// aiResponseMsg represents an AI response
type aiResponseMsg struct {
	response string
	err      error
}

// spinMsg is used to trigger spinner animation updates
type spinMsg struct{}

// Colors - Modern, professional color scheme inspired by ClaudeCode
const (
	colorBg            = "#0F172A" // Slate 900
	colorBgAlt         = "#1E293B" // Slate 800
	colorBgCard        = "#1E293B" // Slate 800
	colorFg            = "#F8FAFC" // Slate 50
	colorFgMuted       = "#94A3B8" // Slate 400
	colorPrimary       = "#3B82F6" // Blue 500
	colorPrimaryLight  = "#60A5FA" // Blue 400
	colorSecondary     = "#8B5CF6" // Purple 500
	colorAccent        = "#06B6D4" // Cyan 500
	colorSuccess       = "#10B981" // Emerald 500
	colorWarning       = "#F59E0B" // Amber 500
	colorError         = "#EF4444" // Red 500
	colorBorder        = "#334155" // Slate 700
	colorCodeBg        = "#1E293B" // Slate 800
	colorCodeBorder    = "#334155" // Slate 700
	colorUserMsg       = "#3B82F6" // Blue 500
	colorAssistantMsg  = "#10B981" // Emerald 500
)

// Styles - ClaudeCode visual design
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorFg)).
			Background(lipgloss.Color(colorBgAlt)).
			Padding(0, 2).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(colorBorder)).
			Width(80)

	userMessageStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorUserMsg)).
			Background(lipgloss.Color(colorBgCard)).
			Padding(1, 3).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder)).
			MarginBottom(2).
			Width(80)

	assistantMessageStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorAssistantMsg)).
			Background(lipgloss.Color(colorBgCard)).
			Padding(1, 3).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder)).
			MarginBottom(2).
			Width(80)

	errorMessageStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorError)).
			Background(lipgloss.Color(colorBgCard)).
			Padding(1, 3).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorError)).
			MarginBottom(2).
			Width(80)

	messageContentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg)).
			Background(lipgloss.Color(colorBgCard)).
			Padding(1, 3).
			MarginBottom(2).
			Width(80)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg)).
			Background(lipgloss.Color(colorBgCard)).
			Padding(1, 3).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder)).
			Width(80)

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPrimary)).
			Background(lipgloss.Color(colorBg)).
			Padding(1, 3).
			Width(80)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFgMuted)).
			Background(lipgloss.Color(colorBg)).
			Padding(0, 3).
			Width(80)
)

// renderHeader renders the header
func (m *Model) renderHeader() string {
	modeStr := string(m.mode)
	headerText := fmt.Sprintf("CLI Agent - Mode: %s", modeStr)
	return headerStyle.Width(m.windowWidth - 4).Render(headerText)
}

// renderMessages renders the chat history
func (m *Model) renderMessages() string {
	var b strings.Builder

	for _, msg := range m.messages {
		var header string
		var style lipgloss.Style

		switch msg.Role {
		case "user":
			header = fmt.Sprintf("You • %s", msg.Timestamp.Format("15:04:05"))
			style = userMessageStyle
		case "assistant":
			header = fmt.Sprintf("AI Assistant • %s", msg.Timestamp.Format("15:04:05"))
			style = assistantMessageStyle
		case "error":
			header = "Error"
			style = errorMessageStyle
		}

		b.WriteString(style.Width(m.windowWidth - 4).Render(header))
		b.WriteString("\n")

		contentStyle := messageContentStyle.Width(m.windowWidth - 4)

		// Render content with markdown
		if msg.Role == "assistant" {
			content := m.markdown.Render(msg.Content, m.windowWidth-8)
			b.WriteString(contentStyle.Render(content))
		} else {
			b.WriteString(contentStyle.Render(msg.Content))
		}

		b.WriteString("\n")
	}

	return b.String()
}

// renderInput renders the input area
func (m *Model) renderInput() string {
	return inputStyle.Width(m.windowWidth - 4).Render(m.input.View())
}
