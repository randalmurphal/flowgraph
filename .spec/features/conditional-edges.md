# Feature: Conditional Edges

**Related ADRs**: 006-edge-representation, 008-cycle-handling

---

## Problem Statement

Workflows need decision points where the next node depends on the current state. Examples:
- If review approved, create PR; otherwise, fix issues
- If validation fails, return error node
- If retry count exceeded, go to failure handler

Conditional edges enable branching without cluttering node logic with routing decisions.

## User Stories

- As a developer, I want to branch based on state so that workflows can make decisions
- As a developer, I want routing logic separate from node logic so that nodes stay focused
- As a developer, I want compile-time validation of edge targets so that I catch typos early
- As a developer, I want conditional edges to END so that workflows can terminate early

---

## API Design

### RouterFunc Type

```go
// RouterFunc determines the next node based on context and state
// Returns a node ID or flowgraph.END
type RouterFunc[S any] func(ctx Context, state S) string
```

### AddConditionalEdge Method

```go
// AddConditionalEdge adds a conditional edge from a node
// The RouterFunc is called during execution to determine the target
// RouterFunc must return a valid node ID or flowgraph.END
func (g *Graph[S]) AddConditionalEdge(from string, router RouterFunc[S]) *Graph[S]
```

### Usage Pattern

```go
graph := flowgraph.NewGraph[ReviewState]().
    AddNode("review", reviewNode).
    AddNode("create-pr", createPRNode).
    AddNode("fix-issues", fixIssuesNode).
    AddConditionalEdge("review", func(ctx Context, s ReviewState) string {
        if s.Approved {
            return "create-pr"
        }
        return "fix-issues"
    }).
    AddEdge("create-pr", flowgraph.END).
    AddEdge("fix-issues", "review").  // Loop back
    SetEntry("review")
```

---

## Behavior Specification

### Routing Decision Timing

The RouterFunc is called **after** the node executes, with the **returned state**:

```
Node executes: state_out, err = node(ctx, state_in)
        │
        ▼
    err != nil?  ──yes──►  Return error
        │
        no
        ▼
RouterFunc(ctx, state_out)  ──►  next node ID
```

### Valid Return Values

| Return Value | Behavior |
|--------------|----------|
| Valid node ID | Execute that node next |
| `flowgraph.END` | Terminate execution successfully |
| Empty string `""` | Runtime error: `ErrInvalidRouterResult` |
| Unknown node ID | Runtime error: `ErrRouterTargetNotFound` |

### Compile-Time vs Runtime Validation

**Compile-time**: Cannot validate RouterFunc return values (they depend on state).

**Runtime**: Executor validates that returned node ID exists:

```go
router := func(ctx Context, s State) string {
    return "nonexistent"  // No compile error
}

result, err := compiled.Run(ctx, state)
// Runtime: ErrRouterTargetNotFound
```

### Multiple Conditional Edges

A node can have only **one** edge type - either simple edges OR one conditional edge:

```go
// Valid: Conditional edge only
graph.AddConditionalEdge("decide", router)

// Invalid: Mixing edge types
graph.AddEdge("decide", "a")
graph.AddConditionalEdge("decide", router)  // panic: already has edges
```

### RouterFunc Panics

Panics in RouterFunc are recovered and wrapped in `PanicError`:

```go
router := func(ctx Context, s State) string {
    panic("router error")
}

_, err := compiled.Run(ctx, state)
var panicErr *flowgraph.PanicError
errors.As(err, &panicErr)  // Node ID will be the from-node
```

---

## Error Cases

### Runtime Errors

```go
var (
    ErrInvalidRouterResult    = errors.New("router returned empty string")
    ErrRouterTargetNotFound   = errors.New("router returned unknown node")
)

// RouterError wraps router-related errors
type RouterError struct {
    FromNode   string  // Node with the conditional edge
    Returned   string  // What the router returned
    Err        error   // Underlying error
}

func (e *RouterError) Error() string {
    return fmt.Sprintf("router from %s returned %q: %v",
        e.FromNode, e.Returned, e.Err)
}
```

### Compile-Time Errors

When adding conditional edge to non-existent source node:

```go
graph.AddConditionalEdge("nonexistent", router)
// Captured at Compile(): edge source 'nonexistent' does not exist
```

---

## Test Cases

### Basic Branching

```go
func TestConditionalEdge_BasicBranching(t *testing.T) {
    var path []string

    leftNode := func(ctx Context, s State) (State, error) {
        path = append(path, "left")
        return s, nil
    }
    rightNode := func(ctx Context, s State) (State, error) {
        path = append(path, "right")
        return s, nil
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("start", startNode).
        AddNode("left", leftNode).
        AddNode("right", rightNode).
        AddConditionalEdge("start", func(ctx Context, s State) string {
            if s.GoLeft { return "left" }
            return "right"
        }).
        AddEdge("left", flowgraph.END).
        AddEdge("right", flowgraph.END).
        SetEntry("start")

    compiled, _ := graph.Compile()

    // Test left path
    path = nil
    compiled.Run(ctx, State{GoLeft: true})
    assert.Equal(t, []string{"left"}, path)

    // Test right path
    path = nil
    compiled.Run(ctx, State{GoLeft: false})
    assert.Equal(t, []string{"right"}, path)
}
```

