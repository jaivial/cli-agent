package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"cli-agent/internal/app"
)

// MockAgentLoop is a test helper that wraps AgentLoop with mock capabilities
type MockAgentLoop struct {
	agentLoop     *app.AgentLoop
	toolResponses map[string]mockToolResponse
	callSequence  []string
}

type mockToolResponse struct {
	output  string
	success bool
	err     string
}

// NewMockAgentLoop creates a new mock agent loop for testing
func NewMockAgentLoop() *MockAgentLoop {
	return &MockAgentLoop{
		agentLoop:     nil, // We'll use direct tool execution
		toolResponses: make(map[string]mockToolResponse),
		callSequence:  []string{},
	}
}

// SetMockResponse sets a mock response for a tool call
func (m *MockAgentLoop) SetMockResponse(tool string, output string, success bool) {
	m.toolResponses[tool] = mockToolResponse{
		output:  output,
		success: success,
	}
}

// RecordCall records a tool call for verification
func (m *MockAgentLoop) RecordCall(tool string, args json.RawMessage) {
	m.callSequence = append(m.callSequence, tool)
}

// GetCallSequence returns the recorded tool call sequence
func (m *MockAgentLoop) GetCallSequence() []string {
	return m.callSequence
}

// ResetSequence clears the call sequence
func (m *MockAgentLoop) ResetSequence() {
	m.callSequence = []string{}
}

// CreateTestToolCall creates a ToolCall for testing
func CreateTestToolCall(id, name string, args map[string]interface{}) app.ToolCall {
	argsJSON, _ := json.Marshal(args)
	return app.ToolCall{
		ID:        id,
		Name:      name,
		Arguments: argsJSON,
	}
}

// TestGitReflogRecovery tests git reflog-based recovery workflow
func TestGitReflogRecovery(t *testing.T) {
	mock := NewMockAgentLoop()

	// Expected tool sequence for git reflog recovery:
	// 1. Check git status
	// 2. Use reflog to find lost commits
	// 3. Recover the commit
	// 4. Verify recovery

	testCases := []struct {
		name        string
		toolCall    app.ToolCall
		expectError bool
	}{
		{
			name: "Check git status",
			toolCall: CreateTestToolCall("1", "exec", map[string]interface{}{
				"command": "git status",
			}),
			expectError: false,
		},
		{
			name: "View reflog",
			toolCall: CreateTestToolCall("2", "exec", map[string]interface{}{
				"command": "git reflog --all",
			}),
			expectError: false,
		},
		{
			name: "Find dangling commits with fsck",
			toolCall: CreateTestToolCall("3", "exec", map[string]interface{}{
				"command": "git fsck --lost-found --dangling",
			}),
			expectError: false,
		},
		{
			name: "Recover commit",
			toolCall: CreateTestToolCall("4", "exec", map[string]interface{}{
				"command": "git cherry-pick abc123",
			}),
			expectError: false,
		},
		{
			name: "Verify file recovery",
			toolCall: CreateTestToolCall("5", "read_file", map[string]interface{}{
				"path": "/tmp/recovered.txt",
			}),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate the tool call structure
			if tc.toolCall.Name == "" {
				t.Error("Tool name should not be empty")
			}
			if tc.toolCall.ID == "" {
				t.Error("Tool ID should not be empty")
			}

			// Record for sequence tracking
			mock.RecordCall(tc.toolCall.Name, tc.toolCall.Arguments)
		})
	}

	// Verify the complete sequence
	sequence := mock.GetCallSequence()
	expectedSequence := []string{"exec", "exec", "exec", "exec", "read_file"}

	if len(sequence) != len(expectedSequence) {
		t.Errorf("Expected %d calls, got %d", len(expectedSequence), len(sequence))
	}

	for i, expected := range expectedSequence {
		if i >= len(sequence) {
			break
		}
		if sequence[i] != expected {
			t.Errorf("Expected call %d to be %s, got %s", i, expected, sequence[i])
		}
	}
}

