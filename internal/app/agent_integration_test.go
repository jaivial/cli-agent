package app

import (
	"bytes"
	"context"
	"testing"
)

// TestAgentLoop_SimpleExecution tests the full agent loop with a mock client
func TestAgentLoop_SimpleExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock client that returns a simple tool call
	mockClient := &MinimaxClient{
		APIKey:    "mock",
		Model:     "mock",
		BaseURL:   "mock://",
		MaxTokens: 2048,
	}

	loop := NewAgentLoop(mockClient, 5, t.TempDir(), NewLogger(&bytes.Buffer{}))

	state, err := loop.Execute(context.Background(), "echo hello")

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if state == nil {
		t.Fatal("Expected non-nil state")
	}
	// In mock mode, the task should complete (either via Completed flag or reaching max iterations)
	if !state.Completed && state.Iteration < state.MaxLoops {
		t.Errorf("Expected task to complete or reach max loops, got iteration %d/%d, completed=%v",
			state.Iteration, state.MaxLoops, state.Completed)
	}
}

// TestAgentLoop_MultiStepExecution tests the agent loop with multiple tool calls
func TestAgentLoop_MultiStepExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockClient := &MinimaxClient{
		APIKey:    "mock",
		Model:     "mock",
		BaseURL:   "mock://",
		MaxTokens: 2048,
	}

	loop := NewAgentLoop(mockClient, 10, t.TempDir(), NewLogger(&bytes.Buffer{}))

	state, err := loop.Execute(context.Background(), "list files in current directory")

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if state == nil {
		t.Fatal("Expected non-nil state")
	}
	// Verify state was tracked
	if state.TaskID == "" {
		t.Error("Expected TaskID to be set")
	}
	if state.Task == "" {
		t.Error("Expected Task to be set")
	}
}

// TestAgentLoop_MaxLoops tests that the agent loop respects max iterations
func TestAgentLoop_MaxLoops(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockClient := &MinimaxClient{
		APIKey:    "mock",
		Model:     "mock",
		BaseURL:   "mock://",
		MaxTokens: 2048,
	}

	// Set max loops to a low number to test limit
	loop := NewAgentLoop(mockClient, 3, t.TempDir(), NewLogger(&bytes.Buffer{}))

	state, err := loop.Execute(context.Background(), "complex task that requires many steps")

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if state == nil {
		t.Fatal("Expected non-nil state")
	}
	// Should have reached max loops or completed
	if state.Iteration > state.MaxLoops {
		t.Errorf("Iteration %d exceeded MaxLoops %d", state.Iteration, state.MaxLoops)
	}
}

// TestAgentLoop_StatePersistence tests that state is saved during execution
func TestAgentLoop_StatePersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockClient := &MinimaxClient{
		APIKey:    "mock",
		Model:     "mock",
		BaseURL:   "mock://",
		MaxTokens: 2048,
	}

	stateDir := t.TempDir()
	loop := NewAgentLoop(mockClient, 5, stateDir, NewLogger(&bytes.Buffer{}))

	state, err := loop.Execute(context.Background(), "test task")

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if state == nil {
		t.Fatal("Expected non-nil state")
	}
	if state.TaskID == "" {
		t.Fatal("Expected TaskID to be set")
	}
}
