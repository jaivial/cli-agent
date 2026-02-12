package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	launchArtCacheMu sync.Mutex
	launchArtCache   = make(map[string]string)
)

const fallbackLaunchTitle = `
 ______    ___    ___
| ____|  / _ \  |_ _|
|  _|   | | | |  | |
| |___  | |_| |  | |
|_____|  \___/  |___|`

const fallbackLaunchMonkey = `
            __,__
   .--.  .-"     "-.  .--.
  / .. \/  .-. .-.  \/ .. \
 | |  '|  /   Y   \  |'  | |
 | \   \  \ 0 | 0 /  /   / |
  \ '- ,\.-"""""""-./, -' /
   ''-' /_   ^   _\ '-''
       |  \._ _./  |
       \   \~'~/   /
        '._ '-=-' _.'
           '-----'`

func (m *MainModel) shouldRenderLaunchArt() bool {
	if m.loading || m.resumePickerActive {
		return false
	}
	for _, msg := range m.messages {
		if msg.IsFileEdit {
			return false
		}
		switch msg.Role {
		case "user", "assistant", "error":
			return false
		}
	}
	return true
}

func (m *MainModel) renderLaunchArt(width int) string {
	if width < 40 {
		return ""
	}
	titleWidth := width - 20
	if titleWidth > 80 {
		titleWidth = 80
	}
	if titleWidth < 24 {
		titleWidth = width - 6
	}
	title := readASCIIAsset("title.txt")
	if strings.TrimSpace(title) == "" {
		title = fallbackLaunchTitle
	}
	titleHeight := 12

	titleLines := fitASCIIBlock(normalizeASCIIBlock(title), titleWidth, titleHeight)
	if len(titleLines) == 0 {
		return ""
	}

	titleBlock := strings.Join(titleLines, "\n")

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorAccent)).
		Bold(true)

	// Subtle startup shimmer for a more "Cupertino" feel.
	if m.animEnabled && !m.startupUntil.IsZero() && time.Now().Before(m.startupUntil) {
		base := blendHex(colorAccent, colorAccent2, 0.25)
		highlight := colorFg
		styled := make([]string, 0, len(titleLines))
		for i, line := range titleLines {
			styled = append(styled, shimmerText(line, m.spinnerPos+i*2, base, highlight))
		}
		return lipgloss.NewStyle().
			Width(width).
			Align(lipgloss.Center).
			Render(strings.Join(styled, "\n"))
	}

	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(titleStyle.Render(titleBlock))
}

func loadLaunchArt(titleCols, titleRows, monkeyCols, monkeyRows int) (title, monkey string) {
	title = loadLaunchArtVariant("title", titleCols, titleRows, fallbackLaunchTitle)
	monkey = loadLaunchArtVariant("monkey", monkeyCols, monkeyRows, fallbackLaunchMonkey)
	return title, monkey
}

func loadLaunchArtVariant(base string, cols, rows int, fallback string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return fallback
	}
	key := fmt.Sprintf("%s:%dx%d", base, cols, rows)

	launchArtCacheMu.Lock()
	if cached, ok := launchArtCache[key]; ok {
		launchArtCacheMu.Unlock()
		return cached
	}
	launchArtCacheMu.Unlock()

	out := renderImageAssetToASCII(base, cols, rows)
	if strings.TrimSpace(out) == "" {
		out = readASCIIAsset(base + ".txt")
	}
	if strings.TrimSpace(out) == "" {
		out = fallback
	}

	launchArtCacheMu.Lock()
	launchArtCache[key] = out
	launchArtCacheMu.Unlock()
	return out
}

func readASCIIAsset(name string) string {
	for _, cand := range candidateAssetPaths(name) {
		cand = strings.TrimSpace(cand)
		if cand == "" {
			continue
		}
		data, err := os.ReadFile(cand)
		if err != nil {
			continue
		}
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		if strings.TrimSpace(content) == "" {
			continue
		}
		return content
	}
	return ""
}

