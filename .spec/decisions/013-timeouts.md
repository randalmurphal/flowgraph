# ADR-013: Timeout Handling

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should timeouts be configured and enforced? Options:
- Graph-level timeout
- Per-node timeout
- Both
- Rely entirely on context

## Decision

**Graph-level timeout via context + optional per-node timeout configuration.**

### Graph-Level Timeout (Primary)

Users wrap context with timeout:

```go
ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
defer cancel()
result, err := compiled.Run(ctx, state)
```

### Per-Node Timeout (Optional)

```go
// Builder API
graph.AddNode("llm-call", llmNode, flowgraph.WithNodeTimeout(2*time.Minute))

// Or with functional options on node
graph.AddNode("llm-call", flowgraph.TimeoutNode(llmNode, 2*time.Minute))
```

### Implementation

```go
type nodeConfig struct {
    fn      NodeFunc[S]
    timeout time.Duration  // 0 means no timeout (use context)
}

func (cg *CompiledGraph[S]) executeNode(ctx Context, nodeID string, state S) (S, error) {
    cfg := cg.nodeConfigs[nodeID]

    // Apply per-node timeout if configured
    if cfg.timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
        defer cancel()
    }

    return cg.runWithRecovery(ctx, nodeID, cfg.fn, state)
}
```

### Timeout Behavior

| Scenario | Behavior |
|----------|----------|
| Graph timeout expires | CancellationError, DeadlineExceeded |
| Node timeout expires | NodeError wrapping DeadlineExceeded |
| Both set, node is shorter | Node timeout applies |
| Both set, graph is shorter | Graph timeout applies (node inherits) |

## Alternatives Considered

### 1. Per-Node Timeout Only

```go
graph.AddNode("a", nodeA, Timeout(1*time.Minute))
graph.AddNode("b", nodeB, Timeout(2*time.Minute))
// No graph-level timeout
```

**Rejected**: Forces every node to have timeout. Graph-level is simpler for most cases.

### 2. Timeout in Node Function

```go
func llmNode(ctx Context, state State) (State, error) {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
    defer cancel()
    // ...
}
```

**Rejected**: Mixes configuration with logic. Timeout should be configurable externally.

### 3. Separate Timeout Type

```go
graph.SetNodeTimeout("llm-call", 2*time.Minute)
```

**Rejected**: Separated from AddNode, easy to forget. Keep configuration together.

### 4. Retry With Timeout

```go
graph.AddNode("llm", llmNode,
    Timeout(2*time.Minute),
    Retry(3, ExponentialBackoff),
)
```

**Rejected for v1**: Retry is a separate concern. Can be composed with timeout later.

## Consequences

### Positive
- **Flexible** - Use context timeout or per-node, or both
- **Go-idiomatic** - Context is the timeout mechanism
- **Composable** - Timeouts nest naturally
- **Explicit** - Clear where timeouts are configured

### Negative
- Two places to set timeouts could be confusing
- Per-node timeout requires additional configuration

### Risks
- Timeout too short → Document: test timeouts in staging
- Timeout too long → Document: monitor execution time, adjust

---

## Recommended Timeout Strategy

### For LLM Workflows

| Component | Recommended Timeout |
|-----------|-------------------|
| Overall workflow | 15-30 minutes |
| Single LLM call | 2-5 minutes |
| File I/O operations | 30 seconds |
| Git operations | 1-2 minutes |

### Example Configuration

```go
compiled, _ := flowgraph.NewGraph[State]().
    AddNode("parse", parseNode).  // No timeout, fast
    AddNode("generate-spec", specNode,
        flowgraph.WithNodeTimeout(3*time.Minute)).
    AddNode("implement", implementNode,
        flowgraph.WithNodeTimeout(5*time.Minute)).
    AddNode("review", reviewNode,
        flowgraph.WithNodeTimeout(3*time.Minute)).
    AddNode("commit", commitNode,
        flowgraph.WithNodeTimeout(1*time.Minute)).
    // edges...
    Compile()

// Overall workflow timeout
ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
defer cancel()

result, err := compiled.Run(ctx, state)
```

---

## Usage Examples

### Graph-Level Timeout Only (Simple)

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()

