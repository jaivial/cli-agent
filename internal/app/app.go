package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Application struct {
	Config   Config
	Logger   *Logger
	Client   *MinimaxClient
	Runner   *Runner
	Jobs     *JobStore
	Prompter *PromptBuilder
	Memory   *MemoryStore
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
	store, err := NewJobStore(jobPath)
	if err != nil {
		return nil, err
	}
	jobRoot := filepath.Join(os.TempDir(), "cli-agent", "logs")
	return &Application{
		Config:   cfg,
		Logger:   logger,
		Client:   client,
		Runner:   NewRunner(logger, jobRoot),
		Jobs:     store,
		Prompter: NewPromptBuilder(),
		Memory:   NewMemoryStore(""),
	}, nil
}

func (a *Application) LoadOrCreateSession(workDir string) (*Session, []StoredMessage, error) {
	if a.Memory == nil {
		a.Memory = NewMemoryStore("")
	}
	return a.Memory.LoadOrCreateCurrentSession(workDir)
}

func (a *Application) CreateSession(workDir string) (*Session, error) {
	if a.Memory == nil {
		a.Memory = NewMemoryStore("")
	}
	return a.Memory.CreateSession(workDir)
}

func (a *Application) LoadSession(workDir string, sessionID string) (*Session, []StoredMessage, error) {
	if a.Memory == nil {
		a.Memory = NewMemoryStore("")
	}
	return a.Memory.LoadSessionForWorkDir(workDir, sessionID)
}

func (a *Application) ListRecentSessions(workDir string, limit int) ([]SessionSummary, error) {
	if a.Memory == nil {
		a.Memory = NewMemoryStore("")
	}
	return a.Memory.ListSessionsForWorkDir(workDir, limit)
}

func (a *Application) LoadPromptHistory(workDir string) ([]string, error) {
	if a.Memory == nil {
		a.Memory = NewMemoryStore("")
	}
	return a.Memory.LoadPromptHistory(workDir)
}

func (a *Application) SavePromptHistory(workDir string, history []string) error {
	if a.Memory == nil {
		a.Memory = NewMemoryStore("")
	}
	return a.Memory.SavePromptHistory(workDir, history)
}

