# Feature: Loop Execution

**Related ADRs**: 008-cycle-handling, 010-execution-model

---

## Problem Statement

Many workflows require loops:
- Retry until success
- Review/fix cycles until approved
- Process items until queue empty
- Iterate until convergence

flowgraph must handle cycles gracefully while preventing infinite loops.

## User Stories

- As a developer, I want to create review loops so that work iterates until approved
- As a developer, I want retry logic so that transient failures are handled
- As a developer, I want protection from infinite loops so that bugs don't hang my program
- As a developer, I want to know when max iterations hit so that I can investigate

---

## API Design

Loops are created using conditional edges that route back to earlier nodes:

```go
// Review loop: implement -> review -> (approved ? END : implement)
graph := flowgraph.NewGraph[WorkState]().
    AddNode("implement", implementNode).
    AddNode("review", reviewNode).
    AddEdge("implement", "review").
    AddConditionalEdge("review", func(ctx Context, s WorkState) string {
        if s.Approved {
            return flowgraph.END
        }
        return "implement"  // Loop back
    }).
    SetEntry("implement")
```

### Max Iterations Option

```go
// Configure maximum iterations to prevent infinite loops
func WithMaxIterations(n int) RunOption

// Usage
result, err := compiled.Run(ctx, state,
    flowgraph.WithMaxIterations(100))
```

---

## Behavior Specification

### Loop Detection (Compile Time)

Compile validates that all cycles have a conditional exit:

```
Valid Loop:
    A ──► B ──► C
    ▲         │
    │    conditional
    └─────────┘  (can return END)

Invalid Loop:
    A ──► B ──► C
    ▲         │
    │    unconditional
    └─────────┘  (no exit possible)
```

Algorithm:
1. Find all strongly connected components (SCCs)
2. For each SCC with > 1 node (a cycle):
   - At least one node must have a conditional edge
   - That conditional edge must be able to exit the SCC (go to END or outside SCC)

### Iteration Counting

Iterations are counted per-execution, not per-node:

```
Iteration 1: implement
Iteration 2: review
Iteration 3: implement  (loop)
Iteration 4: review
Iteration 5: END
```

Each node execution increments the counter.

### Max Iterations Behavior

When max iterations exceeded:
1. Stop execution immediately
2. Return `ErrMaxIterations`
3. Include state at that point
4. Include iteration count in error

```go
result, err := compiled.Run(ctx, state, flowgraph.WithMaxIterations(10))

if errors.Is(err, flowgraph.ErrMaxIterations) {
    // result.IterationCount == 10
    // result contains state after 10th iteration
}
```

### Self-Loops

A node can loop to itself:

```go
graph.AddNode("retry", retryNode).
    AddConditionalEdge("retry", func(ctx Context, s State) string {
        if s.Success || s.Attempts >= 3 {
            return flowgraph.END
        }
        return "retry"  // Self-loop
    }).
    SetEntry("retry")
```

---

## Error Cases

### ErrMaxIterations

```go
var ErrMaxIterations = errors.New("exceeded maximum iterations")

// MaxIterationsError provides context
type MaxIterationsError struct {
    Max        int     // The configured limit
    LastNodeID string  // Node that would have executed next
    State      any     // State at termination
}

func (e *MaxIterationsError) Error() string {
    return fmt.Sprintf("exceeded maximum iterations (%d) at node %s",
        e.Max, e.LastNodeID)
}

func (e *MaxIterationsError) Unwrap() error {
    return ErrMaxIterations
}
```

### ErrNoPathToEnd (Compile Time)

Compile rejects graphs where cycles have no exit:

```go
graph := flowgraph.NewGraph[State]().
    AddNode("a", nodeA).
    AddNode("b", nodeB).
    AddEdge("a", "b").
    AddEdge("b", "a").  // Unconditional loop!
    SetEntry("a")

_, err := graph.Compile()
assert.ErrorIs(t, err, flowgraph.ErrNoPathToEnd)
```

---

## Test Cases

### Valid Loops

```go
func TestLoop_ReviewCycle(t *testing.T) {
    attempts := 0

    reviewNode := func(ctx Context, s State) (State, error) {
        attempts++
        s.Approved = attempts >= 3  // Approve after 3 reviews
        return s, nil
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("implement", implementNode).
        AddNode("review", reviewNode).
        AddEdge("implement", "review").
        AddConditionalEdge("review", func(ctx Context, s State) string {
            if s.Approved { return flowgraph.END }
            return "implement"
        }).
        SetEntry("implement")

    compiled, _ := graph.Compile()
    result, err := compiled.Run(ctx, State{})

    require.NoError(t, err)
    assert.True(t, result.Approved)
    assert.Equal(t, 3, attempts)
}

func TestLoop_SelfLoop(t *testing.T) {
    count := 0

    retryNode := func(ctx Context, s State) (State, error) {
        count++
        s.Done = count >= 5
        return s, nil
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("retry", retryNode).
        AddConditionalEdge("retry", func(ctx Context, s State) string {
            if s.Done { return flowgraph.END }
            return "retry"
        }).
        SetEntry("retry")

    compiled, _ := graph.Compile()
    result, err := compiled.Run(ctx, State{})

    require.NoError(t, err)
    assert.True(t, result.Done)
    assert.Equal(t, 5, count)
}
```

