package tui

import (
	"testing"

	"cli-agent/internal/app"
)

func TestSetupWizardApplyConfigSelections_MiniMaxInternational(t *testing.T) {
	cfg := app.DefaultConfig()
	w := NewSetupWizard(&cfg)
	w.providerIndex = 0 // MiniMax
	w.regionIndex = 0   // International
	w.apiKey = "minimax-key"

	w.applyConfigSelections()

	if cfg.APIKey != "minimax-key" {
		t.Fatalf("API key = %q, want %q", cfg.APIKey, "minimax-key")
	}
	if cfg.Model != app.ModelMiniMaxM25CodingPlan {
		t.Fatalf("model = %q, want %q", cfg.Model, app.ModelMiniMaxM25CodingPlan)
	}
	if cfg.BaseURL != app.MiniMaxBaseURLInternational {
		t.Fatalf("base URL = %q, want %q", cfg.BaseURL, app.MiniMaxBaseURLInternational)
	}
}

func TestSetupWizardApplyConfigSelections_MiniMaxChina(t *testing.T) {
	cfg := app.DefaultConfig()
	w := NewSetupWizard(&cfg)
	w.providerIndex = 0 // MiniMax
	w.regionIndex = 1   // China
	w.apiKey = "minimax-key-cn"

	w.applyConfigSelections()

	if cfg.BaseURL != app.MiniMaxBaseURLChina {
		t.Fatalf("base URL = %q, want %q", cfg.BaseURL, app.MiniMaxBaseURLChina)
	}
	if cfg.Model != app.ModelMiniMaxM25CodingPlan {
		t.Fatalf("model = %q, want %q", cfg.Model, app.ModelMiniMaxM25CodingPlan)
	}
}
