package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func normalizeCollabKind(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "":
		return ""
	case "announce", "status", "question", "answer", "handoff", "decision":
		return kind
	default:
		// Keep unknown kinds, but normalize spacing/case.
		return kind
	}
}

func normalizeCollabScope(scope string) string {
	return strings.TrimSpace(scope)
}

func (s *SQLiteSessionStore) StartRun(ctx context.Context, runID string, sessionID string) (string, error) {
	if s == nil {
		return "", errors.New("collab store unavailable")
	}
	runID = strings.TrimSpace(runID)
	sessionID = strings.TrimSpace(sessionID)
	if runID == "" {
		runID = fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	db, err := s.dbConn()
	if err != nil {
		return "", err
	}
	rootID := runID
	if sessionID != "" {
		if rid, err := s.RootIDForSession(sessionID); err == nil && strings.TrimSpace(rid) != "" {
			rootID = strings.TrimSpace(rid)
		}
	}
	now := time.Now().UnixNano()
	status := "active"
	_, err = db.ExecContext(
		ctx,
		`INSERT INTO collab_runs(run_id, session_id, root_id, status, created_at_ns, updated_at_ns)
		 VALUES(?, ?, ?, ?, ?, ?)
		 ON CONFLICT(run_id) DO UPDATE SET updated_at_ns=excluded.updated_at_ns, status=excluded.status`,
		runID, nullIfEmpty(sessionID), rootID, status, now, now,
	)
	if err != nil {
		return "", err
	}
	return runID, nil
}

func (s *SQLiteSessionStore) touchRun(ctx context.Context, runID string) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return
	}
	db, err := s.dbConn()
	if err != nil {
		return
	}
	_, _ = db.ExecContext(ctx, `UPDATE collab_runs SET updated_at_ns = ? WHERE run_id = ?`, time.Now().UnixNano(), runID)
}

