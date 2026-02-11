package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteChat_ListFilesLocalFastpath(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmp, "bdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".hidden"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	a, err := NewApplication(Config{}, true)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := a.ExecuteChat(ctx, ModeCreate, "list files in this folder")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.txt") {
		t.Fatalf("expected output to include a.txt, got: %q", out)
	}
	if !strings.Contains(out, "bdir/") {
		t.Fatalf("expected output to include bdir/, got: %q", out)
	}
	if !strings.Contains(out, ".hidden") {
		t.Fatalf("expected output to include hidden files, got: %q", out)
	}
}

func TestExecuteChat_ToolModeRunsToolsInMock(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "x.go"), []byte("package x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	a, err := NewApplication(Config{}, true)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Avoid the local "list files" fastpath; force tool-mode execution via mock tool calls.
	out, err := a.ExecuteChat(ctx, ModeCreate, "directory contents")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "x.go") {
		t.Fatalf("expected output to include x.go, got: %q", out)
	}
}

func TestExecuteChat_WritesHTMLWebsiteToIndex(t *testing.T) {
	tmp := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	a, err := NewApplication(Config{}, true)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := a.ExecuteChat(ctx, ModeCreate, "create a website for my pet store only using html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "index.html") || !strings.Contains(out, ":1") {
		t.Fatalf("expected clickable path mentioning index.html:1, got: %q", out)
	}

	data, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(string(data)), "<!doctype html") {
		t.Fatalf("expected index.html to contain doctype, got: %q", string(data))
	}
}

func TestExecuteChat_PlanModeReturnsPlanForAction(t *testing.T) {
	tmp := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	a, err := NewApplication(Config{}, true)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := a.ExecuteChat(ctx, ModePlan, "create a website with html only about my pet store")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(out), "<proposed_plan>") {
		t.Fatalf("expected plan-mode to return a proposed plan block, got: %q", out)
	}
	if !strings.Contains(out, "- [ ]") {
		t.Fatalf("expected plan-mode checklist items, got: %q", out)
	}
	if _, err := os.Stat(filepath.Join(tmp, "index.html")); err == nil {
		t.Fatalf("did not expect index.html to be created in plan mode")
	}
}

func TestExecuteChat_PlanModeDoesNotScaffoldReact(t *testing.T) {
	tmp := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	a, err := NewApplication(Config{}, true)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := a.ExecuteChat(ctx, ModePlan, "create a react website inside a new folder for my pet store")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(out), "<proposed_plan>") {
		t.Fatalf("expected plan response, got: %q", out)
	}
	if _, err := os.Stat(filepath.Join(tmp, "pet-store")); !os.IsNotExist(err) {
		t.Fatalf("did not expect pet-store folder to be scaffolded in plan mode")
	}
}

func TestExecuteChat_PlanModeStreamsProgressEvents(t *testing.T) {
	a, err := NewApplication(Config{}, true)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var events []ProgressEvent
	out, err := a.ExecuteChatWithProgressEvents(ctx, ModePlan, "analyze this repository and propose a plan", func(ev ProgressEvent) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(out), "<proposed_plan>") {
		t.Fatalf("expected plan output, got: %q", out)
	}
	if len(events) == 0 {
		t.Fatalf("expected progress events in plan mode")
	}

	foundReasoning := false
	foundTool := false
	for _, ev := range events {
		if ev.Kind == "reasoning" {
			foundReasoning = true
		}
		if ev.Kind == "tool" {
			foundTool = true
		}
	}
	if !foundReasoning {
		t.Fatalf("expected reasoning progress events in plan mode")
	}
	if !foundTool {
		t.Fatalf("expected tool progress events in plan mode")
	}
}
