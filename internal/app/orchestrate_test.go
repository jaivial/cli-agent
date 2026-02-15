package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTrimmedOrchestrateListMarker(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"dash", "- write tests", "write tests"},
		{"hyphenated", "1 - write tests", "write tests"},
		{"alphaParen", "c) run checks", "run checks"},
		{"alphaParen2", "(d) archive logs", "archive logs"},
		{"alphaDot", "c. archive logs", "archive logs"},
		{"alphaParenSuffix", "a) run lint", "run lint"},
		{"numbered", "1. run all", "run all"},
		{"numberedParen", "1) run all", "run all"},
		{"paren", "2) generate summary", "generate summary"},
		{"star", "* inspect data", "inspect data"},
		{"plain", "compose report", "compose report"},
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := trimmedOrchestrateListMarker(tc.in)
			if got != tc.want {
				t.Fatalf("trimmedOrchestrateListMarker(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCountOrchestrateWords(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"spaces", "   ", 0},
		{"single", "hello", 1},
		{"multiple", "build a report", 3},
		{"punctuation", "hello, world!", 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := countOrchestrateWords(tc.in)
			if got != tc.want {
				t.Fatalf("countOrchestrateWords(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeOrchestrateShards(t *testing.T) {
	got := normalizeOrchestrateShards([]string{
		" first task ",
		"",
		"First Task",
		"second task",
		"second task",
	}, 2)
	if gotLen := len(got); gotLen != 2 {
		t.Fatalf("normalizeOrchestrateShards len = %d, want %d", gotLen, 2)
	}
	want := []string{"first task", "second task"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeOrchestrateShards = %#v, want %#v", got, want)
	}
}

func TestSplitTaskByLines(t *testing.T) {
	got := splitTaskByLines("- build package\n- run tests\n- summarize results")
	want := []string{"build package", "run tests", "summarize results"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTaskByLines = %#v, want %#v", got, want)
	}
}

func TestSplitTaskByLines_WithIndentedContinuationLines(t *testing.T) {
	got := splitTaskByLines(`- build package
  run go test ./...
  capture failing output
- run tests
  include flaky tests only
  return summary`)
	want := []string{
		"build package run go test ./... capture failing output",
		"run tests include flaky tests only return summary",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTaskByLines = %#v, want %#v", got, want)
	}
}

func TestSplitTaskForOrchestration(t *testing.T) {
	got := splitTaskForOrchestration("Compile the report and then publish it", 2)
	want := []string{"Compile the report", "publish it"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTaskForOrchestration = %#v, want %#v", got, want)
	}

	got = splitTaskForOrchestration("Build and test", 2)
	want = []string{"Build and test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTaskForOrchestration = %#v, want %#v", got, want)
	}
}

func TestSplitTaskForOrchestration_AvoidsAnaphoricDependencySplit(t *testing.T) {
	got := splitTaskForOrchestration("Run compile and test; if it fails, fix the first failure", 2)
	want := []string{"Run compile and test; if it fails, fix the first failure"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTaskForOrchestration = %#v, want %#v", got, want)
	}
}

func TestSynthesizeResults_OrdersByIndex(t *testing.T) {
	input := []TaskResult{
		{ID: "2", Index: 1, Output: "two"},
		{ID: "1", Index: 0, Output: "one"},
	}
	out := SynthesizeResults(input)
	if out == "" {
		t.Fatal("expected non-empty synthesis output")
	}
	if len(out) == 0 {
		t.Fatal("unexpected empty output")
	}
	first := strings.Index(out, "[Shard 1]")
	second := strings.Index(out, "[Shard 2]")
	if first == -1 {
		t.Fatalf("expected output to include shard 1 marker, got %q", out)
	}
	if second == -1 {
		t.Fatalf("expected output to include shard 2 marker, got %q", out)
	}
	if second <= first {
		t.Fatalf("expected shard 1 before shard 2, got first=%d second=%d in %q", first, second, out)
	}
}

func TestSynthesizeResults_IncludesErrors(t *testing.T) {
	input := []TaskResult{
		{ID: "1", Index: 0, Err: fmt.Errorf("rate limit")},
		{ID: "2", Index: 1, Output: "ok"},
	}
	out := SynthesizeResults(input)
	if !strings.Contains(out, "[Shard 1 Error]") {
		t.Fatalf("expected error marker, got %q", out)
	}
}

func TestExecuteOrchestrate_UsesUpToTwoParallelShards(t *testing.T) {
	t.Setenv("EAI_TMUX_DISABLE", "1")
	var active int32
	var maxActive int32
	var reqCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&active, 1)
		if cur > atomic.LoadInt32(&maxActive) {
			atomic.StoreInt32(&maxActive, cur)
		}
		defer func() {
			atomic.AddInt32(&active, -1)
			atomic.AddInt32(&reqCount, 1)
		}()

		// Give the client enough time for another request to start so this test can
		// observe overlap when parallel execution is enabled.
		time.Sleep(120 * time.Millisecond)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "ok",
					},
				},
			},
		})
	}))
	defer server.Close()

	a := &Application{
		Config:              Config{MaxParallelAgents: 2},
		Logger:              NewLogger(&bytes.Buffer{}),
		Client:              &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
		Prompter:            NewPromptBuilder(),
		orchestrateCacheTTL: time.Minute,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := a.ExecuteOrchestrate(ctx, ModeOrchestrate, "build package and run tests", 2)
	if err != nil {
		t.Fatalf("ExecuteOrchestrate failed: %v", err)
	}
	if got := atomic.LoadInt32(&reqCount); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
	}
	if got := atomic.LoadInt32(&maxActive); got < 2 {
		t.Fatalf("expected parallel shards (maxActive=%d), got %d", got, got)
	}
	if out == "" || len(out) < 20 {
		t.Fatalf("expected non-empty synthesized output, got %q", out)
	}
	if !strings.Contains(out, "[Shard 1]") || !strings.Contains(out, "[Shard 2]") {
		t.Fatalf("expected synthesis markers in output, got %q", out)
	}
}

