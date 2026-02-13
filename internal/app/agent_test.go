package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createTestAgentLoop creates a minimal AgentLoop for testing
func createTestAgentLoop() *AgentLoop {
	return &AgentLoop{
		Client:   nil,
		Tools:    DefaultTools(),
		MaxLoops: 10,
		StateDir: "",
		Logger:   NewLogger(&bytes.Buffer{}),
	}
}

// createTestContext creates a background context for testing
func createTestContext() context.Context {
	return context.Background()
}

// mustMarshalJSON marshals a value to JSON for test arguments
func mustMarshalJSON(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// TestExecuteTool_WriteFile tests the write_file tool
func TestExecuteTool_WriteFile(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		path          string
		content       string
		expectSuccess bool
	}{
		{
			name:          "Write simple file",
			path:          filepath.Join(tmpDir, "test.txt"),
			content:       "Hello, World!",
			expectSuccess: true,
		},
		{
			name:          "Write file in nested directory",
			path:          filepath.Join(tmpDir, "nested", "dir", "file.txt"),
			content:       "Nested content",
			expectSuccess: true,
		},
		{
			name:          "Write empty file",
			path:          filepath.Join(tmpDir, "empty.txt"),
			content:       "",
			expectSuccess: true,
		},
		{
			name:          "Write file with special characters",
			path:          filepath.Join(tmpDir, "special.txt"),
			content:       "Line 1\nLine 2\n\tTabbed\n\"Quoted\"",
			expectSuccess: true,
		},
		{
			name:          "Overwrite existing file",
			path:          filepath.Join(tmpDir, "overwrite.txt"),
			content:       "New content",
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create file first for overwrite test
			if tt.name == "Overwrite existing file" {
				os.WriteFile(tt.path, []byte("Old content"), 0644)
			}

			call := ToolCall{
				ID:   "write_1",
				Name: "write_file",
				Arguments: mustMarshalJSON(map[string]interface{}{
					"path":    tt.path,
					"content": tt.content,
				}),
			}

			result := l.executeTool(createTestContext(), call)

			if tt.expectSuccess && !result.Success {
				t.Errorf("Expected success, got error: %s", result.Error)
			}

			if tt.expectSuccess {
				// Verify file was created with correct content
				content, err := os.ReadFile(tt.path)
				if err != nil {
					t.Errorf("Failed to read created file: %v", err)
				}
				if string(content) != tt.content {
					t.Errorf("File content mismatch: expected %q, got %q", tt.content, string(content))
				}
			}
		})
	}
}

// TestExecuteTool_ReadFile tests the read_file tool
func TestExecuteTool_ReadFile(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()

	tests := []struct {
		name                string
		setupContent        string
		setupFile           bool
		expectSuccess       bool
		expectedContent     string
		expectErrorContains string
	}{
		{
			name:            "Read existing file",
			setupFile:       true,
			setupContent:    "Hello, World!",
			expectSuccess:   true,
			expectedContent: "Hello, World!",
		},
		{
			name:            "Read file with multiple lines",
			setupFile:       true,
			setupContent:    "Line 1\nLine 2\nLine 3\n",
			expectSuccess:   true,
			expectedContent: "Line 1\nLine 2\nLine 3\n",
		},
		{
			name:                "Read non-existent file",
			setupFile:           false,
			expectSuccess:       false,
			expectErrorContains: "no such file",
		},
		{
			name:            "Read empty file",
			setupFile:       true,
			setupContent:    "",
			expectSuccess:   true,
			expectedContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "read_test.txt")

			if tt.setupFile {
				err := os.WriteFile(testFile, []byte(tt.setupContent), 0644)
				if err != nil {
					t.Fatalf("Failed to setup test file: %v", err)
				}
			} else {
				testFile = filepath.Join(tmpDir, "nonexistent.txt")
			}

			call := ToolCall{
				ID:   "read_1",
				Name: "read_file",
				Arguments: mustMarshalJSON(map[string]interface{}{
					"path": testFile,
				}),
			}

			result := l.executeTool(createTestContext(), call)

			if tt.expectSuccess {
				if !result.Success {
					t.Errorf("Expected success, got error: %s", result.Error)
				}
				if result.Output != tt.expectedContent {
					t.Errorf("Content mismatch: expected %q, got %q", tt.expectedContent, result.Output)
				}
			} else {
				if result.Success {
					t.Error("Expected failure, got success")
				}
				if tt.expectErrorContains != "" && !strings.Contains(strings.ToLower(result.Error), tt.expectErrorContains) {
					t.Errorf("Expected error containing %q, got %q", tt.expectErrorContains, result.Error)
				}
			}
		})
	}
}

