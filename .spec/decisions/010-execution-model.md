# ADR-010: Execution Model

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should the graph executor work? Options include:
- Synchronous (blocking)
- Asynchronous (goroutines per node)
- Streaming (channel-based)
- Hybrid

## Decision

**Synchronous execution with optional parallelism for fan-out/fan-in (v2).**

### V1: Synchronous Execution

```go
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error) {
    current := cg.entryPoint

    for current != END {
        // Check cancellation
        select {
        case <-ctx.Done():
            return state, ctx.Err()
        default:
        }

        // Get node function
        fn, ok := cg.nodes[current]
        if !ok {
            return state, fmt.Errorf("node not found: %s", current)
        }

        // Execute node
        nodeCtx := cg.contextForNode(ctx, current)
        var err error
        state, err = fn(nodeCtx, state)
        if err != nil {
            return state, &NodeError{NodeID: current, Err: err}
        }

        // Determine next node
        current, err = cg.nextNode(ctx, state, current)
        if err != nil {
            return state, err
        }
    }

    return state, nil
}

func (cg *CompiledGraph[S]) nextNode(ctx Context, state S, current string) (string, error) {
    // Check conditional edge first
    if router, ok := cg.conditionalEdges[current]; ok {
        return router(ctx, state), nil
    }

    // Simple edge
    successors := cg.successors[current]
    if len(successors) == 0 {
        return "", fmt.Errorf("no outgoing edge from node: %s", current)
    }
    if len(successors) > 1 {
        return "", fmt.Errorf("multiple outgoing edges without router from: %s", current)
    }
    return successors[0], nil
}
```

### V2 (Future): Parallel Fan-Out

```go
// Future API for parallel execution
func (cg *CompiledGraph[S]) AddParallelEdge(from string, to ...string) *CompiledGraph[S]

// Execution would fork goroutines and join
// state.Results = []Result from each parallel path
```

## Alternatives Considered

### 1. Full Async (Goroutine Per Node)

```go
func (cg *CompiledGraph[S]) Run(ctx Context, state S) <-chan Result[S] {
    result := make(chan Result[S], 1)
    go func() {
        // Execute async
    }()
    return result
}
```

**Rejected for v1**: Adds complexity. Most LLM workflows are sequential anyway.

### 2. Actor Model

```go
type NodeActor[S any] struct {
    inbox  chan S
    outbox chan S
}
```

**Rejected**: Overkill for typical graph sizes (<20 nodes).

### 3. Event-Driven

```go
func (cg *CompiledGraph[S]) Run(ctx Context, state S) {
    for event := range eventStream {
        // React to events
    }
}
```

**Rejected**: Wrong abstraction for request/response LLM workflows.

## Consequences

### Positive
- **Simple** - Easy to understand, debug, trace
- **Predictable** - Execution order matches graph structure
- **Debuggable** - Single goroutine, standard stack traces
- **Compatible** - Works with context cancellation naturally

### Negative
- No parallel execution in v1
- Long-running nodes block entire execution

### Risks
- User wants parallelism → Mitigate: Document v2 plans, provide workarounds

---

## Execution Flow

```
Run(ctx, initialState)
    │
    ▼
┌─────────────┐
│ entryPoint  │
└─────────────┘
    │
    ▼
┌─────────────────────────────────────────────┐
│ Loop until current == END                    │
│ ┌─────────────────────────────────────────┐ │
│ │ 1. Check ctx.Done()                     │ │
│ │ 2. Execute nodes[current](ctx, state)  │ │
│ │ 3. Handle error → return with NodeError │ │
│ │ 4. Determine next node                  │ │
│ │    - If conditional: call router        │ │
│ │    - If simple: follow edge             │ │
│ │ 5. current = next                       │ │
│ └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────┘
    │
    ▼
Return (finalState, nil)
```

---

## Run Options