func candidateAssetPaths(name string) []string {
	candidates := []string{
		filepath.Join("ascii", name),
		filepath.Join("/home/jaime/Downloads/projects/cli-agent/ascii", name),
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		candidates = append(candidates, filepath.Join(wd, "ascii", name))
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "ascii", name),
			filepath.Join(exeDir, "..", "ascii", name),
		)
	}

	out := make([]string, 0, len(candidates))
	seen := make(map[string]bool)
	for _, cand := range candidates {
		cand = strings.TrimSpace(cand)
		if cand == "" || seen[cand] {
			continue
		}
		seen[cand] = true
		out = append(out, cand)
	}
	return out
}

func renderImageAssetToASCII(base string, cols, rows int) string {
	if _, err := exec.LookPath("chafa"); err != nil {
		return ""
	}
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 40
	}
	size := fmt.Sprintf("%dx%d", cols, rows)

	extensions := []string{".png", ".jpg", ".jpeg", ".webp"}
	for _, ext := range extensions {
		for _, cand := range candidateAssetPaths(base + ext) {
			if strings.TrimSpace(cand) == "" {
				continue
			}
			if st, err := os.Stat(cand); err != nil || st.IsDir() {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			cmd := exec.CommandContext(
				ctx,
				"chafa",
				"-f", "symbols",
				"--symbols", "ascii",
				"-c", "none",
				"--size", size,
				cand,
			)
			out, err := cmd.Output()
			cancel()
			if err != nil {
				continue
			}
			text := strings.ReplaceAll(string(out), "\r\n", "\n")
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			return text
		}
	}
	return ""
}

func normalizeASCIIBlock(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}

	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil
	}

	minLead := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lead := leadingSpaces(line)
		if minLead == -1 || lead < minLead {
			minLead = lead
		}
	}
	if minLead > 0 {
		for i := range lines {
			line := lines[i]
			if len(line) <= minLead {
				lines[i] = ""
				continue
			}
			lines[i] = line[minLead:]
		}
	}
	return lines
}

func leadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func fitASCIIBlock(lines []string, maxWidth int, maxHeight int) []string {
	if len(lines) == 0 || maxWidth <= 0 {
		return nil
	}
	if maxHeight > 0 && len(lines) > maxHeight {
		lines = downsampleLines(lines, maxHeight)
	}
	fitted := make([]string, 0, len(lines))
	for _, line := range lines {
		fitted = append(fitted, shrinkLineToWidth(line, maxWidth))
	}
	return fitted
}

func shrinkLineToWidth(line string, maxWidth int) string {
	r := []rune(line)
	if len(r) <= maxWidth {
		return line
	}
	// Downsample across the full line instead of center-cropping so art keeps
	// its overall shape while becoming smaller.
	out := make([]rune, 0, maxWidth)
	srcLast := len(r) - 1
	dstLast := maxWidth - 1
	if dstLast <= 0 {
		return string(r[:1])
	}
	for i := 0; i < maxWidth; i++ {
		src := i * srcLast / dstLast
		if src < 0 {
			src = 0
		}
		if src > srcLast {
			src = srcLast
		}
		out = append(out, r[src])
	}
	return strings.TrimRight(string(out), " ")
}

func downsampleLines(lines []string, maxHeight int) []string {
	if maxHeight <= 0 || len(lines) <= maxHeight {
		return lines
	}
	out := make([]string, 0, maxHeight)
	srcLast := len(lines) - 1
	dstLast := maxHeight - 1
	if dstLast <= 0 {
		return []string{lines[srcLast/2]}
	}
	for i := 0; i < maxHeight; i++ {
		src := i * srcLast / dstLast
		if src < 0 {
			src = 0
		}
		if src > srcLast {
			src = srcLast
		}
		out = append(out, lines[src])
	}
	return out
}
