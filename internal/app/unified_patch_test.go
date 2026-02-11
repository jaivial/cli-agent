package app

import "testing"

func TestApplyUnifiedPatch_RemovesTrailingNewlineWhenMarked(t *testing.T) {
	oldContent := "line1\n"
	patch := "@@ -1 +1 @@\n-line1\n+line1\n\\ No newline at end of file\n"

	out, err := ApplyUnifiedPatch(oldContent, patch)
	if err != nil {
		t.Fatalf("unexpected error applying patch: %v", err)
	}
	if out != "line1" {
		t.Fatalf("expected output without trailing newline, got %q", out)
	}
}

func TestApplyUnifiedPatch_RestoresTrailingNewlineWhenOnlyOldWasMarked(t *testing.T) {
	oldContent := "line1"
	patch := "@@ -1 +1 @@\n-line1\n\\ No newline at end of file\n+line1\n"

	out, err := ApplyUnifiedPatch(oldContent, patch)
	if err != nil {
		t.Fatalf("unexpected error applying patch: %v", err)
	}
	if out != "line1\n" {
		t.Fatalf("expected output with trailing newline, got %q", out)
	}
}