### Max Iterations

```go
func TestLoop_MaxIterations_Exceeded(t *testing.T) {
    infiniteNode := func(ctx Context, s State) (State, error) {
        s.Count++
        return s, nil
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("loop", infiniteNode).
        AddConditionalEdge("loop", func(ctx Context, s State) string {
            return "loop"  // Never exits
        }).
        SetEntry("loop")

    compiled, _ := graph.Compile()
    result, err := compiled.Run(ctx, State{},
        flowgraph.WithMaxIterations(10))

    require.Error(t, err)
    assert.ErrorIs(t, err, flowgraph.ErrMaxIterations)
    assert.Equal(t, 10, result.Count)
}

func TestLoop_MaxIterations_NotExceeded(t *testing.T) {
    loopNode := func(ctx Context, s State) (State, error) {
        s.Count++
        return s, nil
    }

    router := func(ctx Context, s State) string {
        if s.Count >= 5 { return flowgraph.END }
        return "loop"
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("loop", loopNode).
        AddConditionalEdge("loop", router).
        SetEntry("loop")

    compiled, _ := graph.Compile()
    result, err := compiled.Run(ctx, State{},
        flowgraph.WithMaxIterations(100))

    require.NoError(t, err)  // Completed normally
    assert.Equal(t, 5, result.Count)
}

func TestLoop_MaxIterations_DefaultValue(t *testing.T) {
    // Default should be 1000
    count := 0
    loopNode := func(ctx Context, s State) (State, error) {
        count++
        return s, nil
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("loop", loopNode).
        AddConditionalEdge("loop", func(ctx Context, s State) string {
            return "loop"  // Never exits
        }).
        SetEntry("loop")

    compiled, _ := graph.Compile()
    _, err := compiled.Run(ctx, State{})  // No WithMaxIterations

    require.Error(t, err)
    assert.Equal(t, 1000, count)  // Default limit
}
```

### Compile-Time Validation

```go
func TestLoop_InvalidCycle_NoConditionalExit(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddNode("c", nodeC).
        AddEdge("a", "b").
        AddEdge("b", "c").
        AddEdge("c", "a").  // All unconditional - no exit!
        SetEntry("a")

    _, err := graph.Compile()

    assert.ErrorIs(t, err, flowgraph.ErrNoPathToEnd)
}

func TestLoop_ValidCycle_WithConditionalExit(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddNode("c", nodeC).
        AddEdge("a", "b").
        AddEdge("b", "c").
        AddConditionalEdge("c", func(ctx Context, s State) string {
            if s.Done { return flowgraph.END }
            return "a"
        }).
        SetEntry("a")

    compiled, err := graph.Compile()

    require.NoError(t, err)  // Valid - has conditional exit
    assert.NotNil(t, compiled)
}
```

### Complex Cycles

```go
func TestLoop_NestedCycles(t *testing.T) {
    // Outer: A -> B -> C -> A
    // Inner: B -> D -> B
    // With proper conditional exits

    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddNode("c", nodeC).
        AddNode("d", nodeD).
        AddEdge("a", "b").
        AddConditionalEdge("b", func(ctx Context, s State) string {
            if s.SkipInner { return "c" }
            return "d"
        }).
        AddEdge("d", "b").  // Inner loop back
        AddConditionalEdge("c", func(ctx Context, s State) string {
            if s.Done { return flowgraph.END }
            return "a"  // Outer loop back
        }).
        SetEntry("a")

    compiled, err := graph.Compile()
    require.NoError(t, err)
    // ... execution tests
}
```

### State Preservation in Loops

```go
func TestLoop_StateAccumulatesAcrossIterations(t *testing.T) {
    loopNode := func(ctx Context, s State) (State, error) {
        s.Values = append(s.Values, s.Iteration)
        s.Iteration++
        return s, nil
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("accumulate", loopNode).
        AddConditionalEdge("accumulate", func(ctx Context, s State) string {
            if s.Iteration >= 5 { return flowgraph.END }
            return "accumulate"
        }).
        SetEntry("accumulate")

    compiled, _ := graph.Compile()
    result, err := compiled.Run(ctx, State{})

    require.NoError(t, err)
    assert.Equal(t, []int{0, 1, 2, 3, 4}, result.Values)
}
```

---

## Performance Requirements

| Metric | Target |
|--------|--------|
| Iteration overhead | < 100 nanoseconds per loop |
| Cycle detection (compile) | O(V + E) |
| Max iterations check | O(1) per iteration |

---

## Security Considerations

1. **Resource exhaustion**: Max iterations prevents CPU exhaustion from infinite loops
2. **Memory growth**: User must manage state size; loops can accumulate data
3. **Timeout integration**: Combine with context timeouts for time-based limits

---

## Simplicity Check

**What we included**:
- Loops via conditional edges pointing backward
- Max iterations with configurable limit
- Compile-time validation for exit paths
- Clear error when limit exceeded

**What we did NOT include**:
- `break` node or edge - Use conditional edge to END instead
- Loop counters in framework - User tracks in state if needed
- Per-loop limits - One global max is simpler
- Automatic retry logic - User implements in nodes
- Loop detection warnings - Either it's valid (has exit) or invalid (error)

**Is this the simplest solution?** Yes. Loops are just edges that point backward. Max iterations is the only safety mechanism needed.
