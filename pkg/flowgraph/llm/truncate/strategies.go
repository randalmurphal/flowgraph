package truncate

import "strings"

// truncateEnd removes content from the end until it fits.
func (t *Truncator) truncateEnd(text string, maxTokens int) string {
	// Reserve space for suffix
	suffixTokens := t.counter.Count(t.suffix)
	targetTokens := maxTokens - suffixTokens
	if targetTokens <= 0 {
		return t.suffix
	}

	// Binary search for the right length
	runes := []rune(text)
	low, high := 0, len(runes)

	for low < high {
		mid := (low + high + 1) / 2
		candidate := string(runes[:mid])
		if t.counter.FitsInLimit(candidate, targetTokens) {
			low = mid
		} else {
			high = mid - 1
		}
	}

	if low == 0 {
		return t.suffix
	}

	return string(runes[:low]) + t.suffix
}

// truncateMiddle removes content from the middle, keeping start and end.
func (t *Truncator) truncateMiddle(text string, maxTokens int) string {
	// Reserve space for suffix
	suffixTokens := t.counter.Count(t.suffix)
	targetTokens := maxTokens - suffixTokens
	if targetTokens <= 0 {
		return t.suffix
	}

	// Calculate how much to keep from each end
	halfTokens := targetTokens / 2

	runes := []rune(text)
	totalRunes := len(runes)

	// Find how many runes to keep at start
	startRunes := t.findRuneCountForTokens(runes, halfTokens)

	// Find how many runes to keep at end
	endStart := totalRunes - startRunes
	if endStart < startRunes {
		endStart = startRunes
	}

	// Build result
	var sb strings.Builder
	sb.WriteString(string(runes[:startRunes]))
	sb.WriteString(t.suffix)
	if endStart < totalRunes {
		sb.WriteString(string(runes[endStart:]))
	}

	return sb.String()
}

// truncateStart removes content from the start.
func (t *Truncator) truncateStart(text string, maxTokens int) string {
	// Reserve space for suffix (at the start in this case)
	suffixTokens := t.counter.Count(t.suffix)
	targetTokens := maxTokens - suffixTokens
	if targetTokens <= 0 {
		return t.suffix
	}

	runes := []rune(text)

	// Binary search from end to find where to start
	low, high := 0, len(runes)

	for low < high {
		mid := (low + high) / 2
		candidate := string(runes[mid:])
		if t.counter.FitsInLimit(candidate, targetTokens) {
			high = mid
		} else {
			low = mid + 1
		}
	}

	if low >= len(runes) {
		return t.suffix
	}

	return t.suffix + string(runes[low:])
}

// findRuneCountForTokens finds how many runes from the start fit
// in the given token count.
func (t *Truncator) findRuneCountForTokens(runes []rune, maxTokens int) int {
	low, high := 0, len(runes)

	for low < high {
		mid := (low + high + 1) / 2
		candidate := string(runes[:mid])
		if t.counter.FitsInLimit(candidate, maxTokens) {
			low = mid
		} else {
			high = mid - 1
		}
	}

	return low
}
