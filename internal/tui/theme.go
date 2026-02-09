package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

type ThemeName string

const (
	ThemePorcelain ThemeName = "porcelain"
	ThemeMidnight  ThemeName = "midnight"
)

type Theme struct {
	Name ThemeName

	// Colors
	TextPrimary lipgloss.AdaptiveColor
	TextMuted   lipgloss.AdaptiveColor
	TextFaint   lipgloss.AdaptiveColor

	Accent   lipgloss.AdaptiveColor
	Accent2  lipgloss.AdaptiveColor
	Success  lipgloss.AdaptiveColor
	Warn     lipgloss.AdaptiveColor
	Error    lipgloss.AdaptiveColor
	Border   lipgloss.AdaptiveColor
	BorderHi lipgloss.AdaptiveColor

	// Styles
	TopBar      lipgloss.Style
	TopBarTitle lipgloss.Style
	TopBarBadge lipgloss.Style
	TopBarMeta  lipgloss.Style

	Pane         lipgloss.Style
	PaneFocused  lipgloss.Style
	PaneTitle    lipgloss.Style
	PaneTitleF   lipgloss.Style
	PaneDivider  lipgloss.Style
	Footer       lipgloss.Style
	InputBox     lipgloss.Style
	InputBoxF    lipgloss.Style
	Spinner      lipgloss.Style

	RoleYou lipgloss.Style
	RoleAI  lipgloss.Style
	RoleSys lipgloss.Style
	RoleErr lipgloss.Style

	TraceOK     lipgloss.Style
	TraceERR    lipgloss.Style
	TraceNeutral lipgloss.Style
	TraceSel    lipgloss.Style
}

func NewTheme() Theme {
	name := ThemeName(os.Getenv("EAI_THEME"))
	if name == "" {
		// Default. This is intentionally calmer than the previous Dracula-like palette.
		name = ThemePorcelain
	}

	noColor := os.Getenv("EAI_NO_COLOR") == "1"
	if noColor {
		return NewNoColorTheme()
	}

	switch name {
	case ThemeMidnight:
		return newMidnightTheme()
	default:
		return newPorcelainTheme()
	}
}

func NewNoColorTheme() Theme {
	t := Theme{
		Name:        "no-color",
		TextPrimary: lipgloss.AdaptiveColor{Light: "#000000", Dark: "#ffffff"},
		TextMuted:   lipgloss.AdaptiveColor{Light: "#333333", Dark: "#dddddd"},
		TextFaint:   lipgloss.AdaptiveColor{Light: "#555555", Dark: "#bbbbbb"},
		Border:      lipgloss.AdaptiveColor{Light: "#555555", Dark: "#777777"},
		BorderHi:    lipgloss.AdaptiveColor{Light: "#000000", Dark: "#ffffff"},
	}
	t.TopBar = lipgloss.NewStyle().Foreground(t.TextPrimary)
	t.TopBarTitle = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.TopBarBadge = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.TopBarMeta = lipgloss.NewStyle().Foreground(t.TextMuted)

	t.Pane = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.Border).Padding(0, 1)
	t.PaneFocused = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.BorderHi).Padding(0, 1)
	t.PaneTitle = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.PaneTitleF = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.PaneDivider = lipgloss.NewStyle().Foreground(t.Border)
	t.Footer = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.InputBox = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.Border).Padding(0, 1)
	t.InputBoxF = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.BorderHi).Padding(0, 1)
	t.Spinner = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)

	t.RoleYou = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.RoleAI = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.RoleSys = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.RoleErr = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)

	t.TraceOK = lipgloss.NewStyle().Foreground(t.TextPrimary)
	t.TraceERR = lipgloss.NewStyle().Foreground(t.TextPrimary)
	t.TraceNeutral = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.TraceSel = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	return t
}

