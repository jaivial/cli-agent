package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// helpModel represents the help screen state
type helpModel struct {
	keys    keyMap
	width   int
}

// newHelpModel creates a new help model
func newHelpModel() helpModel {
	return helpModel{
		keys:    defaultKeyMap(),
		width:   80,
	}
}

// SetWidth updates the help screen width
func (m *helpModel) SetWidth(width int) {
	m.width = width
}

// View renders the help screen
func (m helpModel) View() string {
	var b strings.Builder

	// Help title
	b.WriteString(helpTitleStyle.Render("CLI Agent Help"))
	b.WriteString("\n\n")

	// Main commands
	b.WriteString(helpSectionStyle.Render("Main Commands"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(" %s %s Send message\n",
		helpKeyStyle.Render(m.keys.Enter.Help().Key),
		helpDescriptionStyle.Render("Send message")))
	b.WriteString(fmt.Sprintf(" %s %s Clear chat history\n",
		helpKeyStyle.Render(m.keys.Clear.Help().Key),
		helpDescriptionStyle.Render("Clear chat history")))
	b.WriteString(fmt.Sprintf(" %s %s Quit application\n",
		helpKeyStyle.Render(m.keys.Quit.Help().Key),
		helpDescriptionStyle.Render("Quit application")))

	b.WriteString("\n")

	// Input tips
	b.WriteString(helpSectionStyle.Render("Input Tips"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("• Type your message and press Enter to send"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("• Use Shift+Enter for new lines"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("• Character limit: 8000 characters"))
	b.WriteString("\n")

	b.WriteString("\n")

	// Current mode
	b.WriteString(helpSectionStyle.Render("About"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("A modern AI CLI agent with enhanced TUI interface"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("Features markdown rendering with syntax highlighting"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("and smooth animations for better user experience."))
	b.WriteString("\n")

	// Footer with tips
	b.WriteString("\n")
	b.WriteString(helpFooterStyle.Render("Press q to quit | Type your message below"))

	return helpStyle.Width(m.width).Render(b.String())
}

// keyMap represents the keyboard bindings
type keyMap struct {
	Quit  key.Binding
	Enter key.Binding
	Clear key.Binding
}

// defaultKeyMap creates a new default key map
func defaultKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c", "ctrl+l"),
			key.WithHelp("c/ctrl+l", "clear chat"),
		),
	}
}

// ShortHelp returns the short help text for the key map
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Clear, k.Quit}
}

// FullHelp returns the full help text for the key map
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter, k.Clear, k.Quit},
	}
}

// Styles for help screen
var (
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPrimary)).
			Background(lipgloss.Color(colorBg)).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderBottomForeground(lipgloss.Color(colorBorder)).
			Padding(0, 1).
			MarginBottom(1)

	helpSectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorSecondary)).
			Background(lipgloss.Color(colorBg)).
			Width(80)

	helpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorAccent)).
			Background(lipgloss.Color(colorBg)).
			Padding(0, 2).
			Width(15)

	helpDescriptionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg)).
			Background(lipgloss.Color(colorBg)).
			Width(60)

	helpFooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFgMuted)).
			Background(lipgloss.Color(colorBg)).
			Italic(true).
			Width(80)
)
