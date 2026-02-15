package app

import (
	"context"
	"os"
	"strings"
	"time"
	"unicode"
)

func splitToolSessionContextWrapper(input string) (prefix, request, suffix string, ok bool) {
	if input == "" {
		return "", "", "", false
	}
	lower := strings.ToLower(input)
	if !strings.HasPrefix(lower, "current request:\n") {
		return "", "", "", false
	}

	// Preserve the original casing/spacing of the wrapper prefix.
	prefixLen := len("Current request:\n")
	if len(input) < prefixLen {
		return "", "", "", false
	}
	prefix = input[:prefixLen]
	rest := input[prefixLen:]

	needleSummary := "\n\nSession summary:\n"
	needleContext := "\n\nConversation context (most recent messages):\n"

	cut := len(rest)
	if idx := strings.Index(rest, needleSummary); idx >= 0 && idx < cut {
		cut = idx
	}
	if idx := strings.Index(rest, needleContext); idx >= 0 && idx < cut {
		cut = idx
	}

	request = rest[:cut]
	suffix = rest[cut:]
	return prefix, request, suffix, true
}

func autoTranslateToEnglishEnabled() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("EAI_AUTO_TRANSLATE_TO_ENGLISH")))
	switch raw {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func likelyNeedsEnglishTranslation(input string) bool {
	s := strings.TrimSpace(input)
	if s == "" {
		return false
	}

	// If the user wrote non-ASCII text, we almost certainly want translation.
	for _, r := range s {
		// Be conservative: non-ASCII punctuation/symbols show up in English too (em dashes,
		// curly quotes, emojis). Non-ASCII letters are a much stronger signal.
		if r > unicode.MaxASCII && unicode.IsLetter(r) {
			return true
		}
	}

	// Very small heuristic: if the request contains common English words, assume English.
	// Otherwise, translate. This avoids adding per-language keyword tables in Go while
	// still keeping most English prompts fast.
	words := strings.Fields(strings.ToLower(s))
	if len(words) == 0 {
		return false
	}

	commonEnglish := map[string]bool{
		"the": true, "a": true, "an": true, "to": true, "and": true, "or": true,
		"in": true, "on": true, "for": true, "with": true, "please": true,
		"hi": true, "hello": true, "hey": true, "yo": true, "sup": true,
		"thanks": true, "thank": true, "thx": true, "ty": true,
		"ok": true, "okay": true,
		"create": true, "make": true, "build": true, "generate": true, "write": true,
		"list": true, "show": true, "run": true, "execute": true, "test": true,
		"fix": true, "debug": true, "update": true, "delete": true, "remove": true,
		"move": true, "rename": true, "install": true, "help": true, "explain": true,
		"summarize": true, "summarise": true,
		"analyze": true, "analyse": true, "review": true, "inspect": true,
		"repo": true, "repository": true, "project": true,
		"compose": true, "report": true,
		"file": true, "files": true, "folder": true, "directory": true,
	}

	hits := 0
	for _, w := range words {
		w = strings.Trim(w, ".,:;!?()[]{}\"'")
		if commonEnglish[w] {
			hits++
		}
	}
	if hits >= 2 {
		return false
	}
	// If it's a single short English-ish command, don't translate.
	if hits == 1 && len(words) <= 5 {
		return false
	}
	return true
}

func translateToEnglish(ctx context.Context, client *MinimaxClient, input string) (string, bool, error) {
	if client == nil {
		return input, false, nil
	}
	if !autoTranslateToEnglishEnabled() {
		return input, false, nil
	}
	// Avoid breaking tests/mock flows: mock completions are not a translation engine.
	if client.APIKey == "mock" || client.BaseURL == "mock://" {
		return input, false, nil
	}
	// If there's no API key, translation can't run anyway; keep existing behavior.
	if strings.TrimSpace(client.APIKey) == "" {
		return input, false, nil
	}

	original := input
	payload := input
	reassemble := func(out string) string { return out }
	if prefix, request, suffix, ok := splitToolSessionContextWrapper(input); ok {
		payload = request
		reassemble = func(out string) string {
			return prefix + out + suffix
		}
	}

	if !likelyNeedsEnglishTranslation(payload) {
		return input, false, nil
	}

	// Keep this fast; translation is just a preprocessing step.
	translateCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	prompt := strings.Builder{}
	prompt.WriteString("[SYSTEM]\n")
	prompt.WriteString("You are a translation engine.\n")
	prompt.WriteString("Detect the language of the user's request.\n")
	prompt.WriteString("If it is already English, return it verbatim.\n")
	prompt.WriteString("If it is not English, translate it to English.\n")
	prompt.WriteString("Preserve code blocks, file paths, commands, flags, and proper nouns exactly.\n")
	prompt.WriteString("Return ONLY the translated text. No quotes. No commentary.\n\n")
	prompt.WriteString("[USER]\n")
	prompt.WriteString(payload)

	out, err := client.Complete(translateCtx, prompt.String())
	if err != nil {
		return original, false, err
	}
	out = strings.TrimSpace(out)
	out = strings.Trim(out, "\"")
	out = strings.TrimSpace(out)
	if out == "" {
		return original, false, nil
	}
	if out == strings.TrimSpace(payload) {
		return original, false, nil
	}
	return reassemble(out), true, nil
}
