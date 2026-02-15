package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cli-agent/internal/app"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	desktopDefaultMode = string(app.ModeOrchestrate)
)

type App struct {
	ctx     context.Context
	mu      sync.Mutex
	ready   bool
	readyAt time.Time

	application *app.Application

	workDir   string
	sessionID string

	activeRunID string
	runCancel   context.CancelFunc
}

type DesktopRunStatus struct {
	RunID               string    `json:"run_id"`
	Status              string    `json:"status"`
	Ready               bool      `json:"ready"`
	ReadyAt             time.Time `json:"ready_at"`
	APIKeyConfigured    bool      `json:"api_key_configured"`
	Mode                string    `json:"mode"`
	HasTmux             bool      `json:"has_tmux"`
	MaxParallelAgents   int       `json:"max_parallel_agents"`
	OrchestrateParallel int       `json:"orchestrate_parallel"`
	Model               string    `json:"model"`
	BaseURL             string    `json:"base_url"`
}

type DesktopProgressEvent struct {
	RunID string `json:"run_id"`
	app.ProgressEvent
}

type DesktopChatMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type DesktopSessionCard struct {
	ID          string    `json:"id"`
	RootID      string    `json:"root_id"`
	Title       string    `json:"title"`
	LastActivity time.Time `json:"last_activity"`
	MessageCount int       `json:"message_count"`
}

func NewApp() *App {
	return &App{readyAt: time.Now()}
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()

	if err := a.ensureApplication(); err != nil {
		runtime.EventsEmit(a.ctx, "desktop:status", map[string]interface{}{
			"ready":    false,
			"error":    err.Error(),
			"ready_at": time.Now().Format(time.RFC3339),
		})
		return
	}

	a.emitStatus("ready")
	a.emitSessions()

	a.emitProgress("", app.ProgressEvent{
		Kind: "system",
		Text: "CLI Agent desktop initialized",
		At:   time.Now(),
	})
}

