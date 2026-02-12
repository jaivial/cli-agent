package tui

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type inputAttachmentKind int

const (
	inputAttachmentLongText inputAttachmentKind = iota
	inputAttachmentImagePath
)

type inputAttachment struct {
	kind  inputAttachmentKind
	raw   string
	label string
}

func (m *MainModel) hasInputAttachments() bool {
	return len(m.inputAttachments) > 0
}

func (m *MainModel) clearInputAttachments() (had bool) {
	had = len(m.inputAttachments) > 0
	m.inputAttachments = nil
	m.nextImageIndex = 1
	return had
}

func (m *MainModel) addLongTextAttachment(raw string) {
	n := len([]rune(raw))
	label := fmt.Sprintf("[Copied text %d]", n)
	m.inputAttachments = append(m.inputAttachments, inputAttachment{kind: inputAttachmentLongText, raw: raw, label: label})
}

func (m *MainModel) addImageAttachment(path string) {
	idx := m.nextImageIndex
	if idx < 1 {
		idx = 1
	}
	label := fmt.Sprintf("[Image %d]", idx)
	m.nextImageIndex = idx + 1
	m.inputAttachments = append(m.inputAttachments, inputAttachment{kind: inputAttachmentImagePath, raw: path, label: label})
}

func normalizeNewlines(s string) string {
	if strings.Contains(s, "\r") {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
	}
	return s
}

func (m *MainModel) tryConsumePasteAsAttachment(pasted string) bool {
	pasted = normalizeNewlines(pasted)
	if strings.TrimSpace(pasted) == "" {
		return false
	}

	// Detect pasted image paths (common for drag+drop into terminals).
	baseDir := m.sessionWorkDir()
	if paths := extractPastedImagePaths(pasted, baseDir); len(paths) > 0 {
		for _, p := range paths {
			m.addImageAttachment(p)
		}
		return true
	}

	// Capture large pasted text blobs as an attachment, but keep the UI clean.
	if len([]rune(pasted)) > 100 {
		m.addLongTextAttachment(pasted)
		return true
	}

	return false
}

func extractPastedImagePaths(pasted string, baseDir string) []string {
	tokens := splitShellLikeFields(pasted)
	if len(tokens) == 0 {
		return nil
	}

	paths := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		p, ok := normalizePastedPath(tok, baseDir)
		if !ok || strings.TrimSpace(p) == "" {
			return nil
		}
		if !isExistingImageFile(p) {
			return nil
		}
		paths = append(paths, p)
	}

	if len(paths) == 0 {
		return nil
	}
	return paths
}

func normalizePastedPath(token string, baseDir string) (string, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}

	// Common terminals emit file:// URIs on drag+drop.
	if strings.HasPrefix(token, "file://") {
		u, err := url.Parse(token)
		if err != nil {
			return "", false
		}
		path := u.Path
		if path == "" && u.Opaque != "" {
			path = u.Opaque
		}
		if path == "" {
			return "", false
		}
		if decoded, err := url.PathUnescape(path); err == nil {
			path = decoded
		}
		token = path
	}

	// Expand ~/
	if strings.HasPrefix(token, "~/") || token == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if token == "~" {
				token = home
			} else {
				token = filepath.Join(home, token[2:])
			}
		}
	}

	if baseDir != "" && !filepath.IsAbs(token) {
		token = filepath.Join(baseDir, token)
	}

	return filepath.Clean(token), true
}

func isExistingImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		// ok
	default:
		return false
	}

	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	if st.IsDir() {
		return false
	}
	if !st.Mode().IsRegular() {
		return false
	}
	return true
}

func splitShellLikeFields(s string) []string {
	s = strings.TrimSpace(normalizeNewlines(s))
	if s == "" {
		return nil
	}

	var out []string
	var b strings.Builder

	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, b.String())
		b.Reset()
	}

	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && !inSingle {
			escaped = true
			continue
		}
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if unicode.IsSpace(r) && !inSingle && !inDouble {
			flush()
			continue
		}
		b.WriteRune(r)
	}

	if escaped {
		// Keep a dangling backslash literal.
		b.WriteRune('\\')
	}
	flush()

	return out
}

func (m *MainModel) buildUserTurn(typed string) (query string, display string) {
	typed = strings.TrimSpace(typed)
	query = typed
	display = typed

	if len(m.inputAttachments) == 0 {
		return query, display
	}

	extraQueryParts := make([]string, 0, len(m.inputAttachments))
	extraDisplayParts := make([]string, 0, len(m.inputAttachments))

	for _, att := range m.inputAttachments {
		label := strings.TrimSpace(att.label)
		if label != "" {
			extraDisplayParts = append(extraDisplayParts, label)
		}
		switch att.kind {
		case inputAttachmentImagePath:
			if label != "" {
				extraQueryParts = append(extraQueryParts, label+" "+att.raw)
			} else {
				extraQueryParts = append(extraQueryParts, att.raw)
			}
		default:
			extraQueryParts = append(extraQueryParts, att.raw)
		}
	}

	extraDisplay := strings.Join(extraDisplayParts, " ")
	if extraDisplay != "" {
		if display != "" {
			display = strings.TrimSpace(display + "\n" + extraDisplay)
		} else {
			display = extraDisplay
		}
	}

	extraQuery := strings.Join(extraQueryParts, "\n\n")
	if extraQuery != "" {
		if query != "" {
			query = strings.TrimSpace(query + "\n\n" + extraQuery)
		} else {
			query = extraQuery
		}
	}

	return query, display
}
