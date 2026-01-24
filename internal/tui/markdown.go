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
		Markdown: md,
		formatter: formatters.Get("terminal256"),
		style: styles.Get("github-dark"),
	}
}

// Render renders markdown content to terminal format
func (r *MarkdownRenderer) Render(content string, width int) string {
	var buf bytes.Buffer

	// Render markdown to HTML
	if err := r.Convert([]byte(content), &buf); err != nil {
		return fmt.Sprintf("Error rendering markdown: %v", err)
	}

	// Convert HTML to terminal output with syntax highlighting
	htmlContent := buf.String()
	htmlContent = r.highlightCodeBlocks(htmlContent)

	// Clean up and format for terminal
	return r.formatForTerminal(htmlContent)
}

// highlightCodeBlocks adds syntax highlighting to code blocks
func (r *MarkdownRenderer) highlightCodeBlocks(content string) string {
	// Regex to find code blocks
	codeBlockRegex := regexp.MustCompile(`<pre><code(?: class="language-([a-zA-Z0-9]+)")?>(.*?)</code></pre>`)
	return codeBlockRegex.ReplaceAllStringFunc(content, func(match string) string {
		matches := codeBlockRegex.FindStringSubmatch(match)
		if len(matches) < 3 {
			return match
		}

		lang := matches[1]
		code := matches[2]

		// Decode HTML entities
		code = r.decodeHTMLEntities(code)

		// Render with syntax highlighting
		highlightedCode := r.RenderCodeBlock(code, lang)

		// Wrap in styled code block
		return codeBlockStyle.Render(highlightedCode)
	})
}

