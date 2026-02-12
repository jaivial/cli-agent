package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

type helpModel struct {
	keys  keyMap
	width int
}

func newHelpModel() helpModel {
	return helpModel{
		keys:  defaultKeyMap(),
		width: 80,
	}
}

func (m *helpModel) SetWidth(width int) {
	m.width = width
}

func (m helpModel) View() string {
	var b strings.Builder

	b.WriteString(helpTitleStyle.Render("eai help"))
	b.WriteString("\n\n")

	b.WriteString(helpSectionStyle.Render("commands"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  send message (queues while running)\n", helpKeyStyle.Render("enter")))
	b.WriteString(fmt.Sprintf("  %s  cancel running task\n", helpKeyStyle.Render("esc")))
	b.WriteString(fmt.Sprintf("  %s  browse sent messages\n", helpKeyStyle.Render("up/down")))
	b.WriteString(fmt.Sprintf("  %s  new chat (clear)\n", helpKeyStyle.Render("/new or /clear")))
	b.WriteString(fmt.Sprintf("  %s  quit\n", helpKeyStyle.Render("shift+q")))

	b.WriteString("\n")

	b.WriteString(helpSectionStyle.Render("execution"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  each message runs the same tool-driven agent as `eai agent \"...\"`"))
	b.WriteString("\n\n")

	b.WriteString(helpSectionStyle.Render("setup"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  /connect  choose provider + auth mode and set EAI_API_KEY"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  /model    choose model (glm-4.7|glm-5)"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  /resume   pick a previous session"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  /permissions  show or set permissions mode"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  /logs    show recent warn/error logs"))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(helpFooterStyle.Render("shift+q quit | esc cancel | enter send"))

	return b.String()
}

type keyMap struct {
	Quit            key.Binding
	Enter           key.Binding
	Clear           key.Binding
	Expand          key.Binding
	ToggleBanner    key.Binding
	ToggleVerbosity key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("Q", "ctrl+c"),
			key.WithHelp("shift+q", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "clear chat"),
		),
		Expand: key.NewBinding(
			key.WithKeys("alt+e"),
			key.WithHelp("alt+e", "expand last long reply"),
		),
		ToggleBanner: key.NewBinding(
			key.WithKeys("alt+b"),
			key.WithHelp("alt+b", "toggle banner"),
		),
		ToggleVerbosity: key.NewBinding(
			key.WithKeys("alt+v"),
			key.WithHelp("alt+v", "cycle verbosity"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Clear, k.Expand, k.ToggleBanner, k.ToggleVerbosity, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter, k.Clear, k.Quit},
	}
}

// Minimal transparent styles
var (
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorAccent))

	helpSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorAccent2))

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent))

	helpDescriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorMuted))

	helpFooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSubtle)).
			Italic(true)
)
