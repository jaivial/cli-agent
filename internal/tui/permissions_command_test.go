package tui

import (
	"strings"
	"testing"

	"cli-agent/internal/app"
)

func TestHandlePermissionsCommand_Status(t *testing.T) {
	model := &MainModel{
		app: &app.Application{Config: app.DefaultConfig()},
	}

	handled, content, role, cmd := model.handlePermissionsCommand("/permissions")
	if !handled {
		t.Fatalf("expected command to be handled")
	}
	if role != "system" {
		t.Fatalf("expected system role, got %q", role)
	}
	if cmd != nil {
		t.Fatalf("expected no follow-up cmd, got one")
	}
	if !strings.Contains(content, "desired: full-access") {
		t.Fatalf("unexpected status content: %q", content)
	}
}

func TestHandlePermissionsCommand_InvalidMode(t *testing.T) {
	model := &MainModel{
		app: &app.Application{Config: app.DefaultConfig()},
	}

	handled, content, role, cmd := model.handlePermissionsCommand("/permissions wrong-mode")
	if !handled {
		t.Fatalf("expected command to be handled")
	}
	if role != "error" {
		t.Fatalf("expected error role, got %q", role)
	}
	if cmd != nil {
		t.Fatalf("expected no follow-up cmd for invalid mode")
	}
	if !strings.Contains(content, "invalid permissions mode") {
		t.Fatalf("unexpected error content: %q", content)
	}
}