// TestExecuteTool_Exec tests the exec tool
func TestExecuteTool_Exec(t *testing.T) {
	l := createTestAgentLoop()

	tests := []struct {
		name                 string
		command              string
		timeout              int
		expectSuccess        bool
		expectOutputContains string
	}{
		{
			name:                 "Successful echo command",
			command:              "echo hello",
			expectSuccess:        true,
			expectOutputContains: "hello",
		},
		{
			name:                 "Successful multi-word echo",
			command:              "echo hello world test",
			expectSuccess:        true,
			expectOutputContains: "hello world test",
		},
		{
			name:          "Failing command - invalid command",
			command:       "nonexistent_command_xyz",
			expectSuccess: false,
		},
		{
			name:          "Command with exit code 1",
			command:       "false",
			expectSuccess: false,
		},
		{
			name:                 "Command with pipe",
			command:              "echo hello | tr a-z A-Z",
			expectSuccess:        true,
			expectOutputContains: "HELLO",
		},
		{
			name:                 "Command with environment variable",
			command:              "echo $HOME",
			expectSuccess:        true,
			expectOutputContains: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"command": tt.command,
			}
			if tt.timeout > 0 {
				args["timeout"] = tt.timeout
			}

			call := ToolCall{
				ID:        "exec_1",
				Name:      "exec",
				Arguments: mustMarshalJSON(args),
			}

			result := l.executeTool(createTestContext(), call)

			if tt.expectSuccess {
				if !result.Success {
					t.Errorf("Expected success, got error: %s", result.Error)
				}
				if tt.expectOutputContains != "" && !strings.Contains(result.Output, tt.expectOutputContains) {
					t.Errorf("Expected output containing %q, got %q", tt.expectOutputContains, result.Output)
				}
			} else {
				// For failing commands, either Success is false or Error is set
				if result.Success && result.Error == "" {
					t.Error("Expected failure, got success with no error")
				}
			}
		})
	}
}

// TestExecuteTool_ExecTimeout tests command timeout handling
func TestExecuteTool_ExecTimeout(t *testing.T) {
	l := createTestAgentLoop()

	// This test uses a very short timeout to ensure the command times out
	call := ToolCall{
		ID:   "exec_timeout",
		Name: "exec",
		Arguments: mustMarshalJSON(map[string]interface{}{
			"command": "sleep 10",
			"timeout": 1, // 1 second timeout
		}),
	}

	start := time.Now()
	result := l.executeTool(createTestContext(), call)
	duration := time.Since(start)

	// Should complete quickly due to timeout, not after 10 seconds
	if duration > 5*time.Second {
		t.Error("Command did not timeout properly")
	}

	// Should report failure
	if result.Success {
		t.Error("Expected timeout to cause failure")
	}
}

