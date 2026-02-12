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

func TestModelPickerCommandAndSelection(t *testing.T) {
	m := NewMainModel(nil, app.ModeCreate)
	m = applyWindowSize(t, m)

	m = sendEnter(t, m, "/model")
	if !m.modelPickerActive {
		t.Fatalf("expected model picker to open after /model")
	}
	if m.modelPickerIndex != 0 {
		t.Fatalf("expected default picker index 0, got %d", m.modelPickerIndex)
	}

	m = pressKey(t, m, tea.KeyDown)
	if m.modelPickerIndex != 1 {
		t.Fatalf("expected picker index 1 after down, got %d", m.modelPickerIndex)
	}

	m = pressKey(t, m, tea.KeyEnter)
	if m.modelPickerActive {
		t.Fatalf("expected model picker to close after selecting")
	}
	last := m.messages[len(m.messages)-1]
	if last.Content != "model set to glm-5" {
		t.Fatalf("unexpected final message: %q", last.Content)
	}
}

func TestQueueUserTurnWhileAgentRunning(t *testing.T) {
	m := NewMainModel(nil, app.ModeCreate)
	m = applyWindowSize(t, m)
	m.loading = true

	m = sendEnter(t, m, "queued prompt")
	if got := len(m.queuedRequests); got != 1 {
		t.Fatalf("expected 1 queued request, got %d", got)
	}
	if got := m.input.Value(); got != "" {
		t.Fatalf("expected input cleared after queue, got %q", got)
	}
	if got := len(m.inputHistory); got != 1 || m.inputHistory[0] != "queued prompt" {
		t.Fatalf("expected input history updated, got %+v", m.inputHistory)
	}
}

func TestEscCancelsWhileAgentRunning(t *testing.T) {
	m := NewMainModel(nil, app.ModeCreate)
	m = applyWindowSize(t, m)
	m.loading = true

	canceled := false
	m.activeCancel = func() { canceled = true }

	m = pressKey(t, m, tea.KeyEsc)
	if !canceled {
		t.Fatalf("expected cancel func to be invoked")
	}
	if !m.cancelQueued {
		t.Fatalf("expected cancelQueued to be set")
	}
	if got := m.loadingText; got != "canceling..." {
		t.Fatalf("expected cancel loading text, got %q", got)
	}
}

func TestPermissionsPickerCommandAndSelection(t *testing.T) {
	m := NewMainModel(nil, app.ModeCreate)
	m = applyWindowSize(t, m)

	m = sendEnter(t, m, "/permissions")
	if !m.permissionsPickerActive {
		t.Fatalf("expected permissions picker to open after /permissions")
	}
	if m.permissionsPickerIndex != 0 {
		t.Fatalf("expected default picker index 0, got %d", m.permissionsPickerIndex)
	}

	m = pressKey(t, m, tea.KeyDown)
	if m.permissionsPickerIndex != 1 {
		t.Fatalf("expected picker index 1 after down, got %d", m.permissionsPickerIndex)
	}

	m = pressKey(t, m, tea.KeyEnter)
	if m.permissionsPickerActive {
		t.Fatalf("expected permissions picker to close after selecting")
	}
	last := m.messages[len(m.messages)-1]
	if last.Content != "permissions set to dangerously-full-access" {
		t.Fatalf("unexpected final message: %q", last.Content)
	}
}