func TestExecuteOrchestrate_UsesExpandedTwoByFiveShardBudget(t *testing.T) {
	t.Setenv("EAI_TMUX_DISABLE", "1")
	var active int32
	var maxActive int32
	var reqCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&active, 1)
		if cur > atomic.LoadInt32(&maxActive) {
			atomic.StoreInt32(&maxActive, cur)
		}
		defer func() {
			atomic.AddInt32(&active, -1)
			atomic.AddInt32(&reqCount, 1)
		}()

		time.Sleep(40 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "ok",
					},
				},
			},
		})
	}))
	defer server.Close()

	t.Setenv("EAI_ORCHESTRATE_MAX_PANES_PER_TASK", "5")
	t.Setenv("EAI_ORCHESTRATE_MAX_SHARDS", "10")
	t.Setenv("EAI_ORCHESTRATE_ACTIVE_PANES", "5")

	a := &Application{
		Config:   Config{MaxParallelAgents: 10},
		Logger:   NewLogger(&bytes.Buffer{}),
		Client:   &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
		Prompter: NewPromptBuilder(),
	}

	task := `1) inspect repository
2) read failing tests
3) validate outputs
4) patch failing snippet
5) run quick checks
6) run narrow regression
7) capture artifacts
8) update docs
9) produce final summary
10) return done`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := a.ExecuteOrchestrate(ctx, ModeOrchestrate, task, 2)
	if err != nil {
		t.Fatalf("ExecuteOrchestrate failed: %v", err)
	}

	if got := atomic.LoadInt32(&reqCount); got != 10 {
		t.Fatalf("expected 10 requests for 10 subtasks, got %d", got)
	}
	if got := atomic.LoadInt32(&maxActive); got > 5 {
		t.Fatalf("expected at most 5 concurrent shard calls, got %d", got)
	}
}

func TestExecuteOrchestrate_RespectsConfiguredMaxParallel(t *testing.T) {
	t.Setenv("EAI_TMUX_DISABLE", "1")
	var active int32
	var maxActive int32
	var reqCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&active, 1)
		if cur > atomic.LoadInt32(&maxActive) {
			atomic.StoreInt32(&maxActive, cur)
		}
		defer func() {
			atomic.AddInt32(&active, -1)
			atomic.AddInt32(&reqCount, 1)
		}()

		time.Sleep(120 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "ok",
					},
				},
			},
		})
	}))

	defer server.Close()

	a := &Application{
		Config:   Config{MaxParallelAgents: 1},
		Logger:   NewLogger(&bytes.Buffer{}),
		Client:   &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
		Prompter: NewPromptBuilder(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := a.ExecuteOrchestrate(ctx, ModeOrchestrate, "build package and run tests", 2)
	if err != nil {
		t.Fatalf("ExecuteOrchestrate failed: %v", err)
	}
	if got := atomic.LoadInt32(&reqCount); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
	}
	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("expected maxActive=1 due config cap, got %d", got)
	}
}

