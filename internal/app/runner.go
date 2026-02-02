package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// MaxOutputBufferSize limits the output buffer to prevent unbounded growth (1MB)
const MaxOutputBufferSize = 1024 * 1024

// BackgroundProcess represents a running background process
type BackgroundProcess struct {
	PID        int
	Command    string
	Stdin      io.WriteCloser
	Stdout     io.ReadCloser
	Stderr     io.ReadCloser
	OutputBuf  *strings.Builder
	OutputMu   sync.Mutex
	StartedAt  time.Time
	Done       chan struct{}
	ExitCode   *int
}

// BackgroundProcessManager manages background processes
type BackgroundProcessManager struct {
	processes map[int]*BackgroundProcess
	mu        sync.RWMutex
}

// NewBackgroundProcessManager creates a new manager
func NewBackgroundProcessManager() *BackgroundProcessManager {
	return &BackgroundProcessManager{
		processes: make(map[int]*BackgroundProcess),
	}
}

// Add registers a process
func (m *BackgroundProcessManager) Add(proc *BackgroundProcess) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processes[proc.PID] = proc
}

// Get retrieves a process by PID
func (m *BackgroundProcessManager) Get(pid int) (*BackgroundProcess, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	proc, ok := m.processes[pid]
	return proc, ok
}

// Remove unregisters a process
func (m *BackgroundProcessManager) Remove(pid int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.processes, pid)
}

// List returns all managed PIDs
func (m *BackgroundProcessManager) List() []int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pids := make([]int, 0, len(m.processes))
	for pid := range m.processes {
		pids = append(pids, pid)
	}
	return pids
}

// Global process manager instance
var globalProcessManager = NewBackgroundProcessManager()

// GetGlobalProcessManager returns the global process manager
func GetGlobalProcessManager() *BackgroundProcessManager {
	return globalProcessManager
}

// Runner executes shell commands and manages background processes.
type Runner struct {
	Logger  *Logger
	JobRoot string
}

// NewRunner creates a new runner with the specified logger and job root directory.
func NewRunner(logger *Logger, jobRoot string) *Runner {
	return &Runner{Logger: logger, JobRoot: jobRoot}
}

// Run executes a command and waits for completion, returning the exit code.
func (r *Runner) Run(ctx context.Context, command string) (int, error) {
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	err := cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// RunBackground starts a command in the background and returns a Job handle.
func (r *Runner) RunBackground(ctx context.Context, command string, store *JobStore) (Job, error) {
	if store == nil {
		return Job{}, errors.New("job store is required")
	}
	jobID := uuid.NewString()
	logPath := filepath.Join(r.JobRoot, fmt.Sprintf("%s.log", jobID))
	if err := os.MkdirAll(r.JobRoot, 0o755); err != nil {
		return Job{}, err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Job{}, err
	}

	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return Job{}, err
	}

	job := Job{
		ID:        jobID,
		Command:   command,
		PID:       cmd.Process.Pid,
		LogPath:   logPath,
		Status:    JobRunning,
		StartedAt: time.Now().UTC(),
	}
	if err := store.Save(job); err != nil {
		_ = logFile.Close()
		return Job{}, err
	}

	go func() {
		defer logFile.Close()
		err := cmd.Wait()
		if err != nil {
			job.Status = JobFailed
			job.EndedAt = time.Now().UTC()
			if exitErr, ok := err.(*exec.ExitError); ok {
				job.ExitCode = exitErr.ExitCode()
			} else {
				job.ExitCode = -1
			}
		} else {
			job.Status = JobExited
			job.EndedAt = time.Now().UTC()
			job.ExitCode = 0
		}
		_ = store.Save(job)
	}()

	return job, nil
}

// TailLog outputs the last N lines of a log file.
func (r *Runner) TailLog(path string, out io.Writer, lines int) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buffer := make([]string, 0, lines)
	for scanner.Scan() {
		buffer = append(buffer, scanner.Text())
		if len(buffer) > lines {
			buffer = buffer[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	for _, line := range buffer {
		_, _ = fmt.Fprintln(out, line)
	}
	return nil
}

// Stop terminates a running job.
func (r *Runner) Stop(job Job) error {
	if job.PID == 0 {
		return errors.New("job has no pid")
	}
	process, err := os.FindProcess(job.PID)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}

// IsPortAvailable checks if a TCP port is available for use
func (r *Runner) IsPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// WaitForPort waits for a port to become available (for VM services)
func (r *Runner) WaitForPort(port int, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		if !r.IsPortAvailable(port) {
			return true // Port is now in use (service started)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// RunVMWithStreaming runs a VM command with output streaming for boot detection
func (r *Runner) RunVMWithStreaming(ctx context.Context, command string, bootIndicators []string, timeout time.Duration) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	
	// Create pipes for streaming output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start VM: %w", err)
	}
	
	// Stream and monitor output
	var outputBuilder strings.Builder
	var outputMu sync.Mutex
	done := make(chan bool)
	
	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputMu.Lock()
			outputBuilder.WriteString(line + "\n")
			outputMu.Unlock()
			for _, indicator := range bootIndicators {
				if strings.Contains(line, indicator) {
					done <- true
					return
				}
			}
		}
	}()
	
	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			outputMu.Lock()
			outputBuilder.WriteString(line + "\n")
			outputMu.Unlock()
			for _, indicator := range bootIndicators {
				if strings.Contains(line, indicator) {
					done <- true
					return
				}
			}
		}
	}()
	
	// Wait for boot indicator or timeout
	select {
	case <-done:
		return outputBuilder.String(), nil
	case <-time.After(timeout):
		return outputBuilder.String(), fmt.Errorf("timeout waiting for VM boot indicators")
	case <-ctx.Done():
		return outputBuilder.String(), ctx.Err()
	}
}

