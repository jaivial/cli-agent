package app

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func DefaultLogPath() string {
	if p := strings.TrimSpace(os.Getenv("EAI_LOG_PATH")); p != "" {
		return p
	}
	if len(os.Args) > 0 && strings.HasSuffix(os.Args[0], ".test") {
		return filepath.Join(os.TempDir(), "cli-agent", "app.test.log")
	}
	cfgPath := DefaultConfigPath()
	if strings.TrimSpace(cfgPath) != "" {
		return filepath.Join(filepath.Dir(cfgPath), "app.log")
	}
	return filepath.Join(os.TempDir(), "cli-agent", "app.log")
}

func DefaultLogWriter() io.Writer {
	path := strings.TrimSpace(DefaultLogPath())
	if path == "" {
		return io.Discard
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return io.Discard
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return io.Discard
	}

	// Bubble Tea owns the screen in TUI mode, so don't log to the terminal by
	// default. Set EAI_LOG_STDERR=1 to also mirror logs to stderr.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("EAI_LOG_STDERR"))); v != "" && v != "0" && v != "false" && v != "off" && v != "no" {
		return io.MultiWriter(f, os.Stderr)
	}
	return f
}

type Logger struct {
	out io.Writer
}

type LogEvent struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

func NewLogger(out io.Writer) *Logger {
	return &Logger{out: out}
}

func (l *Logger) Info(message string, fields map[string]interface{}) {
	l.write("info", message, fields)
}

func (l *Logger) Error(message string, fields map[string]interface{}) {
	l.write("error", message, fields)
}

func (l *Logger) Debug(message string, fields map[string]interface{}) {
	l.write("debug", message, fields)
}

func (l *Logger) Warn(message string, fields map[string]interface{}) {
	l.write("warn", message, fields)
}

func (l *Logger) write(level, message string, fields map[string]interface{}) {
	evt := LogEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}
	payload, _ := json.Marshal(evt)
	payload = append(payload, '\n')
	_, _ = l.out.Write(payload)
}
