# ADR-012: Cancellation Propagation

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should context cancellation be handled? When should we check? How do we ensure nodes respect cancellation?

## Decision

**Check cancellation between nodes; nodes responsible for checking during long operations.**

### Executor Behavior

```go
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error) {
    current := cg.entryPoint

    for current != END {
        // Check before each node
        select {
        case <-ctx.Done():
            return state, &CancellationError{
                NodeID:       current,
                State:        state,
                Cause:        ctx.Err(),
                WasExecuting: false,
            }
        default:
        }

        // Execute node (node should check ctx internally)
        state, err := cg.executeNode(ctx, current, state)
        if err != nil {
            // Check if error is due to cancellation
            if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
                return state, &CancellationError{
                    NodeID:       current,
                    State:        state,
                    Cause:        ctx.Err(),
                    WasExecuting: true,
                }
            }
            return state, err
        }

        current, _ = cg.nextNode(ctx, state, current)
    }

    return state, nil
}
```

### CancellationError Type

```go
type CancellationError struct {
    NodeID       string // Node that was about to execute or executing
    State        any    // State at cancellation (for checkpointing)
    Cause        error  // context.Canceled or context.DeadlineExceeded
    WasExecuting bool   // True if cancelled during node execution
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

### Node Responsibility

Nodes MUST check context during long operations:

```go
// Good: Checks context
func longRunningNode(ctx Context, state State) (State, error) {
    for _, item := range state.Items {
        select {
        case <-ctx.Done():
            return state, ctx.Err()
        default:
        }

        if err := processItem(ctx, item); err != nil {
            return state, err
        }
    }
    return state, nil
}

// Good: Passes context to blocking calls
func llmNode(ctx Context, state State) (State, error) {
    result, err := ctx.LLM().Complete(ctx, prompt)  // ctx passed
    if err != nil {
        return state, err  // Will be context.Canceled if cancelled
    }
    state.Result = result
    return state, nil
}

// Bad: Ignores context
func badNode(ctx Context, state State) (State, error) {
    time.Sleep(time.Hour)  // No cancellation check!
    return state, nil
}
```

## Alternatives Considered

### 1. Force Kill Nodes

```go
// Run node in separate goroutine, kill if cancelled
go func() {
    done <- fn(ctx, state)
}()

select {
case result := <-done:
    return result
case <-ctx.Done():
    // Force kill somehow?
}
```

**Rejected**: Go doesn't support killing goroutines. Would need runtime.Goexit() which is problematic.

### 2. Don't Check in Executor

```go
// Rely entirely on nodes checking ctx
for current != END {
    state, err = fn(ctx, state)
    if errors.Is(err, context.Canceled) {
        return state, err
    }
}
```

**Rejected**: If node ignores context, cancellation never happens. At least check between nodes.

### 3. Interrupt-Based

```go
// Send interrupt signal to node
type InterruptibleNode interface {
    Interrupt()
}
```

**Rejected**: Too complex, not Go-idiomatic. Context is the standard approach.

## Consequences

### Positive
- **Standard Go** - Uses context.Context as expected
- **Predictable** - Cancellation checked at known points
- **Partial state preserved** - CancellationError includes state for checkpoint
- **Node flexibility** - Nodes can do cleanup before returning

### Negative
- Nodes that ignore context won't be cancelled mid-execution
- Relies on node authors doing the right thing

### Risks
- Node blocks forever → Mitigate: Document best practices, timeout at graph level

---

## Best Practices for Node Authors

### 1. Always Pass Context Down

```go
func myNode(ctx Context, state State) (State, error) {
    // Pass ctx to all blocking calls
    result, err := externalService.Call(ctx, ...)
    data, err := database.Query(ctx, ...)
    output, err := ctx.LLM().Complete(ctx, ...)
}
```

### 2. Check Context in Loops

```go
func batchProcessor(ctx Context, state State) (State, error) {
    for i, item := range state.Items {
        // Check every N items or every item
        if i%100 == 0 {
            select {
            case <-ctx.Done():
                // Save progress
                state.ProcessedCount = i
                return state, ctx.Err()
            default:
            }
        }
        process(item)
    }
    return state, nil
}
```

### 3. Handle Cleanup

```go
func resourceNode(ctx Context, state State) (State, error) {
    resource := acquire()
    defer resource.Release()  // Always cleanup

    select {
    case <-ctx.Done():
        // Cancelled before work started
        return state, ctx.Err()
    default:
    }

    // Do work
    result, err := resource.Process(ctx, state.Input)
    if err != nil {
        // Could be cancellation or other error
        return state, err
    }

    state.Output = result
    return state, nil
}
```

---

## Usage Examples

### Timeout at Graph Level

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

result, err := compiled.Run(ctx, initialState)
if err != nil {
    var ce *flowgraph.CancellationError
    if errors.As(err, &ce) {
        if errors.Is(ce.Cause, context.DeadlineExceeded) {
            log.Printf("Timed out at node %s", ce.NodeID)
            // Optionally checkpoint the partial state
            if store != nil {
                store.Save(runID, ce.NodeID, ce.State)
            }
        }
    }
}
```

### Manual Cancellation

```go
ctx, cancel := context.WithCancel(context.Background())

// Run in background
go func() {
    result, err := compiled.Run(ctx, initialState)
    // Handle result
}()

// Cancel when needed (e.g., user request, shutdown)
cancel()
```

### Graceful Shutdown

```go
func (s *Server) Run(ctx context.Context) error {
    for task := range s.taskQueue {
        select {
        case <-ctx.Done():
            log.Println("Shutdown requested, stopping gracefully")
            return nil
        default:
        }

        // Run with timeout, but also respect server ctx
        taskCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
        _, err := s.graph.Run(taskCtx, task.State)
        cancel()

        if err != nil && !errors.Is(err, context.Canceled) {
            log.Printf("Task failed: %v", err)
        }
    }
    return nil
}
```

---

## Test Cases

```go
func TestCancellation_BeforeNode(t *testing.T) {
    executed := false
    node := func(ctx Context, s testState) (testState, error) {
        executed = true
        return s, nil
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("a", node).
        AddEdge("a", flowgraph.END).
        SetEntry("a").
        Compile()

    // Already cancelled
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    _, err := compiled.Run(ctx, testState{})

    require.Error(t, err)
    assert.False(t, executed)  // Node never ran
    assert.True(t, errors.Is(err, context.Canceled))

    var ce *flowgraph.CancellationError
    require.True(t, errors.As(err, &ce))
    assert.False(t, ce.WasExecuting)
}

func TestCancellation_DuringNode(t *testing.T) {
    started := make(chan struct{})
    node := func(ctx Context, s testState) (testState, error) {
        close(started)
        <-ctx.Done()  // Wait for cancellation
        return s, ctx.Err()
    }

    compiled, _ := graph.Compile()

    ctx, cancel := context.WithCancel(context.Background())

    go func() {
        <-started
        cancel()
    }()

    _, err := compiled.Run(ctx, testState{})

    var ce *flowgraph.CancellationError
    require.True(t, errors.As(err, &ce))
    assert.True(t, ce.WasExecuting)
}
```
