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

// Retry configuration for transient failures
var retryableErrors = []string{
	"resource temporarily unavailable",
	"connection reset by peer",
	"connection refused",
	"temporary failure",
	"text file busy",
	"no such process",
}

// isRetryable checks if an error is transient and can be retried
func isRetryable(err string) bool {
	errLower := strings.ToLower(err)
	for _, pattern := range retryableErrors {
		if strings.Contains(errLower, pattern) {
			return true
		}
	}
	return false
}

// isResponseTruncated checks if an API response appears to be truncated
func isResponseTruncated(response string) bool {
	response = strings.TrimSpace(response)
	if len(response) == 0 {
		return false
	}

	// Check for unclosed braces/brackets
	openBraces := strings.Count(response, "{") - strings.Count(response, "}")
	openBrackets := strings.Count(response, "[") - strings.Count(response, "]")
	if openBraces > 0 || openBrackets > 0 {
		return true
	}

	// Check for ends with backslash (line continuation)
	if strings.HasSuffix(response, "\\") {
		return true
	}

	// Check for incomplete patterns
	incompletePatterns := []string{
		`"tool":`,
		`"args":`,
		`"command":`,
		`"path":`,
		`":`,
		`",`,
	}
	for _, pattern := range incompletePatterns {
		if strings.HasSuffix(response, pattern) {
			return true
		}
	}

	// Check for common truncation lengths (API limits)
	commonTruncationLengths := []int{4096, 8192, 16384, 32768}
	responseLen := len(response)
	for _, limit := range commonTruncationLengths {
		if responseLen >= limit-10 && responseLen <= limit {
			// Response is suspiciously close to a common limit
			if strings.Contains(response, `"tool"`) {
				return true
			}
		}
	}

	// Check for incomplete JSON key (ends with a quote)
	lastQuote := strings.LastIndex(response, `"`)
	if lastQuote > 0 && lastQuote == len(response)-1 {
		// Ends with a quote - might be truncated key or value
		return strings.Contains(response, `"tool"`)
	}

	return false
}

// isExplicitCompletion checks if the response indicates explicit task completion
func isExplicitCompletion(response string) bool {
	responseLower := strings.ToLower(response)

	completionPhrases := []string{
		"task complete",
		"task is complete",
		"task completed",
		"task has been completed",
		"i have completed",
		"successfully completed",
		"done with the task",
		"finished the task",
		"task is done",
		"task is finished",
		"all done",
		"task accomplished",
	}

	for _, phrase := range completionPhrases {
		if strings.Contains(responseLower, phrase) {
			return true
		}
	}
	return false
}

// extractExpectedOutputFiles looks for file paths mentioned in the instruction
func extractExpectedOutputFiles(instruction string) []string {
	var paths []string

	// Look for common output file patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:output|create|write|save|generate).*?(/[a-zA-Z0-9_/.-]+\.[a-zA-Z0-9]+)`),
		regexp.MustCompile(`(?:file|path).*?['"](/[^'"]+)['"]`),
		regexp.MustCompile(`(/app/[a-zA-Z0-9_.-]+\.[a-zA-Z0-9]+)`),
		regexp.MustCompile(`output.*?to\s+(/[^\s]+)`),
	}

	for _, re := range patterns {
		matches := re.FindAllStringSubmatch(instruction, -1)
		for _, match := range matches {
			if len(match) > 1 && match[1] != "" {
				paths = append(paths, match[1])
			}
		}
	}

	return paths
}

// verifyFilesExist checks if the expected output files exist
func verifyFilesExist(paths []string) ([]string, []string) {
	var existing, missing []string
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		} else {
			missing = append(missing, path)
		}
	}
	return existing, missing
}

// Package-level config instance (can be overridden via environment)
var agentCfg = AgentConfigFromEnv()

