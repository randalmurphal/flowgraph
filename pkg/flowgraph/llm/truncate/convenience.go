package truncate

import (
	"strings"
	"unicode/utf8"
)

// ToTokens truncates text to fit within the specified token limit.
// Uses end truncation with the default estimating counter.
func ToTokens(text string, maxTokens int) string {
	result, _ := NewFromEnd().Truncate(text, maxTokens)
	return result
}

// ToLines truncates text to a maximum number of lines.
func ToLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return ""
	}

	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}

	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

// ToLength truncates text to a maximum character length.
// Properly handles UTF-8 by counting runes, not bytes.
func ToLength(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	runeCount := utf8.RuneCountInString(text)
	if runeCount <= maxLen {
		return text
	}

	runes := []rune(text)
	if maxLen < 3 {
		return string(runes[:maxLen])
	}

	return string(runes[:maxLen-3]) + "..."
}

// Smart attempts to truncate at word or sentence boundaries.
// Falls back to hard truncation if no good break point is found.
func Smart(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	runeCount := utf8.RuneCountInString(text)
	if runeCount <= maxLen {
		return text
	}

	// Find a good break point near maxLen
	runes := []rune(text)
	breakPoint := maxLen - 3 // Reserve for "..."

	// Look for a sentence boundary (. ! ?)
	for i := breakPoint; i > maxLen/2; i-- {
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			return string(runes[:i+1])
		}
	}

	// Look for a word boundary (space or newline)
	for i := breakPoint; i > maxLen/2; i-- {
		if runes[i] == ' ' || runes[i] == '\n' {
			return string(runes[:i]) + "..."
		}
	}

	// Fall back to hard truncation
	return string(runes[:breakPoint]) + "..."
}