func TestSplitTaskByLines_StrongerMarkers(t *testing.T) {
	got := splitTaskByLines(`
1) bootstrap service
Step 2: run tests
- [ ] validate build
1 - finalize checklist
• finalize deployment
   a) package artifacts
`)
	want := []string{"bootstrap service", "run tests", "validate build", "finalize checklist", "finalize deployment", "package artifacts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTaskByLines = %#v, want %#v", got, want)
	}
}

func TestSplitTaskForOrchestration_AvoidsFragileConnectors(t *testing.T) {
	got := splitTaskForOrchestration("Run verification then if green release the build", 2)
	want := []string{"Run verification then if green release the build"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTaskForOrchestration = %#v, want %#v", got, want)
	}

	got = splitTaskForOrchestration("If build passes then release the artifacts", 2)
	want = []string{"If build passes then release the artifacts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTaskForOrchestration = %#v, want %#v", got, want)
	}
}

func TestSplitTaskForOrchestration_SplitsSentences(t *testing.T) {
	got := splitTaskForOrchestration("Bootstrap service. Run tests. Validate build.", 5)
	want := []string{"Bootstrap service", "Run tests", "Validate build"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitTaskForOrchestration = %#v, want %#v", got, want)
	}
}

func TestCanRunOrchestrateShardsInTmux(t *testing.T) {
	tmp := t.TempDir()
	tm := filepath.Join(tmp, "tmux")
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(tm, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}

	// Ensure the behavior here is not affected by whether the developer happens to be
	// running tests inside a tmux session.
	t.Setenv("TMUX", "")
	t.Setenv("EAI_TMUX_WORKER", "")
	t.Setenv("EAI_TMUX_DISABLE", "")

	a := &Application{}
	if a.canRunOrchestrateShardsInTmux() {
		t.Fatalf("expected tmux orchestration disabled when TMUX is not set")
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmp+":"+oldPath)
	t.Setenv("TMUX", "%1")

	if !a.canRunOrchestrateShardsInTmux() {
		t.Fatalf("expected tmux orchestration enabled when TMUX is set and tmux exists")
	}

	t.Setenv("EAI_TMUX_WORKER", "1")
	if a.canRunOrchestrateShardsInTmux() {
		t.Fatalf("expected worker process to disable nested tmux orchestration")
	}

	t.Setenv("EAI_TMUX_WORKER", "")
	t.Setenv("EAI_TMUX_DISABLE", "1")
	if a.canRunOrchestrateShardsInTmux() {
		t.Fatalf("expected EAI_TMUX_DISABLE to disable tmux orchestration")
	}
}

func TestExecuteOrchestrate_UsesTmuxWorkersWhenAvailable(t *testing.T) {
	tmp := t.TempDir()
	tm := filepath.Join(tmp, "tmux")
	logPath := filepath.Join(tmp, "tmux.log")

	script := `#!/bin/sh
set -eu
log_file="${EAI_TMUX_LOG:-/dev/null}"

if [ "$1" = "split-window" ]; then
  prev=""
  shard=""
  result_file=""
  for arg in "$@"; do
    if [ "$prev" = "--shard-id" ]; then
      shard="$arg"
      prev=""
      continue
    fi
    if [ "$prev" = "--result-file" ]; then
      result_file="$arg"
      prev=""
      continue
    fi
    case "$arg" in
      --shard-id|--result-file)
        prev="$arg"
        ;;
    esac
  done
  printf '%s\n' "split:${shard}" >> "$log_file"
  if [ -n "$result_file" ]; then
    printf '%s' "{\"shard_id\":\"${shard}\",\"attempt\":1,\"output\":\"tmux result ${shard}\",\"duration_ms\":5}" > "$result_file"
  fi
  printf '%%0%s\n' "$shard"
  exit 0
fi

if [ "$1" = "kill-pane" ]; then
  printf '%s\n' "kill:${3:-}" >> "$log_file"
  exit 0
fi

echo "unsupported tmux command: $1" >&2
exit 1
`
	if err := os.WriteFile(tm, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux script: %v", err)
	}

	t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))
	t.Setenv("TMUX", "%1")
	t.Setenv("EAI_TMUX_LOG", logPath)
	t.Setenv("EAI_TMUX_WORKER", "")
	t.Setenv("EAI_TMUX_DISABLE", "")

	a := &Application{
		Logger:   NewLogger(&bytes.Buffer{}),
		Client:   &MinimaxClient{APIKey: "mock", Model: DefaultModel, BaseURL: "mock://", MaxTokens: 32768},
		Config:   Config{MaxParallelAgents: 2},
		Prompter: NewPromptBuilder(),
	}

	out, err := a.ExecuteOrchestrate(context.Background(), ModeOrchestrate, "1) scan repo\n2) patch failures", 2)
	if err != nil {
		t.Fatalf("ExecuteOrchestrate failed: %v", err)
	}
	if !strings.Contains(out, "tmux result 1") || !strings.Contains(out, "tmux result 2") {
		t.Fatalf("expected tmux worker outputs in final synthesis, got %q", out)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	raw := strings.TrimSpace(string(logData))
	if strings.Count(raw, "split:") != 2 {
		t.Fatalf("expected 2 split-window calls, got log: %q", raw)
	}
	if strings.Count(raw, "kill:") != 2 {
		t.Fatalf("expected 2 kill-pane calls, got log: %q", raw)
	}
}