// SetAgentConfig allows overriding the default configuration
func SetAgentConfig(cfg AgentConfig) {
	agentCfg = cfg
}

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

	// Extract expected output files from task for verification
	expectedFiles := extractExpectedOutputFiles(task)

	// Track consecutive no-action responses
	consecutiveNoAction := 0
	maxNoActionAttempts := 3

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

		// Check for truncation if no tool calls found
		if len(toolCalls) == 0 && isResponseTruncated(response) {
			l.Logger.Warn("Response appears truncated", map[string]interface{}{
				"response_length": len(response),
			})

			// If we see a partial tool call, prompt for continuation
			if strings.Contains(response, `"tool"`) || strings.Contains(response, `"write_file"`) || strings.Contains(response, `"exec"`) {
				promptMsg := AgentMessage{
					Role:      "user",
					Content:   "Your response was truncated. Please provide ONLY the tool call JSON without explanation. Keep file content SHORT. Format: {\"tool\": \"write_file\", \"args\": {\"path\": \"/app/file.py\", \"content\": \"short content\"}}",
					Timestamp: time.Now(),
				}
				state.Messages = append(state.Messages, promptMsg)
				state.Iteration++
				continue
			}
		}

		if len(toolCalls) == 0 {
			consecutiveNoAction++

			// Check if model explicitly says task is complete
			explicitlyComplete := isExplicitCompletion(response)

			// Check if we've done any meaningful work
			hasExecutedTools := len(state.Results) > 0

			// If explicit completion and we've done work, verify and finish
			if explicitlyComplete && hasExecutedTools {
				// Verify expected files exist before completing
				if len(expectedFiles) > 0 {
					existing, missing := verifyFilesExist(expectedFiles)
					if len(missing) > 0 {
						l.Logger.Warn("Task claims complete but expected files missing", map[string]interface{}{
							"missing":  missing,
							"existing": existing,
						})
						// Prompt the model to create missing files
						promptMsg := AgentMessage{
							Role:      "user",
							Content:   fmt.Sprintf("WARNING: The following expected output files are missing: %v\nPlease create these files to complete the task. Use the write_file tool with the correct path and content.", missing),
							Timestamp: time.Now(),
						}
						state.Messages = append(state.Messages, promptMsg)
						state.Iteration++
						continue
					}
				}
				state.Completed = true
				state.FinalOutput = response
				break
			}

			// If too many no-action attempts, give up
			if consecutiveNoAction >= maxNoActionAttempts {
				l.Logger.Warn("Too many consecutive no-action responses", map[string]interface{}{
					"attempts": consecutiveNoAction,
				})
				state.Completed = true
				state.FinalOutput = response
				break
			}

			// Prompt the model to take action with emphasis on brevity
			actionPrompt := "You haven't taken any action yet. Please use one of the available tools to complete the task.\n" +
				"IMPORTANT: Respond with ONLY the JSON tool call. No explanation, no prose.\n" +
				"Format: {\"tool\": \"tool_name\", \"args\": {...}}\n" +
				"Available tools: exec, write_file, read_file, list_dir, grep, edit_file\n"

			if !hasExecutedTools {
				actionPrompt += "For write_file: Keep content SHORT. Write skeleton code first, then add details incrementally."
			} else {
				actionPrompt += "If the task is truly complete, respond with 'Task completed successfully'."
			}

			promptMsg := AgentMessage{
				Role:      "user",
				Content:   actionPrompt,
				Timestamp: time.Now(),
			}
			state.Messages = append(state.Messages, promptMsg)
			state.Iteration++
			continue
		}

		// Reset counter when we get valid tool calls
		consecutiveNoAction = 0

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
	return GetEnhancedSystemPrompt()
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

	// === Format -1: Extract JSON from markdown code blocks ===
	// Look for ```json ... ``` blocks first
	jsonBlockRe := regexp.MustCompile("```json\\s*\\n([\\s\\S]*?)\\n```")
	if matches := jsonBlockRe.FindAllStringSubmatch(response, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				content := strings.TrimSpace(match[1])
				// Try to parse as tool call
				var toolResp struct {
					Tool string                 `json:"tool"`
					Args map[string]interface{} `json:"args"`
				}
				if err := json.Unmarshal([]byte(content), &toolResp); err == nil && toolResp.Tool != "" {
					arguments, _ := json.Marshal(toolResp.Args)
					toolCalls = append(toolCalls, ToolCall{
						ID:        fmt.Sprintf("%s_1", toolResp.Tool),
						Name:      toolResp.Tool,
						Arguments: arguments,
					})
					return toolCalls
				}
				// Try flat format {"tool": "exec", "command": "..."} or {"tool": "...", "args": {...}}
				var rawResp map[string]interface{}
				if err := json.Unmarshal([]byte(content), &rawResp); err == nil {
					if toolName, ok := rawResp["tool"].(string); ok && toolName != "" {
						var arguments []byte

						// Check if there's already an "args" key
						if argsVal, hasArgs := rawResp["args"]; hasArgs {
							if argsMap, isMap := argsVal.(map[string]interface{}); isMap {
								arguments, _ = json.Marshal(argsMap)
							} else {
								arguments, _ = json.Marshal(argsVal)
							}
						} else {
							// No "args" key - collect all non-tool keys
							args := make(map[string]interface{})
							for key, val := range rawResp {
								if key != "tool" {
									args[key] = val
								}
							}
							arguments, _ = json.Marshal(args)
						}

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

	// === Format -0.5: Extract code from markdown and convert to write_file ===
	// Look for ```python, ```c, ```go etc. with path hints
	codeBlockRe := regexp.MustCompile("```(python|c|go|bash|sh|javascript|js|rust|cpp|java)\\s*\\n([\\s\\S]*?)\\n```")
	pathHintRe := regexp.MustCompile(`(?:create|write|save|output).*?['"](/[^'"]+)['"]|(?:file|path).*?['"](/[^'"]+)['"]|(/app/[a-zA-Z0-9_.-]+\.[a-z]+)`)

	// Map code block languages to file extensions
	langToExt := map[string][]string{
		"python":     {".py"},
		"c":          {".c", ".h"},
		"go":         {".go"},
		"bash":       {".sh", ".bash"},
		"sh":         {".sh", ".bash"},
		"javascript": {".js"},
		"js":         {".js"},
		"rust":       {".rs"},
		"cpp":        {".cpp", ".cc", ".h", ".hpp"},
		"java":       {".java"},
	}

	if matches := codeBlockRe.FindAllStringSubmatch(response, -1); len(matches) > 0 {
		// Look for path hints in the text before the code block
		pathMatches := pathHintRe.FindAllStringSubmatch(response, -1)
		if len(pathMatches) > 0 && len(matches) > 0 {
			// Get the first non-empty path
			var path string
			for _, pm := range pathMatches {
				for i := 1; i < len(pm); i++ {
					if pm[i] != "" {
						path = pm[i]
						break
					}
				}
				if path != "" {
					break
				}
			}
			if path != "" {
				// Find a code block that matches the file extension
				var code string
				var lang string
				pathExt := strings.ToLower(path[strings.LastIndex(path, "."):])

				for _, match := range matches {
					blockLang := strings.ToLower(match[1])
					if exts, ok := langToExt[blockLang]; ok {
						for _, ext := range exts {
							if ext == pathExt {
								code = match[2]
								lang = blockLang
								break
							}
						}
					}
					if code != "" {
						break
					}
				}

				// If no matching language found, skip this extraction
				if code != "" {
					arguments, _ := json.Marshal(map[string]interface{}{
						"path":    path,
						"content": code,
					})
					toolCalls = append(toolCalls, ToolCall{
						ID:        "write_file_1",
						Name:      "write_file",
						Arguments: arguments,
					})
					l.Logger.Info("Extracted code block as write_file", map[string]interface{}{
						"path": path,
						"lang": lang,
					})
					return toolCalls
				}
			}
		}
	}

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

				// Try direct args: {"tool": "...", "command": "...", ...} or {"tool": "...", "args": {...}}
				var rawResp map[string]interface{}
				if err := json.Unmarshal([]byte(content), &rawResp); err == nil {
					if toolName, ok := rawResp["tool"].(string); ok && toolName != "" {
						var arguments []byte

						// Check if there's already an "args" key
						if argsVal, hasArgs := rawResp["args"]; hasArgs {
							if argsMap, isMap := argsVal.(map[string]interface{}); isMap {
								arguments, _ = json.Marshal(argsMap)
							} else {
								arguments, _ = json.Marshal(argsVal)
							}
						} else {
							// No "args" key - collect all non-tool keys
							args := make(map[string]interface{})
							for key, val := range rawResp {
								if key != "tool" {
									args[key] = val
								}
							}
							arguments, _ = json.Marshal(args)
						}

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
									var arguments []byte

									// Check if there's already an "args" key with a map value
									if argsVal, hasArgs := rawResp["args"]; hasArgs {
										if argsMap, isMap := argsVal.(map[string]interface{}); isMap {
											// Use the existing args map directly
											arguments, _ = json.Marshal(argsMap)
										} else {
											// args exists but isn't a map, marshal as-is
											arguments, _ = json.Marshal(argsVal)
										}
									} else {
										// No "args" key - collect all non-tool keys
										args := make(map[string]interface{})
										for key, val := range rawResp {
											if key != "tool" {
												args[key] = val
											}
										}
										arguments, _ = json.Marshal(args)
									}

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

	// Check context cancellation before executing
	select {
	case <-ctx.Done():
		result.Error = "Operation cancelled"
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	default:
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

		if args.Path == "" {
			result.Error = "Path cannot be empty"
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

		if args.Path == "" {
			result.Error = "Path cannot be empty"
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		// Create parent directory if path contains a slash
		if slashIdx := strings.LastIndex(args.Path, "/"); slashIdx > 0 {
			dir := args.Path[:slashIdx]
			if err := os.MkdirAll(dir, 0755); err != nil {
				result.Error = fmt.Sprintf("Failed to create directory: %v", err)
				result.DurationMs = time.Since(start).Milliseconds()
				return result
			}
		}

		if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
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
		// Check context cancellation
		select {
		case <-ctx.Done():
			result.Error = "Operation cancelled"
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		default:
		}

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
		// Check context cancellation
		select {
		case <-ctx.Done():
			result.Error = "Operation cancelled"
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		default:
		}

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

	// Consistent logging for tool failures
	if !result.Success && l.Logger != nil {
		l.Logger.Error("Tool execution failed", map[string]interface{}{
			"tool":     call.Name,
			"error":    result.Error,
			"duration": result.DurationMs,
		})
	}

	return result
}
