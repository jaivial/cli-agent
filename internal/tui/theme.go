package tui

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Apple-ish palette tuned for dark terminal backgrounds. (Terminals vary, so we
// keep contrast high and avoid relying on background colors.)
const (
	colorFg         = "#EDEDED" // primary label
	colorMuted      = "#8E8E93" // secondary label
	colorSubtle     = "#636366" // tertiary label
	colorBorder     = "#3A3A3C" // separators/borders
	colorBorderSoft = "#2C2C2E"

	colorAccent    = "#0A84FF" // system blue
	colorAccent2   = "#64D2FF" // system cyan (soft highlight)
	colorSuccess   = "#30D158"
	colorWarning   = "#FF9F0A"
	colorError     = "#FF453A"
	colorPurple    = "#BF5AF2"
	colorPink      = "#FF2D55"
	colorScrollbar = "#5A5A5E"
)

const (
	defaultAnimTick       = 80   // ms
	defaultFadeDuration   = 240  // ms
	defaultStartupShimmer = 1200 // ms
)

func animationsEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("EAI_TUI_ANIM"))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func parseHexColor(hex string) (r, g, b int, ok bool) {
	hex = strings.TrimSpace(hex)
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	u, err := strconv.ParseUint(hex, 16, 24)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(u>>16) & 0xFF, int(u>>8) & 0xFF, int(u) & 0xFF, true
}

func lerpInt(a, b int, t float64) int {
	return int(math.Round(float64(a) + (float64(b)-float64(a))*t))
}

func blendHex(from, to string, t float64) string {
	t = clamp01(t)
	r1, g1, b1, ok1 := parseHexColor(from)
	r2, g2, b2, ok2 := parseHexColor(to)
	if !ok1 || !ok2 {
		// Fail "safe": fall back to the to-color if parsing fails.
		if strings.TrimSpace(to) != "" {
			return to
		}
		return from
	}
	r := lerpInt(r1, r2, t)
	g := lerpInt(g1, g2, t)
	b := lerpInt(b1, b2, t)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// shimmerText renders a subtle "Apple loading" shimmer by moving a highlight
// band across the string. It only styles non-space runes to keep output small.
func shimmerText(text string, frame int, baseHex, highlightHex string) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return ""
	}
	if n == 1 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(highlightHex)).Render(text)
	}

	// Let the highlight sweep in/out a bit so it doesn't "teleport" at edges.
	const pad = 4
	cycle := n + pad*2
	pos := 0
	if cycle > 0 {
		pos = frame%cycle - pad
	}

	var b strings.Builder
	for i, r := range runes {
		if r == ' ' || r == '\t' {
			b.WriteRune(r)
			continue
		}

		dist := i - pos
		if dist < 0 {
			dist = -dist
		}

		// Soft falloff: closest chars get the highlight, then blend back to base.
		t := 0.0
		switch {
		case dist <= 0:
			t = 1
		case dist == 1:
			t = 0.75
		case dist == 2:
			t = 0.35
		default:
			t = 0
		}

		color := blendHex(baseHex, highlightHex, t)
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(string(r)))
	}
	return b.String()
}
