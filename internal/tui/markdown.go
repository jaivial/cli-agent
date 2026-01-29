package tui

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Pre-compiled regex patterns for better performance
var (
	codeBlockRegex = regexp.MustCompile(`(?s)<pre><code(?: class="language-([a-zA-Z0-9]+)")?>(.*?)</code></pre>`)
	h1Regex        = regexp.MustCompile(`<h1 id="[^"]*">(.*?)</h1>`)
	h2Regex        = regexp.MustCompile(`<h2 id="[^"]*">(.*?)</h2>`)
	h3Regex        = regexp.MustCompile(`<h3 id="[^"]*">(.*?)</h3>`)
	strongRegex    = regexp.MustCompile(`<strong>(.*?)</strong>`)
	emRegex        = regexp.MustCompile(`<em>(.*?)</em>`)
	linkRegex      = regexp.MustCompile(`<a href="([^"]*)">(.*?)</a>`)
	blockquoteRe   = regexp.MustCompile(`(?s)<blockquote>(.*?)</blockquote>`)
	ulRegex        = regexp.MustCompile(`(?s)<ul>(.*?)</ul>`)
	olRegex        = regexp.MustCompile(`(?s)<ol>(.*?)</ol>`)
	liRegex        = regexp.MustCompile(`<li>(.*?)</li>`)
	htmlTagRegex   = regexp.MustCompile(`<[^>]+>`)
	multiNewline   = regexp.MustCompile(`\n{3,}`)
	inlineCodeRe   = regexp.MustCompile(`<code>([^<]+)</code>`)
)

// MarkdownRenderer renders markdown content with syntax highlighting
type MarkdownRenderer struct {
	goldmark.Markdown
	formatter chroma.Formatter
	style     *chroma.Style
}

// NewMarkdownRenderer creates a new markdown renderer
func NewMarkdownRenderer() *MarkdownRenderer {
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		),
	)

	return &MarkdownRenderer{
		Markdown:  md,
		formatter: formatters.Get("terminal256"),
		style:     styles.Get("dracula"),
	}
}

// Render renders markdown content to terminal format
func (r *MarkdownRenderer) Render(content string, width int) string {
	var buf bytes.Buffer

	// Render markdown to HTML
	if err := r.Convert([]byte(content), &buf); err != nil {
		return content
	}

	// Convert HTML to terminal output with syntax highlighting
	return r.formatForTerminal(buf.String(), width)
}