func TestExecuteTool_ExecDetachedProcessReturnsPromptly(t *testing.T) {
	l := createTestAgentLoop()

	call := ToolCall{
		ID:   "exec_detached",
		Name: "exec",
		Arguments: mustMarshalJSON(map[string]interface{}{
			"command": "sleep 3 & echo detached-ready",
			"timeout": 15,
		}),
	}

	start := time.Now()
	result := l.executeTool(createTestContext(), call)
	duration := time.Since(start)

	if !result.Success {
		t.Fatalf("expected detached command to succeed, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "detached-ready") {
		t.Fatalf("expected detached command output, got: %q", result.Output)
	}
	if duration > 2*time.Second {
		t.Fatalf("detached background command returned too slowly: %s", duration)
	}
}

func TestVerifyPythonHTTPBackgroundLaunch_PortInUseDifferentContent(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<html><head><title>Wanted App</title></head><body>ok</body></html>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "<html><head><title>Other App</title></head><body>other</body></html>")
		}),
	}
	defer srv.Shutdown(context.Background())
	go func() { _ = srv.Serve(ln) }()

	port := ln.Addr().(*net.TCPAddr).Port
	command := fmt.Sprintf("cd %s && nohup python3 -m http.server %d > /dev/null 2>&1 & echo $!", tmpDir, port)
	fakeDeadPID := os.Getpid() + 500000
	for isProcessAlive(fakeDeadPID) {
		fakeDeadPID++
	}

	note, verifyErr := verifyPythonHTTPBackgroundLaunch(command, tmpDir, []byte(fmt.Sprintf("%d\n", fakeDeadPID)))
	if verifyErr == nil {
		t.Fatalf("expected verification error, got note=%q", note)
	}
	if !strings.Contains(strings.ToLower(verifyErr.Error()), "different content") {
		t.Fatalf("expected content mismatch error, got: %v", verifyErr)
	}
}

func TestVerifyPythonHTTPBackgroundLaunch_PortInUseMatchingContent(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<html><head><title>Wanted App</title></head><body>ok</body></html>"), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "<html><head><title>Wanted App</title></head><body>served</body></html>")
		}),
	}
	defer srv.Shutdown(context.Background())
	go func() { _ = srv.Serve(ln) }()

	port := ln.Addr().(*net.TCPAddr).Port
	command := fmt.Sprintf("cd %s && nohup python3 -m http.server %d > /dev/null 2>&1 & echo $!", tmpDir, port)
	fakeDeadPID := os.Getpid() + 800000
	for isProcessAlive(fakeDeadPID) {
		fakeDeadPID++
	}

	note, verifyErr := verifyPythonHTTPBackgroundLaunch(command, tmpDir, []byte(fmt.Sprintf("%d\n", fakeDeadPID)))
	if verifyErr != nil {
		t.Fatalf("expected verification success, got error: %v", verifyErr)
	}
	if !strings.Contains(note, "already serving expected project content") {
		t.Fatalf("expected reuse note, got: %q", note)
	}
}

func TestParsePythonHTTPServerPort(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		wantPort   int
		wantParsed bool
	}{
		{
			name:       "Explicit port",
			command:    "nohup python3 -m http.server 3003 > /dev/null 2>&1 &",
			wantPort:   3003,
			wantParsed: true,
		},
		{
			name:       "Default port with redirection",
			command:    "nohup python3 -m http.server >/dev/null 2>&1 &",
			wantPort:   8000,
			wantParsed: true,
		},
		{
			name:       "Non-http-server command",
			command:    "npm run dev",
			wantPort:   0,
			wantParsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPort, gotParsed := parsePythonHTTPServerPort(tt.command)
			if gotParsed != tt.wantParsed {
				t.Fatalf("parsePythonHTTPServerPort(%q) parsed=%v, want %v", tt.command, gotParsed, tt.wantParsed)
			}
			if gotPort != tt.wantPort {
				t.Fatalf("parsePythonHTTPServerPort(%q) port=%d, want %d", tt.command, gotPort, tt.wantPort)
			}
		})
	}
}

func TestShouldAutoDetachServerCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		{
			name:     "npm dev foreground",
			command:  "npm run dev",
			expected: true,
		},
		{
			name:     "python http server foreground",
			command:  "python3 -m http.server 3000",
			expected: true,
		},
		{
			name:     "already backgrounded",
			command:  "npm run dev > /tmp/dev.log 2>&1 &",
			expected: false,
		},
		{
			name:     "non-server command",
			command:  "npm run test",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAutoDetachServerCommand(tt.command)
			if got != tt.expected {
				t.Fatalf("shouldAutoDetachServerCommand(%q)=%v, want %v", tt.command, got, tt.expected)
			}
		})
	}
}