func envOrInt(name string, fallback int) int {
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

func (a *App) applyDesktopRuntimeDefaults() {
	const defaultOrchestrateParallel = 20
	const defaultMaxShards = 20

	// Prefer tmux headless panes when available (desktop is not launched inside tmux).
	// Fall back to the in-process orchestrator when we can't locate a worker binary.
	if _, err := exec.LookPath("tmux"); err == nil {
		if worker, werr := exec.LookPath("eai"); werr == nil && strings.TrimSpace(worker) != "" {
			os.Setenv("EAI_TMUX_DISABLE", "")
			os.Setenv("EAI_TMUX_HEADLESS", "1")
			os.Setenv("EAI_TMUX_HEADLESS_SESSION", "eai-desktop")
			os.Setenv("EAI_ORCHESTRATE_WORKER_BIN", worker)
		} else {
			os.Setenv("EAI_TMUX_DISABLE", "1")
		}
	} else {
		os.Setenv("EAI_TMUX_DISABLE", "1")
	}

	// Enable companion discovery by default in the desktop app.
	if _, ok := os.LookupEnv("EAI_TOOL_COMPANIONS"); !ok {
		os.Setenv("EAI_TOOL_COMPANIONS", "1")
	}
	// Desktop prioritizes responsiveness; allow an extra (small) LLM call to decompose
	// short-but-large requests into enough shards to keep all panes busy.
	if _, ok := os.LookupEnv("EAI_ORCHESTRATE_LLM_DECOMPOSE"); !ok {
		os.Setenv("EAI_ORCHESTRATE_LLM_DECOMPOSE", "1")
	}

	if _, ok := os.LookupEnv("EAI_ORCHESTRATE_ACTIVE_PANES"); !ok {
		os.Setenv("EAI_ORCHESTRATE_ACTIVE_PANES", strconv.Itoa(defaultOrchestrateParallel))
	}
	if _, ok := os.LookupEnv("EAI_ORCHESTRATE_MAX_PANES_PER_TASK"); !ok {
		os.Setenv("EAI_ORCHESTRATE_MAX_PANES_PER_TASK", strconv.Itoa(defaultOrchestrateParallel))
	}
	if _, ok := os.LookupEnv("EAI_ORCHESTRATE_MAX_SHARDS"); !ok {
		os.Setenv("EAI_ORCHESTRATE_MAX_SHARDS", strconv.Itoa(defaultMaxShards))
	}

	parallel := envOrInt("EAI_ORCHESTRATE_ACTIVE_PANES", defaultOrchestrateParallel)
	if a.application != nil {
		a.application.Config.MaxParallelAgents = maxInt(a.application.Config.MaxParallelAgents, parallel)
	}
}

func maxInt(a, b int) int {
	if a >= b {
		return a
	}
	return b
}

func (a *App) ensureApplication() error {
	a.mu.Lock()
	if a.application != nil && a.ready {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	cfg, err := app.LoadConfig(app.DefaultConfigPath())
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		// keep empty API key for now; user can set it from UI.
	}

	application, appErr := app.NewApplication(cfg, false)
	if appErr != nil {
		fallback, _ := app.NewApplication(cfg, true)
		application = fallback
	}

	a.mu.Lock()
	a.application = application
	a.applyDesktopRuntimeDefaults()
	a.ready = true
	a.readyAt = time.Now()
	a.mu.Unlock()

	// Initialize (or resume) a persisted session for continuity.
	if application != nil {
		wd, _ := os.Getwd()
		if strings.TrimSpace(wd) == "" {
			wd = "."
		}
		if sess, _, err := application.LoadOrCreateSession(wd); err == nil && sess != nil {
			a.mu.Lock()
			a.workDir = wd
			a.sessionID = sess.ID
			a.mu.Unlock()
		} else {
			a.mu.Lock()
			a.workDir = wd
			a.mu.Unlock()
		}
	}

	if application != nil {
		runtime.EventsEmit(a.ctx, "desktop:status", a.GetStatus())
	}
	return nil
}

func (a *App) buildStatus() DesktopRunStatus {
	st := DesktopRunStatus{
		Mode:                desktopDefaultMode,
		HasTmux:             false,
		MaxParallelAgents:   1,
		OrchestrateParallel: envOrInt("EAI_ORCHESTRATE_ACTIVE_PANES", 0),
		Model:               "",
		BaseURL:             "",
		RunID:               a.activeRunID,
		ReadyAt:             a.readyAt,
	}
	if a.application == nil {
		return st
	}
	cfg := a.application.Config
	if cfg.APIKey != "" {
		st.APIKeyConfigured = true
	}
	if strings.TrimSpace(cfg.Model) != "" {
		st.Model = cfg.Model
	}
	if strings.TrimSpace(cfg.BaseURL) != "" {
		st.BaseURL = cfg.BaseURL
	}
	if cfg.MaxParallelAgents > 0 {
		st.MaxParallelAgents = cfg.MaxParallelAgents
	}
	if strings.TrimSpace(os.Getenv("TMUX")) != "" || strings.TrimSpace(os.Getenv("EAI_TMUX_HEADLESS")) == "1" {
		st.HasTmux = true
	}
	return st
}

func (a *App) emitStatus(status string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ctx == nil {
		return
	}
	st := a.buildStatus()
	st.Status = status
	st.Ready = a.ready
	runtime.EventsEmit(a.ctx, "desktop:status", st)
}

func (a *App) emitProgress(runID string, ev app.ProgressEvent) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, "desktop:progress", DesktopProgressEvent{RunID: runID, ProgressEvent: ev})
}

func (a *App) emitSessions() {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return
	}
	sessions, err := a.ListSessions(10)
	if err != nil {
		return
	}
	runtime.EventsEmit(ctx, "desktop:sessions", sessions)
}

