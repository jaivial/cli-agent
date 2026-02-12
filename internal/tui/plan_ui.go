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
	planCheckboxRe      = regexp.MustCompile(`^\s*(?:[-*+]\s*)?\[(?: |x|X)\]\s+(.+)$`)
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

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent2)).Bold(true)
	rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFg))

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
	b.WriteString(row(planDecisionYes, "1. Yes, implement this plan  Switch to Create and start coding."))
	b.WriteString("\n")
	b.WriteString(row(planDecisionNo, "2. No, stay in Plan mode     Continue planning with the model."))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("↑/↓ choose  •  enter confirm  •  esc cancel"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorBorder)).
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
	displayLines, checklist := normalizePlanBody(planBody)
	if len(checklist) == 0 {
		return "", "", false
	}

	display = strings.TrimSpace(strings.Join(displayLines, "\n"))
	if display == "" {
		return "", "", false
	}

	// Carry the full plan body forward for execution, not just extracted checklist lines.
	planText = strings.TrimSpace(planBody)
	return display, planText, true
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

func normalizePlanBody(body string) (displayLines []string, checklist []string) {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	checklist = make([]string, 0, len(lines))

	type sectionKind string
	const (
		sectionUnknown      sectionKind = ""
		sectionSummary      sectionKind = "summary"
		sectionAssumptions  sectionKind = "assumptions"
		sectionPlan         sectionKind = "plan"
		sectionVerification sectionKind = "verification"
	)
	currentSection := sectionUnknown

	isHeading := func(line string) bool {
		return planHeadingRe.MatchString(line)
	}
	headingLevel := func(line string) int {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			return 0
		}
		level := 0
		for i := 0; i < len(line) && line[i] == '#'; i++ {
			level++
		}
		if level > 6 {
			level = 6
		}
		return level
	}
	headingText := func(line string) string {
		return strings.ToLower(strings.TrimSpace(sanitizePlanLine(line)))
	}

	hasExplicitSections := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if isHeading(trim) && headingLevel(trim) >= 2 {
			hasExplicitSections = true
			break
		}
	}

	for _, line := range lines {
		raw := strings.TrimRight(line, " \t")
		trim := strings.TrimSpace(raw)
		if trim == "" {
			// Preserve intentional blank lines for readability.
			displayLines = append(displayLines, "")
			continue
		}

		if isHeading(trim) {
			level := headingLevel(trim)
			h := headingText(trim)
			if level >= 2 {
				switch {
				case strings.Contains(h, "summary"):
					currentSection = sectionSummary
				case strings.Contains(h, "assumption"):
					currentSection = sectionAssumptions
				case strings.Contains(h, "verification") || strings.Contains(h, "verify"):
					currentSection = sectionVerification
				case strings.Contains(h, "plan") || strings.Contains(h, "steps"):
					currentSection = sectionPlan
				default:
					currentSection = sectionUnknown
				}
			}
			displayLines = append(displayLines, sanitizePlanLine(trim))
			continue
		}

		// Checkbox item (preferred)
		if m := planCheckboxRe.FindStringSubmatch(trim); len(m) >= 2 {
			item := sanitizePlanLine(m[1])
			if item == "" {
				continue
			}
			checklist = append(checklist, "- [ ] "+item)
			displayLines = append(displayLines, "[ ] "+item)
			continue
		}

		// Plain list item: render as bullet in Summary/Assumptions, checkbox in Plan/Verification.
		if m := planListItemRe.FindStringSubmatch(trim); len(m) >= 2 {
			item := sanitizePlanLine(m[1])
			if item == "" {
				continue
			}
			if !hasExplicitSections || currentSection == sectionPlan || currentSection == sectionVerification {
				checklist = append(checklist, "- [ ] "+item)
				displayLines = append(displayLines, "[ ] "+item)
			} else {
				displayLines = append(displayLines, "• "+item)
			}
			continue
		}

		// Plain line.
		clean := sanitizePlanLine(trim)
		if clean != "" {
			displayLines = append(displayLines, clean)
		}
	}

	// Require at least a couple actionable steps (checkboxes or inferred plan/verification items).
	if len(checklist) == 0 {
		return nil, nil
	}
	return displayLines, checklist
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

func renderPlanContent(content string, width int) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorAccent))
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFg))
	checkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent2))

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
	b.WriteString("Implement the approved plan end-to-end. Treat the approved plan as the source of truth and verify your changes.\n")
	b.WriteString("CRITICAL: Preserve concrete facts discovered in Plan mode (paths, filenames, constraints). Do not restart discovery unless necessary.\n")
	b.WriteString("If you use the `exec` tool, prefer setting its `cwd` to the target directory discovered in Plan mode instead of assuming the current working directory is correct.\n")

	if planContext != "" {
		b.WriteString("\nPlan-mode context (carry this forward exactly):\n")
		b.WriteString(planContext)
		b.WriteString("\n")
	}

	if planText != "" {
		b.WriteString("\nApproved plan (verbatim):\n\n")
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
