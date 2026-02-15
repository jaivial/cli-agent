package app

import "time"

type Session struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	WorkDir   string `json:"work_dir"`
	Title     string `json:"title,omitempty"`
	// ContextSummary stores compacted session context used when chat history grows
	// beyond model limits.
	ContextSummary string `json:"context_summary,omitempty"`

	// RootID groups a chain of sessions (root + children created by compaction).
	RootID string `json:"root_id,omitempty"`
	// ParentID points to the previous session in the chain.
	ParentID string `json:"parent_id,omitempty"`
	// ChildIndex is 0 for the root session and increments by 1 per compaction.
	ChildIndex int `json:"child_index,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
