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

type MainModel struct {
	app         *app.Application
	mode        app.Mode
	messages    []Message
	input       textarea.Model
	viewport    viewport.Model
	help        helpModel
	width       int
	height      int
	markdown    *MarkdownRenderer
	loading     bool
	loadingText string
	spinnerPos  int
	modeIndex   int
}

type Message struct {
	ID        string
	Role      string
	Content   string
	Timestamp time.Time
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
	ta.Placeholder = "Type a message... (Shift+Tab to switch mode, Enter to send)"
	ta.Focus()
	ta.CharLimit = 8000
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.Prompt = "▍ "

	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	modeIndex := 0
	for i, m := range modes {
		if m == mode {
			modeIndex = i
			break
		}
	}

	m := &MainModel{
		app:         application,
		mode:        mode,
		modeIndex:   modeIndex,
		messages:    []Message{},
		input:       ta,
		help:        newHelpModel(),
		width:       100,
		height:      30,
		markdown:    NewMarkdownRenderer(),
		loading:     false,
		loadingText: "Thinking...",
	}

	m.messages = append(m.messages, Message{
		ID:        "welcome-1",
		Role:      "system",
		Content:   "Welcome to eai! An intelligent CLI assistant for developers.\n\n**Get started:**\n- Type a message and press Enter to send\n- Press Shift+Tab to switch between Plan/Code/Do modes\n- Type /connect to configure your API key\n- Press ? for help",
		Timestamp: time.Now(),
	})

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
		m.viewport = viewport.New(msg.Width-38, msg.Height-14)
		m.viewport.Style = viewportStyle
		m.input.SetWidth(msg.Width - 38)
		m.help.SetWidth(msg.Width)
		m.updateViewport()
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.help.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.help.keys.NextMode):
			m.modeIndex = (m.modeIndex + 1) % len(modes)
			m.mode = modes[m.modeIndex]
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("mode-%d", time.Now().UnixNano()),
				Role:      "system",
				Content:   fmt.Sprintf("Switched to **%s** mode", strings.ToUpper(string(m.mode))),
				Timestamp: time.Now(),
			})
			m.updateViewport()
			m.viewport.GotoBottom()
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
						Content:   fmt.Sprintf("Wizard error: %v", err),
						Timestamp: time.Now(),
					})
				} else {
					m.messages = append(m.messages, Message{
						ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
						Role:      "system",
						Content:   "Configuration saved! Restart eai to use your new API key.",
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
			m.loadingText = "Thinking..."
			m.updateViewport()
			m.viewport.GotoBottom()
			return m, tea.Batch(m.sendMessage(input), m.spinTick())

		case key.Matches(msg, m.help.keys.Clear):
			m.messages = []Message{}
			m.messages = append(m.messages, Message{
				ID:        "welcome-1",
				Role:      "system",
				Content:   "Chat cleared. Type a new message to continue.",
				Timestamp: time.Now(),
			})
			m.updateViewport()
			return m, nil
		}

	case aiResponseMsg:
		m.loading = false
		if msg.err != nil {
			m.messages = append(m.messages, Message{
				ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
				Role:      "error",
				Content:   fmt.Sprintf("Error: %v", msg.err),
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
	m.updateViewport()

	return m, tea.Batch(cmds...)
}

func (m *MainModel) updateViewport() {
	var b strings.Builder

	for _, msg := range m.messages {
		b.WriteString(renderMessage(msg, m.markdown, m.width-42))
		b.WriteString("\n")
	}

	if m.loading {
		sp := spinner[m.spinnerPos]
		b.WriteString(loadingStyle.Width(m.width - 42).Render(fmt.Sprintf("%s %s", sp, m.loadingText)))
	}

	m.viewport.SetContent(b.String())
}

func renderMessage(msg Message, md *MarkdownRenderer, width int) string {
	var bubbleStyle lipgloss.Style
	var roleLabel string

	switch msg.Role {
	case "user":
		bubbleStyle = userBubbleStyle
		roleLabel = "You"
	case "assistant":
		bubbleStyle = aiBubbleStyle
		roleLabel = "AI"
	case "system":
		bubbleStyle = systemBubbleStyle
		roleLabel = "System"
	case "error":
		bubbleStyle = errorBubbleStyle
		roleLabel = "Error"
	default:
		bubbleStyle = systemBubbleStyle
		roleLabel = msg.Role
	}

	timestamp := msg.Timestamp.Format("15:04")
	header := fmt.Sprintf(" %s • %s ", roleLabel, timestamp)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(bubbleStyle.GetBackground()).
		Padding(0, 1).
		Width(width).
		Render(header)

	content := msg.Content
	if msg.Role == "assistant" {
		content = md.Render(msg.Content, width-4)
	}

	body := bubbleStyle.Width(width).Render(content)

	return headerStyle + "\n" + body
}

func (m *MainModel) View() string {
	if m.width < 80 || m.height < 20 {
		return m.minimalView()
	}

	var b strings.Builder

	b.WriteString(renderHeader(m.width))
	b.WriteString("\n")

	b.WriteString(renderMainArea(m))
	b.WriteString("\n")

	b.WriteString(renderSidebar(m.width, m.height, len(m.messages), m.modeIndex))
	b.WriteString("\n")

	b.WriteString(renderStatusBar(m.width, string(m.mode)))
	b.WriteString("\n")

	b.WriteString(renderInput(m.input, m.width-36))

	return b.String()
}

func (m *MainModel) minimalView() string {
	return fmt.Sprintf("Resize terminal to at least 80x24. Current: %dx%d", m.width, m.height)
}

func renderHeader(width int) string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 2).
		Width(width - 4).
		Render(" eai CLI Agent ")

	border := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Render(strings.Repeat("━", width-4))

	return lipgloss.NewStyle().
		Width(width).
		Render(header + "\n" + border)
}

func renderMainArea(m *MainModel) string {
	chatArea := lipgloss.NewStyle().
		Width(m.width - 36).
		Height(m.height - 12).
		Render(m.viewport.View())

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorFg)).
		Background(lipgloss.Color(colorBg)).
		Render(chatArea)
}

