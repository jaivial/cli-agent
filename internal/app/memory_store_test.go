package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestMemoryStoreCreateAndLoadSessionForWorkDir(t *testing.T) {
	store, err := NewSQLiteSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("sqlite store: %v", err)
	}
	workDir := t.TempDir()

	sess, err := store.CreateSession(workDir)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.ID == "" {
		t.Fatalf("expected session id")
	}

	if err := store.AppendMessage(StoredMessage{
		SessionID: sess.ID,
		Role:      "user",
		Content:   "hello",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("append message: %v", err)
	}

	loaded, msgs, err := store.LoadSessionForWorkDir(workDir, sess.ID)
	if err != nil {
		t.Fatalf("load by workdir: %v", err)
	}
	if loaded.ID != sess.ID {
		t.Fatalf("session id mismatch: got %s want %s", loaded.ID, sess.ID)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestEstimateSessionWorkDurationUsesActiveWindows(t *testing.T) {
	base := time.Now().Add(-4 * time.Hour)
	msgs := []StoredMessage{
		{CreatedAt: base},
		{CreatedAt: base.Add(10 * time.Minute)},
		{CreatedAt: base.Add(40 * time.Minute)}, // gap > workIdleGap starts a new window
	}

	duration, last := estimateSessionWorkDuration(msgs, time.Now())
	if last.IsZero() {
		t.Fatalf("expected last message time")
	}
	if duration < 10*time.Minute {
		t.Fatalf("duration too small: %v", duration)
	}
	// First window contributes ~10m; second window contributes at least workMinSlice.
	if duration > 12*time.Minute {
		t.Fatalf("duration too large: %v", duration)
	}
}

func TestMemoryStoreSaveAndLoadPromptHistory(t *testing.T) {
	store := NewFileSessionStore(t.TempDir())
	workDir := t.TempDir()

	in := []string{
		" first ",
		"",
		"first",
		"second",
		"second",
		"third",
	}
	if err := store.SavePromptHistory(workDir, in); err != nil {
		t.Fatalf("save prompt history: %v", err)
	}

	got, err := store.LoadPromptHistory(workDir)
	if err != nil {
		t.Fatalf("load prompt history: %v", err)
	}
	want := []string{"first", "second", "third"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("history mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestMemoryStoreLoadPromptHistorySupportsRawArray(t *testing.T) {
	store := NewFileSessionStore(t.TempDir())
	workDir := t.TempDir()

	projectID, _, err := store.projectID(workDir)
	if err != nil {
		t.Fatalf("project id: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(store.promptHistoryPath(projectID)), 0755); err != nil {
		t.Fatalf("mkdir prompt history dir: %v", err)
	}
	raw := `[" one ","","one","two"]`
	if err := os.WriteFile(store.promptHistoryPath(projectID), []byte(raw), 0644); err != nil {
		t.Fatalf("write raw history: %v", err)
	}

	got, err := store.LoadPromptHistory(workDir)
	if err != nil {
		t.Fatalf("load prompt history: %v", err)
	}
	want := []string{"one", "two"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("history mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}