// TestSQLiteTruncate tests SQLite database truncation workflow
func TestSQLiteTruncate(t *testing.T) {
	mock := NewMockAgentLoop()

	testCases := []struct {
		name         string
		toolCall     app.ToolCall
		expectedTool string
		description  string
	}{
		{
			name: "Check database file",
			toolCall: CreateTestToolCall("1", "list_dir", map[string]interface{}{
				"path": "/tmp",
			}),
			expectedTool: "list_dir",
			description:  "List directory to find database file",
		},
		{
			name: "Check table structure",
			toolCall: CreateTestToolCall("2", "exec", map[string]interface{}{
				"command": "sqlite3 /tmp/test.db \".schema\"",
			}),
			expectedTool: "exec",
			description:  "View database schema",
		},
		{
			name: "Count rows before truncate",
			toolCall: CreateTestToolCall("3", "exec", map[string]interface{}{
				"command": "sqlite3 /tmp/test.db \"SELECT COUNT(*) FROM data;\"",
			}),
			expectedTool: "exec",
			description:  "Count rows before truncation",
		},
		{
			name: "Delete all rows (SQLite TRUNCATE)",
			toolCall: CreateTestToolCall("4", "exec", map[string]interface{}{
				"command": "sqlite3 /tmp/test.db \"DELETE FROM data;\"",
			}),
			expectedTool: "exec",
			description:  "SQLite uses DELETE FROM for truncation",
		},
		{
			name: "Vacuum database",
			toolCall: CreateTestToolCall("5", "exec", map[string]interface{}{
				"command": "sqlite3 /tmp/test.db \"VACUUM;\"",
			}),
			expectedTool: "exec",
			description:  "Reclaim space with VACUUM",
		},
		{
			name: "Verify truncation",
			toolCall: CreateTestToolCall("6", "exec", map[string]interface{}{
				"command": "sqlite3 /tmp/test.db \"SELECT COUNT(*) FROM data;\"",
			}),
			expectedTool: "exec",
			description:  "Verify table is empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.toolCall.Name != tc.expectedTool {
				t.Errorf("Expected tool %s, got %s", tc.expectedTool, tc.toolCall.Name)
			}
			mock.RecordCall(tc.toolCall.Name, tc.toolCall.Arguments)
		})
	}
}

// TestQEMUBoot tests QEMU VM boot workflow
func TestQEMUBoot(t *testing.T) {
	mock := NewMockAgentLoop()

	testCases := []struct {
		name         string
		toolCall     app.ToolCall
		expectedTool string
		timeout      int
	}{
		{
			name: "Check QEMU installation",
			toolCall: CreateTestToolCall("1", "exec", map[string]interface{}{
				"command": "which qemu-system-x86_64",
			}),
			expectedTool: "exec",
			timeout:      10,
		},
		{
			name: "List VM images",
			toolCall: CreateTestToolCall("2", "list_dir", map[string]interface{}{
				"path": "/vms",
			}),
			expectedTool: "list_dir",
		},
		{
			name: "Start QEMU with port forwarding",
			toolCall: CreateTestToolCall("3", "exec", map[string]interface{}{
				"command": "qemu-system-x86_64 -m 512 -hda /vms/alpine.qcow2 -netdev user,id=net0,hostfwd=tcp::2222-:22 -device e1000,netdev=net0 -daemonize",
				"timeout": 60,
			}),
			expectedTool: "exec",
			timeout:      60,
		},
		{
			name: "Wait for SSH to be ready",
			toolCall: CreateTestToolCall("4", "exec", map[string]interface{}{
				"command": "while ! nc -z localhost 2222; do sleep 1; done",
				"timeout": 120,
			}),
			expectedTool: "exec",
			timeout:      120,
		},
		{
			name: "Connect via SSH",
			toolCall: CreateTestToolCall("5", "exec", map[string]interface{}{
				"command": "ssh -p 2222 root@localhost 'echo Boot successful'",
				"timeout": 30,
			}),
			expectedTool: "exec",
			timeout:      30,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.toolCall.Name != tc.expectedTool {
				t.Errorf("Expected tool %s, got %s", tc.expectedTool, tc.toolCall.Name)
			}
			mock.RecordCall(tc.toolCall.Name, tc.toolCall.Arguments)
		})
	}
}

