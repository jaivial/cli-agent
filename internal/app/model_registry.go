package app

import "strings"

// LookupContextWindowTokens returns the known context window size (in tokens) for a model.
//
// This is used for session compaction thresholds. Callers should still allow an explicit
// override via config/env because providers may change limits.
func LookupContextWindowTokens(model string) (int, bool) {
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return 0, false
	}

	// GLM family.
	if strings.Contains(m, "glm-4.7") || strings.HasPrefix(m, "glm-4") {
		return 200_000, true
	}
	if strings.Contains(m, "glm-5") || strings.HasPrefix(m, "glm5") || strings.HasPrefix(m, "glm-5") {
		return 200_000, true
	}

	// MiniMax M2.5 (coding plan).
	if strings.Contains(m, "minimax") && strings.Contains(m, "m2.5") {
		return 205_000, true
	}
	if strings.Contains(m, "codex-minimax-m2.5") {
		return 205_000, true
	}

	return 0, false
}
