# Feature: Error Handling

**Related ADRs**: 002-error-handling

---

## Problem Statement

flowgraph must handle errors consistently and informatively:
1. Distinguish different error types (node error, panic, cancellation)
2. Include context (which node, what operation)
3. Support error wrapping for cause inspection
4. Enable programmatic error handling

## User Stories

- As a developer, I want to know which node failed so that I can debug issues
- As a developer, I want to catch specific error types so that I can handle them differently
- As a developer, I want panic stack traces so that I can fix bugs
- As a developer, I want the state at failure so that I can inspect progress

---

## API Design

### Sentinel Errors

```go
// Graph building errors (Compile returns these)
var (
    ErrNoEntryPoint  = errors.New("entry point not set")
    ErrEntryNotFound = errors.New("entry point node not found")
    ErrNodeNotFound  = errors.New("node not found")
    ErrNoPathToEnd   = errors.New("no path to END from entry")
)

// Execution errors (Run returns these)
var (
    ErrMaxIterations      = errors.New("exceeded maximum iterations")
    ErrNilContext         = errors.New("context cannot be nil")
    ErrCheckpointNotFound = errors.New("checkpoint not found")
    ErrInvalidRouterResult = errors.New("router returned empty string")
    ErrRouterTargetNotFound = errors.New("router returned unknown node")
    ErrRunIDRequired       = errors.New("run ID required for checkpointing")
    ErrSerializeState      = errors.New("failed to serialize state")
    ErrDeserializeState    = errors.New("failed to deserialize state")
)
```

### Error Types

```go
// NodeError wraps errors from node execution
type NodeError struct {
    NodeID string  // Which node failed
    Op     string  // Operation: "execute", "checkpoint", "serialize"
    Err    error   // Underlying error
}

func (e *NodeError) Error() string {
    return fmt.Sprintf("node %s: %s: %v", e.NodeID, e.Op, e.Err)
}

func (e *NodeError) Unwrap() error {
    return e.Err
}

// PanicError captures panic information with stack trace
type PanicError struct {
    NodeID string  // Which node panicked
    Value  any     // The panic value
    Stack  string  // Full stack trace
}

func (e *PanicError) Error() string {
    return fmt.Sprintf("node %s panicked: %v", e.NodeID, e.Value)
}

// CancellationError captures context when execution was cancelled
type CancellationError struct {
    NodeID       string  // Node that was about to execute or was executing
    State        any     // State at cancellation (can type-assert)
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

// RouterError wraps errors from conditional edge routing
type RouterError struct {
    FromNode string  // Node with the conditional edge
    Returned string  // What the router returned
    Err      error   // Underlying error
}

func (e *RouterError) Error() string {
    return fmt.Sprintf("router from %s returned %q: %v", e.FromNode, e.Returned, e.Err)
}

func (e *RouterError) Unwrap() error {
    return e.Err
}

// MaxIterationsError provides context when loop limit exceeded
type MaxIterationsError struct {
    Max        int     // The configured limit
    LastNodeID string  // Node that would have executed next
    State      any     // State at termination
}

func (e *MaxIterationsError) Error() string {
    return fmt.Sprintf("exceeded maximum iterations (%d) at node %s", e.Max, e.LastNodeID)
}

func (e *MaxIterationsError) Unwrap() error {
    return ErrMaxIterations
}
```

---

## Behavior Specification

### Error Wrapping Pattern

All errors wrap underlying causes using `%w`:

```go
// Node execution error
return nil, &NodeError{
    NodeID: nodeID,
    Op:     "execute",
    Err:    err,  // Original error from node
}

// Usage:
var nodeErr *NodeError
if errors.As(err, &nodeErr) {
    fmt.Println("Node:", nodeErr.NodeID)
}

if errors.Is(err, ErrSomeSpecificError) {
    // Check underlying cause
}
```

### Compile Error Aggregation

Compile() returns all errors found, not just the first:

```go
_, err := graph.Compile()

// err is errors.Join() of all validation errors
// Check for specific errors:
if errors.Is(err, ErrNoEntryPoint) { ... }
if errors.Is(err, ErrNodeNotFound) { ... }
```

### State Preservation on Error

Run() always returns the state, even on error:

```go
result, err := compiled.Run(ctx, initialState)
if err != nil {
    // result contains state at point of failure
    log.Printf("Failed at step %d with progress: %v", result.Step, result.Progress)
}
```

### Panic Conversion

Panics are recovered and converted to `PanicError`:

```go
func badNode(ctx Context, s State) (State, error) {
    var nilSlice []int
    nilSlice[0] = 1  // panic!
}

result, err := compiled.Run(ctx, state)

var panicErr *PanicError
if errors.As(err, &panicErr) {
    // panicErr.NodeID = "badNode"
    // panicErr.Value = runtime error: index out of range
    // panicErr.Stack = full stack trace
}
```

### Cancellation Detection

```go
result, err := compiled.Run(ctx, state)

if errors.Is(err, context.Canceled) {
    // User cancelled
}

if errors.Is(err, context.DeadlineExceeded) {
    // Timeout
}

var cancelErr *CancellationError
if errors.As(err, &cancelErr) {
    // More details available
    fmt.Printf("Cancelled at node %s, was executing: %v\n",
        cancelErr.NodeID, cancelErr.WasExecuting)
}
```

---

## Error Cases

### Build-Time Errors (Panics)

Builder methods panic on programmer errors:

| Method | Condition | Panic Message |
|--------|-----------|---------------|
| AddNode | Empty ID | "flowgraph: node ID cannot be empty" |
| AddNode | Reserved ID | "flowgraph: node ID cannot be reserved word 'END'" |
| AddNode | Whitespace in ID | "flowgraph: node ID cannot contain whitespace" |
| AddNode | nil function | "flowgraph: node function cannot be nil" |
| AddNode | Duplicate ID | "flowgraph: duplicate node ID: {id}" |

### Compile-Time Errors

| Error | Condition |
|-------|-----------|
| ErrNoEntryPoint | SetEntry() not called |
| ErrEntryNotFound | Entry references non-existent node |
| ErrNodeNotFound | Edge references non-existent node |
| ErrNoPathToEnd | No path from entry to END |

### Runtime Errors

| Error Type | Condition |
|------------|-----------|
| NodeError | Node returns error |
| PanicError | Node panics |
| CancellationError | Context cancelled |
| MaxIterationsError | Loop limit exceeded |
| RouterError | Conditional edge returns invalid target |

---

## Test Cases

### Sentinel Error Checking

```go
func TestError_SentinelChecks(t *testing.T) {
    // Compile error
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA)
        // No entry point

    _, err := graph.Compile()
    assert.ErrorIs(t, err, flowgraph.ErrNoEntryPoint)

    // Runtime error
    _, err = compiled.Run(ctx, state, flowgraph.WithMaxIterations(1))
    assert.ErrorIs(t, err, flowgraph.ErrMaxIterations)
}
```

### Error Type Matching

```go
func TestError_NodeError(t *testing.T) {
    errCustom := errors.New("custom error")
    failNode := func(ctx Context, s State) (State, error) {
        return s, errCustom
    }

    // ... build graph ...

    _, err := compiled.Run(ctx, State{})

    var nodeErr *flowgraph.NodeError
    require.ErrorAs(t, err, &nodeErr)
    assert.Equal(t, "fail", nodeErr.NodeID)
    assert.Equal(t, "execute", nodeErr.Op)
    assert.ErrorIs(t, err, errCustom)  // Unwrapping works
}

func TestError_PanicError(t *testing.T) {
    panicNode := func(ctx Context, s State) (State, error) {
        panic("test panic")
    }

    // ... build graph ...

    _, err := compiled.Run(ctx, State{})

    var panicErr *flowgraph.PanicError
    require.ErrorAs(t, err, &panicErr)
    assert.Equal(t, "panic-node", panicErr.NodeID)
    assert.Equal(t, "test panic", panicErr.Value)
    assert.Contains(t, panicErr.Stack, "panicNode")
}

func TestError_CancellationError(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())

    slowNode := func(ctx flowgraph.Context, s State) (State, error) {
        cancel()  // Cancel during execution
        time.Sleep(10 * time.Millisecond)
        return s, nil
    }

    // ... build graph ...

    _, err := compiled.Run(flowgraph.NewContext(ctx), State{})

    var cancelErr *flowgraph.CancellationError
    require.ErrorAs(t, err, &cancelErr)
    assert.ErrorIs(t, err, context.Canceled)
}
```

