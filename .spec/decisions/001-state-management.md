# ADR-001: State Management Strategy

**Status**: Proposed
**Date**: 2025-01-19
**Deciders**: @rmurphy

---

## Context

flowgraph passes state between nodes as workflows execute. We need to decide how state flows through the graph - whether nodes receive state by value (copy) or by reference (pointer), and whether nodes mutate state in-place or return new state.

This decision affects:
- API ergonomics
- Testability
- Concurrency safety
- Checkpointing complexity
- Memory usage

## Problem Statement

How should state be passed between nodes, and how should nodes update state?

## Decision Drivers

- **Testability**: Easy to test nodes in isolation
- **Immutability**: Prefer immutable patterns for reliability
- **Concurrency**: Safe for potential future parallel execution
- **Ergonomics**: Natural Go patterns
- **Checkpointing**: Easy to serialize state at any point
- **Memory**: Reasonable memory usage for large state

## Considered Options

### Option 1: Pass by Value, Return New State

```go
type NodeFunc[S any] func(ctx Context, state S) (S, error)

func myNode(ctx Context, state MyState) (MyState, error) {
    state.Output = process(state.Input)
    return state, nil
}
```

**Pros**:
- Immutable semantics (original state unchanged)
- Easy to reason about
- Thread-safe by default
- Natural checkpointing (serialize any state)
- Easy to test (pure functions)
- Familiar functional pattern

**Cons**:
- Copying overhead for large state
- Need to return modified state explicitly
- Might feel verbose

### Option 2: Pass by Pointer, Mutate In-Place

```go
type NodeFunc[S any] func(ctx Context, state *S) error

func myNode(ctx Context, state *MyState) error {
    state.Output = process(state.Input)
    return nil
}
```

**Pros**:
- No copy overhead
- Familiar imperative pattern
- Less verbose (no return)

**Cons**:
- Mutable state harder to reason about
- Not thread-safe (needs synchronization for parallel)
- Checkpointing needs careful timing
- Testing requires more setup
- Original state destroyed on error

### Option 3: State Container with Methods

```go
type State[S any] struct {
    current S
    history []S
}

type NodeFunc[S any] func(ctx Context, state *State[S]) error

func myNode(ctx Context, state *State[MyState]) error {
    state.Update(func(s MyState) MyState {
        s.Output = process(s.Input)
        return s
    })
    return nil
}
```

**Pros**:
- Built-in history/undo
- Controlled mutation
- Can track changes

**Cons**:
- More complex API
- Higher memory for history
- Unfamiliar pattern
- Overhead for simple cases

## Decision

We will use **Option 1: Pass by Value, Return New State** because:

1. **Immutability wins for reliability** - Nodes are pure functions that transform state. No hidden mutation means easier debugging and testing.

2. **Thread-safe by default** - If we add parallel execution later, value semantics prevent data races without additional synchronization.

3. **Natural checkpointing** - State can be serialized at any point. Failed nodes don't corrupt state.

4. **Testable** - Nodes are pure functions: given input, verify output.

5. **Copy overhead is acceptable** - Modern Go compiler optimizes struct copies. For truly large state, use pointers within the struct (e.g., `*LargeData` field).

## Consequences

### Positive

- Nodes are pure functions - easy to test
- State is immutable - easy to reason about
- Checkpointing is trivial - serialize current state
- Future parallel execution possible

### Negative

- **Copy overhead for large state**: Mitigate by using pointer fields for large data:
  ```go
  type State struct {
      ID       string      // Copied (cheap)
      LargeData *BigStruct // Pointer copied, data shared
  }
  ```

- **Must return state explicitly**: This is actually a feature - makes state flow visible.

### Neutral

- Different from Python reference semantics (but that's fine - Go is not Python)

## Implementation Notes

```go
// State struct design guidance
type State struct {
    // Value types for small data (copied)
    ID      string
    Counter int

    // Pointer types for large data (shared)
    LargePayload *Payload
    Transcript   *Transcript

    // Slice/map for collections (headers copied, data shared)
    Results []Result
    Cache   map[string]any
}
```

**Guidance for large state**:
1. Keep top-level struct small (IDs, status, counters)
2. Use pointers for large nested data
3. If truly massive (>1MB), consider external storage with ID reference

## Validation

This decision is correct if:
- [ ] Nodes can be tested as pure functions
- [ ] Checkpointing works without special handling
- [ ] No data races in tests with `-race`
- [ ] Memory usage is reasonable (<2x expected)

## References

- Go FAQ on [pass by value](https://go.dev/doc/faq#pass_by_value)
- LangGraph uses similar value semantics
