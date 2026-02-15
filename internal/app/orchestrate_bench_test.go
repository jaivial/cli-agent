package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var tbenchStyleOrchestrateTasks = []string{
	`1) Inspect repository for failing tests and cleanup
2) Run quick regression checks
3) Summarize fixes with follow-up command list`,
	`- review the benchmark runner
- add deterministic seeds for flaky jobs`,
	`- generate an actionable summary for a terminal-bench failure report
- patch broken setup scripts`,
}

var tbenchRealisticOrchestrateTasks = []string{
	`1) Scan go files and identify packages with build tags.
2) Reproduce with go test and capture the first failure.
3) Propose a minimal fix plan and report exact command sequence.`,
	`- inspect benchmark dataset generation path
Step 2: Re-run the failing slice and collect output
Step 3: summarize root cause`,
	`1 - Open task directory and count candidate files
2 - Read only README and failing logs
3 - Return: affected files, command, expected follow-up`,
}

type orchestrateDelay struct {
	marker string
	delay  time.Duration
}

func orchestratePromptFromBody(body []byte) string {
	var payload struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if len(payload.Messages) == 0 {
		return ""
	}
	return payload.Messages[0].Content
}

func orchestrateLatencyServer(delays []orchestrateDelay, fallbackDelay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		prompt := orchestratePromptFromBody(body)
		time.Sleep(fallbackDelay)
		for _, d := range delays {
			if strings.Contains(prompt, d.marker) {
				time.Sleep(d.delay)
				break
			}
		}
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
}

func collectOrchestratePhaseMetrics(
	ctx context.Context,
	app *Application,
	task string,
) (split, schedule, llm, synthesis time.Duration, cacheHits int64, err error) {
	_, err = app.ExecuteOrchestrateWithProgressEvents(ctx, ModeOrchestrate, task, 2, func(ev ProgressEvent) {
		switch ev.Kind {
		case "orchestrate_split":
			split += time.Duration(ev.DurationMs) * time.Millisecond
		case "orchestrate_schedule":
			schedule += time.Duration(ev.DurationMs) * time.Millisecond
		case "orchestrate_llm":
			llm += time.Duration(ev.DurationMs) * time.Millisecond
		case "orchestrate_synthesis":
			synthesis += time.Duration(ev.DurationMs) * time.Millisecond
		case "orchestrate_cache":
			cacheHits++
		}
	})
	return
}

func cloneHTTPClientWithCountingTransport(base *http.Client, requestCount *int64) *http.Client {
	if base == nil {
		base = &http.Client{}
	}
	cloned := *base
	transport := cloned.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	cloned.Transport = &roundTripCountingTransport{
		base:  transport,
		count: requestCount,
	}
	return &cloned
}

func BenchmarkExecuteOrchestrate_PhaseTiming_TBenchStyle(b *testing.B) {
	server := orchestrateLatencyServer([]orchestrateDelay{
		{marker: "Subtask 1/2", delay: 40 * time.Millisecond},
		{marker: "Subtask 2/2", delay: 35 * time.Millisecond},
	}, 10*time.Millisecond)
	defer server.Close()

	a := &Application{
		Logger:              NewLogger(&testingWriter{}),
		Config:              Config{MaxParallelAgents: 2},
		Client:              &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
		Prompter:            NewPromptBuilder(),
		orchestrateCacheTTL: 0,
	}

	var splitTotal time.Duration
	var scheduleTotal time.Duration
	var llmTotal time.Duration
	var synthTotal time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := tbenchStyleOrchestrateTasks[i%len(tbenchStyleOrchestrateTasks)]
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		split, schedule, llm, synth, _, err := collectOrchestratePhaseMetrics(ctx, a, task)
		cancel()

		if err != nil {
			b.Fatalf("ExecuteOrchestrate failed: %v", err)
		}

		splitTotal += split
		scheduleTotal += schedule
		llmTotal += llm
		synthTotal += synth
	}

	if b.N > 0 {
		avg := float64(b.N)
		b.ReportMetric(float64(splitTotal.Nanoseconds())/avg, "ns/split")
		b.ReportMetric(float64(scheduleTotal.Nanoseconds())/avg, "ns/schedule")
		b.ReportMetric(float64(llmTotal.Nanoseconds())/avg, "ns/llm")
		b.ReportMetric(float64(synthTotal.Nanoseconds())/avg, "ns/synthesis")
	}
}

