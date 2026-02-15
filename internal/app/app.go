package app

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultOrchestratePerTaskParallelism  = 2
	defaultOrchestrateMaxPanesPerTask     = 5
	defaultOrchestrateMaxShards           = defaultOrchestratePerTaskParallelism * defaultOrchestrateMaxPanesPerTask
	defaultOrchestrateMaxActivePanes      = 5
	defaultOrchestrateCacheTTL            = 15 * time.Minute
	defaultOrchestrateRetryCount          = 1
	defaultOrchestrateContextHashWordsMax = 180
)

const (
	orchestrateMaxPanesPerTaskEnv = "EAI_ORCHESTRATE_MAX_PANES_PER_TASK"
	orchestrateMaxShardsEnv       = "EAI_ORCHESTRATE_MAX_SHARDS"
	orchestrateActivePanesEnv     = "EAI_ORCHESTRATE_ACTIVE_PANES"
	orchestrateMetricsEnv         = "EAI_ORCHESTRATE_METRICS"
	orchestrateShardTimeoutEnv    = "EAI_ORCHESTRATE_SHARD_TIMEOUT_SEC"
)

type Application struct {
	Config   Config
	Logger   *Logger
	Client   *MinimaxClient
	Runner   *Runner
	Jobs     *JobStore
	Prompter *PromptBuilder
	Memory   SessionStore

	orchestrateCache    map[string]orchestrateCacheEntry
	orchestrateCacheMu  sync.RWMutex
	orchestrateCacheTTL time.Duration
}

type orchestrateCacheEntry struct {
	Output    string
	CreatedAt time.Time
}

func NewApplication(cfg Config, mockMode bool) (*Application, error) {
	logger := NewLogger(DefaultLogWriter())

	var client *MinimaxClient
	if mockMode {
		// Create mock client for testing
		client = NewMinimaxClient("mock", "mock", "mock://", cfg.MaxTokens)
	} else {
		client = NewMinimaxClient(cfg.APIKey, cfg.Model, cfg.BaseURL, cfg.MaxTokens)
	}

	jobPath := filepath.Join(os.TempDir(), "cli-agent", "jobs.json")
	jobStore, err := NewJobStore(jobPath)
	if err != nil {
		return nil, err
	}
	jobRoot := filepath.Join(os.TempDir(), "cli-agent", "logs")
	cacheTTL := defaultOrchestrateCacheTTL
	if v := strings.TrimSpace(os.Getenv("EAI_ORCHESTRATE_CACHE_TTL_SEC")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			cacheTTL = time.Duration(parsed) * time.Second
		}
	}
	var sessionStore SessionStore
	if st, err := NewSQLiteSessionStore(""); err == nil {
		sessionStore = st
	} else {
		// Backward-compatible fallback when SQLite is unavailable.
		sessionStore = NewFileSessionStore("")
	}

	return &Application{
		Config:              cfg,
		Logger:              logger,
		Client:              client,
		Runner:              NewRunner(logger, jobRoot),
		Jobs:                jobStore,
		Prompter:            NewPromptBuilder(),
		Memory:              sessionStore,
		orchestrateCache:    make(map[string]orchestrateCacheEntry),
		orchestrateCacheTTL: cacheTTL,
	}, nil
}

func (a *Application) LoadOrCreateSession(workDir string) (*Session, []StoredMessage, error) {
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}
	return a.Memory.LoadOrCreateCurrentSession(workDir)
}

func (a *Application) CreateSession(workDir string) (*Session, error) {
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}
	return a.Memory.CreateSession(workDir)
}

func (a *Application) LoadSession(workDir string, sessionID string) (*Session, []StoredMessage, error) {
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}
	return a.Memory.LoadSessionForWorkDir(workDir, sessionID)
}

func (a *Application) ListRecentSessions(workDir string, limit int) ([]SessionSummary, error) {
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}
	return a.Memory.ListSessionsForWorkDir(workDir, limit)
}

func (a *Application) DeleteSession(workDir string, sessionID string) error {
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}
	return a.Memory.DeleteSessionChain(workDir, sessionID)
}

func (a *Application) LoadPromptHistory(workDir string) ([]string, error) {
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}
	return a.Memory.LoadPromptHistory(workDir)
}

func (a *Application) SavePromptHistory(workDir string, history []string) error {
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}
	return a.Memory.SavePromptHistory(workDir, history)
}

func deriveSessionTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		content = line
		break
	}
	content = strings.Join(strings.Fields(content), " ")
	if content == "" {
		return ""
	}
	const max = 60
	if len(content) > max {
		content = strings.TrimSpace(content[:max-3]) + "..."
	}
	return content
}

func (a *Application) AppendSessionMessage(sessionID string, role string, content string, mode Mode, workDir string) error {
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}
	if err := a.Memory.AppendMessage(StoredMessage{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Mode:      string(mode),
		CreatedAt: time.Now(),
	}); err != nil {
		return err
	}
	if strings.TrimSpace(sessionID) != "" {
		_ = a.Memory.SetCurrentSession(workDir, sessionID)
		_ = a.Memory.TouchSession(workDir, sessionID)
	}

	// Best-effort: infer a session title from the first user message so session lists
	// in the TUI/desktop are meaningful without requiring an extra model call.
	if strings.EqualFold(strings.TrimSpace(role), "user") {
		title := deriveSessionTitle(content)
		if title != "" {
			if sess, _, err := a.Memory.LoadSessionForWorkDir(workDir, sessionID); err == nil && sess != nil {
				if strings.TrimSpace(sess.Title) == "" {
					sess.Title = title
					_ = a.Memory.SaveSession(sess)
				}
			}
		}
	}

	return nil
}

func isContinueOnlyTurn(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	s = strings.Trim(s, ".!?")
	return s == "continue" || s == "go on" || s == "keep going"
}

func normalizeContinueTurn(input string) (string, bool) {
	if !isContinueOnlyTurn(input) {
		return input, false
	}
	return "Continue the active session task. Use the session summary and recent conversation context for continuity. Do not ask the user to re-paste context.", true
}

func (a *Application) estimateSessionPromptTokens(mode Mode, sessionSummary string, history []StoredMessage, input string) int {
	sessionSummary = strings.TrimSpace(sessionSummary)
	input = strings.TrimSpace(input)

	// Tool-capable modes build a tool-agent prompt; estimate that shape.
	if IsToolMode(mode) || mode == ModePlan {
		sys := GetAgentSystemPrompt("")
		prelude := buildAgentMemoryPrelude(sessionSummary, history)
		var b strings.Builder
		b.WriteString("[SYSTEM]\n")
		b.WriteString(sys)
		b.WriteString("\n\n")
		if strings.TrimSpace(prelude) != "" {
			b.WriteString("[USER]\n")
			b.WriteString(prelude)
			b.WriteString("\n\n")
		}
		b.WriteString("[USER]\n")
		b.WriteString(input)
		return EstimateTokens(b.String())
	}

	// Text-only chat modes use chat prompts with history.
	systemPrompt := GetChatSystemPrompt(mode, "", a.Config.ChatVerbosity)
	ctxInfo := GetProjectContext()
	if ctxInfo != "" {
		systemPrompt = systemPrompt + "\n\n" + ctxInfo
	}
	historyLimit := sessionPromptHistoryLimit
	if strings.TrimSpace(sessionSummary) != "" {
		historyLimit = sessionPromptHistoryLimitSummary
	}
	prompt := buildSessionChatPrompt(systemPrompt, sessionSummary, history, input, historyLimit)
	return EstimateTokens(prompt)
}

type ContextUsage struct {
	EstimatedTokens     int     `json:"estimated_tokens"`
	ContextWindowTokens int     `json:"context_window_tokens"`
	ThresholdTokens     int     `json:"threshold_tokens"`
	PercentUsed         float64 `json:"percent_used"`
	PercentLeft         float64 `json:"percent_left"`
}

func (a *Application) EstimateContextUsageForTurn(workDir string, sessionID string, mode Mode, input string) (ContextUsage, error) {
	var usage ContextUsage
	if a == nil {
		return usage, errors.New("application is nil")
	}
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}

	var (
		sess    *Session
		history []StoredMessage
		err     error
	)
	if strings.TrimSpace(sessionID) == "" {
		sess, history, err = a.Memory.LoadOrCreateCurrentSession(workDir)
	} else {
		sess, history, err = a.Memory.LoadSessionForWorkDir(workDir, sessionID)
	}
	if err != nil {
		return usage, err
	}
	if sess == nil {
		return usage, errors.New("session unavailable")
	}

	normalized := input
	if n, ok := normalizeContinueTurn(input); ok {
		normalized = n
	}

	ctxTokens := a.effectiveContextWindowTokens()
	threshold := int(0.90 * float64(ctxTokens))
	estimate := a.estimateSessionPromptTokens(mode, sess.ContextSummary, history, normalized)

	usage.EstimatedTokens = estimate
	usage.ContextWindowTokens = ctxTokens
	usage.ThresholdTokens = threshold
	if ctxTokens > 0 {
		usage.PercentUsed = float64(estimate) * 100 / float64(ctxTokens)
		if usage.PercentUsed < 0 {
			usage.PercentUsed = 0
		}
		if usage.PercentUsed > 100 {
			usage.PercentUsed = 100
		}
		usage.PercentLeft = 100 - usage.PercentUsed
		if usage.PercentLeft < 0 {
			usage.PercentLeft = 0
		}
	}
	return usage, nil
}

// PrepareSessionForTurn ensures there is a persisted session and performs
// automatic session compaction when we approach the model context window.
//
// If compaction triggers, it creates a child session (linked via RootID/ParentID)
// and returns that new session. Prior messages are preserved for UI history.
func (a *Application) PrepareSessionForTurn(
	ctx context.Context,
	workDir string,
	sessionID string,
	mode Mode,
	input string,
	progress func(ProgressEvent),
) (*Session, error) {
	if a == nil {
		return nil, errors.New("application is nil")
	}
	if a.Memory == nil {
		if st, err := NewSQLiteSessionStore(""); err == nil {
			a.Memory = st
		} else {
			a.Memory = NewFileSessionStore("")
		}
	}

	var (
		sess    *Session
		history []StoredMessage
		err     error
	)
	if strings.TrimSpace(sessionID) == "" {
		sess, history, err = a.Memory.LoadOrCreateCurrentSession(workDir)
	} else {
		sess, history, err = a.Memory.LoadSessionForWorkDir(workDir, sessionID)
	}
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, errors.New("session unavailable")
	}

	// Normalize "continue" turns early so token estimation reflects the real prompt.
	normalized := input
	if n, ok := normalizeContinueTurn(input); ok {
		normalized = n
	}

	ctxTokens := a.effectiveContextWindowTokens()
	if ctxTokens <= 0 {
		return sess, nil
	}
	threshold := int(0.90 * float64(ctxTokens))
	if threshold <= 0 {
		return sess, nil
	}

	estimate := a.estimateSessionPromptTokens(mode, sess.ContextSummary, history, normalized)
	if estimate < threshold {
		return sess, nil
	}

	if progress != nil {
		progress(ProgressEvent{
			Kind: "thinking",
			Text: "Compacting old session context before request",
			At:   time.Now(),
		})
	}

	// Generate a compact summary and roll to a child session.
	summary, sumErr := a.compactSessionContext(ctx, mode, normalized, history, strings.TrimSpace(sess.ContextSummary))
	if sumErr != nil || strings.TrimSpace(summary) == "" {
		// If compaction fails, keep the current session.
		if progress != nil {
			progress(ProgressEvent{
				Kind: "warn",
				Text: "Context compaction failed; continuing without compaction",
				At:   time.Now(),
			})
		}
		return sess, nil
	}

	child, childErr := a.Memory.CreateChildSession(workDir, sess.ID, summary)
	if childErr != nil || child == nil {
		if progress != nil {
			progress(ProgressEvent{
				Kind: "warn",
				Text: "Unable to create compacted child session; continuing without compaction",
				At:   time.Now(),
			})
		}
		return sess, nil
	}

	if progress != nil {
		progress(ProgressEvent{
			Kind: "thinking",
			Text: "Context compacted, continuing request",
			At:   time.Now(),
		})
	}
	return child, nil
}

func trimForDisplay(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "\n...[truncated]..."
}

const (
	defaultSessionPromptSoftLimitChars = 50000
	sessionPromptHistoryLimit          = 20
	sessionPromptHistoryLimitSummary   = 8
	sessionPromptHistoryLimitCompacted = 6
	sessionSummaryMaxChars             = 12000
	sessionCompactionMaxMessages       = 80
	sessionCompactionPerMessageChars   = 700
	sessionCompactionTranscriptChars   = 22000
	sessionCompactionSummaryMaxWords   = 400
)

func positiveIntEnv(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func normalizeOrchestrateIntEnv(name string, fallback int, minVal int, maxVal int) int {
	value := positiveIntEnv(name, fallback)
	if value < minVal {
		value = minVal
	}
	if maxVal > 0 && value > maxVal {
		value = maxVal
	}
	return value
}

func orchestrateMetricsEnabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(orchestrateMetricsEnv)))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func orchestrateLLMDecomposeEnabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("EAI_ORCHESTRATE_LLM_DECOMPOSE")))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func orchestrateShardTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv(orchestrateShardTimeoutEnv))
	if raw == "" {
		return 0
	}
	sec, err := strconv.Atoi(raw)
	if err != nil || sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}

func (a *Application) orchestrateTaskConcurrencyBudget(requested int) int {
	if requested <= 0 {
		requested = 1
	}
	if requested > defaultOrchestratePerTaskParallelism {
		requested = defaultOrchestratePerTaskParallelism
	}

	maxPanesPerTask := normalizeOrchestrateIntEnv(orchestrateMaxPanesPerTaskEnv, defaultOrchestrateMaxPanesPerTask, 1, 50)
	if maxPanesPerTask < 1 {
		maxPanesPerTask = 1
	}

	shards := requested * maxPanesPerTask
	hardCap := normalizeOrchestrateIntEnv(orchestrateMaxShardsEnv, defaultOrchestrateMaxShards, 1, 100)
	if hardCap > 0 && shards > hardCap {
		shards = hardCap
	}
	if shards <= 0 {
		shards = 1
	}
	return shards
}

func (a *Application) orchestrateActivePaneCap() int {
	capValue := normalizeOrchestrateIntEnv(orchestrateActivePanesEnv, defaultOrchestrateMaxActivePanes, 1, 50)
	if a.Config.MaxParallelAgents > 0 && capValue > a.Config.MaxParallelAgents {
		capValue = a.Config.MaxParallelAgents
	}
	if capValue <= 0 {
		return 1
	}
	return capValue
}

