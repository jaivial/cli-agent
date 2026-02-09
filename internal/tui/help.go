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
	b.WriteString(fmt.Sprintf("  %s  run agent / send\n", helpKeyStyle.Render("enter")))
	b.WriteString(fmt.Sprintf("  %s  focus next pane\n", helpKeyStyle.Render("tab")))
	b.WriteString(fmt.Sprintf("  %s  toggle trace\n", helpKeyStyle.Render("ctrl+t")))
	b.WriteString(fmt.Sprintf("  %s  cancel run\n", helpKeyStyle.Render("ctrl+c")))
	b.WriteString(fmt.Sprintf("  %s  switch mode\n", helpKeyStyle.Render("shift+tab")))
	b.WriteString(fmt.Sprintf("  %s  clear chat\n", helpKeyStyle.Render("x")))
	b.WriteString(fmt.Sprintf("  %s  quit\n", helpKeyStyle.Render("q")))

	b.WriteString("\n")

	b.WriteString(helpSectionStyle.Render("modes"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  plan - planning and analysis"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  code - code generation"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  do   - execute actions"))
	b.WriteString("\n")

	b.WriteString("\n")

	b.WriteString(helpSectionStyle.Render("setup"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("  /connect  configure api key"))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(helpFooterStyle.Render("tab focus | ctrl+t trace | ctrl+c cancel | shift+tab mode | q quit"))

	return b.String()
}

type keyMap struct {
	Quit     key.Binding
	Enter    key.Binding
	Clear    key.Binding
	NextMode key.Binding
	ToggleTrace key.Binding
	FocusNext   key.Binding
	Cancel      key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
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
		ToggleTrace: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "toggle trace"),
		),
		FocusNext: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "focus next"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "cancel run"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.FocusNext, k.ToggleTrace, k.Cancel, k.Clear, k.NextMode, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter, k.FocusNext, k.ToggleTrace, k.Cancel},
		{k.Clear, k.NextMode, k.Quit},
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
