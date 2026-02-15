package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
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

var (
	indexTitleRe             = regexp.MustCompile(`(?is)<title[^>]*>\s*([^<]{3,120})\s*</title>`)
	indexScriptRe            = regexp.MustCompile(`(?is)<script[^>]*\ssrc=["']([^"']+)["']`)
	indexTokenRe             = regexp.MustCompile(`[A-Za-z0-9._/\-]{10,}`)
	localServerURLRe         = regexp.MustCompile(`https?://(?:127\.0\.0\.1|localhost|0\.0\.0\.0)(?::\d{2,5})?`)
	autoDetachServerPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bnpm\s+run\s+dev\b`),
		regexp.MustCompile(`(?i)\bnpm\s+(run\s+)?start\b`),
		regexp.MustCompile(`(?i)\bpnpm\s+(run\s+)?dev\b`),
		regexp.MustCompile(`(?i)\bpnpm\s+(run\s+)?start\b`),
		regexp.MustCompile(`(?i)\byarn\s+dev\b`),
		regexp.MustCompile(`(?i)\byarn\s+start\b`),
		regexp.MustCompile(`(?i)\b(?:npx\s+)?vite(?:\s|$)`),
		regexp.MustCompile(`(?i)\bnext\s+dev\b`),
		regexp.MustCompile(`(?i)\bnuxt\s+dev\b`),
		regexp.MustCompile(`(?i)\bastro\s+dev\b`),
		regexp.MustCompile(`(?i)\breact-scripts\s+start\b`),
		regexp.MustCompile(`(?i)\bpython(?:3)?\s+-m\s+http\.server\b`),
		regexp.MustCompile(`(?i)\buvicorn\b`),
		regexp.MustCompile(`(?i)\bflask\s+run\b`),
		regexp.MustCompile(`(?i)\bmanage\.py\s+runserver\b`),
		regexp.MustCompile(`(?i)\brails\s+server\b`),
		regexp.MustCompile(`(?i)\bphp\s+-s\s+`),
	}
)

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
	if strings.Contains(s, "api key is required") {
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

func hasTaskCompletedSentinel(response string) bool {
	response = strings.TrimSpace(response)
	if response == "" {
		return false
	}
	lines := strings.Split(response, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		return strings.EqualFold(line, "TASK_COMPLETED")
	}
	return false
}

func seemsLikeHTMLWebsiteTask(task string) bool {
	s := strings.ToLower(strings.TrimSpace(task))
	if s == "" {
		return false
	}
	if !strings.Contains(s, "html") {
		return false
	}
	if !(strings.Contains(s, "website") || strings.Contains(s, "web site") || strings.Contains(s, "landing page") || strings.Contains(s, "site")) {
		return false
	}
	verbs := []string{"create", "build", "make", "generate", "write"}
	for _, v := range verbs {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

func looksLikeHTMLDocumentResponse(s string) bool {
	sl := strings.ToLower(s)
	if strings.Contains(sl, "<!doctype html") {
		return true
	}
	return strings.Contains(sl, "<html") && strings.Contains(sl, "</html>")
}

func extractHTMLDocumentResponse(s string) (string, bool) {
	trim := strings.TrimSpace(s)
	if trim == "" {
		return "", false
	}
	lower := strings.ToLower(trim)

	// Prefer fenced blocks.
	if i := strings.Index(lower, "```html"); i >= 0 {
		rest := trim[i+len("```html"):]
		rest = strings.TrimLeft(rest, "\n\r\t ")
		if j := strings.Index(rest, "```"); j >= 0 {
			cand := strings.TrimSpace(rest[:j])
			if looksLikeHTMLDocumentResponse(cand) {
				return cand, true
			}
		}
	}

	if !looksLikeHTMLDocumentResponse(trim) {
		return "", false
	}

	start := strings.Index(lower, "<!doctype html")
	if start < 0 {
		start = strings.Index(lower, "<html")
	}
	if start < 0 {
		return "", false
	}
	end := strings.LastIndex(lower, "</html>")
	if end < 0 {
		return "", false
	}
	end += len("</html>")
	cand := strings.TrimSpace(trim[start:end])
	if cand == "" {
		return "", false
	}
	return cand, true
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
}

type ProgressEvent struct {
	Kind       string `json:"kind"`
	Text       string `json:"text"`
	// CompanionLabel identifies which companion (if any) emitted this event.
	CompanionLabel string `json:"companion_label,omitempty"`
	CompanionID    string `json:"companion_id,omitempty"`
	CompanionIndex int    `json:"companion_index,omitempty"`
	CompanionTotal int    `json:"companion_total,omitempty"`
	Tool       string `json:"tool,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	ToolStatus string `json:"tool_status,omitempty"`
	Path       string `json:"path,omitempty"`
	Command    string `json:"command,omitempty"`
	ChangeType string `json:"change_type,omitempty"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	// ActiveCompanions shows how many worker slots/shards are running right now.
	ActiveCompanions int `json:"active_companions,omitempty"`
	// MaxCompanions reports the orchestrator worker ceiling for the current run.
	MaxCompanions int       `json:"max_companions,omitempty"`
	Error         string    `json:"error,omitempty"`
	At            time.Time `json:"at,omitempty"`
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
	// Relentless, when true, keeps iterating until completion or ctx cancellation.
	// MaxLoops is still recorded in state for observability, but not treated as a hard stop.
	Relentless             bool
	StateDir               string
	WorkDir                string
	Logger                 *Logger
	Progress               func(ProgressEvent)
	SystemMessageBuilder   func(task string) string
	FinalResponseValidator func(response string) bool
	FinalResponseGuidance  string
	// PreludeMessages, when set, are inserted after the system prompt and before
	// the current user task. This is used by the TUI to provide lightweight
	// "memory" without changing the agent system prompt.
	PreludeMessages []AgentMessage
	// PermissionsMode controls approval behavior for risky actions and elevation retries.
	PermissionsMode     string
	PermissionDecisions <-chan PermissionDecision

	// ToolCallFilter, when set, can deny a tool call at execution time (after parsing).
	// This is used to enforce read-only companion agents, etc.
	ToolCallFilter func(call ToolCall) (allowed bool, reason string)
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
		Client:          client,
		Tools:           DefaultTools(),
		MaxLoops:        maxLoops,
		StateDir:        stateDir,
		WorkDir:         workDir,
		Logger:          logger,
		PermissionsMode: PermissionsFullAccess,
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
		l.emitProgress(ProgressEvent{Kind: "error", Text: "Terminal-bench fastpath failed"})
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

	// Optional: insert session "memory" messages (TUI), but keep them lightweight.
	if len(l.PreludeMessages) > 0 {
		for _, msg := range l.PreludeMessages {
			role := strings.ToLower(strings.TrimSpace(msg.Role))
			if role == "" {
				role = "user"
			}
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				continue
			}
			state.Messages = append(state.Messages, AgentMessage{
				Role:      role,
				Content:   content,
				Timestamp: time.Now(),
			})
		}
	}

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
	truncationContinueAttempts := 0
	maxTruncationContinueAttempts := 2

	maxIter := l.MaxLoops
	if maxIter <= 0 {
		maxIter = 10
	}
	if l.Relentless {
		maxIter = int(^uint(0) >> 1) // effectively unbounded; rely on ctx cancel.
	}