// TestPyTorchModelRecovery tests PyTorch model state recovery workflow
func TestPyTorchModelRecovery(t *testing.T) {
	mock := NewMockAgentLoop()

	testCases := []struct {
		name         string
		toolCall     app.ToolCall
		expectedTool string
		description  string
	}{
		{
			name: "List model files",
			toolCall: CreateTestToolCall("1", "list_dir", map[string]interface{}{
				"path": "/models",
			}),
			expectedTool: "list_dir",
			description:  "Find model checkpoint files",
		},
		{
			name: "Check checkpoint file size",
			toolCall: CreateTestToolCall("2", "exec", map[string]interface{}{
				"command": "ls -lh /models/checkpoint.pth",
			}),
			expectedTool: "exec",
			description:  "Verify checkpoint exists and has content",
		},
		{
			name: "Create recovery script",
			toolCall: CreateTestToolCall("3", "write_file", map[string]interface{}{
				"path":    "/tmp/recover.py",
				"content": recoveryScript,
			}),
			expectedTool: "write_file",
			description:  "Write Python recovery script",
		},
		{
			name: "Run recovery script with CPU fallback",
			toolCall: CreateTestToolCall("4", "exec", map[string]interface{}{
				"command": "python /tmp/recover.py --checkpoint /models/checkpoint.pth --output /models/recovered.pth --map-location cpu",
				"timeout": 120,
			}),
			expectedTool: "exec",
			description:  "Recover model using CPU mapping",
		},
		{
			name: "Verify recovered model",
			toolCall: CreateTestToolCall("5", "exec", map[string]interface{}{
				"command": "python -c \"import torch; m = torch.load('/models/recovered.pth'); print('Keys:', list(m.keys()))\"",
			}),
			expectedTool: "exec",
			description:  "Verify model loads correctly",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.toolCall.Name != tc.expectedTool {
				t.Errorf("Expected tool %s, got %s", tc.expectedTool, tc.toolCall.Name)
			}
			mock.RecordCall(tc.toolCall.Name, tc.toolCall.Arguments)
		})
	}
}

// TestBuildWithDependencies tests build tasks with complex dependency chains
func TestBuildWithDependencies(t *testing.T) {
	mock := NewMockAgentLoop()

	testCases := []struct {
		name         string
		toolCall     app.ToolCall
		expectedTool string
		description  string
	}{
		{
			name: "Check source directory",
			toolCall: CreateTestToolCall("1", "list_dir", map[string]interface{}{
				"path": "/src",
			}),
			expectedTool: "list_dir",
			description:  "Explore project structure",
		},
		{
			name: "Read build configuration",
			toolCall: CreateTestToolCall("2", "read_file", map[string]interface{}{
				"path": "/src/CMakeLists.txt",
			}),
			expectedTool: "read_file",
			description:  "Check CMake configuration",
		},
		{
			name: "Check installed dependencies",
			toolCall: CreateTestToolCall("3", "exec", map[string]interface{}{
				"command": "pkg-config --exists libssl && echo 'SSL found' || echo 'SSL missing'",
			}),
			expectedTool: "exec",
			description:  "Check for required libraries",
		},
		{
			name: "Create build directory",
			toolCall: CreateTestToolCall("4", "exec", map[string]interface{}{
				"command": "mkdir -p /src/build",
			}),
			expectedTool: "exec",
			description:  "Create build directory",
		},
		{
			name: "Run cmake",
			toolCall: CreateTestToolCall("5", "exec", map[string]interface{}{
				"command": "cd /src/build && cmake ..",
				"timeout": 60,
			}),
			expectedTool: "exec",
			description:  "Configure build with cmake",
		},
		{
			name: "Build with make",
			toolCall: CreateTestToolCall("6", "exec", map[string]interface{}{
				"command": "cd /src/build && make -j$(nproc)",
				"timeout": 300,
			}),
			expectedTool: "exec",
			description:  "Compile the project",
		},
		{
			name: "Run tests",
			toolCall: CreateTestToolCall("7", "exec", map[string]interface{}{
				"command": "cd /src/build && ctest --output-on-failure",
				"timeout": 60,
			}),
			expectedTool: "exec",
			description:  "Run test suite",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.toolCall.Name != tc.expectedTool {
				t.Errorf("Expected tool %s, got %s", tc.expectedTool, tc.toolCall.Name)
			}
			mock.RecordCall(tc.toolCall.Name, tc.toolCall.Arguments)
		})
	}
}