func (a *App) GetStatus() DesktopRunStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.application == nil {
		_ = a.ensureApplication()
	}
	st := a.buildStatus()
	st.Ready = a.ready
	st.Status = "ready"
	return st
}

func (a *App) GetChatHistory() ([]DesktopChatMessage, error) {
	if err := a.ensureApplication(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	sid := strings.TrimSpace(a.sessionID)
	wd := strings.TrimSpace(a.workDir)
	application := a.application
	a.mu.Unlock()

	if application == nil {
		return nil, fmt.Errorf("application not ready")
	}
	if wd == "" {
		wd, _ = os.Getwd()
		if wd == "" {
			wd = "."
		}
	}
	if sid == "" {
		if sess, _, err := application.LoadOrCreateSession(wd); err == nil && sess != nil {
			sid = sess.ID
		}
	}
	if strings.TrimSpace(sid) == "" {
		return []DesktopChatMessage{}, nil
	}

	_, msgs, err := application.LoadSession(wd, sid)
	if err != nil {
		return nil, err
	}
	out := make([]DesktopChatMessage, 0, len(msgs))
	for _, m := range msgs {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		out = append(out, DesktopChatMessage{
			ID:        m.ID,
			Role:      role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		})
	}

	a.mu.Lock()
	a.workDir = wd
	a.sessionID = sid
	a.mu.Unlock()

	return out, nil
}

func (a *App) ListSessions(limit int) ([]DesktopSessionCard, error) {
	if err := a.ensureApplication(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	wd := strings.TrimSpace(a.workDir)
	application := a.application
	a.mu.Unlock()

	if application == nil {
		return nil, fmt.Errorf("application not ready")
	}
	if wd == "" {
		wd, _ = os.Getwd()
		if wd == "" {
			wd = "."
		}
	}

	summaries, err := application.ListRecentSessions(wd, limit)
	if err != nil {
		return nil, err
	}

	out := make([]DesktopSessionCard, 0, len(summaries))
	for _, s := range summaries {
		sess := s.Session
		title := strings.TrimSpace(sess.Title)
		out = append(out, DesktopSessionCard{
			ID:           sess.ID,
			RootID:       sess.RootID,
			Title:        title,
			LastActivity: s.LastActivity,
			MessageCount: s.MessageCount,
		})
	}
	return out, nil
}

func tmuxHeadlessSessionName() string {
	if v := strings.TrimSpace(os.Getenv("EAI_TMUX_HEADLESS_SESSION")); v != "" {
		return v
	}
	return "eai-headless"
}

func (a *App) ensureHeadlessTmuxSession(ctx context.Context) (string, error) {
	name := tmuxHeadlessSessionName()
	if name == "" {
		return "", errors.New("invalid headless tmux session name")
	}
	if err := exec.CommandContext(ctx, "tmux", "has-session", "-t", name).Run(); err == nil {
		return name, nil
	}
	if err := exec.CommandContext(ctx, "tmux", "new-session", "-d", "-s", name).Run(); err != nil {
		return "", err
	}
	return name, nil
}

func tmuxSafeWindowName(rootID string) string {
	rootID = strings.TrimSpace(rootID)
	if rootID == "" {
		return "chat"
	}
	var b strings.Builder
	b.Grow(len(rootID))
	for _, r := range rootID {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	name := "chat-" + b.String()
	if len(name) > 48 {
		name = name[:48]
	}
	name = strings.TrimRight(name, "-")
	if name == "" {
		return "chat"
	}
	return name
}

func (a *App) ensureSessionTmuxWindow(ctx context.Context, rootID string) (string, error) {
	if strings.TrimSpace(os.Getenv("EAI_TMUX_DISABLE")) == "1" {
		return "", nil
	}
	if strings.TrimSpace(os.Getenv("EAI_TMUX_HEADLESS")) != "1" {
		return "", nil
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return "", nil
	}

	sessName, err := a.ensureHeadlessTmuxSession(ctx)
	if err != nil {
		return "", err
	}
	windowName := tmuxSafeWindowName(rootID)
	target := sessName + ":" + windowName

	out, err := exec.CommandContext(ctx, "tmux", "list-windows", "-t", sessName, "-F", "#{window_name}").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == windowName {
				return target, nil
			}
		}
	}

	// Create the window detached so it persists as long as the tmux session is alive.
	if err := exec.CommandContext(ctx, "tmux", "new-window", "-d", "-t", sessName, "-n", windowName).Run(); err != nil {
		return "", err
	}
	return target, nil
}

func (a *App) killSessionTmuxWindow(ctx context.Context, rootID string) {
	if strings.TrimSpace(os.Getenv("EAI_TMUX_HEADLESS")) != "1" {
		return
	}
	if strings.TrimSpace(os.Getenv("EAI_TMUX_DISABLE")) == "1" {
		return
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return
	}
	name := tmuxHeadlessSessionName()
	windowName := tmuxSafeWindowName(rootID)
	target := name + ":" + windowName
	_ = exec.CommandContext(ctx, "tmux", "kill-window", "-t", target).Run()
}

func (a *App) CreateNewSession() (string, error) {
	if err := a.ensureApplication(); err != nil {
		return "", err
	}

	a.mu.Lock()
	wd := strings.TrimSpace(a.workDir)
	application := a.application
	if a.runCancel != nil {
		a.runCancel()
		a.runCancel = nil
		a.activeRunID = ""
	}
	a.mu.Unlock()

	if application == nil {
		return "", fmt.Errorf("application not ready")
	}
	if wd == "" {
		wd, _ = os.Getwd()
		if wd == "" {
			wd = "."
		}
	}

	sess, err := application.CreateSession(wd)
	if err != nil {
		return "", err
	}
	if sess != nil {
		if target, err := a.ensureSessionTmuxWindow(context.Background(), sess.RootID); err == nil && target != "" {
			os.Setenv("EAI_TMUX_TARGET", target)
		}
	}

	a.mu.Lock()
	a.workDir = wd
	if sess != nil {
		a.sessionID = sess.ID
	} else {
		a.sessionID = ""
	}
	a.mu.Unlock()

	a.emitSessions()
	return a.sessionID, nil
}

func (a *App) SwitchSession(sessionID string) ([]DesktopChatMessage, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("missing sessionID")
	}
	if err := a.ensureApplication(); err != nil {
		return nil, err
	}

	a.mu.Lock()
	wd := strings.TrimSpace(a.workDir)
	application := a.application
	a.mu.Unlock()

	if application == nil {
		return nil, fmt.Errorf("application not ready")
	}
	if wd == "" {
		wd, _ = os.Getwd()
		if wd == "" {
			wd = "."
		}
	}

	sess, msgs, err := application.LoadSession(wd, sessionID)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, fmt.Errorf("session not found")
	}

	_ = application.Memory.SetCurrentSession(wd, sess.ID)
	_ = application.Memory.TouchSession(wd, sess.ID)

	if target, err := a.ensureSessionTmuxWindow(context.Background(), sess.RootID); err == nil && target != "" {
		os.Setenv("EAI_TMUX_TARGET", target)
	}

	out := make([]DesktopChatMessage, 0, len(msgs))
	for _, m := range msgs {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		out = append(out, DesktopChatMessage{
			ID:        m.ID,
			Role:      role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		})
	}

	a.mu.Lock()
	a.workDir = wd
	a.sessionID = sess.ID
	a.mu.Unlock()

	a.emitSessions()
	return out, nil
}

