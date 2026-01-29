package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

type AgentState struct {
	TaskID      string         `json:"task_id"`
	Task        string         `json:"task"`
	Iteration   int            `json:"iteration"`
	MaxLoops    int            `json:"max_loops"`
	Messages    []AgentMessage `json:"messages"`
	Results     []ToolResult   `json:"results"`
	Completed   bool           `json:"completed"`
	FinalOutput string         `json:"final_output,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	EndedAt     time.Time      `json:"ended_at,omitempty"`
}

type AgentMessage struct {
	Role        string       `json:"role"`
	Content     string       `json:"content,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
	Timestamp   time.Time    `json:"timestamp"`
}

type AgentLoop struct {
	Client   *MinimaxClient
	Tools    []Tool
	MaxLoops int
	StateDir string
	Logger   *Logger
}

func NewAgentLoop(client *MinimaxClient, maxLoops int, stateDir string, logger *Logger) *AgentLoop {
	if maxLoops <= 0 {
		maxLoops = 10
	}
	return &AgentLoop{
		Client:   client,
		Tools:    DefaultTools(),
		MaxLoops: maxLoops,
		StateDir: stateDir,
		Logger:   logger,
	}
}

func DefaultTools() []Tool {
	return []Tool{
		{
			Name:        "exec",
			Description: "Execute a shell command and return its output",
			Parameters: mustMarshal(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute",
					},
					"timeout": map[string]interface{}{
						"type":        "integer",
						"description": "Timeout in seconds (default: 30)",
					},
				},
				"required": []string{"command"},
			}),
		},
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			Parameters: mustMarshal(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to read",
					},
				},
				"required": []string{"path"},
			}),
		},
		{
			Name:        "write_file",
			Description: "Create or overwrite a file with the given content",
			Parameters: mustMarshal(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write to the file",
					},
				},
				"required": []string{"path", "content"},
			}),
		},
		{
			Name:        "list_dir",
			Description: "List files in a directory",
			Parameters: mustMarshal(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the directory",
					},
				},
			}),
		},
		{
			Name:        "grep",
			Description: "Search for text in files using grep",
			Parameters: mustMarshal(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Search pattern",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to search in",
					},
					"recursive": map[string]interface{}{
						"type":        "boolean",
						"description": "Search recursively",
					},
				},
				"required": []string{"pattern"},
			}),
		},
		{
			Name:        "search_files",
			Description: "Search for files matching a pattern",
			Parameters: mustMarshal(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Glob pattern (e.g., *.go)",
					},
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Base path to search from",
					},
				},
				"required": []string{"pattern"},
			}),
		},
		{
			Name:        "edit_file",
			Description: "Edit a file by replacing old_text with new_text (in-place modification)",
			Parameters: mustMarshal(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to edit",
					},
					"old_text": map[string]interface{}{
						"type":        "string",
						"description": "Text to find and replace (exact match)",
					},
					"new_text": map[string]interface{}{
						"type":        "string",
						"description": "Replacement text",
					},
				},
				"required": []string{"path", "old_text", "new_text"},
			}),
		},
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return data
}

func (l *AgentLoop) Execute(ctx context.Context, task string) (*AgentState, error) {
	state := &AgentState{
		TaskID:    uuid.New().String(),
		Task:      task,
		MaxLoops:  l.MaxLoops,
		Messages:  []AgentMessage{},
		Results:   []ToolResult{},
		StartedAt: time.Now(),
	}

	if l.StateDir != "" {
		os.MkdirAll(l.StateDir, 0755)
	}

	systemMsg := l.buildSystemMessage()
	state.Messages = append(state.Messages, AgentMessage{
		Role:      "system",
		Content:   systemMsg,
		Timestamp: time.Now(),
	})

	userMsg := AgentMessage{
		Role:      "user",
		Content:   task,
		Timestamp: time.Now(),
	}
	state.Messages = append(state.Messages, userMsg)

	for state.Iteration < l.MaxLoops {
		l.saveState(state)

		response, err := l.Client.Complete(ctx, l.buildPrompt(state.Messages))
		if err != nil {
			l.Logger.Error("Failed to get model response", map[string]interface{}{
				"error": err.Error(),
			})
			state.Messages = append(state.Messages, AgentMessage{
				Role:      "assistant",
				Content:   fmt.Sprintf("Error: %v", err),
				Timestamp: time.Now(),
			})
			break
		}

		state.Messages = append(state.Messages, AgentMessage{
			Role:      "assistant",
			Content:   response,
			Timestamp: time.Now(),
		})

		toolCalls := l.parseToolCalls(response)

		if len(toolCalls) == 0 {
			state.Completed = true
			state.FinalOutput = response
			break
		}

		for _, call := range toolCalls {
			result := l.executeTool(ctx, call)
			state.Results = append(state.Results, result)

			resultMsg := AgentMessage{
				Role:        "user",
				Content:     fmt.Sprintf("Tool result for %s:\n%s", call.Name, result.Output),
				ToolResults: []ToolResult{result},
				Timestamp:   time.Now(),
			}
			state.Messages = append(state.Messages, resultMsg)
		}

		state.Iteration++
	}

	state.EndedAt = time.Now()
	l.saveState(state)

	if !state.Completed {
		state.FinalOutput = fmt.Sprintf("Task did not complete within %d iterations", l.MaxLoops)
	}

	return state, nil
}