func sessionPromptSoftLimitChars() int {
	return positiveIntEnv("EAI_SESSION_PROMPT_SOFT_LIMIT_CHARS", defaultSessionPromptSoftLimitChars)
}

func (a *Application) effectiveContextWindowTokens() int {
	if v := strings.TrimSpace(os.Getenv("EAI_CONTEXT_WINDOW_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if a != nil && a.Config.ContextWindowTokens > 0 {
		return a.Config.ContextWindowTokens
	}
	if a != nil {
		if n, ok := LookupContextWindowTokens(a.Config.Model); ok && n > 0 {
			return n
		}
	}
	if n, ok := LookupContextWindowTokens(DefaultModel); ok && n > 0 {
		return n
	}
	return 0
}

func truncateEllipsis(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "..."
}

func trimToMaxWords(s string, maxWords int) string {
	s = strings.TrimSpace(s)
	if s == "" || maxWords <= 0 {
		return ""
	}
	words := strings.Fields(s)
	if len(words) <= maxWords {
		return s
	}
	return strings.TrimSpace(strings.Join(words[:maxWords], " "))
}

func sessionHistoryForPrompt(history []StoredMessage, max int) []StoredMessage {
	if len(history) == 0 {
		return nil
	}
	filtered := make([]StoredMessage, 0, len(history))
	for _, m := range history {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		filtered = append(filtered, m)
	}
	if max > 0 && len(filtered) > max {
		filtered = filtered[len(filtered)-max:]
	}
	return filtered
}

func dropTrailingUserEcho(history []StoredMessage, latestInput string) []StoredMessage {
	if len(history) == 0 {
		return nil
	}
	last := history[len(history)-1]
	if strings.EqualFold(strings.TrimSpace(last.Role), "user") &&
		strings.TrimSpace(last.Content) == strings.TrimSpace(latestInput) {
		return history[:len(history)-1]
	}
	return history
}

func buildAgentMemoryPrelude(summary string, history []StoredMessage) string {
	summary = strings.TrimSpace(summary)
	history = sessionHistoryForPrompt(history, 12)
	if summary == "" && len(history) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Conversation context (most recent turns). Use for continuity only.\n\n")

	if summary != "" {
		b.WriteString("Session summary:\n")
		b.WriteString(truncateEllipsis(summary, 2500))
		b.WriteString("\n\n")
	}

	if len(history) > 0 {
		b.WriteString("Most recent messages:\n")
		for _, m := range history {
			role := strings.ToUpper(strings.TrimSpace(m.Role))
			if strings.EqualFold(role, "ASSISTANT") {
				role = "EAI"
			}
			txt := strings.TrimSpace(m.Content)
			if txt == "" || role == "" {
				continue
			}
			txt = strings.ReplaceAll(txt, "\n", " ")
			txt = strings.Join(strings.Fields(txt), " ")
			txt = truncateEllipsis(txt, 400)
			b.WriteString(role)
			b.WriteString(": ")
			b.WriteString(txt)
			b.WriteString("\n")
		}
	}

	return strings.TrimSpace(b.String())
}

func buildSessionChatPrompt(systemPrompt, summary string, history []StoredMessage, input string, maxHistory int) string {
	var b strings.Builder
	b.WriteString("[SYSTEM]\n")
	b.WriteString(systemPrompt)
	b.WriteString("\n\n")
	if strings.TrimSpace(summary) != "" {
		b.WriteString("[SYSTEM]\n")
		b.WriteString("Session summary from previous turns:\n")
		b.WriteString(strings.TrimSpace(summary))
		b.WriteString("\n\n")
	}
	for _, m := range sessionHistoryForPrompt(history, maxHistory) {
		switch strings.ToLower(strings.TrimSpace(m.Role)) {
		case "user":
			b.WriteString("[USER]\n")
			b.WriteString(m.Content)
			b.WriteString("\n\n")
		case "assistant":
			b.WriteString("[ASSISTANT]\n")
			b.WriteString(m.Content)
			b.WriteString("\n\n")
		}
	}
	b.WriteString("[USER]\n")
	b.WriteString(input)
	b.WriteString("\n")
	return b.String()
}

func isContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(strings.TrimSpace(err.Error()))
	if s == "" {
		return false
	}
	needles := []string{
		"context length",
		"context window",
		"maximum context",
		"max context",
		"prompt is too long",
		"input is too long",
		"too many tokens",
		"token limit",
		"maximum tokens",
		"input tokens",
		"request too large",
	}
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func buildSessionCompactionPrompt(mode Mode, existingSummary, transcript, nextInput string) string {
	var b strings.Builder
	b.WriteString("[SYSTEM]\n")
	b.WriteString("You compress conversation context for a coding CLI agent.\n")
	b.WriteString("Return concise Markdown only (no code fences), max 400 words.\n")
	b.WriteString("Use exactly these sections:\n")
	b.WriteString("## Goal\n## Constraints\n## Decisions\n## Progress\n## Open Tasks\n## Relevant Files\n\n")
	b.WriteString("Prioritize actionable continuity so another agent can continue immediately.\n")
	b.WriteString(fmt.Sprintf("Current mode: %s\n\n", mode))
	b.WriteString("[USER]\n")
	if strings.TrimSpace(existingSummary) != "" {
		b.WriteString("Existing session summary:\n")
		b.WriteString(existingSummary)
		b.WriteString("\n\n")
	}
	b.WriteString("Recent conversation transcript:\n")
	b.WriteString(transcript)
	b.WriteString("\n\n")
	b.WriteString("Latest user request:\n")
	b.WriteString(nextInput)
	b.WriteString("\n\nGenerate the updated context summary now.")
	return b.String()
}

func buildCompactionTranscript(history []StoredMessage) string {
	msgs := sessionHistoryForPrompt(history, sessionCompactionMaxMessages)
	if len(msgs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, m := range msgs {
		role := strings.ToUpper(strings.TrimSpace(m.Role))
		if role == "" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		content = strings.Join(strings.Fields(content), " ")
		content = truncateEllipsis(content, sessionCompactionPerMessageChars)
		line := fmt.Sprintf("[%s] %s\n", role, content)
		if b.Len()+len(line) > sessionCompactionTranscriptChars {
			break
		}
		b.WriteString(line)
	}
	return strings.TrimSpace(b.String())
}

func heuristicContextSummary(existingSummary string, history []StoredMessage, latestInput string) string {
	var goals []string
	var progress []string
	for i := len(history) - 1; i >= 0 && (len(goals) < 4 || len(progress) < 4); i-- {
		msg := history[i]
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		content := truncateEllipsis(strings.Join(strings.Fields(msg.Content), " "), 220)
		if content == "" {
			continue
		}
		if role == "user" && len(goals) < 4 {
			goals = append(goals, content)
		}
		if role == "assistant" && len(progress) < 4 {
			progress = append(progress, content)
		}
	}
	reverse := func(items []string) {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}
	reverse(goals)
	reverse(progress)

	var b strings.Builder
	b.WriteString("## Goal\n")
	if len(goals) == 0 {
		b.WriteString("- Continue the active session task.\n")
	} else {
		for _, g := range goals {
			b.WriteString("- ")
			b.WriteString(g)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Constraints\n")
	b.WriteString("- Preserve user instructions and current mode behavior.\n")
	b.WriteString("\n## Decisions\n")
	if strings.TrimSpace(existingSummary) != "" {
		b.WriteString("- Existing summary was already present and carried forward.\n")
	} else {
		b.WriteString("- Generated fallback summary because full model compaction was unavailable.\n")
	}
	b.WriteString("\n## Progress\n")
	if len(progress) == 0 {
		b.WriteString("- No assistant progress captured yet.\n")
	} else {
		for _, p := range progress {
			b.WriteString("- ")
			b.WriteString(p)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n## Open Tasks\n")
	b.WriteString("- Address latest request: ")
	b.WriteString(truncateEllipsis(strings.Join(strings.Fields(latestInput), " "), 220))
	b.WriteString("\n\n## Relevant Files\n- (To be discovered from subsequent tool steps)\n")
	out := truncateEllipsis(b.String(), sessionSummaryMaxChars)
	return trimToMaxWords(out, sessionCompactionSummaryMaxWords)
}

func (a *Application) compactSessionContext(ctx context.Context, mode Mode, input string, history []StoredMessage, existingSummary string) (string, error) {
	transcript := buildCompactionTranscript(history)
	if strings.TrimSpace(transcript) == "" && strings.TrimSpace(existingSummary) != "" {
		out := truncateEllipsis(existingSummary, sessionSummaryMaxChars)
		return trimToMaxWords(out, sessionCompactionSummaryMaxWords), nil
	}
	prompt := buildSessionCompactionPrompt(mode, existingSummary, transcript, input)
	out, err := a.Client.Complete(ctx, prompt)
	if err != nil {
		fallback := heuristicContextSummary(existingSummary, history, input)
		if strings.TrimSpace(fallback) != "" {
			return fallback, nil
		}
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		fallback := heuristicContextSummary(existingSummary, history, input)
		if strings.TrimSpace(fallback) != "" {
			return fallback, nil
		}
		return "", errors.New("empty context summary")
	}

	// Enforce max word budget (retry once, then hard-trim).
	if len(strings.Fields(out)) > sessionCompactionSummaryMaxWords {
		shorten := strings.Builder{}
		shorten.WriteString("[SYSTEM]\n")
		shorten.WriteString("Shorten the following session summary to <= 400 words.\n")
		shorten.WriteString("Keep the same Markdown section headings.\n")
		shorten.WriteString("Return Markdown only (no code fences).\n\n")
		shorten.WriteString("[USER]\n")
		shorten.WriteString(out)
		if shorter, serr := a.Client.Complete(ctx, shorten.String()); serr == nil {
			shorter = strings.TrimSpace(shorter)
			if shorter != "" {
				out = shorter
			}
		}
	}

	out = truncateEllipsis(out, sessionSummaryMaxChars)
	out = trimToMaxWords(out, sessionCompactionSummaryMaxWords)
	return out, nil
}

func completeChatWithProgress(
	ctx context.Context,
	client *MinimaxClient,
	prompt string,
	progress func(ProgressEvent),
	emitThinking bool,
) (string, error) {
	if emitThinking && progress != nil {
		progress(ProgressEvent{
			Kind: "thinking",
			Text: "thinking...",
		})
	}
	if client == nil {
		return "", errors.New("client unavailable")
	}
	return client.CompleteWithObserver(ctx, prompt, func(reasoning string) {
		reasoning = strings.TrimSpace(reasoning)
		if reasoning == "" || progress == nil {
			return
		}
		progress(ProgressEvent{
			Kind: "reasoning",
			Text: reasoning,
		})
	})
}

func looksLikeHTMLDocumentText(s string) bool {
	sl := strings.ToLower(s)
	if strings.Contains(sl, "<!doctype html") {
		return true
	}
	// A complete HTML doc usually has <html ...> and </html>.
	return strings.Contains(sl, "<html") && strings.Contains(sl, "</html>")
}

func extractHTMLDocumentText(s string) (string, bool) {
	trim := strings.TrimSpace(s)
	if trim == "" {
		return "", false
	}

	// Prefer fenced ```html blocks if present.
	lower := strings.ToLower(trim)
	if i := strings.Index(lower, "```html"); i >= 0 {
		rest := trim[i+len("```html"):]
		// Strip optional leading newline.
		rest = strings.TrimLeft(rest, "\n\r\t ")
		if j := strings.Index(rest, "```"); j >= 0 {
			cand := strings.TrimSpace(rest[:j])
			if looksLikeHTMLDocumentText(cand) {
				return cand, true
			}
		}
	}

	// Otherwise, extract from doctype/html tags.
	if !looksLikeHTMLDocumentText(trim) {
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

func looksLikeWebsiteHTMLRequest(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return false
	}
	if !strings.Contains(s, "html") {
		return false
	}
	if !(strings.Contains(s, "website") ||
		strings.Contains(s, "web site") ||
		strings.Contains(s, "webpage") ||
		strings.Contains(s, "web page") ||
		strings.Contains(s, "landing page") ||
		strings.Contains(s, "site")) {
		return false
	}

	// Keep this narrow: this helper is used to decide whether we should force writing `index.html`.
	for _, v := range []string{"create", "build", "make", "generate", "write"} {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

func looksLikeHTMLStyleEditRequest(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return false
	}
	if !strings.Contains(s, "html") {
		return false
	}
	if !(strings.Contains(s, "style") || strings.Contains(s, "styles") || strings.Contains(s, "css")) {
		return false
	}
	return hasActionVerb(s)
}

func hasActionVerb(s string) bool {
	// Explicit "do something" verbs.
	actionVerbs := []string{
		"create", "make", "generate", "write", "save",
		"add",
		"edit", "modify", "update", "change",
		"delete", "remove", "move", "rename",
		"continue", "resume",
		"develop", "implement",
		"analyze", "inspect", "review",
		"fix", "repair", "refactor",
		"install", "build", "run", "execute", "test",
	}
	for _, v := range actionVerbs {
		if strings.Contains(s, v+" ") || strings.HasPrefix(s, v) {
			return true
		}
	}
	return false
}

func containsLikelyLocalPath(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	// Common absolute path anchors.
	if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") {
		return true
	}
	if strings.Contains(s, "/home/") || strings.Contains(s, "/users/") || strings.Contains(s, "/documents/") || strings.Contains(s, "/desktop/") {
		return true
	}
	// Tilde + relative forms.
	if strings.Contains(s, "~/") || strings.Contains(s, "./") || strings.Contains(s, "../") {
		return true
	}
	// Windows-ish.
	if strings.Contains(s, "c:\\") || strings.Contains(s, "d:\\") {
		return true
	}
	return false
}

func looksActionableForCreate(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return false
	}

	if hasActionVerb(s) {
		return true
	}

	// If the user referenced local paths, treat it as actionable: they likely want file ops.
	if containsLikelyLocalPath(s) {
		return true
	}

	// Common actionable intents.
	if looksLikeListFilesRequest(input) || looksLikeWebsiteHTMLRequest(input) {
		return true
	}

	return false
}

func localListDir(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "(empty)", nil
	}
	return strings.Join(names, "\n"), nil
}

func looksLikeListFilesRequest(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return false
	}
	// If the user specified an explicit path, don't guess; let the model/tool-mode handle it.
	// Examples: "/etc", "~/proj", "./src".
	if strings.Contains(s, "/") || strings.Contains(s, "~") {
		return false
	}
	if strings.Contains(s, " in ") && !(strings.Contains(s, "this") || strings.Contains(s, "current") || strings.Contains(s, "here")) {
		return false
	}
	// Keep this intentionally narrow to avoid surprising behavior.
	if strings.Contains(s, "list") && (strings.Contains(s, "files") || strings.Contains(s, "file")) {
		return true
	}
	if strings.Contains(s, "show") && (strings.Contains(s, "files") || strings.Contains(s, "folder") || strings.Contains(s, "directory")) {
		return true
	}
	if strings.Contains(s, "what") && strings.Contains(s, "in") && (strings.Contains(s, "folder") || strings.Contains(s, "directory")) {
		return true
	}
	return false
}

func looksLikeTrivialChatTurn(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return true
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	// Keep this intentionally narrow so we don't misclassify short real tasks.
	if len(s) > 48 {
		return false
	}
	s = strings.Trim(s, " \t\r\n.!?…")
	s = strings.Join(strings.Fields(s), " ")
	switch s {
	case "hi", "hi there",
		"hello", "hello there",
		"hey", "hey there",
		"yo", "sup",
		"good morning", "good afternoon", "good evening",
		"thanks", "thank you", "thx", "ty",
		"ok", "okay", "cool", "nice",
		"ping", "test":
		return true
	default:
		return false
	}
}

func trivialChatResponseForMode(mode Mode) string {
	switch mode {
	case ModePlan:
		return "hi. tell me what you want to plan."
	default:
		return "hi. tell me what you want me to do."
	}
}

func isGenericCompletionText(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "task_completed", "task completed", "done", "completed", "edit finished", "step finished", "command finished":
		return true
	}
	return false
}

func stripTaskCompletedSentinel(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if strings.EqualFold(last, "TASK_COMPLETED") {
			lines = lines[:len(lines)-1]
			continue
		}
		break
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func renderAgentStateForChat(state *AgentState) string {
	if state == nil {
		return "error: no agent state"
	}

	// Friendly summary when files were created/edited/appended.
	type fileOp struct {
		verb string
		path string
	}
	var ops []fileOp
	for _, r := range state.Results {
		if !r.Success {
			continue
		}
		out := strings.TrimSpace(r.Output)
		outLower := strings.ToLower(out)
		switch {
		case strings.HasPrefix(outLower, "file written:"):
			ops = append(ops, fileOp{verb: "Created", path: strings.TrimSpace(out[len("File written:"):])})
		case strings.HasPrefix(outLower, "file edited:"):
			ops = append(ops, fileOp{verb: "Updated", path: strings.TrimSpace(out[len("File edited:"):])})
		case strings.HasPrefix(outLower, "file appended:"):
			ops = append(ops, fileOp{verb: "Updated", path: strings.TrimSpace(out[len("File appended:"):])})
		case strings.HasPrefix(outLower, "file patched:"):
			ops = append(ops, fileOp{verb: "Updated", path: strings.TrimSpace(out[len("File patched:"):])})
		}
	}
	if len(ops) > 0 {
		// De-dupe by verb+path while preserving order.
		seen := make(map[string]bool)
		var lines []string
		for _, op := range ops {
			p := strings.TrimSpace(op.path)
			if p == "" {
				continue
			}
			// Make VSCode-terminal clickable: absolute path + :1.
			if !filepath.IsAbs(p) {
				if abs, err := filepath.Abs(p); err == nil {
					p = abs
				}
			}
			key := op.verb + "\x00" + p
			if seen[key] {
				continue
			}
			seen[key] = true
			lines = append(lines, fmt.Sprintf("%s %s:1", op.verb, p))
		}
		final := stripTaskCompletedSentinel(state.FinalOutput)
		final = strings.TrimSpace(final)
		finalLower := strings.ToLower(final)
		if final != "" && !isGenericCompletionText(final) && !strings.Contains(finalLower, "agent_step_limit_reached") && !strings.Contains(finalLower, "did not return any tool calls") {
			// Prefer the model's final report when provided, but append clickable file refs.
			var b strings.Builder
			b.WriteString(final)
			if len(lines) > 0 {
				b.WriteString("\n\nWhat I updated:\n")
				for _, line := range lines {
					b.WriteString("- ")
					b.WriteString(line)
					b.WriteString("\n")
				}
			}
			return strings.TrimSpace(b.String())
		}

		if len(lines) > 0 {
			var b strings.Builder
			b.WriteString("Done. I finished the requested changes.\n\n")
			b.WriteString("What I updated:\n")
			for _, line := range lines {
				b.WriteString("- ")
				b.WriteString(line)
				b.WriteString("\n")
			}
			return strings.TrimSpace(b.String())
		}
	}

	// Prefer showing the most recent non-empty tool output.
	for i := len(state.Results) - 1; i >= 0; i-- {
		out := strings.TrimSpace(state.Results[i].Output)
		if out != "" {
			out = trimForDisplay(out, 32768)
			return "Done. Here is the latest output:\n\n```text\n" + out + "\n```"
		}
	}

	// Fall back to a concise status message.
	if state.FinalOutput != "" && strings.TrimSpace(state.FinalOutput) != "" {
		final := stripTaskCompletedSentinel(state.FinalOutput)
		final = strings.TrimSpace(final)
		lower := strings.ToLower(final)
		if strings.Contains(lower, "agent_step_limit_reached") {
			return "I paused due to an internal step limit. You can retry and I will continue from the latest state."
		}
		if strings.Contains(lower, "did not return any tool calls") {
			return "I couldn't complete this request because the model stopped returning actionable tool steps. I can retry with a stricter execution prompt."
		}
		if isGenericCompletionText(final) {
			return "Done. The task completed successfully."
		}
		return "Done. " + final
	}
	if len(state.Results) > 0 {
		last := state.Results[len(state.Results)-1]
		if last.Error != "" {
			return fmt.Sprintf("error: %s", strings.TrimSpace(last.Error))
		}
	}
	return "no output"
}

func (a *Application) ExecuteChat(ctx context.Context, mode Mode, input string) (string, error) {
	return a.ExecuteChatWithProgress(ctx, mode, input, nil)
}

func (a *Application) ExecuteChatWithProgress(ctx context.Context, mode Mode, input string, progress func(string)) (string, error) {
	var progressEvents func(ProgressEvent)
	if progress != nil {
		progressEvents = func(ev ProgressEvent) {
			if strings.TrimSpace(ev.Text) != "" {
				progress(ev.Text)
			}
		}
	}
	return a.ExecuteChatWithProgressEvents(ctx, mode, input, progressEvents)
}

func (a *Application) ExecuteChatWithProgressEvents(ctx context.Context, mode Mode, input string, progress func(ProgressEvent)) (string, error) {
	origInput := input

	// Local fastpath: scaffold a React site in create mode without requiring an API key.
	if mode == ModeCreate {
		if out, ok, err := tryLocalReactScaffold(input); ok {
			return out, err
		}
	}

	// Prefer English routing/heuristics and rely on a translation pass for non-English inputs.
	// For text-only modes, keep the original user text for the completion, but use translated text for
	// actionable routing checks.
	routingInput := input
	translatedInput := ""
	translatedOK := false
	if translated, ok, err := translateToEnglish(ctx, a.Client, input); err == nil && ok {
		routingInput = translated
		translatedInput = translated
		translatedOK = true
		if mode == ModePlan || IsToolMode(mode) {
			input = translated
		}
		if progress != nil {
			progress(ProgressEvent{
				Kind: "thinking",
				Text: "Auto-translated request to English for execution",
				At:   time.Now(),
			})
		}
	}

	trivialProbe := routingInput
	if _, req, _, ok := splitToolSessionContextWrapper(trivialProbe); ok {
		trivialProbe = strings.TrimSpace(req)
	}
	// Avoid spinning up the plan/tool agents for trivial greetings/smalltalk.
	if (mode == ModePlan || IsToolMode(mode)) && looksLikeTrivialChatTurn(trivialProbe) {
		return trivialChatResponseForMode(mode), nil
	}

	// If the default orchestrate mode receives an actionable request, run the tool agent instead so
	// permissions are enforced and files/commands can be executed.
	if mode == ModeOrchestrate && looksActionableForCreate(routingInput) {
		toolInput := input
		if translatedOK {
			toolInput = translatedInput
		}
		return a.ExecuteAgentTaskWithProgressEvents(ctx, toolInput, progress)
	}

	// Plan mode: use a read-only discovery agent that always returns a plan.
	if mode == ModePlan {
		return a.executePlanModeWithProgressEvents(ctx, input, progress)
	}

	// Orchestrate mode splits a task into independent subtasks and runs them
	// as concurrent LLM calls. Keep this before general tool execution paths.
	if mode == ModeOrchestrate {
		if a.Client != nil && a.Client.APIKey == "" && a.Client.BaseURL != "mock://" {
			return "No API key configured. Run `/connect` in the TUI or set `EAI_API_KEY`/`MINIMAX_API_KEY` (and optionally `EAI_MODEL`, `EAI_BASE_URL`).", nil
		}
		if progress != nil {
			shards := a.orchestrateTaskConcurrencyBudget(2)
			workers := a.orchestrateActivePaneCap()
			progress(ProgressEvent{
				Kind: "thinking",
				Text: fmt.Sprintf("Orchestrate: up to %d shard(s), %d concurrent", shards, workers),
			})
		}
		return a.ExecuteOrchestrateWithProgressEvents(ctx, mode, input, 2, progress)
	}

	// Text-only modes: if user asks to do something, prompt them to switch to create mode.
	if !IsToolMode(mode) && mode != ModePlan && looksActionableForCreate(routingInput) {
		return "This looks like an action request. Switch to `create` mode (shift+tab) to create files / run commands, then send the same request again.", nil
	}

	// In tool-capable modes, actually execute tool calls (TUI "do"/"code").
	if IsToolMode(mode) {
		// Fast local helper in create mode: list files without needing an API call.
		if mode == ModeCreate && looksLikeListFilesRequest(input) {
			out, err := localListDir(".")
			if err != nil {
				return "", err
			}
			return out, nil
		}

		// Avoid confusing agent-loop failures when API key is missing.
		if a.Client != nil && a.Client.APIKey == "" && a.Client.BaseURL != "mock://" {
			return "No API key configured. Run `/connect` in the TUI or set `EAI_API_KEY`/`MINIMAX_API_KEY` (and optionally `EAI_MODEL`, `EAI_BASE_URL`).", nil
		}

		stateDir := filepath.Join(os.TempDir(), "cli-agent", "states")
		agent := NewAgentLoop(a.Client, 12, stateDir, a.Logger)
		agent.Relentless = true
		agent.PermissionsMode = a.Config.Permissions
		if wd, err := os.Getwd(); err == nil && wd != "" {
			agent.WorkDir = wd
		}
		if progress != nil {
			agent.Progress = progress
		}
		// Keep interactive tool runs responsive; the caller may also bound ctx.
		toolCtx, cancel := context.WithTimeout(ctx, 9*time.Minute)
		defer cancel()

		task := input
		if looksLikeWebsiteHTMLRequest(input) {
			task = input + "\n\nCRITICAL: Write the final website to index.html in the location the user requested (if any), otherwise write index.html in the current working directory using the write_file tool. Do NOT run `ls -la` as a primary output; just write the file."
		}

		state, err := agent.Execute(toolCtx, task)
		if err != nil {
			return "", err
		}
		return renderAgentStateForChat(state), nil
	}

	// Interactive TUI chat should produce human-readable output, not raw tool-call JSON.
	chatInput := origInput
	if translatedOK {
		chatInput = translatedInput
	}
	prompt := a.Prompter.BuildChat(mode, chatInput, a.Config.ChatVerbosity)
	out, err := completeChatWithProgress(ctx, a.Client, prompt, progress, true)
	if err != nil {
		if errors.Is(err, ErrAPIKeyRequired) {
			return "No API key configured. Run `/connect` in the TUI or set `EAI_API_KEY`/`MINIMAX_API_KEY` (and optionally `EAI_MODEL`, `EAI_BASE_URL`).", nil
		}
		return "", err
	}

	trim := strings.TrimSpace(out)
	// Safety: if the model (or mock) returns a tool-call JSON blob, don't show it in chat.
	if strings.HasPrefix(trim, "{") && strings.Contains(trim, "\"tool_calls\"") {
		return "This chat UI only shows text replies and does not execute tool calls. Use `eai agent \"...\"` for tool-driven tasks, or ask in plain language for a text-only answer.", nil
	}

	// If the model returns a full HTML document for a "create website using HTML" request,
	// write it to index.html instead of dumping it into the chat.
	if looksLikeWebsiteHTMLRequest(routingInput) {
		if htmlDoc, ok := extractHTMLDocumentText(out); ok {
			if err := os.WriteFile("index.html", []byte(htmlDoc), 0644); err == nil {
				return "Wrote `index.html` in the current directory.", nil
			}
		}
	}

	return out, nil
}

// ExecuteAgentTaskWithProgressEvents runs the same tool-driven agent loop as `eai agent`,
// but returns a chat-friendly final text. This is used by the TUI "chat" so we can keep
// the agent's prompt/behavior identical to Terminal-Bench runs while streaming progress.
func (a *Application) ExecuteAgentTaskWithProgressEvents(ctx context.Context, task string, progress func(ProgressEvent)) (string, error) {
	return a.ExecuteAgentTaskInSessionWithProgressEvents(ctx, "", "", task, progress, nil)
}

func toolCompanionsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("EAI_TOOL_COMPANIONS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func readOnlyCompanionSystemPrompt(workDir string) string {
	if workDir == "" {
		workDir = "/app"
	}

	// Keep this short: companions are meant to gather signal quickly and report back.
	return fmt.Sprintf(`You are EAI Companion, a read-only discovery agent operating in a Linux environment.

WORKDIR: %s

MODE: DISCOVER (READ-ONLY)
- Your job is to gather facts quickly (files, snippets, commands outputs) to help the main agent complete the task.
- You MUST NOT modify files. Do NOT use write/edit/append/patch tools.
- exec is allowed only for read-only commands (listing/searching/testing). Do not run destructive commands.

## Response Rules (CRITICAL)
- Every response must be exactly ONE of:
  1) A single JSON tool call, and nothing else:
     {"tool":"...", "args":{...}}
  2) A final report (plain text) that ends with a single line:
     TASK_COMPLETED

## Tools available
- exec: {"tool":"exec","args":{"command":"...", "timeout":600, "cwd":"optional"}}
- list_dir: {"tool":"list_dir","args":{"path":"..."}}
- read_file: {"tool":"read_file","args":{"path":"..."}}
- grep: {"tool":"grep","args":{"pattern":"...","path":"...","recursive":true}}
- search_files: {"tool":"search_files","args":{"pattern":"*.ext","path":"..."}}

In the final report include:
- Relevant files (paths)
- Key observations (bullet points)
- Suggested next concrete steps (commands/tools)`, workDir)
}

func companionTools() []Tool {
	allow := map[string]bool{
		"exec":         true,
		"read_file":    true,
		"list_dir":     true,
		"grep":         true,
		"search_files": true,
	}
	var out []Tool
	for _, t := range DefaultTools() {
		if allow[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

func companionExecAllowed(command string) (bool, string) {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return false, "missing command"
	}
	cmd = strings.ReplaceAll(cmd, "\r", " ")
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	cmd = strings.Join(strings.Fields(cmd), " ")

	// Block obvious mutation patterns early.
	if strings.Contains(cmd, ">") || strings.Contains(cmd, ">>") {
		return false, "blocked: shell redirection"
	}
	if strings.Contains(cmd, " tee ") || strings.HasPrefix(cmd, "tee ") || strings.Contains(cmd, "|tee") || strings.Contains(cmd, "| tee") {
		return false, "blocked: tee writes output"
	}
	if strings.Contains(cmd, "sed -i") || strings.Contains(cmd, "perl -i") {
		return false, "blocked: in-place edits"
	}
	if strings.Contains(cmd, "rm ") || strings.Contains(cmd, "mv ") || strings.Contains(cmd, "cp ") || strings.Contains(cmd, "mkdir ") || strings.Contains(cmd, "touch ") {
		return false, "blocked: filesystem mutation"
	}

	// Reuse the existing "dangerous" heuristic as a deny list.
	if execCommandNeedsApproval(command) {
		return false, "blocked: risky command"
	}

	allowedPrefixes := []string{
		"ls", "rg ", "grep ", "cat ", "sed -n", "head ", "tail ", "find ", "wc ",
		"git status", "git diff", "git log", "git show",
		"go test", "go list", "go env",
		"npm test", "pnpm test", "yarn test",
		"pytest", "python -m pytest",
	}
	for _, p := range allowedPrefixes {
		if strings.HasPrefix(cmd, p) {
			return true, ""
		}
	}
	return false, "blocked: exec restricted for companions"
}

func buildCompanionDiscoveryTasks(task string, n int) []string {
	task = strings.TrimSpace(task)
	if n <= 1 {
		return []string{task}
	}
	focus := []string{
		"Find relevant files/entrypoints and summarize current behavior.",
		"Identify likely failure points/edge cases and where to change code.",
		"Propose concrete next tool steps (commands + files to edit) to complete the task.",
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		f := focus[i%len(focus)]
		out = append(out, "READ-ONLY DISCOVERY COMPANION\n"+
			"Focus: "+f+"\n\nTask:\n"+task+
			"\n\nConstraints:\n- Do not modify files.\n- Use tools to inspect; keep outputs small.\n")
	}
	return out
}

// ExecuteAgentTaskInSessionWithProgressEvents runs the tool agent with lightweight
// session memory (recent turns + optional summary) injected ahead of the current task.
// This is only used by the interactive TUI and does not affect `eai agent`.
func (a *Application) ExecuteAgentTaskInSessionWithProgressEvents(ctx context.Context, sessionID string, workDir string, task string, progress func(ProgressEvent), decisions <-chan PermissionDecision) (string, error) {
	return a.executeAgentTaskInSessionWithProgressEventsAndPrelude(ctx, sessionID, workDir, task, progress, decisions, "")
}

func (a *Application) executeAgentTaskInSessionWithProgressEventsAndPrelude(ctx context.Context, sessionID string, workDir string, task string, progress func(ProgressEvent), decisions <-chan PermissionDecision, extraPrelude string) (string, error) {
	if a == nil {
		return "", fmt.Errorf("application is nil")
	}

	// Avoid confusing agent-loop failures when API key is missing.
	if a.Client != nil && a.Client.APIKey == "" && a.Client.BaseURL != "mock://" {
		return "No API key configured. Run `/connect` in the TUI or set `EAI_API_KEY`/`MINIMAX_API_KEY` (and optionally `EAI_MODEL`, `EAI_BASE_URL`).", nil
	}

	// Lightweight memory injection (recent turns + optional summary).
	var (
		history        []StoredMessage
		sessionSummary string
	)
	if a.Memory != nil && strings.TrimSpace(sessionID) != "" {
		if msgs, err := a.Memory.LoadMessages(sessionID); err == nil {
			history = msgs
		}
		if sess, _, err := a.Memory.LoadSessionForWorkDir(workDir, sessionID); err == nil && sess != nil {
			sessionSummary = strings.TrimSpace(sess.ContextSummary)
		}
	}
	history = dropTrailingUserEcho(history, task)
	prelude := buildAgentMemoryPrelude(sessionSummary, history)

	stateDir := filepath.Join(os.TempDir(), "cli-agent", "states")
	agent := NewAgentLoop(a.Client, 30, stateDir, a.Logger)
	agent.Relentless = true
	agent.PermissionsMode = a.Config.Permissions
	agent.PermissionDecisions = decisions
	if wd, err := os.Getwd(); err == nil && wd != "" {
		agent.WorkDir = wd
	}
	if progress != nil {
		agent.Progress = progress
	}
	var preludes []AgentMessage
	if strings.TrimSpace(prelude) != "" {
		preludes = append(preludes, AgentMessage{
			Role:      "user",
			Content:   prelude,
			Timestamp: time.Now(),
		})
	}
	if strings.TrimSpace(extraPrelude) != "" {
		preludes = append(preludes, AgentMessage{
			Role:      "user",
			Content:   strings.TrimSpace(extraPrelude),
			Timestamp: time.Now(),
		})
	}
	if len(preludes) > 0 {
		agent.PreludeMessages = preludes
	}

	taskInput := task
	if looksLikeWebsiteHTMLRequest(taskInput) {
		taskInput = taskInput + "\n\nCRITICAL: Write the final website to index.html in the location the user requested (if any), otherwise write index.html in the current working directory using the write_file tool. Do NOT run `ls -la` as a primary output; just write the file."
	}
	if looksLikeHTMLStyleEditRequest(taskInput) && !strings.Contains(taskInput, "/") && !strings.Contains(taskInput, "~") {
		taskInput = taskInput + "\n\nIf the user did not specify a file path, assume the target is `index.html` in the current working directory and update it using the write_file tool. Do not ask the user to paste code; inspect the workspace and proceed."
	}

	state, err := agent.Execute(ctx, taskInput)
	if err != nil {
		return "", err
	}

	// Prefer showing the model's final output verbatim (without the sentinel),
	// like `eai agent` does.
	final := stripTaskCompletedSentinel(state.FinalOutput)
	final = strings.TrimSpace(final)
	if final != "" {
		return final, nil
	}

	// Fall back to a friendly summary when the final output is empty/generic.
	return renderAgentStateForChat(state), nil
}

func (a *Application) ExecuteAgentTaskInSessionWithCompanions(
	ctx context.Context,
	sessionID string,
	workDir string,
	task string,
	progress func(ProgressEvent),
	decisions <-chan PermissionDecision,
) (string, error) {
	if !toolCompanionsEnabled() {
		return a.executeAgentTaskInSessionWithProgressEventsAndPrelude(ctx, sessionID, workDir, task, progress, decisions, "")
	}
	if a == nil || a.Client == nil {
		return a.executeAgentTaskInSessionWithProgressEventsAndPrelude(ctx, sessionID, workDir, task, progress, decisions, "")
	}

	// Load lightweight session memory once and share with companions (small).
	var (
		history        []StoredMessage
		sessionSummary string
	)
	if a.Memory != nil && strings.TrimSpace(sessionID) != "" {
		if msgs, err := a.Memory.LoadMessages(sessionID); err == nil {
			history = msgs
		}
		if sess, _, err := a.Memory.LoadSessionForWorkDir(workDir, sessionID); err == nil && sess != nil {
			sessionSummary = strings.TrimSpace(sess.ContextSummary)
		}
	}
	history = dropTrailingUserEcho(history, task)
	sessionPrelude := buildAgentMemoryPrelude(sessionSummary, history)

	workers := a.orchestrateActivePaneCap()
	if workers < 2 {
		workers = 2
	}
	if workers > 20 {
		workers = 20
	}

	maxBudget := workers
	if maxBudget < 2 {
		maxBudget = 2
	}
	if maxBudget > 20 {
		maxBudget = 20
	}
	companionCount := orchestrateDesiredShardCount(maxBudget, task, 2)
	if companionCount > maxBudget {
		companionCount = maxBudget
	}
	if companionCount < 2 {
		companionCount = 2
	}

	if progress != nil {
		progress(ProgressEvent{
			Kind: "thinking",
			Text: fmt.Sprintf("Companions: starting %d read-only agent(s)", companionCount),
			At:   time.Now(),
		})
	}

	type compRes struct {
		Index int
		Text  string
	}
	resCh := make(chan compRes, companionCount)

	activeMu := sync.Mutex{}
	active := 0
	emitActive := func(delta int) {
		if progress == nil {
			return
		}
		activeMu.Lock()
		active += delta
		if active < 0 {
			active = 0
		}
		n := active
		activeMu.Unlock()
		progress(ProgressEvent{
			Kind:             "orchestrate_companions",
			Text:             fmt.Sprintf("active companions: %d", n),
			ActiveCompanions: n,
			MaxCompanions:    companionCount,
			At:               time.Now(),
		})
	}

	tasks := buildCompanionDiscoveryTasks(task, companionCount)
	for idx := 0; idx < companionCount; idx++ {
		i := idx
		go func() {
			emitActive(1)
			defer emitActive(-1)

			stateDir := filepath.Join(os.TempDir(), "cli-agent", "states")
			agent := NewAgentLoop(a.Client, 25, stateDir, a.Logger)
			agent.Relentless = true
			agent.Tools = companionTools()
			if progress != nil {
				agent.Progress = func(ev ProgressEvent) {
					ev.CompanionLabel = fmt.Sprintf("Companion %d", i+1)
					ev.CompanionID = fmt.Sprintf("%d", i+1)
					ev.CompanionIndex = i + 1
					ev.CompanionTotal = companionCount
					progress(ev)
				}
			}
			agent.SystemMessageBuilder = func(_ string) string {
				wd := ""
				if cwd, err := os.Getwd(); err == nil {
					wd = cwd
				}
				return readOnlyCompanionSystemPrompt(wd)
			}
			agent.ToolCallFilter = func(call ToolCall) (bool, string) {
				if call.Name != "exec" {
					return true, ""
				}
				var args struct {
					Command string `json:"command"`
				}
				if err := json.Unmarshal(call.Arguments, &args); err != nil {
					return false, "blocked: invalid exec args"
				}
				return companionExecAllowed(args.Command)
			}
			if wd, err := os.Getwd(); err == nil && wd != "" {
				agent.WorkDir = wd
			}
			if strings.TrimSpace(sessionPrelude) != "" {
				agent.PreludeMessages = []AgentMessage{{
					Role:      "user",
					Content:   sessionPrelude,
					Timestamp: time.Now(),
				}}
			}

			compCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			state, err := agent.Execute(compCtx, tasks[i])
			out := ""
			if err == nil && state != nil {
				out = strings.TrimSpace(stripTaskCompletedSentinel(state.FinalOutput))
				if out == "" {
					out = strings.TrimSpace(renderAgentStateForChat(state))
				}
			}
			out = truncateEllipsis(strings.TrimSpace(out), 4000)
			if out == "" {
				out = "(no companion output)"
			}

			if progress != nil {
				progress(ProgressEvent{
					Kind:           "reasoning",
					Text:           out,
					CompanionLabel: fmt.Sprintf("Companion %d", i+1),
					CompanionID:    fmt.Sprintf("%d", i+1),
					CompanionIndex: i + 1,
					CompanionTotal: companionCount,
					At:             time.Now(),
				})
			}
			resCh <- compRes{Index: i, Text: out}
		}()
	}

	results := make([]string, companionCount)
	for i := 0; i < companionCount; i++ {
		r := <-resCh
		if r.Index >= 0 && r.Index < len(results) {
			results[r.Index] = r.Text
		}
	}

	var extra strings.Builder
	extra.WriteString("Companion agent findings (read-only discovery):\n")
	for i, txt := range results {
		txt = strings.TrimSpace(txt)
		if txt == "" {
			continue
		}
		extra.WriteString("\n[Companion ")
		extra.WriteString(strconv.Itoa(i + 1))
		extra.WriteString("]\n")
		extra.WriteString(txt)
		extra.WriteString("\n")
	}

	if progress != nil {
		progress(ProgressEvent{
			Kind: "orchestrate_companions_peak",
			Text: fmt.Sprintf("companions finished: %d", companionCount),
			At:   time.Now(),
		})
	}

	return a.executeAgentTaskInSessionWithProgressEventsAndPrelude(ctx, sessionID, workDir, task, progress, decisions, extra.String())
}

func (a *Application) ExecuteChatInSession(ctx context.Context, sessionID string, mode Mode, input string) (string, error) {
	return a.ExecuteChatInSessionWithProgress(ctx, sessionID, mode, input, nil)
}

func (a *Application) ExecuteChatInSessionWithProgress(ctx context.Context, sessionID string, mode Mode, input string, progress func(string)) (string, error) {
	var progressEvents func(ProgressEvent)
	if progress != nil {
		progressEvents = func(ev ProgressEvent) {
			if strings.TrimSpace(ev.Text) != "" {
				progress(ev.Text)
			}
		}
	}
	return a.ExecuteChatInSessionWithProgressEvents(ctx, sessionID, "", mode, input, progressEvents, nil)
}

func (a *Application) ExecuteChatInSessionWithProgressEvents(
	ctx context.Context,
	sessionID string,
	workDir string,
	mode Mode,
	input string,
	progress func(ProgressEvent),
	decisions <-chan PermissionDecision,
) (string, error) {
	origInput := input

	// Local fastpath: scaffold a React site in create mode without requiring an API key.
	if mode == ModeCreate {
		if out, ok, err := tryLocalReactScaffold(input); ok {
			return out, err
		}
	}

	// For now, session awareness only affects chat-mode prompts (history injection) and title generation,
	// while tool-mode uses the same underlying agent loop. The TUI owns persistence; this method reads
	// history for better continuity.

	var history []StoredMessage
	var sessionInfo *Session
	sessionSummary := ""
	if a.Memory != nil && strings.TrimSpace(sessionID) != "" {
		if msgs, err := a.Memory.LoadMessages(sessionID); err == nil {
			history = msgs
		}
		if sess, _, err := a.Memory.LoadSessionForWorkDir(workDir, sessionID); err == nil && sess != nil {
			sessionInfo = sess
			sessionSummary = strings.TrimSpace(sess.ContextSummary)
		}
	}

	// Normalize low-information "continue" turns to avoid the model asking for context.
	if normalized, ok := normalizeContinueTurn(input); ok {
		input = normalized
	}

	// If orchestrate mode received an actionable request, run the tool agent with session memory.
	// This avoids "I can't access local files" responses for local-path tasks.
	if mode == ModeOrchestrate && looksActionableForCreate(origInput) {
		return a.ExecuteAgentTaskInSessionWithCompanions(ctx, sessionID, workDir, input, progress, decisions)
	}

	// For orchestrate mode, inject lightweight session context so shard workers can continue.
	// Keep this small to avoid ARG_MAX issues when tmux is used.
	if strings.TrimSpace(sessionID) != "" &&
		mode == ModeOrchestrate &&
		(len(history) > 0 || strings.TrimSpace(sessionSummary) != "") &&
		!looksLikeTrivialChatTurn(origInput) {
		max := 8
		if len(history) > max {
			history = history[len(history)-max:]
		}
		var b strings.Builder
		b.WriteString("Current request:\n")
		b.WriteString(input)
		b.WriteString("\n\n")

		if strings.TrimSpace(sessionSummary) != "" {
			b.WriteString("Session summary:\n")
			b.WriteString(truncateEllipsis(sessionSummary, 2500))
			b.WriteString("\n\n")
		}

		if len(history) > 0 {
			b.WriteString("Conversation context (most recent messages):\n")
			for _, m := range history {
				role := strings.ToUpper(strings.TrimSpace(m.Role))
				if role == "" {
					continue
				}
				txt := strings.TrimSpace(m.Content)
				if txt == "" {
					continue
				}
				txt = strings.ReplaceAll(txt, "\n", " ")
				txt = strings.Join(strings.Fields(txt), " ")
				if len(txt) > 300 {
					txt = txt[:300] + "..."
				}
				b.WriteString(role)
				b.WriteString(": ")
				b.WriteString(txt)
				b.WriteString("\n")
			}
		}
		input = b.String()
	}

	// Plan mode now runs a read-only tool agent. Inject lightweight context similarly to tool modes.
	toolSessionContext := strings.TrimSpace(os.Getenv("EAI_TOOL_SESSION_CONTEXT")) == "1"
	if toolSessionContext &&
		strings.TrimSpace(sessionID) != "" &&
		(len(history) > 0 || strings.TrimSpace(sessionSummary) != "") &&
		((IsToolMode(mode) && mode != ModeOrchestrate) || mode == ModePlan) &&
		!looksLikeListFilesRequest(origInput) &&
		!looksLikeTrivialChatTurn(origInput) {
		max := 8
		if len(history) > max {
			history = history[len(history)-max:]
		}
		var b strings.Builder
		b.WriteString("Current request:\n")
		b.WriteString(origInput)
		b.WriteString("\n\n")

		if strings.TrimSpace(sessionSummary) != "" {
			b.WriteString("Session summary:\n")
			b.WriteString(truncateEllipsis(sessionSummary, 2500))
			b.WriteString("\n\n")
		}

		if len(history) > 0 {
			b.WriteString("Conversation context (most recent messages):\n")
			for _, m := range history {
				role := strings.ToUpper(strings.TrimSpace(m.Role))
				if role == "" {
					continue
				}
				txt := strings.TrimSpace(m.Content)
				if txt == "" {
					continue
				}
				txt = strings.ReplaceAll(txt, "\n", " ")
				txt = strings.Join(strings.Fields(txt), " ")
				if len(txt) > 300 {
					txt = txt[:300] + "..."
				}
				b.WriteString(role)
				b.WriteString(": ")
				b.WriteString(txt)
				b.WriteString("\n")
			}
		}
		input = b.String()
	}

	// Pure text-only modes (except plan) use chat completions with history.
	if !IsToolMode(mode) && mode != ModePlan {
		routingInput := input
		effectiveInput := input
		if translated, ok, err := translateToEnglish(ctx, a.Client, input); err == nil && ok {
			routingInput = translated
			effectiveInput = translated
		}

		// If this looks actionable, reuse existing behavior.
		if mode != ModeCreate && looksActionableForCreate(routingInput) {
			return "This looks like an action request. Switch to `create` mode (shift+tab) to create files / run commands, then send the same request again.", nil
		}

		systemPrompt := GetChatSystemPrompt(mode, "", a.Config.ChatVerbosity)
		ctxInfo := GetProjectContext()
		if ctxInfo != "" {
			systemPrompt = systemPrompt + "\n\n" + ctxInfo
		}

		historyLimit := sessionPromptHistoryLimit
		if strings.TrimSpace(sessionSummary) != "" {
			historyLimit = sessionPromptHistoryLimitSummary
		}
		promptText := buildSessionChatPrompt(systemPrompt, sessionSummary, history, effectiveInput, historyLimit)

		if len(promptText) > sessionPromptSoftLimitChars() {
			if progress != nil {
				progress(ProgressEvent{
					Kind: "thinking",
					Text: "Compacting old session context before request",
				})
			}
			if compacted, compactErr := a.compactSessionContext(ctx, mode, effectiveInput, history, sessionSummary); compactErr == nil {
				sessionSummary = strings.TrimSpace(compacted)
				if sessionInfo != nil && a.Memory != nil && sessionSummary != "" {
					sessionInfo.ContextSummary = sessionSummary
					_ = a.Memory.SaveSession(sessionInfo)
				}
				historyLimit = sessionPromptHistoryLimitCompacted
				promptText = buildSessionChatPrompt(systemPrompt, sessionSummary, history, effectiveInput, historyLimit)
				if progress != nil {
					progress(ProgressEvent{
						Kind: "thinking",
						Text: "Context compacted, continuing request",
					})
				}
			}
		}

		out, err := completeChatWithProgress(ctx, a.Client, promptText, progress, true)
		if err != nil && isContextOverflowError(err) {
			if progress != nil {
				progress(ProgressEvent{
					Kind: "thinking",
					Text: "Context window reached, compacting session",
				})
			}
			compacted, compactErr := a.compactSessionContext(ctx, mode, effectiveInput, history, sessionSummary)
			if compactErr == nil && strings.TrimSpace(compacted) != "" {
				sessionSummary = strings.TrimSpace(compacted)
				if sessionInfo != nil && a.Memory != nil {
					sessionInfo.ContextSummary = sessionSummary
					_ = a.Memory.SaveSession(sessionInfo)
				}

				// First retry: summary + short tail of recent turns.
				retryPrompt := buildSessionChatPrompt(systemPrompt, sessionSummary, history, effectiveInput, sessionPromptHistoryLimitCompacted)
				out, err = completeChatWithProgress(ctx, a.Client, retryPrompt, progress, false)
				if err != nil && isContextOverflowError(err) {
					// Second retry: summary only + latest user turn.
					minimalPrompt := buildSessionChatPrompt(systemPrompt, sessionSummary, nil, effectiveInput, 0)
					out, err = completeChatWithProgress(ctx, a.Client, minimalPrompt, progress, false)
				}
			}
		}
		if err != nil {
			if errors.Is(err, ErrAPIKeyRequired) {
				return "No API key configured. Run `/connect` in the TUI or set `EAI_API_KEY`/`MINIMAX_API_KEY` (and optionally `EAI_MODEL`, `EAI_BASE_URL`).", nil
			}
			return "", err
		}
		return out, nil
	}

	return a.ExecuteChatWithProgressEvents(ctx, mode, input, progress)
}

func titleFromHeuristic(messages []StoredMessage) string {
	var first string
	for _, m := range messages {
		if strings.ToLower(m.Role) == "user" {
			first = strings.TrimSpace(m.Content)
			break
		}
	}
	if first == "" {
		return "New Chat"
	}
	first = strings.ReplaceAll(first, "\n", " ")
	first = strings.Join(strings.Fields(first), " ")
	// Strip leading verbs.
	lower := strings.ToLower(first)
	verbs := []string{"create", "make", "generate", "build", "write", "help", "please"}
	for _, v := range verbs {
		if strings.HasPrefix(lower, v+" ") {
			first = strings.TrimSpace(first[len(v):])
			break
		}
	}
	words := strings.Fields(first)
	if len(words) > 6 {
		words = words[:6]
	}
	out := strings.Join(words, " ")
	if out == "" {
		return "New Chat"
	}
	// Simple title casing.
	parts := strings.Fields(out)
	for i := range parts {
		p := parts[i]
		if len(p) > 1 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		} else {
			parts[i] = strings.ToUpper(p)
		}
	}
	return strings.Join(parts, " ")
}

func (a *Application) GenerateChatTitle(ctx context.Context, messages []StoredMessage) (string, error) {
	// If no key (or mock), fall back quickly.
	if a.Client == nil || a.Client.APIKey == "" || a.Client.BaseURL == "mock://" {
		return titleFromHeuristic(messages), nil
	}

	// Keep the prompt tiny: derive title from the last few user messages.
	var recent []string
	for i := len(messages) - 1; i >= 0 && len(recent) < 4; i-- {
		if strings.ToLower(messages[i].Role) == "user" {
			txt := strings.TrimSpace(messages[i].Content)
			if txt != "" {
				recent = append(recent, txt)
			}
		}
	}
	// Reverse to chronological.
	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}

	prompt := strings.Builder{}
	prompt.WriteString("[SYSTEM]\n")
	prompt.WriteString("Generate a short chat title (3-6 words). Title Case. No quotes. Output ONLY the title.\n\n")
	prompt.WriteString("[USER]\n")
	prompt.WriteString("Requests:\n")
	for _, r := range recent {
		r = strings.ReplaceAll(r, "\n", " ")
		r = strings.Join(strings.Fields(r), " ")
		prompt.WriteString("- ")
		prompt.WriteString(r)
		prompt.WriteString("\n")
	}

	out, err := a.Client.Complete(ctx, prompt.String())
	if err != nil {
		return titleFromHeuristic(messages), nil
	}
	title := strings.TrimSpace(out)
	title = strings.Trim(title, "\"")
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		title = titleFromHeuristic(messages)
	}
	return title, nil
}

func (a *Application) ExecuteOrchestrate(ctx context.Context, mode Mode, input string, agents int) (string, error) {
	return a.ExecuteOrchestrateWithProgressEvents(ctx, mode, input, agents, nil)
}

func (a *Application) ExecuteOrchestrateWithProgressEvents(
	ctx context.Context,
	mode Mode,
	input string,
	agents int,
	progress func(ProgressEvent),
) (string, error) {
	if a == nil {
		return "", errors.New("application is nil")
	}
	if a.Client == nil {
		return "", errors.New("client is required")
	}
	if agents <= 0 {
		agents = 1
	}

	splitStart := time.Now()
	maxBudget := a.orchestrateTaskConcurrencyBudget(agents)
	tasks := splitTaskForOrchestration(input, maxBudget)
	if len(tasks) == 0 {
		tasks = []string{input}
	}

	desired := orchestrateDesiredShardCount(maxBudget, input, len(tasks))
	if desired > len(tasks) && orchestrateLLMDecomposeEnabled() && maxBudget >= 4 {
		if decomp, err := llmDecomposeOrchestrate(ctx, a.Client, input, desired); err == nil && len(decomp) > len(tasks) {
			tasks = decomp
		}
	}
	if desired > len(tasks) {
		tasks = padOrchestrateTasks(tasks, desired, input)
	}
	if len(tasks) > maxBudget {
		tasks = tasks[:maxBudget]
	}

	agents = len(tasks)
	if agents <= 0 {
		agents = 1
		tasks = []string{input}
	}

	orchestrateEmitProgress(progress, "orchestrate_split", fmt.Sprintf("split task into %d shard(s)", agents), time.Since(splitStart))

	scheduleStart := time.Now()
	shards := make([]TaskShard, 0, agents)
	for i := 0; i < agents; i++ {
		shards = append(shards, TaskShard{
			ID:      fmt.Sprintf("%d", i+1),
			Index:   i,
			Total:   agents,
			Subtask: tasks[i],
			// Keep shard prompts minimal: these are short, tool-less completions that may
			// be executed via tmux worker panes. Large prompts are more likely to hit
			// ARG_MAX when passed as tmux command arguments.
			Prompt: buildOrchestrateSubtaskPrompt(i+1, agents, tasks[i], input),
		})
	}
	orchestrateEmitProgress(progress, "orchestrate_schedule", fmt.Sprintf("created %d shard prompt(s)", len(shards)), time.Since(scheduleStart))
	orchestrateEmitProgress(progress, "orchestrate_llm", "starting shard calls", 0)

	results := a.executeOrchestrateShardsWithRetries(ctx, mode, input, shards, progress)
	if len(results) == 1 && results[0].Err != nil {
		return "", results[0].Err
	}

	synthStart := time.Now()
	out := SynthesizeResults(results)
	orchestrateEmitProgress(progress, "orchestrate_synthesis", "synthesized shard outputs", time.Since(synthStart))
	if strings.TrimSpace(out) == "" {
		return "", errors.New("empty orchestrate output")
	}
	return out, nil
}

func orchestrateEmitProgress(progress func(ProgressEvent), kind, text string, duration time.Duration) {
	if progress == nil {
		return
	}
	progress(ProgressEvent{
		Kind:       kind,
		Text:       text,
		DurationMs: duration.Milliseconds(),
		At:         time.Now(),
	})
}

type orchestrateShardAttempt struct {
	Shard    TaskShard
	Attempt  int
	Output   string
	Err      error
	CacheHit bool
	Duration time.Duration

	UsedTmux       bool
	TmuxSpawn      time.Duration
	TmuxWait       time.Duration
	WorkerDuration time.Duration
}

func (a *Application) executeOrchestrateShardsWithRetries(ctx context.Context, mode Mode, fullTask string, shards []TaskShard, progress func(ProgressEvent)) []TaskResult {
	if len(shards) == 0 {
		return nil
	}

	results := make([]TaskResult, 0, len(shards))
	for range shards {
		results = append(results, TaskResult{ID: "", Index: -1})
	}

	metricsEnabled := orchestrateMetricsEnabled()
	shardDurations := make([]time.Duration, len(shards))
	shardTmuxSpawn := make([]time.Duration, len(shards))
	shardTmuxWait := make([]time.Duration, len(shards))
	shardWorkerDuration := make([]time.Duration, len(shards))
	shardUsedTmux := make([]bool, len(shards))
	shardCacheHit := make([]bool, len(shards))
	activeCompanions := 0
	maxObservedCompanions := 0
	companionMu := sync.Mutex{}

	finalized := make([]bool, len(shards))
	retried := make([]bool, len(shards))
	remaining := len(shards)
	orchestrateCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type orchestrateShardWork struct {
		Shard   TaskShard
		Attempt int
	}

	maxAttempts := 1 + defaultOrchestrateRetryCount
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	resultBuffer := len(shards) * maxAttempts
	if resultBuffer < 1 {
		resultBuffer = 1
	}
	workBuffer := len(shards) * maxAttempts
	if workBuffer < 1 {
		workBuffer = 1
	}
	workCh := make(chan orchestrateShardWork, workBuffer)
	resultCh := make(chan orchestrateShardAttempt, resultBuffer)

	workers := a.orchestrateActivePaneCap()
	if workers > len(shards) {
		workers = len(shards)
	}
	if workers <= 0 {
		workers = 1
	}

	emitActive := func(delta int) {
		if progress == nil {
			return
		}
		companionMu.Lock()
		activeCompanions += delta
		if activeCompanions < 0 {
			activeCompanions = 0
		}
		if activeCompanions > maxObservedCompanions {
			maxObservedCompanions = activeCompanions
		}
		n := activeCompanions
		companionMu.Unlock()
		progress(ProgressEvent{
			Kind:             "orchestrate_companions",
			Text:             fmt.Sprintf("active companions: %d", n),
			ActiveCompanions: n,
			MaxCompanions:    workers,
			At:               time.Now(),
		})
	}

	var workerWG sync.WaitGroup
	workerWG.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer workerWG.Done()
			for work := range workCh {
				emitActive(1)
				orchestrateEmitProgress(progress, "orchestrate_tmux", fmt.Sprintf("started shard %s attempt %d", work.Shard.ID, work.Attempt), 0)
				resultCh <- a.executeOrchestrateShard(orchestrateCtx, mode, fullTask, work.Shard, work.Attempt, progress)
				emitActive(-1)
			}
		}()
	}

	startShard := func(shard TaskShard, attempt int) {
		workCh <- orchestrateShardWork{Shard: shard, Attempt: attempt}
	}

	// Dispatch short shards first to keep throughput high when we have more shards than workers.
	queue := make([]TaskShard, 0, len(shards))
	queue = append(queue, shards...)
	estimateCost := func(s TaskShard) int {
		subtask := strings.TrimSpace(s.Subtask)
		return countOrchestrateWords(subtask)*50 + len(subtask)
	}
	sort.SliceStable(queue, func(i, j int) bool {
		return estimateCost(queue[i]) < estimateCost(queue[j])
	})
	for _, shard := range queue {
		startShard(shard, 1)
	}

	retryAllowed := defaultOrchestrateRetryCount > 0
	for remaining > 0 {
		res := <-resultCh
		idx := res.Shard.Index
		if idx < 0 || idx >= len(shards) {
			continue
		}
		if finalized[idx] {
			continue
		}

		if progress != nil {
			t := fmt.Sprintf("shard %s attempt %d completed in %dms", res.Shard.ID, res.Attempt, res.Duration.Milliseconds())
			if res.CacheHit {
				t = fmt.Sprintf("shard %s attempt %d cache hit (%dms)", res.Shard.ID, res.Attempt, res.Duration.Milliseconds())
			}
			orchestrateEmitProgress(progress, "orchestrate_llm", t, res.Duration)

			// Surface companion shard outputs for cache hits (non-streaming) and shard errors.
			compLabel := fmt.Sprintf("Companion %s", strings.TrimSpace(res.Shard.ID))
			if strings.TrimSpace(compLabel) == "Companion" {
				compLabel = "Companion"
			}
			if res.Err != nil {
				progress(ProgressEvent{
					Kind:           "warn",
					Text:           truncateEllipsis(strings.TrimSpace(res.Err.Error()), 2000),
					CompanionLabel: compLabel,
					CompanionID:    strings.TrimSpace(res.Shard.ID),
					CompanionIndex: res.Shard.Index + 1,
					CompanionTotal: len(shards),
					At:             time.Now(),
				})
			} else if res.CacheHit {
				txt := strings.TrimSpace(res.Output)
				if txt != "" {
					progress(ProgressEvent{
						Kind:           "reasoning",
						Text:           truncateEllipsis(txt, 4000),
						CompanionLabel: compLabel,
						CompanionID:    strings.TrimSpace(res.Shard.ID),
						CompanionIndex: res.Shard.Index + 1,
						CompanionTotal: len(shards),
						At:             time.Now(),
					})
				}
			}
		}
		if metricsEnabled && progress != nil && res.UsedTmux && !res.CacheHit {
			if res.TmuxSpawn > 0 {
				orchestrateEmitProgress(progress, "orchestrate_tmux_spawn", fmt.Sprintf("shard %s tmux spawn", res.Shard.ID), res.TmuxSpawn)
			}
			if res.TmuxWait > 0 {
				orchestrateEmitProgress(progress, "orchestrate_tmux_wait", fmt.Sprintf("shard %s tmux wait", res.Shard.ID), res.TmuxWait)
			}
			if res.WorkerDuration > 0 {
				orchestrateEmitProgress(progress, "orchestrate_worker_llm", fmt.Sprintf("shard %s worker llm", res.Shard.ID), res.WorkerDuration)
			}
		}

		if res.Err != nil && !retried[idx] && retryAllowed {
			if ctx.Err() == nil {
				retried[idx] = true
				orchestrateEmitProgress(progress, "orchestrate_retry", fmt.Sprintf("retrying failed shard %s with constrained prompt", res.Shard.ID), 0)
				startShard(TaskShard{ID: res.Shard.ID, Index: idx, Total: res.Shard.Total, Subtask: res.Shard.Subtask, Prompt: res.Shard.Prompt}, res.Attempt+1)
				orchestrateEmitProgress(progress, "orchestrate_shard_done", fmt.Sprintf("shard %s attempt %d finished with error; retry queued", res.Shard.ID, res.Attempt), 0)
				continue
			}
		}
		status := "failed"
		if res.Err == nil {
			status = "succeeded"
		}
		orchestrateEmitProgress(progress, "orchestrate_shard_done", fmt.Sprintf("shard %s attempt %d %s", res.Shard.ID, res.Attempt, status), res.Duration)

		results[idx] = TaskResult{
			ID:     res.Shard.ID,
			Index:  idx,
			Output: res.Output,
			Err:    res.Err,
		}
		if metricsEnabled {
			shardDurations[idx] = res.Duration
			shardTmuxSpawn[idx] = res.TmuxSpawn
			shardTmuxWait[idx] = res.TmuxWait
			shardWorkerDuration[idx] = res.WorkerDuration
			shardUsedTmux[idx] = res.UsedTmux
			shardCacheHit[idx] = res.CacheHit
		}
		finalized[idx] = true
		remaining--
		if res.Err == nil {
			if progress != nil {
				orchestrateEmitProgress(progress, "orchestrate_cache", fmt.Sprintf("shard %s ready", res.Shard.ID), 0)
			}
		}
	}

	close(workCh)
	workerWG.Wait()

	orchestrateEmitProgress(progress, "orchestrate_sync", fmt.Sprintf("all %d shard(s) finished", len(shards)), 0)
	if progress != nil {
		progress(ProgressEvent{
			Kind:             "orchestrate_companions",
			Text:             "companions: all clear",
			ActiveCompanions: 0,
			MaxCompanions:    workers,
			At:               time.Now(),
		})
	}

	if maxObservedCompanions > 0 && progress != nil {
		progress(ProgressEvent{
			Kind: "orchestrate_companions_peak",
			Text: fmt.Sprintf("peak companions observed: %d", maxObservedCompanions),
			At:   time.Now(),
		})
	}

	if metricsEnabled && progress != nil {
		collect := func(values []time.Duration) []time.Duration {
			out := make([]time.Duration, 0, len(values))
			for _, d := range values {
				if d > 0 {
					out = append(out, d)
				}
			}
			return out
		}
		percentile := func(values []time.Duration, p float64) time.Duration {
			if len(values) == 0 {
				return 0
			}
			sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
			if p <= 0 {
				return values[0]
			}
			if p >= 1 {
				return values[len(values)-1]
			}
			idx := int(p * float64(len(values)-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(values) {
				idx = len(values) - 1
			}
			return values[idx]
		}

		cacheHits := 0
		failures := 0
		retriedCount := 0
		tmuxCount := 0
		for i := range shards {
			if shardCacheHit[i] {
				cacheHits++
			}
			if results[i].Err != nil {
				failures++
			}
			if retried[i] {
				retriedCount++
			}
			if shardUsedTmux[i] {
				tmuxCount++
			}
		}

		shardVals := collect(shardDurations)
		p50 := percentile(shardVals, 0.50)
		p90 := percentile(shardVals, 0.90)
		p99 := percentile(shardVals, 0.99)

		text := fmt.Sprintf(
			"metrics: shards=%d workers=%d cache_hits=%d retried=%d failures=%d shard_p50=%dms shard_p90=%dms shard_p99=%dms",
			len(shards),
			workers,
			cacheHits,
			retriedCount,
			failures,
			p50.Milliseconds(),
			p90.Milliseconds(),
			p99.Milliseconds(),
		)

		if tmuxCount > 0 {
			spawnVals := collect(shardTmuxSpawn)
			waitVals := collect(shardTmuxWait)
			workerVals := collect(shardWorkerDuration)
			text = text + fmt.Sprintf(
				" tmux_spawn_p50=%dms tmux_wait_p50=%dms worker_p50=%dms",
				percentile(spawnVals, 0.50).Milliseconds(),
				percentile(waitVals, 0.50).Milliseconds(),
				percentile(workerVals, 0.50).Milliseconds(),
			)
		}
		orchestrateEmitProgress(progress, "orchestrate_metrics", text, 0)
	}

	return results
}

const orchestrateTmuxWorkerPollInterval = 70 * time.Millisecond

type orchestrateWorkerResult struct {
	ShardID    string `json:"shard_id"`
	Attempt    int    `json:"attempt"`
	Output     string `json:"output"`
	Error      string `json:"error"`
	CacheHit   bool   `json:"cache_hit"`
	DurationMs int64  `json:"duration_ms"`
}

type orchestrateWorkerProgressLine struct {
	Kind string `json:"kind,omitempty"`
	Text string `json:"text"`
}

func tailOrchestrateWorkerProgress(ctx context.Context, path string, onLine func(orchestrateWorkerProgressLine)) {
	if strings.TrimSpace(path) == "" || onLine == nil {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				time.Sleep(orchestrateTmuxWorkerPollInterval)
				continue
			}
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var msg orchestrateWorkerProgressLine
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		msg.Text = strings.TrimSpace(msg.Text)
		if msg.Text == "" {
			continue
		}
		onLine(msg)
	}
}

func (a *Application) orchestrateWorkerBinaryPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("EAI_ORCHESTRATE_WORKER_BIN")); override != "" {
		return override, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return exe, nil
}

func (a *Application) canRunOrchestrateShardsInTmux() bool {
	if a == nil {
		return false
	}
	if strings.TrimSpace(os.Getenv("EAI_TMUX_WORKER")) == "1" {
		return false
	}
	if strings.TrimSpace(os.Getenv("EAI_TMUX_DISABLE")) == "1" {
		return false
	}
	headless := strings.TrimSpace(os.Getenv("EAI_TMUX_HEADLESS")) == "1"
	if !headless && strings.TrimSpace(os.Getenv("TMUX")) == "" {
		return false
	}
	_, err := exec.LookPath("tmux")
	return err == nil
}

func tmuxHeadlessSessionName() string {
	if v := strings.TrimSpace(os.Getenv("EAI_TMUX_HEADLESS_SESSION")); v != "" {
		return v
	}
	return "eai-headless"
}

func (a *Application) ensureHeadlessTmuxSession(ctx context.Context) (string, error) {
	name := tmuxHeadlessSessionName()
	if name == "" {
		return "", errors.New("invalid headless tmux session name")
	}
	// Check for existing session.
	if err := exec.CommandContext(ctx, "tmux", "has-session", "-t", name).Run(); err == nil {
		return name, nil
	}
	// Create a detached session (window 0).
	if err := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", name).Run(); err != nil {
		return "", err
	}
	return name, nil
}

