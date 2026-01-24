package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
)

type Runner struct {
	Logger  *Logger
	JobRoot string
}

func NewRunner(logger *Logger, jobRoot string) *Runner {
	return &Runner{Logger: logger, JobRoot: jobRoot}
}

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