// formatForTerminal formats HTML for terminal output
func (r *MarkdownRenderer) formatForTerminal(htmlContent string, width int) string {
	result := htmlContent

	// Extract and process code blocks first (before other transformations)
	var codeBlocks []string
	result = codeBlockRegex.ReplaceAllStringFunc(result, func(m string) string {
		matches := codeBlockRegex.FindStringSubmatch(m)
		if len(matches) < 3 {
			return m
		}

		lang := matches[1]
		code := r.decodeHTMLEntities(matches[2])

		// Render with syntax highlighting
		highlighted := r.RenderCodeBlock(code, lang)

		// Style the code block with dynamic width
		codeWidth := width - 8
		if codeWidth < 20 {
			codeWidth = 20
		}

		styled := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			Background(lipgloss.Color("#282A36")).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6272A4")).
			Width(codeWidth).
			Render(highlighted)

		index := len(codeBlocks)
		codeBlocks = append(codeBlocks, styled)
		return fmt.Sprintf("\n{{CODE_BLOCK_%d}}\n", index)
	})

	// Handle inline code
	result = inlineCodeRe.ReplaceAllStringFunc(result, func(m string) string {
		matches := inlineCodeRe.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		code := r.decodeHTMLEntities(matches[1])
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8F8F2")).
			Background(lipgloss.Color("#44475A")).
			Padding(0, 1).
			Render(code)
	})

	// Replace headers
	result = h1Regex.ReplaceAllStringFunc(result, func(m string) string {
		matches := h1Regex.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#BD93F9")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("#6272A4")).
			Width(width - 4).
			Render(matches[1]) + "\n"
	})

	result = h2Regex.ReplaceAllStringFunc(result, func(m string) string {
		matches := h2Regex.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF79C6")).
			Width(width - 4).
			Render(matches[1]) + "\n"
	})

	result = h3Regex.ReplaceAllStringFunc(result, func(m string) string {
		matches := h3Regex.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8BE9FD")).
			Width(width - 4).
			Render(matches[1]) + "\n"
	})

	// Replace bold
	result = strongRegex.ReplaceAllStringFunc(result, func(m string) string {
		matches := strongRegex.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8F8F2")).
			Render(matches[1])
	})

	// Replace italic
	result = emRegex.ReplaceAllStringFunc(result, func(m string) string {
		matches := emRegex.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		return lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("#8BE9FD")).
			Render(matches[1])
	})

	// Replace links
	result = linkRegex.ReplaceAllStringFunc(result, func(m string) string {
		matches := linkRegex.FindStringSubmatch(m)
		if len(matches) < 3 {
			return m
		}
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8BE9FD")).
			Underline(true).
			Render(fmt.Sprintf("%s (%s)", matches[2], matches[1]))
	})

	// Replace blockquotes
	result = blockquoteRe.ReplaceAllStringFunc(result, func(m string) string {
		matches := blockquoteRe.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		content := strings.TrimSpace(matches[1])
		content = htmlTagRegex.ReplaceAllString(content, "")
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272A4")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(lipgloss.Color("#BD93F9")).
			PaddingLeft(2).
			Width(width - 4).
			Render(content) + "\n"
	})

	// Replace unordered lists
	result = ulRegex.ReplaceAllStringFunc(result, func(m string) string {
		matches := ulRegex.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		items := liRegex.FindAllStringSubmatch(matches[1], -1)
		var list strings.Builder
		for _, item := range items {
			if len(item) >= 2 {
				itemContent := htmlTagRegex.ReplaceAllString(item[1], "")
				list.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("#50FA7B")).
					Render("  "))
				list.WriteString(itemContent)
				list.WriteString("\n")
			}
		}
		return list.String()
	})

	// Replace ordered lists
	result = olRegex.ReplaceAllStringFunc(result, func(m string) string {
		matches := olRegex.FindStringSubmatch(m)
		if len(matches) < 2 {
			return m
		}
		items := liRegex.FindAllStringSubmatch(matches[1], -1)
		var list strings.Builder
		for i, item := range items {
			if len(item) >= 2 {
				itemContent := htmlTagRegex.ReplaceAllString(item[1], "")
				list.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFB86C")).
					Bold(true).
					Render(fmt.Sprintf("  %d. ", i+1)))
				list.WriteString(itemContent)
				list.WriteString("\n")
			}
		}
		return list.String()
	})

	// Replace paragraphs and line breaks
	result = strings.ReplaceAll(result, "<p>", "")
	result = strings.ReplaceAll(result, "</p>", "\n")
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<br />", "\n")

	// Restore code blocks
	for i, codeBlock := range codeBlocks {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{CODE_BLOCK_%d}}", i), codeBlock)
	}

	// Clean up any remaining HTML tags
	result = htmlTagRegex.ReplaceAllString(result, "")

	// Decode HTML entities
	result = r.decodeHTMLEntities(result)

	// Clean up excessive newlines
	result = multiNewline.ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	return result
}

// decodeHTMLEntities decodes common HTML entities
func (r *MarkdownRenderer) decodeHTMLEntities(s string) string {
	replacements := []struct {
		old string
		new string
	}{
		{"&amp;", "&"},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&quot;", "\""},
		{"&#39;", "'"},
		{"&nbsp;", " "},
		{"&mdash;", "—"},
		{"&ndash;", "–"},
		{"&hellip;", "..."},
		{"&copy;", "(c)"},
		{"&reg;", "(R)"},
		{"&trade;", "(TM)"},
		{"&#x27;", "'"},
		{"&#x60;", "`"},
	}

	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.old, r.new)
	}
	return s
}

// RenderCodeBlock renders a code block with syntax highlighting
func (r *MarkdownRenderer) RenderCodeBlock(code, lang string) string {
	var buf bytes.Buffer

	// Determine the lexer
	var lexer chroma.Lexer
	if lang != "" {
		lexer = lexers.Get(lang)
	}
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Tokenize the code
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	// Format with terminal colors
	if err := r.formatter.Format(&buf, r.style, iterator); err != nil {
		return code
	}

	return strings.TrimRight(buf.String(), "\n")
}