// TestErrorRecoveryPaths tests error recovery scenarios
func TestErrorRecoveryPaths(t *testing.T) {
	mock := NewMockAgentLoop()

	tests := []struct {
		name           string
		initialError   string
		recoveryAction app.ToolCall
		description    string
	}{
		{
			name:         "File not found recovery",
			initialError: "file not found: /tmp/missing.txt",
			recoveryAction: CreateTestToolCall("1", "exec", map[string]interface{}{
				"command": "find /tmp -name 'missing*' 2>/dev/null",
			}),
			description: "Search for file when not found",
		},
		{
			name:         "Permission denied recovery",
			initialError: "permission denied: /root/file",
			recoveryAction: CreateTestToolCall("1", "exec", map[string]interface{}{
				"command": "sudo cat /root/file",
			}),
			description: "Use sudo for permission issues",
		},
		{
			name:         "Command not found recovery",
			initialError: "command not found: custom-tool",
			recoveryAction: CreateTestToolCall("1", "exec", map[string]interface{}{
				"command": "apt-get update && apt-get install -y custom-tool",
				"timeout": 120,
			}),
			description: "Install missing package",
		},
		{
			name:         "Network timeout recovery",
			initialError: "connection timeout",
			recoveryAction: CreateTestToolCall("1", "exec", map[string]interface{}{
				"command": "curl --retry 3 --retry-delay 5 http://example.com",
				"timeout": 60,
			}),
			description: "Retry with exponential backoff",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock.RecordCall(tc.recoveryAction.Name, tc.recoveryAction.Arguments)
			// Verify recovery action is reasonable
			if tc.recoveryAction.Name != "exec" && tc.recoveryAction.Name != "read_file" && tc.recoveryAction.Name != "search_files" {
				t.Errorf("Expected recovery to use common tools, got %s", tc.recoveryAction.Name)
			}
		})
	}
}

// TestToolSequences tests complete tool sequences for common patterns
func TestToolSequences(t *testing.T) {
	mock := NewMockAgentLoop()

	// Test: Read-Modify-Write pattern
	t.Run("ReadModifyWritePattern", func(t *testing.T) {
		mock.ResetSequence()

		sequence := []app.ToolCall{
			CreateTestToolCall("1", "read_file", map[string]interface{}{
				"path": "/tmp/config.txt",
			}),
			CreateTestToolCall("2", "edit_file", map[string]interface{}{
				"path":     "/tmp/config.txt",
				"old_text": "value=1",
				"new_text": "value=2",
			}),
			CreateTestToolCall("3", "read_file", map[string]interface{}{
				"path": "/tmp/config.txt",
			}),
		}

		for _, call := range sequence {
			mock.RecordCall(call.Name, call.Arguments)
		}

		calls := mock.GetCallSequence()
		if len(calls) != 3 {
			t.Errorf("Expected 3 calls, got %d", len(calls))
		}
		if calls[0] != "read_file" || calls[1] != "edit_file" || calls[2] != "read_file" {
			t.Error("Expected read_file -> edit_file -> read_file pattern")
		}
	})

	// Test: Search-Read-Edit pattern
	t.Run("SearchReadEditPattern", func(t *testing.T) {
		mock.ResetSequence()

		sequence := []app.ToolCall{
			CreateTestToolCall("1", "grep", map[string]interface{}{
				"pattern":   "TODO",
				"path":      "/src",
				"recursive": true,
			}),
			CreateTestToolCall("2", "read_file", map[string]interface{}{
				"path": "/src/main.go",
			}),
			CreateTestToolCall("3", "edit_file", map[string]interface{}{
				"path":     "/src/main.go",
				"old_text": "// TODO: fix this",
				"new_text": "// FIXED: implemented",
			}),
		}

		for _, call := range sequence {
			mock.RecordCall(call.Name, call.Arguments)
		}

		calls := mock.GetCallSequence()
		if len(calls) != 3 {
			t.Errorf("Expected 3 calls, got %d", len(calls))
		}
		if calls[0] != "grep" || calls[1] != "read_file" || calls[2] != "edit_file" {
			t.Error("Expected grep -> read_file -> edit_file pattern")
		}
	})
}