func TestHasOrchestrateContinuationPrefix_Stronger(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		match bool
	}{
		{"if", "if the build is green, ship it", true},
		{"in case", "in case you need rollback, keep notes", true},
		{"after that", "after that run smoke tests", true},
		{"regular sentence", "run setup first", false},
		{"simple bullet", "then check", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasOrchestrateContinuationPrefix(tc.in); got != tc.match {
				t.Fatalf("hasOrchestrateContinuationPrefix(%q) = %v, want %v", tc.in, got, tc.match)
			}
		})
	}
}

func TestOrchestrateCacheKeyUsesSubtaskContext(t *testing.T) {
	a := &Application{
		Config: Config{Model: DefaultModel},
	}
	k1 := a.orchestrateCacheKey(ModeOrchestrate, "  Fix project   tests ", "Run test suite", "Subtask 1/2: Run test suite")
	k2 := a.orchestrateCacheKey(ModeOrchestrate, "Fix project tests", "Run test suite", "Subtask 1/2: Run test suite")
	k3 := a.orchestrateCacheKey(ModeOrchestrate, "Fix project tests", "Run integration suite", "Subtask 1/2: Run test suite")
	k4 := a.orchestrateCacheKey(ModeCreate, "Fix project tests", "Run test suite", "Subtask 1/2: Run test suite")

	if k1 != k2 {
		t.Fatalf("expected cache key normalization across whitespace, got %q != %q", k1, k2)
	}
	if k1 == k3 {
		t.Fatalf("expected different subtask to produce different cache key")
	}
	if k1 == k4 {
		t.Fatalf("expected different mode to produce different cache key")
	}
}