func (a *Application) readOrchestrateWorkerResult(ctx context.Context, resultPath string) (orchestrateWorkerResult, error) {
	var result orchestrateWorkerResult
	ticker := time.NewTicker(orchestrateTmuxWorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-ticker.C:
			payload, err := os.ReadFile(resultPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return result, err
			}
			if len(payload) == 0 {
				continue
			}

			if err := json.Unmarshal(payload, &result); err != nil {
				continue
			}
			return result, nil
		}
	}
}

func (a *Application) killTmuxPane(pane string) {
	pane = strings.TrimSpace(pane)
	if pane == "" {
		return
	}
	_ = exec.Command("tmux", "kill-pane", "-t", pane).Run()
}

type orchestrateShardCallMeta struct {
	UsedTmux       bool
	TmuxSpawn      time.Duration
	TmuxWait       time.Duration
	WorkerDuration time.Duration
}

func (a *Application) executeOrchestrateShardInTmuxWithMeta(ctx context.Context, prompt string, shard TaskShard, attempt int, progress func(ProgressEvent)) (string, orchestrateShardCallMeta, error) {
	var meta orchestrateShardCallMeta

	resultPath, err := os.CreateTemp("", "eai-orchestrate-shard-*.json")
	if err != nil {
		return "", meta, err
	}
	defer os.Remove(resultPath.Name())

	progressPath, err := os.CreateTemp("", "eai-orchestrate-progress-*.jsonl")
	if err != nil {
		return "", meta, err
	}
	_ = progressPath.Close()
	defer os.Remove(progressPath.Name())

	binary, err := a.orchestrateWorkerBinaryPath()
	if err != nil {
		return "", meta, err
	}

	tmuxArgs := []string{"split-window", "-d", "-P", "-F", "#{pane_id}"}
	// tmux panes inherit the tmux server environment, not this process env. Provide
	// critical env vars explicitly so workers can read API keys and config overrides.
	tmuxArgs = append(tmuxArgs, "-e", "EAI_TMUX_WORKER=1")
	for _, name := range []string{
		"EAI_API_KEY",
		"MINIMAX_API_KEY",
		"EAI_BASE_URL",
		"EAI_MODEL",
		"EAI_MAX_TOKENS",
		"EAI_PERMISSIONS",
		"EAI_SKIP_TLS_VERIFY",
		"EAI_HTTP_TIMEOUT_SEC",
		"EAI_LLM_MAX_RETRIES",
		"EAI_LLM_REQUEST_TIMEOUT_SEC",
	} {
		if v := strings.TrimSpace(os.Getenv(name)); v != "" {
			tmuxArgs = append(tmuxArgs, "-e", fmt.Sprintf("%s=%s", name, v))
		}
	}
		target := strings.TrimSpace(os.Getenv("EAI_TMUX_TARGET"))
		if target == "" {
			if pane := strings.TrimSpace(os.Getenv("TMUX_PANE")); pane != "" {
				target = pane
			} else if strings.TrimSpace(os.Getenv("EAI_TMUX_HEADLESS")) == "1" {
				if sessName, err := a.ensureHeadlessTmuxSession(ctx); err == nil && sessName != "" {
					target = sessName + ":0"
				}
			}
		}
	if target != "" {
		tmuxArgs = append(tmuxArgs, "-t", target)
	}
	tmuxArgs = append(tmuxArgs,
		binary,
		"orchestrate-worker",
		"--shard-id", shard.ID,
		"--attempt", strconv.Itoa(attempt),
		"--prompt", prompt,
		"--result-file", resultPath.Name(),
		"--progress-file", progressPath.Name(),
	)

	spawnStart := time.Now()
	tmuxCmd := exec.CommandContext(ctx, "tmux", tmuxArgs...)
	tmuxCmd.Env = append(os.Environ(), "EAI_TMUX_WORKER=1")
	output, err := tmuxCmd.Output()
	meta.TmuxSpawn = time.Since(spawnStart)
	if err != nil {
		return "", meta, fmt.Errorf("tmux split-window failed: %w", err)
	}
	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) == 0 {
		return "", meta, fmt.Errorf("tmux split-window did not return pane id")
	}
	paneID := parts[0]
	defer a.killTmuxPane(paneID)

	progCtx, progCancel := context.WithCancel(ctx)
	if progress != nil {
		compLabel := fmt.Sprintf("Companion %s", strings.TrimSpace(shard.ID))
		compID := strings.TrimSpace(shard.ID)
		compIndex := shard.Index + 1
		compTotal := shard.Total
		go tailOrchestrateWorkerProgress(progCtx, progressPath.Name(), func(msg orchestrateWorkerProgressLine) {
			progress(ProgressEvent{
				Kind:           "reasoning",
				Text:           msg.Text,
				CompanionLabel: compLabel,
				CompanionID:    compID,
				CompanionIndex: compIndex,
				CompanionTotal: compTotal,
				At:             time.Now(),
			})
		})
	}

	waitStart := time.Now()
	workerRes, err := a.readOrchestrateWorkerResult(ctx, resultPath.Name())
	meta.TmuxWait = time.Since(waitStart)
	progCancel()
	if err != nil {
		return "", meta, err
	}
	_ = os.Remove(resultPath.Name())

	meta.UsedTmux = true
	if workerRes.DurationMs > 0 {
		meta.WorkerDuration = time.Duration(workerRes.DurationMs) * time.Millisecond
	}

	if workerRes.Error != "" {
		return "", meta, errors.New(workerRes.Error)
	}
	return workerRes.Output, meta, nil
}

