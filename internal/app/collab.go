package app

import (
	"context"
	"time"
)

// CollabStore is a lightweight shared "blackboard" used to coordinate multiple
// agents/shards so they can avoid duplicating work.
//
// Implementations should be safe for concurrent use. The desktop app uses the
// SQLite-backed store so tmux worker processes can share state.
type CollabStore interface {
	// StartRun creates the run row (idempotent) and returns the effective runID.
	StartRun(ctx context.Context, runID string, sessionID string) (string, error)

	PostMessage(ctx context.Context, runID string, sessionID string, fromAgent string, toAgent string, kind string, scope string, body string) (int64, error)
	PollMessages(ctx context.Context, runID string, sinceID int64, limit int) ([]CollabMessage, int64, error)

	ClaimScope(ctx context.Context, runID string, scope string, claimedBy string, ttl time.Duration) (claimed bool, currentOwner string, err error)
	ListClaims(ctx context.Context, runID string) ([]CollabClaim, error)
	ReleaseClaim(ctx context.Context, runID string, scope string, claimedBy string) error
}

type CollabMessage struct {
	ID        int64     `json:"id"`
	RunID     string    `json:"run_id"`
	SessionID string    `json:"session_id"`
	FromAgent string    `json:"from_agent"`
	ToAgent   string    `json:"to_agent,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Scope     string    `json:"scope,omitempty"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type CollabClaim struct {
	RunID     string    `json:"run_id"`
	Scope     string    `json:"scope"`
	ClaimedBy string    `json:"claimed_by"`
	ClaimedAt time.Time `json:"claimed_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}
