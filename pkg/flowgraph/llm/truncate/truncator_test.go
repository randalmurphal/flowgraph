package truncate

import (
	"strings"
	"testing"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/llm/tokens"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		strategy       Strategy
		expectedSuffix string
	}{
		{
			name:           "FromEnd strategy",
			strategy:       FromEnd,
			expectedSuffix: DefaultEndSuffix,
		},
		{
			name:           "FromMiddle strategy",
			strategy:       FromMiddle,
			expectedSuffix: DefaultMiddleSuffix,
		},
		{
			name:           "FromStart strategy",
			strategy:       FromStart,
			expectedSuffix: DefaultStartSuffix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := New(tt.strategy)
			if tr.Strategy() != tt.strategy {
				t.Errorf("Strategy() = %v, expected %v", tr.Strategy(), tt.strategy)
			}
			if tr.Suffix() != tt.expectedSuffix {
				t.Errorf("Suffix() = %q, expected %q", tr.Suffix(), tt.expectedSuffix)
			}
		})
	}
}

func TestNewFromEnd(t *testing.T) {
	tr := NewFromEnd()
	if tr.Strategy() != FromEnd {
		t.Errorf("Strategy() = %v, expected FromEnd", tr.Strategy())
	}
	if tr.Suffix() != DefaultEndSuffix {
		t.Errorf("Suffix() = %q, expected %q", tr.Suffix(), DefaultEndSuffix)
	}
}

func TestNewFromMiddle(t *testing.T) {
	tr := NewFromMiddle()
	if tr.Strategy() != FromMiddle {
		t.Errorf("Strategy() = %v, expected FromMiddle", tr.Strategy())
	}
	if tr.Suffix() != DefaultMiddleSuffix {
		t.Errorf("Suffix() = %q, expected %q", tr.Suffix(), DefaultMiddleSuffix)
	}
}

func TestNewFromStart(t *testing.T) {
	tr := NewFromStart()
	if tr.Strategy() != FromStart {
		t.Errorf("Strategy() = %v, expected FromStart", tr.Strategy())
	}
	if tr.Suffix() != DefaultStartSuffix {
		t.Errorf("Suffix() = %q, expected %q", tr.Suffix(), DefaultStartSuffix)
	}
}

func TestTruncator_WithCounter(t *testing.T) {
	customCounter := tokens.NewEstimatingCounterWithRatio(2.0)
	tr := NewFromEnd().WithCounter(customCounter)

	// With 2.0 ratio, 8 chars = 4 tokens
	// With 4.0 ratio, 8 chars = 2 tokens
	text := strings.Repeat("a", 20) // 20 chars = 10 tokens with 2.0 ratio, 5 with 4.0

	result, truncated := tr.Truncate(text, 6)
	if !truncated {
		t.Error("expected truncation with custom counter")
	}
	if len(result) >= len(text) {
		t.Error("result should be shorter than original")
	}
}

func TestTruncator_WithSuffix(t *testing.T) {
	customSuffix := "[...]"
	tr := NewFromEnd().WithSuffix(customSuffix)

	if tr.Suffix() != customSuffix {
		t.Errorf("Suffix() = %q, expected %q", tr.Suffix(), customSuffix)
	}

	text := strings.Repeat("a", 100)
	result, _ := tr.Truncate(text, 10)

	if !strings.HasSuffix(result, customSuffix) {
		t.Errorf("result should end with custom suffix, got: %q", result)
	}
}

func TestTruncator_Truncate_NoTruncationNeeded(t *testing.T) {
	tr := NewFromEnd()

	text := "short text"
	result, truncated := tr.Truncate(text, 100)

	if result != text {
		t.Errorf("result = %q, expected %q", result, text)
	}
	if truncated {
		t.Error("expected no truncation")
	}
}

func TestTruncator_TruncateEnd(t *testing.T) {
	tr := NewFromEnd()

	// Create text that will need truncation
	// With 4 chars per token, 100 chars = 25 tokens
	text := strings.Repeat("a", 100)
	result, truncated := tr.Truncate(text, 10)

	if !truncated {
		t.Error("expected truncation")
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected suffix ..., got: %q", result)
	}
	// Should be significantly shorter than original
	if len(result) >= len(text) {
		t.Error("result should be shorter than original")
	}
}

func TestTruncator_TruncateMiddle(t *testing.T) {
	tr := NewFromMiddle()

	// Create text that will need truncation
	text := strings.Repeat("a", 50) + strings.Repeat("b", 50)
	result, truncated := tr.Truncate(text, 10)

	if !truncated {
		t.Error("expected truncation")
	}
	if !strings.Contains(result, "[content truncated]") {
		t.Errorf("expected middle suffix, got: %q", result)
	}
	// Should have some a's at start
	if !strings.HasPrefix(result, "aaa") {
		t.Errorf("expected to start with 'aaa', got: %q", result)
	}
}

