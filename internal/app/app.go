package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Application struct {
	Config   Config
	Logger   *Logger
	Client   *MinimaxClient
	Runner   *Runner
	Jobs     *JobStore
	Prompter *PromptBuilder
}

func NewApplication(cfg Config, mockMode bool) (*Application, error) {
	logger := NewLogger(os.Stdout)
	
	var client *MinimaxClient
	if mockMode {
		// Create mock client for testing
		client = NewMinimaxClient("mock", "mock", "mock://", cfg.MaxTokens)
	} else {
		client = NewMinimaxClient(cfg.MinimaxAPIKey, cfg.Model, cfg.BaseURL, cfg.MaxTokens)
	}
	
	jobPath := filepath.Join(os.TempDir(), "cli-agent", "jobs.json")
	store, err := NewJobStore(jobPath)
	if err != nil {
		return nil, err
	}
	jobRoot := filepath.Join(os.TempDir(), "cli-agent", "logs")
	return &Application{
		Config:   cfg,
		Logger:   logger,
		Client:   client,
		Runner:   NewRunner(logger, jobRoot),
		Jobs:     store,
		Prompter: NewPromptBuilder(),
	}, nil
}

func (a *Application) ExecuteChat(ctx context.Context, mode Mode, input string) (string, error) {
	// Interactive TUI chat should produce human-readable output, not raw tool-call JSON.
	prompt := a.Prompter.BuildChat(mode, input)
	out, err := a.Client.Complete(ctx, prompt)
	if err != nil {
		if errors.Is(err, ErrAPIKeyRequired) {
			return "No API key configured. Run `/connect` in the TUI or set `MINIMAX_API_KEY` (and optionally `MINIMAX_MODEL`, `MINIMAX_BASE_URL`).", nil
		}
		return "", err
	}

	trim := strings.TrimSpace(out)
	// Safety: if the model (or mock) returns a tool-call JSON blob, don't show it in chat.
	if strings.HasPrefix(trim, "{") && strings.Contains(trim, "\"tool_calls\"") {
		return "This chat UI only shows text replies and does not execute tool calls. Use `eai agent \"...\"` for tool-driven tasks, or ask in plain language for a text-only answer.", nil
	}

	return out, nil
}

func (a *Application) ExecuteOrchestrate(ctx context.Context, mode Mode, input string, agents int) (string, error) {
	if agents <= 0 {
		return "", errors.New("agents must be > 0")
	}
	if agents > a.Config.MaxParallelAgents {
		agents = a.Config.MaxParallelAgents
	}

	shards := make([]TaskShard, 0, agents)
	for i := 0; i < agents; i++ {
		shards = append(shards, TaskShard{
			ID:     fmt.Sprintf("%d", i+1),
			Prompt: a.Prompter.Build(mode, fmt.Sprintf("Shard %d/%d: %s", i+1, agents, input)),
		})
	}
	orchestrator := NewOrchestrator(a.Client, a.Config.MaxParallelAgents)
	results, err := orchestrator.Run(ctx, shards)
	if err != nil {
		return "", err
	}
	return SynthesizeResults(results), nil
}

func (a *Application) RunCommand(ctx context.Context, command string, background bool) (Job, int, error) {
	if background {
		job, err := a.Runner.RunBackground(ctx, command, a.Jobs)
		return job, -1, err
	}
	code, err := a.Runner.Run(ctx, command)
	return Job{}, code, err
}

// ReloadClient updates the client with new configuration
func (a *Application) ReloadClient(cfg Config) {
	a.Config = cfg
	a.Client = NewMinimaxClient(cfg.MinimaxAPIKey, cfg.Model, cfg.BaseURL, cfg.MaxTokens)
}
