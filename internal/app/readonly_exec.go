package app

import "strings"

// ReadOnlyExecAllowed applies a conservative allowlist for exec commands used by
// read-only companions/shard workers. It blocks redirection and obvious mutation.
func ReadOnlyExecAllowed(command string) (bool, string) {
	cmd := strings.ToLower(strings.TrimSpace(command))
	if cmd == "" {
		return false, "missing command"
	}
	cmd = strings.ReplaceAll(cmd, "\r", " ")
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	cmd = strings.Join(strings.Fields(cmd), " ")

	// Block obvious mutation patterns early.
	if strings.Contains(cmd, ">") || strings.Contains(cmd, ">>") {
		return false, "blocked: shell redirection"
	}
	if strings.Contains(cmd, " tee ") || strings.HasPrefix(cmd, "tee ") || strings.Contains(cmd, "|tee") || strings.Contains(cmd, "| tee") {
		return false, "blocked: tee writes output"
	}
	if strings.Contains(cmd, "sed -i") || strings.Contains(cmd, "perl -i") {
		return false, "blocked: in-place edits"
	}
	if strings.Contains(cmd, "rm ") || strings.Contains(cmd, "mv ") || strings.Contains(cmd, "cp ") || strings.Contains(cmd, "mkdir ") || strings.Contains(cmd, "touch ") {
		return false, "blocked: filesystem mutation"
	}

	// Reuse the existing "dangerous" heuristic as a deny list.
	if execCommandNeedsApproval(command) {
		return false, "blocked: risky command"
	}

	allowedPrefixes := []string{
		"ls", "rg ", "grep ", "cat ", "sed -n", "head ", "tail ", "find ", "wc ",
		"git status", "git diff", "git log", "git show",
		"go test", "go list", "go env",
		"npm test", "pnpm test", "yarn test",
		"pytest", "python -m pytest",
	}
	for _, p := range allowedPrefixes {
		if strings.HasPrefix(cmd, p) {
			return true, ""
		}
	}
	return false, "blocked: exec restricted for read-only agents"
}