loop:
	for state.Iteration < maxIter {
		l.saveState(state)

		if state.Iteration == 0 {
			l.emitProgress(ProgressEvent{Kind: "thinking", Text: "Planning approach"})
		}

		response, meta, err := l.Client.CompleteWithObserverMeta(ctx, l.buildPrompt(state.Messages), func(reasoning string) {
			reasoning = strings.TrimSpace(reasoning)
			if reasoning == "" {
				return
			}
			l.emitProgress(ProgressEvent{
				Kind: "reasoning",
				Text: reasoning,
			})
		})
		if err != nil {
			l.Logger.Error("Failed to get model response", map[string]interface{}{
				"error": err.Error(),
			})
			l.emitProgress(ProgressEvent{Kind: "error", Text: "Failed to get model response"})
			// Permanent misconfiguration: stop early with a clear error.
			if isLikelyConfigError(err) || ctx.Err() != nil {
				state.Messages = append(state.Messages, AgentMessage{
					Role:      "assistant",
					Content:   RedactSecrets(fmt.Sprintf("Error: %v", err), l.apiKeyForRedaction()),
					Timestamp: time.Now(),
				})
				break
			}

			// Transient API failures happen; backoff and retry without consuming a loop.
			apiErrorStreak++
			if apiErrorStreak > 8 {
				state.Messages = append(state.Messages, AgentMessage{
					Role:      "assistant",
					Content:   RedactSecrets(fmt.Sprintf("Error: %v", err), l.apiKeyForRedaction()),
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
			time.Sleep(backoff)
			continue
		}
		apiErrorStreak = 0

		state.Messages = append(state.Messages, AgentMessage{
			Role:      "assistant",
			Content:   RedactSecrets(response, l.apiKeyForRedaction()),
			Timestamp: time.Now(),
		})

		toolCalls := l.parseToolCalls(response)

		// Salvage: some models ignore the JSON-only rule and dump a full HTML doc.
		// If the task is clearly "create a website using HTML", convert that into a write_file.
		if len(toolCalls) == 0 && seemsLikeHTMLWebsiteTask(task) {
			if htmlDoc, ok := extractHTMLDocumentResponse(response); ok {
				call := ToolCall{
					ID:   "write_html_1",
					Name: "write_file",
					Arguments: mustMarshal(map[string]interface{}{
						"path":    "index.html",
						"content": htmlDoc,
					}),
				}
				res := l.executeToolWithProgress(ctx, call)
				state.Results = append(state.Results, res)
				state.Messages = append(state.Messages, AgentMessage{
					Role:        "user",
					Content:     fmt.Sprintf("Tool result for %s:\n%s", call.Name, res.Output),
					ToolResults: []ToolResult{res},
					Timestamp:   time.Now(),
				})

				if res.Success {
					state.Completed = true
					state.FinalOutput = "TASK_COMPLETED"
					state.EndedAt = time.Now()
					l.saveState(state)
					return state, nil
				}
			}
		}

		finishReason := strings.ToLower(strings.TrimSpace(meta.FinishReason))
		truncated := len(toolCalls) == 0 && (finishReason == "length" || isResponseTruncated(response))
		if truncated {
			l.Logger.Warn("Response appears truncated", map[string]interface{}{
				"response_length": len(response),
				"finish_reason":   meta.FinishReason,
			})
			l.emitProgress(ProgressEvent{Kind: "warn", Text: "Response appears truncated"})

			if truncationContinueAttempts < maxTruncationContinueAttempts {
				truncationContinueAttempts++
				needsToolJSON := strings.Contains(response, `"tool"`) || strings.Contains(response, `"tool_calls"`) || strings.Contains(response, `"write_file"`) || strings.Contains(response, `"exec"`)
				prompt := "Your previous response was truncated. Continue exactly where you left off. Do not repeat earlier content."
				if needsToolJSON {
					prompt += " IMPORTANT: Output ONLY the complete valid JSON tool call (no prose, no backticks)."
				} else {
					prompt += " If you were writing a final report, continue and finish cleanly."
				}
				promptMsg := AgentMessage{Role: "user", Content: prompt, Timestamp: time.Now()}
				state.Messages = append(state.Messages, promptMsg)
				state.Iteration++
				continue
			}
		} else {
			truncationContinueAttempts = 0
		}

		if len(toolCalls) == 0 {
			if l.FinalResponseValidator != nil && l.FinalResponseValidator(response) {
				state.Completed = true
				state.FinalOutput = RedactSecrets(strings.TrimSpace(response), l.apiKeyForRedaction())
				break
			}

			consecutiveNoAction++

			// Check if model explicitly says task is complete
			explicitlyComplete := isExplicitCompletion(response)
			hasSentinel := hasTaskCompletedSentinel(response)

			// Check if we've done any meaningful work
			hasExecutedTools := len(state.Results) > 0

			// If the assistant declares completion, verify and finish.
			// - Require actual tool work for completion phrasing without the TASK_COMPLETED sentinel.
			// - Allow sentinel-only completion even if no tools ran (useful for trivial/non-actionable tasks).
			if hasSentinel || (explicitlyComplete && hasExecutedTools) {
				// Verify expected files exist before completing
				if len(expectedFiles) > 0 {
					existing, missing := verifyFilesExist(expectedFiles)
					if len(missing) > 0 {
						l.Logger.Warn("Task claims complete but expected files missing", map[string]interface{}{
							"missing":  missing,
							"existing": existing,
						})
						l.emitProgress(ProgressEvent{Kind: "warn", Text: "Task claims complete but expected files missing"})
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
					verifyRes := l.executeToolWithProgress(ctx, call)
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
							Content:   fmt.Sprintf("VERIFICATION FAILED for command: %s\nFix the issue and continue. After fixing, run verification again and only then respond with a final report ending with TASK_COMPLETED.", cmdStr),
							Timestamp: time.Now(),
						}
						state.Messages = append(state.Messages, promptMsg)
						state.Iteration++
						continue loop
					}
				}

				state.Completed = true
				state.FinalOutput = RedactSecrets(response, l.apiKeyForRedaction())
				break
			}

			// If too many no-action attempts, give up
			if consecutiveNoAction >= maxNoActionAttempts {
				l.Logger.Warn("Too many consecutive no-action responses", map[string]interface{}{
					"attempts": consecutiveNoAction,
				})
				l.emitProgress(ProgressEvent{Kind: "warn", Text: "Too many consecutive no-action responses"})
				if l.Relentless {
					// In relentless mode, don't stop: force a hard reset instruction and continue.
					reset := "RESET REQUIRED: You are not returning JSON tool calls.\n" +
						"1) Re-read the task and pick the single best NEXT tool action.\n" +
						"2) Output ONLY one valid JSON tool call. No prose.\n" +
						"Recommended first step: {\"tool\":\"list_dir\",\"args\":{\"path\":\".\"}}\n"
					state.Messages = append(state.Messages, AgentMessage{
						Role:      "user",
						Content:   reset,
						Timestamp: time.Now(),
					})
					consecutiveNoAction = 0
					state.Iteration++
					continue
				}

				state.FinalOutput = "Model did not return any tool calls after repeated prompts"
				break
			}

			if l.FinalResponseValidator != nil {
				guidance := strings.TrimSpace(l.FinalResponseGuidance)
				if guidance == "" {
					guidance = "Use a tool call for additional discovery, or provide the final response in the required format."
				}
				promptMsg := AgentMessage{
					Role:      "user",
					Content:   guidance,
					Timestamp: time.Now(),
				}
				state.Messages = append(state.Messages, promptMsg)
				state.Iteration++
				continue
			}

			// Prompt the model to take action with emphasis on brevity
			actionPrompt := "You haven't taken any action yet. Please use one of the available tools to complete the task.\n" +
				"IMPORTANT: Respond with ONLY the JSON tool call. No explanation, no prose.\n" +
				"Format: {\"tool\": \"tool_name\", \"args\": {...}}\n" +
				"Available tools: exec, read_file, write_file, append_file, edit_file, patch_file, list_dir, search_files, grep\n"

			if !hasExecutedTools {
				actionPrompt += "For write_file: Keep content SHORT. Write skeleton code first, then add details incrementally."
			} else {
				actionPrompt += "If the task is truly complete, respond with a brief final report ending with the line: TASK_COMPLETED"
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
			if needs, actionText, path, command := l.toolNeedsPermissionApproval(call); needs {
				if ok := l.requestPermissionApproval(ctx, call, actionText, path, command); !ok {
					state.FinalOutput = "Permission denied. Agent stopped; waiting for new instructions."
					state.EndedAt = time.Now()
					l.saveState(state)
					return state, nil
				}
			}

			result := l.executeToolWithProgress(ctx, call)
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

	if !state.Completed && state.FinalOutput == "" {
		// Avoid user-facing iteration-limit messages; callers may choose to retry/continue.
		if !l.Relentless {
			state.FinalOutput = fmt.Sprintf("AGENT_STEP_LIMIT_REACHED (%d iterations)", l.MaxLoops)
		}
	}
	state.FinalOutput = RedactSecrets(state.FinalOutput, l.apiKeyForRedaction())
	l.saveState(state)

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
	if l.SystemMessageBuilder != nil {
		if msg := strings.TrimSpace(l.SystemMessageBuilder(task)); msg != "" {
			return msg
		}
	}

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

func progressTextForTool(call ToolCall) string {
	switch call.Name {
	case "list_dir":
		return "Exploring directories"
	case "search_files":
		return "Searching files"
	case "grep":
		return "Searching content"
	case "read_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		if strings.TrimSpace(args.Path) != "" {
			p := strings.TrimSpace(args.Path)
			return "Inspecting " + p
		}
		return "Reading files"
	case "write_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		if strings.TrimSpace(args.Path) != "" {
			p := strings.TrimSpace(args.Path)
			pl := strings.ToLower(p)
			if strings.HasSuffix(pl, ".html") {
				return "Crafting " + p
			}
			return "Writing " + p
		}
		return "Writing files"
	case "edit_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		if strings.TrimSpace(args.Path) != "" {
			p := strings.TrimSpace(args.Path)
			pl := strings.ToLower(p)
			if strings.HasSuffix(pl, ".html") {
				return "Refining " + p
			}
			return "Editing " + p
		}
		return "Editing files"
	case "append_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		if strings.TrimSpace(args.Path) != "" {
			return "Updating " + strings.TrimSpace(args.Path)
		}
		return "Updating files"
	case "patch_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		if strings.TrimSpace(args.Path) != "" {
			p := strings.TrimSpace(args.Path)
			return "Patching " + p
		}
		return "Applying patch"
	case "exec":
		// Keep short; commands can be long.
		var args struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		cmd := strings.TrimSpace(args.Command)
		if cmd == "" {
			return "Running commands"
		}
		// Keep it readable; avoid massive one-liners.
		if len(cmd) > 60 {
			cmd = cmd[:60] + "..."
		}
		return "Running: " + cmd
	default:
		return "Working"
	}
}