func newPorcelainTheme() Theme {
	t := Theme{
		Name:        ThemePorcelain,
		TextPrimary: lipgloss.AdaptiveColor{Light: "#1d2433", Dark: "#f2f2f2"},
		TextMuted:   lipgloss.AdaptiveColor{Light: "#4a5568", Dark: "#c7c7c7"},
		TextFaint:   lipgloss.AdaptiveColor{Light: "#718096", Dark: "#9aa0a6"},

		Accent:   lipgloss.AdaptiveColor{Light: "#1f6feb", Dark: "#7aa2ff"},
		Accent2:  lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#f4b27d"},
		Success:  lipgloss.AdaptiveColor{Light: "#0f766e", Dark: "#46d1b7"},
		Warn:     lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#f4b27d"},
		Error:    lipgloss.AdaptiveColor{Light: "#b42318", Dark: "#ff7a7a"},
		Border:   lipgloss.AdaptiveColor{Light: "#cbd5e0", Dark: "#3a3a3a"},
		BorderHi: lipgloss.AdaptiveColor{Light: "#1f6feb", Dark: "#7aa2ff"},
	}

	t.TopBar = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.TopBarTitle = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.TopBarBadge = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.TopBarMeta = lipgloss.NewStyle().Foreground(t.TextMuted)

	t.Pane = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.Border).Padding(0, 1)
	t.PaneFocused = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.BorderHi).Padding(0, 1)
	t.PaneTitle = lipgloss.NewStyle().Bold(true).Foreground(t.TextMuted)
	t.PaneTitleF = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.PaneDivider = lipgloss.NewStyle().Foreground(t.Border)
	t.Footer = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.InputBox = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.Border).Padding(0, 1)
	t.InputBoxF = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.BorderHi).Padding(0, 1)
	t.Spinner = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)

	t.RoleYou = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.RoleAI = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.RoleSys = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.RoleErr = lipgloss.NewStyle().Bold(true).Foreground(t.Error)

	t.TraceOK = lipgloss.NewStyle().Foreground(t.Success)
	t.TraceERR = lipgloss.NewStyle().Foreground(t.Error)
	t.TraceNeutral = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.TraceSel = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	return t
}

func newMidnightTheme() Theme {
	t := Theme{
		Name:        ThemeMidnight,
		TextPrimary: lipgloss.AdaptiveColor{Light: "#111827", Dark: "#eaeaea"},
		TextMuted:   lipgloss.AdaptiveColor{Light: "#374151", Dark: "#b7b7b7"},
		TextFaint:   lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#8d8d8d"},

		Accent:   lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#7aa2ff"},
		Accent2:  lipgloss.AdaptiveColor{Light: "#0ea5e9", Dark: "#5cc8ff"},
		Success:  lipgloss.AdaptiveColor{Light: "#059669", Dark: "#46d1b7"},
		Warn:     lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#f4b27d"},
		Error:    lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#ff7a7a"},
		Border:   lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#2a2a2a"},
		BorderHi: lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#7aa2ff"},
	}

	t.TopBar = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.TopBarTitle = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.TopBarBadge = lipgloss.NewStyle().Bold(true).Foreground(t.Accent2)
	t.TopBarMeta = lipgloss.NewStyle().Foreground(t.TextMuted)

	t.Pane = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.Border).Padding(0, 1)
	t.PaneFocused = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.BorderHi).Padding(0, 1)
	t.PaneTitle = lipgloss.NewStyle().Bold(true).Foreground(t.TextMuted)
	t.PaneTitleF = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.PaneDivider = lipgloss.NewStyle().Foreground(t.Border)
	t.Footer = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.InputBox = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.Border).Padding(0, 1)
	t.InputBoxF = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(t.BorderHi).Padding(0, 1)
	t.Spinner = lipgloss.NewStyle().Bold(true).Foreground(t.Accent2)

	t.RoleYou = lipgloss.NewStyle().Bold(true).Foreground(t.Accent2)
	t.RoleAI = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	t.RoleSys = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.RoleErr = lipgloss.NewStyle().Bold(true).Foreground(t.Error)

	t.TraceOK = lipgloss.NewStyle().Foreground(t.Success)
	t.TraceERR = lipgloss.NewStyle().Foreground(t.Error)
	t.TraceNeutral = lipgloss.NewStyle().Foreground(t.TextMuted)
	t.TraceSel = lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
	return t
}

