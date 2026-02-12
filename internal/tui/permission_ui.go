package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

const (
	permissionDecisionAllow = 0
	permissionDecisionDeny  = 1
)

func (m *MainModel) permissionDecisionHeight() int {
	if !m.permissionDecisionActive {
		return 0
	}
	h := lipgloss.Height(m.renderPermissionDecision())
	if h < 0 {
		return 0
	}
	return h
}

func (m *MainModel) renderPermissionDecision() string {
	if !m.permissionDecisionActive {
		return ""
	}

	width := m.chatAreaWidth() - 2
	if width < 30 {
		width = 30
		if maxWidth := m.chatAreaWidth() - 2; width > maxWidth {
			width = maxWidth
		}
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent2)).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFg))
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))

	selected := m.permissionDecisionChoice
	if selected != permissionDecisionDeny {
		selected = permissionDecisionAllow
	}

	row := func(idx int, text string) string {
		prefix := "  "
		style := rowStyle
		if idx == selected {
			prefix = "› "
			style = activeStyle
		}
		return style.Render(prefix + text)
	}

	action := strings.TrimSpace(m.pendingPermissionText)
	if action == "" {
		action = "EAI agent requests permission."
	}
	action = truncate.StringWithTail(action, uint(width), "…")

	var b strings.Builder
	b.WriteString(titleStyle.Render("Permission approval"))
	b.WriteString("\n")
	b.WriteString(metaStyle.Render(action))
	b.WriteString("\n\n")
	b.WriteString(row(permissionDecisionAllow, "1. Yes, allow"))
	b.WriteString("\n")
	b.WriteString(row(permissionDecisionDeny, "2. Don't allow (stop agent)"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/↓ choose  •  enter confirm  •  esc deny"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(0, 1).
		Width(width).
		Render(b.String())
	return box
}