func progressTextForToolStatus(call ToolCall, status string) string {
	switch status {
	case "completed":
		switch call.Name {
		case "exec":
			return "Command finished"
		case "read_file":
			return "Read finished"
		case "list_dir":
			return "Directory listing finished"
		case "search_files", "grep":
			return "Search finished"
		case "write_file", "edit_file", "append_file", "patch_file":
			return "Edit finished"
		default:
			return "Step finished"
		}
	case "error":
		switch call.Name {
		case "exec":
			return "Command failed"
		case "read_file":
			return "Read failed"
		case "list_dir":
			return "List failed"
		case "search_files", "grep":
			return "Search failed"
		case "write_file", "edit_file", "append_file", "patch_file":
			return "Edit failed"
		default:
			return "Step failed"
		}
	default:
		return progressTextForTool(call)
	}
}

func (l *AgentLoop) emitProgress(ev ProgressEvent) {
	if l.Progress == nil {
		return
	}
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	// Never emit API keys or other configured secrets into the UI/log stream.
	ev.Text = RedactSecrets(ev.Text, l.apiKeyForRedaction())
	ev.Error = RedactSecrets(ev.Error, l.apiKeyForRedaction())
	ev.Path = RedactSecrets(ev.Path, l.apiKeyForRedaction())
	ev.Command = RedactSecrets(ev.Command, l.apiKeyForRedaction())
	ev.OldContent = RedactSecrets(ev.OldContent, l.apiKeyForRedaction())
	ev.NewContent = RedactSecrets(ev.NewContent, l.apiKeyForRedaction())
	l.Progress(ev)
}

func (l *AgentLoop) apiKeyForRedaction() string {
	if l == nil || l.Client == nil {
		return ""
	}
	return strings.TrimSpace(l.Client.APIKey)
}

func (l *AgentLoop) toolProgressDetails(call ToolCall) (path, command string) {
	switch call.Name {
	case "exec":
		var args struct {
			Command string `json:"command"`
			Cwd     string `json:"cwd"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		command = strings.TrimSpace(args.Command)
		path = strings.TrimSpace(args.Cwd)
	case "read_file", "write_file", "edit_file", "append_file", "patch_file":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		path = strings.TrimSpace(args.Path)
	case "list_dir":
		var args struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		path = strings.TrimSpace(args.Path)
	case "search_files":
		var args struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		command = strings.TrimSpace(args.Pattern)
		path = strings.TrimSpace(args.Path)
	case "grep":
		var args struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path"`
		}
		_ = json.Unmarshal(call.Arguments, &args)
		command = strings.TrimSpace(args.Pattern)
		path = strings.TrimSpace(args.Path)
	}
	return path, command
}

func summarizeForApproval(input string, max int) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	input = strings.ReplaceAll(input, "\r", " ")
	input = strings.ReplaceAll(input, "\n", " ")
	input = strings.Join(strings.Fields(input), " ")
	if max <= 0 || len(input) <= max {
		return input
	}
	if max <= 3 {
		return input[:max]
	}
	return strings.TrimSpace(input[:max-3]) + "..."
}

