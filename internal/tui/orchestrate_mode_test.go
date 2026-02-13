package tui

import (
	"cli-agent/internal/app"
	"testing"
)

func TestNewMainModel_UsesProvidedMode(t *testing.T) {
	m := NewMainModel(nil, app.ModeOrchestrate)
	if m.mode != app.ModeOrchestrate {
		t.Fatalf("expected mode %q, got %q", app.ModeOrchestrate, m.mode)
	}
	if m.modeIndex != 0 {
		t.Fatalf("expected modeIndex 0 for default mode list position, got %d", m.modeIndex)
	}
}
