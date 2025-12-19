# ADR-005: Node Function Signature

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

What should the function signature be for nodes in the graph? This is the most fundamental API decision affecting every user.

## Decision

**Pure function signature: `func(Context, S) (S, error)`**

```go
type NodeFunc[S any] func(ctx Context, state S) (S, error)

// Example
func processTicket(ctx Context, state TicketState) (TicketState, error) {
    result, err := ctx.LLM().Complete(ctx, processPrompt(state.Ticket))
    if err != nil {
        return state, fmt.Errorf("process ticket: %w", err)
    }
    state.Processed = result
    return state, nil
}
```

### Design Rationale

1. **Context first** - Go convention, carries cancellation/deadline/services
2. **State by value** - Immutability, per ADR-001
3. **Returns new state** - Makes checkpointing natural (save return value)
4. **Returns error** - Explicit error handling, per ADR-002
5. **Generic over S** - Type safety for state, no casting

### Restrictions

- **No variadic args** - State shape must be known at compile time
- **No multiple state returns** - Single state keeps graph simple
- **No callbacks** - Pure function, easier to test

## Alternatives Considered

### 1. State as Pointer (Mutation)

```go
type NodeFunc[S any] func(ctx Context, state *S) error
```

**Rejected**: Per ADR-001, we chose immutable state.

### 2. Result Type Instead of Error

```go
type Result[S any] struct {
    State S
    Err   error
    Skip  bool  // Skip remaining nodes
}

type NodeFunc[S any] func(ctx Context, state S) Result[S]
```

**Rejected**: Over-engineered for v1. Skip can be achieved via conditional edges.

### 3. Separate Input/Output Types

```go
type NodeFunc[In, Out any] func(ctx Context, in In) (Out, error)
```

**Rejected**: Makes graph typing extremely complex. Same type for in/out is simpler.

### 4. Method on State Type

```go
type Processable interface {
    Process(ctx Context) error
}
```

**Rejected**: State becomes behavior-aware, harder to test, less flexible.

### 5. Return (S, S, error) for Checkpoint

```go
// Returns: (state to pass to next node, state to checkpoint, error)
type NodeFunc[S any] func(ctx Context, state S) (S, S, error)
```

**Rejected**: Confusing. Checkpoint state should equal returned state.

## Consequences

### Positive
- **Simple** - Easy to understand, hard to misuse
- **Testable** - Pure function, mock context, assert output
- **Type-safe** - Generics prevent runtime type errors
- **Go-idiomatic** - Follows standard patterns

### Negative
- State must be a single type (no dynamic shapes)
- Large states copied on each node (but Go optimizes this well)

### Risks
- Performance with huge states → Mitigate: Use pointers inside state struct for large data

---

## Usage Patterns

### Basic Node
```go
func addOne(ctx Context, state Counter) (Counter, error) {
    state.Value++
    return state, nil
}
```

### Node with LLM Call
```go
func generateSpec(ctx Context, state TicketState) (TicketState, error) {
    prompt := fmt.Sprintf("Generate spec for: %s", state.Ticket.Description)

    result, err := ctx.LLM().Complete(ctx, prompt)
    if err != nil {
        return state, fmt.Errorf("generate spec: %w", err)
    }

    state.Spec = result.Text
    state.TokensUsed += result.TokensIn + result.TokensOut
    return state, nil
}
```

### Node with Side Effects
```go
func saveToDatabase(ctx Context, state Order) (Order, error) {
    // Side effects are allowed - node is pure in the graph sense
    // (same input + same DB state = same output)
    if err := db.Save(ctx, state.Order); err != nil {
        return state, fmt.Errorf("save order: %w", err)
    }

    state.SavedAt = time.Now()
    return state, nil
}
```

### Node with Early Exit
```go
func validateInput(ctx Context, state Request) (Request, error) {
    if state.Input == "" {
        return state, ErrEmptyInput  // Graph handles routing on error
    }
    state.Validated = true
    return state, nil
}
```

---

## Test Patterns

```go
func TestProcessTicket(t *testing.T) {
    // Arrange
    ctx := NewMockContext(t).
        WithLLM(&MockLLM{
            CompleteFunc: func(ctx Context, prompt string) (*LLMResult, error) {
                return &LLMResult{Text: "processed"}, nil
            },
        })

    input := TicketState{Ticket: &Ticket{ID: "123"}}

    // Act
    output, err := processTicket(ctx, input)

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "processed", output.Processed)
    assert.Equal(t, "123", output.Ticket.ID)  // Original unchanged
}
```