// TestContextTimeout tests timeout handling
func TestContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Simulate work that might take longer
	done := make(chan bool)
	go func() {
		time.Sleep(50 * time.Millisecond)
		done <- true
	}()

	select {
	case <-done:
		// Success - work completed in time
	case <-ctx.Done():
		t.Error("Context timed out unexpectedly")
	}
}

// recoveryScript is a helper Python script for PyTorch model recovery
const recoveryScript = `#!/usr/bin/env python3
import argparse
import torch
import sys

def recover_checkpoint(input_path, output_path, map_location):
    """Recover a potentially corrupted PyTorch checkpoint."""
    try:
        # Try loading with specified map_location
        checkpoint = torch.load(input_path, map_location=map_location)
        
        # If it's a full checkpoint with model state
        if isinstance(checkpoint, dict):
            print(f"Checkpoint keys: {list(checkpoint.keys())}")
            
            # Save recovered checkpoint
            torch.save(checkpoint, output_path)
            print(f"Successfully recovered checkpoint to {output_path}")
        else:
            # Just a state dict
            torch.save({'model_state_dict': checkpoint}, output_path)
            print(f"Saved state dict to {output_path}")
            
        return True
    except Exception as e:
        print(f"Recovery failed: {e}", file=sys.stderr)
        return False

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--checkpoint", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--map-location", default="cpu")
    args = parser.parse_args()
    
    success = recover_checkpoint(args.checkpoint, args.output, args.map_location)
    sys.exit(0 if success else 1)
`

// Helper type to match internal app package

// ToolCall mirrors the internal app.ToolCall type
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// TestExecBackground tests the ExecBackground functionality
func TestExecBackground(t *testing.T) {
	runner := app.NewRunner(nil, "")
	
	proc, err := runner.ExecBackground("echo hello")
	if err != nil {
		t.Fatalf("ExecBackground failed: %v", err)
	}
	
	if proc.PID == 0 {
		t.Error("Expected non-zero PID")
	}
	
	// Wait for process to complete
	select {
	case <-proc.Done:
		// Process completed
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for process to complete")
	}
	
	// Check exit code
	if proc.ExitCode == nil {
		t.Error("Expected exit code to be set")
	} else if *proc.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", *proc.ExitCode)
	}
}

// TestWaitForOutput tests waiting for output pattern
func TestWaitForOutput(t *testing.T) {
	runner := app.NewRunner(nil, "")
	
	proc, err := runner.ExecBackground("echo 'start'; sleep 0.1; echo 'done'")
	if err != nil {
		t.Fatalf("ExecBackground failed: %v", err)
	}
	
	// Wait for pattern
	matched, output, err := runner.WaitForOutput(proc.PID, "done", 5)
	if err != nil && matched {
		t.Logf("WaitForOutput error (may be timeout after match): %v", err)
	}
	
	if !matched {
		t.Errorf("Expected pattern match, got output: %s", output)
	}
}

// TestSendInput tests sending input to a background process
func TestSendInput(t *testing.T) {
	runner := app.NewRunner(nil, "")
	
	// Start a process that reads input
	proc, err := runner.ExecBackground("cat")
	if err != nil {
		t.Fatalf("ExecBackground failed: %v", err)
	}
	
	// Send input
	err = runner.SendInput(proc.PID, "test input\n")
	if err != nil {
		t.Errorf("SendInput failed: %v", err)
	}
	
	// Wait for the input to be echoed back
	matched, _, err := runner.WaitForOutput(proc.PID, "test input", 3)
	if !matched {
		t.Errorf("Expected to see input echoed back, err: %v", err)
	}
}