### Error Aggregation

```go
func TestCompile_AggregatesErrors(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddEdge("a", "missing1").
        AddEdge("missing2", flowgraph.END)
        // No entry point

    _, err := graph.Compile()

    // All errors present
    assert.ErrorIs(t, err, flowgraph.ErrNoEntryPoint)
    assert.ErrorIs(t, err, flowgraph.ErrNodeNotFound)

    // Error message contains all issues
    assert.Contains(t, err.Error(), "entry point not set")
    assert.Contains(t, err.Error(), "missing1")
    assert.Contains(t, err.Error(), "missing2")
}
```

### State Preservation

```go
func TestError_StatePreserved(t *testing.T) {
    nodes := []string{}
    makeNode := func(name string, shouldFail bool) flowgraph.NodeFunc[State] {
        return func(ctx flowgraph.Context, s State) (State, error) {
            nodes = append(nodes, name)
            s.Progress = append(s.Progress, name)
            if shouldFail {
                return s, errors.New("failed")
            }
            return s, nil
        }
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("a", makeNode("a", false)).
        AddNode("b", makeNode("b", false)).
        AddNode("c", makeNode("c", true)).  // Fails
        AddEdge("a", "b").
        AddEdge("b", "c").
        AddEdge("c", flowgraph.END).
        SetEntry("a")

    compiled, _ := graph.Compile()
    result, err := compiled.Run(ctx, State{})

    require.Error(t, err)
    assert.Equal(t, []string{"a", "b", "c"}, result.Progress)  // All progress preserved
}
```

### Error Formatting

```go
func TestError_Formatting(t *testing.T) {
    err := &flowgraph.NodeError{
        NodeID: "process",
        Op:     "execute",
        Err:    errors.New("connection failed"),
    }

    assert.Equal(t, "node process: execute: connection failed", err.Error())
}

func TestPanicError_Formatting(t *testing.T) {
    err := &flowgraph.PanicError{
        NodeID: "crash",
        Value:  "unexpected nil",
        Stack:  "goroutine 1 [running]:\n...",
    }

    assert.Equal(t, "node crash panicked: unexpected nil", err.Error())
}
```

---

## Performance Requirements

| Operation | Target |
|-----------|--------|
| Error creation | < 1 microsecond |
| Error wrapping | < 100 nanoseconds |
| Stack capture (panic) | < 100 microseconds |

---

## Security Considerations

1. **Stack traces**: May reveal internal structure; don't expose to end users
2. **Error messages**: May contain sensitive state; log carefully
3. **Panic values**: Could be arbitrary; sanitize before display

---

## Simplicity Check

**What we included**:
- Sentinel errors for well-known conditions
- Typed errors with context (NodeError, PanicError, CancellationError)
- Standard error wrapping (`Unwrap()`)
- State preservation on failure

**What we did NOT include**:
- Error codes - Use sentinel errors and type assertions instead
- Error severity levels - All execution errors are fatal; logging handles severity
- Automatic retry - User decides in conditional edges or resume logic
- Error callbacks/hooks - Use errors.As() after Run returns
- Error recovery within execution - Node returns error → execution stops
- Custom error types per node - NodeError wraps with node ID

**Is this the simplest solution?** Yes. Standard Go error patterns with typed errors for context.