// GetVMPid finds the PID of a running QEMU process
func (r *Runner) GetVMPid(vmName string) (int, error) {
	cmd := exec.Command("pgrep", "-f", "qemu.*"+vmName)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("VM not found: %s", vmName)
	}
	
	pidStr := strings.TrimSpace(string(output))
	pid := 0
	_, err = fmt.Sscanf(pidStr, "%d", &pid)
	if err != nil {
		return 0, fmt.Errorf("failed to parse PID: %w", err)
	}
	return pid, nil
}

// IsVMRunning checks if a QEMU VM is currently running
func (r *Runner) IsVMRunning(vmName string) bool {
	_, err := r.GetVMPid(vmName)
	return err == nil
}

// ExecBackground starts a background process and returns the PID
func (r *Runner) ExecBackground(command string) (*BackgroundProcess, error) {
	cmd := exec.Command("sh", "-c", command)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	proc := &BackgroundProcess{
		PID:       cmd.Process.Pid,
		Command:   command,
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    stderr,
		OutputBuf: &strings.Builder{},
		StartedAt: time.Now(),
		Done:      make(chan struct{}),
	}

	// Start goroutines to capture output
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			proc.appendOutput(scanner.Text())
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			proc.appendOutput(scanner.Text())
		}
	}()

	// Wait for process to complete
	go func() {
		err := cmd.Wait()
		proc.OutputMu.Lock()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				proc.ExitCode = &code
			} else {
				code := -1
				proc.ExitCode = &code
			}
		} else {
			code := 0
			proc.ExitCode = &code
		}
		proc.OutputMu.Unlock()
		close(proc.Done)

		// Cleanup after a delay to allow final output reads
		time.AfterFunc(5*time.Minute, func() {
			globalProcessManager.Remove(proc.PID)
		})
	}()

	globalProcessManager.Add(proc)
	return proc, nil
}

// appendOutput adds a line to the output buffer with size limiting
func (p *BackgroundProcess) appendOutput(line string) {
	p.OutputMu.Lock()
	defer p.OutputMu.Unlock()

	// If buffer exceeds limit, trim from the beginning
	if p.OutputBuf.Len() > MaxOutputBufferSize {
		content := p.OutputBuf.String()
		p.OutputBuf.Reset()
		// Keep last half
		p.OutputBuf.WriteString(content[len(content)/2:])
	}
	p.OutputBuf.WriteString(line)
	p.OutputBuf.WriteString("\n")
}

// SendInput sends input to a running background process
func (r *Runner) SendInput(pid int, input string) error {
	proc, ok := globalProcessManager.Get(pid)
	if !ok {
		return fmt.Errorf("process with PID %d not found", pid)
	}

	_, err := fmt.Fprint(proc.Stdin, input)
	if err != nil {
		return fmt.Errorf("failed to send input: %w", err)
	}
	return nil
}

// WaitForOutput waits for a specific pattern in the process output
func (r *Runner) WaitForOutput(pid int, pattern string, timeoutSecs int) (bool, string, error) {
	proc, ok := globalProcessManager.Get(pid)
	if !ok {
		return false, "", fmt.Errorf("process with PID %d not found", pid)
	}

	timeout := time.Duration(timeoutSecs) * time.Second
	if timeoutSecs <= 0 {
		timeout = 30 * time.Second
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, "", fmt.Errorf("invalid pattern: %w", err)
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeoutChan := time.After(timeout)

	for {
		select {
		case <-timeoutChan:
			proc.OutputMu.Lock()
			output := proc.OutputBuf.String()
			proc.OutputMu.Unlock()
			return false, output, fmt.Errorf("timeout waiting for pattern: %s", pattern)

		case <-proc.Done:
			proc.OutputMu.Lock()
			output := proc.OutputBuf.String()
			proc.OutputMu.Unlock()
			matched := re.MatchString(output)
			return matched, output, nil

		case <-ticker.C:
			proc.OutputMu.Lock()
			output := proc.OutputBuf.String()
			proc.OutputMu.Unlock()
			if re.MatchString(output) {
				return true, output, nil
			}
		}
	}
}
