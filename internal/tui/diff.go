package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DiffRenderer renders git-style diffs with colors
type DiffRenderer struct {
	addedStyle    lipgloss.Style
	removedStyle  lipgloss.Style
	headerStyle   lipgloss.Style
	hunkStyle     lipgloss.Style
	lineNumStyle  lipgloss.Style
	contextStyle  lipgloss.Style
	filePathStyle lipgloss.Style
}

// NewDiffRenderer creates a new diff renderer
func NewDiffRenderer() *DiffRenderer {
	return &DiffRenderer{
		addedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSuccess)),
		removedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorError)),
		headerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent2)).
			Bold(true),
		hunkStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)),
		lineNumStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)),
		contextStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg)),
		filePathStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAccent)).
			Bold(true),
	}
}

// RenderDiff renders a git-style diff
func (d *DiffRenderer) RenderDiff(diff string, width int) string {
	var result strings.Builder
	lines := strings.Split(diff, "\n")

	for _, line := range lines {
		if len(line) == 0 {
			result.WriteString("\n")
			continue
		}

		switch {
		case strings.HasPrefix(line, "diff --git"):
			// Diff header
			result.WriteString(d.headerStyle.Render(line))
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			// File paths
			result.WriteString(d.filePathStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			// Hunk header
			result.WriteString(d.hunkStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			// Added line
			result.WriteString(d.addedStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			// Removed line
			result.WriteString(d.removedStyle.Render(line))
		default:
			// Context line
			result.WriteString(d.contextStyle.Render(line))
		}
		result.WriteString("\n")
	}

	return strings.TrimRight(result.String(), "\n")
}

// RenderFileEdit renders a file edit notification with inline diff
func (d *DiffRenderer) RenderFileEdit(filePath string, oldContent, newContent string, width int) string {
	var result strings.Builder

	// Header
	headerBox := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorAccent)).
		Bold(true).
		Render(fmt.Sprintf(" File Modified: %s ", filePath))

	result.WriteString(headerBox)
	result.WriteString("\n")

	// Generate simple diff
	diff := d.generateSimpleDiff(oldContent, newContent)
	result.WriteString(d.RenderDiff(diff, width))

	return result.String()
}

// generateSimpleDiff creates a simple line-by-line diff
func (d *DiffRenderer) generateSimpleDiff(old, new string) string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	var result strings.Builder

	// Find changes using simple LCS-based approach
	changes := computeChanges(oldLines, newLines)

	for _, change := range changes {
		switch change.Type {
		case "equal":
			result.WriteString(fmt.Sprintf(" %s\n", change.Line))
		case "add":
			result.WriteString(fmt.Sprintf("+%s\n", change.Line))
		case "remove":
			result.WriteString(fmt.Sprintf("-%s\n", change.Line))
		}
	}

	return result.String()
}

type change struct {
	Type string
	Line string
}

// computeChanges computes line changes between old and new content
func computeChanges(old, new []string) []change {
	var changes []change

	// Simple approach: compare line by line
	oldIdx, newIdx := 0, 0

	for oldIdx < len(old) || newIdx < len(new) {
		if oldIdx >= len(old) {
			// Remaining lines are additions
			for ; newIdx < len(new); newIdx++ {
				changes = append(changes, change{Type: "add", Line: new[newIdx]})
			}
			break
		}
		if newIdx >= len(new) {
			// Remaining lines are removals
			for ; oldIdx < len(old); oldIdx++ {
				changes = append(changes, change{Type: "remove", Line: old[oldIdx]})
			}
			break
		}

		if old[oldIdx] == new[newIdx] {
			changes = append(changes, change{Type: "equal", Line: old[oldIdx]})
			oldIdx++
			newIdx++
		} else {
			// Look ahead to find matching lines
			matchOld, matchNew := findNextMatch(old[oldIdx:], new[newIdx:])

			if matchOld == -1 && matchNew == -1 {
				// No match found, treat as replacement
				changes = append(changes, change{Type: "remove", Line: old[oldIdx]})
				changes = append(changes, change{Type: "add", Line: new[newIdx]})
				oldIdx++
				newIdx++
			} else if matchNew < matchOld || matchOld == -1 {
				// Addition is closer
				for i := 0; i < matchNew; i++ {
					changes = append(changes, change{Type: "add", Line: new[newIdx+i]})
				}
				newIdx += matchNew
			} else {
				// Removal is closer
				for i := 0; i < matchOld; i++ {
					changes = append(changes, change{Type: "remove", Line: old[oldIdx+i]})
				}
				oldIdx += matchOld
			}
		}
	}

	return changes
}

// findNextMatch finds the next matching line in both arrays
func findNextMatch(old, new []string) (int, int) {
	maxLook := 5 // Look ahead up to 5 lines

	for i := 1; i < len(old) && i <= maxLook; i++ {
		for j := 0; j < len(new) && j <= maxLook; j++ {
			if old[i] == new[j] {
				return i, j
			}
		}
	}

	for j := 1; j < len(new) && j <= maxLook; j++ {
		for i := 0; i < len(old) && i <= maxLook; i++ {
			if old[i] == new[j] {
				return i, j
			}
		}
	}

	return -1, -1
}

// FormatEditMessage formats a file edit message for the chat
func FormatEditMessage(filePath string, changeType string, oldContent, newContent string) string {
	renderer := NewDiffRenderer()

	var result strings.Builder

	// Icon and header based on change type
	var icon, action string
	switch changeType {
	case "create":
		icon = "+"
		action = "Created"
	case "modify":
		icon = "~"
		action = "Modified"
	case "delete":
		icon = "-"
		action = "Deleted"
	default:
		icon = "*"
		action = "Changed"
	}

	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorAccent)).
		Bold(true).
		Render(fmt.Sprintf("%s %s: %s", icon, action, filePath))

	result.WriteString(header)
	result.WriteString("\n\n")

	// Show diff for modifications
	if changeType == "modify" && oldContent != "" && newContent != "" {
		diff := renderer.generateSimpleDiff(oldContent, newContent)
		result.WriteString(renderer.RenderDiff(diff, 80))
	} else if changeType == "create" && newContent != "" {
		// Show new content as additions
		lines := strings.Split(newContent, "\n")
		maxLines := 20
		if len(lines) > maxLines {
			for i := 0; i < maxLines; i++ {
				result.WriteString(renderer.addedStyle.Render(fmt.Sprintf("+%s", lines[i])))
				result.WriteString("\n")
			}
			result.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorMuted)).
				Italic(true).
				Render(fmt.Sprintf("... and %d more lines", len(lines)-maxLines)))
		} else {
			for _, line := range lines {
				result.WriteString(renderer.addedStyle.Render(fmt.Sprintf("+%s", line)))
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}
