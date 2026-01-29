package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-agent/internal/app"
)

type SetupWizard struct {
	step      int
	provider  string
	apiKey    string
	model     string
	statusMsg string
	input     textinput.Model
	done      bool
	cfg       *app.Config
	width     int
	height    int
	providers []string
	selected  int
}

func NewSetupWizard(cfg *app.Config) *SetupWizard {
	s := &SetupWizard{
		step:      0,
		providers: []string{"MiniMax"},
		selected:  0,
		cfg:       cfg,
		input:     textinput.New(),
	}
	s.input.Placeholder = "sk-cp-..."
	s.input.Focus()
	return s
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
			switch s.step {
			case 0:
				s.provider = s.providers[s.selected]
				s.step = 1
				s.input.Focus()
			case 1:
				s.apiKey = s.input.Value()
				if s.apiKey == "" {
					s.statusMsg = "Please enter an API key"
					break
				}
				if !strings.HasPrefix(s.apiKey, "sk-") && !strings.HasPrefix(s.apiKey, "sk-cp-") {
					s.statusMsg = "Warning: API key doesn't look like a MiniMax key (should start with sk-)"
				}
				s.step = 2
			case 2:
				s.model = "minimax-m2.1"
				s.step = 3
			case 3:
				s.cfg.MinimaxAPIKey = s.apiKey
				s.cfg.Model = s.model
				s.cfg.BaseURL = "https://api.minimax.io/anthropic/v1/messages"
				s.cfg.MaxTokens = 2048
				s.cfg.Installed = true

				if err := app.SaveConfig(*s.cfg, ""); err != nil {
					s.statusMsg = fmt.Sprintf("Error saving config: %v", err)
				} else {
					s.statusMsg = "Configuration saved successfully!"
					s.done = true
					return s, tea.Quit
				}
			}

		case "up", "k":
			if s.step == 0 && s.selected > 0 {
				s.selected--
			} else if s.step > 0 {
				s.step--
				if s.step == 0 {
					s.selected = 0
				}
			}
		case "down", "j":
			if s.step == 0 && s.selected < len(s.providers)-1 {
				s.selected++
			} else if s.step < 3 {
				s.step++
			}

		default:
			if s.step == 1 {
				s.input, cmd = s.input.Update(msg)
				return s, cmd
			}
		}

	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	}

	return s, cmd
}

func (s *SetupWizard) View() string {
	if s.done {
		return ""
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 2).
		Width(s.width - 4).
		Render("  EAI Setup Wizard  ")

	var body string
	var progressBar string

	switch s.step {
	case 0:
		progressBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4")).
			Render("▓▓▓░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░")
		options := ""
		for i, p := range s.providers {
			marker := "○"
			if i == s.selected {
				marker = "●"
			}
			options += fmt.Sprintf("  %s %s\n", marker, p)
		}
		body = fmt.Sprintf(`
Step 1 of 4: Select Provider

%s
Use ↑/↓ to select, Enter to confirm.

`, options)

	case 1:
		progressBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4")).
			Render("▓▓▓▓▓▓░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░")
		body = fmt.Sprintf(`
Step 2 of 4: Enter API Key

Get your key from: https://minimax.ai

%s

API Key: %s

Use ↑ to go back, Enter to continue, Ctrl+C to cancel.
`, s.statusMsg, s.input.View())
		s.statusMsg = ""

	case 2:
		progressBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4")).
			Render("▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░")
		body = fmt.Sprintf(`
Step 3 of 4: Select Model

  ● minimax-m2.1 (recommended)

Selected: %s

Use ↑ to go back, Enter to save, Ctrl+C to cancel.
`, s.model)

	case 3:
		progressBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4")).
			Render("▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓")
		body = fmt.Sprintf(`
Step 4 of 4: Confirm Configuration

  ✓ Provider:   %s
  ✓ Model:      %s
  ✓ API Key:    %s...%s

Configuration saved to:
%s

Use ↑ to go back, Enter to confirm, Ctrl+C to cancel.
`, s.provider, s.model, s.apiKey[:4], s.apiKey[len(s.apiKey)-4:], app.GetBinaryConfigPath())
	}

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render("\n↑/↓ Navigate  |  Enter Confirm  |  Ctrl+C Cancel")

	content := header + "\n\n" + progressBar + "\n\n" + body + help

	paddingTop := maxInt(0, (s.height-20)/2)
	paddingSides := maxInt(0, (s.width-lipgloss.Width(content)-4)/2)

	result := strings.Repeat("\n", paddingTop)
	if paddingSides > 0 {
		result += strings.Repeat(" ", paddingSides)
	}
	result += content

	return lipgloss.NewStyle().
		Width(s.width).
		Height(s.height).
		Render(result)
}

func (s *SetupWizard) Done() bool {
	return s.done
}

// maxInt returns the larger of two integers (Go 1.18 compatibility)
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
