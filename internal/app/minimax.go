package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type MinimaxClient struct {
	APIKey string
	Model  string
	BaseURL string
	MaxTokens int
	HTTP   *http.Client
}

type minimaxRequest struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	Messages  []minimaxMessage `json:"messages"`
}

type minimaxMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type minimaxResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error,omitempty"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func NewMinimaxClient(apiKey, model, baseURL string, maxTokens int) *MinimaxClient {
	if model == "" {
		model = "minimax-m2.1"
	}
	if baseURL == "" {
		baseURL = "https://api.minimax.io/anthropic/v1/messages"
	}
	if maxTokens <= 0 {
		maxTokens = 2048
	}
	return &MinimaxClient{
		APIKey: apiKey,
		Model:  model,
		BaseURL: baseURL,
		MaxTokens: maxTokens,
		HTTP:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *MinimaxClient) Complete(ctx context.Context, prompt string) (string, error) {
	// Mock mode check
	if c.APIKey == "mock" || c.BaseURL == "mock://" {
		return c.mockComplete(ctx, prompt)
	}
	
	if c.APIKey == "" {
		return "", errors.New("minimax api key is required")
	}
	reqBody := minimaxRequest{
		Model:     c.Model,
		MaxTokens: c.MaxTokens,
		Messages: []minimaxMessage{{Role: "user", Content: prompt}},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-api-key", c.APIKey)
	request.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTP.Do(request)
	if err != nil {
		return "", fmt.Errorf("api request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode >= 300 {
		// Try to parse error response
		var errResp struct {
			Error string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(bodyBytes, &errResp)
		if errResp.Message != "" {
			return "", fmt.Errorf("minimax api error: status %d, message: %s", resp.StatusCode, errResp.Message)
		}
		if errResp.Error != "" {
			return "", fmt.Errorf("minimax api error: status %d, error: %s", resp.StatusCode, errResp.Error)
		}
		return "", fmt.Errorf("minimax api error: status %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Try to parse Anthropic format response first (since endpoint is /anthropic/v1/messages)
	var anthropicResp anthropicResponse
	if err := json.Unmarshal(bodyBytes, &anthropicResp); err == nil {
		if anthropicResp.Error != nil {
			return "", fmt.Errorf("anthropic api error: %s", anthropicResp.Error.Message)
		}
		for _, content := range anthropicResp.Content {
			if content.Type == "text" && content.Text != "" {
				return content.Text, nil
			}
		}
	}

	// Fall back to Minimax format
	var minimaxResp minimaxResponse
	if err := json.Unmarshal(bodyBytes, &minimaxResp); err == nil {
		if minimaxResp.Error != nil {
			return "", fmt.Errorf("minimax api error: code %d, message: %s", minimaxResp.Error.Code, minimaxResp.Error.Message)
		}
		if len(minimaxResp.Content) > 0 && minimaxResp.Content[0].Text != "" {
			return minimaxResp.Content[0].Text, nil
		}
	}

	// If no valid response found
	return "", fmt.Errorf("invalid api response format: %s", string(bodyBytes))
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
					break  // Stop at next tag
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
