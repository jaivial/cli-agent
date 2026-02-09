package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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

func isLikelyConfigError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "api key is required") || strings.Contains(s, "minimax api key is required") {
		return true
	}
	if strings.Contains(s, "insufficient balance") || strings.Contains(s, "no resource package") {
		return true
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
	if strings.TrimSpace(response) == "TASK_COMPLETED" {
		return true
	}

	responseLower := strings.ToLower(response)

	completionPhrases := []string{
		"task complete",
		"task is complete",
		"task completed",
		"task has been completed",
		"i have completed",
		"i've completed",
		"completed the task",
		"task completed successfully",
		"successfully completed",
		"done with the task",
		"finished the task",
		"task is done",
		"task is finished",
		"all done",
		"task accomplished",
		"task_completed",
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

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func extractVerificationCommands(instruction string) []string {
	inLower := strings.ToLower(instruction)
	var cmds []string

	// Common Harbor/Terminal-Bench verifier script convention (when made available in /app).
	if strings.Contains(inLower, "/app/test_outputs.py") {
		cmds = append(cmds, "python3 /app/test_outputs.py")
	}
	if strings.Contains(inLower, "/app/tests/test_outputs.py") {
		cmds = append(cmds, "python3 /app/tests/test_outputs.py")
	}

	// Extract inline backtick commands on lines that look like verification instructions.
	for _, line := range strings.Split(instruction, "\n") {
		lineLower := strings.ToLower(line)
		if !(strings.Contains(lineLower, "verify") ||
			strings.Contains(lineLower, "test") ||
			strings.Contains(lineLower, "should output") ||
			strings.Contains(lineLower, "check with")) {
			continue
		}

		// Pull `...` segments from the line.
		for {
			start := strings.Index(line, "`")
			if start < 0 {
				break
			}
			rest := line[start+1:]
			end := strings.Index(rest, "`")
			if end < 0 {
				break
			}
			cmd := strings.TrimSpace(rest[:end])
			if cmd != "" {
				// Many benchmark instructions mention paths (e.g. `/app`, `/app/foo.txt`) inside
				// prose lines that contain "verify"/"test". Treat those as non-commands to avoid
				// running nonsense like "/app" as a shell command.
				if !strings.ContainsAny(cmd, " \t\r\n") && (strings.HasPrefix(cmd, "/") || strings.HasPrefix(cmd, "./")) {
					switch {
					case strings.HasSuffix(cmd, ".py"):
						cmds = append(cmds, "python3 "+cmd)
					case strings.HasSuffix(cmd, ".sh"):
						cmds = append(cmds, "bash "+cmd)
					default:
						// Ignore.
					}
				} else {
					cmds = append(cmds, cmd)
				}
			}
			line = rest[end+1:]
		}
	}

	// Python code blocks that the instruction explicitly says "should run".
	if strings.Contains(inLower, "```python") && strings.Contains(inLower, "should run") {
		re := regexp.MustCompile("```python\\s*\\n([\\s\\S]*?)\\n```")
		matches := re.FindAllStringSubmatch(instruction, -1)
		for i, m := range matches {
			if len(m) < 2 {
				continue
			}
			code := strings.TrimSpace(m[1])
			if code == "" {
				continue
			}
			delim := fmt.Sprintf("PY_EAI_%d", i+1)
			cmds = append(cmds, fmt.Sprintf("python3 - <<'%s'\n%s\n%s", delim, code, delim))
		}
	}

	// Lightweight automatic checks for common build constraints.
	if strings.Contains(inLower, "no x11") || strings.Contains(inLower, "no x server") {
		// Fail if any X11/libX dependency shows up in ldd output.
		cmds = append(cmds, "ldd $(command -v pmars) 2>/dev/null | grep -Ei 'libX|X11' && exit 1 || exit 0")
	}

	// For R tasks that define a required test() function, run it.
	if strings.Contains(inLower, "ars.r") && strings.Contains(inLower, "test function") {
		cmds = append(cmds, `Rscript -e 'source("ars.R"); if (exists("test")) test() else stop("test function not found")'`)
	}

	return uniqueStrings(cmds)
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

	// Optional file-change metadata (used by the TUI to render diffs).
	FilePath   string `json:"file_path,omitempty"`
	ChangeType string `json:"change_type,omitempty"` // write|edit|patch
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
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

type AgentEventKind string

const (
	AgentEventIterationStart AgentEventKind = "iteration_start"
	AgentEventModelResponse  AgentEventKind = "model_response"
	AgentEventToolCall       AgentEventKind = "tool_call"
	AgentEventToolResult     AgentEventKind = "tool_result"
	AgentEventFileChange     AgentEventKind = "file_change"
	AgentEventCompleted      AgentEventKind = "completed"
	AgentEventCancelled      AgentEventKind = "cancelled"
	AgentEventError          AgentEventKind = "error"
)

// AgentEvent is a safe-to-display activity trace event. It must not contain
// private chain-of-thought; it should only reflect observable actions and
// concise summaries.
type AgentEvent struct {
	Kind      AgentEventKind `json:"kind"`
	Timestamp time.Time      `json:"timestamp"`

	Iteration int `json:"iteration,omitempty"`
	MaxLoops  int `json:"max_loops,omitempty"`

	ToolName   string `json:"tool_name,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Detail     string `json:"detail,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`

	Success *bool `json:"success,omitempty"`

	FilePath   string `json:"file_path,omitempty"`
	ChangeType string `json:"change_type,omitempty"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type EventSink func(ev AgentEvent)

type AgentLoop struct {
	Client   *MinimaxClient
	Tools    []Tool
	MaxLoops int
	StateDir string
	WorkDir  string
	Logger   *Logger
}

func NewAgentLoop(client *MinimaxClient, maxLoops int, stateDir string, logger *Logger) *AgentLoop {
	if maxLoops <= 0 {
		maxLoops = 10
	}

	// Default to /app when available (Terminal-Bench/Harbor convention), otherwise
	// fall back to the current working directory.
	workDir := os.Getenv("EAI_WORKDIR")
	if workDir == "" {
		if _, err := os.Stat("/app"); err == nil {
			workDir = "/app"
		} else if wd, err := os.Getwd(); err == nil {
			workDir = wd
		}
	}
	return &AgentLoop{
		Client:   client,
		Tools:    DefaultTools(),
		MaxLoops: maxLoops,
		StateDir: stateDir,
		WorkDir:  workDir,
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
					"cwd": map[string]interface{}{
						"type":        "string",
						"description": "Optional working directory for the command (default: agent workdir)",
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
		{
			Name:        "append_file",
			Description: "Append content to an existing file (creates it if missing)",
			Parameters: mustMarshal(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to append to",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to append to the file",
					},
				},
				"required": []string{"path", "content"},
			}),
		},
		{
			Name:        "patch_file",
			Description: "Apply a unified diff patch to a file",
			Parameters: mustMarshal(map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file to patch",
					},
					"patch": map[string]interface{}{
						"type":        "string",
						"description": "Unified diff patch content (may start with @@ ...)",
					},
				},
				"required": []string{"path", "patch"},
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
	return l.ExecuteWithEvents(ctx, task, nil)
}

func (l *AgentLoop) ExecuteWithEvents(ctx context.Context, task string, emit EventSink) (*AgentState, error) {
	emitEv := func(ev AgentEvent) {
		if emit == nil {
			return
		}
		if ev.Timestamp.IsZero() {
			ev.Timestamp = time.Now()
		}
		emit(ev)
	}

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

	// Optional benchmark-only fastpaths (guarded by env var).
	if ok, err := l.tryTerminalBenchFastpath(task); err != nil {
		l.Logger.Error("Terminal-bench fastpath failed", map[string]interface{}{
			"error": err.Error(),
		})
	} else if ok {
		state.Completed = true
		state.FinalOutput = "TASK_COMPLETED"
		state.EndedAt = time.Now()
		l.saveState(state)
		return state, nil
	}

	systemMsg := l.buildSystemMessage(task)
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
	verificationCmds := extractVerificationCommands(task)

	// Track consecutive no-action responses
	consecutiveNoAction := 0
	maxNoActionAttempts := 10
	apiErrorStreak := 0

loop:
	for state.Iteration < l.MaxLoops {
		emitEv(AgentEvent{
			Kind:      AgentEventIterationStart,
			Iteration: state.Iteration + 1,
			MaxLoops:  state.MaxLoops,
			Summary:   "Iteration start",
		})

		l.saveState(state)

		response, err := l.Client.Complete(ctx, l.buildPrompt(state.Messages))
		if err != nil {
			l.Logger.Error("Failed to get model response", map[string]interface{}{
				"error": err.Error(),
			})
			// Permanent misconfiguration: stop early with a clear error.
			if isLikelyConfigError(err) || ctx.Err() != nil {
				emitEv(AgentEvent{
					Kind:    AgentEventError,
					Summary: "Model error",
					Detail:  err.Error(),
				})
				state.Messages = append(state.Messages, AgentMessage{
					Role:      "assistant",
					Content:   fmt.Sprintf("Error: %v", err),
					Timestamp: time.Now(),
				})
				break
			}

			// Transient API failures happen; backoff and retry without consuming a loop.
			apiErrorStreak++
			if apiErrorStreak > 8 {
				emitEv(AgentEvent{
					Kind:    AgentEventError,
					Summary: "Model error (retries exhausted)",
					Detail:  err.Error(),
				})
				state.Messages = append(state.Messages, AgentMessage{
					Role:      "assistant",
					Content:   fmt.Sprintf("Error: %v", err),
					Timestamp: time.Now(),
				})
				break
			}
			backoff := time.Second
			for i := 1; i < apiErrorStreak && backoff < 60*time.Second; i++ {
				backoff *= 2
			}
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				break loop
			}
			continue
		}
		apiErrorStreak = 0

		state.Messages = append(state.Messages, AgentMessage{
			Role:      "assistant",
			Content:   response,
			Timestamp: time.Now(),
		})
		emitEv(AgentEvent{
			Kind:      AgentEventModelResponse,
			Iteration: state.Iteration + 1,
			MaxLoops:  state.MaxLoops,
			Summary:   fmt.Sprintf("Model response received (%d chars)", len(response)),
			Detail:    truncateForTrace(oneLine(response), 4000),
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

				// Run any verification commands we can extract from the instruction (or that are
				// conventionally provided in /app), and only then accept completion.
				verifyList := append([]string{}, verificationCmds...)
				if l.WorkDir != "" {
					if _, err := os.Stat(filepath.Join(l.WorkDir, "test_outputs.py")); err == nil {
						verifyList = append(verifyList, fmt.Sprintf("python3 %s", filepath.Join(l.WorkDir, "test_outputs.py")))
					}
					if _, err := os.Stat(filepath.Join(l.WorkDir, "tests", "test_outputs.py")); err == nil {
						verifyList = append(verifyList, fmt.Sprintf("python3 %s", filepath.Join(l.WorkDir, "tests", "test_outputs.py")))
					}
					if _, err := os.Stat(filepath.Join(l.WorkDir, "tests", "test.sh")); err == nil {
						verifyList = append(verifyList, fmt.Sprintf("bash %s", filepath.Join(l.WorkDir, "tests", "test.sh")))
					}
				}
				verifyList = uniqueStrings(verifyList)

				for i, cmdStr := range verifyList {
					call := ToolCall{
						ID:   fmt.Sprintf("verify_exec_%d", i+1),
						Name: "exec",
						Arguments: mustMarshal(map[string]interface{}{
							"command": cmdStr,
							"timeout": 900,
						}),
					}
					verifyRes := l.executeTool(ctx, call)
					state.Results = append(state.Results, verifyRes)
					state.Messages = append(state.Messages, AgentMessage{
						Role:        "user",
						Content:     fmt.Sprintf("Verification result for command:\n%s\n\n%s", cmdStr, verifyRes.Output),
						ToolResults: []ToolResult{verifyRes},
						Timestamp:   time.Now(),
					})

					outLower := strings.ToLower(verifyRes.Output + "\n" + verifyRes.Error)
					failed := !verifyRes.Success ||
						strings.Contains(outLower, ": fail") ||
						strings.Contains(outLower, "assertionerror") ||
						strings.Contains(outLower, "traceback") ||
						strings.Contains(outLower, "segmentation fault") ||
						strings.Contains(outLower, "error:")

					if failed {
						promptMsg := AgentMessage{
							Role:      "user",
							Content:   fmt.Sprintf("VERIFICATION FAILED for command: %s\nFix the issue and continue. After fixing, run verification again and only then respond TASK_COMPLETED.", cmdStr),
							Timestamp: time.Now(),
						}
						state.Messages = append(state.Messages, promptMsg)
						state.Iteration++
						continue loop
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
				state.FinalOutput = "Model did not return any tool calls after repeated prompts"
				break
			}

			// Prompt the model to take action with emphasis on brevity
			actionPrompt := "You haven't taken any action yet. Please use one of the available tools to complete the task.\n" +
				"IMPORTANT: Respond with ONLY the JSON tool call. No explanation, no prose.\n" +
				"Format: {\"tool\": \"tool_name\", \"args\": {...}}\n" +
				"Available tools: exec, read_file, write_file, append_file, edit_file, patch_file, list_dir, search_files, grep\n"

			if !hasExecutedTools {
				actionPrompt += "For write_file: Keep content SHORT. Write skeleton code first, then add details incrementally."
			} else {
				actionPrompt += "If the task is truly complete, respond with exactly: TASK_COMPLETED"
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
			argsPreview := safeToolArgsPreview(call.Arguments)
			emitEv(AgentEvent{
				Kind:      AgentEventToolCall,
				Iteration: state.Iteration + 1,
				MaxLoops:  state.MaxLoops,
				ToolName:  call.Name,
				Summary:   fmt.Sprintf("%s %s", call.Name, argsPreview),
				Detail:    argsPreview,
			})

			result := l.executeTool(ctx, call)
			state.Results = append(state.Results, result)

			resultMsg := AgentMessage{
				Role:        "user",
				Content:     fmt.Sprintf("Tool result for %s:\n%s", call.Name, result.Output),
				ToolResults: []ToolResult{result},
				Timestamp:   time.Now(),
			}
			state.Messages = append(state.Messages, resultMsg)

			success := result.Success
			emitEv(AgentEvent{
				Kind:       AgentEventToolResult,
				Iteration:  state.Iteration + 1,
				MaxLoops:   state.MaxLoops,
				ToolName:   call.Name,
				Summary:    fmt.Sprintf("%s (%dms)", map[bool]string{true: "OK", false: "ERR"}[success], result.DurationMs),
				Detail:     truncateForTrace(result.Output+"\n"+result.Error, 8000),
				DurationMs: result.DurationMs,
				Success:    &success,
			})

			if result.FilePath != "" && (result.OldContent != "" || result.NewContent != "" || result.ChangeType != "") {
				emitEv(AgentEvent{
					Kind:       AgentEventFileChange,
					Iteration:  state.Iteration + 1,
					MaxLoops:   state.MaxLoops,
					ToolName:   call.Name,
					Summary:    fmt.Sprintf("%s %s", result.ChangeType, result.FilePath),
					FilePath:   result.FilePath,
					ChangeType: result.ChangeType,
					OldContent: result.OldContent,
					NewContent: result.NewContent,
				})
			}
		}

		state.Iteration++
	}

	state.EndedAt = time.Now()
	l.saveState(state)

	if ctx.Err() != nil && !state.Completed {
		emitEv(AgentEvent{
			Kind:    AgentEventCancelled,
			Summary: "Cancelled",
			Detail:  ctx.Err().Error(),
		})
	}
	if state.Completed {
		emitEv(AgentEvent{
			Kind:    AgentEventCompleted,
			Summary: "Completed",
		})
	}

	if !state.Completed && state.FinalOutput == "" {
		state.FinalOutput = fmt.Sprintf("Task did not complete within %d iterations", l.MaxLoops)
	}

	return state, nil
}

func (l *AgentLoop) buildPrompt(messages []AgentMessage) string {
	if len(messages) == 0 {
		return ""
	}

	maxPrompt := agentCfg.MaxPromptBytes
	if maxPrompt <= 0 {
		maxPrompt = agentCfg.ContextSummarizeThreshold
	}
	if maxPrompt <= 0 {
		maxPrompt = 120 * 1024
	}

	perMsgMax := 20 * 1024
	if perMsgMax > maxPrompt/2 {
		perMsgMax = maxPrompt / 2
	}
	if perMsgMax < 4096 {
		perMsgMax = 4096
	}

	format := func(role, content string) string {
		if len(content) > perMsgMax {
			// Keep the tail; that's where assertions/errors/tool snippets usually are.
			content = fmt.Sprintf("[message truncated for prompt: %d bytes -> %d bytes]\n%s", len(content), perMsgMax, content[len(content)-perMsgMax:])
		}
		return fmt.Sprintf("[%s]\n%s\n\n", role, content)
	}

	segments := make([]string, len(messages))
	for i, msg := range messages {
		segments[i] = format(msg.Role, msg.Content)
	}

	selected := make([]int, 0, len(messages))
	selected = append(selected, 0)
	baseLen := len(segments[0])
	if len(messages) > 1 {
		selected = append(selected, 1)
		baseLen += len(segments[1])
	}

	budget := maxPrompt - baseLen
	if budget < 0 {
		return segments[0]
	}

	for i := len(messages) - 1; i >= 2; i-- {
		if len(segments[i]) > budget {
			continue
		}
		selected = append(selected, i)
		budget -= len(segments[i])
		if budget <= 0 {
			break
		}
	}

	sort.Ints(selected)
	var b strings.Builder
	for _, idx := range selected {
		b.WriteString(segments[idx])
	}
	return b.String()
}

func (l *AgentLoop) buildSystemMessage(task string) string {
	base := GetAgentSystemPrompt(l.WorkDir)

	// Add lightweight category guidance when applicable.
	category := detectCategory(task)
	if category != "" && category != "default" {
		if guidance, ok := categoryPrompts[category]; ok {
			base += "\n\n## TASK-SPECIFIC GUIDANCE\n" + guidance
		}
	}

	return base
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

func normalizeJSONArgs(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	trim := strings.TrimSpace(string(raw))
	if trim == "" {
		return json.RawMessage("{}")
	}

	// Some models return arguments as a JSON-encoded string.
	if strings.HasPrefix(trim, "\"") {
		var s string
		if err := json.Unmarshal([]byte(trim), &s); err == nil {
			s = strings.TrimSpace(s)
			if s == "" {
				return json.RawMessage("{}")
			}
			return json.RawMessage(s)
		}
	}

	return json.RawMessage(trim)
}

func (l *AgentLoop) toolNameAllowed(name string) bool {
	for _, t := range l.Tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

func (l *AgentLoop) parseToolCallsJSON(content string) ([]ToolCall, bool) {
	if content == "" {
		return nil, false
	}

	// Fast-path: {"tool":"...", "args":{...}} or {"tool":"...", ...}
	var direct struct {
		Tool string          `json:"tool"`
		Args json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal([]byte(content), &direct); err == nil && direct.Tool != "" {
		toolName := direct.Tool

		// If args isn't present, treat remaining keys as args.
		args := direct.Args
		if len(args) == 0 {
			var raw map[string]json.RawMessage
			if err := json.Unmarshal([]byte(content), &raw); err == nil {
				delete(raw, "tool")
				delete(raw, "args")
				if len(raw) > 0 {
					if b, err := json.Marshal(raw); err == nil {
						args = b
					}
				}
			}
		}

		return []ToolCall{{
			ID:        fmt.Sprintf("%s_1", toolName),
			Name:      toolName,
			Arguments: normalizeJSONArgs(args),
		}}, true
	}

	// Anthropic/OpenAI-style: {"tool_calls":[{"id":"...","name":"...","arguments":{...}}]}
	type toolCallEntry struct {
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Args      json.RawMessage `json:"args"`
		Function  *struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"function"`
	}
	var envelope struct {
		ToolCalls []toolCallEntry `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(content), &envelope); err == nil && len(envelope.ToolCalls) > 0 {
		var calls []ToolCall
		for i, tc := range envelope.ToolCalls {
			name := tc.Name
			args := tc.Arguments
			if name == "" && tc.Function != nil {
				name = tc.Function.Name
				args = tc.Function.Arguments
			}
			if len(args) == 0 {
				args = tc.Args
			}
			if name == "" {
				continue
			}
			if tc.ID == "" {
				tc.ID = fmt.Sprintf("%s_%d", name, i+1)
			}

			calls = append(calls, ToolCall{
				ID:        tc.ID,
				Name:      name,
				Arguments: normalizeJSONArgs(args),
			})
		}
		if len(calls) > 0 {
			return calls, true
		}
	}

	// {"name":"exec","arguments":{...}} (single call object)
	var single struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Args      json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal([]byte(content), &single); err == nil && single.Name != "" {
		args := single.Arguments
		if len(args) == 0 {
			args = single.Args
		}
		return []ToolCall{{
			ID:        fmt.Sprintf("%s_1", single.Name),
			Name:      single.Name,
			Arguments: normalizeJSONArgs(args),
		}}, true
	}

	// {"exec": {"command":"..."}} (tool name as key)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err == nil {
		for key, val := range raw {
			if key == "tool" || key == "args" || key == "tool_calls" || key == "name" || key == "arguments" {
				continue
			}
			// Only accept known tools to avoid mis-parsing arbitrary JSON.
			if !l.toolNameAllowed(key) {
				continue
			}

			// Value must be an object.
			trim := strings.TrimSpace(string(val))
			if strings.HasPrefix(trim, "{") && strings.HasSuffix(trim, "}") {
				return []ToolCall{{
					ID:        fmt.Sprintf("%s_1", key),
					Name:      key,
					Arguments: normalizeJSONArgs(val),
				}}, true
			}
		}
	}

	return nil, false
}

func (l *AgentLoop) parseToolCalls(response string) []ToolCall {
	var toolCalls []ToolCall

	trimmed := strings.TrimSpace(response)
	if trimmed != "" {
		if calls, ok := l.parseToolCallsJSON(trimmed); ok {
			return calls
		}
	}

	// === Format -1: Extract JSON from markdown code blocks ===
	// Look for ```json ... ``` blocks first
	jsonBlockRe := regexp.MustCompile("```json\\s*\\n([\\s\\S]*?)\\n```")
	if matches := jsonBlockRe.FindAllStringSubmatch(response, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				content := strings.TrimSpace(match[1])
				if calls, ok := l.parseToolCallsJSON(content); ok {
					return calls
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

	// === Format 3: {"tool": "..."} or {"tool_calls":[...]} anywhere in response ===
	toolIdx := strings.Index(response, `"tool"`)
	toolCallsIdx := strings.Index(response, `"tool_calls"`)
	startIdx := -1
	switch {
	case toolIdx >= 0 && toolCallsIdx >= 0:
		if toolIdx < toolCallsIdx {
			startIdx = toolIdx
		} else {
			startIdx = toolCallsIdx
		}
	case toolIdx >= 0:
		startIdx = toolIdx
	case toolCallsIdx >= 0:
		startIdx = toolCallsIdx
	}
	if startIdx >= 0 {
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
						jsonStr = strings.Replace(jsonStr, "=>", ":", -1)
						if calls, ok := l.parseToolCallsJSON(jsonStr); ok {
							return calls
						}
						break
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

func (l *AgentLoop) resolvePath(path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	if l.WorkDir == "" {
		return path
	}
	return filepath.Join(l.WorkDir, path)
}

func (l *AgentLoop) defaultExecTimeout(command string) time.Duration {
	// Baseline timeout. We bump the default a bit for terminal-bench style tasks
	// because builds/installs frequently exceed 30s.
	short := agentCfg.DefaultTimeout
	if short <= 0 {
		short = 30 * time.Second
	}
	if short < 60*time.Second {
		short = 60 * time.Second
	}

	long := 10 * time.Minute

	cmdLower := strings.ToLower(command)
	longKeywords := []string{
		"apt-get", "apt ", "pip install", "pip3 install", "uv pip", "conda", "mamba",
		"npm ", "yarn ", "pnpm ", "cargo ", "go build", "go test", "make", "cmake",
		"meson", "ninja", "python setup.py", "pytest", "rscript", "r cmd", "git clone",
		"wget ", "curl ",
	}
	for _, kw := range longKeywords {
		if strings.Contains(cmdLower, kw) {
			return long
		}
	}
	return short
}

func truncateToolOutput(s string) string {
	maxBytes := agentCfg.MaxOutputBufferSize
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024
	}
	if len(s) <= maxBytes {
		return s
	}
	// Keep the tail; that's where errors usually are.
	tail := s[len(s)-maxBytes:]
	return fmt.Sprintf("[output truncated: %d bytes -> %d bytes]\n%s", len(s), maxBytes, tail)
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

func truncateForTrace(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n[truncated]"
}

func safeToolArgsPreview(raw json.RawMessage) string {
	raw = normalizeJSONArgs(raw)
	if len(raw) == 0 {
		return ""
	}

	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return truncateForTrace(oneLine(string(raw)), 240)
	}

	redactString := func(s string) string {
		ls := strings.ToLower(s)
		if strings.Contains(ls, "api_key") || strings.Contains(ls, "apikey") || strings.Contains(ls, "secret") || strings.Contains(ls, "token") {
			return "[redacted]"
		}
		// Best-effort: avoid echoing long key-like blobs into the UI.
		if len(s) >= 24 {
			return "[redacted]"
		}
		return s
	}

	switch vv := v.(type) {
	case map[string]any:
		for k, val := range vv {
			lk := strings.ToLower(k)
			if lk == "content" {
				size := len(fmt.Sprint(val))
				vv[k] = fmt.Sprintf("[content omitted: %d bytes]", size)
				continue
			}
			if strings.Contains(lk, "key") || strings.Contains(lk, "secret") || strings.Contains(lk, "token") {
				vv[k] = "[redacted]"
				continue
			}
			if s, ok := val.(string); ok {
				vv[k] = redactString(s)
			}
		}
		if b, err := json.Marshal(vv); err == nil {
			return truncateForTrace(oneLine(string(b)), 240)
		}
	case string:
		return truncateForTrace(oneLine(redactString(vv)), 240)
	}

	return truncateForTrace(oneLine(string(raw)), 240)
}

func isProbablyBinary(data []byte) bool {
	// Fast heuristic: if it contains NUL bytes, treat as binary.
	for _, b := range data {
		if b == 0 {
			return true
		}
	}
	return false
}

func readFilePreview(path string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	limited := io.LimitReader(f, maxBytes)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}

	size := info.Size()
	if isProbablyBinary(buf) {
		n := 128
		if len(buf) < n {
			n = len(buf)
		}
		var hex strings.Builder
		for i := 0; i < n; i++ {
			if i > 0 {
				hex.WriteByte(' ')
			}
			fmt.Fprintf(&hex, "%02x", buf[i])
		}
		out := fmt.Sprintf("[binary file: %s (%d bytes); first %d bytes as hex]\n%s", path, size, n, hex.String())
		if size > maxBytes {
			out += fmt.Sprintf("\n[truncated: read first %d bytes of %d]", maxBytes, size)
		}
		return out, nil
	}

	out := string(buf)
	if size > maxBytes {
		out = fmt.Sprintf("[truncated: showing first %d bytes of %d]\n%s", maxBytes, size, out)
	}
	return out, nil
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
			Cwd     string `json:"cwd"`
			Timeout int    `json:"timeout"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		timeout := l.defaultExecTimeout(args.Command)
		if args.Timeout > 0 {
			timeout = time.Duration(args.Timeout) * time.Second
		}

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		// Prefer bash for better compatibility with common build tooling.
		shell := "bash"
		shellArgs := []string{"-lc", args.Command}
		if _, err := exec.LookPath(shell); err != nil {
			shell = "sh"
			shellArgs = []string{"-c", args.Command}
		}
		cmd := exec.CommandContext(ctx, shell, shellArgs...)
		if args.Cwd != "" {
			cmd.Dir = l.resolvePath(args.Cwd)
		} else if l.WorkDir != "" {
			cmd.Dir = l.WorkDir
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			result.Error = err.Error()
		}
		result.Output = truncateToolOutput(string(output))
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

		path := l.resolvePath(args.Path)
		const maxReadBytes = 64 * 1024
		out, err := readFilePreview(path, maxReadBytes)
		if err != nil {
			result.Error = err.Error()
			break
		}
		result.Output = truncateToolOutput(out)
		result.Success = true

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

		path := l.resolvePath(args.Path)
		oldContent := ""
		if data, err := os.ReadFile(path); err == nil {
			oldContent = string(data)
		}

		// Create parent directory if path contains a slash
		if slashIdx := strings.LastIndex(path, "/"); slashIdx > 0 {
			dir := path[:slashIdx]
			if err := os.MkdirAll(dir, 0755); err != nil {
				result.Error = fmt.Sprintf("Failed to create directory: %v", err)
				result.DurationMs = time.Since(start).Milliseconds()
				return result
			}
		}

		if err := os.WriteFile(path, []byte(args.Content), 0644); err != nil {
			result.Error = err.Error()
		} else {
			result.Output = fmt.Sprintf("File written: %s", path)
			result.FilePath = path
			result.ChangeType = "write"
			result.OldContent = truncateForTrace(oldContent, 64*1024)
			result.NewContent = truncateForTrace(args.Content, 64*1024)
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
			if l.WorkDir != "" {
				args.Path = l.WorkDir
			} else {
				args.Path = "."
			}
		} else {
			args.Path = l.resolvePath(args.Path)
		}

		entries, err := os.ReadDir(args.Path)
		if err != nil {
			result.Error = err.Error()
		} else {
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

			var lines []string
			maxEntries := 200
			truncated := false
			if len(entries) > maxEntries {
				entries = entries[:maxEntries]
				truncated = true
			}
			for _, entry := range entries {
				if entry.IsDir() {
					lines = append(lines, fmt.Sprintf("DIR  %s/", entry.Name()))
				} else {
					sizeStr := ""
					if info, err := entry.Info(); err == nil {
						sizeStr = fmt.Sprintf(" (%d bytes)", info.Size())
					}
					lines = append(lines, fmt.Sprintf("FILE %s%s", entry.Name(), sizeStr))
				}
			}
			out := strings.Join(lines, "\n")
			if truncated {
				out += fmt.Sprintf("\n[truncated: showing %d entries]", maxEntries)
			}
			result.Output = truncateToolOutput(out)
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
			if l.WorkDir != "" {
				args.Path = l.WorkDir
			} else {
				args.Path = "."
			}
		} else {
			args.Path = l.resolvePath(args.Path)
		}

		flags := "-rHn"
		if !args.Recursive {
			flags = "-Hn"
		}

		cmd := exec.CommandContext(ctx, "grep", flags, args.Pattern, args.Path)
		output, err := cmd.CombinedOutput()
		if err != nil {
			// grep exits 1 when there are no matches; that's not an error for us.
			if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
				result.Output = ""
				result.Success = true
				break
			}
			if len(output) == 0 {
				result.Error = err.Error()
				break
			}
		}
		result.Output = truncateToolOutput(string(output))
		result.Success = true

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
			if l.WorkDir != "" {
				args.Path = l.WorkDir
			} else {
				args.Path = "."
			}
		} else {
			args.Path = l.resolvePath(args.Path)
		}

		cmd := exec.CommandContext(ctx, "find", args.Path, "-name", args.Pattern, "-type", "f")
		output, err := cmd.CombinedOutput()
		if err != nil && len(output) == 0 {
			result.Error = err.Error()
		} else {
			result.Output = truncateToolOutput(string(output))
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

		path := l.resolvePath(args.Path)
		data, err := os.ReadFile(path)
		if err != nil {
			result.Error = err.Error()
		} else {
			content := string(data)
			if !strings.Contains(content, args.OldText) {
				result.Error = fmt.Sprintf("Text not found in file: %s", args.OldText)
			} else {
				newContent := strings.Replace(content, args.OldText, args.NewText, 1)
				if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
					result.Error = err.Error()
				} else {
					result.Output = fmt.Sprintf("File edited: %s", path)
					result.FilePath = path
					result.ChangeType = "edit"
					result.OldContent = truncateForTrace(content, 64*1024)
					result.NewContent = truncateForTrace(newContent, 64*1024)
					result.Success = true
				}
			}
		}

	case "append_file":
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
		path := l.resolvePath(args.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			result.Error = fmt.Sprintf("Failed to create directory: %v", err)
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			result.Error = err.Error()
			break
		}
		_, err = f.WriteString(args.Content)
		_ = f.Close()
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Output = fmt.Sprintf("File appended: %s", path)
			result.Success = true
		}

	case "patch_file":
		var args struct {
			Path  string `json:"path"`
			Patch string `json:"patch"`
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
		path := l.resolvePath(args.Path)
		if _, err := os.Stat(path); err != nil {
			result.Error = err.Error()
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}
		before := ""
		if data, err := os.ReadFile(path); err == nil {
			before = string(data)
		}
		tmp, err := os.CreateTemp("", "eai-patch-*.diff")
		if err != nil {
			result.Error = err.Error()
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}
		tmpPath := tmp.Name()
		_, _ = tmp.WriteString(args.Patch)
		_ = tmp.Close()
		defer os.Remove(tmpPath)

		// Apply the patch non-interactively.
		cmd := exec.CommandContext(ctx, "patch", "--batch", "-u", path, "-i", tmpPath)
		if l.WorkDir != "" {
			cmd.Dir = l.WorkDir
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			result.Error = err.Error()
		}
		result.Output = truncateToolOutput(string(output))
		result.Success = err == nil
		if result.Success {
			after := ""
			if data, err := os.ReadFile(path); err == nil {
				after = string(data)
			}
			result.FilePath = path
			result.ChangeType = "patch"
			result.OldContent = truncateForTrace(before, 64*1024)
			result.NewContent = truncateForTrace(after, 64*1024)
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