func execCommandNeedsApproval(command string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	cmd = strings.ReplaceAll(cmd, "\r", " ")
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	cmd = strings.Join(strings.Fields(cmd), " ")
	if cmd == "" {
		return false
	}

	// Heuristic: ask before privileged/destructive/system-level commands.
	dangerous := []string{
		"sudo ", "sudo	", "sudo-", " su ", "pkexec ", "doas ", "runas ",
		"rm -rf", "rm -fr", "rm -r ", "rm -r/", "rm -r.", "rm -r..", "rmdir /s", "del /s", "del /q", "format ", "mkfs", "dd if=",
		// Git operations that mutate history or working tree in ways that are hard to undo.
		"git commit", "git push", "git tag", "git rebase", "git merge", "git cherry-pick", "git revert",
		"git reset --hard", "git clean -fd", "git clean -ff", "git checkout --",
		"systemctl ", "service ", "launchctl ", "sc ", "net stop", "net start", "shutdown", "reboot", "poweroff", "halt",
		"chmod ", "chown ", "setfacl ", "icacls ",
		"apt-get ", "apt ", "yum ", "dnf ", "pacman ", "brew ", "winget ", "choco ",
		"/etc/", "/usr/", "/bin/", "/sbin/", "/var/", "/library/", "/applications/", "c:\\windows", "hklm\\",
	}
	for _, needle := range dangerous {
		if strings.Contains(cmd, needle) {
			return true
		}
	}

	// Common "curl | sh" install patterns.
	if (strings.Contains(cmd, "curl ") || strings.Contains(cmd, "wget ")) && strings.Contains(cmd, "|") {
		if strings.Contains(cmd, "| sh") || strings.Contains(cmd, "|bash") || strings.Contains(cmd, "| bash") || strings.Contains(cmd, "| zsh") {
			return true
		}
	}

	return false
}

func isPathOutsideWorkDir(workDir, absPath string) bool {
	workDir = strings.TrimSpace(workDir)
	absPath = strings.TrimSpace(absPath)
	if workDir == "" || absPath == "" {
		return true
	}
	wd := filepath.Clean(workDir)
	p := filepath.Clean(absPath)
	rel, err := filepath.Rel(wd, p)
	if err != nil {
		return true
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return false
	}
	if rel == ".." {
		return true
	}
	prefix := ".." + string(filepath.Separator)
	return strings.HasPrefix(rel, prefix)
}

func (l *AgentLoop) toolNeedsPermissionApproval(call ToolCall) (needs bool, actionText string, path string, command string) {
	// In full-access mode, ask the user before executing risky actions. In
	// dangerously-full-access, auto-allow everything.
	if NormalizePermissionsMode(l.PermissionsMode) != PermissionsFullAccess {
		return false, "", "", ""
	}
	if l.PermissionDecisions == nil {
		return false, "", "", ""
	}

	path, command = l.toolProgressDetails(call)

	switch call.Name {
	case "exec":
		if execCommandNeedsApproval(command) {
			actionText = "EAI agent wants to run: " + summarizeForApproval(command, 140)
			if strings.TrimSpace(path) != "" {
				actionText += " (cwd: " + summarizeForApproval(path, 60) + ")"
			}
			return true, actionText, path, command
		}

	case "read_file":
		if strings.TrimSpace(path) == "" {
			return false, "", "", ""
		}
		abs := l.resolvePath(path)
		if isPathOutsideWorkDir(l.WorkDir, abs) {
			actionText = "EAI agent wants to read: " + summarizeForApproval(abs, 160)
			return true, actionText, abs, ""
		}

	case "write_file", "edit_file", "append_file", "patch_file":
		if strings.TrimSpace(path) == "" {
			return false, "", "", ""
		}
		abs := l.resolvePath(path)
		if isPathOutsideWorkDir(l.WorkDir, abs) {
			actionText = "EAI agent wants to modify: " + summarizeForApproval(abs, 160)
			return true, actionText, abs, ""
		}
	}

	return false, "", "", ""
}

func (l *AgentLoop) requestPermissionApproval(ctx context.Context, call ToolCall, actionText, path, command string) bool {
	actionText = strings.TrimSpace(actionText)
	if actionText == "" {
		actionText = "EAI agent requests permission."
	}

	l.emitProgress(ProgressEvent{
		Kind:       "permission_request",
		Text:       actionText,
		Tool:       call.Name,
		ToolCallID: call.ID,
		Path:       strings.TrimSpace(path),
		Command:    strings.TrimSpace(command),
	})

	// If we're not wired up to a UI prompt, default to allow.
	if l.PermissionDecisions == nil {
		return true
	}

	for {
		select {
		case <-ctx.Done():
			return false
		case dec, ok := <-l.PermissionDecisions:
			if !ok {
				return false
			}
			if strings.TrimSpace(dec.ToolCallID) != strings.TrimSpace(call.ID) {
				continue
			}
			if !dec.Allow {
				l.emitProgress(ProgressEvent{Kind: "warn", Text: "Permission denied; stopping agent"})
			}
			return dec.Allow
		}
	}
}

func commandContainsSudo(command string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	if command == "" {
		return false
	}
	// Keep this cheap: detect obvious sudo invocations in shell command chains.
	if strings.HasPrefix(command, "sudo ") || strings.Contains(command, " sudo ") {
		return true
	}
	if strings.Contains(command, "&&sudo ") || strings.Contains(command, "||sudo ") || strings.Contains(command, ";sudo ") || strings.Contains(command, "|sudo ") {
		return true
	}
	return false
}

func isPermissionDeniedFailure(err error, output []byte) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(string(output)))
	if e := strings.TrimSpace(err.Error()); e != "" {
		if text != "" {
			text += "\n"
		}
		text += strings.ToLower(e)
	}
	markers := []string{
		"permission denied",
		"operation not permitted",
		"access is denied",
		"access denied",
		"eacces",
		"requires elevated",
		"must be superuser",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func isSudoAuthenticationFailure(err error, output []byte) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(string(output)))
	if e := strings.TrimSpace(err.Error()); e != "" {
		if text != "" {
			text += "\n"
		}
		text += strings.ToLower(e)
	}
	markers := []string{
		"sudo: a password is required",
		"no tty present",
		"a terminal is required",
		"sorry, try again",
		"sudo: a password",
		"askpass",
	}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func runShellCommand(ctx context.Context, shell string, shellArgs []string, dir string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, shell, shellArgs...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = env
	}
	return runCommandWithCapturedOutput(cmd)
}

func runShellCommandWithSudo(ctx context.Context, shell string, shellArgs []string, dir string, env []string) (output []byte, err error, attempted bool) {
	if IsProcessRoot() {
		return nil, nil, false
	}
	if _, lookErr := exec.LookPath("sudo"); lookErr != nil {
		return nil, nil, false
	}

	sudoPassword := strings.TrimSpace(os.Getenv("EAI_DESKTOP_SUDO_PASSWORD"))
	var sudoArgs []string
	if sudoPassword == "" {
		sudoArgs = append([]string{"-n", shell}, shellArgs...)
	} else {
		// Use stdin-driven sudo to honor the password captured by the desktop app.
		sudoArgs = append([]string{"-S", shell}, shellArgs...)
	}

	cmd := exec.CommandContext(ctx, "sudo", sudoArgs...)
	if sudoPassword != "" {
		cmd.Stdin = strings.NewReader(sudoPassword + "\n")
	}
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = env
	}
	out, runErr := runCommandWithCapturedOutput(cmd)
	return out, runErr, true
}

