package app

import (
	"testing"
	"time"
)

// TestBackgroundProcessManager tests the BackgroundProcessManager Add/Get/Remove/List operations
func TestBackgroundProcessManager(t *testing.T) {
	manager := NewBackgroundProcessManager()

	// Test Add/Get/Remove cycle
	proc := &BackgroundProcess{
		PID:       12345,
		Command:   "test command",
		StartedAt: time.Now(),
		Done:      make(chan struct{}),
	}
	manager.Add(proc)

	got, ok := manager.Get(12345)
	if !ok {
		t.Error("Get failed - process not found")
	}
	if got.PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", got.PID)
	}
	if got.Command != "test command" {
		t.Errorf("Expected command 'test command', got %s", got.Command)
	}

	// Test List
	pids := manager.List()
	if len(pids) != 1 || pids[0] != 12345 {
		t.Errorf("Expected [12345], got %v", pids)
	}

	manager.Remove(12345)
	_, ok = manager.Get(12345)
	if ok {
		t.Error("Remove failed - process still exists")
	}

	// List should be empty after removal
	pids = manager.List()
	if len(pids) != 0 {
		t.Errorf("Expected empty list, got %v", pids)
	}
}

// TestBackgroundProcessManagerMultipleProcesses tests managing multiple processes
func TestBackgroundProcessManagerMultipleProcesses(t *testing.T) {
	manager := NewBackgroundProcessManager()

	// Add multiple processes
	procs := []*BackgroundProcess{
		{PID: 1001, Command: "proc1", StartedAt: time.Now(), Done: make(chan struct{})},
		{PID: 1002, Command: "proc2", StartedAt: time.Now(), Done: make(chan struct{})},
		{PID: 1003, Command: "proc3", StartedAt: time.Now(), Done: make(chan struct{})},
	}

	for _, proc := range procs {
		manager.Add(proc)
	}

	// Verify all are retrievable
	for _, proc := range procs {
		got, ok := manager.Get(proc.PID)
		if !ok {
			t.Errorf("Process %d not found", proc.PID)
		}
		if got.Command != proc.Command {
			t.Errorf("Expected command %s, got %s", proc.Command, got.Command)
		}
	}

	// List should return all PIDs
	pids := manager.List()
	if len(pids) != 3 {
		t.Errorf("Expected 3 PIDs, got %d", len(pids))
	}

	// Remove one and verify
	manager.Remove(1002)
	_, ok := manager.Get(1002)
	if ok {
		t.Error("Process 1002 should have been removed")
	}

	// Others should still exist
	_, ok = manager.Get(1001)
	if !ok {
		t.Error("Process 1001 should still exist")
	}
	_, ok = manager.Get(1003)
	if !ok {
		t.Error("Process 1003 should still exist")
	}
}

// TestBackgroundProcessManagerConcurrentAccess tests concurrent access to the manager
func TestBackgroundProcessManagerConcurrentAccess(t *testing.T) {
	manager := NewBackgroundProcessManager()
	done := make(chan bool)

	// Concurrent adds
	go func() {
		for i := 0; i < 10; i++ {
			manager.Add(&BackgroundProcess{
				PID:       2000 + i,
				Command:   "concurrent",
				StartedAt: time.Now(),
				Done:      make(chan struct{}),
			})
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 10; i++ {
			manager.List()
			manager.Get(2000 + i)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify all processes were added
	pids := manager.List()
	if len(pids) != 10 {
		t.Errorf("Expected 10 PIDs, got %d", len(pids))
	}
}
