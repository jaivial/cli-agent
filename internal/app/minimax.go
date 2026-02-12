package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type MinimaxClient struct {
	APIKey    string
	Model     string
	BaseURL   string
	MaxTokens int
	HTTP      *http.Client
}

var ErrAPIKeyRequired = errors.New("api key is required")

type CompletionMeta struct {
	FinishReason string
}

// Z.ai is compatible with an OpenAI-style /chat/completions API.
// Docs: https://docs.z.ai/api-reference/llm/chat-completion.md
type zaiChatCompletionRequest struct {
	Model       string           `json:"model"`
	Messages    []zaiChatMessage `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	Stream      bool             `json:"stream"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	TopP        float64          `json:"top_p,omitempty"`
	Stop        json.RawMessage  `json:"stop,omitempty"`
}

type zaiChatMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type zaiChatDelta struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type zaiChatCompletionChoice struct {
	Index   int             `json:"index"`
	Message *zaiChatMessage `json:"message,omitempty"`
	Delta   *zaiChatDelta   `json:"delta,omitempty"`

	FinishReason string `json:"finish_reason,omitempty"`
}

type zaiChatCompletionResponse struct {
	Choices []zaiChatCompletionChoice `json:"choices"`
	Error   *struct {
		Message string      `json:"message"`
		Type    string      `json:"type,omitempty"`
		Code    interface{} `json:"code,omitempty"`
	} `json:"error,omitempty"`
}