func (a *Application) completeOrchestrateShardWithMeta(ctx context.Context, prompt string, shard TaskShard, attempt int, progress func(ProgressEvent)) (string, orchestrateShardCallMeta, error) {
	if a.canRunOrchestrateShardsInTmux() {
		return a.executeOrchestrateShardInTmuxWithMeta(ctx, prompt, shard, attempt, progress)
	}

	start := time.Now()
	if progress == nil {
		out, err := a.Client.Complete(ctx, prompt)
		return out, orchestrateShardCallMeta{WorkerDuration: time.Since(start)}, err
	}

	compLabel := fmt.Sprintf("Companion %s", strings.TrimSpace(shard.ID))
	compID := strings.TrimSpace(shard.ID)
	compIndex := shard.Index + 1
	compTotal := shard.Total

	var lastFlush time.Time
	var pending strings.Builder
	flushPending := func() {
		if pending.Len() == 0 {
			return
		}
		progress(ProgressEvent{
			Kind:           "reasoning",
			Text:           pending.String(),
			CompanionLabel: compLabel,
			CompanionID:    compID,
			CompanionIndex: compIndex,
			CompanionTotal: compTotal,
			At:             time.Now(),
		})
		pending.Reset()
		lastFlush = time.Now()
	}
	addDelta := func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		pending.WriteString(text)
		if pending.Len() >= 512 || lastFlush.IsZero() || time.Since(lastFlush) >= 120*time.Millisecond {
			flushPending()
		}
	}

	out, _, err := a.Client.CompleteStreamingWithObserverMeta(ctx, prompt, addDelta, addDelta)
	flushPending()
	return out, orchestrateShardCallMeta{WorkerDuration: time.Since(start)}, err
}