func (l *AgentLoop) buildPrompt(messages []AgentMessage) string {
	prompt := ""
	for _, msg := range messages {
		prompt += fmt.Sprintf("[%s]\n%s\n\n", msg.Role, msg.Content)
	}
	return prompt
}

func (l *AgentLoop) buildSystemMessage() string {
	return l.buildSystemMessageEnhanced()
}

// GetTaskCategory returns the category for task-specific prompts
func GetTaskCategory(task string) string {
	taskLower := strings.ToLower(task)

	switch {
	case strings.Contains(taskLower, "git"):
		return "git"
	case strings.Contains(taskLower, "build") || strings.Contains(taskLower, "compile") ||
		strings.Contains(taskLower, "cmake") || strings.Contains(taskLower, "make") ||
		strings.Contains(taskLower, "cython") || strings.Contains(taskLower, "pov-ray") ||
		strings.Contains(taskLower, "caffe") || strings.Contains(taskLower, "pmars"):
		return "build"
	case strings.Contains(taskLower, "nginx") || strings.Contains(taskLower, "ssh") ||
		strings.Contains(taskLower, "docker") || strings.Contains(taskLower, "ssl") ||
		strings.Contains(taskLower, "cert") || strings.Contains(taskLower, "qemu") ||
		strings.Contains(taskLower, "pypi") || strings.Contains(taskLower, "windows"):
		return "devops"
	case strings.Contains(taskLower, "pytorch") || strings.Contains(taskLower, "torch") ||
		strings.Contains(taskLower, "tensorflow") || strings.Contains(taskLower, "model") ||
		strings.Contains(taskLower, "mcmc") || strings.Contains(taskLower, "stan") ||
		strings.Contains(taskLower, "sampling"):
		return "ml"
	case strings.Contains(taskLower, "sqlite") || strings.Contains(taskLower, "database") ||
		strings.Contains(taskLower, "sql"):
		return "database"
	case strings.Contains(taskLower, "regex") || strings.Contains(taskLower, "extract") ||
		strings.Contains(taskLower, "crack") || strings.Contains(taskLower, "eigenval") ||
		strings.Contains(taskLower, "distribution") || strings.Contains(taskLower, "query") ||
		strings.Contains(taskLower, "adaptive") || strings.Contains(taskLower, "install"):
		return "simple"
	default:
		return "default"
	}
}

