package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteSessionStore struct {
	Root   string
	dbPath string

	mu   sync.Mutex
	db   *sql.DB
	once sync.Once
	err  error

	// Used only for legacy import.
	legacy *FileSessionStore
}

func NewSQLiteSessionStore(root string) (*SQLiteSessionStore, error) {
	if strings.TrimSpace(root) == "" {
		root = DefaultMemoryRoot()
	}
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	st := &SQLiteSessionStore{
		Root:   root,
		dbPath: filepath.Join(root, "cli-agent.db"),
		legacy: NewFileSessionStore(root),
	}
	// Initialize eagerly so callers fail fast.
	if err := st.init(); err != nil {
		return nil, err
	}
	// One-time best-effort import.
	_ = st.importLegacyIfNeeded()
	return st, nil
}

func (s *SQLiteSessionStore) init() error {
	s.once.Do(func() {
		db, err := sql.Open("sqlite", s.dbPath)
		if err != nil {
			s.err = err
			return
		}
		// Keep sqlite responsive under contention.
		_, _ = db.Exec("PRAGMA busy_timeout = 5000;")
		_, _ = db.Exec("PRAGMA journal_mode = WAL;")
		_, _ = db.Exec("PRAGMA synchronous = NORMAL;")

		schema := []string{
			`CREATE TABLE IF NOT EXISTS sessions (
					id TEXT PRIMARY KEY,
					root_id TEXT NOT NULL,
					parent_id TEXT,
				child_index INTEGER NOT NULL,
				project_id TEXT NOT NULL,
				work_dir TEXT NOT NULL,
				title TEXT,
				context_summary TEXT,
				created_at_ns INTEGER NOT NULL,
				updated_at_ns INTEGER NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_sessions_project_updated ON sessions(project_id, updated_at_ns);`,
			`CREATE INDEX IF NOT EXISTS idx_sessions_root_child ON sessions(root_id, child_index);`,
			`CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions(parent_id);`,
			`CREATE TABLE IF NOT EXISTS current_sessions (
				project_id TEXT PRIMARY KEY,
				session_id TEXT NOT NULL,
				updated_at_ns INTEGER NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS messages (
					id TEXT NOT NULL,
					session_id TEXT NOT NULL,
					root_id TEXT NOT NULL,
					role TEXT NOT NULL,
					mode TEXT,
					content TEXT NOT NULL,
					created_at_ns INTEGER NOT NULL,
					PRIMARY KEY (session_id, id)
				);`,
			`CREATE INDEX IF NOT EXISTS idx_messages_root_created ON messages(root_id, created_at_ns);`,
			`CREATE TABLE IF NOT EXISTS collab_runs (
					run_id TEXT PRIMARY KEY,
					session_id TEXT,
					root_id TEXT NOT NULL,
					status TEXT,
					created_at_ns INTEGER NOT NULL,
					updated_at_ns INTEGER NOT NULL
				);`,
			`CREATE INDEX IF NOT EXISTS idx_collab_runs_root_updated ON collab_runs(root_id, updated_at_ns);`,
			`CREATE TABLE IF NOT EXISTS collab_messages (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					run_id TEXT NOT NULL,
					session_id TEXT,
					from_agent TEXT NOT NULL,
					to_agent TEXT,
					kind TEXT,
					scope TEXT,
					body TEXT NOT NULL,
					created_at_ns INTEGER NOT NULL
				);`,
			`CREATE INDEX IF NOT EXISTS idx_collab_messages_run_id ON collab_messages(run_id, id);`,
			`CREATE TABLE IF NOT EXISTS collab_claims (
					run_id TEXT NOT NULL,
					scope TEXT NOT NULL,
					claimed_by TEXT NOT NULL,
					claimed_at_ns INTEGER NOT NULL,
					expires_at_ns INTEGER,
					PRIMARY KEY (run_id, scope)
				);`,
			`CREATE INDEX IF NOT EXISTS idx_collab_claims_run ON collab_claims(run_id);`,
		}
		for _, stmt := range schema {
			if _, err := db.Exec(stmt); err != nil {
				_ = db.Close()
				s.err = err
				return
			}
		}

		s.db = db
	})
	return s.err
}

