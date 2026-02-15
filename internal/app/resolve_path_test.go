package app

import (
	"testing"
)

func TestResolvePath_ExpandsHomeAndCommonDirs(t *testing.T) {
	t.Setenv("HOME", "/home/alice")

	l := &AgentLoop{WorkDir: "/work"}

	if got := l.resolvePath("~/Desktop/eai"); got != "/home/alice/Desktop/eai" {
		t.Fatalf("expected ~ expansion, got %q", got)
	}
	if got := l.resolvePath("Desktop/eai"); got != "/home/alice/Desktop/eai" {
		t.Fatalf("expected Desktop/ to be home-relative, got %q", got)
	}
	if got := l.resolvePath("Downloads/file.txt"); got != "/home/alice/Downloads/file.txt" {
		t.Fatalf("expected Downloads/ to be home-relative, got %q", got)
	}
	if got := l.resolvePath("website/index.html"); got != "/work/website/index.html" {
		t.Fatalf("expected relative to WorkDir, got %q", got)
	}
	if got := l.resolvePath("/etc/hosts"); got != "/etc/hosts" {
		t.Fatalf("expected absolute path preserved, got %q", got)
	}
}