func TestTruncator_TruncateStart(t *testing.T) {
	tr := NewFromStart()

	// Create text that will need truncation
	text := strings.Repeat("a", 100)
	result, truncated := tr.Truncate(text, 10)

	if !truncated {
		t.Error("expected truncation")
	}
	if !strings.HasPrefix(result, "...") {
		t.Errorf("expected prefix ..., got: %q", result)
	}
	// Should have content from the end
	if !strings.HasSuffix(result, "a") {
		t.Errorf("expected to end with 'a', got: %q", result)
	}
}

func TestTruncator_VerySmallLimit(t *testing.T) {
	tr := NewFromEnd()

	text := strings.Repeat("a", 100)
	result, truncated := tr.Truncate(text, 0)

	if !truncated {
		t.Error("expected truncation")
	}
	if result != "..." {
		t.Errorf("expected just suffix, got: %q", result)
	}
}

func TestTruncator_DefaultStrategy(t *testing.T) {
	// Test that an invalid strategy defaults to end truncation
	tr := &Truncator{
		counter:  tokens.NewEstimatingCounter(),
		strategy: Strategy(99), // Invalid
		suffix:   "...",
	}

	text := strings.Repeat("a", 100)
	result, truncated := tr.Truncate(text, 5)

	if !truncated {
		t.Error("expected truncation")
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected suffix ..., got: %q", result)
	}
}

func TestToTokens(t *testing.T) {
	// Convenience function
	text := strings.Repeat("x", 100)
	result := ToTokens(text, 10)

	if len(result) >= len(text) {
		t.Error("result should be shorter than original")
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected suffix ..., got: %q", result)
	}
}

