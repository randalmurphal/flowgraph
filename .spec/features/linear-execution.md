# Feature: Linear Execution

**Related ADRs**: 010-execution-model, 011-panic-recovery, 012-cancellation, 013-timeouts

---

## Problem Statement

Users need to execute compiled graphs from entry to END. The execution engine must:
1. Execute nodes in order, passing state through
2. Handle errors gracefully without losing state
3. Recover from panics with useful diagnostics
4. Respect cancellation and timeouts
5. Be predictable and debuggable

## User Stories

- As a developer, I want to run a graph and get the final state so that I can build workflows
- As a developer, I want errors to include which node failed so that I can debug issues
- As a developer, I want panics to be caught and reported so that one bad node doesn't crash my program
- As a developer, I want to cancel execution gracefully so that I can implement timeouts
- As a developer, I want access to the state even when execution fails so that I can inspect progress

---

## API Design

### Run Method

```go
// Run executes the graph with the given initial state
// Returns the final state and any error encountered
// On error, returns the state at the point of failure
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error)
```

### Context Creation

```go
// NewContext creates an execution context from a standard context
func NewContext(ctx context.Context, opts ...ContextOption) Context

// ContextOption configures the context
type ContextOption func(*contextConfig)

// WithLogger sets the logger for this context
func WithLogger(logger *slog.Logger) ContextOption

// WithLLM sets the LLM client for this context
func WithLLM(client LLMClient) ContextOption

// WithCheckpointer sets the checkpoint store for this context
func WithCheckpointer(store CheckpointStore) ContextOption

// WithRunID sets the run identifier for this context
func WithRunID(id string) ContextOption
```

### Run Options

```go
// RunOption configures execution behavior
type RunOption func(*runConfig)

// WithMaxIterations sets the maximum number of node executions
// Default: 1000
// Prevents infinite loops from hanging forever
func WithMaxIterations(n int) RunOption

// WithCheckpointing enables checkpoint saving during execution
func WithCheckpointing(store CheckpointStore) RunOption

// WithRunID sets the run identifier for checkpointing
func WithRunID(id string) RunOption
```

---

## Behavior Specification

### Execution Flow

```
Run(ctx, initialState) called
        │
        ▼
    current = entryPoint
        │
        ▼
┌─►  Check ctx.Done()  ──cancelled──► return CancellationError
│       │
│       │ not cancelled
│       ▼
│   Check iterations < max  ──exceeded──► return ErrMaxIterations
│       │
│       │ ok
│       ▼
│   Execute node(ctx, state)
│       │
│       ├──panic──► recover, return PanicError
│       │
│       ├──error──► wrap in NodeError, return
│       │
│       │ success
│       ▼
│   state = returned state
│       │
│       ▼
│   Determine next node
│       │
│       ├── simple edge ──► current = target
│       │
│       └── conditional edge ──► call RouterFunc, current = result
│       │
│       ▼
│   current == END?
│       │
│       ├── yes ──► return state, nil
│       │
└───────┴── no ──► loop
```

### Cancellation Behavior (ADR-012)

- **Between nodes**: Checked before each node execution
- **During node**: Node is responsible for checking `ctx.Done()`
- **On cancellation**: Returns `CancellationError` with last successful state

```go
// Cancellation mid-execution
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := compiled.Run(flowgraph.NewContext(ctx), state)
if errors.Is(err, context.DeadlineExceeded) {
    // Timed out - result contains state at point of timeout
}
```

### Panic Recovery (ADR-011)

Panics in nodes are caught and converted to `PanicError`:

```go
func badNode(ctx Context, s State) (State, error) {
    panic("something went wrong")  // Recovered by executor
}

result, err := compiled.Run(ctx, state)
var panicErr *flowgraph.PanicError
if errors.As(err, &panicErr) {
    fmt.Println("Node:", panicErr.NodeID)
    fmt.Println("Value:", panicErr.Value)
    fmt.Println("Stack:", panicErr.Stack)
}
```

### Max Iterations

Prevents infinite loops from hanging:

```go
// Loop that never exits
graph.AddConditionalEdge("loop", func(ctx Context, s State) string {
    return "loop"  // Always loops
})

result, err := compiled.Run(ctx, state)
// After 1000 iterations: ErrMaxIterations

// Can configure:
result, err := compiled.Run(ctx, state, flowgraph.WithMaxIterations(10))
```

---

## Error Cases

### Error Types

