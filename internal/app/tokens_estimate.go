package app

import "unicode/utf8"

// EstimateTokens returns a conservative estimate of token count for a piece of text.
//
// We intentionally over-estimate a bit so compaction triggers early rather than late.
// This is not a tokenizer; it is only used for safety thresholds.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Most BPE tokenizers end up around ~3-4 chars/token for English-ish text.
	// Using bytes/3 is a decent conservative bound, and we also bound by runes/2
	// to avoid undercounting for mostly-ASCII short tokens.
	b := len(text)
	r := utf8.RuneCountInString(text)
	byBytes := b / 3
	byRunes := r / 2
	if byBytes < byRunes {
		return byRunes
	}
	return byBytes
}
