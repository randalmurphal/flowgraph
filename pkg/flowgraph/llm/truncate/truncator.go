package truncate

import "github.com/randalmurphal/flowgraph/pkg/flowgraph/llm/tokens"

// Strategy defines how text is truncated.
type Strategy int

const (
	// FromEnd removes content from the end (default).
	FromEnd Strategy = iota

	// FromMiddle removes content from the middle, keeping start and end.
	FromMiddle

	// FromStart removes content from the start.
	FromStart
)

// DefaultEndSuffix is the default suffix for end truncation.
const DefaultEndSuffix = "..."

// DefaultMiddleSuffix is the default suffix for middle truncation.
const DefaultMiddleSuffix = "\n...[content truncated]...\n"

// DefaultStartSuffix is the default suffix for start truncation.
const DefaultStartSuffix = "..."

// Truncator truncates text to fit within token limits.
type Truncator struct {
	counter  tokens.Counter
	strategy Strategy
	suffix   string
}

// New creates a truncator with the given strategy.
func New(strategy Strategy) *Truncator {
	suffix := DefaultEndSuffix
	if strategy == FromMiddle {
		suffix = DefaultMiddleSuffix
	}
	return &Truncator{
		counter:  tokens.NewEstimatingCounter(),
		strategy: strategy,
		suffix:   suffix,
	}
}

// NewFromEnd creates a truncator that removes content from the end.
func NewFromEnd() *Truncator {
	return New(FromEnd)
}

// NewFromMiddle creates a truncator that removes content from the middle.
func NewFromMiddle() *Truncator {
	return New(FromMiddle)
}

// NewFromStart creates a truncator that removes content from the start.
func NewFromStart() *Truncator {
	return New(FromStart)
}

// WithCounter sets a custom token counter.
func (t *Truncator) WithCounter(counter tokens.Counter) *Truncator {
	t.counter = counter
	return t
}

// WithSuffix sets a custom suffix for truncation.
func (t *Truncator) WithSuffix(suffix string) *Truncator {
	t.suffix = suffix
	return t
}

// Truncate reduces the text to fit within the token limit.
// Returns the truncated text and whether truncation occurred.
func (t *Truncator) Truncate(text string, maxTokens int) (string, bool) {
	if t.counter.FitsInLimit(text, maxTokens) {
		return text, false
	}

	switch t.strategy {
	case FromEnd:
		return t.truncateEnd(text, maxTokens), true
	case FromMiddle:
		return t.truncateMiddle(text, maxTokens), true
	case FromStart:
		return t.truncateStart(text, maxTokens), true
	default:
		return t.truncateEnd(text, maxTokens), true
	}
}

// Strategy returns the truncator's strategy.
func (t *Truncator) Strategy() Strategy {
	return t.strategy
}

// Suffix returns the truncator's suffix.
func (t *Truncator) Suffix() string {
	return t.suffix
}