```go
// NodeError wraps errors from node execution
type NodeError struct {
    NodeID string  // Which node failed
    Op     string  // Operation that failed ("execute")
    Err    error   // Underlying error
}

func (e *NodeError) Error() string {
    return fmt.Sprintf("node %s: %s: %v", e.NodeID, e.Op, e.Err)
}

func (e *NodeError) Unwrap() error {
    return e.Err
}

// PanicError captures panic information
type PanicError struct {
    NodeID string  // Which node panicked
    Value  any     // panic(value)
    Stack  string  // Stack trace
}

func (e *PanicError) Error() string {
    return fmt.Sprintf("node %s panicked: %v", e.NodeID, e.Value)
}

// CancellationError captures state at cancellation
type CancellationError struct {
    NodeID       string  // Node that was about to execute (or was executing)
    State        any     // State at cancellation
    Cause        error   // context.Canceled or context.DeadlineExceeded
    WasExecuting bool    // True if cancelled during node execution
}

func (e *CancellationError) Error() string {
    if e.WasExecuting {
        return fmt.Sprintf("cancelled during node %s: %v", e.NodeID, e.Cause)
    }
    return fmt.Sprintf("cancelled before node %s: %v", e.NodeID, e.Cause)
}

func (e *CancellationError) Unwrap() error {
    return e.Cause
}
```

### Sentinel Errors

```go
var (
    ErrMaxIterations = errors.New("exceeded maximum iterations")
    ErrNilContext    = errors.New("context cannot be nil")
)
```

---

## Test Cases

### Successful Execution

```go
func TestRun_LinearFlow(t *testing.T) {
    graph := flowgraph.NewGraph[Counter]().
        AddNode("inc1", increment).
        AddNode("inc2", increment).
        AddNode("inc3", increment).
        AddEdge("inc1", "inc2").
        AddEdge("inc2", "inc3").
        AddEdge("inc3", flowgraph.END).
        SetEntry("inc1")

    compiled, _ := graph.Compile()
    ctx := flowgraph.NewContext(context.Background())

    result, err := compiled.Run(ctx, Counter{Value: 0})

    require.NoError(t, err)
    assert.Equal(t, 3, result.Value)
}

func TestRun_StatePassedBetweenNodes(t *testing.T) {
    var nodeAState, nodeBState State

    nodeA := func(ctx Context, s State) (State, error) {
        nodeAState = s
        s.Step = 1
        return s, nil
    }
    nodeB := func(ctx Context, s State) (State, error) {
        nodeBState = s
        s.Step = 2
        return s, nil
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddEdge("a", "b").
        AddEdge("b", flowgraph.END).
        SetEntry("a")

    compiled, _ := graph.Compile()
    result, _ := compiled.Run(ctx, State{Initial: "test"})

    assert.Equal(t, "test", nodeAState.Initial)
    assert.Equal(t, 1, nodeBState.Step)  // B received A's output
    assert.Equal(t, 2, result.Step)
}
```

### Error Handling

```go
func TestRun_NodeError_WrapsWithNodeID(t *testing.T) {
    errBoom := errors.New("boom")
    failingNode := func(ctx Context, s State) (State, error) {
        return s, errBoom
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("ok", okNode).
        AddNode("fail", failingNode).
        AddEdge("ok", "fail").
        AddEdge("fail", flowgraph.END).
        SetEntry("ok")

    compiled, _ := graph.Compile()
    result, err := compiled.Run(ctx, State{})

    require.Error(t, err)

    var nodeErr *flowgraph.NodeError
    require.ErrorAs(t, err, &nodeErr)
    assert.Equal(t, "fail", nodeErr.NodeID)
    assert.ErrorIs(t, err, errBoom)

    // State should be from last successful node
    assert.NotEmpty(t, result)
}

func TestRun_NodeError_StatePreserved(t *testing.T) {
    failingNode := func(ctx Context, s State) (State, error) {
        s.Progress = "halfway"
        return s, errors.New("failed")
    }

    // ... build graph with failingNode ...

    result, err := compiled.Run(ctx, State{})

    require.Error(t, err)
    assert.Equal(t, "halfway", result.Progress)  // State preserved
}
```

### Panic Recovery

