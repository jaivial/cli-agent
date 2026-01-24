package app

import "time"

type JobStatus string

const (
	JobRunning JobStatus = "running"
	JobExited  JobStatus = "exited"
	JobFailed  JobStatus = "failed"
)

type Job struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	PID       int       `json:"pid"`
	LogPath   string    `json:"log_path"`
	Status    JobStatus `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	ExitCode  int       `json:"exit_code"`
}