// TestExecuteTool_ListDir tests the list_dir tool
func TestExecuteTool_ListDir(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()

	// Create test files and directories
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("content2"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	tests := []struct {
		name           string
		path           string
		expectSuccess  bool
		expectContains []string
	}{
		{
			name:           "List directory with files",
			path:           tmpDir,
			expectSuccess:  true,
			expectContains: []string{"file1.txt", "file2.go", "subdir"},
		},
		{
			name:           "List empty directory",
			path:           t.TempDir(),
			expectSuccess:  true,
			expectContains: []string{},
		},
		{
			name:           "List non-existent directory",
			path:           filepath.Join(tmpDir, "nonexistent"),
			expectSuccess:  false,
			expectContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call := ToolCall{
				ID:   "list_1",
				Name: "list_dir",
				Arguments: mustMarshalJSON(map[string]interface{}{
					"path": tt.path,
				}),
			}

			result := l.executeTool(createTestContext(), call)

			if tt.expectSuccess && !result.Success {
				t.Errorf("Expected success, got error: %s", result.Error)
			}
			if !tt.expectSuccess && result.Success {
				t.Error("Expected failure, got success")
			}
			for _, expected := range tt.expectContains {
				if !strings.Contains(result.Output, expected) {
					t.Errorf("Expected output to contain %q, got: %s", expected, result.Output)
				}
			}
		})
	}
}

// TestExecuteTool_Grep tests the grep tool
func TestExecuteTool_Grep(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("hello world\ntest content\nhello again\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("package main\nfunc main() {}\n"), 0644)

	tests := []struct {
		name           string
		pattern        string
		path           string
		recursive      bool
		expectSuccess  bool
		expectContains []string
	}{
		{
			name:           "Grep simple pattern",
			pattern:        "hello",
			path:           tmpDir,
			recursive:      true,
			expectSuccess:  true,
			expectContains: []string{"file1.txt", "hello world", "hello again"},
		},
		{
			name:           "Grep pattern not found",
			pattern:        "xyz123notfound",
			path:           tmpDir,
			recursive:      true,
			expectSuccess:  false,
			expectContains: []string{},
		},
		{
			name:           "Grep in subdirectory file",
			pattern:        "package main",
			path:           filepath.Join(tmpDir, "file2.go"),
			recursive:      false,
			expectSuccess:  true,
			expectContains: []string{"package main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"pattern":   tt.pattern,
				"path":      tt.path,
				"recursive": tt.recursive,
			}

			call := ToolCall{
				ID:        "grep_1",
				Name:      "grep",
				Arguments: mustMarshalJSON(args),
			}

			result := l.executeTool(createTestContext(), call)

			if tt.expectSuccess && !result.Success {
				// Grep returns error when pattern not found, which is expected behavior
				if !strings.Contains(result.Error, "exit status 1") {
					t.Errorf("Expected success, got error: %s", result.Error)
				}
			}
			for _, expected := range tt.expectContains {
				if !strings.Contains(result.Output, expected) {
					t.Errorf("Expected output to contain %q, got: %s", expected, result.Output)
				}
			}
		})
	}
}

// TestExecuteTool_SearchFiles tests the search_files tool
func TestExecuteTool_SearchFiles(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("readme"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "nested.go"), []byte("package sub"), 0644)

	tests := []struct {
		name           string
		pattern        string
		path           string
		expectSuccess  bool
		expectContains []string
	}{
		{
			name:           "Search for Go files",
			pattern:        "*.go",
			path:           tmpDir,
			expectSuccess:  true,
			expectContains: []string{"file1.go", "file2.go", "nested.go"},
		},
		{
			name:           "Search for txt files",
			pattern:        "*.txt",
			path:           tmpDir,
			expectSuccess:  true,
			expectContains: []string{"readme.txt"},
		},
		{
			name:          "Search with no matches",
			pattern:       "*.py",
			path:          tmpDir,
			expectSuccess: true, // find returns empty output on success
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"pattern": tt.pattern,
				"path":    tt.path,
			}

			call := ToolCall{
				ID:        "search_1",
				Name:      "search_files",
				Arguments: mustMarshalJSON(args),
			}

			result := l.executeTool(createTestContext(), call)

			if tt.expectSuccess && !result.Success {
				t.Errorf("Expected success, got error: %s", result.Error)
			}
			for _, expected := range tt.expectContains {
				if !strings.Contains(result.Output, expected) {
					t.Errorf("Expected output to contain %q, got: %s", expected, result.Output)
				}
			}
		})
	}
}