func (a *Application) AppendSessionMessage(sessionID string, role string, content string, mode Mode, workDir string) error {
	if a.Memory == nil {
		a.Memory = NewMemoryStore("")
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
	return nil
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

func sessionPromptSoftLimitChars() int {
	return positiveIntEnv("EAI_SESSION_PROMPT_SOFT_LIMIT_CHARS", defaultSessionPromptSoftLimitChars)
}

func truncateEllipsis(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "..."
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
	b.WriteString("Return concise Markdown only (no code fences), max 700 words.\n")
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
	return truncateEllipsis(b.String(), sessionSummaryMaxChars)
}

func (a *Application) compactSessionContext(ctx context.Context, mode Mode, input string, history []StoredMessage, existingSummary string) (string, error) {
	transcript := buildCompactionTranscript(history)
	if strings.TrimSpace(transcript) == "" && strings.TrimSpace(existingSummary) != "" {
		return truncateEllipsis(existingSummary, sessionSummaryMaxChars), nil
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
	return truncateEllipsis(out, sessionSummaryMaxChars), nil
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

func looksActionableForCreate(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return false
	}

	// Explicit "do something" verbs.
	actionVerbs := []string{
		"create", "make", "generate", "write", "save",
		"edit", "modify", "update", "change",
		"delete", "remove", "move", "rename",
		"install", "build", "run", "execute", "test",
	}
	for _, v := range actionVerbs {
		if strings.Contains(s, v+" ") || strings.HasPrefix(s, v) {
			return true
		}
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
		"hola", "buenas", "buenos dias", "buenas tardes", "buenas noches",
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
		if final != "" && !isGenericCompletionText(final) && !strings.Contains(finalLower, "did not complete within") && !strings.Contains(finalLower, "did not return any tool calls") {
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
		if strings.Contains(lower, "did not complete within") {
			return "I wasn't able to fully complete the task within the current step limit. I can continue from the current state if you want."
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
	// Local fastpath: scaffold a React site in create mode without requiring an API key.
	if mode == ModeCreate {
		if out, ok, err := tryLocalReactScaffold(input); ok {
			return out, err
		}
	}

	// Avoid spinning up the plan/tool agents for trivial greetings/smalltalk.
	if (mode == ModePlan || IsToolMode(mode)) && looksLikeTrivialChatTurn(input) {
		return trivialChatResponseForMode(mode), nil
	}

	// Plan mode: use a read-only discovery agent that always returns a plan.
	if mode == ModePlan {
		return a.executePlanModeWithProgressEvents(ctx, input, progress)
	}

	// Text-only modes: if user asks to do something, prompt them to switch to create mode.
	if !IsToolMode(mode) && mode != ModePlan && looksActionableForCreate(input) {
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
			task = input + "\n\nCRITICAL: Write the final website to index.html in the current working directory using the write_file tool. Do NOT run `ls -la` as a primary output; just write the file."
		}

		state, err := agent.Execute(toolCtx, task)
		if err != nil {
			return "", err
		}
		return renderAgentStateForChat(state), nil
	}

	// Interactive TUI chat should produce human-readable output, not raw tool-call JSON.
	prompt := a.Prompter.BuildChat(mode, input, a.Config.ChatVerbosity)
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
	if looksLikeWebsiteHTMLRequest(input) {
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

// ExecuteAgentTaskInSessionWithProgressEvents runs the tool agent with lightweight
// session memory (recent turns + optional summary) injected ahead of the current task.
// This is only used by the interactive TUI and does not affect `eai agent`.
func (a *Application) ExecuteAgentTaskInSessionWithProgressEvents(ctx context.Context, sessionID string, workDir string, task string, progress func(ProgressEvent), decisions <-chan PermissionDecision) (string, error) {
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
	agent.PermissionsMode = a.Config.Permissions
	agent.PermissionDecisions = decisions
	if wd, err := os.Getwd(); err == nil && wd != "" {
		agent.WorkDir = wd
	}
	if progress != nil {
		agent.Progress = progress
	}
	if strings.TrimSpace(prelude) != "" {
		agent.PreludeMessages = []AgentMessage{{
			Role:      "user",
			Content:   prelude,
			Timestamp: time.Now(),
		}}
	}

	state, err := agent.Execute(ctx, task)
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
	return a.ExecuteChatInSessionWithProgressEvents(ctx, sessionID, mode, input, progressEvents)
}

func (a *Application) ExecuteChatInSessionWithProgressEvents(
	ctx context.Context,
	sessionID string,
	mode Mode,
	input string,
	progress func(ProgressEvent),
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
		if sess, _, err := a.Memory.LoadSessionForWorkDir("", sessionID); err == nil && sess != nil {
			sessionInfo = sess
			sessionSummary = strings.TrimSpace(sess.ContextSummary)
		}
	}

	// Plan mode now runs a read-only tool agent. Inject lightweight context similarly to tool modes.
	toolSessionContext := strings.TrimSpace(os.Getenv("EAI_TOOL_SESSION_CONTEXT")) == "1"
	if toolSessionContext &&
		strings.TrimSpace(sessionID) != "" &&
		(len(history) > 0 || strings.TrimSpace(sessionSummary) != "") &&
		(IsToolMode(mode) || mode == ModePlan) &&
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
		// If this looks actionable, reuse existing behavior.
		if mode != ModeCreate && looksActionableForCreate(input) {
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
		promptText := buildSessionChatPrompt(systemPrompt, sessionSummary, history, input, historyLimit)

		if len(promptText) > sessionPromptSoftLimitChars() {
			if progress != nil {
				progress(ProgressEvent{
					Kind: "thinking",
					Text: "Compacting old session context before request",
				})
			}
			if compacted, compactErr := a.compactSessionContext(ctx, mode, input, history, sessionSummary); compactErr == nil {
				sessionSummary = strings.TrimSpace(compacted)
				if sessionInfo != nil && a.Memory != nil && sessionSummary != "" {
					sessionInfo.ContextSummary = sessionSummary
					_ = a.Memory.SaveSession(sessionInfo)
				}
				historyLimit = sessionPromptHistoryLimitCompacted
				promptText = buildSessionChatPrompt(systemPrompt, sessionSummary, history, input, historyLimit)
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
			compacted, compactErr := a.compactSessionContext(ctx, mode, input, history, sessionSummary)
			if compactErr == nil && strings.TrimSpace(compacted) != "" {
				sessionSummary = strings.TrimSpace(compacted)
				if sessionInfo != nil && a.Memory != nil {
					sessionInfo.ContextSummary = sessionSummary
					_ = a.Memory.SaveSession(sessionInfo)
				}

				// First retry: summary + short tail of recent turns.
				retryPrompt := buildSessionChatPrompt(systemPrompt, sessionSummary, history, input, sessionPromptHistoryLimitCompacted)
				out, err = completeChatWithProgress(ctx, a.Client, retryPrompt, progress, false)
				if err != nil && isContextOverflowError(err) {
					// Second retry: summary only + latest user turn.
					minimalPrompt := buildSessionChatPrompt(systemPrompt, sessionSummary, nil, input, 0)
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
	if agents <= 0 {
		return "", errors.New("agents must be > 0")
	}
	if agents > a.Config.MaxParallelAgents {
		agents = a.Config.MaxParallelAgents
	}

	shards := make([]TaskShard, 0, agents)
	for i := 0; i < agents; i++ {
		shards = append(shards, TaskShard{
			ID:     fmt.Sprintf("%d", i+1),
			Prompt: a.Prompter.Build(mode, fmt.Sprintf("Shard %d/%d: %s", i+1, agents, input)),
		})
	}
	orchestrator := NewOrchestrator(a.Client, a.Config.MaxParallelAgents)
	results, err := orchestrator.Run(ctx, shards)
	if err != nil {
		return "", err
	}
	return SynthesizeResults(results), nil
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
