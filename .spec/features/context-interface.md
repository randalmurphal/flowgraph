# Feature: Context Interface

**Related ADRs**: 003-context-interface

---

## Problem Statement

Nodes need access to services and metadata beyond the state:
1. Logger for debugging
2. LLM client for AI calls
3. Checkpoint store for manual checkpointing
4. Cancellation and deadlines
5. Run metadata (ID, current node)

The Context interface provides all of this through a single parameter.

## User Stories

- As a developer, I want a single context parameter so that node signatures stay simple
- As a developer, I want the logger pre-configured so that logs include run/node context
- As a developer, I want cancellation to propagate so that workflows stop cleanly
- As a developer, I want to access metadata so that nodes can make decisions based on context

---

## API Design

### Context Interface

```go
// Context provides execution context to nodes
// Extends context.Context with flowgraph-specific services
type Context interface {
    context.Context

    // Services
    Logger() *slog.Logger
    LLM() LLMClient
    Checkpointer() CheckpointStore

    // Metadata
    RunID() string
    NodeID() string
    Attempt() int  // Retry attempt number (1 = first attempt)
}
```

### Context Creation

```go
// NewContext creates an execution context from a standard context
func NewContext(ctx context.Context, opts ...ContextOption) Context

// ContextOption configures the context
type ContextOption func(*executionContext)

// WithLogger sets the logger for this context
func WithLogger(logger *slog.Logger) ContextOption

// WithLLM sets the LLM client for this context
func WithLLM(client LLMClient) ContextOption

// WithCheckpointer sets the checkpoint store for this context
func WithCheckpointer(store CheckpointStore) ContextOption

// WithRunID sets the run identifier
func WithRunID(id string) ContextOption
```

### Default Values

| Method | Default |
|--------|---------|
| `Logger()` | `slog.Default()` |
| `LLM()` | `nil` (nodes must check) |
| `Checkpointer()` | `nil` (nodes must check) |
| `RunID()` | Auto-generated UUID |
| `NodeID()` | Set by executor (empty until execution) |
| `Attempt()` | `1` |

---

## Behavior Specification

### Context Lifecycle

```
User creates Context
        │
        ▼
Run(ctx, state) called
        │
        ▼
Executor enriches context per-node:
  - Sets NodeID
  - Enriches logger with node_id
  - Increments Attempt on retry
        │
        ▼
Node receives enriched context
        │
        ▼
Node uses context services
```

### Logger Enrichment

The executor automatically adds structured fields:

```go
// User configures
ctx := flowgraph.NewContext(ctx, flowgraph.WithLogger(myLogger))

// Inside node execution, ctx.Logger() includes:
// - run_id: "run-123"
// - node_id: "process"
// - attempt: 1

ctx.Logger().Info("processing")
// Output: level=INFO msg=processing run_id=run-123 node_id=process attempt=1
```

### Cancellation Propagation

Context embeds `context.Context`, so cancellation flows through:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

fgCtx := flowgraph.NewContext(ctx)

// In node:
func myNode(ctx flowgraph.Context, s State) (State, error) {
    select {
    case <-ctx.Done():
        return s, ctx.Err()  // Timeout or cancel
    case result := <-longOperation():
        s.Result = result
        return s, nil
    }
}
```

### Nil Service Handling

Nodes should check for nil services:

```go
func myNode(ctx flowgraph.Context, s State) (State, error) {
    llm := ctx.LLM()
    if llm == nil {
        return s, errors.New("LLM client not configured")
    }

    resp, err := llm.Complete(ctx, req)
    // ...
}
```

### Thread Safety

Context is immutable after creation. Multiple goroutines can safely read from it.

The executor creates derived contexts for each node (with updated NodeID), which is also safe.

---

## Error Cases

No specific errors from Context itself. Services may return errors when used.

Common patterns:

```go
// LLM not configured
if ctx.LLM() == nil {
    return s, errors.New("node requires LLM client but none configured")
}

// Checkpointer not configured
if ctx.Checkpointer() == nil {
    return s, errors.New("manual checkpoint requires checkpointer")
}
```

---

## Test Cases

### Context Creation

```go
func TestNewContext_DefaultValues(t *testing.T) {
    ctx := flowgraph.NewContext(context.Background())

    assert.NotNil(t, ctx.Logger())
    assert.Nil(t, ctx.LLM())
    assert.Nil(t, ctx.Checkpointer())
    assert.NotEmpty(t, ctx.RunID())  // Auto-generated
    assert.Equal(t, "", ctx.NodeID())  // Not yet executing
    assert.Equal(t, 1, ctx.Attempt())
}