func (a *App) DeleteSession(sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", fmt.Errorf("missing sessionID")
	}
	if err := a.ensureApplication(); err != nil {
		return "", err
	}

	a.mu.Lock()
	wd := strings.TrimSpace(a.workDir)
	application := a.application
	currentID := strings.TrimSpace(a.sessionID)
	a.mu.Unlock()

	if application == nil {
		return "", fmt.Errorf("application not ready")
	}
	if wd == "" {
		wd, _ = os.Getwd()
		if wd == "" {
			wd = "."
		}
	}

	sess, _, err := application.LoadSession(wd, sessionID)
	if err != nil {
		return "", err
	}
	if sess == nil {
		return "", fmt.Errorf("session not found")
	}

	rootID := strings.TrimSpace(sess.RootID)
	if rootID == "" {
		rootID = sess.ID
	}

	if err := application.DeleteSession(wd, sessionID); err != nil {
		return "", err
	}

	a.killSessionTmuxWindow(context.Background(), rootID)

	// If we deleted the active session, roll forward to a new/current one.
	if currentID == sessionID {
		if next, _, err := application.LoadOrCreateSession(wd); err == nil && next != nil {
			a.mu.Lock()
			a.sessionID = next.ID
			a.mu.Unlock()
		} else {
			a.mu.Lock()
			a.sessionID = ""
			a.mu.Unlock()
		}
	}

	a.emitSessions()
	return "deleted", nil
}

