package main

import (
	"testing"

	"cli-agent/internal/app"
)

func TestApplyEnvOverrides_UsesMiniMaxAPIKeyFallback(t *testing.T) {
	t.Setenv("EAI_API_KEY", "")
	t.Setenv("MINIMAX_API_KEY", "minimax-env-key")

	cfg := app.DefaultConfig()
	cfg.APIKey = ""

	applyEnvOverrides(&cfg)

	if cfg.APIKey != "minimax-env-key" {
		t.Fatalf("API key = %q, want %q", cfg.APIKey, "minimax-env-key")
	}
}

func TestApplyEnvOverrides_EAIAPIKeyTakesPrecedence(t *testing.T) {
	t.Setenv("EAI_API_KEY", "eai-key")
	t.Setenv("MINIMAX_API_KEY", "minimax-env-key")

	cfg := app.DefaultConfig()
	cfg.APIKey = ""

	applyEnvOverrides(&cfg)

	if cfg.APIKey != "eai-key" {
		t.Fatalf("API key = %q, want %q", cfg.APIKey, "eai-key")
	}
}
