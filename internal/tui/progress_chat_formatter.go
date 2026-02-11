package tui

import (
	"strings"

	"cli-agent/internal/app"
)

// FormatProgressEventForChat converts progress events to inline chat-zone lines
// (no message bubble), optimized for live readability while a task runs.
func FormatProgressEventForChat(ev app.ProgressEvent) string {
	kind := strings.ToLower(strings.TrimSpace(ev.Kind))
	switch kind {
	case "thinking":
		text := sanitizeProgressText(ev.Text)
		if text == "" {
			text = "Thinking..."
		}
		return "• Thinking\n└ " + text
	case "reasoning":
		text := sanitizeProgressText(ev.Text)
		if text == "" {
			return ""
		}
		return "• Reasoning\n└ " + truncateProgressText(text, 700)
	case "tool":
		return formatToolProgressEventForChat(ev)
	default:
		text := sanitizeProgressText(ev.Text)
		if text == "" {
			return ""
		}
		return "• Progress\n└ " + truncateProgressText(text, 220)
	}
}

func formatToolProgressEventForChat(ev app.ProgressEvent) string {
	status := strings.ToLower(strings.TrimSpace(ev.ToolStatus))
	// Show starts immediately; keep failures visible. Suppress generic completions
	// to avoid noisy duplicates.
	if status == "completed" {
		return ""
	}

	entry, ok := timelineEntryForTool(ev)
	if !ok {
		return ""
	}
	detail := entry.Detail
	if status == "error" && !strings.Contains(detail, "(failed)") {
		detail += " (failed)"
	}
	return "• " + entry.Group + "\n└ " + detail
}

func truncateProgressText(input string, max int) string {
	input = strings.TrimSpace(input)
	if max <= 0 || len(input) <= max {
		return input
	}
	return strings.TrimSpace(input[:max]) + "..."
}

func sanitizeProgressText(input string) string {
	text := strings.TrimSpace(input)
	if text == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"**", "",
		"__", "",
		"`", "",
	)
	text = replacer.Replace(text)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "* ") {
			lines[i] = strings.Replace(line, "* ", "• ", 1)
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			lines[i] = strings.Replace(line, "- ", "• ", 1)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
