package tui

import (
	"strings"
	"testing"
)

func TestBuildPlanImplementationPrompt_IncludesPlanContextAndChecklist(t *testing.T) {
	planText := "[ ] Edit /home/jaime/Desktop/eai/index.html\n[ ] Verify all images load"
	planContext := "Key facts discovered in Plan mode:\n- Likely target file: /home/jaime/Desktop/eai/index.html"

	out := buildPlanImplementationPrompt(planText, planContext)

	if !strings.Contains(out, "Plan-mode context") {
		t.Fatalf("expected prompt to include plan context header, got: %q", out)
	}
	if !strings.Contains(out, "/home/jaime/Desktop/eai/index.html") {
		t.Fatalf("expected prompt to include target path, got: %q", out)
	}
	if !strings.Contains(out, "Approved plan (verbatim):") {
		t.Fatalf("expected prompt to include approved plan header, got: %q", out)
	}
	if !strings.Contains(out, "Verify all images load") {
		t.Fatalf("expected prompt to include checklist content, got: %q", out)
	}
	if strings.Contains(strings.ToLower(out), "in this repository") {
		t.Fatalf("did not expect repository-scoping language, got: %q", out)
	}
}
