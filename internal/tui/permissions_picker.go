package tui

import (
	"fmt"
	"strings"
	"time"

	"cli-agent/internal/app"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

func (m *MainModel) openPermissionsPicker() {
	m.permissionsPickerActive = true

	desired := app.PermissionsFullAccess
	if m.app != nil {
		desired = app.NormalizePermissionsMode(m.app.Config.Permissions)
	}

	m.permissionsPickerIndex = 0
	for i, option := range m.permissionsOptions {
		if strings.EqualFold(strings.TrimSpace(option), desired) {
			m.permissionsPickerIndex = i
			break
		}
	}
}

func (m *MainModel) closePermissionsPicker() {
	m.permissionsPickerActive = false
	m.input.Focus()
}

func (m *MainModel) selectPermissionsAt(index int) string {
	if index < 0 || index >= len(m.permissionsOptions) {
		return ""
	}
	mode := app.NormalizePermissionsMode(m.permissionsOptions[index])
	if m.app == nil {
		return mode
	}

	cfg := m.app.Config
	cfg.Permissions = mode
	if err := app.SaveConfig(cfg, app.DefaultConfigPath()); err != nil {
		m.messages = append(m.messages, Message{
			ID:        fmt.Sprintf("error-%d", time.Now().UnixNano()),
			Role:      "error",
			Content:   fmt.Sprintf("failed to save permissions: %v", err),
			Timestamp: time.Now(),
		})
		return ""
	}
	m.app.Config.Permissions = mode
	return mode
}

func (m *MainModel) renderPermissionsPicker() string {
	if !m.permissionsPickerActive || len(m.permissionsOptions) == 0 {
		return ""
	}

	desired := app.PermissionsFullAccess
	effective := app.PermissionsFullAccess
	isRoot := false
	if m.app != nil {
		desired = app.NormalizePermissionsMode(m.app.Config.Permissions)
		effective, isRoot = app.EffectivePermissionsMode(desired)
	}

	width := m.chatAreaWidth() - 2
	if width < 30 {
		width = m.chatAreaWidth()
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent2)).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFg))
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))

	var b strings.Builder
	b.WriteString(titleStyle.Render("permissions"))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/↓ select • enter apply • esc cancel"))
	b.WriteString("\n\n")

	statusLine := fmt.Sprintf("desired: %s  ·  effective: %s", desired, effective)
	if isRoot {
		statusLine += "  ·  root: yes"
	} else {
		statusLine += "  ·  root: no"
	}
	b.WriteString(metaStyle.Render(truncate.StringWithTail(statusLine, uint(width), "…")))
	b.WriteString("\n\n")

	idx := m.permissionsPickerIndex
	if idx < 0 || idx >= len(m.permissionsOptions) {
		idx = 0
	}

	for i, opt := range m.permissionsOptions {
		line := "  " + opt
		style := rowStyle
		if i == idx {
			line = "› " + opt
			style = activeStyle
		}
		b.WriteString(style.Render(line))
		if i < len(m.permissionsOptions)-1 {
			b.WriteString("\n")
		}
	}

	if !isRoot {
		b.WriteString("\n\n")
		b.WriteString(metaStyle.Render("note: `dangerously-full-access` only takes effect when running as root"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(0, 1).
		Width(width).
		Render(b.String())
}

