package tui

import (
	"fmt"
	"strings"
	"testing"

	"cli-agent/internal/app"
)

func TestFormatTimeline_GroupsToolEvents(t *testing.T) {
	events := []app.ProgressEvent{
		{Kind: "thinking", Text: "Planning approach"},
		{Kind: "tool", Tool: "read_file", ToolStatus: "pending", Path: "internal/app/app.go"},
		{Kind: "tool", Tool: "read_file", ToolStatus: "completed", Path: "internal/app/app.go"},
		{Kind: "tool", Tool: "grep", ToolStatus: "completed", Command: "updateEditingArticleTypeAtom", Path: "frontend"},
		{Kind: "tool", Tool: "edit_file", ToolStatus: "completed", Path: "frontend/src/ArticleItem.tsx"},
		{Kind: "tool", Tool: "exec", ToolStatus: "error", Command: "npm run build"},
	}

	got := FormatTimeline(events)
	want := strings.Join([]string{
		"• Thinking",
		"  └ Planning approach",
		"• Explored",
		"  └ Read `internal/app/app.go`",
		"  └ Search `updateEditingArticleTypeAtom` in `frontend`",
		"• Edited",
		"  └ Edit `frontend/src/ArticleItem.tsx`",
		"• Executed",
		"  └ Run `npm run build` (failed)",
	}, "\n")

	if got != want {
		t.Fatalf("unexpected timeline output\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestFormatTimeline_EmptyWhenNoRenderableEvents(t *testing.T) {
	events := []app.ProgressEvent{
		{Kind: ""},
		{Kind: "tool", Tool: "read_file", ToolStatus: "pending", Path: "x.go"},
	}

	if got := FormatTimeline(events); got != "" {
		t.Fatalf("expected empty timeline, got: %q", got)
	}
}

func TestFormatTimeline_TrimsLongTimelines(t *testing.T) {
	events := []app.ProgressEvent{
		{Kind: "thinking", Text: "Planning approach"},
	}
	for i := 0; i < 20; i++ {
		events = append(events, app.ProgressEvent{
			Kind:       "tool",
			Tool:       "read_file",
			ToolStatus: "completed",
			Path:       fmt.Sprintf("file-%02d.txt", i),
		})
	}

	got := FormatTimeline(events)
	if !strings.Contains(got, "• Thinking") {
		t.Fatalf("expected thinking entry to be preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "`file-19.txt`") {
		t.Fatalf("expected latest steps to be kept, got:\n%s", got)
	}
	if strings.Contains(got, "`file-00.txt`") {
		t.Fatalf("expected old steps to be trimmed, got:\n%s", got)
	}
	if !strings.Contains(got, "earlier steps omitted") {
		t.Fatalf("expected truncation marker, got:\n%s", got)
	}
}
