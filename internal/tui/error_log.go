package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cli-agent/internal/app"
)

type tuiErrorLogEntry struct {
	Timestamp string `json:"timestamp"`
	Context   string `json:"context,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message"`
}

func tuiErrorLogPath() string {
	if p := strings.TrimSpace(os.Getenv("EAI_TUI_ERROR_LOG")); p != "" {
		return p
	}
	cfgPath := app.DefaultConfigPath()
	if strings.TrimSpace(cfgPath) == "" {
		return filepath.Join(os.TempDir(), "cli-agent", "error.log")
	}
	return filepath.Join(filepath.Dir(cfgPath), "error.log")
}

func appendTUIErrorLog(context, sessionID, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	path := tuiErrorLogPath()
	if path == "" {
		return
	}

	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	entry := tuiErrorLogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Context:   strings.TrimSpace(context),
		SessionID: strings.TrimSpace(sessionID),
		Message:   message,
	}
	b, _ := json.Marshal(entry)
	b = append(b, '\n')
	_, _ = f.Write(b)
}
