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

	b.WriteString(helpTitleStyle.Render("CLI Agent Help"))
	b.WriteString("\n\n")

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

	b.WriteString(helpSectionStyle.Render("Modes"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(" %s %s Cycle through modes (plan/code/act)\n",
		helpKeyStyle.Render(m.keys.NextMode.Help().Key),
		helpDescriptionStyle.Render("Switch mode")))
	b.WriteString(fmt.Sprintf(" Current mode: %s\n",
		helpDescriptionStyle.Render("plan | code | act")))

	b.WriteString("\n")

	b.WriteString(helpSectionStyle.Render("Configuration"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(" %s %s Configure API provider\n",
		helpKeyStyle.Render("/connect"),
		helpDescriptionStyle.Render("Open setup wizard for MiniMax API key")))
	b.WriteString(helpDescriptionStyle.Render("   • Choose provider (MiniMax)"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("   • Enter API key"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("   • Select model (minimax-m2.1)"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("   • Config saved locally to settings.json"))
	b.WriteString("\n")

	b.WriteString("\n")

	b.WriteString(helpSectionStyle.Render("Input Tips"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("• Type your message and press Enter to send"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("• Use Shift+Enter for new lines"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("• Character limit: 8000 characters"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("• Type /connect to configure your API key"))
	b.WriteString("\n")

	b.WriteString("\n")

	b.WriteString(helpSectionStyle.Render("About"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("A modern AI CLI agent with enhanced TUI interface"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("Features markdown rendering with syntax highlighting"))
	b.WriteString("\n")
	b.WriteString(helpDescriptionStyle.Render("and smooth animations for better user experience."))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(helpFooterStyle.Render("Press q to quit | Shift+Tab to switch mode | Enter to send"))

	return helpStyle.Width(m.width).Render(b.String())
}

type keyMap struct {
	Quit     key.Binding
	Enter    key.Binding
	Clear    key.Binding
	NextMode key.Binding
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
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Clear, k.NextMode, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Enter, k.Clear, k.NextMode, k.Quit},
	}
}

var (
	helpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderBottomForeground(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginBottom(1)

	helpSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#4ECDC4")).
				Background(lipgloss.Color("#1A1A2E")).
				Width(80)

	helpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color("#1A1A2E")).
			Padding(0, 2).
			Width(15)

	helpDescriptionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E0E0E0")).
				Background(lipgloss.Color("#1A1A2E")).
				Width(60)

	helpFooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Background(lipgloss.Color("#1A1A2E")).
			Italic(true).
			Width(80)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Background(lipgloss.Color("#0F172A")).
			Padding(0, 3).
			Width(80)
)