func (s *SQLiteSessionStore) PostMessage(ctx context.Context, runID string, sessionID string, fromAgent string, toAgent string, kind string, scope string, body string) (int64, error) {
	if s == nil {
		return 0, errors.New("collab store unavailable")
	}
	runID = strings.TrimSpace(runID)
	body = strings.TrimSpace(body)
	fromAgent = strings.TrimSpace(fromAgent)
	if runID == "" {
		return 0, errors.New("missing run_id")
	}
	if fromAgent == "" {
		return 0, errors.New("missing from_agent")
	}
	if body == "" {
		return 0, errors.New("missing body")
	}

	db, err := s.dbConn()
	if err != nil {
		return 0, err
	}

	kind = normalizeCollabKind(kind)
	scope = normalizeCollabScope(scope)

	// Keep the shared chat lightweight. The detailed, streaming reasoning belongs in
	// per-pane output; collab is for decisions and handoffs.
	const maxBody = 2200
	if len(body) > maxBody {
		body = strings.TrimSpace(body[:maxBody-3]) + "..."
	}

	// Apply basic role rules to prevent cross-talk loops:
	// - Only the coordinator should broadcast ANNOUNCE/DECISION.
	if (kind == "announce" || kind == "decision") && !strings.HasPrefix(fromAgent, "Coordinator") {
		return 0, fmt.Errorf("collab: %s messages are coordinator-only", kind)
	}

	// For most message kinds, require that the sender owns the scope claim. This avoids
	// multiple agents "arguing" in the same channel and redoing work.
	if kind != "" && kind != "announce" && kind != "status" && kind != "question" {
		if scope == "" {
			return 0, errors.New("collab: scope is required for this message kind")
		}
		var owner string
		var expires sql.NullInt64
		err := db.QueryRowContext(ctx, `SELECT claimed_by, expires_at_ns FROM collab_claims WHERE run_id = ? AND scope = ?`, runID, scope).
			Scan(&owner, &expires)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, errors.New("collab: scope is unclaimed; claim it before posting")
			}
			return 0, err
		}
		owner = strings.TrimSpace(owner)
		nowNS := time.Now().UnixNano()
		if expires.Valid && expires.Int64 > 0 && expires.Int64 < nowNS {
			return 0, errors.New("collab: scope claim expired; claim it before posting")
		}
		if owner == "" || owner != fromAgent {
			return 0, fmt.Errorf("collab: scope is claimed by %q (not %q)", owner, fromAgent)
		}
	}

	now := time.Now().UnixNano()

	// Deduplicate fast repeats (common when models "double send" or retry).
	{
		const dedupeWindow = 10 * time.Second
		since := time.Now().Add(-dedupeWindow).UnixNano()
		var existingID int64
		if err := db.QueryRowContext(
			ctx,
			`SELECT id FROM collab_messages
			 WHERE run_id = ? AND from_agent = ? AND IFNULL(kind,'') = ? AND IFNULL(scope,'') = ? AND body = ? AND created_at_ns >= ?
			 ORDER BY id DESC LIMIT 1`,
			runID, fromAgent, kind, scope, body, since,
		).Scan(&existingID); err == nil && existingID > 0 {
			return existingID, nil
		}
	}

	res, err := db.ExecContext(
		ctx,
		`INSERT INTO collab_messages(run_id, session_id, from_agent, to_agent, kind, scope, body, created_at_ns)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		nullIfEmpty(strings.TrimSpace(sessionID)),
		fromAgent,
		nullIfEmpty(strings.TrimSpace(toAgent)),
		nullIfEmpty(strings.TrimSpace(kind)),
		nullIfEmpty(strings.TrimSpace(scope)),
		body,
		now,
	)
	if err != nil {
		return 0, err
	}
	s.touchRun(ctx, runID)
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *SQLiteSessionStore) PollMessages(ctx context.Context, runID string, sinceID int64, limit int) ([]CollabMessage, int64, error) {
	if s == nil {
		return nil, sinceID, errors.New("collab store unavailable")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, sinceID, errors.New("missing run_id")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	db, err := s.dbConn()
	if err != nil {
		return nil, sinceID, err
	}

	rows, err := db.QueryContext(
		ctx,
		`SELECT id, run_id, session_id, from_agent, to_agent, kind, scope, body, created_at_ns
		 FROM collab_messages
		 WHERE run_id = ? AND id > ?
		 ORDER BY id ASC
		 LIMIT ?`,
		runID, sinceID, limit,
	)
	if err != nil {
		return nil, sinceID, err
	}
	defer rows.Close()

	out := make([]CollabMessage, 0, minInt(limit, 64))
	last := sinceID
	for rows.Next() {
		var m CollabMessage
		var session sql.NullString
		var to sql.NullString
		var kind sql.NullString
		var scope sql.NullString
		var createdNS int64
		if err := rows.Scan(&m.ID, &m.RunID, &session, &m.FromAgent, &to, &kind, &scope, &m.Body, &createdNS); err != nil {
			continue
		}
		if session.Valid {
			m.SessionID = session.String
		}
		if to.Valid {
			m.ToAgent = to.String
		}
		if kind.Valid {
			m.Kind = kind.String
		}
		if scope.Valid {
			m.Scope = scope.String
		}
		m.CreatedAt = time.Unix(0, createdNS)
		out = append(out, m)
		if m.ID > last {
			last = m.ID
		}
	}
	return out, last, nil
}

func (s *SQLiteSessionStore) ClaimScope(ctx context.Context, runID string, scope string, claimedBy string, ttl time.Duration) (bool, string, error) {
	if s == nil {
		return false, "", errors.New("collab store unavailable")
	}
	runID = strings.TrimSpace(runID)
	scope = strings.TrimSpace(scope)
	claimedBy = strings.TrimSpace(claimedBy)
	if runID == "" {
		return false, "", errors.New("missing run_id")
	}
	if scope == "" {
		return false, "", errors.New("missing scope")
	}
	if claimedBy == "" {
		return false, "", errors.New("missing claimed_by")
	}
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	now := time.Now()
	nowNS := now.UnixNano()
	expiresNS := now.Add(ttl).UnixNano()

	db, err := s.dbConn()
	if err != nil {
		return false, "", err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, "", err
	}
	defer func() { _ = tx.Rollback() }()

	// Insert claim if missing; otherwise steal it only if expired.
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO collab_claims(run_id, scope, claimed_by, claimed_at_ns, expires_at_ns)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(run_id, scope) DO UPDATE SET
		   claimed_by=excluded.claimed_by,
		   claimed_at_ns=excluded.claimed_at_ns,
		   expires_at_ns=excluded.expires_at_ns
		 WHERE collab_claims.claimed_by = excluded.claimed_by OR (collab_claims.expires_at_ns IS NOT NULL AND collab_claims.expires_at_ns < ?)`,
		runID, scope, claimedBy, nowNS, expiresNS, nowNS,
	)
	if err != nil {
		return false, "", err
	}

	var owner string
	var expires sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT claimed_by, expires_at_ns FROM collab_claims WHERE run_id = ? AND scope = ?`, runID, scope).
		Scan(&owner, &expires); err != nil {
		return false, "", err
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		owner = claimedBy
	}
	claimed := owner == claimedBy

	if err := tx.Commit(); err != nil {
		return false, "", err
	}
	s.touchRun(ctx, runID)
	return claimed, owner, nil
}

func (s *SQLiteSessionStore) ListClaims(ctx context.Context, runID string) ([]CollabClaim, error) {
	if s == nil {
		return nil, errors.New("collab store unavailable")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, errors.New("missing run_id")
	}
	db, err := s.dbConn()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(
		ctx,
		`SELECT run_id, scope, claimed_by, claimed_at_ns, expires_at_ns
		 FROM collab_claims
		 WHERE run_id = ?
		 ORDER BY claimed_at_ns ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]CollabClaim, 0, 32)
	for rows.Next() {
		var c CollabClaim
		var claimedNS int64
		var expires sql.NullInt64
		if err := rows.Scan(&c.RunID, &c.Scope, &c.ClaimedBy, &claimedNS, &expires); err != nil {
			continue
		}
		c.ClaimedAt = time.Unix(0, claimedNS)
		if expires.Valid && expires.Int64 > 0 {
			c.ExpiresAt = time.Unix(0, expires.Int64)
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *SQLiteSessionStore) ReleaseClaim(ctx context.Context, runID string, scope string, claimedBy string) error {
	if s == nil {
		return errors.New("collab store unavailable")
	}
	runID = strings.TrimSpace(runID)
	scope = strings.TrimSpace(scope)
	claimedBy = strings.TrimSpace(claimedBy)
	if runID == "" {
		return errors.New("missing run_id")
	}
	if scope == "" {
		return errors.New("missing scope")
	}
	if claimedBy == "" {
		return errors.New("missing claimed_by")
	}
	db, err := s.dbConn()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `DELETE FROM collab_claims WHERE run_id = ? AND scope = ? AND claimed_by = ?`, runID, scope, claimedBy)
	if err == nil {
		s.touchRun(ctx, runID)
	}
	return err
}