func TestExecuteOrchestrate_ProgressPhases(t *testing.T) {
	t.Setenv("EAI_TMUX_DISABLE", "1")
	var events []ProgressEvent
	var eventsMu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		prompt := parseOrchestrateRequestPrompt(t, body)

		time.Sleep(30 * time.Millisecond)
		if strings.Contains(prompt, "Subtask 1/2") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "subtask one done",
						},
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "subtask two done",
					},
				},
			},
		})
	}))
	defer server.Close()

	a := &Application{
		Config:              Config{MaxParallelAgents: 2},
		Logger:              NewLogger(&bytes.Buffer{}),
		Client:              &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
		Prompter:            NewPromptBuilder(),
		orchestrateCacheTTL: time.Minute,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out, err := a.ExecuteOrchestrateWithProgressEvents(ctx, ModeOrchestrate, "build package and run tests", 2, func(ev ProgressEvent) {
		eventsMu.Lock()
		events = append(events, ev)
		eventsMu.Unlock()
	})
	if err != nil {
		t.Fatalf("ExecuteOrchestrate failed: %v", err)
	}
	if out == "" || !strings.Contains(out, "[Shard 1]") || !strings.Contains(out, "[Shard 2]") {
		t.Fatalf("expected synthesized shard output, got %q", out)
	}

	eventsMu.Lock()
	snapshot := make([]ProgressEvent, len(events))
	copy(snapshot, events)
	eventsMu.Unlock()

	kinds := map[string]int{}
	for _, ev := range snapshot {
		kinds[ev.Kind]++
	}
	required := map[string]bool{
		"orchestrate_split":      false,
		"orchestrate_schedule":   false,
		"orchestrate_llm":        false,
		"orchestrate_shard_done": false,
		"orchestrate_synthesis":  false,
	}
	for _, ev := range snapshot {
		if _, ok := required[ev.Kind]; ok {
			required[ev.Kind] = true
		}
	}
	for kind, seen := range required {
		if !seen {
			t.Fatalf("missing phase event %q, kinds=%v", kind, kinds)
		}
	}

	for _, kind := range []string{"orchestrate_split", "orchestrate_schedule", "orchestrate_llm", "orchestrate_synthesis"} {
		if _, ok := kinds[kind]; !ok {
			t.Fatalf("missing phase kind map entry for %s", kind)
		}
	}

	splitIdx := -1
	scheduleIdx := -1
	llmIdx := -1
	shardDoneIdx := -1
	synthIdx := -1
	for i, ev := range snapshot {
		switch ev.Kind {
		case "orchestrate_split":
			if splitIdx == -1 {
				splitIdx = i
			}
		case "orchestrate_schedule":
			if scheduleIdx == -1 {
				scheduleIdx = i
			}
		case "orchestrate_llm":
			if llmIdx == -1 {
				llmIdx = i
			}
		case "orchestrate_shard_done":
			if shardDoneIdx == -1 {
				shardDoneIdx = i
			}
		case "orchestrate_synthesis":
			if synthIdx == -1 {
				synthIdx = i
			}
		}
	}
	if !(splitIdx >= 0 && scheduleIdx > splitIdx && llmIdx > scheduleIdx && shardDoneIdx >= 0 && synthIdx > shardDoneIdx) {
		t.Fatalf("unexpected phase ordering: split=%d schedule=%d llm=%d shard_done=%d synth=%d", splitIdx, scheduleIdx, llmIdx, shardDoneIdx, synthIdx)
	}
}

func TestExecuteOrchestrate_SyncEmitsAfterAllShardCompletions(t *testing.T) {
	t.Setenv("EAI_TMUX_DISABLE", "1")
	var events []ProgressEvent
	var eventsMu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "ok",
					},
				},
			},
		})
	}))
	defer server.Close()

	a := &Application{
		Config:              Config{MaxParallelAgents: 2},
		Logger:              NewLogger(&bytes.Buffer{}),
		Client:              &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
		Prompter:            NewPromptBuilder(),
		orchestrateCacheTTL: time.Minute,
	}

	input := `- scan repository
- inspect docs
- summarize actions`

	out, err := a.ExecuteOrchestrateWithProgressEvents(context.Background(), ModeOrchestrate, input, 3, func(ev ProgressEvent) {
		eventsMu.Lock()
		events = append(events, ev)
		eventsMu.Unlock()
	})
	if err != nil {
		t.Fatalf("ExecuteOrchestrate failed: %v", err)
	}
	if !strings.Contains(out, "[Shard 1]") || !strings.Contains(out, "[Shard 2]") || !strings.Contains(out, "[Shard 3]") {
		t.Fatalf("expected 3 shard outputs, got %q", out)
	}

	eventsMu.Lock()
	snapshot := make([]ProgressEvent, len(events))
	copy(snapshot, events)
	eventsMu.Unlock()

	syncIdx := -1
	shardDoneCount := 0
	for i, ev := range snapshot {
		switch ev.Kind {
		case "orchestrate_shard_done":
			shardDoneCount++
		case "orchestrate_sync":
			syncIdx = i
		}
	}
	if shardDoneCount != 3 {
		t.Fatalf("expected 3 shard_done events, got %d", shardDoneCount)
	}
	if syncIdx == -1 {
		t.Fatalf("expected orchestrate_sync event")
	}
	for i, ev := range snapshot {
		if ev.Kind == "orchestrate_shard_done" && i > syncIdx {
			t.Fatalf("shard_done appeared after sync: sync_idx=%d shard_done_idx=%d", syncIdx, i)
		}
	}
}