func (s *SQLiteSessionStore) dbConn() (*sql.DB, error) {
	if err := s.init(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	db := s.db
	s.mu.Unlock()
	if db == nil {
		return nil, errors.New("sqlite store unavailable")
	}
	return db, nil
}

func (s *SQLiteSessionStore) projectID(workDir string) (string, string, error) {
	// Reuse the legacy store logic for stable project IDs.
	if s.legacy == nil {
		s.legacy = NewFileSessionStore(s.Root)
	}
	return s.legacy.projectID(workDir)
}

func (s *SQLiteSessionStore) RootIDForSession(sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", errors.New("missing sessionID")
	}
	db, err := s.dbConn()
	if err != nil {
		return "", err
	}
	var rootID string
	err = db.QueryRow(`SELECT root_id FROM sessions WHERE id = ?`, sessionID).Scan(&rootID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("session not found")
		}
		return "", err
	}
	rootID = strings.TrimSpace(rootID)
	if rootID == "" {
		return sessionID, nil
	}
	return rootID, nil
}

func (s *SQLiteSessionStore) SetCurrentSession(workDir string, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing sessionID")
	}
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return err
	}
	db, err := s.dbConn()
	if err != nil {
		return err
	}
	now := time.Now().UnixNano()
	_, err = db.Exec(
		`INSERT INTO current_sessions(project_id, session_id, updated_at_ns)
		 VALUES(?, ?, ?)
		 ON CONFLICT(project_id) DO UPDATE SET session_id=excluded.session_id, updated_at_ns=excluded.updated_at_ns`,
		projectID, sessionID, now,
	)
	return err
}

func (s *SQLiteSessionStore) CreateSession(workDir string) (*Session, error) {
	projectID, absWorkDir, err := s.projectID(workDir)
	if err != nil {
		return nil, err
	}
	db, err := s.dbConn()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	id := fmt.Sprintf("%d", now.UnixNano())
	sess := &Session{
		ID:         id,
		ProjectID:  projectID,
		WorkDir:    absWorkDir,
		RootID:     id,
		ParentID:   "",
		ChildIndex: 0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	_, err = db.Exec(
		`INSERT INTO sessions(id, root_id, parent_id, child_index, project_id, work_dir, title, context_summary, created_at_ns, updated_at_ns)
		 VALUES(?, ?, NULL, 0, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.RootID, sess.ProjectID, sess.WorkDir, nullIfEmpty(sess.Title), nullIfEmpty(sess.ContextSummary), sess.CreatedAt.UnixNano(), sess.UpdatedAt.UnixNano(),
	)
	if err != nil {
		return nil, err
	}
	_ = s.SetCurrentSession(absWorkDir, sess.ID)
	return sess, nil
}

func (s *SQLiteSessionStore) CreateChildSession(workDir string, parentSessionID string, summary string) (*Session, error) {
	parentSessionID = strings.TrimSpace(parentSessionID)
	if parentSessionID == "" {
		return nil, errors.New("missing parentSessionID")
	}
	db, err := s.dbConn()
	if err != nil {
		return nil, err
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

	now := time.Now()
	id := fmt.Sprintf("%d", now.UnixNano())
	child := &Session{
		ID:             id,
		ProjectID:      parent.ProjectID,
		WorkDir:        parent.WorkDir,
		Title:          parent.Title,
		ContextSummary: strings.TrimSpace(summary),
		RootID:         rootID,
		ParentID:       parent.ID,
		ChildIndex:     parent.ChildIndex + 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	_, err = db.Exec(
		`INSERT INTO sessions(id, root_id, parent_id, child_index, project_id, work_dir, title, context_summary, created_at_ns, updated_at_ns)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		child.ID, child.RootID, nullIfEmpty(child.ParentID), child.ChildIndex, child.ProjectID, child.WorkDir, nullIfEmpty(child.Title), nullIfEmpty(child.ContextSummary), child.CreatedAt.UnixNano(), child.UpdatedAt.UnixNano(),
	)
	if err != nil {
		return nil, err
	}
	_ = s.SetCurrentSession(child.WorkDir, child.ID)
	return child, nil
}

func (s *SQLiteSessionStore) LoadOrCreateCurrentSession(workDir string) (*Session, []StoredMessage, error) {
	projectID, absWorkDir, err := s.projectID(workDir)
	if err != nil {
		return nil, nil, err
	}
	db, err := s.dbConn()
	if err != nil {
		return nil, nil, err
	}
	var sessionID string
	err = db.QueryRow(`SELECT session_id FROM current_sessions WHERE project_id = ?`, projectID).Scan(&sessionID)
	if err == nil && strings.TrimSpace(sessionID) != "" {
		if sess, msgs, err := s.LoadSessionForWorkDir(absWorkDir, sessionID); err == nil && sess != nil {
			return sess, msgs, nil
		}
	}
	sess, err := s.CreateSession(absWorkDir)
	if err != nil {
		return nil, nil, err
	}
	return sess, []StoredMessage{}, nil
}

func (s *SQLiteSessionStore) LoadSessionForWorkDir(workDir, sessionID string) (*Session, []StoredMessage, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, nil, errors.New("missing sessionID")
	}
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return nil, nil, err
	}
	db, err := s.dbConn()
	if err != nil {
		return nil, nil, err
	}

	var sess Session
	var title sql.NullString
	var summary sql.NullString
	var parentID sql.NullString
	var createdNS int64
	var updatedNS int64
	err = db.QueryRow(
		`SELECT id, root_id, parent_id, child_index, project_id, work_dir, title, context_summary, created_at_ns, updated_at_ns
		 FROM sessions WHERE id = ? AND project_id = ?`,
		sessionID, projectID,
	).Scan(&sess.ID, &sess.RootID, &parentID, &sess.ChildIndex, &sess.ProjectID, &sess.WorkDir, &title, &summary, &createdNS, &updatedNS)
	if err != nil {
		return nil, nil, err
	}
	if parentID.Valid {
		sess.ParentID = parentID.String
	}
	if title.Valid {
		sess.Title = title.String
	}
	if summary.Valid {
		sess.ContextSummary = summary.String
	}
	sess.CreatedAt = time.Unix(0, createdNS)
	sess.UpdatedAt = time.Unix(0, updatedNS)
	if strings.TrimSpace(sess.RootID) == "" {
		sess.RootID = sess.ID
	}

	msgs, err := s.LoadMessages(sess.ID)
	if err != nil {
		return &sess, nil, err
	}
	return &sess, msgs, nil
}

