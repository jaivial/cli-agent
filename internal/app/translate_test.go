package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLikelyNeedsEnglishTranslation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"hello", false},
		{"please create a website in html", false},
		{"fix bug", false},
		{"haz una web en html", true},
		{"¿como estas?", true},
	}
	for _, tc := range cases {
		if got := likelyNeedsEnglishTranslation(tc.in); got != tc.want {
			t.Fatalf("likelyNeedsEnglishTranslation(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestTranslateToEnglish_SkipsWhenDisabled(t *testing.T) {
	t.Setenv("EAI_AUTO_TRANSLATE_TO_ENGLISH", "0")

	client := &MinimaxClient{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		Model:   "test-model",
	}
	out, translated, err := translateToEnglish(context.Background(), client, "haz una web en html")
	if err != nil {
		t.Fatalf("translateToEnglish returned error: %v", err)
	}
	if translated {
		t.Fatalf("translateToEnglish translated=true, want false")
	}
	if out != "haz una web en html" {
		t.Fatalf("translateToEnglish output %q, want %q", out, "haz una web en html")
	}
}

func TestTranslateToEnglish_SkipsWhenNoAPIKey(t *testing.T) {
	t.Parallel()

	client := &MinimaxClient{
		APIKey:  "",
		BaseURL: "http://example.invalid",
		Model:   "test-model",
	}
	out, translated, err := translateToEnglish(context.Background(), client, "haz una web en html")
	if err != nil {
		t.Fatalf("translateToEnglish returned error: %v", err)
	}
	if translated {
		t.Fatalf("translateToEnglish translated=true, want false")
	}
	if out != "haz una web en html" {
		t.Fatalf("translateToEnglish output %q, want %q", out, "haz una web en html")
	}
}

func TestTranslateToEnglish_WrappedInputOnlyTranslatesCurrentRequest(t *testing.T) {
	t.Parallel()

	wantTranslation := "create a website in html"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var reqBody zaiChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(reqBody.Messages) != 1 {
			t.Fatalf("unexpected messages length: %d", len(reqBody.Messages))
		}
		prompt := reqBody.Messages[0].Content
		if strings.Contains(prompt, "Session summary:") || strings.Contains(prompt, "Conversation context") {
			t.Fatalf("translation prompt unexpectedly contains wrapper context: %q", prompt)
		}

		resp := zaiChatCompletionResponse{
			Choices: []zaiChatCompletionChoice{{
				Index: 0,
				Message: &zaiChatMessage{
					Role:    "assistant",
					Content: wantTranslation,
				},
				FinishReason: "stop",
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(server.Close)

	client := &MinimaxClient{
		APIKey:    "test-key",
		BaseURL:   server.URL,
		Model:     "test-model",
		MaxTokens: 128,
		HTTP:      server.Client(),
	}

	wrapped := strings.Join([]string{
		"Current request:",
		"haz una web en html",
		"",
		"Session summary:",
		"prev work",
		"",
		"Conversation context (most recent messages):",
		"USER: hola",
		"",
	}, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, translated, err := translateToEnglish(ctx, client, wrapped)
	if err != nil {
		t.Fatalf("translateToEnglish returned error: %v", err)
	}
	if !translated {
		t.Fatalf("translateToEnglish translated=false, want true")
	}

	wantOut := strings.Join([]string{
		"Current request:",
		wantTranslation,
		"",
		"Session summary:",
		"prev work",
		"",
		"Conversation context (most recent messages):",
		"USER: hola",
		"",
	}, "\n")
	if out != wantOut {
		t.Fatalf("translateToEnglish output:\n%q\nwant:\n%q", out, wantOut)
	}
}