// TestExecuteTool_EditFileNonExistent tests edit_file with non-existent file
func TestExecuteTool_EditFileNonExistent(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()

	nonExistentFile := filepath.Join(tmpDir, "does_not_exist.txt")

	call := ToolCall{
		ID:   "edit_1",
		Name: "edit_file",
		Arguments: mustMarshalJSON(map[string]interface{}{
			"path":     nonExistentFile,
			"old_text": "old content",
			"new_text": "new content",
		}),
	}

	result := l.executeTool(createTestContext(), call)

	if result.Success {
		t.Error("Expected failure for non-existent file, got success")
	}
	if !strings.Contains(result.Error, "no such file") && !strings.Contains(result.Error, "cannot find") {
		t.Errorf("Expected error about non-existent file, got: %s", result.Error)
	}
}

// TestExecuteTool_EditFileOldTextNotFound tests edit_file when old_text is not found
func TestExecuteTool_EditFileOldTextNotFound(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("actual content in file\n"), 0644)

	call := ToolCall{
		ID:   "edit_1",
		Name: "edit_file",
		Arguments: mustMarshalJSON(map[string]interface{}{
			"path":     testFile,
			"old_text": "text that does not exist",
			"new_text": "replacement text",
		}),
	}

	result := l.executeTool(createTestContext(), call)

	if result.Success {
		t.Error("Expected failure when old_text not found, got success")
	}
}

