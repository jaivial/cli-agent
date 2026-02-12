package tui

import (
	"fmt"
	"strings"
	"time"

	"cli-agent/internal/app"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

type traceEntry struct {
	kind       string
	group      string
	detail     string
	status     string
	toolCallID string
	command    string
	output     string
}

type traceBlock struct {
	kind    string
	group   string
	status  string
	details []string

	command string
	output  string
}

func (m *MainModel) startTurnTrace() {
	if !m.timelineEnabled {
		m.traceMsgIndex = -1
		return
	}
	traceMsg := Message{
		ID:        fmt.Sprintf("trace-%d", time.Now().UnixNano()),
		Role:      "system",
		Content:   "",
		Timestamp: time.Now(),
		IsStatus:  true,
	}
	m.messages = append(m.messages, traceMsg)
	m.traceMsgIndex = len(m.messages) - 1
}

func (m *MainModel) updateTurnTrace(animate bool) {
	if !m.timelineEnabled || m.traceMsgIndex < 0 || m.traceMsgIndex >= len(m.messages) {
		return
	}
	width := m.chatAreaWidth() - 2
	if width < 30 {
		width = m.chatAreaWidth()
	}
	content := strings.TrimSpace(m.renderTurnTrace(width, animate))
	// If the run produced no traceable events, hide the status message once the
	// turn is done.
	if !m.loading && len(m.turnEvents) == 0 {
		content = ""
	}
	m.messages[m.traceMsgIndex].Content = content
}

func (m *MainModel) renderTurnTrace(width int, animate bool) string {
	if width < 24 {
		width = 24
	}

	entries := m.collectTraceEntries()

	blocks, omitted := buildTraceBlocks(entries)
	if len(blocks) == 0 {
		return ""
	}

	var out strings.Builder
	for i, blk := range blocks {
		if i > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(m.renderTraceBlock(blk, width, animate))
	}
	if omitted > 0 {
		out.WriteString("\n\n")
		out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(fmt.Sprintf("… +%d earlier steps", omitted)))
	}
	return out.String()
}

func hasThinkingEntry(entries []traceEntry) bool {
	for _, e := range entries {
		if e.kind == "thinking" {
			return true
		}
	}
	return false
}

