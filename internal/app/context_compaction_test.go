package app

import (
	"strings"
	"testing"
	"time"
)

func TestIsContextOverflowError(t *testing.T) {
	cases := []struct {
		errText string
		want    bool
	}{
		{"prompt is too long for this model", true},
		{"maximum context length exceeded", true},
		{"input tokens exceed context window", true},
		{"context deadline exceeded", false},
		{"temporary network failure", false},
	}
	for _, tc := range cases {
		got := isContextOverflowError(assertErr(tc.errText))
		if got != tc.want {
			t.Fatalf("isContextOverflowError(%q) = %v, want %v", tc.errText, got, tc.want)
		}
	}
}

func TestBuildSessionChatPromptIncludesSummaryAndLimitsHistory(t *testing.T) {
	history := []StoredMessage{
		{Role: "user", Content: "old-user-1"},
		{Role: "assistant", Content: "old-assistant-1"},
		{Role: "user", Content: "new-user-2"},
		{Role: "assistant", Content: "new-assistant-2"},
	}

	prompt := buildSessionChatPrompt("base-system", "carry-summary", history, "latest-input", 2)
	if !strings.Contains(prompt, "Session summary from previous turns") {
		t.Fatalf("expected prompt to include session summary header, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "carry-summary") {
		t.Fatalf("expected prompt to include session summary content, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "old-user-1") || strings.Contains(prompt, "old-assistant-1") {
		t.Fatalf("expected older history to be trimmed, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "new-user-2") || !strings.Contains(prompt, "new-assistant-2") {
		t.Fatalf("expected recent history in prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "[USER]\nlatest-input") {
		t.Fatalf("expected latest input at tail of prompt, got:\n%s", prompt)
	}
}

func TestBuildCompactionTranscriptCapsOutputSize(t *testing.T) {
	history := make([]StoredMessage, 0, 200)
	for i := 0; i < 200; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history = append(history, StoredMessage{
			Role:      role,
			Content:   strings.Repeat("x", 1200),
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	transcript := buildCompactionTranscript(history)
	if transcript == "" {
		t.Fatal("expected transcript to be non-empty")
	}
	if len(transcript) > sessionCompactionTranscriptChars {
		t.Fatalf("transcript too large: got %d > %d", len(transcript), sessionCompactionTranscriptChars)
	}
	if !strings.Contains(transcript, "[USER]") || !strings.Contains(transcript, "[ASSISTANT]") {
		t.Fatalf("expected transcript roles, got:\n%s", transcript)
	}
}

func assertErr(msg string) error {
	return &testErr{msg: msg}
}

type testErr struct {
	msg string
}

func (e *testErr) Error() string {
	return e.msg
}
