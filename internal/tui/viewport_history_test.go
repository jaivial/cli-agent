package tui

import (
	"testing"

	"cli-agent/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

func applyWindowSize(t *testing.T, m *MainModel) *MainModel {
	t.Helper()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	out, ok := updated.(*MainModel)
	if !ok {
		t.Fatalf("expected *MainModel, got %T", updated)
	}
	return out
}

func sendEnter(t *testing.T, m *MainModel, value string) *MainModel {
	t.Helper()
	m.input.SetValue(value)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	out, ok := updated.(*MainModel)
	if !ok {
		t.Fatalf("expected *MainModel, got %T", updated)
	}
	return out
}

func pressKey(t *testing.T, m *MainModel, keyType tea.KeyType) *MainModel {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: keyType})
	out, ok := updated.(*MainModel)
	if !ok {
		t.Fatalf("expected *MainModel, got %T", updated)
	}
	return out
}

func TestInputHistoryArrowNavigation(t *testing.T) {
	m := NewMainModel(nil, app.ModeCreate)
	m = applyWindowSize(t, m)

	m = sendEnter(t, m, "first prompt")
	m = sendEnter(t, m, "second prompt")

	m = pressKey(t, m, tea.KeyUp)
	if got := m.input.Value(); got != "second prompt" {
		t.Fatalf("up once: got %q, want %q", got, "second prompt")
	}

	m = pressKey(t, m, tea.KeyUp)
	if got := m.input.Value(); got != "first prompt" {
		t.Fatalf("up twice: got %q, want %q", got, "first prompt")
	}

	m = pressKey(t, m, tea.KeyDown)
	if got := m.input.Value(); got != "second prompt" {
		t.Fatalf("down from oldest: got %q, want %q", got, "second prompt")
	}

	m = pressKey(t, m, tea.KeyDown)
	if got := m.input.Value(); got != "" {
		t.Fatalf("down to draft: got %q, want empty", got)
	}
}

func TestPlanModeReasoningProgressIsSuppressed(t *testing.T) {
	m := NewMainModel(nil, app.ModePlan)
	m = applyWindowSize(t, m)
	m.loading = true

	updated, _ := m.Update(progressUpdateMsg{event: app.ProgressEvent{
		Kind: "reasoning",
		Text: "hidden reasoning line",
	}})
	out, ok := updated.(*MainModel)
	if !ok {
		t.Fatalf("expected *MainModel, got %T", updated)
	}

	for _, msg := range out.messages {
		if msg.IsStatus {
			t.Fatalf("unexpected status message in plan reasoning path: %+v", msg)
		}
	}
}