func (a *Application) executeOrchestrateShard(ctx context.Context, mode Mode, fullTask string, shard TaskShard, attempt int, progress func(ProgressEvent)) orchestrateShardAttempt {
	prompt := shard.Prompt
	if attempt > 1 {
		prompt = buildOrchestrateSubtaskRetryPrompt(shard.Subtask, fullTask, attempt)
	}
	cacheKey := a.orchestrateCacheKey(mode, fullTask, shard.Subtask, prompt)
	if cached, ok := a.getOrchestrateCache(cacheKey); ok {
		return orchestrateShardAttempt{
			Shard:    shard,
			Attempt:  attempt,
			Output:   cached,
			CacheHit: true,
		}
	}
	start := time.Now()
	callCtx := ctx
	var cancel context.CancelFunc
	if timeout := orchestrateShardTimeout(); timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	out, meta, err := a.completeOrchestrateShardWithMeta(callCtx, prompt, shard, attempt, progress)
	if cancel != nil {
		cancel()
	}
	duration := time.Since(start)
	if err == nil {
		a.setOrchestrateCache(cacheKey, out)
	}
	return orchestrateShardAttempt{
		Shard:          shard,
		Attempt:        attempt,
		Output:         out,
		Err:            err,
		Duration:       duration,
		UsedTmux:       meta.UsedTmux,
		TmuxSpawn:      meta.TmuxSpawn,
		TmuxWait:       meta.TmuxWait,
		WorkerDuration: meta.WorkerDuration,
	}
}