func TestNewContext_WithOptions(t *testing.T) {
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    llm := flowgraph.NewMockLLM("test")
    store := flowgraph.NewMemoryStore()

    ctx := flowgraph.NewContext(context.Background(),
        flowgraph.WithLogger(logger),
        flowgraph.WithLLM(llm),
        flowgraph.WithCheckpointer(store),
        flowgraph.WithRunID("custom-run-id"))

    assert.Same(t, logger, ctx.Logger())
    assert.Same(t, llm, ctx.LLM())
    assert.Same(t, store, ctx.Checkpointer())
    assert.Equal(t, "custom-run-id", ctx.RunID())
}
```

### Context Cancellation

```go
func TestContext_CancellationPropagates(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    fgCtx := flowgraph.NewContext(ctx)

    cancel()

    assert.Error(t, fgCtx.Err())
    assert.ErrorIs(t, fgCtx.Err(), context.Canceled)
}

func TestContext_DeadlinePropagates(t *testing.T) {
    deadline := time.Now().Add(1 * time.Hour)
    ctx, cancel := context.WithDeadline(context.Background(), deadline)
    defer cancel()

    fgCtx := flowgraph.NewContext(ctx)

    d, ok := fgCtx.Deadline()
    assert.True(t, ok)
    assert.Equal(t, deadline, d)
}
```

### Logger Enrichment

```go
func TestContext_LoggerEnrichedDuringExecution(t *testing.T) {
    var logOutput bytes.Buffer
    handler := slog.NewTextHandler(&logOutput, nil)
    logger := slog.New(handler)

    var capturedCtx flowgraph.Context
    captureNode := func(ctx flowgraph.Context, s State) (State, error) {
        capturedCtx = ctx
        ctx.Logger().Info("from node")
        return s, nil
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("test", captureNode).
        AddEdge("test", flowgraph.END).
        SetEntry("test")

    compiled, _ := graph.Compile()
    ctx := flowgraph.NewContext(context.Background(),
        flowgraph.WithLogger(logger),
        flowgraph.WithRunID("run-123"))

    compiled.Run(ctx, State{})

    assert.Equal(t, "test", capturedCtx.NodeID())
    assert.Contains(t, logOutput.String(), "run_id=run-123")
    assert.Contains(t, logOutput.String(), "node_id=test")
}
```

### Value Propagation

```go
func TestContext_ValuesFromParent(t *testing.T) {
    type keyType string
    key := keyType("custom")

    parentCtx := context.WithValue(context.Background(), key, "value")
    fgCtx := flowgraph.NewContext(parentCtx)

    assert.Equal(t, "value", fgCtx.Value(key))
}
```

### Service Usage in Nodes

```go
func TestNode_UsesContextLLM(t *testing.T) {
    mock := flowgraph.NewMockLLM("response")

    llmNode := func(ctx flowgraph.Context, s State) (State, error) {
        resp, err := ctx.LLM().Complete(ctx, flowgraph.CompletionRequest{
            Messages: []flowgraph.Message{{Role: flowgraph.RoleUser, Content: "hello"}},
        })
        if err != nil {
            return s, err
        }
        s.Output = resp.Content
        return s, nil
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("llm", llmNode).
        AddEdge("llm", flowgraph.END).
        SetEntry("llm")

    compiled, _ := graph.Compile()
    ctx := flowgraph.NewContext(context.Background(),
        flowgraph.WithLLM(mock))

    result, err := compiled.Run(ctx, State{})

    require.NoError(t, err)
    assert.Equal(t, "response", result.Output)
}

func TestNode_NilLLM_Handled(t *testing.T) {
    llmNode := func(ctx flowgraph.Context, s State) (State, error) {
        if ctx.LLM() == nil {
            return s, errors.New("LLM not configured")
        }
        // ...
        return s, nil
    }

    // ... run with no LLM configured ...

    _, err := compiled.Run(ctx, State{})

    assert.ErrorContains(t, err, "LLM not configured")
}
```

---

## Performance Requirements

| Operation | Target |
|-----------|--------|
| Context creation | < 1 microsecond |
| Method access | < 10 nanoseconds |
| Logger with fields | < 100 nanoseconds |

---

## Security Considerations

1. **Context values**: Don't store secrets in context values
2. **Logger output**: May contain sensitive data; configure log output carefully
3. **LLM client**: Has full process permissions; user must trust provider

---

## Simplicity Check

**What we included**:
- Single interface extending context.Context
- Essential services: Logger, LLM, Checkpointer
- Essential metadata: RunID, NodeID, Attempt
- Functional options for configuration

**What we did NOT include**:
- Metrics client - Use logger or external metrics library
- Tracing - Use standard context values for trace propagation
- Request ID - Use RunID or context values
- User identity - Not flowgraph's concern; add to state
- Configuration - Add to state if needed
- Custom service injection - Use context.Value() for rare cases

**Is this the simplest solution?** Yes. Context has exactly what nodes need, nothing more.
