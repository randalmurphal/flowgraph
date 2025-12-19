# ADR-011: Panic Recovery Strategy

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

Should the executor recover from panics in node functions? If so, how should they be handled?

## Decision

**Recover panics, convert to errors, include stack trace.**

```go
func (cg *CompiledGraph[S]) executeNode(ctx Context, nodeID string, state S) (S, error) {
    fn := cg.nodes[nodeID]

    // Recover wrapper
    var result S
    var err error

    func() {
        defer func() {
            if r := recover(); r != nil {
                stack := debug.Stack()
                err = &PanicError{
                    NodeID: nodeID,
                    Value:  r,
                    Stack:  string(stack),
                }
            }
        }()
        result, err = fn(ctx, state)
    }()

    return result, err
}
```

### PanicError Type

```go
type PanicError struct {
    NodeID string
    Value  any
    Stack  string
}

func (e *PanicError) Error() string {
    return fmt.Sprintf("panic in node %s: %v\n%s", e.NodeID, e.Value, e.Stack)
}

func (e *PanicError) Unwrap() error {
    if err, ok := e.Value.(error); ok {
        return err
    }
    return nil
}

// IsPanic checks if error originated from a panic
func IsPanic(err error) bool {
    var pe *PanicError
    return errors.As(err, &pe)
}
```

## Alternatives Considered

### 1. Let Panics Propagate

```go
// No recovery - panic crashes the program
fn(ctx, state)
```

**Rejected**: One bad node shouldn't crash entire application. Especially important for long-running servers.

### 2. Recover Without Stack Trace

```go
defer func() {
    if r := recover(); r != nil {
        err = fmt.Errorf("panic in node %s: %v", nodeID, r)
    }
}()
```

**Rejected**: Stack trace is essential for debugging. Without it, finding panic source is nearly impossible.

### 3. Option to Disable Recovery

```go
func WithPanicRecovery(enable bool) RunOption
```

**Rejected for v1**: Simplicity. Recovery is always on. If you want panics, re-panic in error handler.

### 4. Separate Panic Handler

```go
func WithPanicHandler(fn func(nodeID string, panic any) error) RunOption
```

**Rejected for v1**: Over-engineered. Convert to error, let normal error handling deal with it.

## Consequences

### Positive
- **Robust** - Node panic doesn't crash application
- **Debuggable** - Stack trace preserved in error
- **Consistent** - All node failures return errors
- **Composable** - Can handle PanicError like any error

### Negative
- Stack trace can be large
- Might hide programming errors (but stack trace helps)

### Risks
- Panic in hot loop → Mitigate: Performance tests, optimize if needed

---

## Integration with Checkpointing

```go
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error) {
    // ...
    for current != END {
        state, err := cg.executeNode(ctx, current, state)
        if err != nil {
            // Checkpoint the state before the failed node
            if store != nil {
                _ = store.Save(runID, current, state)
            }

            // Check if panic
            if IsPanic(err) {
                ctx.Logger().Error("node panicked",
                    "node_id", current,
                    "panic", err.(*PanicError).Value,
                )
            }

            return state, err
        }
        // ...
    }
}
```

---

## Usage Examples

### Basic Panic Handling
```go
compiled, _ := graph.Compile()

result, err := compiled.Run(ctx, initialState)
if err != nil {
    if flowgraph.IsPanic(err) {
        var pe *flowgraph.PanicError
        errors.As(err, &pe)
        log.Printf("Node %s panicked!\nValue: %v\nStack:\n%s",
            pe.NodeID, pe.Value, pe.Stack)
        // Alert, page on-call, etc.
    } else {
        log.Printf("Node error: %v", err)
    }
}
```

### Re-panic if Desired
```go
result, err := compiled.Run(ctx, initialState)
if flowgraph.IsPanic(err) {
    var pe *flowgraph.PanicError
    errors.As(err, &pe)
    panic(pe.Value)  // Re-panic with original value
}
```

### Testing Panic Recovery
```go
func TestPanicRecovery(t *testing.T) {
    panicNode := func(ctx Context, s testState) (testState, error) {
        panic("something went wrong")
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("panicker", panicNode).
        AddEdge("panicker", flowgraph.END).
        SetEntry("panicker").
        Compile()

    _, err := compiled.Run(context.Background(), testState{})

    require.Error(t, err)
    assert.True(t, flowgraph.IsPanic(err))

    var pe *flowgraph.PanicError
    require.True(t, errors.As(err, &pe))
    assert.Equal(t, "panicker", pe.NodeID)
    assert.Equal(t, "something went wrong", pe.Value)
    assert.Contains(t, pe.Stack, "panic_test.go")
}

func TestPanicWithError(t *testing.T) {
    panicNode := func(ctx Context, s testState) (testState, error) {
        panic(fmt.Errorf("wrapped error"))
    }

    compiled, _ := graph.Compile()
    _, err := compiled.Run(ctx, testState{})

    var pe *flowgraph.PanicError
    require.True(t, errors.As(err, &pe))

    // Unwrap to get the underlying error
    unwrapped := errors.Unwrap(pe)
    assert.EqualError(t, unwrapped, "wrapped error")
}
```

---

## Logging Example

When a panic occurs:

```
2024-01-19T15:30:45Z ERROR node panicked node_id=process-ticket panic="runtime error: index out of range [5] with length 3"
Stack:
goroutine 1 [running]:
runtime/debug.Stack()
    /usr/local/go/src/runtime/debug/stack.go:24 +0x5e
flowgraph.(*CompiledGraph[...]).executeNode.func1()
    /path/to/flowgraph/execute.go:45 +0x85
panic({0x10a3e20?, 0xc0001a2000?})
    /usr/local/go/src/runtime/panic.go:770 +0x132
main.processTicketNode(...)
    /path/to/app/nodes.go:127 +0x1f4
flowgraph.(*CompiledGraph[...]).executeNode(...)
    /path/to/flowgraph/execute.go:52 +0xa5
```
