package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// MockMinimaxClient simulates MiniMax API for testing
type MockMinimaxClient struct {
	APIKey string
	Model  string
	Calls  int
}

// MockResponse simulates AI responses
type MockResponse struct {
	Responses map[string]string
}

func NewMockMinimaxClient(apiKey, model string) *MockMinimaxClient {
	return &MockMinimaxClient{
		APIKey: apiKey,
		Model:  model,
		Calls:  0,
	}
}

func (c *MockMinimaxClient) Complete(ctx context.Context, prompt string) (string, error) {
	c.Calls++
	
	// Parse the task from the prompt
	task := extractTask(prompt)
	
	// Generate appropriate response based on task
	response := c.generateResponse(task)
	
	return response, nil
}

func (c *MockMinimaxClient) generateResponse(task string) string {
	task = strings.ToLower(task)
	
	// Detect intent and generate appropriate command-based response
	switch {
	case strings.Contains(task, "list") && strings.Contains(task, "file"):
		return generateListFilesResponse()
	case strings.Contains(task, "read") && strings.Contains(task, "file"):
		return generateReadFileResponse(task)
	case strings.Contains(task, "create") && strings.Contains(task, "file"):
		return generateCreateFileResponse(task)
	case strings.Contains(task, "find") || strings.Contains(task, "search"):
		return generateSearchResponse(task)
	case strings.Contains(task, "go version"):
		return generateGoVersionResponse()
	case strings.Contains(task, "date") || strings.Contains(task, "time"):
		return generateDateTimeResponse()
	case strings.Contains(task, "cpu") || strings.Contains(task, "core"):
		return generateCPUResponse()
	case strings.Contains(task, "count"):
		return generateCountResponse(task)
	case strings.Contains(task, "directory") || strings.Contains(task, "folder"):
		return generateDirResponse(task)
	case strings.Contains(task, "exist"):
		return generateExistResponse(task)
	default:
		return generateDefaultResponse(task)
	}
}

func extractTask(prompt string) string {
	// Extract user message from prompt
	lines := strings.Split(prompt, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[user]") || strings.HasPrefix(line, "[USER]") {
			return strings.TrimSpace(strings.TrimPrefix(line, "[user]"))
		}
	}
	// Fallback to last line
	if len(lines) > 0 {
		return strings.TrimSpace(lines[len(lines)-1])
	}
	return prompt
}

func generateListFilesResponse() string {
	return `I'll list the files in the current directory.

{
  "tool_calls": [
    {
      "id": "list_dir_1",
      "name": "list_dir",
      "arguments": {
        "path": "."
      }
    }
  ]
}`
}

func generateReadFileResponse(task string) string {
	// Extract filename from task
	filename := "go.mod"
	if strings.Contains(task, "go.mod") {
		filename = "go.mod"
	} else if strings.Contains(task, "test.txt") {
		filename = "test.txt"
	}
	
	return fmt.Sprintf(`I'll read the %s file.

{
  "tool_calls": [
    {
      "id": "read_%s_1",
      "name": "read_file",
      "arguments": {
        "path": "%s"
      }
    }
  ]
}`, filename, filename, filename)
}

func generateCreateFileResponse(task string) string {
	filename := "test.txt"
	if strings.Contains(task, "test.txt") {
		filename = "test.txt"
	}
	
	return fmt.Sprintf(`I'll create the %s file with the specified content.

{
  "tool_calls": [
    {
      "id": "write_%s_1",
      "name": "write_file",
      "arguments": {
        "path": "%s",
        "content": "Hello World"
      }
    }
  ]
}`, filename, filename, filename)
}

func generateSearchResponse(task string) string {
	if strings.Contains(task, ".go file") {
		return `I'll find all .go files in the project.

{
  "tool_calls": [
    {
      "id": "search_go_1",
      "name": "search_files",
      "arguments": {
        "pattern": "*.go"
      }
    }
  ]
}`
	}
	
	if strings.Contains(task, "func") {
		return `I'll search for 'func' in all .go files.

{
  "tool_calls": [
    {
      "id": "grep_func_1",
      "name": "grep",
      "arguments": {
        "pattern": "func",
        "path": ".",
        "recursive": true
      }
    }
  ]
}`
	}
	
	return `I'll search for the requested content.

{
  "tool_calls": [
    {
      "id": "grep_search_1",
      "name": "grep",
      "arguments": {
        "pattern": "test",
        "path": ".",
        "recursive": true
      }
    }
  ]
}`
}