func (m *MainModel) collectTraceEntries() []traceEntry {
	items := make([]traceEntry, 0, len(m.turnEvents))
	toolIndex := make(map[string]int)
	toolOutput := make(map[string]string)

	for _, ev := range m.turnEvents {
		kind := strings.ToLower(strings.TrimSpace(ev.Kind))
		if kind == "" {
			continue
		}
		if kind == "thinking" {
			continue
		}
		if kind == "file_edit" {
			continue
		}
		if kind == "reasoning" && m.mode == app.ModePlan {
			continue
		}
		if kind == "tool_output" {
			id := strings.TrimSpace(ev.ToolCallID)
			if id != "" {
				toolOutput[id] = strings.TrimRight(ev.Text, "\n")
			}
			continue
		}

		switch kind {
		case "reasoning":
			txt := strings.TrimSpace(ev.Text)
			if txt == "" {
				continue
			}
			txt = strings.ReplaceAll(txt, "\r\n", "\n")
			txt = strings.ReplaceAll(txt, "\n", " ")
			txt = strings.Join(strings.Fields(txt), " ")
			if len(txt) > 1400 {
				txt = strings.TrimSpace(txt[:1400]) + "..."
			}
			items = append(items, traceEntry{
				kind:   "reasoning",
				detail: txt,
			})

		case "tool":
			id := strings.TrimSpace(ev.ToolCallID)
			status := strings.ToLower(strings.TrimSpace(ev.ToolStatus))
			tool := strings.ToLower(strings.TrimSpace(ev.Tool))

			entry := traceEntry{
				kind:       "tool",
				status:     status,
				toolCallID: id,
			}

			switch tool {
			case "exec":
				entry.kind = "ran"
				entry.command = formatExecCommandForTrace(strings.TrimSpace(ev.Path), strings.TrimSpace(ev.Command))
			case "read_file":
				entry.group = "Explored"
				entry.detail = "Read " + defaultIfEmpty(strings.TrimSpace(ev.Path), "(unknown)")
			case "list_dir":
				entry.group = "Explored"
				entry.detail = "List " + defaultIfEmpty(strings.TrimSpace(ev.Path), ".")
			case "search_files":
				entry.group = "Explored"
				entry.detail = traceSearchLine("Find files", strings.TrimSpace(ev.Command), strings.TrimSpace(ev.Path))
			case "grep":
				entry.group = "Explored"
				entry.detail = traceSearchLine("Search", strings.TrimSpace(ev.Command), strings.TrimSpace(ev.Path))
			case "write_file":
				entry.group = "Edited"
				entry.detail = "Write " + defaultIfEmpty(strings.TrimSpace(ev.Path), "(unknown)")
			case "edit_file":
				entry.group = "Edited"
				entry.detail = "Edit " + defaultIfEmpty(strings.TrimSpace(ev.Path), "(unknown)")
			case "append_file":
				entry.group = "Edited"
				entry.detail = "Append " + defaultIfEmpty(strings.TrimSpace(ev.Path), "(unknown)")
			case "patch_file":
				entry.group = "Edited"
				entry.detail = "Patch " + defaultIfEmpty(strings.TrimSpace(ev.Path), "(unknown)")
			default:
				if strings.TrimSpace(ev.Text) != "" {
					entry.detail = strings.TrimSpace(ev.Text)
				} else if tool != "" {
					entry.detail = strings.ReplaceAll(tool, "_", " ")
				}
				entry.group = "Worked"
			}

			if id != "" {
				if ix, ok := toolIndex[id]; ok && ix >= 0 && ix < len(items) {
					// Update status/details in place.
					items[ix].status = entry.status
					if entry.kind == "ran" {
						items[ix].kind = "ran"
						items[ix].command = entry.command
					} else {
						items[ix].kind = "tool"
						items[ix].group = entry.group
						items[ix].detail = entry.detail
					}
				} else {
					toolIndex[id] = len(items)
					items = append(items, entry)
				}
			} else {
				items = append(items, entry)
			}

		default:
			txt := strings.TrimSpace(ev.Text)
			if txt == "" {
				continue
			}
			txt = strings.ReplaceAll(txt, "\r\n", "\n")
			txt = strings.ReplaceAll(txt, "\n", " ")
			txt = strings.Join(strings.Fields(txt), " ")
			if len(txt) > 600 {
				txt = strings.TrimSpace(txt[:600]) + "..."
			}
			items = append(items, traceEntry{
				kind:   "reasoning",
				detail: txt,
			})
		}
	}

	for i := range items {
		id := strings.TrimSpace(items[i].toolCallID)
		if id == "" {
			continue
		}
		if out, ok := toolOutput[id]; ok && strings.TrimSpace(out) != "" {
			items[i].output = out
		}
	}

	return items
}

func buildTraceBlocks(entries []traceEntry) (blocks []traceBlock, omitted int) {
	if len(entries) == 0 {
		return nil, 0
	}

	for _, e := range entries {
		switch e.kind {
		case "thinking":
			blocks = append(blocks, traceBlock{
				kind:    "thinking",
				group:   "Thinking",
				status:  e.status,
				details: []string{e.detail},
			})
		case "ran":
			blocks = append(blocks, traceBlock{
				kind:    "ran",
				status:  e.status,
				command: e.command,
				output:  e.output,
			})
		case "tool":
			if e.group == "" || e.detail == "" {
				continue
			}
			if len(blocks) > 0 && blocks[len(blocks)-1].kind == "group" && blocks[len(blocks)-1].group == e.group {
				blocks[len(blocks)-1].details = mergeTraceDetails(blocks[len(blocks)-1].details, e.detail)
				continue
			}
			blocks = append(blocks, traceBlock{
				kind:    "group",
				group:   e.group,
				details: []string{e.detail},
			})
		case "reasoning":
			if strings.TrimSpace(e.detail) == "" {
				continue
			}
			blocks = append(blocks, traceBlock{
				kind:    "reasoning",
				details: []string{e.detail},
			})
		}
	}

	const maxBlocks = 18
	if len(blocks) <= maxBlocks {
		return blocks, 0
	}

	head := 0
	if blocks[0].kind == "thinking" {
		head = 1
	}
	if head >= maxBlocks {
		head = maxBlocks - 1
	}
	tail := maxBlocks - head
	if tail < 1 {
		tail = 1
	}
	trimmed := make([]traceBlock, 0, maxBlocks)
	if head > 0 {
		trimmed = append(trimmed, blocks[:head]...)
	}
	trimmed = append(trimmed, blocks[len(blocks)-tail:]...)
	return trimmed, len(blocks) - len(trimmed)
}