```go
func TestRun_PanicRecovery(t *testing.T) {
    panicNode := func(ctx Context, s State) (State, error) {
        panic("unexpected error")
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("panic", panicNode).
        AddEdge("panic", flowgraph.END).
        SetEntry("panic")

    compiled, _ := graph.Compile()
    _, err := compiled.Run(ctx, State{})

    require.Error(t, err)

    var panicErr *flowgraph.PanicError
    require.ErrorAs(t, err, &panicErr)
    assert.Equal(t, "panic", panicErr.NodeID)
    assert.Equal(t, "unexpected error", panicErr.Value)
    assert.Contains(t, panicErr.Stack, "panicNode")
}

func TestRun_PanicRecovery_NonStringValue(t *testing.T) {
    panicNode := func(ctx Context, s State) (State, error) {
        panic(42)  // Non-string panic value
    }

    // ... setup ...

    _, err := compiled.Run(ctx, State{})

    var panicErr *flowgraph.PanicError
    require.ErrorAs(t, err, &panicErr)
    assert.Equal(t, 42, panicErr.Value)
}
```

### Cancellation

```go
func TestRun_CancellationBetweenNodes(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())

    slowNode := func(ctx Context, s State) (State, error) {
        cancel()  // Cancel after this node
        s.Completed = append(s.Completed, "slow")
        return s, nil
    }

    // ... graph with slow -> next -> END ...

    result, err := compiled.Run(flowgraph.NewContext(ctx), State{})

    require.Error(t, err)
    assert.ErrorIs(t, err, context.Canceled)

    var cancelErr *flowgraph.CancellationError
    require.ErrorAs(t, err, &cancelErr)
    assert.False(t, cancelErr.WasExecuting)
    assert.Contains(t, result.Completed, "slow")  // First node completed
}

func TestRun_Timeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    slowNode := func(ctx Context, s State) (State, error) {
        time.Sleep(100 * time.Millisecond)
        return s, nil
    }

    // ... graph ...

    _, err := compiled.Run(flowgraph.NewContext(ctx), State{})

    assert.ErrorIs(t, err, context.DeadlineExceeded)
}
```

### Max Iterations

```go
func TestRun_MaxIterations_PreventsInfiniteLoop(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("loop", func(ctx Context, s State) (State, error) {
            s.Count++
            return s, nil
        }).
        AddConditionalEdge("loop", func(ctx Context, s State) string {
            return "loop"  // Always loops
        }).
        SetEntry("loop")

    compiled, _ := graph.Compile()

    result, err := compiled.Run(ctx, State{},
        flowgraph.WithMaxIterations(10))

    require.ErrorIs(t, err, flowgraph.ErrMaxIterations)
    assert.Equal(t, 10, result.Count)
}

func TestRun_MaxIterations_DefaultValue(t *testing.T) {
    // Default should be 1000
    // ... test that loop runs 1000 times before error
}
```

### Context Propagation

```go
func TestRun_ContextPropagated(t *testing.T) {
    var receivedCtx flowgraph.Context

    captureNode := func(ctx flowgraph.Context, s State) (State, error) {
        receivedCtx = ctx
        return s, nil
    }

    // ... graph ...

    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    ctx := flowgraph.NewContext(context.Background(),
        flowgraph.WithLogger(logger),
        flowgraph.WithRunID("test-123"))

    compiled.Run(ctx, State{})

    assert.Equal(t, "test-123", receivedCtx.RunID())
    assert.Same(t, logger, receivedCtx.Logger())
}
```

---

## Performance Requirements

| Metric | Target | Notes |
|--------|--------|-------|
| Per-node overhead | < 1 microsecond | Execution loop only |
| Context creation | < 100 nanoseconds | Allocation cost |
| Panic recovery | < 10 microseconds | Stack capture |

The execution overhead should be negligible compared to actual node work.

---

## Security Considerations

1. **Panic safety**: Recovered panics don't leak goroutine state
2. **Resource cleanup**: Cancellation ensures execution stops promptly
3. **No infinite hangs**: Max iterations provides guaranteed termination

---

## Simplicity Check

**What we included**:
- Single Run() method with options
- State returned even on error
- Clear error types (NodeError, PanicError, CancellationError)
- Context carries run-wide configuration
- Max iterations prevents infinite loops

**What we did NOT include**:
- Step-by-step execution (Run to next node) - Use checkpointing + Resume instead
- Retry logic - User implements in node or via conditional edge
- Progress callbacks - Use logging or checkpointing for visibility
- Parallel execution - Deferred to v2
- Return channels/streaming - Not needed for synchronous execution

**Is this the simplest solution?** Yes. Run takes initial state, returns final state. Everything else is error handling.
