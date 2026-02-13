package app

import "testing"

func TestNormalizeBaseURL_AllowsMiniMaxEndpoints(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "international",
			in:   "https://api.minimax.io/v1/",
			want: "https://api.minimax.io/v1",
		},
		{
			name: "china",
			in:   "https://api.minimaxi.com/v1/",
			want: "https://api.minimaxi.com/v1",
		},
		{
			name: "legacy z.ai",
			in:   "https://api.z.ai/api/paas/v4/",
			want: "https://api.z.ai/api/paas/v4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeBaseURL(tc.in)
			if got != tc.want {
				t.Fatalf("NormalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeModel_AllowsMiniMaxCodingPlan(t *testing.T) {
	got := NormalizeModel("codex-MiniMax-M2.5")
	if got != ModelMiniMaxM25CodingPlan {
		t.Fatalf("NormalizeModel(minimax coding plan) = %q, want %q", got, ModelMiniMaxM25CodingPlan)
	}
}

func TestDefaultConfig_ModeIsOrchestrate(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DefaultMode != "orchestrate" {
		t.Fatalf("DefaultConfig().DefaultMode = %q, want %q", cfg.DefaultMode, "orchestrate")
	}
}
