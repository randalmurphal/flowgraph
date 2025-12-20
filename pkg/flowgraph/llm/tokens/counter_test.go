package tokens

import (
	"strings"
	"testing"
)

func TestNewEstimatingCounter(t *testing.T) {
	c := NewEstimatingCounter()

	if c.CharsPerToken != DefaultCharsPerToken {
		t.Errorf("expected CharsPerToken %v, got %v", DefaultCharsPerToken, c.CharsPerToken)
	}
}

func TestNewEstimatingCounterWithRatio(t *testing.T) {
	tests := []struct {
		name     string
		ratio    float64
		expected float64
	}{
		{
			name:     "custom ratio",
			ratio:    3.0,
			expected: 3.0,
		},
		{
			name:     "zero ratio uses default",
			ratio:    0,
			expected: DefaultCharsPerToken,
		},
		{
			name:     "negative ratio uses default",
			ratio:    -1,
			expected: DefaultCharsPerToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewEstimatingCounterWithRatio(tt.ratio)
			if c.CharsPerToken != tt.expected {
				t.Errorf("expected CharsPerToken %v, got %v", tt.expected, c.CharsPerToken)
			}
		})
	}
}

func TestEstimatingCounter_Count(t *testing.T) {
	c := NewEstimatingCounter()

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string",
			text:     "",
			expected: 0,
		},
		{
			name:     "single character",
			text:     "a",
			expected: 0, // 1/4 = 0.25 rounds to 0
		},
		{
			name:     "four characters",
			text:     "test",
			expected: 1, // 4/4 = 1
		},
		{
			name:     "eight characters",
			text:     "testtest",
			expected: 2, // 8/4 = 2
		},
		{
			name:     "hello world",
			text:     "Hello World",
			expected: 3, // 11/4 = 2.75 rounds to 3
		},
		{
			name:     "unicode characters",
			text:     "Hello, World!",
			expected: 3, // 13 runes / 4 = 3.25 rounds to 3
		},
		{
			name:     "emoji",
			text:     "Hello!",
			expected: 2, // 7 runes (includes emoji as 1) / 4 = 1.75 rounds to 2
		},
		{
			name:     "longer text",
			text:     "This is a longer piece of text that should estimate to more tokens.",
			expected: 17, // 68 chars / 4 = 17
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Count(tt.text)
			if result != tt.expected {
				t.Errorf("Count(%q) = %d, expected %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestEstimatingCounter_Count_CustomRatio(t *testing.T) {
	// 3 chars per token is tighter estimate
	c := NewEstimatingCounterWithRatio(3.0)

	text := "Hello World" // 11 chars
	expected := 4         // 11/3 = 3.67 rounds to 4

	result := c.Count(text)
	if result != expected {
		t.Errorf("Count(%q) with ratio 3.0 = %d, expected %d", text, result, expected)
	}
}

func TestEstimatingCounter_FitsInLimit(t *testing.T) {
	c := NewEstimatingCounter()

	tests := []struct {
		name     string
		text     string
		limit    int
		expected bool
	}{
		{
			name:     "empty fits any limit",
			text:     "",
			limit:    1,
			expected: true,
		},
		{
			name:     "fits exactly",
			text:     "test",
			limit:    1,
			expected: true,
		},
		{
			name:     "fits with room",
			text:     "test",
			limit:    10,
			expected: true,
		},
		{
			name:     "does not fit",
			text:     "test test test test test", // ~6 tokens
			limit:    3,
			expected: false,
		},
		{
			name:     "zero limit",
			text:     "hello",
			limit:    0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.FitsInLimit(tt.text, tt.limit)
			if result != tt.expected {
				t.Errorf("FitsInLimit(%q, %d) = %v, expected %v",
					tt.text, tt.limit, result, tt.expected)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	// Convenience function should work the same as NewEstimatingCounter().Count()
	text := "Hello World"
	expected := NewEstimatingCounter().Count(text)

	result := EstimateTokens(text)
	if result != expected {
		t.Errorf("EstimateTokens(%q) = %d, expected %d", text, result, expected)
	}
}

func TestEstimateTokens_LargeText(t *testing.T) {
	// Create a large text
	text := strings.Repeat("Hello World ", 1000)

	result := EstimateTokens(text)
	// 12 chars * 1000 = 12000 chars, / 4 = 3000 tokens
	if result < 2900 || result > 3100 {
		t.Errorf("EstimateTokens for large text = %d, expected ~3000", result)
	}
}

func TestCounter_Interface(t *testing.T) {
	// Verify EstimatingCounter implements Counter interface
	var _ Counter = (*EstimatingCounter)(nil)
}

func TestGetModelLimit(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected int
	}{
		{
			name:     "claude opus 4",
			model:    "claude-opus-4",
			expected: 200000,
		},
		{
			name:     "claude sonnet 4",
			model:    "claude-sonnet-4",
			expected: 200000,
		},
		{
			name:     "claude 3.5 sonnet",
			model:    "claude-3.5-sonnet",
			expected: 200000,
		},
		{
			name:     "claude 3 opus",
			model:    "claude-3-opus",
			expected: 200000,
		},
		{
			name:     "claude haiku 3",
			model:    "claude-haiku-3",
			expected: 200000,
		},
		{
			name:     "unknown model gets default",
			model:    "gpt-4",
			expected: 100000,
		},
		{
			name:     "empty model gets default",
			model:    "",
			expected: 100000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetModelLimit(tt.model)
			if result != tt.expected {
				t.Errorf("GetModelLimit(%q) = %d, expected %d", tt.model, result, tt.expected)
			}
		})
	}
}

func TestModelLimits_HasDefault(t *testing.T) {
	_, ok := ModelLimits["default"]
	if !ok {
		t.Error("ModelLimits should have a 'default' entry")
	}
}

func TestModelLimits_AllPositive(t *testing.T) {
	for model, limit := range ModelLimits {
		if limit <= 0 {
			t.Errorf("ModelLimits[%q] = %d, should be positive", model, limit)
		}
	}
}

func BenchmarkEstimatingCounter_Count(b *testing.B) {
	c := NewEstimatingCounter()
	text := strings.Repeat("Hello World ", 100)

	b.ResetTimer()
	for range b.N {
		c.Count(text)
	}
}

func BenchmarkEstimateTokens(b *testing.B) {
	text := strings.Repeat("Hello World ", 100)

	b.ResetTimer()
	for range b.N {
		EstimateTokens(text)
	}
}