func BenchmarkExecuteOrchestrate_PhaseTiming_TBenchRealistic(b *testing.B) {
	server := orchestrateLatencyServer([]orchestrateDelay{
		{marker: "Subtask 1/3", delay: 45 * time.Millisecond},
		{marker: "Subtask 2/3", delay: 35 * time.Millisecond},
		{marker: "Subtask 3/3", delay: 30 * time.Millisecond},
		{marker: "Subtask 1/2", delay: 40 * time.Millisecond},
		{marker: "Subtask 2/2", delay: 35 * time.Millisecond},
	}, 10*time.Millisecond)
	defer server.Close()

	a := &Application{
		Logger:              NewLogger(&testingWriter{}),
		Config:              Config{MaxParallelAgents: 2},
		Client:              &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
		Prompter:            NewPromptBuilder(),
		orchestrateCacheTTL: time.Minute,
	}

	var splitTotal time.Duration
	var scheduleTotal time.Duration
	var llmTotal time.Duration
	var synthTotal time.Duration
	var cacheHitTotal int64

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := tbenchRealisticOrchestrateTasks[i%len(tbenchRealisticOrchestrateTasks)]
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		split, schedule, llm, synth, cacheHits, err := collectOrchestratePhaseMetrics(ctx, a, task)
		cancel()

		if err != nil {
			b.Fatalf("ExecuteOrchestrate failed: %v", err)
		}

		splitTotal += split
		scheduleTotal += schedule
		llmTotal += llm
		synthTotal += synth
		cacheHitTotal += cacheHits
	}

	if b.N > 0 {
		avg := float64(b.N)
		b.ReportMetric(float64(splitTotal.Nanoseconds())/avg, "ns/split")
		b.ReportMetric(float64(scheduleTotal.Nanoseconds())/avg, "ns/schedule")
		b.ReportMetric(float64(llmTotal.Nanoseconds())/avg, "ns/llm")
		b.ReportMetric(float64(synthTotal.Nanoseconds())/avg, "ns/synthesis")
		b.ReportMetric(float64(cacheHitTotal)/avg, "cache_hits_per_run")
	}
}

func BenchmarkExecuteOrchestrate_TBenchCacheWarmupProfile(b *testing.B) {
	server := orchestrateLatencyServer([]orchestrateDelay{}, 12*time.Millisecond)
	defer server.Close()

	var requestCount int64
	countedClient := cloneHTTPClientWithCountingTransport(server.Client(), &requestCount)

	a := &Application{
		Logger:              NewLogger(&testingWriter{}),
		Config:              Config{MaxParallelAgents: 2},
		Client:              &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: countedClient, MaxTokens: 32768},
		Prompter:            NewPromptBuilder(),
		orchestrateCacheTTL: 10 * time.Minute,
	}

	seedTask := tbenchStyleOrchestrateTasks[0]
	if _, err := a.ExecuteOrchestrate(context.Background(), ModeOrchestrate, seedTask, 2); err != nil {
		b.Fatalf("cache warmup run failed: %v", err)
	}
	atomic.StoreInt64(&requestCount, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.ExecuteOrchestrate(context.Background(), ModeOrchestrate, seedTask, 2); err != nil {
			b.Fatalf("cache run failed: %v", err)
		}
	}

	if got := atomic.LoadInt64(&requestCount); got != 0 {
		b.Fatalf("expected warm cache to absorb repeat subtasks, saw %d requests", got)
	}
}

func BenchmarkExecuteOrchestrate_ActivePanesScaling(b *testing.B) {
	b.Setenv("EAI_TMUX_DISABLE", "1")
	b.Setenv("EAI_ORCHESTRATE_MAX_PANES_PER_TASK", "5")
	b.Setenv("EAI_ORCHESTRATE_MAX_SHARDS", "10")

	server := orchestrateLatencyServer(nil, 15*time.Millisecond)
	defer server.Close()

	task := `1) subtask one
2) subtask two
3) subtask three
4) subtask four
5) subtask five
6) subtask six
7) subtask seven
8) subtask eight
9) subtask nine
10) subtask ten`

	for _, panes := range []int{1, 2, 5, 10} {
		b.Run(fmt.Sprintf("panes=%d", panes), func(b *testing.B) {
			b.Setenv("EAI_ORCHESTRATE_ACTIVE_PANES", strconv.Itoa(panes))
			a := &Application{
				Logger:              NewLogger(&testingWriter{}),
				Config:              Config{MaxParallelAgents: panes},
				Client:              &MinimaxClient{APIKey: "k", Model: DefaultModel, BaseURL: server.URL, HTTP: server.Client(), MaxTokens: 32768},
				Prompter:            NewPromptBuilder(),
				orchestrateCacheTTL: 0,
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				_, err := a.ExecuteOrchestrate(ctx, ModeOrchestrate, task, 2)
				cancel()
				if err != nil {
					b.Fatalf("ExecuteOrchestrate failed: %v", err)
				}
			}
		})
	}
}

type roundTripCountingTransport struct {
	base  http.RoundTripper
	count *int64
}

func (t *roundTripCountingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt64(t.count, 1)
	return t.base.RoundTrip(req)
}

// testingWriter keeps benchmark logs compact while preserving logger API compatibility.
type testingWriter struct{}

func (testingWriter) Write(p []byte) (int, error) { return len(p), nil }
