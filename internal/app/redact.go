package app

import (
	"os"
	"strings"
)

const redactedPlaceholder = "[REDACTED]"

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// RedactSecrets replaces known secret values with a placeholder.
// Keep this conservative: we only replace provided values and well-known env var values.
func RedactSecrets(input string, secrets ...string) string {
	if strings.TrimSpace(input) == "" {
		return input
	}

	known := append([]string{}, secrets...)
	known = append(known, os.Getenv("EAI_API_KEY"))
	known = append(known, os.Getenv("MINIMAX_API_KEY"))
	known = uniqueNonEmpty(known)
	if len(known) == 0 {
		return input
	}

	out := input
	for _, s := range known {
		out = strings.ReplaceAll(out, s, redactedPlaceholder)
	}
	return out
}