// TestIsResponseTruncated tests the truncation detection function
func TestIsResponseTruncated(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected bool
	}{
		{
			name:     "Empty response",
			response: "",
			expected: false,
		},
		{
			name:     "Complete JSON",
			response: `{"tool": "exec", "args": {"command": "ls"}}`,
			expected: false,
		},
		{
			name:     "Unclosed brace",
			response: `{"tool": "exec", "args": {"command": "ls"`,
			expected: true,
		},
		{
			name:     "Unclosed bracket",
			response: `["item1", "item2"`,
			expected: true,
		},
		{
			name:     "Ends with backslash",
			response: `{"tool": "exec"\`,
			expected: true,
		},
		{
			name:     "Ends with colon",
			response: `{"tool":`,
			expected: true,
		},
		{
			name:     "Ends with comma quote",
			response: `{"tool": "exec",`,
			expected: true,
		},
		{
			name:     "Complete text response",
			response: "Task completed successfully.",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isResponseTruncated(tt.response)
			if result != tt.expected {
				t.Errorf("isResponseTruncated(%q) = %v, want %v", tt.response, result, tt.expected)
			}
		})
	}
}

// TestIsRetryable tests the retryable error detection function
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "Connection refused",
			errMsg:   "dial tcp: connection refused",
			expected: true,
		},
		{
			name:     "Connection reset",
			errMsg:   "read: connection reset by peer",
			expected: true,
		},
		{
			name:     "Resource temporarily unavailable",
			errMsg:   "resource temporarily unavailable",
			expected: true,
		},
		{
			name:     "Text file busy",
			errMsg:   "text file busy",
			expected: true,
		},
		{
			name:     "File not found",
			errMsg:   "no such file or directory",
			expected: false,
		},
		{
			name:     "Permission denied",
			errMsg:   "permission denied",
			expected: false,
		},
		{
			name:     "Empty error",
			errMsg:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryable(tt.errMsg)
			if result != tt.expected {
				t.Errorf("isRetryable(%q) = %v, want %v", tt.errMsg, result, tt.expected)
			}
		})
	}
}

// TestValidatePath tests the path validation function
func TestValidatePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{
			name:      "Normal path",
			path:      "/home/user/file.txt",
			expectErr: false,
		},
		{
			name:      "Relative path",
			path:      "src/main.go",
			expectErr: false,
		},
		{
			name:      "Path traversal",
			path:      "../../../etc/passwd",
			expectErr: true,
		},
		{
			name:      "Hidden path traversal",
			path:      "/home/user/../other/file.txt",
			expectErr: true,
		},
		{
			name:      "Current directory",
			path:      ".",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if tt.expectErr && err == nil {
				t.Errorf("validatePath(%q) expected error, got nil", tt.path)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("validatePath(%q) unexpected error: %v", tt.path, err)
			}
		})
	}
}

// TestExecuteToolContextCancellation tests that tools respect context cancellation
func TestExecuteToolContextCancellation(t *testing.T) {
	l := createTestAgentLoop()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Test that grep respects cancellation
	call := ToolCall{
		ID:   "grep_cancel",
		Name: "grep",
		Arguments: mustMarshalJSON(map[string]interface{}{
			"pattern": "test",
			"path":    ".",
		}),
	}

	result := l.executeTool(ctx, call)

	if result.Success {
		t.Error("Expected failure due to cancellation, got success")
	}
	if !strings.Contains(result.Error, "cancelled") {
		t.Errorf("Expected cancellation error, got: %s", result.Error)
	}
}

func TestParseToolCalls_ToolCallsEnvelope(t *testing.T) {
	l := createTestAgentLoop()

	resp := `I'll run a command.

{
  "tool_calls": [
    {
      "id": "exec_1",
      "name": "exec",
      "arguments": {"command": "echo hello"}
    }
  ]
}`

	calls := l.parseToolCalls(resp)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "exec" {
		t.Fatalf("expected tool name exec, got %q", calls[0].Name)
	}
	if !strings.Contains(string(calls[0].Arguments), "echo hello") {
		t.Fatalf("expected arguments to include command, got %s", string(calls[0].Arguments))
	}
}

func TestExecuteTool_PatchFile(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatalf("failed to setup file: %v", err)
	}

	patch := "@@ -1,2 +1,2 @@\n-line1\n+LINE1\n line2\n"
	call := ToolCall{
		ID:   "patch_1",
		Name: "patch_file",
		Arguments: mustMarshalJSON(map[string]interface{}{
			"path":  path,
			"patch": patch,
		}),
	}

	result := l.executeTool(createTestContext(), call)
	if !result.Success {
		t.Fatalf("expected patch_file success, got error: %s", result.Error)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read patched file: %v", err)
	}
	if string(data) != "LINE1\nline2\n" {
		t.Fatalf("unexpected patched content: %q", string(data))
	}
}

func TestExecuteTool_PatchFile_PreservesFileMode(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "script.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("failed to setup file: %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("failed to chmod test file: %v", err)
	}

	patch := "@@ -1,2 +1,2 @@\n #!/bin/sh\n-echo hi\n+echo hello\n"
	call := ToolCall{
		ID:   "patch_mode_1",
		Name: "patch_file",
		Arguments: mustMarshalJSON(map[string]interface{}{
			"path":  path,
			"patch": patch,
		}),
	}

	result := l.executeTool(createTestContext(), call)
	if !result.Success {
		t.Fatalf("expected patch_file success, got error: %s", result.Error)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat patched file: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected mode 0755 after patch, got %o", info.Mode().Perm())
	}
}

func TestExecuteTool_AppendFile(t *testing.T) {
	l := createTestAgentLoop()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "append.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0644); err != nil {
		t.Fatalf("failed to setup file: %v", err)
	}

	call := ToolCall{
		ID:   "append_1",
		Name: "append_file",
		Arguments: mustMarshalJSON(map[string]interface{}{
			"path":    path,
			"content": "b\n",
		}),
	}

	result := l.executeTool(createTestContext(), call)
	if !result.Success {
		t.Fatalf("expected append_file success, got error: %s", result.Error)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read appended file: %v", err)
	}
	if string(data) != "a\nb\n" {
		t.Fatalf("unexpected appended content: %q", string(data))
	}
}