func TestExecuteOrchestrate_CachePerSubtask(t *testing.T) {
	t.Setenv("EAI_TMUX_DISABLE", "1")
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		prompt := parseOrchestrateRequestPrompt(t, body)
		content := "ok"
		if strings.Contains(prompt, "Subtask 1/2") {
			content = "shard one"
		} else if strings.Contains(prompt, "Subtask 2/2") {
			content = "shard two"
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": content,
					},
				},
			},
		})
	}))
	defer server.Close()

	a := &Application{
		Config:              Config{MaxParallelAgents: 2},
		Logger:              NewLogger(&bytes.Buffer{}),
		Client:              &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
		Prompter:            NewPromptBuilder(),
		orchestrateCacheTTL: time.Minute,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := a.ExecuteOrchestrate(ctx, ModeOrchestrate, "build package and run tests", 2)
	if err != nil {
		t.Fatalf("first ExecuteOrchestrate failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Fatalf("expected first run to use 2 requests, got %d", got)
	}

	var events []ProgressEvent
	_, err = a.ExecuteOrchestrateWithProgressEvents(ctx, ModeOrchestrate, "build package and run tests", 2, func(ev ProgressEvent) {
		if ev.Kind == "orchestrate_cache" {
			events = append(events, ev)
		}
	})
	if err != nil {
		t.Fatalf("second ExecuteOrchestrate failed: %v", err)
	}
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Fatalf("expected second run to hit cache (still 2 requests), got %d", got)
	}
	if len(events) == 0 {
		t.Fatalf("expected orchestrate_cache events, got none")
	}
}

func TestExecuteOrchestrate_PredictiveRetry_OnlyFailedShardRetries(t *testing.T) {
	t.Setenv("EAI_TMUX_DISABLE", "1")
	t.Setenv("EAI_LLM_MAX_RETRIES", "0")
	var mu sync.Mutex
	var calls []string
	var slowFinish time.Time
	var retryStarted time.Time
	var requestCount int32
	attempts := map[string]int{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		prompt := parseOrchestrateRequestPrompt(t, body)
		promptLower := strings.ToLower(prompt)

		mu.Lock()
		calls = append(calls, prompt)
		mu.Unlock()

		switch {
		case strings.Contains(promptLower, "run tests"):
			mu.Lock()
			attempts["run tests"]++
			current := attempts["run tests"]
			if current == 2 && strings.Contains(promptLower, "original request") {
				t.Fatalf("retry prompt should be constrained and omit full original request")
			}
			mu.Unlock()
			if current == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"message": "simulated failure",
					},
				})
				return
			}
			mu.Lock()
			retryStarted = time.Now()
			mu.Unlock()
		case strings.Contains(promptLower, "build package"):
			time.Sleep(120 * time.Millisecond)
			mu.Lock()
			slowFinish = time.Now()
			mu.Unlock()
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "shard done",
					},
				},
			},
		})
	}))
	defer server.Close()

	a := &Application{
		Config:   Config{MaxParallelAgents: 2},
		Logger:   NewLogger(&bytes.Buffer{}),
		Client:   &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
		Prompter: NewPromptBuilder(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var retryEventCount int
	out, err := a.ExecuteOrchestrateWithProgressEvents(ctx, ModeOrchestrate, "build package and run tests", 2, func(ev ProgressEvent) {
		if ev.Kind == "orchestrate_retry" {
			retryEventCount++
		}
	})
	if err != nil {
		t.Fatalf("ExecuteOrchestrate failed: %v", err)
	}
	if out == "" {
		t.Fatalf("expected synthesized output")
	}
	if retryEventCount != 1 {
		t.Fatalf("expected one retry event, got %d", retryEventCount)
	}
	if atomic.LoadInt32(&requestCount) != 3 {
		t.Fatalf("expected 3 requests (failed run, retry, slow sibling), got %d", requestCount)
	}

	mu.Lock()
	runAttempts := attempts["run tests"]
	retrySeen := !retryStarted.IsZero()
	slowDone := slowFinish
	mu.Unlock()

	if runAttempts != 2 {
		t.Fatalf("expected 2 attempts for run tests shard, got %d", runAttempts)
	}
	if !retrySeen {
		t.Fatalf("expected retry timestamp to be captured")
	}
	if slowDone.IsZero() {
		t.Fatalf("expected slow shard to complete")
	}

	mu.Lock()
	retryBeforeSlowComplete := retryStarted.Before(slowDone)
	mu.Unlock()
	if !retryBeforeSlowComplete {
		t.Fatalf("expected retry to be scheduled before slow shard completion")
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 total LLM calls, got %d", len(calls))
	}
}

func parseOrchestrateRequestPrompt(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid request body: %v", err)
	}
	if len(payload.Messages) == 0 {
		return ""
	}
	return payload.Messages[0].Content
}
