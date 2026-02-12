package tui

import (
	"strings"

	"cli-agent/internal/app"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

type slashPopupItem struct {
	Label       string
	Description string
	InsertText  string
}

func (m *MainModel) updateSlashPopupState() {
	key, items := m.slashPopupState()
	if key != m.slashPopupKey {
		m.slashPopupKey = key
		m.slashPopupIndex = 0
	}
	if len(items) == 0 {
		m.slashPopupIndex = 0
		return
	}
	if m.slashPopupIndex < 0 {
		m.slashPopupIndex = 0
	}
	if m.slashPopupIndex >= len(items) {
		m.slashPopupIndex = len(items) - 1
	}
}

func (m *MainModel) slashPopupItems() []slashPopupItem {
	_, items := m.slashPopupState()
	return items
}

func (m *MainModel) slashPopupState() (key string, items []slashPopupItem) {
	if m.modelPickerActive || m.permissionsPickerActive || m.resumePickerActive || m.planDecisionActive {
		return "", nil
	}

	raw := strings.TrimLeft(m.input.Value(), " \t")
	if raw == "" || !strings.HasPrefix(raw, "/") {
		return "", nil
	}
	if strings.Contains(raw, "\n") || strings.Contains(raw, "\r") {
		return "", nil
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return "", nil
	}
	hasSpace := strings.ContainsAny(raw, " \t")
	endsWithSpace := strings.HasSuffix(raw, " ") || strings.HasSuffix(raw, "\t")
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", nil
	}

	cmdToken := parts[0]
	if cmdToken == "/" {
		cmdToken = ""
	}

	baseCommands := []slashPopupItem{
		{Label: "/new", Description: "start a new session", InsertText: "/new"},
		{Label: "/clear", Description: "clear chat (alias: /new)", InsertText: "/clear"},
		{Label: "/connect", Description: "setup provider + API key", InsertText: "/connect"},
		{Label: "/model", Description: "choose model", InsertText: "/model"},
		{Label: "/resume", Description: "resume a previous session", InsertText: "/resume"},
		{Label: "/permissions", Description: "show/set permissions mode", InsertText: "/permissions"},
	}

	if len(parts) == 1 && !hasSpace {
		prefix := strings.ToLower(cmdToken)
		key = "cmd:" + prefix
		for _, cmd := range baseCommands {
			if strings.HasPrefix(strings.ToLower(cmd.Label), prefix) {
				items = append(items, cmd)
			}
		}
		return key, items
	}

	if strings.EqualFold(cmdToken, "/permissions") && (len(parts) == 2 || (len(parts) == 1 && endsWithSpace)) {
		argPrefix := ""
		if len(parts) == 2 {
			argPrefix = parts[1]
		}
		argPrefix = strings.ToLower(strings.TrimSpace(argPrefix))
		key = "perm:" + argPrefix

		opts := []struct {
			value string
			desc  string
		}{
			{value: app.PermissionsFullAccess, desc: "default"},
			{value: app.PermissionsDangerouslyFullAccess, desc: "requires root"},
		}
		for _, opt := range opts {
			if argPrefix == "" || strings.HasPrefix(strings.ToLower(opt.value), argPrefix) {
				items = append(items, slashPopupItem{
					Label:       opt.value,
					Description: opt.desc,
					InsertText:  "/permissions " + opt.value,
				})
			}
		}
		return key, items
	}

	return "", nil
}

func (m *MainModel) renderSlashPopup() string {
	items := m.slashPopupItems()
	if len(items) == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent2)).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFg))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))

	idx := m.slashPopupIndex
	if idx < 0 || idx >= len(items) {
		idx = 0
	}

	width := m.chatAreaWidth() - 2
	if width < 24 {
		width = m.chatAreaWidth()
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("commands"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/↓ select • enter complete"))
	b.WriteString("\n")

	labelW := 14
	if width > 0 && labelW > width/2 {
		labelW = width / 2
	}
	if labelW < 10 {
		labelW = 10
	}

	for i, item := range items {
		prefix := "  "
		style := labelStyle
		if i == idx {
			prefix = "› "
			style = activeStyle
		}
		label := truncate.StringWithTail(item.Label, uint(labelW), "…")
		descW := width - 4 - labelW
		if descW < 0 {
			descW = 0
		}
		desc := truncate.StringWithTail(item.Description, uint(descW), "…")
		line := prefix + style.Render(label)
		if strings.TrimSpace(desc) != "" {
			line += " " + descStyle.Render(desc)
		}
		b.WriteString(line)
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(0, 1).
		Width(width).
		Render(b.String())
}