func (a *Application) orchestrateCacheEnabled() bool {
	if a == nil {
		return false
	}
	return a.orchestrateCacheTTL > 0
}

func (a *Application) orchestrateCacheKey(mode Mode, fullTask, subtask, prompt string) string {
	normalized := []string{
		strings.ToLower(strings.TrimSpace(subtask)),
		normalizeOrchestrateContextForCache(fullTask),
		normalizeOrchestrateContextForCache(prompt),
		string(mode),
		strings.TrimSpace(a.Config.Model),
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\n")))
	return hex.EncodeToString(sum[:])
}

func (a *Application) getOrchestrateCache(key string) (string, bool) {
	if !a.orchestrateCacheEnabled() {
		return "", false
	}
	if a.orchestrateCache == nil {
		a.orchestrateCacheMu.Lock()
		if a.orchestrateCache == nil {
			a.orchestrateCache = make(map[string]orchestrateCacheEntry)
		}
		a.orchestrateCacheMu.Unlock()
	}

	a.orchestrateCacheMu.RLock()
	entry, ok := a.orchestrateCache[key]
	a.orchestrateCacheMu.RUnlock()
	if !ok {
		return "", false
	}
	if a.orchestrateCacheTTL > 0 && time.Since(entry.CreatedAt) > a.orchestrateCacheTTL {
		a.orchestrateCacheMu.Lock()
		delete(a.orchestrateCache, key)
		a.orchestrateCacheMu.Unlock()
		return "", false
	}
	return entry.Output, true
}

func (a *Application) setOrchestrateCache(key string, value string) {
	if !a.orchestrateCacheEnabled() {
		return
	}
	if a.orchestrateCache == nil {
		a.orchestrateCacheMu.Lock()
		if a.orchestrateCache == nil {
			a.orchestrateCache = make(map[string]orchestrateCacheEntry)
		}
		a.orchestrateCacheMu.Unlock()
	}
	a.orchestrateCacheMu.Lock()
	a.orchestrateCache[key] = orchestrateCacheEntry{
		Output:    value,
		CreatedAt: time.Now(),
	}
	a.orchestrateCacheMu.Unlock()
}

func isAlphaNumByte(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}

func containsOrchestrateToken(haystackLower, tokenLower string) bool {
	haystackLower = strings.TrimSpace(haystackLower)
	tokenLower = strings.TrimSpace(tokenLower)
	if haystackLower == "" || tokenLower == "" {
		return false
	}

	// For non-word tokens (contains punctuation/space), plain substring match is fine.
	for i := 0; i < len(tokenLower); i++ {
		if !isAlphaNumByte(tokenLower[i]) {
			return strings.Contains(haystackLower, tokenLower)
		}
	}

	idx := strings.Index(haystackLower, tokenLower)
	for idx >= 0 {
		leftOK := idx == 0 || !isAlphaNumByte(haystackLower[idx-1])
		rightIdx := idx + len(tokenLower)
		rightOK := rightIdx >= len(haystackLower) || !isAlphaNumByte(haystackLower[rightIdx])
		if leftOK && rightOK {
			return true
		}
		next := idx + 1
		if next >= len(haystackLower) {
			return false
		}
		off := strings.Index(haystackLower[next:], tokenLower)
		if off < 0 {
			return false
		}
		idx = next + off
	}
	return false
}

func orchestrateComplexityScore(input string) int {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return 0
	}

	score := 0

	// Structural hints.
	if strings.Contains(s, "\n") {
		score += 1
	}
	if strings.Contains(s, "\n-") || strings.Contains(s, "\n*") || strings.Contains(s, "\n1") {
		score += 2
	}
	if strings.Contains(s, " and then ") || strings.Contains(s, ", then ") {
		score += 1
	}
	if strings.Contains(s, " with ") || strings.Contains(s, " including ") {
		score += 1
	}

	// Weighted keyword hints (short prompts can still be large scope).
	type kw struct {
		token  string
		weight int
	}
	keywords := []kw{
		{token: "full", weight: 4},
		{token: "complete", weight: 4},
		{token: "end-to-end", weight: 4},
		{token: "production", weight: 3},
		{token: "enterprise", weight: 3},
		{token: "scalable", weight: 2},

		{token: "dashboard", weight: 3},
		{token: "admin", weight: 2},
		{token: "crm", weight: 4},
		{token: "erp", weight: 4},

		{token: "frontend", weight: 3},
		{token: "ui", weight: 2},
		{token: "ux", weight: 2},
		{token: "react", weight: 2},
		{token: "nextjs", weight: 2},
		{token: "tailwind", weight: 1},
		{token: "wails", weight: 2},

		{token: "backend", weight: 3},
		{token: "api", weight: 2},
		{token: "server", weight: 2},
		{token: "go", weight: 2},
		{token: "golang", weight: 2},
		{token: "grpc", weight: 2},

		{token: "database", weight: 3},
		{token: "db", weight: 2},
		{token: "postgres", weight: 3},
		{token: "mysql", weight: 2},
		{token: "sqlite", weight: 2},

		{token: "auth", weight: 3},
		{token: "authentication", weight: 3},
		{token: "authorization", weight: 3},
		{token: "login", weight: 2},
		{token: "oauth", weight: 2},
		{token: "jwt", weight: 2},

		{token: "deploy", weight: 2},
		{token: "deployment", weight: 2},
		{token: "docker", weight: 2},
		{token: "kubernetes", weight: 2},
		{token: "ci", weight: 1},
		{token: "cd", weight: 1},
		{token: "github actions", weight: 1},

		{token: "tests", weight: 2},
		{token: "testing", weight: 2},
		{token: "e2e", weight: 2},
	}

	seen := map[string]bool{}
	for _, k := range keywords {
		if seen[k.token] {
			continue
		}
		if containsOrchestrateToken(s, k.token) {
			score += k.weight
			seen[k.token] = true
		}
	}

	return score
}

