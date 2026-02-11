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
	b.WriteString(fmt.Sprintf("  %s  send message\n", helpKeyStyle.Render("enter")))
	b.WriteString(fmt.Sprintf("  %s  browse sent messages\n", helpKeyStyle.Render("up/down")))
	b.WriteString(fmt.Sprintf("  %s  switch mode\n", helpKeyStyle.Render("shift+tab")))
	b.WriteString(fmt.Sprintf("  %s  clear chat\n", helpKeyStyle.Render("x")))
	b.WriteString(fmt.Sprintf("  %s  quit\n", helpKeyStyle.Render("shift+q")))

	b.WriteString("\n")

	b.WriteString(helpSectionStyle.Render("modes"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  plan - planning and analysis"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  create - create files / execute actions"))
	b.WriteString("\n")

	b.WriteString("\n")

	b.WriteString(helpSectionStyle.Render("setup"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  /connect  configure api key"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  /resume   pick a previous session"))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(helpFooterStyle.Render("shift+q quit | shift+tab mode | enter send"))

	return b.String()
}

type keyMap struct {
	Quit            key.Binding
	Enter           key.Binding
	Clear           key.Binding
	NextMode        key.Binding
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
			key.WithKeys("x", "ctrl+l"),
			key.WithHelp("x/ctrl+l", "clear chat"),
		),
		NextMode: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "switch mode"),
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
	return []key.Binding{k.Enter, k.Clear, k.NextMode, k.Expand, k.ToggleBanner, k.ToggleVerbosity, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter, k.Clear, k.NextMode, k.Quit},
	}
}

// Minimal transparent styles
var (
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFB86C"))

	helpSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#BD93F9"))

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF79C6"))

	helpDescriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6272A4"))

	helpFooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#44475A")).
			Italic(true)
)