func generateGoVersionResponse() string {
	return `I'll check the Go version.

{
  "tool_calls": [
    {
      "id": "exec_go_version_1",
      "name": "exec",
      "arguments": {
        "command": "go version"
      }
    }
  ]
}`
}

func generateDateTimeResponse() string {
	return `I'll show the current date and time.

{
  "tool_calls": [
    {
      "id": "exec_date_1",
      "name": "exec",
      "arguments": {
        "command": "date"
      }
    }
  ]
}`
}

func generateCPUResponse() string {
	return `I'll check the CPU cores available.

{
  "tool_calls": [
    {
      "id": "exec_cpu_1",
      "name": "exec",
      "arguments": {
        "command": "sysctl -n hw.ncpu"
      }
    }
  ]
}`
}

func generateCountResponse(task string) string {
	if strings.Contains(task, "file") {
		return `I'll list and count all files in the directory.

{
  "tool_calls": [
    {
      "id": "list_count_1",
      "name": "list_dir",
      "arguments": {
        "path": "."
      }
    }
  ]
}`
	}
	return `I'll count the requested items.

{
  "tool_calls": [
    {
      "id": "exec_wc_1",
      "name": "exec",
      "arguments": {
        "command": "echo 0"
      }
    }
  ]
}`
}

func generateDirResponse(task string) string {
	if strings.Contains(task, "cmd") {
		return `I'll list the contents of the cmd directory.

{
  "tool_calls": [
    {
      "id": "list_cmd_1",
      "name": "list_dir",
      "arguments": {
        "path": "cmd"
      }
    }
  ]
}`
	}
	
	if strings.Contains(task, "internal") {
		return `I'll check if internal directory exists and list it.

{
  "tool_calls": [
    {
      "id": "list_internal_1",
      "name": "list_dir",
      "arguments": {
        "path": "internal"
      }
    }
  ]
}`
	}
	
	if strings.Contains(task, "create") {
		return `I'll create the temp_test directory and list it.

{
  "tool_calls": [
    {
      "id": "exec_mkdir_1",
      "name": "exec",
      "arguments": {
        "command": "mkdir -p temp_test && ls -la temp_test"
      }
    }
  ]
}`
	}
	
	return `I'll list the directory contents.

{
  "tool_calls": [
    {
      "id": "list_dir_1",
      "name": "list_dir",
      "arguments": {
        "path": "."
      }
    }
  ]
}`
}

func generateExistResponse(task string) string {
	if strings.Contains(task, "internal") {
		return `I'll check if the internal directory exists.

{
  "tool_calls": [
    {
      "id": "list_internal_1",
      "name": "list_dir",
      "arguments": {
        "path": "internal"
      }
    }
  ]
}`
	}
	return `I'll check if the requested item exists.

{
  "tool_calls": [
    {
      "id": "exec_test_1",
      "name": "exec",
      "arguments": {
        "command": "ls -la"
      }
    }
  ]
}`
}

func generateDefaultResponse(task string) string {
	return fmt.Sprintf(`I'll complete the task: %s

{
  "tool_calls": [
    {
      "id": "exec_default_1",
      "name": "exec",
      "arguments": {
        "command": "echo 'Task completed'"
      }
    }
  ]
}`, task[:100])
}

// MockComplete is a simple mock that returns success
func MockComplete(ctx context.Context, prompt string) (string, error) {
	return "✅ Task completed successfully!\n\nI've analyzed your request and executed the necessary commands. The task has been completed as requested.", nil
}

// UseMockClient creates a mock application for testing
func UseMockClient(app *Application) {
	// Replace the real client with mock
	app.Client = &MinimaxClient{
		APIKey: "mock",
		Model:  "mock",
		BaseURL: "mock://",
		MaxTokens: 2048,
		HTTP: &http.Client{Timeout: 1 * time.Second},
	}
}

// MockAgentLoop creates an agent loop with mock client
func MockAgentLoop(maxLoops int, stateDir string, logger *Logger) *AgentLoop {
	mockClient := &MinimaxClient{
		APIKey: "mock",
		Model:  "mock",
		BaseURL: "mock://",
		MaxTokens: 2048,
		HTTP: &http.Client{Timeout: 1 * time.Second},
	}
	
	return &AgentLoop{
		Client:   mockClient,
		Tools:    DefaultTools(),
		MaxLoops: maxLoops,
		StateDir: stateDir,
		Logger:   logger,
	}
}