func orchestrateDesiredShardCount(maxBudget int, input string, baseCount int) int {
	if maxBudget <= 1 {
		return 1
	}
	if baseCount <= 0 {
		baseCount = 1
	}
	if baseCount > maxBudget {
		baseCount = maxBudget
	}

	score := orchestrateComplexityScore(input)
	words := countOrchestrateWords(input)

	desiredByScore := baseCount
	switch {
	case score >= 16:
		desiredByScore = maxBudget
	case score >= 12:
		desiredByScore = minInt(maxBudget, 12)
	case score >= 9:
		desiredByScore = minInt(maxBudget, 8)
	case score >= 6:
		desiredByScore = minInt(maxBudget, 6)
	case score >= 4:
		desiredByScore = minInt(maxBudget, 4)
	default:
		desiredByScore = minInt(maxBudget, maxInt(2, baseCount))
	}

	// Length scaling: long prompts should fan out even if they don't hit the keyword
	// heuristic (e.g. non-English requirements dumps).
	desiredByWords := 0
	if words > 0 {
		desiredByWords = minInt(maxBudget, maxInt(2, words/20))
	}

	desired := maxInt(desiredByScore, desiredByWords)

	if baseCount > desired {
		desired = baseCount
	}
	if desired < 2 {
		desired = 2
	}
	if desired > maxBudget {
		desired = maxBudget
	}
	return desired
}

func parseJSONStringArray(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}

	// Strip markdown fences if present.
	if i := strings.Index(raw, "```"); i >= 0 {
		rest := raw[i+3:]
		rest = strings.TrimLeft(rest, " \n\r\t")
		if j := strings.LastIndex(rest, "```"); j >= 0 {
			raw = strings.TrimSpace(rest[:j])
		}
	}

	cand := raw
	if !strings.HasPrefix(strings.TrimSpace(cand), "[") {
		start := strings.Index(cand, "[")
		end := strings.LastIndex(cand, "]")
		if start >= 0 && end > start {
			cand = cand[start : end+1]
		}
	}

	var arr []string
	if err := json.Unmarshal([]byte(cand), &arr); err == nil {
		return arr, true
	}

	var obj struct {
		Tasks  []string `json:"tasks"`
		Shards []string `json:"shards"`
	}
	if err := json.Unmarshal([]byte(cand), &obj); err == nil {
		if len(obj.Tasks) > 0 {
			return obj.Tasks, true
		}
		if len(obj.Shards) > 0 {
			return obj.Shards, true
		}
	}

	return nil, false
}

func normalizeDecomposedSubtasks(items []string, max int) []string {
	if max <= 0 {
		max = 1
	}
	seen := map[string]bool{}
	out := make([]string, 0, minInt(len(items), max))
	for _, it := range items {
		clean := strings.TrimSpace(it)
		if clean == "" {
			continue
		}
		if m := orchestrateListMarkerRE.FindStringSubmatch(clean); len(m) == 2 {
			clean = strings.TrimSpace(m[1])
		}
		clean = strings.ReplaceAll(clean, "\n", " ")
		clean = strings.Join(strings.Fields(clean), " ")
		if countOrchestrateWords(clean) < 2 {
			continue
		}
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, clean)
		if len(out) >= max {
			break
		}
	}
	return out
}