### Conditional to END

```go
func TestConditionalEdge_ToEND(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("check", checkNode).
        AddNode("process", processNode).
        AddConditionalEdge("check", func(ctx Context, s State) string {
            if s.Skip {
                return flowgraph.END  // Skip processing
            }
            return "process"
        }).
        AddEdge("process", flowgraph.END).
        SetEntry("check")

    compiled, _ := graph.Compile()

    // Skip to END
    result, err := compiled.Run(ctx, State{Skip: true})
    require.NoError(t, err)
    assert.False(t, result.Processed)  // process node not called

    // Go through process
    result, err = compiled.Run(ctx, State{Skip: false})
    require.NoError(t, err)
    assert.True(t, result.Processed)
}
```

### Router Errors

```go
func TestConditionalEdge_EmptyStringError(t *testing.T) {
    router := func(ctx Context, s State) string {
        return ""  // Invalid
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("decide", decideNode).
        AddConditionalEdge("decide", router).
        SetEntry("decide")

    compiled, _ := graph.Compile()
    _, err := compiled.Run(ctx, State{})

    assert.ErrorIs(t, err, flowgraph.ErrInvalidRouterResult)
}

func TestConditionalEdge_UnknownNodeError(t *testing.T) {
    router := func(ctx Context, s State) string {
        return "nonexistent"
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("decide", decideNode).
        AddConditionalEdge("decide", router).
        SetEntry("decide")

    compiled, _ := graph.Compile()
    _, err := compiled.Run(ctx, State{})

    assert.ErrorIs(t, err, flowgraph.ErrRouterTargetNotFound)

    var routerErr *flowgraph.RouterError
    require.ErrorAs(t, err, &routerErr)
    assert.Equal(t, "decide", routerErr.FromNode)
    assert.Equal(t, "nonexistent", routerErr.Returned)
}
```

### Router Panic

```go
func TestConditionalEdge_RouterPanic(t *testing.T) {
    router := func(ctx Context, s State) string {
        panic("router failed")
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("decide", decideNode).
        AddConditionalEdge("decide", router).
        SetEntry("decide")

    compiled, _ := graph.Compile()
    _, err := compiled.Run(ctx, State{})

    var panicErr *flowgraph.PanicError
    require.ErrorAs(t, err, &panicErr)
    assert.Equal(t, "decide", panicErr.NodeID)  // From-node
    assert.Equal(t, "router failed", panicErr.Value)
}
```

### Multi-Way Branching

```go
func TestConditionalEdge_MultiWay(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("classify", classifyNode).
        AddNode("handle-a", handleANode).
        AddNode("handle-b", handleBNode).
        AddNode("handle-c", handleCNode).
        AddConditionalEdge("classify", func(ctx Context, s State) string {
            switch s.Type {
            case "A": return "handle-a"
            case "B": return "handle-b"
            default: return "handle-c"
            }
        }).
        AddEdge("handle-a", flowgraph.END).
        AddEdge("handle-b", flowgraph.END).
        AddEdge("handle-c", flowgraph.END).
        SetEntry("classify")

    compiled, _ := graph.Compile()

    tests := []struct{
        input string
        want  string
    }{
        {"A", "handled-by-a"},
        {"B", "handled-by-b"},
        {"C", "handled-by-c"},
        {"unknown", "handled-by-c"},
    }

    for _, tt := range tests {
        result, err := compiled.Run(ctx, State{Type: tt.input})
        require.NoError(t, err)
        assert.Equal(t, tt.want, result.Handler)
    }
}
```

### State Passed to Router

```go
func TestConditionalEdge_ReceivesNodeOutput(t *testing.T) {
    var routerState State

    node := func(ctx Context, s State) (State, error) {
        s.Modified = true  // Modify state
        return s, nil
    }

    router := func(ctx Context, s State) string {
        routerState = s  // Capture state
        return flowgraph.END
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("modify", node).
        AddConditionalEdge("modify", router).
        SetEntry("modify")

    compiled, _ := graph.Compile()
    compiled.Run(ctx, State{Modified: false})

    assert.True(t, routerState.Modified)  // Router received modified state
}
```

---

## Performance Requirements

| Operation | Target |
|-----------|--------|
| RouterFunc invocation overhead | < 100 nanoseconds |
| Edge resolution | O(1) map lookup |

---

## Security Considerations

1. **Router code trust**: RouterFunc is user code, executes with full permissions
2. **Panic recovery**: Router panics are caught, don't crash the executor

---

## Simplicity Check

**What we included**:
- Single `AddConditionalEdge()` method
- RouterFunc receives both Context and State
- Returns string (node ID or END)
- Runtime validation of return values

**What we did NOT include**:
- Multiple routers per node - One conditional edge is enough. Use node logic for complex decisions.
- Router registration by name - Inline functions are simpler.
- Typed router returns (enum) - String is flexible enough.
- Router with error return - Just panic on errors, keep signature simple.
- Default fallback node - Explicit routing is clearer.

**Is this the simplest solution?** Yes. One function determines the next node. Return a string.
