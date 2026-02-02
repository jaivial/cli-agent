package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-agent/internal/app"
)

type SetupWizard struct {
	apiKey    string
	statusMsg string
	input     textinput.Model
	done      bool
	saved     bool
	cfg       *app.Config
	width     int
	height    int
}

func NewSetupWizard(cfg *app.Config) *SetupWizard {
	ti := textinput.New()
	ti.Placeholder = "paste your api key here..."
	ti.Focus()
	ti.Width = 50
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	return &SetupWizard{
		cfg:   cfg,
		input: ti,
	}
}

func (s *SetupWizard) Init() tea.Cmd {
	return textinput.Blink
}

func (s *SetupWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			s.done = true
			return s, tea.Quit

		case "enter":
			s.apiKey = strings.TrimSpace(s.input.Value())
			if s.apiKey == "" {
				s.statusMsg = "api key cannot be empty"
				return s, nil
			}

			// Save config
			s.cfg.MinimaxAPIKey = s.apiKey
			s.cfg.Model = "minimax-m2.1"
			s.cfg.BaseURL = "https://api.minimax.io/anthropic/v1/messages"
			s.cfg.MaxTokens = 2048
			s.cfg.Installed = true

			if err := app.SaveConfig(*s.cfg, ""); err != nil {
				s.statusMsg = "failed to save: " + err.Error()
				return s, nil
			}

			s.saved = true
			s.done = true
			return s, tea.Quit

		default:
			s.input, cmd = s.input.Update(msg)
			return s, cmd
		}

	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	}

	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

func (s *SetupWizard) View() string {
	if s.done {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB86C")).
		Bold(true)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4"))

	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#BD93F9")).
		Padding(0, 1)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5555"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#44475A"))

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("connect to minimax"))
	b.WriteString("\n\n")

	b.WriteString(subtitleStyle.Render("enter your api key from minimax.ai"))
	b.WriteString("\n\n")

	// Input box
	b.WriteString(inputBoxStyle.Render(s.input.View()))
	b.WriteString("\n\n")

	// Error message
	if s.statusMsg != "" {
		b.WriteString(errorStyle.Render(s.statusMsg))
		b.WriteString("\n\n")
	}

	b.WriteString(hintStyle.Render("enter to save • esc to cancel"))

	return b.String()
}

func (s *SetupWizard) Done() bool {
	return s.done
}

func (s *SetupWizard) Saved() bool {
	return s.saved
}

func (s *SetupWizard) GetConfig() app.Config {
	return *s.cfg
}
