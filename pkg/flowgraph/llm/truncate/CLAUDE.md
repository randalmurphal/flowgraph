# llm/truncate

Text truncation utilities for managing LLM context windows.

## Quick Reference

| Type | Purpose |
|------|---------|
| `Truncator` | Configurable text truncator |
| `Strategy` | Truncation strategy (FromEnd, FromMiddle, FromStart) |

## Strategies

| Strategy | Suffix | Use Case |
|----------|--------|----------|
| `FromEnd` | `...` | Most common - preserves context start |
| `FromMiddle` | `[content truncated]` | Keeps both start and end |
| `FromStart` | `...` | Preserves recent content |

## Usage

### Basic Truncation

```go
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/llm/truncate"

// Create truncator
tr := truncate.NewFromEnd()
result, truncated := tr.Truncate(text, 100)  // 100 tokens max

// Other strategies
tr := truncate.NewFromMiddle()
tr := truncate.NewFromStart()
tr := truncate.New(truncate.FromEnd)
```

### Custom Configuration

```go
// Custom suffix
tr := truncate.NewFromEnd().WithSuffix("[...]")

// Custom token counter
tr := truncate.NewFromEnd().WithCounter(myCounter)
```

### Convenience Functions

```go
// Token-based (uses default estimator)
result := truncate.ToTokens(text, 100)

// Line-based
result := truncate.ToLines(text, 50)

// Character-based
result := truncate.ToLength(text, 500)

// Smart truncation (word boundaries)
result := truncate.Smart(text, 500)
```

## Files

| File | Contents |
|------|----------|
| `truncator.go` | Truncator struct, Strategy type, constructors |
| `strategies.go` | Strategy implementations (binary search) |
| `convenience.go` | ToTokens, ToLines, ToLength, Smart functions |
| `truncator_test.go` | Comprehensive tests |
| `doc.go` | Package documentation |

## Implementation Notes

- Uses binary search for efficient token-aware truncation
- Properly handles UTF-8 (counts runes, not bytes)
- Default token counter: ~4 chars/token estimate
- Reserves space for suffix in token calculations

## Dependencies

- `github.com/randalmurphal/flowgraph/pkg/flowgraph/llm/tokens` for Counter interface
