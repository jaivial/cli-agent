package app

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type unifiedHunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []string
}

var unifiedHunkHeaderRE = regexp.MustCompile(`^@@\s+\-(\d+)(?:,(\d+))?\s+\+(\d+)(?:,(\d+))?\s+@@`)

func parseUnifiedDiffHunks(patch string) ([]unifiedHunk, error) {
	patch = strings.ReplaceAll(patch, "\r\n", "\n")
	lines := strings.Split(patch, "\n")

	var hunks []unifiedHunk
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		m := unifiedHunkHeaderRE.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}

		oldStart, _ := strconv.Atoi(m[1])
		oldCount := 1
		if m[2] != "" {
			oldCount, _ = strconv.Atoi(m[2])
		}
		newStart, _ := strconv.Atoi(m[3])
		newCount := 1
		if m[4] != "" {
			newCount, _ = strconv.Atoi(m[4])
		}

		h := unifiedHunk{
			oldStart: oldStart,
			oldCount: oldCount,
			newStart: newStart,
			newCount: newCount,
		}

		// Collect hunk body until the next header (or end).
		for j := i + 1; j < len(lines); j++ {
			next := lines[j]
			if unifiedHunkHeaderRE.MatchString(next) {
				i = j - 1
				break
			}
			if next == "" {
				// Ignore the empty trailing line produced by Split; real hunk lines always have a prefix.
				continue
			}
			switch next[0] {
			case ' ', '+', '-', '\\':
				h.lines = append(h.lines, next)
			default:
				// Ignore file headers and other metadata lines.
			}
			if j == len(lines)-1 {
				i = j
			}
		}

		hunks = append(hunks, h)
	}

	if len(hunks) == 0 {
		return nil, fmt.Errorf("no unified diff hunks found")
	}
	return hunks, nil
}

func applyUnifiedHunks(fileLines []string, hunks []unifiedHunk) ([]string, *bool, error) {
	offset := 0
	// nil => no explicit EOF newline directive in patch, true => no trailing newline, false => trailing newline.
	var eofNoTrailingNewline *bool
	for _, h := range hunks {
		if h.oldStart <= 0 {
			return nil, nil, fmt.Errorf("invalid hunk header: oldStart=%d", h.oldStart)
		}
		idx := (h.oldStart - 1) + offset
		if idx < 0 || idx > len(fileLines) {
			return nil, nil, fmt.Errorf("hunk out of range: oldStart=%d (idx=%d) len=%d", h.oldStart, idx, len(fileLines))
		}

		pos := idx
		var replacement []string
		for i := 0; i < len(h.lines); i++ {
			hl := h.lines[i]
			if hl == "" {
				continue
			}
			prefix := hl[0]
			text := hl[1:]
			hasNoNewlineMarker := i+1 < len(h.lines) && strings.HasPrefix(h.lines[i+1], `\ No newline at end of file`)
			switch prefix {
			case ' ':
				if pos >= len(fileLines) || fileLines[pos] != text {
					return nil, nil, fmt.Errorf("patch context mismatch at line %d", pos+1)
				}
				replacement = append(replacement, fileLines[pos])
				pos++
				if hasNoNewlineMarker {
					noTrailing := true
					eofNoTrailingNewline = &noTrailing
				}
			case '-':
				if pos >= len(fileLines) || fileLines[pos] != text {
					return nil, nil, fmt.Errorf("patch delete mismatch at line %d", pos+1)
				}
				pos++
				if hasNoNewlineMarker {
					withTrailing := false
					eofNoTrailingNewline = &withTrailing
				}
			case '+':
				replacement = append(replacement, text)
				if hasNoNewlineMarker {
					noTrailing := true
					eofNoTrailingNewline = &noTrailing
				} else if eofNoTrailingNewline != nil {
					// Marker may have applied to a removed line, while new side restores trailing newline.
					withTrailing := false
					eofNoTrailingNewline = &withTrailing
				}
			case '\\':
				// "\ No newline at end of file" - ignore.
			default:
				return nil, nil, fmt.Errorf("unexpected patch line prefix: %q", string(prefix))
			}
		}

		consumed := pos - idx
		if h.oldCount > 0 && consumed != h.oldCount {
			return nil, nil, fmt.Errorf("hunk count mismatch: expected %d old lines, consumed %d", h.oldCount, consumed)
		}

		updated := make([]string, 0, len(fileLines)-consumed+len(replacement))
		updated = append(updated, fileLines[:idx]...)
		updated = append(updated, replacement...)
		updated = append(updated, fileLines[pos:]...)
		fileLines = updated
		offset += len(replacement) - consumed
	}
	return fileLines, eofNoTrailingNewline, nil
}

func ApplyUnifiedPatch(oldContent string, patch string) (string, error) {
	oldContent = strings.ReplaceAll(oldContent, "\r\n", "\n")
	hasTrailingNewline := strings.HasSuffix(oldContent, "\n")
	oldContent = strings.TrimSuffix(oldContent, "\n")

	var fileLines []string
	if oldContent == "" && !hasTrailingNewline {
		fileLines = []string{}
	} else {
		fileLines = strings.Split(oldContent, "\n")
	}

	hunks, err := parseUnifiedDiffHunks(patch)
	if err != nil {
		return "", err
	}

	updated, eofNoTrailingNewline, err := applyUnifiedHunks(fileLines, hunks)
	if err != nil {
		return "", err
	}

	out := strings.Join(updated, "\n")
	switch {
	case eofNoTrailingNewline != nil && !*eofNoTrailingNewline:
		out += "\n"
	case eofNoTrailingNewline == nil && hasTrailingNewline:
		out += "\n"
	}
	return out, nil
}

func writeFilePreserveMode(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode := info.Mode().Perm()
	if err := os.WriteFile(path, data, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