func renderSidebar(width, height, msgCount, modeIndex int) string {
	sidebarWidth := 34

	var b strings.Builder
	b.WriteString(sidebarTitle.Render(" Context "))
	b.WriteString("\n\n")

	b.WriteString(sidebarSection.Render("Messages"))
	b.WriteString(fmt.Sprintf("  %d\n\n", msgCount))

	b.WriteString(sidebarSection.Render("Shortcuts"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  Send message\n", keyStyle.Render("Enter")))
	b.WriteString(fmt.Sprintf("  %s  Switch mode\n", keyStyle.Render("Shift+Tab")))
	b.WriteString(fmt.Sprintf("  %s  Clear chat\n", keyStyle.Render("x")))
	b.WriteString(fmt.Sprintf("  %s  Show help\n", keyStyle.Render("?")))
	b.WriteString(fmt.Sprintf("  %s  Quit\n", keyStyle.Render("q")))

	b.WriteString("\n")
	b.WriteString(sidebarSection.Render("Modes"))
	b.WriteString("\n")
	for i, mode := range []string{"Plan", "Code", "Do"} {
		marker := "○"
		if i == modeIndex {
			marker = "●"
		}
		current := ""
		if i == modeIndex {
			current = " (current)"
		}
		b.WriteString(fmt.Sprintf("  %s %s%s\n", marker, mode, current))
	}

	b.WriteString("\n")
	b.WriteString(sidebarSection.Render("Commands"))
	b.WriteString("\n")
	b.WriteString("  /connect  Configure\n")

	content := b.String()

	return lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(height-10).
		Foreground(lipgloss.Color(colorFgMuted)).
		Background(lipgloss.Color(colorBgAlt)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(1, 1).
		Render(content)
}

func renderStatusBar(width int, mode string) string {
	left := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color(colorBgAlt)).
		Padding(0, 1).
		Render(fmt.Sprintf(" eai | %s ", strings.ToUpper(mode)))

	right := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorFgMuted)).
		Background(lipgloss.Color(colorBgAlt)).
		Padding(0, 1).
		Render(time.Now().Format("15:04"))

	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorBorder)).
		Render("│")

	middle := strings.Repeat(" ", width-4-lipgloss.Width(left)-lipgloss.Width(right)-2)

	return lipgloss.NewStyle().
		Width(width).
		Foreground(lipgloss.Color(colorFg)).
		Background(lipgloss.Color(colorBgAlt)).
		Render(left + separator + middle + separator + right)
}

func renderInput(input textarea.Model, width int) string {
	return inputStyle.Width(width).Render(input.View())
}

func (m *MainModel) spinTick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(_ time.Time) tea.Msg {
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

const (
	colorBg         = "#0F172A"
	colorBgAlt      = "#1E293B"
	colorFg         = "#F8FAFC"
	colorFgMuted    = "#94A3B8"
	colorBorder     = "#334155"
	colorPrimary    = "#3B82F6"
	colorSecondary  = "#8B5CF6"
	colorAccent     = "#06B6D4"
	colorCodeBg     = "#1E293B"
	colorCodeBorder = "#334155"
)

var (
	viewportStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg)).
			Background(lipgloss.Color(colorBg)).
			Padding(0, 1)

	userBubbleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#3B82F6")).
			Padding(1, 2).
			Width(80)

	aiBubbleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#10B981")).
			Padding(1, 2).
			Width(80)

	systemBubbleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F59E0B")).
				Background(lipgloss.Color("#1E293B")).
				Padding(1, 2).
				Width(80)

	errorBubbleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#EF4444")).
				Padding(1, 2).
				Width(80)

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Padding(1, 1)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg)).
			Background(lipgloss.Color(colorBgAlt)).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder))

	sidebarTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Width(30)

	sidebarSection = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4ECDC4")).
			Width(30)

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color("#1E293B")).
			Padding(0, 1)
)
