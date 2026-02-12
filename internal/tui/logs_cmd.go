package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"cli-agent/internal/app"
)

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

	var extra []string
	if ev.Fields != nil {
		if v, ok := ev.Fields["response_length"]; ok {
			extra = append(extra, fmt.Sprintf("response_length=%v", v))
		}
		if v, ok := ev.Fields["finish_reason"]; ok {
			extra = append(extra, fmt.Sprintf("finish_reason=%v", v))
		}
		if v, ok := ev.Fields["attempts"]; ok {
			extra = append(extra, fmt.Sprintf("attempts=%v", v))
		}
	}
	if len(extra) > 0 {
		parts = append(parts, "("+strings.Join(extra, ", ")+")")
	}

	line := strings.Join(parts, " ")
	line = strings.ReplaceAll(line, "\r", " ")
	line = strings.ReplaceAll(line, "\n", " ")
	line = strings.Join(strings.Fields(line), " ")
	return line
}
