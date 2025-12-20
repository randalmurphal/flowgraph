package tokens

import (
	"unicode/utf8"
)

// DefaultCharsPerToken is the default character-to-token ratio.
// Approximately 4 characters equals 1 token for English text.
const DefaultCharsPerToken = 4.0

// Counter estimates token counts for text.
type Counter interface {
	// Count estimates the number of tokens in the given text.
	Count(text string) int

	// FitsInLimit returns true if the text fits within the token limit.
	FitsInLimit(text string, limit int) bool
}

// EstimatingCounter uses a character-to-token ratio for estimation.
// Default ratio is ~4 chars per token (Claude's approximate tokenization).
type EstimatingCounter struct {
	// CharsPerToken is the average characters per token.
	// Default is 4, which works well for English text.
	CharsPerToken float64
}

// NewEstimatingCounter creates a token counter with default settings.
func NewEstimatingCounter() *EstimatingCounter {
	return &EstimatingCounter{
		CharsPerToken: DefaultCharsPerToken,
	}
}

// NewEstimatingCounterWithRatio creates a token counter with a custom ratio.
// If charsPerToken is <= 0, the default ratio (4.0) is used.
func NewEstimatingCounterWithRatio(charsPerToken float64) *EstimatingCounter {
	if charsPerToken <= 0 {
		charsPerToken = DefaultCharsPerToken
	}
	return &EstimatingCounter{
		CharsPerToken: charsPerToken,
	}
}

// Count estimates the number of tokens in the given text.
// This uses a simple heuristic of ~4 characters per token.
// Actual token counts may vary based on the specific tokenizer used.
func (c *EstimatingCounter) Count(text string) int {
	// Count runes (Unicode code points) rather than bytes for better accuracy
	runeCount := utf8.RuneCountInString(text)
	tokens := float64(runeCount) / c.CharsPerToken

	// Round to nearest integer
	return int(tokens + 0.5)
}

// FitsInLimit returns true if the text fits within the token limit.
func (c *EstimatingCounter) FitsInLimit(text string, limit int) bool {
	return c.Count(text) <= limit
}

// EstimateTokens is a convenience function using the default estimator.
func EstimateTokens(text string) int {
	return NewEstimatingCounter().Count(text)
}

// ModelLimits contains context window sizes for common models.
var ModelLimits = map[string]int{
	// Claude 4 models
	"claude-opus-4":   200000,
	"claude-sonnet-4": 200000,

	// Claude 3.5 models
	"claude-3.5-sonnet": 200000,
	"claude-3.5-haiku":  200000,

	// Claude 3 models
	"claude-3-opus":   200000,
	"claude-3-sonnet": 200000,
	"claude-3-haiku":  200000,
	"claude-haiku-3":  200000,

	// Default fallback
	"default": 100000,
}

// GetModelLimit returns the token limit for a model, or a default if not found.
func GetModelLimit(model string) int {
	if limit, ok := ModelLimits[model]; ok {
		return limit
	}
	return ModelLimits["default"]
}