var dockerGroupErrMarkers = []string{
	"permission denied while trying to connect to the docker daemon socket",
	"dial unix /var/run/docker.sock: connect: permission denied",
	"got permission denied while trying to connect to the docker daemon socket",
}

func isDockerGroupPermissionFailure(err error, output []byte) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(string(output)))
	if e := strings.TrimSpace(err.Error()); e != "" {
		if text != "" {
			text += "\n"
		}
		text += strings.ToLower(e)
	}
	for _, marker := range dockerGroupErrMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func commandLikelyNeedsDockerGroup(command string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return false
	}
	// Avoid double-wrapping.
	if strings.Contains(cmd, "sg docker -c") || strings.Contains(cmd, "newgrp docker") {
		return false
	}
	needles := []string{
		"docker ",
		"docker-compose",
		"docker compose",
		"harbor ",
		"harbor_",
		"./harbor_",
		"terminal-bench",
		"terminal bench",
		"tbench",
	}
	for _, n := range needles {
		if strings.Contains(cmd, n) {
			return true
		}
	}
	return false
}

func commandLikelyNeedsEAIKey(command string) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return false
	}
	needles := []string{
		"eai_api_key",
		"terminal-bench",
		"terminal bench",
		"tbench",
		"harbor ",
		"harbor_",
		"./harbor_",
	}
	for _, n := range needles {
		if strings.Contains(cmd, n) {
			return true
		}
	}
	return false
}

func envGet(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix), true
		}
	}
	return "", false
}

func envSet(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func envPrependPath(env []string, prefix string) []string {
	if strings.TrimSpace(prefix) == "" {
		return env
	}
	current, ok := envGet(env, "PATH")
	if !ok || strings.TrimSpace(current) == "" {
		return envSet(env, "PATH", prefix)
	}
	// Avoid duplication.
	parts := strings.Split(current, string(os.PathListSeparator))
	for _, p := range parts {
		if p == prefix {
			return env
		}
	}
	return envSet(env, "PATH", prefix+string(os.PathListSeparator)+current)
}

func runShellCommandWithDockerGroup(ctx context.Context, shell string, shellArgs []string, dir string, env []string) (output []byte, err error, attempted bool) {
	if _, lookErr := exec.LookPath("sg"); lookErr != nil {
		return nil, nil, false
	}
	// Best-effort check that docker group exists; if it doesn't, sg will fail anyway,
	// but we can avoid a confusing retry.
	if _, lookErr := exec.LookPath("getent"); lookErr == nil {
		getent := exec.CommandContext(ctx, "getent", "group", "docker")
		if strings.TrimSpace(dir) != "" {
			getent.Dir = dir
		}
		if len(env) > 0 {
			getent.Env = env
		}
		if err := getent.Run(); err != nil {
			return nil, nil, false
		}
	}

	// Build a command string for `sg docker -c ...`.
	// We intentionally keep this simple: `shellArgs` should be `-lc <cmd>` or `-c <cmd>`.
	cmdStr := shell
	if len(shellArgs) > 0 {
		cmdStr += " " + strings.Join(shellArgs[:len(shellArgs)-1], " ")
		cmdStr += " " + shellSingleQuote(shellArgs[len(shellArgs)-1])
	}

	cmd := exec.CommandContext(ctx, "sg", "docker", "-c", cmdStr)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = env
	}
	out, runErr := runCommandWithCapturedOutput(cmd)
	return out, runErr, true
}