// decodeHTMLEntities decodes common HTML entities
func (r *MarkdownRenderer) decodeHTMLEntities(s string) string {
	// Decode HTML entities
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

// formatForTerminal formats HTML for terminal output
func (r *MarkdownRenderer) formatForTerminal(htmlContent string) string {
	// Remove HTML tags and format for terminal
	result := htmlContent

	// First, handle code blocks (we want to preserve these as they're already styled)
	// Extract and preserve code blocks
	var codeBlocks []string
	codeBlockRegex := regexp.MustCompile(`<pre><code(?: class="language-([a-zA-Z0-9]+)")?>(.*?)</code></pre>`)
	result = codeBlockRegex.ReplaceAllStringFunc(result, func(m string) string {
		index := len(codeBlocks)
		codeBlocks = append(codeBlocks, m)
		return fmt.Sprintf("{{CODE_BLOCK_%d}}", index)
	})

	// Replace headers
	result = regexp.MustCompile(`<h1 id=".*?">(.*?)</h1>`).ReplaceAllStringFunc(result, func(m string) string {
		content := regexp.MustCompile(`<h1 id=".*?">(.*?)</h1>`).FindStringSubmatch(m)[1]
		return heading1Style.Render(content) + "\n"
	})

	result = regexp.MustCompile(`<h2 id=".*?">(.*?)</h2>`).ReplaceAllStringFunc(result, func(m string) string {
		content := regexp.MustCompile(`<h2 id=".*?">(.*?)</h2>`).FindStringSubmatch(m)[1]
		return heading2Style.Render(content) + "\n"
	})

	result = regexp.MustCompile(`<h3 id=".*?">(.*?)</h3>`).ReplaceAllStringFunc(result, func(m string) string {
		content := regexp.MustCompile(`<h3 id=".*?">(.*?)</h3>`).FindStringSubmatch(m)[1]
		return heading3Style.Render(content) + "\n"
	})

	// Replace bold
	result = regexp.MustCompile(`<strong>(.*?)</strong>`).ReplaceAllStringFunc(result, func(m string) string {
		content := regexp.MustCompile(`<strong>(.*?)</strong>`).FindStringSubmatch(m)[1]
		return boldStyle.Render(content)
	})

	// Replace italic
	result = regexp.MustCompile(`<em>(.*?)</em>`).ReplaceAllStringFunc(result, func(m string) string {
		content := regexp.MustCompile(`<em>(.*?)</em>`).FindStringSubmatch(m)[1]
		return italicStyle.Render(content)
	})

	// Replace links
	result = regexp.MustCompile(`<a href="(.*?)">(.*?)</a>`).ReplaceAllStringFunc(result, func(m string) string {
		matches := regexp.MustCompile(`<a href="(.*?)">(.*?)</a>`).FindStringSubmatch(m)
		return linkStyle.Render(fmt.Sprintf("%s (%s)", matches[2], matches[1]))
	})

	// Replace quotes
	result = regexp.MustCompile(`<blockquote>(.*?)</blockquote>`).ReplaceAllStringFunc(result, func(m string) string {
		content := regexp.MustCompile(`<blockquote>(.*?)</blockquote>`).FindStringSubmatch(m)[1]
		return quoteStyle.Render(content) + "\n"
	})

	// Replace unordered lists
	result = regexp.MustCompile(`<ul>(.*?)</ul>`).ReplaceAllStringFunc(result, func(m string) string {
		content := regexp.MustCompile(`<ul>(.*?)</ul>`).FindStringSubmatch(m)[1]
		items := regexp.MustCompile(`<li>(.*?)</li>`).FindAllStringSubmatch(content, -1)
		var list string
		for _, item := range items {
			list += fmt.Sprintf("• %s\n", item[1])
		}
		return list
	})

	// Replace ordered lists
	result = regexp.MustCompile(`<ol>(.*?)</ol>`).ReplaceAllStringFunc(result, func(m string) string {
		content := regexp.MustCompile(`<ol>(.*?)</ol>`).FindStringSubmatch(m)[1]
		items := regexp.MustCompile(`<li>(.*?)</li>`).FindAllStringSubmatch(content, -1)
		var list string
		for i, item := range items {
			list += fmt.Sprintf("%d. %s\n", i+1, item[1])
		}
		return list
	})

	// Replace paragraphs
	result = strings.ReplaceAll(result, "<p>", "")
	result = strings.ReplaceAll(result, "</p>", "\n\n")
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<br />", "\n")

	// Restore code blocks with syntax highlighting
	for i, codeBlock := range codeBlocks {
		// Highlight the code block
		highlighted := r.highlightCodeBlocks(codeBlock)
		// Replace the placeholder with the highlighted code
		result = strings.ReplaceAll(result, fmt.Sprintf("{{CODE_BLOCK_%d}}", i), highlighted)
	}

	// Clean up any remaining HTML tags
	htmlTagRegex := regexp.MustCompile(`<[^>]+>`)
	result = htmlTagRegex.ReplaceAllString(result, "")

	// Decode HTML entities
	result = r.decodeHTMLEntities(result)

	// Trim extra whitespace
	result = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	return result
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

	return buf.String()
}

// Styles for markdown rendering - ClaudeCode design
var (
	heading1Style = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorPrimary)).
			Background(lipgloss.Color(colorBg)).
			Padding(1, 0).
			Margin(1, 0).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderBottomForeground(lipgloss.Color(colorBorder)).
			Width(80)

	heading2Style = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorSecondary)).
			Background(lipgloss.Color(colorBg)).
			Padding(0, 0).
			Margin(1, 0).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderBottomForeground(lipgloss.Color(colorBorder)).
			Width(80)

	heading3Style = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorAccent)).
			Background(lipgloss.Color(colorBg)).
			Padding(0, 0).
			Margin(1, 0).
			Width(80)

	boldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorFg))

	italicStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color(colorFgMuted))

	linkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorPrimary)).
			Underline(true)

	codeBlockStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFg)).
			Background(lipgloss.Color(colorCodeBg)).
			Padding(2, 4).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorCodeBorder)).
			Width(80).
			Margin(1, 0)

	quoteStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorFgMuted)).
			Background(lipgloss.Color(colorBg)).
			Padding(1, 3).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderLeftForeground(lipgloss.Color(colorPrimary)).
			Width(80).
			Margin(1, 0)
)
