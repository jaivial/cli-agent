package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cli-agent/internal/app"
)

type setupStep int

const (
	stepProvider setupStep = iota
	stepAuthMethod
	stepAPIKey
)

const (
	providerMiniMax = "MiniMax"
	providerZAI     = "z.ai"

	authCodingPlan = "Coding Plan"
	authAPIKey     = "API Key"
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
	step      setupStep

	providers []string
	authModes []string

	providerIndex int
	authModeIndex int
}

func NewSetupWizard(cfg *app.Config) *SetupWizard {
	ti := textinput.New()
	ti.Placeholder = "paste your api key here..."
	ti.Width = 50
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	return &SetupWizard{
		cfg:       cfg,
		input:     ti,
		step:      stepProvider,
		providers: []string{providerMiniMax, providerZAI},
		authModes: []string{authCodingPlan, authAPIKey},
		// Start with z.ai selected by default.
		providerIndex: 1,
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
		case "ctrl+c":
			s.done = true
			return s, tea.Quit
		}

		switch msg.String() {
		case "esc":
			switch s.step {
			case stepAPIKey:
				s.statusMsg = ""
				s.input.Blur()
				if s.selectedProvider() == providerZAI {
					s.step = stepAuthMethod
				} else {
					s.step = stepProvider
				}
				return s, nil
			case stepAuthMethod:
				s.statusMsg = ""
				s.step = stepProvider
				return s, nil
			default:
				s.done = true
				return s, tea.Quit
			}

		case "enter":
			switch s.step {
			case stepProvider:
				s.statusMsg = ""
				if s.selectedProvider() == providerZAI {
					s.step = stepAuthMethod
				} else {
					s.step = stepAPIKey
					s.input.SetValue("")
					s.input.Focus()
				}
				return s, nil
			case stepAuthMethod:
				s.statusMsg = ""
				s.step = stepAPIKey
				s.input.SetValue("")
				s.input.Focus()
				return s, nil
			case stepAPIKey:
				s.apiKey = strings.TrimSpace(s.input.Value())
				if s.apiKey == "" {
					s.statusMsg = "api key cannot be empty"
					return s, nil
				}

				// Save config and apply selected provider/mode.
				s.cfg.APIKey = s.apiKey
				s.cfg.Model = app.DefaultModel
				if s.cfg.MaxTokens <= 0 {
					s.cfg.MaxTokens = app.DefaultConfig().MaxTokens
				}
				s.cfg.Installed = true

				if s.selectedProvider() == providerZAI {
					if s.selectedAuthMode() == authCodingPlan {
						s.cfg.BaseURL = "https://api.z.ai/api/coding/paas/v4"
					} else {
						s.cfg.BaseURL = "https://api.z.ai/api/paas/v4"
					}
				} else {
					// MiniMax entry remains available as requested.
					s.cfg.BaseURL = "https://api.z.ai/api/paas/v4"
				}

				if err := app.SaveConfig(*s.cfg, app.DefaultConfigPath()); err != nil {
					appendTUIErrorLog("connect save", "", err.Error())
					s.statusMsg = "failed to save: " + err.Error()
					return s, nil
				}

				s.saved = true
				s.done = true
				return s, tea.Quit
			}

		default:
			switch s.step {
			case stepProvider:
				switch msg.String() {
				case "up", "k":
					s.providerIndex = (s.providerIndex - 1 + len(s.providers)) % len(s.providers)
					return s, nil
				case "down", "j":
					s.providerIndex = (s.providerIndex + 1) % len(s.providers)
					return s, nil
				}
			case stepAuthMethod:
				switch msg.String() {
				case "up", "k":
					s.authModeIndex = (s.authModeIndex - 1 + len(s.authModes)) % len(s.authModes)
					return s, nil
				case "down", "j":
					s.authModeIndex = (s.authModeIndex + 1) % len(s.authModes)
					return s, nil
				}
			case stepAPIKey:
				s.input, cmd = s.input.Update(msg)
				return s, cmd
			}
		}

	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	}

	if s.step == stepAPIKey {
		s.input, cmd = s.input.Update(msg)
	}
	return s, cmd
}

func (s *SetupWizard) View() string {
	if s.done {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorAccent)).
		Bold(true)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorMuted))

	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorAccent2)).
		Padding(0, 1)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorError))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorSubtle))

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("connect"))
	b.WriteString("\n\n")

	switch s.step {
	case stepProvider:
		b.WriteString(subtitleStyle.Render("step 1/3: choose provider"))
		b.WriteString("\n\n")
		for i, p := range s.providers {
			prefix := "  "
			if i == s.providerIndex {
				prefix = "❯ "
			}
			b.WriteString(prefix + p + "\n")
		}
		b.WriteString("\n")
	case stepAuthMethod:
		b.WriteString(subtitleStyle.Render("step 2/3: z.ai auth mode"))
		b.WriteString("\n\n")
		for i, p := range s.authModes {
			prefix := "  "
			if i == s.authModeIndex {
				prefix = "❯ "
			}
			b.WriteString(prefix + p + "\n")
		}
		b.WriteString("\n")
		if s.selectedAuthMode() == authCodingPlan {
			b.WriteString(hintStyle.Render("docs index: https://docs.z.ai/llms.txt"))
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("coding plan base url: https://api.z.ai/api/coding/paas/v4"))
			b.WriteString("\n\n")
		}
	case stepAPIKey:
		b.WriteString(subtitleStyle.Render("step 3/3: enter EAI_API_KEY"))
		b.WriteString("\n\n")
		b.WriteString(inputBoxStyle.Render(s.input.View()))
		b.WriteString("\n\n")
	}

	// Error message
	if s.statusMsg != "" {
		b.WriteString(errorStyle.Render(s.statusMsg))
		b.WriteString("\n\n")
	}

	b.WriteString(hintStyle.Render("↑/↓ choose • enter next/save • esc back/cancel"))

	out := b.String()
	if s.width > 0 && s.height > 0 {
		// Fill the screen to avoid redraw artifacts when switching between steps.
		return lipgloss.Place(s.width, s.height, lipgloss.Left, lipgloss.Top, out)
	}
	return out
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

func (s *SetupWizard) Summary() string {
	if s.selectedProvider() == providerZAI {
		if s.selectedAuthMode() == authCodingPlan {
			return "connected: z.ai (Coding Plan)"
		}
		return "connected: z.ai (API Key)"
	}
	return "connected: MiniMax"
}

func (s *SetupWizard) selectedProvider() string {
	if s.providerIndex < 0 || s.providerIndex >= len(s.providers) {
		return providerZAI
	}
	return s.providers[s.providerIndex]
}

func (s *SetupWizard) selectedAuthMode() string {
	if s.authModeIndex < 0 || s.authModeIndex >= len(s.authModes) {
		return authCodingPlan
	}
	return s.authModes[s.authModeIndex]
}