func NewMinimaxClient(apiKey, model, baseURL string, maxTokens int) *MinimaxClient {
	mockMode := strings.EqualFold(strings.TrimSpace(apiKey), "mock") || strings.EqualFold(strings.TrimSpace(baseURL), "mock://")
	if mockMode {
		if model == "" {
			model = "mock"
		}
		if strings.TrimSpace(baseURL) == "" {
			baseURL = "mock://"
		}
	} else {
		model = NormalizeModel(model)
		baseURL = NormalizeBaseURL(baseURL)
	}
	if maxTokens <= 0 {
		maxTokens = 32768
	}

	// Create HTTP client with optional TLS skip for container environments.
	// Keep this reasonably high; per-request timeouts are enforced separately.
	httpTimeout := 300 * time.Second
	if v := strings.TrimSpace(os.Getenv("EAI_HTTP_TIMEOUT_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			httpTimeout = time.Duration(n) * time.Second
		}
	}
	httpClient := &http.Client{Timeout: httpTimeout}

	// Skip TLS verification if EAI_SKIP_TLS_VERIFY is set (for container environments)
	if os.Getenv("EAI_SKIP_TLS_VERIFY") == "1" || os.Getenv("EAI_SKIP_TLS_VERIFY") == "true" {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return &MinimaxClient{
		APIKey:    apiKey,
		Model:     model,
		BaseURL:   baseURL,
		MaxTokens: maxTokens,
		HTTP:      httpClient,
	}
}

func (c *MinimaxClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithObserver(ctx, prompt, nil)
}

func (c *MinimaxClient) CompleteWithObserver(ctx context.Context, prompt string, onReasoning func(string)) (string, error) {
	out, _, err := c.CompleteWithObserverMeta(ctx, prompt, onReasoning)
	return out, err
}

func (c *MinimaxClient) CompleteWithObserverMeta(ctx context.Context, prompt string, onReasoning func(string)) (string, CompletionMeta, error) {
	// Mock mode check
	if c.APIKey == "mock" || c.BaseURL == "mock://" {
		out, err := c.mockComplete(ctx, prompt)
		return out, CompletionMeta{FinishReason: "stop"}, err
	}

	if c.APIKey == "" {
		return "", CompletionMeta{}, ErrAPIKeyRequired
	}

	maxRetries := 2
	if v := strings.TrimSpace(os.Getenv("EAI_LLM_MAX_RETRIES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			maxRetries = n
		}
	}

	reqTimeout := 180 * time.Second
	if v := strings.TrimSpace(os.Getenv("EAI_LLM_REQUEST_TIMEOUT_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			reqTimeout = time.Duration(n) * time.Second
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return "", CompletionMeta{}, ctx.Err()
		}

		var out string
		var meta CompletionMeta
		var err error

		reqCtx := ctx
		var cancel context.CancelFunc
		if reqTimeout > 0 {
			reqCtx, cancel = context.WithTimeout(ctx, reqTimeout)
		}

		// Z.AI: /chat/completions (OpenAI-style)
		out, meta, err = c.completeZAiChatCompletionsMeta(reqCtx, prompt, onReasoning)
		if cancel != nil {
			cancel()
		}

		if err == nil {
			return out, meta, nil
		}
		lastErr = err
		if attempt == maxRetries || !isRetryableLLMError(err) {
			break
		}
		backoff := time.Duration(1<<attempt) * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
		time.Sleep(backoff)
	}
	return "", CompletionMeta{}, lastErr
}

func isRetryableLLMError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "insufficient balance") || strings.Contains(s, "no resource package") {
		return false
	}
	retryable := []string{
		"context deadline exceeded",
		"client.timeout exceeded",
		"timeout",
		"connection reset",
		"connection refused",
		"unexpected eof",
		"operation failed",
		"status 429",
		"status 500",
		"status 502",
		"status 503",
		"status 504",
	}
	for _, needle := range retryable {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func zaiChatCompletionsURL(baseURL string) string {
	base := strings.TrimSpace(baseURL)
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

func (c *MinimaxClient) completeZAiChatCompletions(ctx context.Context, prompt string, onReasoning func(string)) (string, error) {
	out, _, err := c.completeZAiChatCompletionsMeta(ctx, prompt, onReasoning)
	return out, err
}

func (c *MinimaxClient) completeZAiChatCompletionsMeta(ctx context.Context, prompt string, onReasoning func(string)) (string, CompletionMeta, error) {
	url := zaiChatCompletionsURL(c.BaseURL)
	reqBody := zaiChatCompletionRequest{
		Model:       c.Model,
		Messages:    []zaiChatMessage{{Role: "user", Content: prompt}},
		Temperature: 0.2,
		Stream:      false,
		MaxTokens:   c.MaxTokens,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", CompletionMeta{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", CompletionMeta{}, err
	}
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "en-US,en")

	resp, err := c.HTTP.Do(request)
	if err != nil {
		return "", CompletionMeta{}, fmt.Errorf("api request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", CompletionMeta{}, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode >= 300 {
		// Try OpenAI-style error first.
		var openaiErr zaiChatCompletionResponse
		if err := json.Unmarshal(bodyBytes, &openaiErr); err == nil && openaiErr.Error != nil && openaiErr.Error.Message != "" {
			return "", CompletionMeta{}, fmt.Errorf("z.ai api error: status %d, message: %s", resp.StatusCode, openaiErr.Error.Message)
		}

		// Try {code,message,data} wrapper style.
		var wrapped struct {
			Code    int    `json:"code,omitempty"`
			Message string `json:"message,omitempty"`
			Error   *struct {
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal(bodyBytes, &wrapped); err == nil {
			if wrapped.Error != nil && wrapped.Error.Message != "" {
				return "", CompletionMeta{}, fmt.Errorf("z.ai api error: status %d, message: %s", resp.StatusCode, wrapped.Error.Message)
			}
			if wrapped.Message != "" {
				return "", CompletionMeta{}, fmt.Errorf("z.ai api error: status %d, message: %s", resp.StatusCode, wrapped.Message)
			}
		}

		if msg := extractErrorMessageFromBody(bodyBytes); msg != "" {
			return "", CompletionMeta{}, fmt.Errorf("z.ai api error: status %d, message: %s", resp.StatusCode, msg)
		}
		return "", CompletionMeta{}, fmt.Errorf("z.ai api error: status %d", resp.StatusCode)
	}

	// First try direct OpenAI-style response.
	var respDirect zaiChatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &respDirect); err == nil {
		if respDirect.Error != nil {
			return "", CompletionMeta{}, fmt.Errorf("z.ai api error: %s", respDirect.Error.Message)
		}
		content, reasoning, finishReason := extractZAiChoiceContentMeta(respDirect.Choices)
		if onReasoning != nil && strings.TrimSpace(reasoning) != "" {
			onReasoning(reasoning)
		}
		if content != "" {
			return content, CompletionMeta{FinishReason: finishReason}, nil
		}
	}

	// Then try {code,message,data:{choices:...}} wrapper.
	var respWrapped struct {
		Code    int                       `json:"code,omitempty"`
		Message string                    `json:"message,omitempty"`
		Data    zaiChatCompletionResponse `json:"data"`
		Error   *struct{ Message string } `json:"error,omitempty"`
	}
	if err := json.Unmarshal(bodyBytes, &respWrapped); err == nil {
		if respWrapped.Error != nil && respWrapped.Error.Message != "" {
			return "", CompletionMeta{}, fmt.Errorf("z.ai api error: %s", respWrapped.Error.Message)
		}
		if respWrapped.Data.Error != nil && respWrapped.Data.Error.Message != "" {
			return "", CompletionMeta{}, fmt.Errorf("z.ai api error: %s", respWrapped.Data.Error.Message)
		}
		content, reasoning, finishReason := extractZAiChoiceContentMeta(respWrapped.Data.Choices)
		if onReasoning != nil && strings.TrimSpace(reasoning) != "" {
			onReasoning(reasoning)
		}
		if content != "" {
			return content, CompletionMeta{FinishReason: finishReason}, nil
		}
		if respWrapped.Message != "" && respWrapped.Code != 0 {
			return "", CompletionMeta{}, fmt.Errorf("z.ai api error: code %d, message: %s", respWrapped.Code, respWrapped.Message)
		}
	}

	// Avoid dumping raw JSON into the TUI error line.
	if summary := summarizeResponseBody(bodyBytes, 240); summary != "" && !strings.HasPrefix(summary, "{") && !strings.HasPrefix(summary, "[") {
		return "", CompletionMeta{}, fmt.Errorf("invalid z.ai api response format: %s", summary)
	}
	return "", CompletionMeta{}, fmt.Errorf("invalid z.ai api response format")
}

func extractZAiChoiceContent(choices []zaiChatCompletionChoice) (content string, reasoning string) {
	for _, ch := range choices {
		if ch.Message != nil {
			if reasoning == "" && strings.TrimSpace(ch.Message.ReasoningContent) != "" {
				reasoning = ch.Message.ReasoningContent
			}
			if strings.TrimSpace(ch.Message.Content) != "" {
				return ch.Message.Content, reasoning
			}
		}
		if ch.Delta != nil {
			if reasoning == "" && strings.TrimSpace(ch.Delta.ReasoningContent) != "" {
				reasoning = ch.Delta.ReasoningContent
			}
			if strings.TrimSpace(ch.Delta.Content) != "" {
				return ch.Delta.Content, reasoning
			}
		}
	}
	return "", reasoning
}

func extractZAiChoiceContentMeta(choices []zaiChatCompletionChoice) (content string, reasoning string, finishReason string) {
	firstFinish := ""
	for _, ch := range choices {
		fr := strings.TrimSpace(ch.FinishReason)
		if firstFinish == "" && fr != "" {
			firstFinish = fr
		}
		if ch.Message != nil {
			if reasoning == "" && strings.TrimSpace(ch.Message.ReasoningContent) != "" {
				reasoning = ch.Message.ReasoningContent
			}
			if strings.TrimSpace(ch.Message.Content) != "" {
				if fr == "" {
					fr = firstFinish
				}
				return ch.Message.Content, reasoning, fr
			}
		}
		if ch.Delta != nil {
			if reasoning == "" && strings.TrimSpace(ch.Delta.ReasoningContent) != "" {
				reasoning = ch.Delta.ReasoningContent
			}
			if strings.TrimSpace(ch.Delta.Content) != "" {
				if fr == "" {
					fr = firstFinish
				}
				return ch.Delta.Content, reasoning, fr
			}
		}
	}
	return "", reasoning, firstFinish
}

func extractErrorMessageFromBody(body []byte) string {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		// non-JSON errors can be returned as plain text.
		summary := summarizeResponseBody(body, 200)
		if summary != "" && !strings.HasPrefix(summary, "{") && !strings.HasPrefix(summary, "[") {
			return summary
		}
		return ""
	}

	// Common fields.
	keys := []string{"message", "msg", "detail", "error_description", "errorMessage", "error_message"}
	for _, k := range keys {
		if s, ok := obj[k].(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				return s
			}
		}
	}

	// Nested error.
	if ev, ok := obj["error"]; ok {
		switch t := ev.(type) {
		case map[string]interface{}:
			for _, k := range keys {
				if s, ok := t[k].(string); ok {
					s = strings.TrimSpace(s)
					if s != "" {
						return s
					}
				}
			}
		case string:
			s := strings.TrimSpace(t)
			if s != "" {
				return s
			}
		}
	}

	return ""
}

func summarizeResponseBody(body []byte, maxLen int) string {
	s := strings.TrimSpace(string(body))
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if maxLen <= 0 {
		maxLen = 240
	}
	if len(s) > maxLen {
		if maxLen <= 3 {
			return s[:maxLen]
		}
		s = s[:maxLen-3] + "..."
	}
	return s
}

// mockComplete simulates API responses for testing
func (c *MinimaxClient) mockComplete(ctx context.Context, prompt string) (string, error) {
	// Extract the task from the prompt
	task := extractTaskFromPrompt(prompt)

	// Check if this is a follow-up with tool results
	// If prompt contains "Tool result" or "SUCCESS", return completion
	if strings.Contains(prompt, "Tool result") || strings.Contains(prompt, "SUCCESS") {
		// Task completed - return a completion message with no tool calls
		return "I've completed the task successfully. The tool execution was successful and the results have been provided. No further action is needed.", nil
	}

	// First call - generate appropriate tool call
	return generateMockResponse(task)
}

// extractActionableContent tries to find tool calls or actionable content from thinking text
func extractActionableContent(thinking string) string {
	// If thinking contains JSON-like tool call patterns, try to extract them
	// This handles cases where the model thinks through tool calls but hits max_tokens

	// Look for patterns like: read_file, write_file, exec, list_dir, grep
	// and try to construct a reasonable tool call

	thinkingLower := strings.ToLower(thinking)

	// Check for file reading intent
	if strings.Contains(thinkingLower, "read") && strings.Contains(thinkingLower, "file") {
		// Try to extract a file path from the thinking
		if idx := strings.Index(thinking, "/app/"); idx != -1 {
			end := idx
			for end < len(thinking) && thinking[end] != ' ' && thinking[end] != '\n' && thinking[end] != '"' && thinking[end] != '\'' {
				end++
			}
			path := thinking[idx:end]
			return fmt.Sprintf(`{"tool_calls":[{"id":"read_1","name":"read_file","arguments":{"path":"%s"}}]}`, path)
		}
		return `{"tool_calls":[{"id":"read_1","name":"read_file","arguments":{"path":"."}}]}`
	}

	// Check for directory listing intent
	if strings.Contains(thinkingLower, "list") && (strings.Contains(thinkingLower, "dir") || strings.Contains(thinkingLower, "folder") || strings.Contains(thinkingLower, "file")) {
		return `{"tool_calls":[{"id":"list_1","name":"list_dir","arguments":{"path":"."}}]}`
	}

	// Check for command execution intent
	if strings.Contains(thinkingLower, "run") || strings.Contains(thinkingLower, "execute") || strings.Contains(thinkingLower, "command") {
		return `{"tool_calls":[{"id":"exec_1","name":"exec","arguments":{"command":"ls -la"}}]}`
	}

	// Default: return the thinking as a message (agent will continue)
	// Truncate if too long
	if len(thinking) > 500 {
		thinking = thinking[:500] + "..."
	}
	return fmt.Sprintf("Based on analysis: %s", thinking)
}

func extractTaskFromPrompt(prompt string) string {
	lines := strings.Split(prompt, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		// Check for [user] tag (both cases)
		if strings.HasPrefix(line, "[user]") || strings.HasPrefix(line, "[USER]") {
			// Task is on the next line(s) until we hit [assistant] or end
			for j := i + 1; j < len(lines); j++ {
				nextLine := strings.TrimSpace(lines[j])
				if strings.HasPrefix(nextLine, "[") {
					break // Stop at next tag
				}
				if nextLine != "" {
					return nextLine
				}
			}
		}
	}
	return ""
}

func generateMockResponse(task string) (string, error) {
	taskLower := strings.ToLower(task)

	// More flexible pattern matching
	switch {
	case strings.Contains(taskLower, "website") && strings.Contains(taskLower, "html"):
		// For TUI chat-mode tests: return a plain HTML document (not tool calls).
		return "<!doctype html>\n<html>\n<head><meta charset=\"utf-8\"><title>Pet Store</title></head>\n<body><h1>Pet Store</h1></body>\n</html>\n", nil

	case strings.Contains(taskLower, "list") && strings.Contains(taskLower, "file"):
		return `{"tool_calls":[{"id":"list_dir_1","name":"list_dir","arguments":{"path":"."}}]}`, nil

	case strings.Contains(taskLower, "read") && (strings.Contains(taskLower, "file") || strings.Contains(taskLower, "content")):
		return `{"tool_calls":[{"id":"read_file_1","name":"read_file","arguments":{"path":"."}}]}`, nil

	case strings.Contains(taskLower, "create") && strings.Contains(taskLower, "file"):
		return `{"tool_calls":[{"id":"write_file_1","name":"write_file","arguments":{"path":"test.txt","content":"Hello World"}}]}`, nil

	case strings.Contains(taskLower, "find") || strings.Contains(taskLower, "search"):
		return `{"tool_calls":[{"id":"grep_search_1","name":"grep","arguments":{"pattern":"test","path":".","recursive":true}}]}`, nil

	case strings.Contains(taskLower, "go version"):
		return `{"tool_calls":[{"id":"exec_go_1","name":"exec","arguments":{"command":"go version"}}]}`, nil

	case strings.Contains(taskLower, "date") || strings.Contains(taskLower, "time"):
		return `{"tool_calls":[{"id":"exec_date_1","name":"exec","arguments":{"command":"date"}}]}`, nil

	case strings.Contains(taskLower, "cpu") || strings.Contains(taskLower, "core") || strings.Contains(taskLower, "ncpu"):
		return `{"tool_calls":[{"id":"exec_cpu_1","name":"exec","arguments":{"command":"sysctl -n hw.ncpu"}}]}`, nil

	case strings.Contains(taskLower, "count"):
		return `{"tool_calls":[{"id":"exec_wc_1","name":"exec","arguments":{"command":"find . -type f | wc -l"}}]}`, nil

	case strings.Contains(taskLower, "directory") || strings.Contains(taskLower, "folder") || strings.Contains(taskLower, "cmd"):
		return `{"tool_calls":[{"id":"list_dir_1","name":"list_dir","arguments":{"path":"."}}]}`, nil

	case strings.Contains(taskLower, "exist") || strings.Contains(taskLower, "check"):
		return `{"tool_calls":[{"id":"exec_test_1","name":"exec","arguments":{"command":"ls -la"}}]}`, nil

	default:
		// Try to extract any specific command mentioned
		if strings.Contains(taskLower, "go ") {
			return `{"tool_calls":[{"id":"exec_go_1","name":"exec","arguments":{"command":"go version"}}]}`, nil
		}
		return `{"tool_calls":[{"id":"exec_echo_1","name":"exec","arguments":{"command":"echo 'Done'"}}]}`, nil
	}
}
