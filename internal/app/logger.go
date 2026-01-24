package app

import (
	"encoding/json"
	"io"
	"time"
)

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
