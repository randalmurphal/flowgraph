# Open Questions Resolution

**Date**: 2025-12-19
**Status**: Resolved

This document resolves the open questions from PLANNING.md with clear decisions and rationale.

---

## 1. Parallel Node Execution

**Question**: Should v1 include fan-out/fan-in parallel execution?

### Decision: Deferred to v2

**Rationale**:
1. **Complexity cost**: Parallel execution requires:
   - Fan-out semantics (which nodes run in parallel)
   - Fan-in semantics (how to merge results)
   - Partial failure handling (one parallel node fails, others succeed)
   - State merging (combining outputs from parallel nodes)
   - Checkpoint complexity (checkpoint per parallel branch)

2. **v1 scope**: flowgraph v1 is about establishing the core patterns:
   - Graph construction
   - Sequential execution
   - Conditional branching
   - Checkpointing

3. **Sequential is sufficient**: Most LLM workflows are naturally sequential:
   - Fetch ticket → Generate spec → Implement → Review → Create PR
   - Parallel operations can be done inside a single node if needed

**What we're NOT doing**:
- `AddParallelNodes([]string)` method
- Fan-out edges
- State merging logic
- Parallel checkpoint coordination

**What v2 would need**:
- Clear API for defining parallel sections: `graph.Parallel("branch1", "branch2")`
- State merging strategy: last-write-wins, merge function, or accumulator
- Partial failure policy: fail-all, continue, timeout
- Checkpoint strategy: per-branch or wait-for-all
- OpenTelemetry span handling for parallel children

---

## 2. Sub-graphs

**Question**: Can a node execute another CompiledGraph?

### Decision: Yes, but manually composed

**Rationale**:
1. **Nodes can call anything**: A node is just a function. It can:
   - Call another compiled graph's Run() method
   - Pass state transformation
   - Handle errors
   - This requires no framework support

2. **No special API needed**: Users compose naturally:

```go
func subFlowNode(ctx flowgraph.Context, s ParentState) (ParentState, error) {
    // Transform parent state to child state
    childState := ChildState{Input: s.Input}

    // Run sub-graph
    result, err := childGraph.Run(ctx, childState)
    if err != nil {
        return s, fmt.Errorf("sub-flow: %w", err)
    }

    // Transform result back
    s.SubResult = result.Output
    return s, nil
}
```

3. **Keep it simple**: Framework-level composition adds:
   - State type conversion complexity
   - Checkpoint coordination
   - Error attribution
   - Trace nesting

**What we're NOT doing**:
- `AddSubGraph(id string, graph *CompiledGraph[S])` method
- Automatic state conversion
- Cross-graph checkpointing
- Graph nesting at compile time

**What we ARE doing**:
- Documenting the pattern above in examples
- Ensuring CompiledGraph.Run() is safe to call from within a node

---

## 3. Dynamic Graphs

**Question**: Can edges be added after Compile()?

### Decision: No. Graphs are immutable after compilation.

**Rationale**:
1. **Immutability enables optimization**: CompiledGraph can pre-compute:
   - Successors/predecessors maps
   - Cycle detection
   - Unreachable node warnings
   - These would need recalculation on mutation

2. **Immutability enables safety**: CompiledGraph is thread-safe because it never changes. Mutation would require synchronization.

3. **Use case is unclear**: Why would edges change at runtime?
   - If it's data-dependent routing → Use conditional edges
   - If it's configuration-dependent → Build different graphs
   - If it's user-defined → Build graph from configuration

4. **Build multiple graphs instead**:

```go
// Instead of modifying at runtime
if config.UseOptimization {
    graph.AddEdge("optimize", "finish")
}

// Build the right graph
func buildGraph(config Config) *CompiledGraph[State] {
    graph := flowgraph.NewGraph[State]()
    // ... build based on config
    compiled, _ := graph.Compile()
    return compiled
}
```

**What we're NOT doing**:
- `CompiledGraph.AddEdge()` method
- `CompiledGraph.RemoveNode()` method
- Runtime graph mutation
- Graph cloning with modifications

**What we ARE doing**:
- Clear documentation that CompiledGraph is immutable
- Examples of building different graphs from configuration

---

## 4. State Versioning

**Question**: How to handle checkpoint schema changes when state struct changes?

### Decision: User responsibility with simple helpers

**Rationale**:
1. **State schema is user-defined**: flowgraph doesn't know what fields exist or what changes mean. Only the user can define migration logic.

2. **JSON is forgiving**: By default:
   - New fields get zero values
   - Removed fields are ignored
   - This handles simple additions/removals

3. **Complex migrations need explicit handling**:

```go
// User defines migration in state override
result, err := compiled.Resume(ctx, store, runID,
    flowgraph.WithStateOverride(func(s State) State {
        // Handle old schema
        if s.SchemaVersion < 2 {
            s.NewField = computeFromOld(s.OldField)
            s.SchemaVersion = 2
        }
        return s
    }))
```

4. **Embed version in state**:

```go
type State struct {
    SchemaVersion int `json:"schema_version"`
    // ... fields
}
```

**What we're NOT doing**:
- Automatic schema migration
- Schema version tracking in checkpoint metadata
- Migration registration at compile time
- Breaking change detection

**What we ARE doing**:
- Documenting the `SchemaVersion` pattern
- Ensuring `WithStateOverride` is called before validation
- JSON's forgiving nature handles simple changes

---

## 5. Error Retry

**Question**: Should flowgraph have built-in retry?

### Decision: No built-in retry. User implements via patterns.

**Rationale**:
1. **Retry policy is domain-specific**:
   - Which errors are retryable?
   - How many retries?
   - What backoff strategy?
   - Should state be reset or preserved?

2. **Conditional edges handle retry**:

```go
graph.AddConditionalEdge("call-api", func(ctx Context, s State) string {
    if s.LastError != nil && s.Retries < 3 {
        return "call-api"  // Retry
    }
    if s.LastError != nil {
        return "handle-failure"
    }
    return "process-result"
})
```

3. **Nodes can implement retry internally**:

```go
func callAPINode(ctx flowgraph.Context, s State) (State, error) {
    var resp *Response
    var err error

    for attempt := 1; attempt <= 3; attempt++ {
        resp, err = api.Call(ctx, s.Request)
        if err == nil {
            break
        }
        if !isRetryable(err) {
            return s, err
        }
        time.Sleep(backoff(attempt))
    }

    s.Response = resp
    return s, err
}
```

4. **Keep framework simple**: Adding retry adds:
   - RetryPolicy type
   - Per-node retry configuration
   - Backoff strategies
   - Retry counting in context
   - This is a lot of API surface for something users can do themselves

**What we're NOT doing**:
- `WithRetry(policy)` option
- `AddNode(...).WithRetries(3)` configuration
- Built-in backoff strategies
- Retry state in Context

**What we ARE doing**:
- Documenting retry patterns with conditional edges
- Documenting internal retry in nodes
- Providing `Attempt()` in Context for manual tracking

---

## Summary

| Question | Decision | Reason |
|----------|----------|--------|
| Parallel execution | Deferred to v2 | Complexity, v1 is about core patterns |
| Sub-graphs | Manual composition | Nodes can call Run(), no special API needed |
| Dynamic graphs | No | Immutability enables optimization and safety |
| State versioning | User responsibility | Domain-specific, JSON is forgiving |
| Error retry | No built-in | Domain-specific, conditional edges handle it |

**Philosophy**: flowgraph provides the minimal core. Users compose patterns on top.
