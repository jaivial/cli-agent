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

// FileSessionStore is the legacy JSON-on-disk store. It is kept for backward
// compatibility and for importing into the SQLite store.
//
// Layout (inspired by opencode):
//
//	<root>/session/<projectID>/current
//	<root>/session/<projectID>/<sessionID>.json
//	<root>/message/<sessionID>/<msgID>.json
type FileSessionStore struct {
	Root string
}

type PromptHistory struct {
	Entries   []string  `json:"entries"`
	UpdatedAt time.Time `json:"updated_at"`
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

func NewFileSessionStore(root string) *FileSessionStore {
	if strings.TrimSpace(root) == "" {
		root = DefaultMemoryRoot()
	}
	return &FileSessionStore{Root: root}
}

func (s *FileSessionStore) projectID(workDir string) (string, string, error) {
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

func (s *FileSessionStore) sessionDir(projectID string) string {
	return filepath.Join(s.Root, "session", projectID)
}

func (s *FileSessionStore) messagesDir(sessionID string) string {
	return filepath.Join(s.Root, "message", sessionID)
}

func (s *FileSessionStore) promptHistoryDir() string {
	return filepath.Join(s.Root, "history")
}

func (s *FileSessionStore) currentSessionPath(projectID string) string {
	return filepath.Join(s.sessionDir(projectID), "current")
}

func (s *FileSessionStore) sessionPath(projectID, sessionID string) string {
	return filepath.Join(s.sessionDir(projectID), sessionID+".json")
}

func (s *FileSessionStore) promptHistoryPath(projectID string) string {
	return filepath.Join(s.promptHistoryDir(), projectID+".json")
}

func (s *FileSessionStore) CreateSession(workDir string) (*Session, error) {
	projectID, absWorkDir, err := s.projectID(workDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(s.sessionDir(projectID), 0o755); err != nil {
		return nil, err
	}

	now := time.Now()
	id := fmt.Sprintf("%d", now.UnixNano())
	sess := &Session{
		ID:         id,
		ProjectID:  projectID,
		WorkDir:    absWorkDir,
		RootID:     id,
		ChildIndex: 0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.SaveSession(sess); err != nil {
		return nil, err
	}
	if err := s.SetCurrentSession(absWorkDir, sess.ID); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *FileSessionStore) CreateChildSession(workDir string, parentSessionID string, summary string) (*Session, error) {
	parentSessionID = strings.TrimSpace(parentSessionID)
	if parentSessionID == "" {
		return nil, errors.New("missing parentSessionID")
	}
	parent, _, err := s.LoadSessionForWorkDir(workDir, parentSessionID)
	if err != nil {
		return nil, err
	}
	if parent == nil {
		return nil, errors.New("parent session not found")
	}
	rootID := strings.TrimSpace(parent.RootID)
	if rootID == "" {
		rootID = parent.ID
	}

	projectID, absWorkDir, err := s.projectID(workDir)
	if err != nil {
		return nil, err
	}
	if projectID != parent.ProjectID {
		// Be defensive: ensure parent session belongs to this workdir/project.
		return nil, fmt.Errorf("parent session project mismatch")
	}

	now := time.Now()
	id := fmt.Sprintf("%d", now.UnixNano())
	child := &Session{
		ID:             id,
		ProjectID:      projectID,
		WorkDir:        absWorkDir,
		Title:          parent.Title,
		ContextSummary: strings.TrimSpace(summary),
		RootID:         rootID,
		ParentID:       parent.ID,
		ChildIndex:     parent.ChildIndex + 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.SaveSession(child); err != nil {
		return nil, err
	}
	_ = s.SetCurrentSession(absWorkDir, child.ID)
	return child, nil
}

func (s *FileSessionStore) RootIDForSession(sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", errors.New("missing sessionID")
	}
	// Root id is stored inside the session JSON; locate by scanning all projects.
	// This is only used as a fallback, so keep it simple.
	sess, err := s.findSessionByID(sessionID)
	if err != nil {
		return "", err
	}
	if sess == nil {
		return "", errors.New("session not found")
	}
	if strings.TrimSpace(sess.RootID) != "" {
		return strings.TrimSpace(sess.RootID), nil
	}
	return sess.ID, nil
}

func (s *FileSessionStore) SetCurrentSession(workDir string, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing sessionID")
	}
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.sessionDir(projectID), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.currentSessionPath(projectID), []byte(sessionID), 0o644)
}

func (s *FileSessionStore) LoadOrCreateCurrentSession(workDir string) (*Session, []StoredMessage, error) {
	projectID, absWorkDir, err := s.projectID(workDir)
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(s.sessionDir(projectID), 0o755); err != nil {
		return nil, nil, err
	}

	curPath := s.currentSessionPath(projectID)
	if b, err := os.ReadFile(curPath); err == nil {
		sid := strings.TrimSpace(string(b))
		if sid != "" {
			if sess, msgs, err := s.LoadSessionForWorkDir(absWorkDir, sid); err == nil {
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

func (s *FileSessionStore) LoadSessionForWorkDir(workDir, sessionID string) (*Session, []StoredMessage, error) {
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return nil, nil, err
	}
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
	if strings.TrimSpace(sess.RootID) == "" {
		sess.RootID = sess.ID
		sess.ChildIndex = 0
	}

	msgs, err := s.LoadMessages(sessionID)
	if err != nil {
		return &sess, nil, err
	}
	return &sess, msgs, nil
}

func (s *FileSessionStore) SaveSession(sess *Session) error {
	if sess == nil {
		return errors.New("nil session")
	}
	if strings.TrimSpace(sess.ProjectID) == "" || strings.TrimSpace(sess.ID) == "" {
		return errors.New("missing session fields")
	}
	if strings.TrimSpace(sess.RootID) == "" {
		sess.RootID = sess.ID
	}
	if err := os.MkdirAll(s.sessionDir(sess.ProjectID), 0o755); err != nil {
		return err
	}
	sess.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.sessionPath(sess.ProjectID, sess.ID), b, 0o644)
}

func (s *FileSessionStore) TouchSession(workDir string, sessionID string) error {
	sess, _, err := s.LoadSessionForWorkDir(workDir, sessionID)
	if err != nil {
		return err
	}
	return s.SaveSession(sess)
}

func (s *FileSessionStore) AppendMessage(msg StoredMessage) error {
	if strings.TrimSpace(msg.SessionID) == "" {
		return errors.New("missing sessionID")
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if err := os.MkdirAll(s.messagesDir(msg.SessionID), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(&msg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.messagesDir(msg.SessionID), msg.ID+".json")
	return os.WriteFile(path, b, 0o644)
}

func (s *FileSessionStore) findSessionByID(sessionID string) (*Session, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("missing sessionID")
	}
	root := strings.TrimSpace(s.Root)
	if root == "" {
		return nil, errors.New("missing root")
	}
	root = filepath.Clean(root)
	base := filepath.Join(root, "session")
	projects, err := os.ReadDir(base)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		projectID := strings.TrimSpace(p.Name())
		if projectID == "" {
			continue
		}
		path := s.sessionPath(projectID, sessionID)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var sess Session
		if err := json.Unmarshal(b, &sess); err != nil {
			continue
		}
		if strings.TrimSpace(sess.RootID) == "" {
			sess.RootID = sess.ID
			sess.ChildIndex = 0
		}
		return &sess, nil
	}
	return nil, nil
}

func (s *FileSessionStore) LoadMessages(sessionID string) ([]StoredMessage, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("missing sessionID")
	}

	// Load all messages for the root conversation so UI history survives compaction.
	rootID := ""
	if sess, err := s.findSessionByID(sessionID); err == nil && sess != nil {
		rootID = strings.TrimSpace(sess.RootID)
		if rootID == "" {
			rootID = sess.ID
		}
	}
	if rootID == "" {
		rootID = sessionID
	}

	rootSessionIDs := []string{}
	{
		base := filepath.Join(filepath.Clean(s.Root), "session")
		projects, _ := os.ReadDir(base)
		for _, p := range projects {
			if !p.IsDir() {
				continue
			}
			projectID := strings.TrimSpace(p.Name())
			if projectID == "" {
				continue
			}
			ents, _ := os.ReadDir(s.sessionDir(projectID))
			for _, e := range ents {
				if e.IsDir() {
					continue
				}
				name := strings.TrimSpace(e.Name())
				if !strings.HasSuffix(name, ".json") {
					continue
				}
				sid := strings.TrimSuffix(name, ".json")
				if sid == "" {
					continue
				}
				b, err := os.ReadFile(s.sessionPath(projectID, sid))
				if err != nil {
					continue
				}
				var sess Session
				if err := json.Unmarshal(b, &sess); err != nil {
					continue
				}
				sessRoot := strings.TrimSpace(sess.RootID)
				if sessRoot == "" {
					sessRoot = sess.ID
				}
				if sessRoot == rootID {
					rootSessionIDs = append(rootSessionIDs, sid)
				}
			}
		}
	}
	if len(rootSessionIDs) == 0 {
		rootSessionIDs = append(rootSessionIDs, sessionID)
	}

	var msgs []StoredMessage
	for _, sid := range rootSessionIDs {
		dir := s.messagesDir(sid)
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
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

func (s *FileSessionStore) ClearSessionMessages(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing sessionID")
	}
	// Clear all messages for the root conversation.
	msgs, err := s.LoadMessages(sessionID)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(msgs))
	for _, m := range msgs {
		if m.SessionID == "" || m.ID == "" {
			continue
		}
		key := m.SessionID + ":" + m.ID
		seen[key] = struct{}{}
	}
	for key := range seen {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		sid := parts[0]
		id := parts[1]
		_ = os.Remove(filepath.Join(s.messagesDir(sid), id+".json"))
	}
	return nil
}

func (s *FileSessionStore) DeleteSessionChain(workDir string, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing sessionID")
	}
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return err
	}

	sess, _, err := s.LoadSessionForWorkDir(workDir, sessionID)
	if err != nil {
		return err
	}
	if sess == nil {
		return errors.New("session not found")
	}
	rootID := strings.TrimSpace(sess.RootID)
	if rootID == "" {
		rootID = sess.ID
	}

	dir := s.sessionDir(projectID)
	ents, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	deleted := make(map[string]struct{}, 4)
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSpace(e.Name())
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		if id == "" {
			continue
		}
		b, err := os.ReadFile(s.sessionPath(projectID, id))
		if err != nil {
			continue
		}
		var candidate Session
		if err := json.Unmarshal(b, &candidate); err != nil {
			continue
		}
		candRoot := strings.TrimSpace(candidate.RootID)
		if candRoot == "" {
			candRoot = candidate.ID
		}
		if candRoot != rootID {
			continue
		}
		deleted[id] = struct{}{}
	}

	if len(deleted) == 0 {
		return nil
	}

	for id := range deleted {
		_ = os.Remove(s.sessionPath(projectID, id))
		_ = os.RemoveAll(s.messagesDir(id))
	}

	// Clear the current session pointer if it referenced a deleted session.
	curPath := s.currentSessionPath(projectID)
	if b, err := os.ReadFile(curPath); err == nil {
		curID := strings.TrimSpace(string(b))
		if curID != "" {
			if _, ok := deleted[curID]; ok {
				_ = os.Remove(curPath)
			}
		}
	}

	return nil
}

func (s *FileSessionStore) ListSessionsForWorkDir(workDir string, limit int) ([]SessionSummary, error) {
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
		sess, msgs, err := s.LoadSessionForWorkDir(workDir, sessionID)
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

func (s *FileSessionStore) SavePromptHistory(workDir string, entries []string) error {
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return err
	}
	history := PromptHistory{
		Entries:   normalizePromptHistory(entries, 200),
		UpdatedAt: time.Now(),
	}
	if err := os.MkdirAll(s.promptHistoryDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.promptHistoryPath(projectID), b, 0o644)
}

func (s *FileSessionStore) LoadPromptHistory(workDir string) ([]string, error) {
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
