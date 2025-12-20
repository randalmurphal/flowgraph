# llm/tokens

Token counting and budget management for LLM prompts.

## Quick Reference

| Type | Purpose |
|------|---------|
| `Counter` | Interface for token counting |
| `EstimatingCounter` | ~4 chars/token estimation |
| `Budget` | Token allocation management |
| `ModelLimits` | Context window sizes |

## Usage

### Token Counting

```go
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/llm/tokens"

// Default estimator (~4 chars/token)
counter := tokens.NewEstimatingCounter()
count := counter.Count("Hello, world!")      // ~3 tokens
fits := counter.FitsInLimit("text", 1000)    // true

// Custom ratio (tighter estimate)
counter := tokens.NewEstimatingCounterWithRatio(3.0)

// Convenience function
count := tokens.EstimateTokens("Hello, world!")
```

### Budget Management

```go
// Default allocation: 20% system, 40% context, 30% user, 10% reserved
budget := tokens.NewBudget(100000)

// Check fits
budget.FitsSystem(systemPrompt)
budget.FitsContext(contextData)
budget.FitsUser(userMessage)

// Check remaining
remaining := budget.RemainingContext(usedTokens)
remaining := budget.RemainingTotal(systemUsed, contextUsed, userUsed)

// Custom allocation
budget := tokens.NewBudgetWithAllocation(
    100000,  // total
    30,      // 30% system
    40,      // 40% context
    20,      // 20% user
    10,      // 10% reserved
)
```

### Model Limits

```go
limit := tokens.GetModelLimit("claude-opus-4")   // 200000
limit := tokens.GetModelLimit("unknown")          // 100000 (default)
```

## Files

| File | Contents |
|------|----------|
| `counter.go` | Counter interface, EstimatingCounter, ModelLimits |
| `budget.go` | Budget struct and allocation |
| `counter_test.go` | Counter tests |
| `budget_test.go` | Budget tests |
| `doc.go` | Package documentation |

## Default Budget Allocation

| Component | Percentage | Purpose |
|-----------|------------|---------|
| System | 20% | System prompt |
| Context | 40% | Task context, history |
| User | 30% | User messages |
| Reserved | 10% | Response generation |

## Counter Interface

```go
type Counter interface {
    Count(text string) int
    FitsInLimit(text string, limit int) bool
}
```

All token-aware utilities accept this interface, allowing custom implementations
(e.g., tiktoken-based counters) to be swapped in.

## Estimation Accuracy

The EstimatingCounter uses ~4 characters per token, which is a good
approximation for English text with Claude models. For other languages
or more precise counting, use a proper tokenizer implementation.