func (s *SQLiteSessionStore) SaveSession(sess *Session) error {
	if sess == nil {
		return errors.New("nil session")
	}
	if strings.TrimSpace(sess.ID) == "" {
		return errors.New("missing session id")
	}
	db, err := s.dbConn()
	if err != nil {
		return err
	}
	now := time.Now()
	sess.UpdatedAt = now
	_, err = db.Exec(
		`UPDATE sessions
		 SET title = ?, context_summary = ?, updated_at_ns = ?
		 WHERE id = ?`,
		nullIfEmpty(sess.Title), nullIfEmpty(sess.ContextSummary), now.UnixNano(), sess.ID,
	)
	return err
}

func (s *SQLiteSessionStore) TouchSession(workDir string, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing sessionID")
	}
	sess, _, err := s.LoadSessionForWorkDir(workDir, sessionID)
	if err != nil {
		return err
	}
	return s.SaveSession(sess)
}

func (s *SQLiteSessionStore) AppendMessage(msg StoredMessage) error {
	if strings.TrimSpace(msg.SessionID) == "" {
		return errors.New("missing sessionID")
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	db, err := s.dbConn()
	if err != nil {
		return err
	}

	rootID, err := s.RootIDForSession(msg.SessionID)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		`INSERT OR REPLACE INTO messages(id, session_id, root_id, role, mode, content, created_at_ns)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, rootID, strings.TrimSpace(msg.Role), nullIfEmpty(msg.Mode), msg.Content, msg.CreatedAt.UnixNano(),
	)
	return err
}

func (s *SQLiteSessionStore) LoadMessages(sessionID string) ([]StoredMessage, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("missing sessionID")
	}
	db, err := s.dbConn()
	if err != nil {
		return nil, err
	}

	rootID, err := s.RootIDForSession(sessionID)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(
		`SELECT id, session_id, role, mode, content, created_at_ns
		 FROM messages
		 WHERE root_id = ?
		 ORDER BY created_at_ns ASC, id ASC`,
		rootID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]StoredMessage, 0, 64)
	for rows.Next() {
		var m StoredMessage
		var mode sql.NullString
		var createdNS int64
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &mode, &m.Content, &createdNS); err != nil {
			continue
		}
		if mode.Valid {
			m.Mode = mode.String
		}
		m.CreatedAt = time.Unix(0, createdNS)
		out = append(out, m)
	}
	return out, nil
}

func (s *SQLiteSessionStore) ClearSessionMessages(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing sessionID")
	}
	db, err := s.dbConn()
	if err != nil {
		return err
	}
	rootID, err := s.RootIDForSession(sessionID)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM messages WHERE root_id = ?`, rootID)
	return err
}

