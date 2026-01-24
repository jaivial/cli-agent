package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type JobStore struct {
	path string
	mu   sync.Mutex
	jobs map[string]Job
}

func NewJobStore(path string) (*JobStore, error) {
	store := &JobStore{path: path, jobs: map[string]Job{}}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *JobStore) load() error {
	if s.path == "" {
		return errors.New("job store path required")
	}
	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &s.jobs)
}

func (s *JobStore) Save(job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return errors.New("job store path required")
	}
	s.jobs[job.ID] = job
	payload, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, payload, 0o644)
}

func (s *JobStore) List() ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *JobStore) Get(id string) (Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	return job, ok
}
