package tui

import (
	"path/filepath"
	"sort"
	"strings"

	"cli-agent/internal/app"
)

type pathHit struct {
	path string
	kind string // read_file|list_dir|search_files|grep|exec|other
}

func buildPlanContextForExecution(events []app.ProgressEvent) string {
	if len(events) == 0 {
		return ""
	}

	// Prefer tool "completed" events; those include normalized args like Path/Command.
	var hits []pathHit

	for _, ev := range events {
		if strings.ToLower(strings.TrimSpace(ev.Kind)) != "tool" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(ev.ToolStatus)) != "completed" {
			continue
		}
		p := strings.TrimSpace(ev.Path)
		if p == "" {
			continue
		}
		tool := strings.ToLower(strings.TrimSpace(ev.Tool))
		hits = append(hits, pathHit{path: p, kind: tool})
	}

	readFiles := uniquePreserveOrder(filterByKind(hits, "read_file"))
	listDirs := uniquePreserveOrder(filterByKind(hits, "list_dir"))
	searchRoots := uniquePreserveOrder(filterKinds(hits, []string{"search_files", "grep"}))

	targetFile := pickLikelyTargetFile(readFiles)
	targetDir := ""
	if targetFile != "" {
		targetDir = filepath.Dir(targetFile)
	}

	var b strings.Builder
	if targetFile != "" {
		b.WriteString("Key facts discovered in Plan mode:\n")
		b.WriteString("- Likely target file: ")
		b.WriteString(targetFile)
		b.WriteString("\n")
		if targetDir != "" {
			b.WriteString("- Likely target directory: ")
			b.WriteString(targetDir)
			b.WriteString("\n")
		}
	}

	explored := collapseExploredPaths(listDirs, searchRoots, targetDir)
	if len(explored) > 0 {
		if b.Len() == 0 {
			b.WriteString("Key facts discovered in Plan mode:\n")
		}
		b.WriteString("- Explored paths: ")
		b.WriteString(strings.Join(explored, ", "))
		b.WriteString("\n")
	}

	// Include a compact tool timeline for additional continuity; it is trimmed internally.
	if timeline := strings.TrimSpace(FormatTimeline(events)); timeline != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Tool trace:\n")
		b.WriteString(timeline)
		b.WriteString("\n")
	}

	return strings.TrimSpace(b.String())
}

func filterByKind(hits []pathHit, kind string) []string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		if strings.ToLower(strings.TrimSpace(h.kind)) == kind {
			out = append(out, h.path)
		}
	}
	return out
}

func filterKinds(hits []pathHit, kinds []string) []string {
	set := make(map[string]struct{}, len(kinds))
	for _, k := range kinds {
		set[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		if _, ok := set[strings.ToLower(strings.TrimSpace(h.kind))]; ok {
			out = append(out, h.path)
		}
	}
	return out
}

func uniquePreserveOrder(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" || seen[it] {
			continue
		}
		seen[it] = true
		out = append(out, it)
	}
	return out
}

func pickLikelyTargetFile(readFiles []string) string {
	if len(readFiles) == 0 {
		return ""
	}
	// Prefer an HTML file, especially index.html.
	candidates := make([]string, 0, len(readFiles))
	for _, p := range readFiles {
		base := strings.ToLower(filepath.Base(p))
		if base == "index.html" {
			return p
		}
		if strings.HasSuffix(strings.ToLower(p), ".html") {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) > 0 {
		return candidates[len(candidates)-1]
	}
	return readFiles[len(readFiles)-1]
}

func collapseExploredPaths(listDirs []string, searchRoots []string, targetDir string) []string {
	all := append([]string{}, listDirs...)
	all = append(all, searchRoots...)
	all = uniquePreserveOrder(all)

	// If we have a target dir, prefer showing it first.
	if targetDir != "" {
		targetDir = strings.TrimSpace(targetDir)
		for i, p := range all {
			if p == targetDir {
				all = append([]string{p}, append(all[:i], all[i+1:]...)...)
				break
			}
		}
	}

	// Keep it compact.
	if len(all) > 6 {
		all = all[:6]
	}

	// Make stable for tests when targetDir isn't forcing order.
	if targetDir == "" {
		cp := append([]string{}, all...)
		sort.Strings(cp)
		return cp
	}
	return all
}