func mergeTraceDetails(details []string, next string) []string {
	next = strings.TrimSpace(next)
	if next == "" {
		return details
	}
	if len(details) == 0 {
		return append(details, next)
	}
	last := strings.TrimSpace(details[len(details)-1])
	if strings.HasPrefix(last, "Read ") && strings.HasPrefix(next, "Read ") {
		a := strings.TrimSpace(strings.TrimPrefix(last, "Read "))
		b := strings.TrimSpace(strings.TrimPrefix(next, "Read "))
		if a != "" && b != "" {
			details[len(details)-1] = "Read " + a + ", " + b
			return details
		}
	}
	return append(details, next)
}

func formatExecCommandForTrace(cwd, command string) string {
	command = strings.TrimSpace(command)
	cwd = strings.TrimSpace(cwd)
	if command == "" {
		command = "shell command"
	}
	if cwd == "" || cwd == "." {
		return command
	}
	return "cd " + cwd + " && " + command
}

func traceSearchLine(prefix, pattern, path string) string {
	pattern = strings.TrimSpace(pattern)
	path = strings.TrimSpace(path)

	switch {
	case pattern != "" && path != "":
		return prefix + " " + pattern + " in " + path
	case pattern != "":
		return prefix + " " + pattern
	case path != "":
		return prefix + " in " + path
	default:
		return prefix
	}
}

func (m *MainModel) renderTraceBlock(blk traceBlock, width int, animate bool) string {
	groupStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorAccent2)).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	fgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorFg))
	reasoningBaseHex := blendHex(colorMuted, colorFg, 0.50)
	reasoningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(reasoningBaseHex)).Italic(true)

	switch blk.kind {
	case "thinking":
		detail := ""
		if len(blk.details) > 0 {
			detail = strings.TrimSpace(blk.details[0])
		}
		if detail == "" {
			detail = "Planning approach"
		}

		header := "• " + groupStyle.Render("Thinking")
		line := "  └ " + detail
		if animate && m.loading && !m.cancelQueued {
			dots := m.renderLoadingDots(1)
			dotW := lipgloss.Width(dots)
			maxText := width - lipgloss.Width("  └ ") - dotW - 2
			if maxText < 8 {
				maxText = 8
			}
			detail = truncate.StringWithTail(detail, uint(maxText), "…")
			shimmered := shimmerText(detail, m.spinnerPos, colorMuted, colorAccent2)
			line = "  └ " + shimmered + "  " + dots
		}
		return header + "\n" + line

	case "group":
		var b strings.Builder
		b.WriteString("• ")
		b.WriteString(groupStyle.Render(defaultIfEmpty(blk.group, "Progress")))
		if len(blk.details) == 0 {
			return b.String()
		}
		b.WriteString("\n")
		for i, d := range blk.details {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			prefix := "  └ "
			if len(blk.details) > 1 && i < len(blk.details)-1 {
				prefix = "  ├ "
			}
			lines := wrapWithPrefixes(d, prefix, "    ", width, 0)
			for j, line := range lines {
				if j == 0 {
					b.WriteString(mutedStyle.Render(line))
				} else {
					b.WriteString("\n")
					b.WriteString(mutedStyle.Render(line))
				}
			}
			if i < len(blk.details)-1 {
				b.WriteString("\n")
			}
		}
		return b.String()

	case "ran":
		command := strings.TrimSpace(blk.command)
		if command == "" {
			command = "shell command"
		}
		lines := wrapWithPrefixes(command, "• Ran ", "  │ ", width, 6)
		var b strings.Builder
		for i, line := range lines {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(fgStyle.Render(line))
		}
		if strings.TrimSpace(blk.output) != "" {
			outLines := renderToolOutputPreview(blk.output, width)
			if len(outLines) > 0 {
				b.WriteString("\n")
				for i, line := range outLines {
					if i > 0 {
						b.WriteString("\n")
					}
					b.WriteString(mutedStyle.Render(line))
				}
			}
		}
		return b.String()

	case "reasoning":
		txt := ""
		if len(blk.details) > 0 {
			txt = strings.TrimSpace(blk.details[0])
		}
		if txt == "" {
			return ""
		}

		// Pulse the bullet between 50% and 100% brightness over ~2 seconds.
		pulse := 0.0
		if animate && m.loading && !m.cancelQueued {
			frames := 2000 / defaultAnimTick
			if frames < 2 {
				frames = 2
			}
			pos := m.spinnerPos % frames
			half := frames / 2
			if half < 1 {
				half = 1
			}
			if pos <= half {
				pulse = 1.0 - float64(pos)/float64(half) // 1 -> 0
			} else {
				den := frames - half
				if den < 1 {
					den = 1
				}
				pulse = float64(pos-half) / float64(den) // 0 -> 1
			}
		}
		bulletHex := blendHex(reasoningBaseHex, colorFg, pulse)
		bullet := lipgloss.NewStyle().Foreground(lipgloss.Color(bulletHex)).Render("•")

		lines := wrapWithPrefixes(txt, "• ", "  ", width, 0)
		var b strings.Builder
		for i, line := range lines {
			if i > 0 {
				b.WriteString("\n")
			}
			switch {
			case strings.HasPrefix(line, "• "):
				rest := strings.TrimPrefix(line, "• ")
				b.WriteString(bullet)
				b.WriteString(" ")
				b.WriteString(reasoningStyle.Render(rest))
			case strings.HasPrefix(line, "  "):
				rest := strings.TrimPrefix(line, "  ")
				b.WriteString("  ")
				b.WriteString(reasoningStyle.Render(rest))
			default:
				b.WriteString(reasoningStyle.Render(line))
			}
		}
		return b.String()

	default:
		return mutedStyle.Render(strings.TrimSpace(strings.Join(blk.details, "\n")))
	}
}