```go
type RunOption func(*runConfig)

type runConfig struct {
    maxIterations   int
    onNodeStart     func(nodeID string, state any)
    onNodeComplete  func(nodeID string, state any, err error)
    checkpointStore CheckpointStore
    resumeFromNode  string
}

func WithMaxIterations(n int) RunOption {
    return func(c *runConfig) { c.maxIterations = n }
}

func WithNodeHooks(start, complete func(string, any)) RunOption {
    return func(c *runConfig) {
        c.onNodeStart = start
        c.onNodeComplete = complete
    }
}

func WithCheckpointing(store CheckpointStore) RunOption {
    return func(c *runConfig) { c.checkpointStore = store }
}

func WithResume(nodeID string) RunOption {
    return func(c *runConfig) { c.resumeFromNode = nodeID }
}
```

---

## Usage Examples

### Basic Execution
```go
compiled, _ := graph.Compile()

result, err := compiled.Run(ctx, State{Input: "hello"})
if err != nil {
    log.Fatalf("execution failed: %v", err)
}
fmt.Printf("Output: %s\n", result.Output)
```

### With Timeout
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

result, err := compiled.Run(ctx, initialState)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("execution timed out")
}
```

### With Progress Hooks
```go
result, err := compiled.Run(ctx, initialState,
    flowgraph.WithNodeHooks(
        func(nodeID string, state any) {
            log.Printf("Starting node: %s", nodeID)
        },
        func(nodeID string, state any, err error) {
            if err != nil {
                log.Printf("Node %s failed: %v", nodeID, err)
            } else {
                log.Printf("Node %s completed", nodeID)
            }
        },
    ),
)
```

### With Iteration Limit
```go
result, err := compiled.Run(ctx, initialState,
    flowgraph.WithMaxIterations(100),
)
if errors.Is(err, flowgraph.ErrMaxIterations) {
    log.Println("graph hit iteration limit - possible infinite loop")
}
```

---

## Test Cases

```go
func TestExecution_LinearFlow(t *testing.T) {
    var execOrder []string

    makeNode := func(name string) NodeFunc[testState] {
        return func(ctx Context, s testState) (testState, error) {
            execOrder = append(execOrder, name)
            return s, nil
        }
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("a", makeNode("a")).
        AddNode("b", makeNode("b")).
        AddNode("c", makeNode("c")).
        AddEdge("a", "b").
        AddEdge("b", "c").
        AddEdge("c", flowgraph.END).
        SetEntry("a").
        Compile()

    _, err := compiled.Run(context.Background(), testState{})

    require.NoError(t, err)
    assert.Equal(t, []string{"a", "b", "c"}, execOrder)
}

func TestExecution_ContextCancellation(t *testing.T) {
    slowNode := func(ctx Context, s testState) (testState, error) {
        select {
        case <-ctx.Done():
            return s, ctx.Err()
        case <-time.After(time.Hour):
            return s, nil
        }
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("slow", slowNode).
        AddEdge("slow", flowgraph.END).
        SetEntry("slow").
        Compile()

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
    defer cancel()

    _, err := compiled.Run(ctx, testState{})

    require.Error(t, err)
    assert.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestExecution_ConditionalBranch(t *testing.T) {
    var path []string

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("start", func(ctx Context, s testState) (testState, error) {
            path = append(path, "start")
            return s, nil
        }).
        AddNode("left", func(ctx Context, s testState) (testState, error) {
            path = append(path, "left")
            return s, nil
        }).
        AddNode("right", func(ctx Context, s testState) (testState, error) {
            path = append(path, "right")
            return s, nil
        }).
        AddConditionalEdge("start", func(ctx Context, s testState) string {
            if s.GoLeft {
                return "left"
            }
            return "right"
        }).
        AddEdge("left", flowgraph.END).
        AddEdge("right", flowgraph.END).
        SetEntry("start").
        Compile()

    // Test left path
    path = nil
    _, _ = compiled.Run(context.Background(), testState{GoLeft: true})
    assert.Equal(t, []string{"start", "left"}, path)

    // Test right path
    path = nil
    _, _ = compiled.Run(context.Background(), testState{GoLeft: false})
    assert.Equal(t, []string{"start", "right"}, path)
}
```