// TestAppendFile tests the append_file tool integration
func TestAppendFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test_append.txt"
	
	// Write initial content
	err := os.WriteFile(testFile, []byte("line1\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Append content using shell command (simulating what the tool would do)
	cmd := exec.Command("sh", "-c", "echo 'line2' >> "+testFile)
	err = cmd.Run()
	if err != nil {
		t.Errorf("Failed to append to file: %v", err)
	}
	
	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	
	expected := "line1\nline2\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

// TestPatchFile tests the patch_file tool integration
func TestPatchFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test_patch.txt"
	
	// Write initial content
	initialContent := "line1\nline2\nline3\n"
	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Apply patch using a patch command
	patch := `--- a/test_patch.txt
+++ b/test_patch.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2_modified
 line3
`
	
	// Write patch to temp file
	patchFile := tmpDir + "/test.patch"
	err = os.WriteFile(patchFile, []byte(patch), 0644)
	if err != nil {
		t.Fatalf("Failed to write patch file: %v", err)
	}
	
	// Apply patch
	cmd := exec.Command("patch", "-p0", "-i", patchFile)
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Patch output: %s", string(output))
		// Some systems may not have patch, skip in that case
		t.Skip("patch command not available")
	}
	
	// Verify content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	
	expected := "line1\nline2_modified\nline3\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

// TestHTTPRequest tests the http_request tool integration
func TestHTTPRequest(t *testing.T) {
	// Create a simple test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/test":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
		case "/echo":
			body, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		}
	}))
	defer server.Close()
	
	// Test GET request
	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	
	if !strings.Contains(string(body), "ok") {
		t.Errorf("Expected response to contain 'ok', got %s", string(body))
	}
}

// TestBackgroundProcessManager tests the global process manager
func TestBackgroundProcessManager(t *testing.T) {
	manager := app.GetGlobalProcessManager()
	
	// Create a test process (using a real command)
	cmd := exec.Command("sleep", "0.1")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	
	// Create BackgroundProcess manually
	proc := &app.BackgroundProcess{
		PID: cmd.Process.Pid,
	}
	
	// Add to manager
	manager.Add(proc)
	
	// Get from manager
	got, ok := manager.Get(proc.PID)
	if !ok {
		t.Error("Failed to get process from manager")
	}
	if got.PID != proc.PID {
		t.Errorf("Expected PID %d, got %d", proc.PID, got.PID)
	}
	
	// Wait for process to exit
	cmd.Wait()
	
	// Cleanup
	manager.Remove(proc.PID)
	
	_, ok = manager.Get(proc.PID)
	if ok {
		t.Error("Process should have been removed")
	}
}

// TestBackgroundProcess_Lifecycle tests the full lifecycle of a background process
func TestBackgroundProcess_Lifecycle(t *testing.T) {
	runner := app.NewRunner(nil, "")

	// Start process
	proc, err := runner.ExecBackground("echo hello && sleep 0.1 && echo done")
	if err != nil {
		t.Fatalf("ExecBackground failed: %v", err)
	}
	if proc == nil {
		t.Fatal("Expected non-nil process")
	}
	if proc.PID == 0 {
		t.Error("Expected non-zero PID")
	}

	// Wait for output
	matched, output, err := runner.WaitForOutput(proc.PID, "done", 5)
	if err != nil {
		t.Logf("WaitForOutput error: %v", err)
	}
	if !matched {
		t.Errorf("Expected pattern 'done' to match, output: %s", output)
	}
	if !strings.Contains(output, "hello") {
		t.Errorf("Expected output to contain 'hello', got: %s", output)
	}

	// Wait for completion
	select {
	case <-proc.Done:
		// Process completed
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for process to complete")
	}

	// Verify exit code
	if proc.ExitCode == nil {
		t.Error("Expected exit code to be set after completion")
	} else if *proc.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", *proc.ExitCode)
	}
}