func fallbackOrchestrateCoverageTasks(input string) []string {
	s := strings.ToLower(strings.TrimSpace(input))
	out := []string{
		"Extract requirements and constraints from the request.",
		"List key risks, unknowns, and assumptions to unblock execution.",
		"Define acceptance criteria and how success will be validated.",
	}

	if containsOrchestrateToken(s, "frontend") || containsOrchestrateToken(s, "dashboard") || containsOrchestrateToken(s, "ui") || containsOrchestrateToken(s, "react") {
		out = append(out, "Outline the main UI screens, flows, and UX details.")
	}
	if containsOrchestrateToken(s, "backend") || containsOrchestrateToken(s, "api") || containsOrchestrateToken(s, "server") || containsOrchestrateToken(s, "grpc") {
		out = append(out, "Design the backend/API surface: endpoints, contracts, and responsibilities.")
	}
	if containsOrchestrateToken(s, "database") || containsOrchestrateToken(s, "db") || containsOrchestrateToken(s, "postgres") || containsOrchestrateToken(s, "sqlite") {
		out = append(out, "Propose a database schema and data access approach.")
	}
	if containsOrchestrateToken(s, "auth") || containsOrchestrateToken(s, "login") || containsOrchestrateToken(s, "oauth") || containsOrchestrateToken(s, "jwt") {
		out = append(out, "Plan authentication and authorization: roles, sessions, and security.")
	}
	if containsOrchestrateToken(s, "deploy") || containsOrchestrateToken(s, "deployment") || containsOrchestrateToken(s, "docker") || containsOrchestrateToken(s, "kubernetes") {
		out = append(out, "Define deployment/runtime plan: environments, config, and rollout steps.")
	}
	if containsOrchestrateToken(s, "tests") || containsOrchestrateToken(s, "testing") || containsOrchestrateToken(s, "ci") {
		out = append(out, "Define testing strategy and CI checks.")
	}

	out = append(out, "Produce a concrete step-by-step implementation plan.")
	return out
}

func padOrchestrateTasks(existing []string, desired int, input string) []string {
	if desired <= 0 {
		return existing
	}
	if len(existing) >= desired {
		return existing[:desired]
	}

	seen := map[string]bool{}
	out := make([]string, 0, desired)
	for _, t := range existing {
		clean := strings.TrimSpace(t)
		if clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}

	for _, t := range fallbackOrchestrateCoverageTasks(input) {
		if len(out) >= desired {
			break
		}
		clean := strings.TrimSpace(t)
		if clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		if countOrchestrateWords(clean) < 2 {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}

	return out
}

func llmDecomposeOrchestrate(ctx context.Context, client *MinimaxClient, input string, desired int) ([]string, error) {
	if client == nil {
		return nil, errors.New("client unavailable")
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, errors.New("empty task")
	}
	if desired < 2 {
		desired = 2
	}
	if desired > 50 {
		desired = 50
	}

	var b strings.Builder
	b.WriteString("Split the following user request into independent subtasks for parallel execution.\n")
	b.WriteString("Return ONLY valid JSON: an array of strings.\n")
	b.WriteString("No markdown. No commentary. No numbering.\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Return exactly ")
	b.WriteString(strconv.Itoa(desired))
	b.WriteString(" items.\n")
	b.WriteString("- Each item must be 2-12 words and start with an action verb.\n")
	b.WriteString("- Make items mutually independent and specific.\n\n")
	b.WriteString("User request:\n")
	b.WriteString(input)

	decompCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()

	out, err := client.Complete(decompCtx, b.String())
	if err != nil {
		return nil, err
	}
	items, ok := parseJSONStringArray(out)
	if !ok {
		return nil, errors.New("invalid decomposition output")
	}
	tasks := normalizeDecomposedSubtasks(items, desired)
	if len(tasks) == 0 {
		return nil, errors.New("empty decomposition tasks")
	}
	return tasks, nil
}

func splitTaskForOrchestration(input string, maxShards int) []string {
	task := strings.TrimSpace(input)
	if task == "" {
		return []string{""}
	}
	if maxShards <= 1 {
		return []string{task}
	}
	if maxShards > 100 {
		maxShards = 100
	}

	if shards := normalizeOrchestrateShards(splitTaskByLines(task), maxShards); len(shards) >= 2 {
		return shards
	}
	if shards := normalizeOrchestrateShards(splitTaskByConnectors(task), maxShards); len(shards) >= 2 {
		return shards
	}
	if shards := normalizeOrchestrateShards(splitTaskBySentences(task), maxShards); len(shards) >= 2 {
		return shards
	}
	return []string{task}
}

func splitTaskByLines(task string) []string {
	lines := strings.Split(task, "\n")
	subtasks := make([]string, 0, len(lines))
	var current string

	appendCurrent := func() {
		if strings.TrimSpace(current) == "" {
			return
		}
		subtasks = append(subtasks, strings.TrimSpace(current))
		current = ""
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Treat indented lines and explicit continuation prefixes as a continuation of
		// the previous list item, unless they begin with a list marker themselves.
		if current != "" && isOrchestrateListContinuation(line) {
			current = strings.TrimSpace(current + " " + trimmed)
			continue
		}

		appendCurrent()
		current = trimmedOrchestrateListMarker(line)
	}

	appendCurrent()
	return subtasks
}

func isOrchestrateListContinuation(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if hasOrchestrateListMarker(line) {
		return false
	}
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return true
	}
	return hasOrchestrateContinuationPrefix(trimmed)
}

func splitTaskByConnectors(task string) []string {
	best := struct {
		left  string
		right string
		ok    bool
	}{}
	if strings.TrimSpace(task) == "" {
		return nil
	}

	low := strings.ToLower(task)
	separators := []string{" and then ", " and ", ", then ", ";", " then ", " plus ", "\n"}

	for _, sep := range separators {
		start := 0
		for {
			idx := strings.Index(low[start:], sep)
			if idx < 0 {
				break
			}
			idx += start
			rawLeft := strings.TrimSpace(task[:idx])
			rawRight := strings.TrimSpace(task[idx+len(sep):])

			if strings.EqualFold(sep, ";") || strings.EqualFold(sep, "\n") {
				rawLeft = strings.TrimSuffix(rawLeft, ",")
			}

			if !isLikelyTopLevelOrchestrateSeparator(task, rawLeft, rawRight, idx, len(sep)) {
				start = idx + len(sep)
				continue
			}
			if countOrchestrateWords(rawLeft) < 2 || countOrchestrateWords(rawRight) < 2 {
				start = idx + len(sep)
				continue
			}

			leftLen := len(rawLeft)
			rightLen := len(rawRight)
			if !best.ok || absInt(leftLen-rightLen) < absInt(len(best.left)-len(best.right)) {
				best = struct {
					left  string
					right string
					ok    bool
				}{left: rawLeft, right: rawRight, ok: true}
			}

			start = idx + len(sep)
		}
	}

	if best.ok {
		return []string{best.left, best.right}
	}
	return nil
}

func splitTaskBySentences(task string) []string {
	rest := strings.TrimSpace(task)
	if rest == "" {
		return nil
	}

	trimSentence := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.TrimRight(s, ".!?;")
		return strings.TrimSpace(s)
	}

	var parts []string
	for len(parts) < 200 {
		splitIdx := -1
		splitLen := 0
		for i := 0; i+1 < len(rest); i++ {
			ch := rest[i]
			if ch != '.' && ch != '!' && ch != '?' {
				continue
			}
			if next := rest[i+1]; next != ' ' && next != '\n' && next != '\t' && next != '\r' {
				continue
			}
			left := strings.TrimSpace(rest[:i])
			right := strings.TrimSpace(rest[i+1:])
			if left == "" || right == "" {
				continue
			}
			if !isLikelyTopLevelOrchestrateSeparator(rest, left, right, i, 1) {
				continue
			}
			splitIdx = i
			splitLen = 1
			break
		}
		if splitIdx < 0 {
			break
		}

		left := strings.TrimSpace(rest[:splitIdx])
		right := strings.TrimSpace(rest[splitIdx+splitLen:])
		if left = trimSentence(left); left != "" {
			parts = append(parts, left)
		}
		rest = right
		if rest == "" {
			break
		}
	}

	if rest = trimSentence(rest); rest != "" {
		parts = append(parts, rest)
	}
	return parts
}

func normalizeOrchestrateShards(subtasks []string, max int) []string {
	if max <= 0 {
		max = 1
	}
	seen := map[string]bool{}
	out := make([]string, 0, max)
	for _, subtask := range subtasks {
		clean := strings.TrimSpace(subtask)
		if clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		if countOrchestrateWords(clean) < 2 {
			continue
		}
		seen[key] = true
		out = append(out, clean)
		if len(out) >= max {
			break
		}
	}
	if len(out) < 2 {
		return out
	}
	return out
}

func countOrchestrateWords(text string) int {
	return len(strings.Fields(text))
}

func trimmedOrchestrateListMarker(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	matches := orchestrateListMarkerRE.FindStringSubmatch(trimmed)
	if len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return trimmed
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func hasOrchestrateListMarker(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	return orchestrateListMarkerRE.MatchString(line)
}

func normalizeOrchestrateContextForCache(input string) string {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return ""
	}
	if len(fields) > defaultOrchestrateContextHashWordsMax {
		fields = fields[:defaultOrchestrateContextHashWordsMax]
	}
	return strings.ToLower(strings.Join(fields, " "))
}

func isLikelyTopLevelOrchestrateSeparator(task, left, right string, index, sepLen int) bool {
	if sepLen <= 0 {
		return false
	}
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	if countOrchestrateWords(left) < 2 || countOrchestrateWords(right) < 2 {
		return false
	}
	if hasOpenOrchestrateDelimiters(task[:index]) || hasOpenOrchestrateDelimiters(task[index+sepLen:]) {
		return false
	}
	if hasUnclosedQuote(task[:index]) || hasUnclosedQuote(task[index+sepLen:]) {
		return false
	}
	leftTrimmed := strings.TrimSpace(left)
	rightTrimmed := strings.TrimSpace(right)
	if hasOrchestrateContinuationPrefix(leftTrimmed) || hasOrchestrateContinuationPrefix(rightTrimmed) {
		return false
	}
	if hasOrchestrateDependencyStart(leftTrimmed) || hasOrchestrateDependencyStart(rightTrimmed) {
		return false
	}
	if hasOrchestrateDependencyMidClause(rightTrimmed) {
		return false
	}
	if isAnaphoricOrchestrateBoundary(leftTrimmed) || isAnaphoricOrchestrateBoundary(rightTrimmed) {
		return false
	}
	if strings.HasSuffix(leftTrimmed, ",") || strings.HasSuffix(leftTrimmed, ";") || strings.HasSuffix(leftTrimmed, " and") || strings.HasSuffix(leftTrimmed, " or") {
		return false
	}
	return true
}

func hasOrchestrateDependencyStart(input string) bool {
	input = strings.ToLower(strings.TrimSpace(input))
	for _, prefix := range []string{
		"if ",
		"when ",
		"unless ",
		"until ",
		"while ",
		"before ",
		"after ",
		"since ",
		"because ",
		"in case ",
		"despite ",
		"although ",
		"as soon as ",
		"as long as ",
		"once ",
		"provided ",
		"except ",
	} {
		if strings.HasPrefix(input, prefix) {
			return true
		}
	}
	return false
}

func hasOrchestrateDependencyMidClause(input string) bool {
	low := strings.ToLower(input)
	for i, ch := range low {
		switch ch {
		case ';', ',', '.', '\n':
			rest := strings.TrimSpace(low[i+1:])
			if hasOrchestrateDependencyStart(rest) {
				return true
			}
		}
	}
	return false
}

func isAnaphoricOrchestrateBoundary(input string) bool {
	input = strings.ToLower(strings.TrimSpace(input))
	for _, prefix := range []string{
		"it ",
		"it's ",
		"its ",
		"this ",
		"that ",
		"these ",
		"those ",
		"them ",
		"their ",
		"there ",
		"then ",
	} {
		if strings.HasPrefix(input, prefix) {
			return true
		}
	}
	return false
}

func hasOrchestrateContinuationPrefix(input string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(input))
	if trimmed == "" {
		return false
	}
	for _, prefix := range []string{
		"and then ",
		"after ",
		"after that ",
		"as long as ",
		"as soon as ",
		"before ",
		"because ",
		"despite ",
		"even if ",
		"even though ",
		"if ",
		"in case ",
		"in order to ",
		"once ",
		"since ",
		"so ",
		"though ",
		"unless ",
		"until ",
		"when ",
		"while ",
	} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func hasOpenOrchestrateDelimiters(s string) bool {
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	for _, ch := range s {
		switch ch {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		}
	}
	return parenDepth > 0 || bracketDepth > 0 || braceDepth > 0
}

func hasUnclosedQuote(s string) bool {
	inSingle := false
	inDouble := false
	var prev rune
	for _, ch := range s {
		switch ch {
		case '\'':
			if !inDouble && prev != '\\' {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && prev != '\\' {
				inDouble = !inDouble
			}
		}
		prev = ch
	}
	return inSingle || inDouble
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func buildOrchestrateSubtaskRetryPrompt(subtask, fullTask string, attempt int) string {
	if strings.TrimSpace(subtask) == "" {
		subtask = "continue the requested task"
	}
	fullTask = strings.TrimSpace(fullTask)
	if attempt <= 1 {
		return buildOrchestrateSubtaskPrompt(1, 1, subtask, fullTask)
	}
	return fmt.Sprintf("Retry only the failed subtask. Return only the concrete output for this item.\n\nSubtask:\n%s", subtask)
}

var orchestrateListMarkerRE = regexp.MustCompile(`(?i)^\s*(?:(?:[-*+•]\s*(?:\[[ xX]\]\s*)?|\d+\s*[\.)]|\d+\s*[-–—]|\([a-zA-Z0-9]{1,4}\)|[a-zA-Z][.)]|[a-zA-Z]\)|\bstep\s+\d+[:\.\)]))\s+(.*)$`)

func buildOrchestrateSubtaskPrompt(index, total int, subtask, fullTask string) string {
	subtask = strings.TrimSpace(subtask)
	if total <= 1 {
		return subtask
	}
	return fmt.Sprintf("Subtask %d/%d:\n%s", index, total, subtask)
}

func (a *Application) RunCommand(ctx context.Context, command string, background bool) (Job, int, error) {
	if background {
		job, err := a.Runner.RunBackground(ctx, command, a.Jobs)
		return job, -1, err
	}
	code, err := a.Runner.Run(ctx, command)
	return Job{}, code, err
}

// ReloadClient updates the client with new configuration
func (a *Application) ReloadClient(cfg Config) {
	a.Config = cfg
	a.Client = NewMinimaxClient(cfg.APIKey, cfg.Model, cfg.BaseURL, cfg.MaxTokens)
}