func (s *SQLiteSessionStore) DeleteSessionChain(workDir string, sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("missing sessionID")
	}
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return err
	}
	db, err := s.dbConn()
	if err != nil {
		return err
	}

	var rootID string
	err = db.QueryRow(`SELECT root_id FROM sessions WHERE id = ? AND project_id = ?`, sessionID, projectID).Scan(&rootID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("session not found")
		}
		return err
	}
	rootID = strings.TrimSpace(rootID)
	if rootID == "" {
		rootID = sessionID
	}

	rows, err := db.Query(`SELECT id FROM sessions WHERE root_id = ? AND project_id = ?`, rootID, projectID)
	if err != nil {
		return err
	}
	defer rows.Close()

	sessionIDs := make([]string, 0, 8)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		id = strings.TrimSpace(id)
		if id != "" {
			sessionIDs = append(sessionIDs, id)
		}
	}
	if len(sessionIDs) == 0 {
		return errors.New("session not found")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// If the deleted session chain was active for this project, clear the pointer.
	var curID string
	_ = tx.QueryRow(`SELECT session_id FROM current_sessions WHERE project_id = ?`, projectID).Scan(&curID)
	if strings.TrimSpace(curID) != "" {
		for _, id := range sessionIDs {
			if id == curID {
				_, _ = tx.Exec(`DELETE FROM current_sessions WHERE project_id = ?`, projectID)
				break
			}
		}
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(sessionIDs)), ",")
	args := make([]interface{}, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		args = append(args, id)
	}

	if _, err := tx.Exec(`DELETE FROM messages WHERE session_id IN (`+placeholders+`)`, args...); err != nil {
		return err
	}
	// Collab state is tied to the root conversation; delete it too.
	if _, err := tx.Exec(`DELETE FROM collab_messages WHERE run_id IN (SELECT run_id FROM collab_runs WHERE root_id = ?)`, rootID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM collab_claims WHERE run_id IN (SELECT run_id FROM collab_runs WHERE root_id = ?)`, rootID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM collab_runs WHERE root_id = ?`, rootID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE id IN (`+placeholders+`)`, args...); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteSessionStore) ListSessionsForWorkDir(workDir string, limit int) ([]SessionSummary, error) {
	projectID, _, err := s.projectID(workDir)
	if err != nil {
		return nil, err
	}
	db, err := s.dbConn()
	if err != nil {
		return nil, err
	}

	q := `
		SELECT id, root_id, parent_id, child_index, project_id, work_dir, title, context_summary, created_at_ns, updated_at_ns
		FROM sessions
		WHERE project_id = ?
		  AND id NOT IN (SELECT DISTINCT parent_id FROM sessions WHERE parent_id IS NOT NULL)
		ORDER BY updated_at_ns DESC
	`
	args := []interface{}{projectID}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	summaries := make([]SessionSummary, 0, 16)
	for rows.Next() {
		var sess Session
		var title sql.NullString
		var summary sql.NullString
		var parentID sql.NullString
		var createdNS int64
		var updatedNS int64
		if err := rows.Scan(&sess.ID, &sess.RootID, &parentID, &sess.ChildIndex, &sess.ProjectID, &sess.WorkDir, &title, &summary, &createdNS, &updatedNS); err != nil {
			continue
		}
		if parentID.Valid {
			sess.ParentID = parentID.String
		}
		if title.Valid {
			sess.Title = title.String
		}
		if summary.Valid {
			sess.ContextSummary = summary.String
		}
		sess.CreatedAt = time.Unix(0, createdNS)
		sess.UpdatedAt = time.Unix(0, updatedNS)
		if strings.TrimSpace(sess.RootID) == "" {
			sess.RootID = sess.ID
		}

		msgs, _ := s.LoadMessages(sess.ID)
		workDuration, lastMsg := estimateSessionWorkDuration(msgs, now)
		lastActivity := sess.UpdatedAt
		if lastActivity.IsZero() {
			lastActivity = sess.CreatedAt
		}
		if !lastMsg.IsZero() && lastMsg.After(lastActivity) {
			lastActivity = lastMsg
		}

		summaries = append(summaries, SessionSummary{
			Session:      sess,
			MessageCount: len(msgs),
			WorkDuration: workDuration,
			LastActivity: lastActivity,
		})
	}

	return summaries, nil
}

func (s *SQLiteSessionStore) SavePromptHistory(workDir string, entries []string) error {
	// Keep prompt history stored as a JSON file for now to avoid schema churn.
	if s.legacy == nil {
		s.legacy = NewFileSessionStore(s.Root)
	}
	return s.legacy.SavePromptHistory(workDir, entries)
}

func (s *SQLiteSessionStore) LoadPromptHistory(workDir string) ([]string, error) {
	if s.legacy == nil {
		s.legacy = NewFileSessionStore(s.Root)
	}
	return s.legacy.LoadPromptHistory(workDir)
}

func (s *SQLiteSessionStore) importLegacyIfNeeded() error {
	db, err := s.dbConn()
	if err != nil {
		return err
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(1) FROM sessions`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	// Import sessions and messages from the legacy file layout, best-effort.
	base := filepath.Join(s.Root, "session")
	projects, err := os.ReadDir(base)
	if err != nil {
		return nil
	}

	type legacySession struct {
		ProjectID string
		SessionID string
		Path      string
	}

	var sessions []legacySession
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		projectID := strings.TrimSpace(p.Name())
		if projectID == "" {
			continue
		}
		ents, err := os.ReadDir(filepath.Join(base, projectID))
		if err != nil {
			continue
		}
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
			sessions = append(sessions, legacySession{
				ProjectID: projectID,
				SessionID: sessionID,
				Path:      filepath.Join(base, projectID, name),
			})
		}
	}
	if len(sessions) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, entry := range sessions {
		payload, err := os.ReadFile(entry.Path)
		if err != nil {
			continue
		}
		var sess Session
		if err := json.Unmarshal(payload, &sess); err != nil {
			continue
		}
		if strings.TrimSpace(sess.ID) == "" {
			sess.ID = entry.SessionID
		}
		if strings.TrimSpace(sess.ProjectID) == "" {
			sess.ProjectID = entry.ProjectID
		}
		if strings.TrimSpace(sess.RootID) == "" {
			sess.RootID = sess.ID
			sess.ChildIndex = 0
		}
		createdNS := sess.CreatedAt.UnixNano()
		updatedNS := sess.UpdatedAt.UnixNano()
		if createdNS == 0 {
			createdNS = time.Now().UnixNano()
		}
		if updatedNS == 0 {
			updatedNS = createdNS
		}

		_, _ = tx.Exec(
			`INSERT OR IGNORE INTO sessions(id, root_id, parent_id, child_index, project_id, work_dir, title, context_summary, created_at_ns, updated_at_ns)
			 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sess.ID, sess.RootID, nullIfEmpty(sess.ParentID), sess.ChildIndex, sess.ProjectID, sess.WorkDir, nullIfEmpty(sess.Title), nullIfEmpty(sess.ContextSummary), createdNS, updatedNS,
		)

		// Messages
		msgDir := filepath.Join(s.Root, "message", sess.ID)
		msgEnts, _ := os.ReadDir(msgDir)
		for _, me := range msgEnts {
			if me.IsDir() {
				continue
			}
			name := me.Name()
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(msgDir, name))
			if err != nil {
				continue
			}
			var m StoredMessage
			if err := json.Unmarshal(b, &m); err != nil {
				continue
			}
			if strings.TrimSpace(m.ID) == "" {
				m.ID = strings.TrimSuffix(name, ".json")
			}
			if strings.TrimSpace(m.SessionID) == "" {
				m.SessionID = sess.ID
			}
			created := m.CreatedAt.UnixNano()
			if created == 0 {
				created = time.Now().UnixNano()
			}
			_, _ = tx.Exec(
				`INSERT OR IGNORE INTO messages(id, session_id, root_id, role, mode, content, created_at_ns)
				 VALUES(?, ?, ?, ?, ?, ?, ?)`,
				m.ID, m.SessionID, sess.RootID, strings.TrimSpace(m.Role), nullIfEmpty(m.Mode), m.Content, created,
			)
		}
	}

	// Current pointers per project.
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		projectID := strings.TrimSpace(p.Name())
		if projectID == "" {
			continue
		}
		curPath := filepath.Join(base, projectID, "current")
		b, err := os.ReadFile(curPath)
		if err != nil {
			continue
		}
		sid := strings.TrimSpace(string(b))
		if sid == "" {
			continue
		}
		_, _ = tx.Exec(
			`INSERT INTO current_sessions(project_id, session_id, updated_at_ns)
			 VALUES(?, ?, ?)
			 ON CONFLICT(project_id) DO UPDATE SET session_id=excluded.session_id, updated_at_ns=excluded.updated_at_ns`,
			projectID, sid, time.Now().UnixNano(),
		)
	}

	return tx.Commit()
}

func nullIfEmpty(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