func wrapWithPrefixes(text, firstPrefix, contPrefix string, width, maxLines int) []string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return nil
	}

	firstW := width - lipgloss.Width(firstPrefix)
	contW := width - lipgloss.Width(contPrefix)
	if firstW < 8 {
		firstW = 8
	}
	if contW < 8 {
		contW = 8
	}

	words := strings.Fields(text)
	var lines []string
	var cur strings.Builder
	curW := 0
	limit := firstW
	prefix := firstPrefix

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		lines = append(lines, prefix+cur.String())
		cur.Reset()
		curW = 0
		prefix = contPrefix
		limit = contW
	}

	for _, w := range words {
		wordW := lipgloss.Width(w)
		if cur.Len() == 0 {
			if wordW > limit {
				lines = append(lines, prefix+truncate.StringWithTail(w, uint(limit), "…"))
				prefix = contPrefix
				limit = contW
				continue
			}
			cur.WriteString(w)
			curW = wordW
			continue
		}

		if curW+1+wordW <= limit {
			cur.WriteByte(' ')
			cur.WriteString(w)
			curW += 1 + wordW
			continue
		}

		flush()
		if wordW > limit {
			lines = append(lines, prefix+truncate.StringWithTail(w, uint(limit), "…"))
			prefix = contPrefix
			limit = contW
			continue
		}
		cur.WriteString(w)
		curW = wordW
	}
	flush()

	if maxLines > 0 && len(lines) > maxLines {
		keep := maxLines - 1
		if keep < 1 {
			keep = 1
		}
		omitted := len(lines) - keep
		lines = append(lines[:keep], contPrefix+fmt.Sprintf("… +%d lines", omitted))
	}

	return lines
}

func renderToolOutputPreview(output string, width int) []string {
	output = strings.TrimRight(output, "\n")
	if strings.TrimSpace(output) == "" {
		return nil
	}
	rawLines := strings.Split(output, "\n")
	if len(rawLines) == 0 {
		return nil
	}

	const maxPreviewLines = 10
	head := 3
	tail := 3
	lines := rawLines

	if len(lines) > maxPreviewLines {
		omitted := len(lines) - head - tail
		if omitted < 0 {
			omitted = 0
		}
		trimmed := make([]string, 0, head+tail+1)
		trimmed = append(trimmed, lines[:head]...)
		if omitted > 0 {
			trimmed = append(trimmed, fmt.Sprintf("… +%d lines", omitted))
		}
		trimmed = append(trimmed, lines[len(lines)-tail:]...)
		lines = trimmed
	}

	out := make([]string, 0, len(lines)+1)
	firstPrefix := "  └ "
	contPrefix := "    "
	firstW := width - lipgloss.Width(firstPrefix)
	contW := width - lipgloss.Width(contPrefix)
	if firstW < 8 {
		firstW = 8
	}
	if contW < 8 {
		contW = 8
	}

	for i, line := range lines {
		if i == 0 {
			line = truncate.StringWithTail(line, uint(firstW), "…")
			out = append(out, firstPrefix+line)
			continue
		}
		line = truncate.StringWithTail(line, uint(contW), "…")
		out = append(out, contPrefix+line)
	}
	return out
}
