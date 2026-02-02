package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ToolExecutor is a function that executes a tool
type ToolExecutor func(ctx context.Context, args json.RawMessage, l *AgentLoop) ToolResult

var toolRegistry = make(map[string]ToolExecutor)

// RegisterTool registers a new tool executor
func RegisterTool(name string, executor ToolExecutor) {
	toolRegistry[name] = executor
}

// GetToolExecutor retrieves a tool executor by name
func GetToolExecutor(name string) (ToolExecutor, bool) {
	executor, ok := toolRegistry[name]
	return executor, ok
}

// init registers all built-in tools
func init() {
	RegisterTool("exec", executeExecTool)
	RegisterTool("read_file", executeReadFileTool)
	RegisterTool("write_file", executeWriteFileTool)
	RegisterTool("list_dir", executeListDirTool)
	RegisterTool("grep", executeGrepTool)
	RegisterTool("search_files", executeSearchFilesTool)
	RegisterTool("edit_file", executeEditFileTool)
}

// executeExecTool executes the exec tool
func executeExecTool(ctx context.Context, args json.RawMessage, l *AgentLoop) ToolResult {
	start := time.Now()
	result := ToolResult{Success: false}

	var execArgs struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &execArgs); err != nil {
		result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	timeout := agentCfg.DefaultTimeout
	if execArgs.Timeout > 0 {
		timeout = time.Duration(execArgs.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", execArgs.Command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = err.Error()
	}
	result.Output = string(output)
	result.Success = err == nil

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// executeReadFileTool executes the read_file tool
func executeReadFileTool(ctx context.Context, args json.RawMessage, l *AgentLoop) ToolResult {
	start := time.Now()
	result := ToolResult{Success: false}

	var readArgs struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &readArgs); err != nil {
		result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Validate path for security
	if err := validatePath(readArgs.Path); err != nil {
		result.Error = err.Error()
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	data, err := os.ReadFile(readArgs.Path)
	if err != nil {
		result.Error = err.Error()
	} else {
		result.Output = string(data)
		result.Success = true
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// executeWriteFileTool executes the write_file tool
func executeWriteFileTool(ctx context.Context, args json.RawMessage, l *AgentLoop) ToolResult {
	start := time.Now()
	result := ToolResult{Success: false}

	var writeArgs struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &writeArgs); err != nil {
		result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Validate path for security
	if err := validatePath(writeArgs.Path); err != nil {
		result.Error = err.Error()
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	dir := filepath.Dir(writeArgs.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Error = fmt.Sprintf("Failed to create directory: %v", err)
	} else if err := os.WriteFile(writeArgs.Path, []byte(writeArgs.Content), 0644); err != nil {
		result.Error = err.Error()
	} else {
		result.Output = fmt.Sprintf("File written: %s", writeArgs.Path)
		result.Success = true
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// executeListDirTool executes the list_dir tool
func executeListDirTool(ctx context.Context, args json.RawMessage, l *AgentLoop) ToolResult {
	start := time.Now()
	result := ToolResult{Success: false}

	var listArgs struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &listArgs); err != nil {
		result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	if listArgs.Path == "" {
		listArgs.Path = "."
	}

	entries, err := os.ReadDir(listArgs.Path)
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

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// executeGrepTool executes the grep tool with context cancellation
func executeGrepTool(ctx context.Context, args json.RawMessage, l *AgentLoop) ToolResult {
	start := time.Now()
	result := ToolResult{Success: false}

	// Check context before starting
	select {
	case <-ctx.Done():
		result.Error = "Operation cancelled"
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	default:
	}

	var grepArgs struct {
		Pattern   string `json:"pattern"`
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal(args, &grepArgs); err != nil {
		result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	if grepArgs.Path == "" {
		grepArgs.Path = "."
	}

	flags := "-rHn"
	if !grepArgs.Recursive {
		flags = "-Hn"
	}

	cmd := exec.CommandContext(ctx, "grep", flags, grepArgs.Pattern, grepArgs.Path)
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		result.Error = err.Error()
	} else {
		result.Output = string(output)
		result.Success = true
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// executeSearchFilesTool executes the search_files tool with context cancellation
func executeSearchFilesTool(ctx context.Context, args json.RawMessage, l *AgentLoop) ToolResult {
	start := time.Now()
	result := ToolResult{Success: false}

	// Check context before starting
	select {
	case <-ctx.Done():
		result.Error = "Operation cancelled"
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	default:
	}

	var searchArgs struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(args, &searchArgs); err != nil {
		result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	if searchArgs.Path == "" {
		searchArgs.Path = "."
	}

	cmd := exec.CommandContext(ctx, "find", searchArgs.Path, "-name", searchArgs.Pattern, "-type", "f")
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		result.Error = err.Error()
	} else {
		result.Output = string(output)
		result.Success = true
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// executeEditFileTool executes the edit_file tool
func executeEditFileTool(ctx context.Context, args json.RawMessage, l *AgentLoop) ToolResult {
	start := time.Now()
	result := ToolResult{Success: false}

	var editArgs struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal(args, &editArgs); err != nil {
		result.Error = fmt.Sprintf("Failed to parse arguments: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Validate path for security
	if err := validatePath(editArgs.Path); err != nil {
		result.Error = err.Error()
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	data, err := os.ReadFile(editArgs.Path)
	if err != nil {
		result.Error = err.Error()
	} else {
		content := string(data)
		if !strings.Contains(content, editArgs.OldText) {
			result.Error = fmt.Sprintf("Text not found in file: %s", editArgs.OldText)
		} else {
			newContent := strings.Replace(content, editArgs.OldText, editArgs.NewText, 1)
			if err := os.WriteFile(editArgs.Path, []byte(newContent), 0644); err != nil {
				result.Error = err.Error()
			} else {
				result.Output = fmt.Sprintf("File edited: %s", editArgs.Path)
				result.Success = true
			}
		}
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// validatePath checks for dangerous path patterns
func validatePath(path string) error {
	// Check for path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected: %s", path)
	}
	return nil
}

// executeToolWithRetry executes a tool with retry logic for transient failures
func (l *AgentLoop) executeToolWithRetry(ctx context.Context, call ToolCall) ToolResult {
	maxRetries := agentCfg.MaxRetries
	retryDelay := agentCfg.RetryDelay

	var result ToolResult
	for attempt := 0; attempt <= maxRetries; attempt++ {
		result = l.executeTool(ctx, call)

		// If successful or not retryable, return immediately
		if result.Success || !isRetryable(result.Error) {
			return result
		}

		// Log retry attempt
		if l.Logger != nil {
			l.Logger.Info("Retrying operation", map[string]interface{}{
				"tool":    call.Name,
				"attempt": attempt + 1,
				"max":     maxRetries,
				"error":   result.Error,
			})
		}

		// Wait before retry (unless this is the last attempt)
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				result.Error = "Operation cancelled during retry"
				return result
			case <-time.After(retryDelay):
			}
		}
	}

	return result
}
