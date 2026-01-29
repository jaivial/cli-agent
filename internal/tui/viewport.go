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

// Layout constants
const (
	sidebarWidth   = 32
	headerHeight   = 3
	statusHeight   = 1
	inputHeight    = 5
	minWidth       = 80
	minHeight      = 24
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
	ready       bool
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
	ta.ShowLineNumbers = false

	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ta.FocusedStyle.Base = lipgloss.NewStyle().Background(lipgloss.Color(colorBgAlt))
	ta.BlurredStyle.Base = lipgloss.NewStyle().Background(lipgloss.Color(colorBgAlt))

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
		ready:       false,
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

func (m *MainModel) chatAreaWidth() int {
	return m.width - sidebarWidth - 4
}

func (m *MainModel) chatAreaHeight() int {
	return m.height - headerHeight - statusHeight - inputHeight - 2
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
			m.viewport.Style = lipgloss.NewStyle().
				Background(lipgloss.Color(colorBg))
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

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *MainModel) updateViewport() {
	var b strings.Builder
	chatWidth := m.chatAreaWidth() - 4

	for _, msg := range m.messages {
		b.WriteString(m.renderMessage(msg, chatWidth))
		b.WriteString("\n\n")
	}

	if m.loading {
		sp := spinner[m.spinnerPos]
		loadingBox := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")).
			Background(lipgloss.Color(colorBgAlt)).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#60A5FA")).
			Width(chatWidth).
			Render(fmt.Sprintf("%s %s", sp, m.loadingText))
		b.WriteString(loadingBox)
		b.WriteString("\n")
	}

	m.viewport.SetContent(b.String())
}

func (m *MainModel) renderMessage(msg Message, width int) string {
	var headerBg lipgloss.Color
	var roleLabel string
	var bodyBg lipgloss.Color
	var bodyFg lipgloss.Color

	switch msg.Role {
	case "user":
		headerBg = lipgloss.Color("#3B82F6")
		bodyBg = lipgloss.Color("#1E3A5F")
		bodyFg = lipgloss.Color("#E0F2FE")
		roleLabel = "You"
	case "assistant":
		headerBg = lipgloss.Color("#10B981")
		bodyBg = lipgloss.Color("#1E3B32")
		bodyFg = lipgloss.Color("#D1FAE5")
		roleLabel = "AI"
	case "system":
		headerBg = lipgloss.Color("#6B7280")
		bodyBg = lipgloss.Color("#1E293B")
		bodyFg = lipgloss.Color("#F59E0B")
		roleLabel = "System"
	case "error":
		headerBg = lipgloss.Color("#EF4444")
		bodyBg = lipgloss.Color("#451A1A")
		bodyFg = lipgloss.Color("#FEE2E2")
		roleLabel = "Error"
	default:
		headerBg = lipgloss.Color("#6B7280")
		bodyBg = lipgloss.Color("#1E293B")
		bodyFg = lipgloss.Color("#F8FAFC")
		roleLabel = msg.Role
	}

	timestamp := msg.Timestamp.Format("15:04")
	headerText := fmt.Sprintf(" %s  %s ", roleLabel, timestamp)

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(headerBg).
		Padding(0, 1).
		Width(width).
		Render(headerText)

	content := msg.Content
	if msg.Role == "assistant" {
		content = m.markdown.Render(msg.Content, width-4)
	}

	body := lipgloss.NewStyle().
		Foreground(bodyFg).
		Background(bodyBg).
		Padding(1, 2).
		Width(width).
		Render(content)

	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m *MainModel) View() string {
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true).
			Padding(2).
			Render(fmt.Sprintf("Please resize terminal to at least %dx%d\nCurrent: %dx%d", minWidth, minHeight, m.width, m.height))
	}

	if !m.ready {
		return "Initializing..."
	}

	// Build the layout
	header := m.renderHeader()
	mainContent := m.renderMainContent()
	statusBar := m.renderStatusBar()
	inputArea := m.renderInputArea()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		mainContent,
		statusBar,
		inputArea,
	)
}