func (l *AgentLoop) parseToolCalls(response string) []ToolCall {
	var toolCalls []ToolCall

	// === Format 0: Perl hash syntax {tool => "...", args => { --key "value" }} ===
	if strings.Contains(response, "{tool") && strings.Contains(response, "=>") {
		toolMatch := regexp.MustCompile(`tool\s*=>\s*"([^"]+)"`).FindStringSubmatch(response)
		if len(toolMatch) > 1 && toolMatch[1] != "" {
			toolName := toolMatch[1]
			argsMatch := regexp.MustCompile(`args\s*=>\s*\{([\s\S]*?)\}`).FindStringSubmatch(response)
			args := make(map[string]interface{})
			if len(argsMatch) > 1 {
				pairs := regexp.MustCompile(`--(\w+)\s*"([^"]+)"`).FindAllStringSubmatch(argsMatch[1], -1)
				for _, pair := range pairs {
					if len(pair) > 2 {
						args[pair[1]] = pair[2]
					}
				}
			}
			arguments, _ := json.Marshal(args)
			toolCalls = append(toolCalls, ToolCall{
				ID:        fmt.Sprintf("%s_1", toolName),
				Name:      toolName,
				Arguments: arguments,
			})
			return toolCalls
		}
	}

	// === Format 1: [TOOL_CALL] or [ToolCall] ===
	if strings.Contains(response, "[TOOL_CALL]") || strings.Contains(response, "[ToolCall]") || strings.Contains(response, "[tool_call]") {
		if startIdx := strings.Index(response, "[TOOL_CALL]"); startIdx >= 0 {
			content := response[startIdx+len("[TOOL_CALL]"):]
			endIdx := -1
			if strings.Contains(content, "[/TOOL_CALL]") {
				endIdx = strings.Index(content, "[/TOOL_CALL]")
			} else if strings.Contains(content, "[/ToolCall]") {
				endIdx = strings.Index(content, "[/ToolCall]")
			} else if strings.Contains(content, "[/tool_call]") {
				endIdx = strings.Index(content, "[/tool_call]")
			}
			if endIdx >= 0 {
				content = content[:endIdx]

				// Handle remaining => syntax
				content = strings.Replace(content, "=>", ":", -1)

				// Try {"tool": "...", "args": {...}}
				var resp struct {
					Tool string                 `json:"tool"`
					Args map[string]interface{} `json:"args"`
				}
				if err := json.Unmarshal([]byte(content), &resp); err == nil && resp.Tool != "" {
					arguments, _ := json.Marshal(resp.Args)
					toolCalls = append(toolCalls, ToolCall{
						ID:        fmt.Sprintf("%s_1", resp.Tool),
						Name:      resp.Tool,
						Arguments: arguments,
					})
					return toolCalls
				}

				// Try direct args: {"tool": "...", "command": "...", ...}
				var rawResp map[string]interface{}
				if err := json.Unmarshal([]byte(content), &rawResp); err == nil {
					if toolName, ok := rawResp["tool"].(string); ok && toolName != "" {
						args := make(map[string]interface{})
						for key, val := range rawResp {
							if key != "tool" {
								args[key] = val
							}
						}
						arguments, _ := json.Marshal(args)
						toolCalls = append(toolCalls, ToolCall{
							ID:        fmt.Sprintf("%s_1", toolName),
							Name:      toolName,
							Arguments: arguments,
						})
						return toolCalls
					}

					// Try {"exec": {"command": ...}} or {"list_dir": {"path": ...}}
					for toolName, args := range rawResp {
						if argsMap, ok := args.(map[string]interface{}); ok {
							arguments, _ := json.Marshal(argsMap)
							toolCalls = append(toolCalls, ToolCall{
								ID:        fmt.Sprintf("%s_1", toolName),
								Name:      toolName,
								Arguments: arguments,
							})
							return toolCalls
						}
					}
				}
			}
		}
	}

	// === Format 2: [tool_calls]{"command": "..."}[/tool_calls] - exec without tool name ===
	if strings.Contains(response, "[tool_calls]") {
		if startIdx := strings.Index(response, "[tool_calls]"); startIdx >= 0 {
			content := response[startIdx+len("[tool_calls]"):]
			if endIdx := strings.Index(content, "[/tool_calls]"); endIdx >= 0 {
				content = content[:endIdx]

				// Handle => syntax
				content = strings.Replace(content, "=>", ":", -1)

				// Try {"command": "..."} - no tool name, default to "exec"
				var execResp struct {
					Command string `json:"command"`
					Timeout int    `json:"timeout"`
				}
				if err := json.Unmarshal([]byte(content), &execResp); err == nil && execResp.Command != "" {
					arguments, _ := json.Marshal(execResp)
					toolCalls = append(toolCalls, ToolCall{
						ID:        "exec_1",
						Name:      "exec",
						Arguments: arguments,
					})
					return toolCalls
				}

				// Try {"list_dir": {"path": "."}} - tool name as key
				var toolObj map[string]map[string]interface{}
				if err := json.Unmarshal([]byte(content), &toolObj); err == nil {
					for toolName, args := range toolObj {
						arguments, _ := json.Marshal(args)
						toolCalls = append(toolCalls, ToolCall{
							ID:        fmt.Sprintf("%s_1", toolName),
							Name:      toolName,
							Arguments: arguments,
						})
						return toolCalls
					}
				}

				// Try {"tool_name", {"key": value}} - comma syntax
				commaMatch := regexp.MustCompile(`\{\s*"(\w+)"\s*,\s*\{([^}]+)\}\s*\}`).FindStringSubmatch(content)
				if len(commaMatch) > 2 {
					toolName := commaMatch[1]
					argsStr := commaMatch[2]
					args := make(map[string]interface{})
					pairs := regexp.MustCompile(`"(\w+)":\s*"([^"]+)"`).FindAllStringSubmatch(argsStr, -1)
					for _, pair := range pairs {
						if len(pair) > 2 {
							args[pair[1]] = pair[2]
						}
					}
					arguments, _ := json.Marshal(args)
					toolCalls = append(toolCalls, ToolCall{
						ID:        fmt.Sprintf("%s_1", toolName),
						Name:      toolName,
						Arguments: arguments,
					})
					return toolCalls
				}

				// Try {"name": "...", "args": {...}}
				var nameResp struct {
					Name string                 `json:"name"`
					Args map[string]interface{} `json:"args"`
				}
				if err := json.Unmarshal([]byte(content), &nameResp); err == nil && nameResp.Name != "" {
					arguments, _ := json.Marshal(nameResp.Args)
					toolCalls = append(toolCalls, ToolCall{
						ID:        fmt.Sprintf("%s_1", nameResp.Name),
						Name:      nameResp.Name,
						Arguments: arguments,
					})
					return toolCalls
				}
			}
		}
	}

	// === Format 3: {"tool": "...", ...} anywhere in response ===
	// === Format 5: {"tool": "...", ...} anywhere in response ===
	if strings.Contains(response, `"tool"`) {
		if startIdx := strings.Index(response, `"tool"`); startIdx >= 0 {
			braceStart := strings.LastIndex(response[:startIdx], "{")
			if braceStart >= 0 {
				braceCount := 0
				for i := braceStart; i < len(response); i++ {
					if response[i] == '{' {
						braceCount++
					} else if response[i] == '}' {
						braceCount--
						if braceCount == 0 {
							jsonStr := response[braceStart : i+1]

							// Handle => syntax
							jsonStr = strings.Replace(jsonStr, "=>", ":", -1)

							var rawResp map[string]interface{}
							if err := json.Unmarshal([]byte(jsonStr), &rawResp); err == nil {
								if toolName, ok := rawResp["tool"].(string); ok && toolName != "" {
									args := make(map[string]interface{})
									for key, val := range rawResp {
										if key != "tool" {
											args[key] = val
										}
									}
									arguments, _ := json.Marshal(args)
									toolCalls = append(toolCalls, ToolCall{
										ID:        fmt.Sprintf("%s_1", toolName),
										Name:      toolName,
										Arguments: arguments,
									})
									return toolCalls
								}
							}
							break
						}
					}
				}
			}
		}
	}

	// === Format 4: Plain text with command - fallback ===
	// Look for common command patterns in plain text
	commandPatterns := []string{
		`go version`,
		`go build`,
		`ls -la`,
		`ls -l`,
		`ls `,
		`echo `,
		`cat `,
		`find `,
		`grep `,
		`touch `,
		`mkdir -p`,
	}
	for _, pattern := range commandPatterns {
		if strings.Contains(response, pattern) {
			// Extract the command
			cmdMatch := regexp.MustCompile(`'` + pattern + `[^']*'`).FindString(response)
			if cmdMatch == "" {
				cmdMatch = regexp.MustCompile(pattern + `[^\s'"` + "`" + `]+`).FindString(response)
			}
			if cmdMatch == "" {
				cmdMatch = pattern
			}
			if cmdMatch != "" {
				toolCalls = append(toolCalls, ToolCall{
					ID:        "exec_1",
					Name:      "exec",
					Arguments: json.RawMessage(fmt.Sprintf(`{"command": "%s"}`, cmdMatch)),
				})
				return toolCalls
			}
		}
	}

	// === Format 4b: Additional patterns for common commands ===
	// Match "I'll run/go version" patterns
	runMatch := regexp.MustCompile(`(?:run|execute)\s+(?:the\s+)?(?:command\s+)?["']?([a-zA-Z0-9\s\-./_]+?)["']?(?:\s|$|\.)`).FindStringSubmatch(response)
	if len(runMatch) > 1 {
		cmd := strings.TrimSpace(runMatch[1])
		if len(cmd) > 2 && len(cmd) < 100 {
			toolCalls = append(toolCalls, ToolCall{
				ID:        "exec_1",
				Name:      "exec",
				Arguments: json.RawMessage(fmt.Sprintf(`{"command": "%s"}`, cmd)),
			})
			return toolCalls
		}
	}

	return toolCalls
}