func (a *App) GetSupportedModels() []string {
	return append([]string(nil), app.SupportedModels...)
}

func (a *App) GetContextUsage(draft string) (app.ContextUsage, error) {
	if err := a.ensureApplication(); err != nil {
		return app.ContextUsage{}, err
	}

	a.mu.Lock()
	wd := strings.TrimSpace(a.workDir)
	sid := strings.TrimSpace(a.sessionID)
	application := a.application
	a.mu.Unlock()

	if application == nil {
		return app.ContextUsage{}, fmt.Errorf("application not ready")
	}
	if wd == "" {
		wd, _ = os.Getwd()
		if wd == "" {
			wd = "."
		}
	}
	if sid == "" {
		if sess, _, err := application.LoadOrCreateSession(wd); err == nil && sess != nil {
			sid = sess.ID
		}
	}

	return application.EstimateContextUsageForTurn(wd, sid, app.ModeOrchestrate, strings.TrimSpace(draft))
}

func (a *App) SetPermissions(mode string) string {
	if err := a.ensureApplication(); err != nil {
		return err.Error()
	}
	normalized, ok := app.ParsePermissionsMode(mode)
	if !ok {
		return "invalid permissions mode"
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.application == nil {
		return "application not ready"
	}
	cfg := a.application.Config
	cfg.Permissions = normalized
	if err := app.SaveConfig(cfg, app.DefaultConfigPath()); err != nil {
		return err.Error()
	}
	a.application.Config = cfg
	os.Setenv("EAI_PERMISSIONS", normalized)
	a.application.ReloadClient(cfg)
	return "stored"
}

func readRecentWarnErrorLogs(path string, limit int) ([]app.LogEvent, error) {
	if limit <= 0 {
		limit = 40
	}
	if limit > 500 {
		limit = 500
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var events []app.LogEvent
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var ev app.LogEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}

		lvl := strings.ToLower(strings.TrimSpace(ev.Level))
		if lvl != "warn" && lvl != "error" {
			continue
		}

		events = append(events, ev)
		if len(events) > limit*5 {
			events = events[len(events)-limit*2:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events, nil
}

func formatWarnErrorLogEvent(ev app.LogEvent) string {
	ts := strings.TrimSpace(ev.Timestamp)
	lvl := strings.ToUpper(strings.TrimSpace(ev.Level))
	msg := strings.TrimSpace(ev.Message)

	parts := []string{}
	if ts != "" {
		parts = append(parts, ts)
	}
	if lvl != "" {
		parts = append(parts, "["+lvl+"]")
	}
	if msg != "" {
		parts = append(parts, msg)
	}
	line := strings.Join(parts, " ")
	line = strings.ReplaceAll(line, "\r", " ")
	line = strings.ReplaceAll(line, "\n", " ")
	line = strings.Join(strings.Fields(line), " ")
	return line
}

func (a *App) GetRecentLogs(limit int) (string, error) {
	if err := a.ensureApplication(); err != nil {
		return "", err
	}
	logPath := app.DefaultLogPath()
	events, err := readRecentWarnErrorLogs(logPath, limit)
	if err != nil {
		return "", err
	}
	if len(events) == 0 {
		return "no warn/error logs found.", nil
	}
	var b strings.Builder
	b.WriteString("recent logs (warn/error):\n")
	for i, ev := range events {
		b.WriteString(formatWarnErrorLogEvent(ev))
		if i < len(events)-1 {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func (a *App) SendPrompt(prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt is empty")
	}
	if err := a.ensureApplication(); err != nil {
		return "", err
	}

	a.mu.Lock()
	sid := strings.TrimSpace(a.sessionID)
	wd := strings.TrimSpace(a.workDir)
	application := a.application
	a.mu.Unlock()

	if application == nil {
		return "", fmt.Errorf("application not ready")
	}
	if wd == "" {
		wd, _ = os.Getwd()
		if wd == "" {
			wd = "."
		}
	}
	if sid == "" {
		if sess, _, err := application.LoadOrCreateSession(wd); err == nil && sess != nil {
			sid = sess.ID
		}
	}

	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	runCtx, cancel := context.WithCancel(context.Background())

	a.mu.Lock()
	if a.runCancel != nil {
		a.runCancel()
	}
	a.activeRunID = runID
	a.runCancel = cancel
	a.mu.Unlock()

	a.emitProgress(runID, app.ProgressEvent{Kind: "run_state", Text: "started", At: time.Now()})

	progress := func(ev app.ProgressEvent) {
		a.emitProgress(runID, ev)
	}

	// Ensure compaction/session chaining happens before persisting this user turn.
	if sess, err := application.PrepareSessionForTurn(runCtx, wd, sid, app.ModeOrchestrate, prompt, progress); err == nil && sess != nil {
		sid = sess.ID
		if target, err := a.ensureSessionTmuxWindow(runCtx, sess.RootID); err == nil && target != "" {
			os.Setenv("EAI_TMUX_TARGET", target)
		}
	}

	_ = application.AppendSessionMessage(sid, "user", prompt, app.ModeOrchestrate, wd)
	a.emitSessions()

	out, err := application.ExecuteChatInSessionWithProgressEvents(
		runCtx,
		sid,
		wd,
		app.ModeOrchestrate,
		prompt,
		progress,
		nil,
	)

	if err != nil {
		a.emitProgress(runID, app.ProgressEvent{Kind: "run_state", Text: "failed", Error: err.Error(), At: time.Now()})
	}
	a.emitProgress(runID, app.ProgressEvent{Kind: "run_state", Text: "completed", At: time.Now()})

	a.mu.Lock()
	a.runCancel = nil
	a.activeRunID = ""
	a.mu.Unlock()

	if err != nil && strings.Contains(strings.ToLower(err.Error()), "context canceled") {
		a.emitProgress(runID, app.ProgressEvent{Kind: "run_state", Text: "canceled", At: time.Now()})
		return out, nil
	}

	if err == nil {
		_ = application.AppendSessionMessage(sid, "assistant", out, app.ModeOrchestrate, wd)
	}
	a.emitSessions()

	a.mu.Lock()
	a.workDir = wd
	a.sessionID = sid
	a.mu.Unlock()

	return out, err
}

func (a *App) CancelCurrentRun() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runCancel == nil {
		return false
	}
	a.runCancel()
	a.runCancel = nil
	a.activeRunID = ""
	a.emitProgress("", app.ProgressEvent{Kind: "run_state", Text: "cancelled", At: time.Now()})
	return true
}

func (a *App) OpenInFileManager(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "missing path"
	}

	expanded := path
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if expanded == "~" {
				expanded = home
			} else {
				expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~/"))
			}
		}
	}

	if !filepath.IsAbs(expanded) {
		if wd, err := os.Getwd(); err == nil && wd != "" {
			expanded = filepath.Join(wd, expanded)
		}
	}
	expanded = filepath.Clean(expanded)

	info, err := os.Stat(expanded)
	if err != nil {
		return err.Error()
	}

	target := expanded
	if !info.IsDir() {
		target = filepath.Dir(expanded)
	}

	// Open the folder in the system file manager (Fedora/GNOME, KDE, etc.).
	cmd := exec.Command("xdg-open", target)
	if err := cmd.Start(); err != nil {
		return err.Error()
	}
	return "opened"
}

func (a *App) SetSudoPassword(password string) string {
	password = strings.TrimSpace(password)
	if password == "" {
		os.Unsetenv("EAI_DESKTOP_SUDO_PASSWORD")
		return "cleared"
	}
	os.Setenv("EAI_DESKTOP_SUDO_PASSWORD", password)
	a.emitProgress("", app.ProgressEvent{Kind: "system", Text: "sudo password stored for session", At: time.Now()})
	return "stored"
}

func (a *App) SetApiKey(apiKey string) string {
	if err := a.ensureApplication(); err != nil {
		return err.Error()
	}
	apiKey = strings.TrimSpace(apiKey)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.application == nil {
		return "application not ready"
	}

	a.application.Config.APIKey = apiKey
	if apiKey == "" {
		os.Unsetenv("EAI_API_KEY")
		os.Unsetenv("MINIMAX_API_KEY")
	} else {
		os.Setenv("EAI_API_KEY", apiKey)
		os.Setenv("MINIMAX_API_KEY", apiKey)
	}
	a.application.ReloadClient(a.application.Config)
	if apiKey != "" {
		return "stored"
	}
	return "cleared"
}

func (a *App) SetModel(model string) string {
	if err := a.ensureApplication(); err != nil {
		return err.Error()
	}
	model = app.NormalizeModel(strings.TrimSpace(model))
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.application == nil {
		return "application not ready"
	}
	a.application.Config.Model = model
	if n, ok := app.LookupContextWindowTokens(model); ok && n > 0 {
		a.application.Config.ContextWindowTokens = n
		os.Setenv("EAI_CONTEXT_WINDOW_TOKENS", strconv.Itoa(n))
	}
	os.Setenv("EAI_MODEL", model)
	a.application.ReloadClient(a.application.Config)
	return "stored"
}

func (a *App) SetBaseURL(baseURL string) string {
	if err := a.ensureApplication(); err != nil {
		return err.Error()
	}
	baseURL = app.NormalizeBaseURL(strings.TrimSpace(baseURL))
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.application == nil {
		return "application not ready"
	}
	a.application.Config.BaseURL = baseURL
	os.Setenv("EAI_BASE_URL", baseURL)
	a.application.ReloadClient(a.application.Config)
	return "stored"
}

func (a *App) SetOrchestrateParallel(parallel int) string {
	if err := a.ensureApplication(); err != nil {
		return err.Error()
	}
	if parallel < 1 || parallel > 50 {
		return "parallel must be between 1 and 50"
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.application == nil {
		return "application not ready"
	}

	a.application.Config.MaxParallelAgents = parallel
	os.Setenv("EAI_ORCHESTRATE_ACTIVE_PANES", strconv.Itoa(parallel))
	os.Setenv("EAI_ORCHESTRATE_MAX_PANES_PER_TASK", strconv.Itoa(parallel))
	os.Setenv("EAI_ORCHESTRATE_MAX_SHARDS", strconv.Itoa(parallel*2))

	return fmt.Sprintf("orchestrate parallel set to %d", parallel)
}

func (a *App) emitReady() {
	a.emitStatus("ready")
}