func (m *MainModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 2)

	title := titleStyle.Render(" eai CLI Agent ")

	modeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4C1D95")).
		Padding(0, 1)

	modeText := modeStyle.Render(fmt.Sprintf(" %s ", strings.ToUpper(string(m.mode))))

	headerContent := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", modeText)

	border := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Render(strings.Repeat("─", m.width))

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().
			Width(m.width).
			Background(lipgloss.Color("#0F172A")).
			Padding(0, 1).
			Render(headerContent),
		border,
	)
}

func (m *MainModel) renderMainContent() string {
	chatWidth := m.chatAreaWidth()

	// Chat area with border
	chatBox := lipgloss.NewStyle().
		Width(chatWidth).
		Height(m.chatAreaHeight()).
		Background(lipgloss.Color(colorBg)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
		Render(m.viewport.View())

	// Sidebar
	sidebar := m.renderSidebar()

	// Join horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, chatBox, sidebar)
}

func (m *MainModel) renderSidebar() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Width(sidebarWidth - 4)

	b.WriteString(titleStyle.Render(" Context "))
	b.WriteString("\n\n")

	// Section style
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#4ECDC4"))

	// Messages count
	b.WriteString(sectionStyle.Render("Messages"))
	b.WriteString(fmt.Sprintf("\n  %d\n\n", len(m.messages)))

	// Shortcuts
	b.WriteString(sectionStyle.Render("Shortcuts"))
	b.WriteString("\n")

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B6B")).
		Background(lipgloss.Color("#1E293B")).
		Padding(0, 1)

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"Enter", "Send"},
		{"Shift+Tab", "Mode"},
		{"x", "Clear"},
		{"?", "Help"},
		{"q", "Quit"},
	}

	for _, s := range shortcuts {
		b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render(s.key), s.desc))
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Modes"))
	b.WriteString("\n")

	modeNames := []string{"Plan", "Code", "Do"}
	for i, mode := range modeNames {
		marker := "○"
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFgMuted))
		if i == m.modeIndex {
			marker = "●"
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
		}
		b.WriteString(style.Render(fmt.Sprintf("  %s %s\n", marker, mode)))
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Commands"))
	b.WriteString("\n")
	b.WriteString("  /connect  Setup API\n")

	return lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(m.chatAreaHeight()).
		Foreground(lipgloss.Color(colorFgMuted)).
		Background(lipgloss.Color(colorBgAlt)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(1, 1).
		Render(b.String())
}

func (m *MainModel) renderStatusBar() string {
	leftStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#334155")).
		Padding(0, 1)

	rightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorFgMuted)).
		Background(lipgloss.Color("#334155")).
		Padding(0, 1)

	left := leftStyle.Render(fmt.Sprintf(" eai | %s ", strings.ToUpper(string(m.mode))))

	scrollInfo := ""
	if m.viewport.TotalLineCount() > m.viewport.Height {
		scrollInfo = fmt.Sprintf(" Scroll: %d%% ", int(m.viewport.ScrollPercent()*100))
	}

	right := rightStyle.Render(scrollInfo + time.Now().Format("15:04"))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	middle := lipgloss.NewStyle().
		Background(lipgloss.Color("#334155")).
		Render(strings.Repeat(" ", gap))

	return left + middle + right
}

func (m *MainModel) renderInputArea() string {
	inputWidth := m.chatAreaWidth()

	inputBox := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorFg)).
		Background(lipgloss.Color(colorBgAlt)).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorPrimary)).
		Width(inputWidth).
		Render(m.input.View())

	// Hint text
	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorFgMuted)).
		Width(sidebarWidth).
		Padding(1, 1).
		Render("PgUp/PgDn to scroll")

	return lipgloss.JoinHorizontal(lipgloss.Top, inputBox, hint)
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

// Color constants
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
