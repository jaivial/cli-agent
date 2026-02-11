package tui

import (
	"strings"
	"testing"

	"cli-agent/internal/app"
)

func TestBuildPlanDisplayIfApplicable_FromProposedPlanBlock(t *testing.T) {
	raw := `<proposed_plan>
# Improve Plan UX
- Add plan card renderer
- Add yes/no prompt
- Auto switch mode on approval
</proposed_plan>`

	display, planText, ok := buildPlanDisplayIfApplicable(app.ModePlan, raw)
	if !ok {
		t.Fatalf("expected plan to be detected")
	}
	if !strings.Contains(display, "plan") {
		t.Fatalf("expected display to include a plan lead, got: %q", display)
	}
	if !strings.Contains(display, "[ ] Add plan card renderer") {
		t.Fatalf("expected checklist entry in display, got: %q", display)
	}
	if strings.Contains(display, "**") {
		t.Fatalf("expected markdown emphasis removed, got: %q", display)
	}
	if !strings.Contains(planText, "[ ] Add yes/no prompt") {
		t.Fatalf("expected plan text checklist, got: %q", planText)
	}
}

func TestBuildPlanDisplayIfApplicable_RejectsNonPlanMode(t *testing.T) {
	raw := `<proposed_plan>
- one
- two
</proposed_plan>`

	_, _, ok := buildPlanDisplayIfApplicable(app.ModeCreate, raw)
	if ok {
		t.Fatalf("expected non-plan mode to reject plan formatting")
	}
}

func TestBuildPlanDisplayIfApplicable_CleansMarkdownItems(t *testing.T) {
	raw := "<proposed_plan>\n" +
		"1. **Analyze** current behavior\n" +
		"2. __Design__ new flow\n" +
		"3. `Implement` and verify\n" +
		"</proposed_plan>"

	display, _, ok := buildPlanDisplayIfApplicable(app.ModePlan, raw)
	if !ok {
		t.Fatalf("expected plan detection")
	}
	if strings.Contains(display, "**") || strings.Contains(display, "__") || strings.Contains(display, "`") {
		t.Fatalf("expected markdown symbols removed, got: %q", display)
	}
	if !strings.Contains(display, "[ ] Analyze current behavior") {
		t.Fatalf("expected cleaned checklist item, got: %q", display)
	}
}
