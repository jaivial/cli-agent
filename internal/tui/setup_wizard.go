package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-agent/internal/app"
)

var (
	setupSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4"))
	
	setupErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B"))
)

// SetupWizard implements the configuration wizard
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
	}
	
	// Initialize API key input
	s.input = textinput.New()
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
			case 0: // Provider selection
				s.provider = s.providers[s.selected]
				s.step = 1
				s.input.Focus()
			case 1: // API Key
				s.apiKey = s.input.Value()
				if s.apiKey == "" {
					s.statusMsg = "Please enter an API key"
					break
				}
				if !strings.HasPrefix(s.apiKey, "sk-") && !strings.HasPrefix(s.apiKey, "sk-cp-") {
					s.statusMsg = "Warning: API key doesn't look like a MiniMax key (should start with sk-)"
				}
				s.step = 2
			case 2: // Model selection
				s.model = "minimax-m2.1"
				s.step = 3
			case 3: // Save and exit
				// Save config
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
			}
		case "down", "j":
			if s.step == 0 && s.selected < len(s.providers)-1 {
				s.selected++
			}
		}
	
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	
	default:
		if s.step == 1 {
			s.input, cmd = s.input.Update(msg)
		}
	}
	
	return s, cmd
}

func (s *SetupWizard) View() string {
	if s.done {
		return ""
	}
	
	// Calculate centered layout
	contentWidth := min(50, s.width-4)
	
	header := headerStyle.Render("  EAI Setup Wizard  ")
	header = lipgloss.NewStyle().Width(contentWidth + 4).Render(header)
	
	var body string
	
	switch s.step {
	case 0:
		// Provider selection
		options := ""
		for i, p := range s.providers {
			marker := "○"
			if i == s.selected {
				marker = "●"
			}
			options += fmt.Sprintf(" %s %s\n", marker, p)
		}
		body = fmt.Sprintf(`
Step 1 of 3: Choose Provider

%s

Use ↑/↓ to select, Enter to confirm, Ctrl+C to cancel.
`, options)
	
	case 1:
		body = fmt.Sprintf(`
Step 2 of 3: Enter your MiniMax API Key

Get your API key from: https://minimax.ai

%s

API Key: %s

Press Enter to continue or Ctrl+C to cancel.
`, s.statusMsg, s.input.View())
		s.statusMsg = ""
	
	case 2:
		body = fmt.Sprintf(`
Step 3 of 3: Select Model

Available models:
  ● minimax-m2.1 (recommended)

Selected: %s

Press Enter to save configuration or Ctrl+C to cancel.
`, s.model)
	
	case 3:
		body = fmt.Sprintf(`
✓ Setup Complete!

Provider:   MiniMax
Model:      %s
API Key:    %s...

Configuration saved to:
%s

Run 'eai' to start using the CLI agent!

Press any key to exit.
`, s.model, s.apiKey[:10], app.GetBinaryConfigPath())
	}
	
	// Center the content
	paddingTop := max(0, (s.height-15)/2)
	paddingSides := max(0, (s.width-contentWidth-4)/2)
	
	result := strings.Repeat("\n", paddingTop)
	result += strings.Repeat(" ", paddingSides) + header + "\n\n"
	result += strings.Repeat(" ", paddingSides) + body
	
	return result
}

func (s *SetupWizard) Done() bool {
	return s.done
}