result, err := compiled.Run(ctx, state)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Workflow timed out")
}
```

### Per-Node Timeout

```go
graph := flowgraph.NewGraph[State]().
    AddNode("fast", fastNode).  // Inherits context timeout
    AddNode("slow", slowNode, flowgraph.WithNodeTimeout(5*time.Minute))
```

### Handling Timeout Errors

```go
result, err := compiled.Run(ctx, state)
if err != nil {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        // Could be graph-level or node-level timeout
        var ne *flowgraph.NodeError
        if errors.As(err, &ne) {
            log.Printf("Node %s timed out", ne.NodeID)
        } else {
            log.Println("Graph execution timed out")
        }

    case errors.Is(err, context.Canceled):
        log.Println("Execution was cancelled")

    default:
        log.Printf("Execution failed: %v", err)
    }
}
```

### Dynamic Timeout Based on Input

```go
// Adjust timeout based on input size
timeout := time.Duration(len(state.Items)) * time.Minute
if timeout > 30*time.Minute {
    timeout = 30 * time.Minute
}

ctx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()

result, err := compiled.Run(ctx, state)
```

---

## Test Cases

```go
func TestTimeout_GraphLevel(t *testing.T) {
    slowNode := func(ctx Context, s testState) (testState, error) {
        time.Sleep(time.Hour)  // Would block forever
        return s, nil
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("slow", slowNode).
        AddEdge("slow", flowgraph.END).
        SetEntry("slow").
        Compile()

    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    start := time.Now()
    _, err := compiled.Run(ctx, testState{})
    elapsed := time.Since(start)

    require.Error(t, err)
    assert.True(t, errors.Is(err, context.DeadlineExceeded))
    assert.Less(t, elapsed, 200*time.Millisecond)  // Didn't wait for Hour
}

func TestTimeout_PerNode(t *testing.T) {
    slowNode := func(ctx Context, s testState) (testState, error) {
        <-ctx.Done()  // Waits for context
        return s, ctx.Err()
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("slow", slowNode, flowgraph.WithNodeTimeout(50*time.Millisecond)).
        AddEdge("slow", flowgraph.END).
        SetEntry("slow").
        Compile()

    // No graph timeout
    start := time.Now()
    _, err := compiled.Run(context.Background(), testState{})
    elapsed := time.Since(start)

    require.Error(t, err)
    assert.True(t, errors.Is(err, context.DeadlineExceeded))
    assert.Less(t, elapsed, 200*time.Millisecond)

    // Verify it's a NodeError
    var ne *flowgraph.NodeError
    require.True(t, errors.As(err, &ne))
    assert.Equal(t, "slow", ne.NodeID)
}

func TestTimeout_NodeShorterThanGraph(t *testing.T) {
    var nodeTimedOut bool
    slowNode := func(ctx Context, s testState) (testState, error) {
        select {
        case <-ctx.Done():
            nodeTimedOut = true
            return s, ctx.Err()
        case <-time.After(time.Hour):
            return s, nil
        }
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("slow", slowNode, flowgraph.WithNodeTimeout(50*time.Millisecond)).
        AddEdge("slow", flowgraph.END).
        SetEntry("slow").
        Compile()

    // Graph timeout is much longer
    ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
    defer cancel()

    _, err := compiled.Run(ctx, testState{})

    require.Error(t, err)
    assert.True(t, nodeTimedOut)  // Node's timeout triggered, not graph's
}
```

---

## Timeout Monitoring

```go
// Emit metrics for timeout monitoring
func (cg *CompiledGraph[S]) executeNode(ctx Context, nodeID string, state S) (S, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        metrics.NodeDuration.WithLabelValues(nodeID).Observe(duration.Seconds())

        cfg := cg.nodeConfigs[nodeID]
        if cfg.timeout > 0 {
            utilization := float64(duration) / float64(cfg.timeout)
            metrics.TimeoutUtilization.WithLabelValues(nodeID).Observe(utilization)

            if utilization > 0.8 {
                ctx.Logger().Warn("node approaching timeout",
                    "node_id", nodeID,
                    "duration", duration,
                    "timeout", cfg.timeout,
                    "utilization_pct", utilization*100,
                )
            }
        }
    }()

    return cg.runWithRecovery(ctx, nodeID, state)
}
```