func (l *AgentLoop) saveState(state *AgentState) {
	if l.StateDir == "" {
		return
	}
	statePath := fmt.Sprintf("%s/%s.json", l.StateDir, state.TaskID)
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(statePath, data, 0644)
}

func (l *AgentLoop) executeTool(ctx context.Context, call ToolCall) ToolResult {
	start := time.Now()
	result := ToolResult{
		ToolCallID: call.ID,
		Success:    false,
	}

	switch call.Name {
	case "exec":
		var args struct {
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		timeout := 30 * time.Second
		if args.Timeout > 0 {
			timeout = time.Duration(args.Timeout) * time.Second
		}

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sh", "-c", args.Command)
		output, err := cmd.CombinedOutput()
		if err != nil {
			result.Error = err.Error()
		}
		result.Output = string(output)
		result.Success = err == nil

	case "read_file":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		data, err := os.ReadFile(args.Path)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Output = string(data)
			result.Success = true
		}

	case "write_file":
		var args struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		dir := args.Path[:strings.LastIndex(args.Path, "/")]
		if err := os.MkdirAll(dir, 0755); err != nil {
			result.Error = fmt.Sprintf("Failed to create directory: %v", err)
		} else if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
			result.Error = err.Error()
		} else {
			result.Output = fmt.Sprintf("File written: %s", args.Path)
			result.Success = true
		}

	case "list_dir":
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		if args.Path == "" {
			args.Path = "."
		}

		entries, err := os.ReadDir(args.Path)
		if err != nil {
			result.Error = err.Error()
		} else {
			var lines []string
			for _, entry := range entries {
				if entry.IsDir() {
					lines = append(lines, fmt.Sprintf("📁 %s/", entry.Name()))
				} else {
					lines = append(lines, fmt.Sprintf("📄 %s", entry.Name()))
				}
			}
			result.Output = strings.Join(lines, "\n")
			result.Success = true
		}

	case "grep":
		var args struct {
			Pattern   string `json:"pattern"`
			Path      string `json:"path"`
			Recursive bool   `json:"recursive"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		if args.Path == "" {
			args.Path = "."
		}

		flags := "-rHn"
		if !args.Recursive {
			flags = "-Hn"
		}

		cmd := exec.CommandContext(ctx, "grep", flags, args.Pattern, args.Path)
		output, err := cmd.CombinedOutput()
		if err != nil && len(output) == 0 {
			result.Error = err.Error()
		} else {
			result.Output = string(output)
			result.Success = true
		}

	case "search_files":
		var args struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		if args.Path == "" {
			args.Path = "."
		}

		cmd := exec.CommandContext(ctx, "find", args.Path, "-name", args.Pattern, "-type", "f")
		output, err := cmd.CombinedOutput()
		if err != nil && len(output) == 0 {
			result.Error = err.Error()
		} else {
			result.Output = string(output)
			result.Success = true
		}

	case "edit_file":
		var args struct {
			Path    string `json:"path"`
			OldText string `json:"old_text"`
			NewText string `json:"new_text"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		data, err := os.ReadFile(args.Path)
		if err != nil {
			result.Error = err.Error()
		} else {
			content := string(data)
			if !strings.Contains(content, args.OldText) {
				result.Error = fmt.Sprintf("Text not found in file: %s", args.OldText)
			} else {
				newContent := strings.Replace(content, args.OldText, args.NewText, 1)
				if err := os.WriteFile(args.Path, []byte(newContent), 0644); err != nil {
					result.Error = err.Error()
				} else {
					result.Output = fmt.Sprintf("File edited: %s", args.Path)
					result.Success = true
				}
			}
		}

	default:
		result.Error = fmt.Sprintf("Unknown tool: %s", call.Name)
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}