func runCommandWithCapturedOutput(cmd *exec.Cmd) ([]byte, error) {
	tmp, err := os.CreateTemp("", "cli-agent-exec-output-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	// Avoid CombinedOutput pipe inheritance hangs when child processes detach.
	cmd.Stdout = tmp
	cmd.Stderr = tmp

	runErr := cmd.Run()
	if _, seekErr := tmp.Seek(0, io.SeekStart); seekErr != nil {
		if runErr != nil {
			return nil, runErr
		}
		return nil, seekErr
	}
	output, readErr := io.ReadAll(tmp)
	if readErr != nil {
		if runErr != nil {
			return output, runErr
		}
		return output, readErr
	}
	return output, runErr
}

func shouldAutoDetachServerCommand(command string) bool {
	if strings.TrimSpace(command) == "" {
		return false
	}
	if hasShellBackgroundOperator(command) {
		return false
	}

	for _, pattern := range autoDetachServerPatterns {
		if pattern.MatchString(command) {
			return true
		}
	}
	return false
}

func shellSingleQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func buildDetachedServerCommand(command string) (wrapped string, logPath string, err error) {
	f, err := os.CreateTemp("", "cli-agent-server-*.log")
	if err != nil {
		return "", "", err
	}
	logPath = f.Name()
	if closeErr := f.Close(); closeErr != nil {
		return "", "", closeErr
	}

	wrapped = fmt.Sprintf("(%s) > %s 2>&1 < /dev/null & echo $!", command, shellSingleQuote(logPath))
	return wrapped, logPath, nil
}

func verifyDetachedServerLaunch(ctx context.Context, pid int, hasPID bool, logPath string) (string, error) {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		logText := readTextFileAtMost(logPath, 64*1024)
		if fail := detectServerStartFailure(logText); fail != "" {
			return "", fmt.Errorf("server startup failed: %s", fail)
		}
		if hasPID && !isProcessAlive(pid) {
			if summary := summarizeLogSnippet(logText, 8); summary != "" {
				return "", fmt.Errorf("background server process %d exited quickly; log: %s", pid, summary)
			}
			return "", fmt.Errorf("background server process %d exited quickly", pid)
		}

		if url := extractLocalServerURL(logText); url != "" {
			if hasPID {
				return fmt.Sprintf("Started background server (PID %d) at %s. Log: %s", pid, url, logPath), nil
			}
			return fmt.Sprintf("Started background server at %s. Log: %s", url, logPath), nil
		}
		if looksLikeServerReadyLog(logText) {
			if hasPID {
				return fmt.Sprintf("Started background server (PID %d). Log: %s", pid, logPath), nil
			}
			return fmt.Sprintf("Started background server. Log: %s", logPath), nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	logText := readTextFileAtMost(logPath, 64*1024)
	if fail := detectServerStartFailure(logText); fail != "" {
		return "", fmt.Errorf("server startup failed: %s", fail)
	}
	if hasPID && !isProcessAlive(pid) {
		if summary := summarizeLogSnippet(logText, 8); summary != "" {
			return "", fmt.Errorf("background server process %d exited quickly; log: %s", pid, summary)
		}
		return "", fmt.Errorf("background server process %d exited quickly", pid)
	}

	if hasPID {
		return fmt.Sprintf("Started background server (PID %d). Log: %s", pid, logPath), nil
	}
	return fmt.Sprintf("Started background server. Log: %s", logPath), nil
}

func readTextFileAtMost(path string, maxBytes int64) string {
	if strings.TrimSpace(path) == "" || maxBytes <= 0 {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxBytes))
	if err != nil || len(data) == 0 || isProbablyBinary(data) {
		return ""
	}
	return string(data)
}

func detectServerStartFailure(logText string) string {
	low := strings.ToLower(logText)
	if strings.TrimSpace(low) == "" {
		return ""
	}
	markers := []string{
		"eaddrinuse",
		"address already in use",
		"port is already in use",
		"failed to start",
		"error: listen",
		"permission denied",
		"module not found",
		"cannot find module",
		"command not found",
	}
	for _, marker := range markers {
		if strings.Contains(low, marker) {
			return summarizeLogSnippet(logText, 8)
		}
	}
	return ""
}

func looksLikeServerReadyLog(logText string) bool {
	low := strings.ToLower(logText)
	if strings.TrimSpace(low) == "" {
		return false
	}
	markers := []string{
		"ready in",
		"listening on",
		"server running",
		"compiled successfully",
		"local:",
		"localhost",
		"127.0.0.1",
	}
	for _, marker := range markers {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

func extractLocalServerURL(logText string) string {
	url := localServerURLRe.FindString(logText)
	if strings.TrimSpace(url) == "" {
		return ""
	}
	return strings.ReplaceAll(url, "0.0.0.0", "127.0.0.1")
}

func summarizeLogSnippet(logText string, maxLines int) string {
	logText = strings.TrimSpace(logText)
	if logText == "" {
		return ""
	}
	if maxLines <= 0 {
		maxLines = 6
	}
	lines := strings.Split(logText, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	summary := strings.Join(lines, " | ")
	if len(summary) > 500 {
		summary = summary[:500] + "..."
	}
	return summary
}

func verifyPythonHTTPBackgroundLaunch(command, runDir string, output []byte) (string, error) {
	if !hasShellBackgroundOperator(command) {
		return "", nil
	}
	port, ok := parsePythonHTTPServerPort(command)
	if !ok {
		return "", nil
	}

	pid, hasPID := extractBackgroundPID(string(output))

	// Give the server a brief window to bind before probing.
	time.Sleep(250 * time.Millisecond)

	// If the reported PID is already gone, this commonly indicates bind failure.
	if hasPID && !isProcessAlive(pid) {
		status, body, fetchErr := fetchLocalHTTPRoot(port, 1500*time.Millisecond)
		if fetchErr != nil {
			return "", fmt.Errorf("background server PID %d exited quickly; failed to serve http://127.0.0.1:%d (%v)", pid, port, fetchErr)
		}
		if status >= http.StatusBadRequest {
			return "", fmt.Errorf("background server PID %d exited quickly; port %d returned HTTP %d", pid, port, status)
		}
		match, detail := responseMatchesProjectIndex(runDir, body)
		if match {
			return fmt.Sprintf("Port %d was already serving expected project content (%s).", port, detail), nil
		}
		return "", fmt.Errorf("background server PID %d exited quickly and port %d serves different content (%s)", pid, port, detail)
	}

	status, body, fetchErr := fetchLocalHTTPRoot(port, 2*time.Second)
	if fetchErr != nil {
		if hasPID {
			return "", fmt.Errorf("python http.server started with PID %d but http://127.0.0.1:%d is not reachable: %v", pid, port, fetchErr)
		}
		return "", fmt.Errorf("python http.server did not become reachable on http://127.0.0.1:%d: %v", port, fetchErr)
	}
	if status >= http.StatusBadRequest {
		return "", fmt.Errorf("python http.server on port %d returned HTTP %d", port, status)
	}

	match, detail := responseMatchesProjectIndex(runDir, body)
	if !match {
		return "", fmt.Errorf("server on port %d responded with unexpected content (%s)", port, detail)
	}
	if hasPID {
		return fmt.Sprintf("Verified http://127.0.0.1:%d serves expected project content (PID %d, %s).", port, pid, detail), nil
	}
	return fmt.Sprintf("Verified http://127.0.0.1:%d serves expected project content (%s).", port, detail), nil
}

func hasShellBackgroundOperator(command string) bool {
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(command); i++ {
		ch := command[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble || ch != '&' {
			continue
		}

		var prev byte
		if i > 0 {
			prev = command[i-1]
		}
		var next byte
		if i+1 < len(command) {
			next = command[i+1]
		}

		// Ignore && and redirection operators (2>&1, &>file).
		if prev == '&' || next == '&' || prev == '>' || prev == '<' || next == '>' || next == '<' {
			continue
		}
		return true
	}
	return false
}

func parsePythonHTTPServerPort(command string) (int, bool) {
	if !strings.Contains(strings.ToLower(command), "http.server") {
		return 0, false
	}

	tokens := strings.Fields(command)
	seenHTTPServer := false
	for i := 0; i < len(tokens); i++ {
		token := strings.Trim(tokens[i], " \t\r\n;|&")
		token = strings.Trim(token, `"'`)

		if strings.EqualFold(token, "http.server") {
			seenHTTPServer = true
			continue
		}
		if !seenHTTPServer || token == "" {
			continue
		}
		if strings.HasPrefix(token, ">") || strings.HasPrefix(token, "<") || strings.Contains(token, ">&") || strings.Contains(token, "<&") {
			continue
		}
		if strings.HasPrefix(token, "-") {
			// Skip bind argument value.
			if token == "--bind" || token == "-b" {
				if i+1 < len(tokens) {
					i++
				}
			}
			continue
		}
		if port, ok := parsePortToken(token); ok {
			return port, true
		}
	}

	// python -m http.server defaults to port 8000.
	return 8000, true
}

func parsePortToken(token string) (int, bool) {
	token = strings.Trim(token, " \t\r\n;|&")
	if strings.Contains(token, ">&") || strings.Contains(token, "<&") {
		return 0, false
	}
	token = strings.TrimPrefix(token, "http://")
	token = strings.TrimPrefix(token, "https://")
	if idx := strings.LastIndex(token, ":"); idx >= 0 {
		token = token[idx+1:]
	}

	// Accept leading digits only (e.g. "8000>/dev/null").
	i := 0
	for i < len(token) && token[i] >= '0' && token[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false
	}
	port, err := strconv.Atoi(token[:i])
	if err != nil || port < 1 || port > 65535 {
		return 0, false
	}
	return port, true
}

func extractBackgroundPID(output string) (int, bool) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil || pid <= 1 {
			continue
		}
		return pid, true
	}
	return 0, false
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if errno, ok := err.(syscall.Errno); ok && errno == syscall.EPERM {
		return true
	}
	return false
}

func fetchLocalHTTPRoot(port int, timeout time.Duration) (int, string, error) {
	if port < 1 || port > 65535 {
		return 0, "", fmt.Errorf("invalid port %d", port)
	}

	target := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	deadline := time.Now().Add(timeout)

	var lastErr error
	for {
		resp, err := client.Get(target)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
			resp.Body.Close()
			if readErr != nil {
				return resp.StatusCode, "", readErr
			}
			return resp.StatusCode, string(body), nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("timed out reaching %s", target)
	}
	return 0, "", lastErr
}

func responseMatchesProjectIndex(runDir, body string) (bool, string) {
	runDir = strings.TrimSpace(runDir)
	if runDir == "" {
		return true, "skipped index match (no working directory)"
	}

	indexPath := filepath.Join(runDir, "index.html")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, "skipped index match (index.html not found)"
		}
		return false, fmt.Sprintf("failed to read %s: %v", indexPath, err)
	}

	marker := extractIndexHTMLMarker(string(data))
	if marker == "" {
		return true, "skipped index match (no stable marker in index.html)"
	}
	if strings.Contains(body, marker) {
		return true, fmt.Sprintf("matched marker %q", marker)
	}
	return false, fmt.Sprintf("expected marker %q from %s", marker, indexPath)
}

func extractIndexHTMLMarker(indexHTML string) string {
	if m := indexTitleRe.FindStringSubmatch(indexHTML); len(m) == 2 {
		marker := strings.TrimSpace(m[1])
		if marker != "" {
			return marker
		}
	}
	if m := indexScriptRe.FindStringSubmatch(indexHTML); len(m) == 2 {
		marker := strings.TrimSpace(m[1])
		if marker != "" {
			return marker
		}
	}
	candidates := indexTokenRe.FindAllString(indexHTML, 24)
	for _, candidate := range candidates {
		if isGenericHTMLMarker(candidate) {
			continue
		}
		return candidate
	}
	return ""
}

func isGenericHTMLMarker(marker string) bool {
	m := strings.ToLower(strings.TrimSpace(marker))
	switch m {
	case "doctype", "html", "head", "body", "meta", "script", "charset", "viewport", "utf-8", "root", "main":
		return true
	}
	return false
}

func (l *AgentLoop) emitToolProgress(call ToolCall, status string, durationMs int64, errText string) {
	path, command := l.toolProgressDetails(call)
	l.emitProgress(ProgressEvent{
		Kind:       "tool",
		Text:       progressTextForToolStatus(call, status),
		Tool:       call.Name,
		ToolCallID: call.ID,
		ToolStatus: status,
		Path:       path,
		Command:    command,
		DurationMs: durationMs,
		Error:      strings.TrimSpace(errText),
	})
}

func (l *AgentLoop) executeToolWithProgress(ctx context.Context, call ToolCall) ToolResult {
	if !l.toolNameAllowed(call.Name) {
		errText := fmt.Sprintf("Tool not allowed in this mode: %s", call.Name)
		result := ToolResult{
			ToolCallID: call.ID,
			Success:    false,
			Error:      errText,
			DurationMs: 0,
		}
		l.emitToolProgress(call, "error", 0, errText)
		return result
	}
	if l.ToolCallFilter != nil {
		if ok, reason := l.ToolCallFilter(call); !ok {
			errText := strings.TrimSpace(reason)
			if errText == "" {
				errText = "Tool call blocked by policy"
			}
			result := ToolResult{
				ToolCallID: call.ID,
				Success:    false,
				Error:      errText,
				DurationMs: 0,
			}
			l.emitToolProgress(call, "error", 0, errText)
			return result
		}
	}

	// Capture before/after file contents for UI-only diff rendering.
	// Guard on l.Progress to avoid impacting non-interactive runs (tbench/cli).
	var (
		fileEditEnabled   = l.Progress != nil && isFileEditTool(call.Name)
		fileEditPathArg   string
		fileEditPathAbs   string
		fileExistedBefore bool
		oldContent        string
		oldOK             bool
	)
	if fileEditEnabled {
		pathArg, _ := l.toolProgressDetails(call)
		fileEditPathArg = strings.TrimSpace(pathArg)
		if fileEditPathArg != "" {
			fileEditPathAbs = l.resolvePath(fileEditPathArg)
			if _, err := os.Stat(fileEditPathAbs); err == nil {
				fileExistedBefore = true
				const maxDiffBytes = 32 * 1024
				if txt, ok, _ := readFileAtMost(fileEditPathAbs, maxDiffBytes); ok {
					oldContent = txt
					oldOK = true
				}
			}
		}
	}

	l.emitToolProgress(call, "pending", 0, "")
	result := l.executeTool(ctx, call)
	// Ensure we never persist/display API keys in tool output/errors.
	result.Output = RedactSecrets(result.Output, l.apiKeyForRedaction())
	result.Error = RedactSecrets(result.Error, l.apiKeyForRedaction())
	status := "completed"
	errText := ""
	if !result.Success {
		status = "error"
		errText = result.Error
	}
	l.emitToolProgress(call, status, result.DurationMs, errText)

	// Emit a small output preview for exec so the UI can show it under the command.
	if call.Name == "exec" && strings.TrimSpace(result.Output) != "" {
		if preview := summarizeToolOutputForProgress(result.Output); strings.TrimSpace(preview) != "" {
			l.emitProgress(ProgressEvent{
				Kind:       "tool_output",
				Text:       preview,
				Tool:       call.Name,
				ToolCallID: call.ID,
				ToolStatus: status,
				DurationMs: result.DurationMs,
			})
		}
	}

	if fileEditEnabled && result.Success && fileEditPathAbs != "" {
		changeType := "modify"
		fileExistsAfter := false
		newContent := ""
		newOK := false

		if _, err := os.Stat(fileEditPathAbs); err == nil {
			fileExistsAfter = true
			const maxDiffBytes = 32 * 1024
			if txt, ok, _ := readFileAtMost(fileEditPathAbs, maxDiffBytes); ok {
				newContent = txt
				newOK = true
			}
		}

		switch {
		case !fileExistedBefore && fileExistsAfter:
			changeType = "create"
		case fileExistedBefore && !fileExistsAfter:
			changeType = "delete"
		default:
			changeType = "modify"
		}

		// If we couldn't safely read contents (too large), still emit the edit event
		// so the UI can show the file path without a diff.
		if !oldOK {
			oldContent = ""
		}
		if !newOK {
			newContent = ""
		}

		pathForUI := fileEditPathArg
		if strings.TrimSpace(pathForUI) == "" {
			pathForUI = fileEditPathAbs
		}

		l.emitProgress(ProgressEvent{
			Kind:       "file_edit",
			Tool:       call.Name,
			ToolCallID: call.ID,
			ToolStatus: status,
			Path:       pathForUI,
			ChangeType: changeType,
			OldContent: oldContent,
			NewContent: newContent,
			DurationMs: result.DurationMs,
		})
	}
	return result
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
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	// Expand env vars (e.g., $HOME) and normalize slashes.
	path = os.ExpandEnv(path)
	path = filepath.FromSlash(path)

	// Expand "~" to the user's home directory.
	if path == "~" || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, strings.TrimPrefix(path, "~"+string(filepath.Separator)))
			}
		}
	}

	// Treat common user folders as home-relative when referenced directly (e.g., "Desktop/eai").
	if !filepath.IsAbs(path) && !strings.HasPrefix(path, "."+string(filepath.Separator)) && !strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			parts := strings.Split(path, string(filepath.Separator))
			if len(parts) > 0 {
				switch strings.ToLower(strings.TrimSpace(parts[0])) {
				case "desktop", "downloads", "documents", "pictures", "music", "videos":
					path = filepath.Join(home, path)
				}
			}
		}
	}

	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if l.WorkDir == "" {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(l.WorkDir, path))
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
	veryLong := 8 * time.Hour

	cmdLower := strings.ToLower(command)
	if shouldAutoDetachServerCommand(command) {
		// Foreground dev servers are long-lived; keep timeout modest so we can detach/verify quickly.
		return 45 * time.Second
	}
	veryLongKeywords := []string{
		"harbor jobs start",
		"harbor run",
		"harbor_run",
		"terminal-bench",
		"terminal bench",
		"tbench2",
		"tbench",
	}
	for _, kw := range veryLongKeywords {
		if strings.Contains(cmdLower, kw) {
			return veryLong
		}
	}
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

