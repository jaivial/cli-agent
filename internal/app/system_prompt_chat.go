package app

// GetChatSystemPrompt is used by the interactive TUI chat.
// It now reuses the same system prompt as `eai agent`.
func GetChatSystemPrompt(mode Mode, workDir string, verbosity string) string {
	_ = mode
	_ = verbosity
	return GetAgentSystemPrompt(workDir)
}
