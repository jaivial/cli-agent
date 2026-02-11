package tui

import (
	"regexp"
	"strings"

	"cli-agent/internal/app"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

var (
	proposedPlanBlockRe = regexp.MustCompile(`(?is)<proposed_plan>\s*(.*?)\s*</proposed_plan>`)
	planHeadingRe       = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)
	planListItemRe      = regexp.MustCompile(`^\s*(?:[-*+]|(?:\d+[.)]))\s+(.+)$`)
	planCheckboxRe      = regexp.MustCompile(`^\s*\[(?: |x|X)\]\s+(.+)$`)
)

const (
	planDecisionYes = 0
	planDecisionNo  = 1
)

func (m *MainModel) planDecisionHeight() int {
	if !m.planDecisionActive {
		return 0
	}
	h := lipgloss.Height(m.renderPlanDecision())
	if h < 0 {
		return 0
	}
	return h
}

func (m *MainModel) renderPlanDecision() string {
	if !m.planDecisionActive {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB86C"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))

	selected := m.planDecisionChoice
	if selected != planDecisionNo {
		selected = planDecisionYes
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

	var b strings.Builder
	b.WriteString(titleStyle.Render("Implement this plan?"))
	b.WriteString("\n")
	b.WriteString(row(planDecisionYes, "1. Yes, implement this plan  Switch to Default and start coding."))
	b.WriteString("\n")
	b.WriteString(row(planDecisionNo, "2. No, stay in Plan mode     Continue planning with the model."))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/↓ choose  •  enter confirm  •  esc cancel"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#44475A")).
		Padding(0, 1).
		Width(m.chatAreaWidth() - 2).
		Render(b.String())
	return box
}

func buildPlanDisplayIfApplicable(mode app.Mode, raw string) (display string, planText string, ok bool) {
	if mode != app.ModePlan {
		return "", "", false
	}
	planBody, found := extractPlanBody(raw)
	if !found {
		return "", "", false
	}
	title, checklist := normalizePlanBody(planBody)
	if len(checklist) == 0 {
		return "", "", false
	}
	lead := choosePlanLead(planBody)

	lines := make([]string, 0, len(checklist)+4)
	lines = append(lines, lead)
	if title != "" {
		lines = append(lines, title)
	}
	lines = append(lines, "")
	lines = append(lines, checklist...)
	planText = strings.Join(checklist, "\n")
	if title != "" {
		planText = title + "\n" + planText
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), strings.TrimSpace(planText), true
}

func extractPlanBody(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	if m := proposedPlanBlockRe.FindStringSubmatch(raw); len(m) >= 2 {
		body := strings.TrimSpace(m[1])
		return body, body != ""
	}

	lines := strings.Split(raw, "\n")
	listCount := 0
	lower := strings.ToLower(raw)
	hasPlanCue := strings.Contains(lower, "plan") || strings.Contains(lower, "roadmap") || strings.Contains(lower, "implementation")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if planListItemRe.MatchString(line) || planCheckboxRe.MatchString(line) {
			listCount++
		}
	}
	if hasPlanCue && listCount >= 2 {
		return raw, true
	}
	return "", false
}

func normalizePlanBody(body string) (title string, checklist []string) {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	checklist = make([]string, 0, len(lines))

	for _, line := range lines {
		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}

		if m := planCheckboxRe.FindStringSubmatch(raw); len(m) >= 2 {
			item := sanitizePlanLine(m[1])
			if item != "" {
				checklist = append(checklist, "[ ] "+item)
			}
			continue
		}

		if m := planListItemRe.FindStringSubmatch(raw); len(m) >= 2 {
			item := sanitizePlanLine(m[1])
			if item != "" {
				checklist = append(checklist, "[ ] "+item)
			}
			continue
		}

		clean := sanitizePlanLine(raw)
		if clean == "" {
			continue
		}
		if title == "" {
			title = clean
		}
	}

	return strings.TrimSpace(title), checklist
}

func sanitizePlanLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	line = strings.TrimSpace(planHeadingRe.ReplaceAllString(line, ""))
	replacer := strings.NewReplacer(
		"**", "",
		"__", "",
		"`", "",
		"<proposed_plan>", "",
		"</proposed_plan>", "",
	)
	line = replacer.Replace(line)
	line = strings.TrimSpace(line)
	line = strings.Join(strings.Fields(line), " ")
	return strings.TrimSpace(line)
}

func choosePlanLead(seed string) string {
	variants := []string{
		"Plan ready. Here is what we should execute:",
		"I drafted this plan:",
		"Proposed implementation plan:",
		"This is the plan to run:",
	}
	if len(variants) == 0 {
		return "Plan:"
	}
	sum := 0
	for _, r := range seed {
		sum += int(r)
	}
	return variants[sum%len(variants)]
}

func renderPlanContent(content string, width int) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB86C"))
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2"))
	checkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD"))

	out := make([]string, 0, len(lines))
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, "")
			continue
		}
		if idx == 0 {
			wrapped := wrapPreservingNewlines(titleStyle.Render(trimmed), width)
			out = append(out, wrapped)
			continue
		}
		if strings.HasPrefix(trimmed, "[ ] ") {
			entry := strings.TrimSpace(strings.TrimPrefix(trimmed, "[ ] "))
			row := checkStyle.Render("[ ] ") + lineStyle.Render(entry)
			out = append(out, wrapPreservingNewlines(row, width))
			continue
		}
		out = append(out, wrapPreservingNewlines(lineStyle.Render(trimmed), width))
	}
	return strings.Join(out, "\n")
}

func buildPlanImplementationPrompt(planText string, planContext string) string {
	planText = strings.TrimSpace(planText)
	planContext = strings.TrimSpace(planContext)

	var b strings.Builder
	b.WriteString("Implement the approved plan end-to-end. Follow the plan checklist as the source of truth and verify your changes.\n")
	b.WriteString("CRITICAL: Preserve concrete facts discovered in Plan mode (paths, filenames, constraints). Do not restart discovery unless necessary.\n")
	b.WriteString("If you use the `exec` tool, prefer setting its `cwd` to the target directory discovered in Plan mode instead of assuming the current working directory is correct.\n")

	if planContext != "" {
		b.WriteString("\nPlan-mode context (carry this forward exactly):\n")
		b.WriteString(planContext)
		b.WriteString("\n")
	}

	if planText != "" {
		b.WriteString("\nApproved plan checklist:\n\n")
		b.WriteString(planText)
		return strings.TrimSpace(b.String())
	}

	return strings.TrimSpace(b.String())
}

func wrapPreservingNewlines(content string, width int) string {
	if width < 8 {
		return content
	}
	parts := strings.Split(content, "\n")
	for i := range parts {
		parts[i] = truncate.StringWithTail(parts[i], uint(width*4), "…")
	}
	return strings.Join(parts, "\n")
}
