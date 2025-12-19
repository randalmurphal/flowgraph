# ADR-003: Context Interface Design

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

Node functions need access to shared resources (logger, LLM client, checkpoint store) without polluting the state type. Go's `context.Context` provides cancellation and deadlines but not service access.

## Decision

**Create a custom `flowgraph.Context` that wraps `context.Context` and provides service access.**

```go
// Context provides execution context to nodes
type Context interface {
    // Standard context methods
    context.Context

    // Service access
    Logger() *slog.Logger
    LLM() LLMClient              // nil if not configured
    Checkpointer() Checkpointer  // nil if not configured

    // Execution metadata
    RunID() string
    NodeID() string
    Attempt() int

    // User-defined values (type-safe)
    Value(key any) any
    WithValue(key, val any) Context
}
```

### Implementation

```go
type executionContext struct {
    context.Context
    logger       *slog.Logger
    llm          LLMClient
    checkpointer Checkpointer
    runID        string
    nodeID       string
    attempt      int
    values       map[any]any
}

func (c *executionContext) Logger() *slog.Logger {
    return c.logger.With("run_id", c.runID, "node_id", c.nodeID)
}

func (c *executionContext) WithValue(key, val any) Context {
    newValues := make(map[any]any, len(c.values)+1)
    for k, v := range c.values {
        newValues[k] = v
    }
    newValues[key] = val
    return &executionContext{
        Context:      c.Context,
        logger:       c.logger,
        llm:          c.llm,
        checkpointer: c.checkpointer,
        runID:        c.runID,
        nodeID:       c.nodeID,
        attempt:      c.attempt,
        values:       newValues,
    }
}
```

## Alternatives Considered

### 1. Embed Services in State

```go
type MyState struct {
    Logger *slog.Logger
    LLM    LLMClient
    // ... actual state
}
```

**Rejected**: Pollutes state, makes serialization complex, forces state to be mutable.

### 2. Use Standard context.Context Only

```go
func myNode(ctx context.Context, state S) (S, error) {
    logger := ctx.Value("logger").(*slog.Logger)
}
```

**Rejected**: No type safety, ugly casts, easy to forget values.

### 3. Global Service Registry

```go
var globalLogger *slog.Logger

func myNode(ctx Context, state S) (S, error) {
    globalLogger.Info("processing")
}
```

**Rejected**: Testing nightmare, hidden dependencies.

## Consequences

### Positive
- Type-safe service access
- Automatic context enrichment (run_id, node_id in logs)
- Standard context.Context compatibility
- Easy to test (mock the interface)

### Negative
- Custom interface to learn
- Must wrap context.Context before calling Run()

### Risks
- Interface could grow too large → Mitigate: Keep it minimal, use Value() for extension

---

## Design Principles Applied

1. **Explicit over implicit** - Services passed through context, not global
2. **Interface segregation** - Context only exposes what nodes need
3. **Testing in mind** - Easy to mock the whole interface

---

## Test Cases

```go
// Mock implementation for testing
type mockContext struct {
    context.Context
    logger *slog.Logger
    llm    *MockLLM
}

func (m *mockContext) Logger() *slog.Logger { return m.logger }
func (m *mockContext) LLM() LLMClient { return m.llm }
// ... etc

func TestNodeWithContext(t *testing.T) {
    ctx := &mockContext{
        Context: context.Background(),
        logger:  slog.Default(),
        llm:     &MockLLM{Response: "test"},
    }

    state := MyState{Input: "hello"}
    result, err := myNode(ctx, state)

    assert.NoError(t, err)
    assert.Equal(t, "expected", result.Output)
}
```