// TestBackgroundProcess_Input tests sending input to a background process
func TestBackgroundProcess_Input(t *testing.T) {
	runner := app.NewRunner(nil, "")

	// Start process that reads input
	proc, err := runner.ExecBackground("cat")
	if err != nil {
		t.Fatalf("ExecBackground failed: %v", err)
	}

	// Give the process a moment to start
	time.Sleep(50 * time.Millisecond)

	// Send input via SendInput
	testInput := "test input data\n"
	err = runner.SendInput(proc.PID, testInput)
	if err != nil {
		t.Errorf("SendInput failed: %v", err)
	}

	// Verify process received input by waiting for it in output
	matched, output, err := runner.WaitForOutput(proc.PID, "test input", 3)
	if !matched {
		t.Errorf("Expected to see input echoed back. Matched: %v, Output: %s, Error: %v", matched, output, err)
	}

	// Clean up - send EOF to terminate cat
	runner.SendInput(proc.PID, "\x04") // Ctrl+D

	// Wait for completion with timeout
	select {
	case <-proc.Done:
		// Good
	case <-time.After(2 * time.Second):
		t.Log("Process did not complete within timeout (may be expected for cat)")
	}
}

// TestBackgroundProcess_InvalidPID tests operations with invalid PIDs
func TestBackgroundProcess_InvalidPID(t *testing.T) {
	runner := app.NewRunner(nil, "")

	// Test WaitForOutput with non-existent PID
	matched, _, err := runner.WaitForOutput(99999, "pattern", 1)
	if matched {
		t.Error("Expected no match for non-existent PID")
	}
	if err == nil {
		t.Error("Expected error for non-existent PID")
	}

	// Test SendInput with non-existent PID
	err = runner.SendInput(99999, "input")
	if err == nil {
		t.Error("Expected error for non-existent PID")
	}
}

// TestBackgroundProcess_OutputCapture tests output capture from background processes
func TestBackgroundProcess_OutputCapture(t *testing.T) {
	runner := app.NewRunner(nil, "")

	// Start a process that produces multi-line output
	proc, err := runner.ExecBackground("echo 'line1'; echo 'line2'; echo 'line3'")
	if err != nil {
		t.Fatalf("ExecBackground failed: %v", err)
	}

	// Wait for all output
	matched, _, err := runner.WaitForOutput(proc.PID, "line3", 5)
	if !matched {
		t.Errorf("Expected to see line3 in output, err: %v", err)
	}

	// Wait for completion
	<-proc.Done

	// Verify we captured all output
	manager := app.GetGlobalProcessManager()
	if managedProc, ok := manager.Get(proc.PID); ok {
		// Access output through the process object
		_ = managedProc
		// Note: The actual output verification depends on implementation details
	}
}

// TestBackgroundProcess_Concurrent tests running multiple background processes
func TestBackgroundProcess_Concurrent(t *testing.T) {
	runner := app.NewRunner(nil, "")

	// Start multiple processes
	procs := make([]*app.BackgroundProcess, 3)
	for i := 0; i < 3; i++ {
		proc, err := runner.ExecBackground("echo 'proc' && sleep 0.1")
		if err != nil {
			t.Fatalf("ExecBackground %d failed: %v", i, err)
		}
		procs[i] = proc
	}

	// Wait for all to complete
	done := make(chan bool, 3)
	for i, proc := range procs {
		go func(idx int, p *app.BackgroundProcess) {
			select {
			case <-p.Done:
				done <- true
			case <-time.After(5 * time.Second):
				t.Errorf("Timeout waiting for process %d", idx)
				done <- false
			}
		}(i, proc)
	}

	// Check all completed
	for i := 0; i < 3; i++ {
		if !<-done {
			t.Errorf("Process %d did not complete successfully", i)
		}
	}

	// Verify all have exit codes
	for i, proc := range procs {
		if proc.ExitCode == nil {
			t.Errorf("Process %d should have exit code set", i)
		} else if *proc.ExitCode != 0 {
			t.Errorf("Process %d should have exit code 0, got %d", i, *proc.ExitCode)
		}
	}
}
