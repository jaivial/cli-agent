package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MemoryStore persists chat sessions/messages on disk.
//
// Layout (inspired by opencode):
//
//	<root>/session/<projectID>/current
//	<root>/session/<projectID>/<sessionID>.json
//	<root>/message/<sessionID>/<msgID>.json
type MemoryStore struct {
	Root string
}

type PromptHistory struct {
	Entries   []string  `json:"entries"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Session struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	WorkDir   string `json:"work_dir"`
	Title     string `json:"title,omitempty"`
	// ContextSummary stores compacted session context used when chat history grows
	// beyond model limits.
	ContextSummary string    `json:"context_summary,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type StoredMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // user|assistant|system|error
	Content   string    `json:"content"`
	Mode      string    `json:"mode,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SessionSummary struct {
	Session      Session       `json:"session"`
	MessageCount int           `json:"message_count"`
	WorkDuration time.Duration `json:"work_duration"`
	LastActivity time.Time     `json:"last_activity"`
}

func DefaultMemoryRoot() string {
	// Prefer XDG data dir (Linux/macOS). If unavailable, fall back to ~/.local/share.
	if base := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); base != "" {
		return filepath.Join(base, "cli-agent", "storage")
	}
	if base, err := os.UserHomeDir(); err == nil && base != "" {
		return filepath.Join(base, ".local", "share", "cli-agent", "storage")
	}
	return filepath.Join(os.TempDir(), "cli-agent", "storage")
}

func NewMemoryStore(root string) *MemoryStore {
	if strings.TrimSpace(root) == "" {
		root = DefaultMemoryRoot()
	}
	return &MemoryStore{Root: root}
}

func (s *MemoryStore) projectID(workDir string) (string, string, error) {
	wd := strings.TrimSpace(workDir)
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			return "", "", err
		}
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(abs))
	id := hex.EncodeToString(sum[:])[:16]
	return id, abs, nil
}

func (s *MemoryStore) sessionDir(projectID string) string {
	return filepath.Join(s.Root, "session", projectID)
}

func (s *MemoryStore) messagesDir(sessionID string) string {
	return filepath.Join(s.Root, "message", sessionID)
}

func (s *MemoryStore) promptHistoryDir() string {
	return filepath.Join(s.Root, "history")
}

func (s *MemoryStore) currentSessionPath(projectID string) string {
	return filepath.Join(s.sessionDir(projectID), "current")
}

func (s *MemoryStore) sessionPath(projectID, sessionID string) string {
	return filepath.Join(s.sessionDir(projectID), sessionID+".json")
}

func (s *MemoryStore) promptHistoryPath(projectID string) string {
	return filepath.Join(s.promptHistoryDir(), projectID+".json")
}

func (s *MemoryStore) CreateSession(workDir string) (*Session, error) {
	projectID, absWorkDir, err := s.projectID(workDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.sessionDir(projectID), 0755); err != nil {
		return nil, err
	}

	now := time.Now()
	sess := &Session{
		ID:        fmt.Sprintf("%d", now.UnixNano()),
		ProjectID: projectID,
		WorkDir:   absWorkDir,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.SaveSession(sess); err != nil {
		return nil, err
	}
	if err := s.SetCurrentSession(workDir, sess.ID); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *MemoryStore) SetCurrentSession(workDir string, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing sessionID")
	}
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.sessionDir(projectID), 0755); err != nil {
		return err
	}
	return os.WriteFile(s.currentSessionPath(projectID), []byte(sessionID), 0644)
}

func (s *MemoryStore) LoadOrCreateCurrentSession(workDir string) (*Session, []StoredMessage, error) {
	projectID, absWorkDir, err := s.projectID(workDir)
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(s.sessionDir(projectID), 0755); err != nil {
		return nil, nil, err
	}

	curPath := s.currentSessionPath(projectID)
	if b, err := os.ReadFile(curPath); err == nil {
		sid := strings.TrimSpace(string(b))
		if sid != "" {
			if sess, msgs, err := s.LoadSession(projectID, sid); err == nil {
				return sess, msgs, nil
			}
		}
	}

	sess, err := s.CreateSession(absWorkDir)
	if err != nil {
		return nil, nil, err
	}
	return sess, []StoredMessage{}, nil
}

func (s *MemoryStore) LoadSession(projectID, sessionID string) (*Session, []StoredMessage, error) {
	if strings.TrimSpace(projectID) == "" || strings.TrimSpace(sessionID) == "" {
		return nil, nil, errors.New("missing projectID or sessionID")
	}
	b, err := os.ReadFile(s.sessionPath(projectID, sessionID))
	if err != nil {
		return nil, nil, err
	}
	var sess Session
	if err := json.Unmarshal(b, &sess); err != nil {
		return nil, nil, err
	}
	msgs, err := s.LoadMessages(sessionID)
	if err != nil {
		return &sess, nil, err
	}
	return &sess, msgs, nil
}

func (s *MemoryStore) LoadSessionForWorkDir(workDir, sessionID string) (*Session, []StoredMessage, error) {
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return nil, nil, err
	}
	return s.LoadSession(projectID, sessionID)
}

func (s *MemoryStore) SaveSession(sess *Session) error {
	if sess == nil {
		return errors.New("nil session")
	}
	if strings.TrimSpace(sess.ProjectID) == "" || strings.TrimSpace(sess.ID) == "" {
		return errors.New("missing session fields")
	}
	if err := os.MkdirAll(s.sessionDir(sess.ProjectID), 0755); err != nil {
		return err
	}
	sess.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.sessionPath(sess.ProjectID, sess.ID), b, 0644)
}

func (s *MemoryStore) TouchSession(workDir string, sessionID string) error {
	sess, _, err := s.LoadSessionForWorkDir(workDir, sessionID)
	if err != nil {
		return err
	}
	return s.SaveSession(sess)
}

func (s *MemoryStore) AppendMessage(msg StoredMessage) error {
	if strings.TrimSpace(msg.SessionID) == "" {
		return errors.New("missing sessionID")
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if err := os.MkdirAll(s.messagesDir(msg.SessionID), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(&msg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.messagesDir(msg.SessionID), msg.ID+".json")
	return os.WriteFile(path, b, 0644)
}

func (s *MemoryStore) LoadMessages(sessionID string) ([]StoredMessage, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("missing sessionID")
	}
	dir := s.messagesDir(sessionID)
	ents, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []StoredMessage{}, nil
		}
		return nil, err
	}

	var msgs []StoredMessage
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var m StoredMessage
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}

	sort.Slice(msgs, func(i, j int) bool {
		// Stable ordering by timestamp then ID.
		if msgs[i].CreatedAt.Equal(msgs[j].CreatedAt) {
			return msgs[i].ID < msgs[j].ID
		}
		return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
	})
	return msgs, nil
}

func (s *MemoryStore) ClearSessionMessages(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("missing sessionID")
	}
	dir := s.messagesDir(sessionID)
	ents, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
	return nil
}

func (s *MemoryStore) ListSessionsForWorkDir(workDir string, limit int) ([]SessionSummary, error) {
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return nil, err
	}
	dir := s.sessionDir(projectID)
	ents, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []SessionSummary{}, nil
		}
		return nil, err
	}

	now := time.Now()
	summaries := make([]SessionSummary, 0, len(ents))

	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSpace(e.Name())
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		sessionID := strings.TrimSuffix(name, ".json")
		if sessionID == "" {
			continue
		}
		sess, msgs, err := s.LoadSession(projectID, sessionID)
		if err != nil {
			continue
		}

		workDuration, lastMsg := estimateSessionWorkDuration(msgs, now)
		lastActivity := sess.UpdatedAt
		if lastActivity.IsZero() {
			lastActivity = sess.CreatedAt
		}
		if !lastMsg.IsZero() && lastMsg.After(lastActivity) {
			lastActivity = lastMsg
		}

		summaries = append(summaries, SessionSummary{
			Session:      *sess,
			MessageCount: len(msgs),
			WorkDuration: workDuration,
			LastActivity: lastActivity,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].LastActivity.Equal(summaries[j].LastActivity) {
			if summaries[i].Session.UpdatedAt.Equal(summaries[j].Session.UpdatedAt) {
				return summaries[i].Session.CreatedAt.After(summaries[j].Session.CreatedAt)
			}
			return summaries[i].Session.UpdatedAt.After(summaries[j].Session.UpdatedAt)
		}
		return summaries[i].LastActivity.After(summaries[j].LastActivity)
	})

	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}
	return summaries, nil
}

func normalizePromptHistory(entries []string, max int) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1] == entry {
			continue
		}
		out = append(out, entry)
	}
	if max > 0 && len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}

func (s *MemoryStore) SavePromptHistory(workDir string, entries []string) error {
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return err
	}
	history := PromptHistory{
		Entries:   normalizePromptHistory(entries, 200),
		UpdatedAt: time.Now(),
	}
	if err := os.MkdirAll(s.promptHistoryDir(), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.promptHistoryPath(projectID), b, 0644)
}

func (s *MemoryStore) LoadPromptHistory(workDir string) ([]string, error) {
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.promptHistoryPath(projectID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}

	var payload PromptHistory
	if err := json.Unmarshal(b, &payload); err == nil {
		return normalizePromptHistory(payload.Entries, 200), nil
	}

	// Backward-compatible fallback if file content is a raw JSON string array.
	var raw []string
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return normalizePromptHistory(raw, 200), nil
}

const (
	workIdleGap  = 15 * time.Minute
	workMinSlice = 30 * time.Second
)

func estimateSessionWorkDuration(msgs []StoredMessage, now time.Time) (time.Duration, time.Time) {
	if len(msgs) == 0 {
		return 0, time.Time{}
	}

	times := make([]time.Time, 0, len(msgs))
	for _, m := range msgs {
		if !m.CreatedAt.IsZero() {
			times = append(times, m.CreatedAt)
		}
	}
	if len(times) == 0 {
		return 0, time.Time{}
	}
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	last := times[len(times)-1]
	start := times[0]
	prev := times[0]
	total := time.Duration(0)

	for i := 1; i < len(times); i++ {
		ts := times[i]
		if ts.Sub(prev) > workIdleGap {
			total += withMinWorkSlice(prev.Sub(start))
			start = ts
		}
		prev = ts
	}

	segmentEnd := prev
	if !now.IsZero() && now.After(prev) && now.Sub(prev) <= workIdleGap {
		segmentEnd = now
	}
	total += withMinWorkSlice(segmentEnd.Sub(start))

	return total, last
}

func withMinWorkSlice(d time.Duration) time.Duration {
	if d < workMinSlice {
		return workMinSlice
	}
	return d
}
