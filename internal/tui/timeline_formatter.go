package tui

import (
	"fmt"
	"strings"

	"cli-agent/internal/app"
)

const timelineMaxEntries = 12

type timelineEntry struct {
	Group  string
	Detail string
}

// FormatTimeline renders tool progress as grouped bullets similar to OpenCode/Codex
// session traces (Explored/Edited/Executed + per-step detail).
func FormatTimeline(events []app.ProgressEvent) string {
	if len(events) == 0 {
		return ""
	}

	entries := make([]timelineEntry, 0, len(events))
	for _, ev := range events {
		ev.Kind = strings.TrimSpace(ev.Kind)
		if ev.Kind == "" {
			continue
		}

		var (
			entry timelineEntry
			ok    bool
		)

		switch strings.ToLower(ev.Kind) {
		case "thinking":
			detail := strings.TrimSpace(ev.Text)
			if detail == "" {
				detail = "Planning approach"
			}
			entry = timelineEntry{Group: "Thinking", Detail: detail}
			ok = true
		case "file_edit":
			continue
		case "tool_output":
			continue
		case "tool":
			status := strings.ToLower(strings.TrimSpace(ev.ToolStatus))
			if status == "pending" || status == "running" || status == "in_progress" {
				continue
			}
			entry, ok = timelineEntryForTool(ev)
			if ok && status == "error" && !strings.Contains(entry.Detail, "(failed)") {
				entry.Detail += " (failed)"
			}
		default:
			detail := strings.TrimSpace(ev.Text)
			if detail != "" {
				entry = timelineEntry{Group: "Progress", Detail: detail}
				ok = true
			}
		}

		if !ok || entry.Group == "" || entry.Detail == "" {
			continue
		}
		// De-noise duplicate adjacent events.
		if len(entries) > 0 && entries[len(entries)-1] == entry {
			continue
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return ""
	}

	entries, omitted := trimTimelineEntries(entries, timelineMaxEntries)
	return renderTimeline(entries, omitted)
}

func trimTimelineEntries(entries []timelineEntry, max int) ([]timelineEntry, int) {
	if len(entries) <= max || max <= 0 {
		return entries, 0
	}

	headCount := 0
	if strings.EqualFold(entries[0].Group, "Thinking") {
		headCount = 1
	}
	if headCount >= max {
		headCount = max - 1
	}
	tailCount := max - headCount
	if tailCount < 1 {
		tailCount = 1
	}

	trimmed := make([]timelineEntry, 0, max)
	if headCount > 0 {
		trimmed = append(trimmed, entries[:headCount]...)
	}
	trimmed = append(trimmed, entries[len(entries)-tailCount:]...)

	return trimmed, len(entries) - len(trimmed)
}

func renderTimeline(entries []timelineEntry, omitted int) string {
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder
	lastGroup := ""

	for _, entry := range entries {
		if entry.Group != lastGroup {
			b.WriteString("• ")
			b.WriteString(entry.Group)
			b.WriteString("\n")
			lastGroup = entry.Group
		}
		b.WriteString("  └ ")
		b.WriteString(entry.Detail)
		b.WriteString("\n")
	}

	if omitted > 0 {
		b.WriteString("• Progress\n")
		b.WriteString(fmt.Sprintf("  └ ... %d earlier steps omitted\n", omitted))
	}

	return strings.TrimSpace(b.String())
}

func timelineEntryForTool(ev app.ProgressEvent) (timelineEntry, bool) {
	tool := strings.ToLower(strings.TrimSpace(ev.Tool))
	switch tool {
	case "read_file":
		return timelineEntry{
			Group:  "Explored",
			Detail: "Read " + timelineCode(defaultIfEmpty(ev.Path, "(unknown path)")),
		}, true
	case "list_dir":
		return timelineEntry{
			Group:  "Explored",
			Detail: "List " + timelineCode(defaultIfEmpty(ev.Path, ".")),
		}, true
	case "search_files":
		return timelineEntry{
			Group:  "Explored",
			Detail: timelineSearchDetail("Search files", ev.Command, ev.Path),
		}, true
	case "grep":
		return timelineEntry{
			Group:  "Explored",
			Detail: timelineSearchDetail("Search", ev.Command, ev.Path),
		}, true
	case "write_file":
		return timelineEntry{
			Group:  "Edited",
			Detail: "Write " + timelineCode(defaultIfEmpty(ev.Path, "(unknown path)")),
		}, true
	case "edit_file":
		return timelineEntry{
			Group:  "Edited",
			Detail: "Edit " + timelineCode(defaultIfEmpty(ev.Path, "(unknown path)")),
		}, true
	case "append_file":
		return timelineEntry{
			Group:  "Edited",
			Detail: "Append " + timelineCode(defaultIfEmpty(ev.Path, "(unknown path)")),
		}, true
	case "patch_file":
		return timelineEntry{
			Group:  "Edited",
			Detail: "Patch " + timelineCode(defaultIfEmpty(ev.Path, "(unknown path)")),
		}, true
	case "exec":
		cmd := strings.TrimSpace(ev.Command)
		if cmd == "" {
			cmd = "shell command"
		}
		return timelineEntry{
			Group:  "Executed",
			Detail: "Run " + timelineCode(truncateForTimeline(cmd, 96)),
		}, true
	default:
		detail := strings.TrimSpace(ev.Text)
		if detail == "" {
			if tool == "" {
				return timelineEntry{}, false
			}
			detail = strings.ReplaceAll(tool, "_", " ")
		}
		return timelineEntry{Group: "Worked", Detail: detail}, true
	}
}

func timelineSearchDetail(prefix, pattern, path string) string {
	pattern = strings.TrimSpace(pattern)
	path = strings.TrimSpace(path)

	switch {
	case pattern != "" && path != "":
		return prefix + " " + timelineCode(pattern) + " in " + timelineCode(path)
	case pattern != "":
		return prefix + " " + timelineCode(pattern)
	case path != "":
		return prefix + " in " + timelineCode(path)
	default:
		return prefix
	}
}

func timelineCode(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return "``"
	}
	input = strings.ReplaceAll(input, "`", "'")
	return "`" + input + "`"
}

func truncateForTimeline(input string, max int) string {
	input = strings.TrimSpace(input)
	if max <= 0 || len(input) <= max {
		return input
	}
	return strings.TrimSpace(input[:max]) + "..."
}

func defaultIfEmpty(input string, fallback string) string {
	if strings.TrimSpace(input) == "" {
		return fallback
	}
	return input
}
