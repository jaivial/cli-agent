package tui

import (
	"strings"
	"testing"
	"time"

	"cli-agent/internal/app"
)

func TestBuildPlanContextForExecution_PinsLikelyTargetFile(t *testing.T) {
	events := []app.ProgressEvent{
		{
			Kind:       "tool",
			Tool:       "list_dir",
			ToolStatus: "completed",
			Path:       "/home/jaime/Desktop/eai",
			At:         time.Now(),
		},
		{
			Kind:       "tool",
			Tool:       "read_file",
			ToolStatus: "completed",
			Path:       "/home/jaime/Desktop/eai/index.html",
			At:         time.Now(),
		},
	}

	out := buildPlanContextForExecution(events)
	if !strings.Contains(out, "Likely target file: /home/jaime/Desktop/eai/index.html") {
		t.Fatalf("expected prompt context to pin index.html, got: %q", out)
	}
	if !strings.Contains(out, "Likely target directory: /home/jaime/Desktop/eai") {
		t.Fatalf("expected prompt context to include directory, got: %q", out)
	}
	if !strings.Contains(out, "Tool trace:") {
		t.Fatalf("expected prompt context to include tool trace, got: %q", out)
	}
}
