package tui

import (
	"regexp"
	"strings"
	"testing"

	"cli-agent/internal/app"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

func TestRenderStatusBar_DoesNotOverflow(t *testing.T) {
	m := NewMainModel(&app.Application{Config: app.DefaultConfig()}, app.ModeDo)
	m.width = 60
	m.height = 30

	// Force scroll info + other right-side tokens.
	m.viewport = viewport.New(20, 3)
	m.viewport.SetContent(strings.Repeat("x\n", 100))
	m.stickToBottom = false
	m.unseenCount = 1234
	m.loading = true
	m.cancelQueued = true
	m.queuedRequests = make([]queuedRequest, 99)
	m.title = strings.Repeat("very-long-title-", 10)

	out := m.renderStatusBar()
	if !regexp.MustCompile(`\b\d{2}:\d{2}\b`).MatchString(out) {
		t.Fatalf("expected status bar to include a time token, got: %q", out)
	}

	maxW := m.chatAreaWidth()
	for _, line := range strings.Split(out, "\n") {
		if got := lipgloss.Width(line); got > maxW {
			t.Fatalf("status bar line overflows: got %d > %d: %q", got, maxW, line)
		}
	}
}