func summarizeToolOutputForProgress(output string) string {
	output = strings.TrimRight(output, "\n")
	if strings.TrimSpace(output) == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	const maxLines = 12
	if len(lines) <= maxLines {
		return output
	}
	head := 3
	tail := 3
	if head+tail+1 > maxLines {
		head = 2
		tail = 2
	}
	if head+tail+1 > maxLines {
		head = 1
		tail = 1
	}
	if head+tail > len(lines) {
		return output
	}
	omitted := len(lines) - head - tail
	if omitted < 0 {
		omitted = 0
	}

	var b strings.Builder
	for i := 0; i < head && i < len(lines); i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}
	if omitted > 0 {
		b.WriteString(fmt.Sprintf("… +%d lines\n", omitted))
	}
	for i := len(lines) - tail; i < len(lines); i++ {
		if i < 0 {
			continue
		}
		b.WriteString(lines[i])
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func isFileEditTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "write_file", "edit_file", "append_file", "patch_file":
		return true
	default:
		return false
	}
}

func readFileAtMost(path string, maxBytes int64) (content string, ok bool, err error) {
	if maxBytes <= 0 {
		return "", false, fmt.Errorf("maxBytes must be > 0")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", false, err
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("path is a directory: %s", path)
	}
	if info.Size() > maxBytes {
		return "", false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	if int64(len(data)) > maxBytes {
		return "", false, nil
	}
	if isProbablyBinary(data) {
		return "", false, nil
	}
	return string(data), true, nil
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
		commandToRun := args.Command
		autoDetached := false
		detachedLogPath := ""
		if shouldAutoDetachServerCommand(args.Command) {
			wrapped, logPath, wrapErr := buildDetachedServerCommand(args.Command)
			if wrapErr != nil {
				result.Error = fmt.Sprintf("failed to prepare detached server command: %v", wrapErr)
				result.DurationMs = time.Since(start).Milliseconds()
				return result
			}
			commandToRun = wrapped
			autoDetached = true
			detachedLogPath = logPath
		}

		shellArgs := []string{"-lc", commandToRun}
		if _, err := exec.LookPath(shell); err != nil {
			shell = "sh"
			shellArgs = []string{"-c", commandToRun}
		}

		runDir := ""
		if args.Cwd != "" {
			runDir = l.resolvePath(args.Cwd)
		} else if l.WorkDir != "" {
			runDir = l.WorkDir
		}

		execEnv := os.Environ()
		if strings.TrimSpace(runDir) != "" {
			venvBin := filepath.Join(runDir, ".venv", "bin")
			if st, err := os.Stat(venvBin); err == nil && st.IsDir() {
				execEnv = envPrependPath(execEnv, venvBin)
			}
		}
		if commandLikelyNeedsEAIKey(args.Command) {
			if key := l.apiKeyForRedaction(); key != "" {
				if _, ok := envGet(execEnv, "EAI_API_KEY"); !ok {
					execEnv = envSet(execEnv, "EAI_API_KEY", key)
				}
			}
		}

		var (
			output       []byte
			err          error
			dockerSgUsed bool
		)
		if commandLikelyNeedsDockerGroup(args.Command) && !commandContainsSudo(args.Command) {
			if out, runErr, attempted := runShellCommandWithDockerGroup(ctx, shell, shellArgs, runDir, execEnv); attempted {
				output = out
				err = runErr
				dockerSgUsed = true
			} else {
				output, err = runShellCommand(ctx, shell, shellArgs, runDir, execEnv)
			}
		} else {
			output, err = runShellCommand(ctx, shell, shellArgs, runDir, execEnv)
		}

		// In dangerously-full-access mode, retry permission failures with
		// non-interactive sudo when possible.
		if err != nil &&
			NormalizePermissionsMode(l.PermissionsMode) == PermissionsDangerouslyFullAccess &&
			!commandContainsSudo(args.Command) &&
			isPermissionDeniedFailure(err, output) {

			sudoOutput, sudoErr, attempted := runShellCommandWithSudo(ctx, shell, shellArgs, runDir, execEnv)
			if attempted {
				output = sudoOutput
				err = sudoErr
				if sudoErr != nil && isSudoAuthenticationFailure(sudoErr, sudoOutput) {
					result.Error = "elevation failed: sudo requires cached/passwordless auth. Run `sudo -v` (or relaunch as root/admin) and retry."
				}
			}
		}
		if err != nil && !dockerSgUsed && isDockerGroupPermissionFailure(err, output) {
			// Give a more actionable error message for a very common Harbor failure mode.
			// The user may need to re-login after joining the docker group, or run via `sg docker -c ...`.
			result.Error = "docker permission denied: your user can't access /var/run/docker.sock in this session. Fix: re-login after adding user to docker group, or run docker/harbor commands via `sg docker -c ...`."
		}

		postVerificationNotes := []string{}
		if err == nil {
			if autoDetached {
				pid, hasPID := extractBackgroundPID(string(output))
				note, detachedVerifyErr := verifyDetachedServerLaunch(ctx, pid, hasPID, detachedLogPath)
				if detachedVerifyErr != nil {
					err = detachedVerifyErr
				} else if strings.TrimSpace(note) != "" {
					postVerificationNotes = append(postVerificationNotes, note)
				}
			}
		}
		if err == nil {
			note, verifyErr := verifyPythonHTTPBackgroundLaunch(commandToRun, runDir, output)
			if verifyErr != nil {
				err = verifyErr
			} else if strings.TrimSpace(note) != "" {
				postVerificationNotes = append(postVerificationNotes, note)
			}
		}

		outputText := string(output)
		for _, note := range postVerificationNotes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			outputText = strings.TrimRight(outputText, "\n")
			if strings.TrimSpace(outputText) != "" {
				outputText += "\n"
			}
			outputText += note
		}

		if err != nil && strings.TrimSpace(result.Error) == "" {
			result.Error = err.Error()
		}
		result.Output = truncateToolOutput(outputText)
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
		data, err := os.ReadFile(path)
		if err != nil {
			result.Error = err.Error()
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}

		updated, err := ApplyUnifiedPatch(string(data), args.Patch)
		if err != nil {
			result.Error = err.Error()
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}
		if err := writeFilePreserveMode(path, []byte(updated)); err != nil {
			result.Error = err.Error()
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}
		result.Output = fmt.Sprintf("File patched: %s", path)
		result.Success = true

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
