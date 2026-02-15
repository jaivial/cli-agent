package app

// SessionStore persists sessions/messages and provides lightweight session helpers
// for the interactive TUI and desktop app.
//
// Implementations must preserve message ordering by CreatedAt, and must NOT delete
// prior messages when session compaction creates child sessions.
type SessionStore interface {
	LoadOrCreateCurrentSession(workDir string) (*Session, []StoredMessage, error)
	CreateSession(workDir string) (*Session, error)
	SetCurrentSession(workDir string, sessionID string) error
	LoadSessionForWorkDir(workDir, sessionID string) (*Session, []StoredMessage, error)
	SaveSession(sess *Session) error
	TouchSession(workDir string, sessionID string) error

	AppendMessage(msg StoredMessage) error
	LoadMessages(sessionID string) ([]StoredMessage, error)
	ClearSessionMessages(sessionID string) error
	// DeleteSessionChain deletes the entire root conversation containing sessionID,
	// including all chained (compacted) child sessions and messages, for the given workDir.
	DeleteSessionChain(workDir string, sessionID string) error

	ListSessionsForWorkDir(workDir string, limit int) ([]SessionSummary, error)

	SavePromptHistory(workDir string, entries []string) error
	LoadPromptHistory(workDir string) ([]string, error)

	// Compaction/session-chaining helpers.
	CreateChildSession(workDir string, parentSessionID string, summary string) (*Session, error)
	RootIDForSession(sessionID string) (string, error)
}