func TestToLines(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLines int
		expected string
	}{
		{
			name:     "fewer lines than max",
			text:     "line1\nline2",
			maxLines: 5,
			expected: "line1\nline2",
		},
		{
			name:     "more lines than max",
			text:     "line1\nline2\nline3\nline4\nline5",
			maxLines: 3,
			expected: "line1\nline2\nline3\n...",
		},
		{
			name:     "zero max lines",
			text:     "line1\nline2",
			maxLines: 0,
			expected: "",
		},
		{
			name:     "negative max lines",
			text:     "line1\nline2",
			maxLines: -1,
			expected: "",
		},
		{
			name:     "exact lines",
			text:     "line1\nline2\nline3",
			maxLines: 3,
			expected: "line1\nline2\nline3",
		},
		{
			name:     "single line",
			text:     "single line",
			maxLines: 1,
			expected: "single line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToLines(tt.text, tt.maxLines)
			if result != tt.expected {
				t.Errorf("ToLines() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestToLength(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{
			name:     "shorter than max",
			text:     "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "longer than max",
			text:     "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "zero max length",
			text:     "hello",
			maxLen:   0,
			expected: "",
		},
		{
			name:     "negative max length",
			text:     "hello",
			maxLen:   -1,
			expected: "",
		},
		{
			name:     "very small max length",
			text:     "hello",
			maxLen:   2,
			expected: "he",
		},
		{
			name:     "exact length",
			text:     "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "max length 3",
			text:     "hello world",
			maxLen:   3,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToLength(tt.text, tt.maxLen)
			if result != tt.expected {
				t.Errorf("ToLength() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestToLength_Unicode(t *testing.T) {
	// Test with unicode characters
	text := "Hello, World!" // 13 runes including emoji
	result := ToLength(text, 10)

	// Should truncate properly counting runes
	if len([]rune(result)) > 10 {
		t.Errorf("result has %d runes, expected <= 10", len([]rune(result)))
	}
}

func TestSmart(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		maxLen     int
		shouldEnd  string
		shouldHave string
	}{
		{
			name:       "shorter than max",
			text:       "hello",
			maxLen:     10,
			shouldEnd:  "hello",
			shouldHave: "hello",
		},
		{
			name:       "truncate at sentence",
			text:       "First sentence. Second sentence starts here.",
			maxLen:     20,
			shouldEnd:  ".",
			shouldHave: "First sentence.",
		},
		{
			name:       "truncate at word boundary",
			text:       "The quick brown fox jumps over the lazy dog",
			maxLen:     20,
			shouldEnd:  "...",
			shouldHave: "The quick",
		},
		{
			name:       "zero max length",
			text:       "hello",
			maxLen:     0,
			shouldEnd:  "",
			shouldHave: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Smart(tt.text, tt.maxLen)
			if tt.shouldEnd != "" {
				if !strings.HasSuffix(result, tt.shouldEnd) {
					t.Errorf("expected to end with %q, got %q", tt.shouldEnd, result)
				}
			}
			if tt.shouldHave != "" {
				if !strings.Contains(result, tt.shouldHave) {
					t.Errorf("expected to contain %q, got %q", tt.shouldHave, result)
				}
			}
		})
	}
}

func TestSmart_SentenceBoundary(t *testing.T) {
	// Test that we prefer sentence boundaries
	text := "Hello! How are you? I am fine."
	result := Smart(text, 25)

	// Should truncate at "you?" or nearby sentence end
	if !strings.HasSuffix(result, "?") && !strings.HasSuffix(result, "!") {
		t.Errorf("expected to end at sentence boundary, got: %s", result)
	}
}

func TestSmart_WordBoundary(t *testing.T) {
	// No sentence boundaries, should truncate at word
	text := "word1 word2 word3 word4 word5"
	result := Smart(text, 15)

	// Should truncate at a space with "..."
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected to end with ..., got: %s", result)
	}
	// Should have truncated at a word boundary
	if !strings.Contains(result, "word") {
		t.Errorf("expected to contain word, got: %s", result)
	}
}

func TestSmart_HardTruncation(t *testing.T) {
	// No good break points
	text := strings.Repeat("x", 100)
	result := Smart(text, 20)

	// Should fall back to hard truncation
	if !strings.HasSuffix(result, "...") {
		t.Errorf("expected to end with ..., got: %s", result)
	}
	if len([]rune(result)) > 20 {
		t.Errorf("result is too long: %d runes", len([]rune(result)))
	}
}

// MockCounter is a test counter that returns a fixed token count.
type MockCounter struct {
	TokensPerChar float64
}

func (m *MockCounter) Count(text string) int {
	return int(float64(len([]rune(text))) * m.TokensPerChar)
}

func (m *MockCounter) FitsInLimit(text string, limit int) bool {
	return m.Count(text) <= limit
}

func TestTruncator_WithMockCounter(t *testing.T) {
	// Use a mock counter for deterministic tests
	mock := &MockCounter{TokensPerChar: 1.0} // 1 token per char

	tr := NewFromEnd().WithCounter(mock)

	text := strings.Repeat("a", 20) // 20 tokens with mock
	result, truncated := tr.Truncate(text, 10)

	if !truncated {
		t.Error("expected truncation")
	}
	// With 1 token per char, 10 tokens = 10 chars, minus 3 for "..."
	// So we should get ~7 chars + "..."
	expectedLen := 10 // ~7 + 3 for suffix
	if len(result) > expectedLen+1 {
		t.Errorf("result length = %d, expected around %d", len(result), expectedLen)
	}
}

func TestTruncator_EmptyText(t *testing.T) {
	tr := NewFromEnd()

	result, truncated := tr.Truncate("", 10)
	if result != "" {
		t.Errorf("expected empty string, got: %q", result)
	}
	if truncated {
		t.Error("expected no truncation for empty string")
	}
}

func TestTruncator_ExactFit(t *testing.T) {
	tr := NewFromEnd()

	// 8 chars = 2 tokens with default 4.0 ratio
	text := "abcdefgh"
	result, truncated := tr.Truncate(text, 2)

	if truncated {
		t.Error("expected no truncation when exact fit")
	}
	if result != text {
		t.Errorf("expected %q, got %q", text, result)
	}
}

func BenchmarkTruncator_TruncateEnd(b *testing.B) {
	tr := NewFromEnd()
	text := strings.Repeat("Hello World ", 1000)

	b.ResetTimer()
	for range b.N {
		tr.Truncate(text, 100)
	}
}

func BenchmarkTruncator_TruncateMiddle(b *testing.B) {
	tr := NewFromMiddle()
	text := strings.Repeat("Hello World ", 1000)

	b.ResetTimer()
	for range b.N {
		tr.Truncate(text, 100)
	}
}

func BenchmarkTruncator_TruncateStart(b *testing.B) {
	tr := NewFromStart()
	text := strings.Repeat("Hello World ", 1000)

	b.ResetTimer()
	for range b.N {
		tr.Truncate(text, 100)
	}
}

func BenchmarkToTokens(b *testing.B) {
	text := strings.Repeat("Hello World ", 1000)

	b.ResetTimer()
	for range b.N {
		ToTokens(text, 100)
	}
}

func BenchmarkToLines(b *testing.B) {
	text := strings.Repeat("Line of text\n", 1000)

	b.ResetTimer()
	for range b.N {
		ToLines(text, 50)
	}
}

func BenchmarkSmart(b *testing.B) {
	text := strings.Repeat("Hello World. ", 1000)

	b.ResetTimer()
	for range b.N {
		Smart(text, 500)
	}
}
