# ADR-007: Validation Timing

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

When should graph validity be checked? Options:
1. **Build time** - When AddNode/AddEdge called
2. **Compile time** - When Compile() called
3. **Runtime** - When Run() called
4. **Progressive** - Different checks at different times

## Decision

**Progressive validation with compile-time as primary:**

| Check | When | Fails |
|-------|------|-------|
| Node ID format | AddNode | Immediately |
| Duplicate node | AddNode | Immediately |
| Edge source exists | Compile | At compile |
| Edge target exists | Compile | At compile |
| Entry point set | Compile | At compile |
| Entry point exists | Compile | At compile |
| Path to END exists | Compile | At compile |
| Unreachable nodes | Compile | Warning only |
| State serializable | Checkpoint save | At runtime |

### Build-Time Validation

Fast-fail on obvious errors:

```go
func (g *Graph[S]) AddNode(id string, fn NodeFunc[S]) *Graph[S] {
    // Immediate validation
    if id == "" {
        panic("node ID cannot be empty")
    }
    if id == END {
        panic("node ID cannot be reserved word 'END'")
    }
    if strings.ContainsAny(id, " \t\n") {
        panic("node ID cannot contain whitespace")
    }
    if _, exists := g.nodes[id]; exists {
        panic(fmt.Sprintf("duplicate node ID: %s", id))
    }
    if fn == nil {
        panic("node function cannot be nil")
    }

    g.nodes[id] = fn
    return g
}
```

**Why panic?** Build-time errors are programmer errors, not runtime conditions. Panics give stack traces.

### Compile-Time Validation

Comprehensive graph structure validation:

```go
func (g *Graph[S]) Compile() (*CompiledGraph[S], error) {
    var errs []error

    // Entry point validation
    if g.entryPoint == "" {
        errs = append(errs, ErrNoEntryPoint)
    } else if _, exists := g.nodes[g.entryPoint]; !exists {
        errs = append(errs, fmt.Errorf("%w: %s", ErrEntryNotFound, g.entryPoint))
    }

    // Edge validation
    for from, targets := range g.edges {
        if _, exists := g.nodes[from]; !exists {
            errs = append(errs, fmt.Errorf("edge source '%s' does not exist", from))
        }
        for _, to := range targets {
            if to != END {
                if _, exists := g.nodes[to]; !exists {
                    errs = append(errs, fmt.Errorf("edge target '%s' does not exist", to))
                }
            }
        }
    }

    // Path to END validation
    if !g.hasPathToEnd() {
        errs = append(errs, ErrNoPathToEnd)
    }

    // Unreachable node detection (warning)
    unreachable := g.findUnreachableNodes()
    for _, id := range unreachable {
        slog.Warn("unreachable node detected", "node_id", id)
    }

    if len(errs) > 0 {
        return nil, errors.Join(errs...)
    }

    return g.buildCompiledGraph()
}
```

**Why errors, not panic?** Compile errors may come from dynamic graph construction; they're recoverable.

### Runtime Validation

Minimal - trust compile-time:

```go
func (cg *CompiledGraph[S]) Run(ctx Context, state S) (S, error) {
    // Only runtime checks:
    // 1. Context not nil
    // 2. State serializable (if checkpointing enabled)

    if ctx == nil {
        return state, ErrNilContext
    }

    // All structural validation done at compile time
    return cg.execute(ctx, state)
}
```

## Alternatives Considered

### 1. All Validation at Build Time

```go
graph.AddEdge("a", "b")  // Panics if "a" or "b" don't exist yet
```

**Rejected**: Forces declaration order. User would need to add all nodes before any edges.

### 2. All Validation at Runtime

```go
compiled.Run(ctx, state)  // Validates structure here
```

**Rejected**: Wasteful to validate on every run. Errors should be caught early.

### 3. Deferred Panic (Build Time)

```go
graph.AddEdge("a", "b")  // Stores error
graph.Compile()          // Panics with all accumulated errors
```

**Rejected**: Panics should be immediate or not at all.

## Consequences

### Positive
- **Fast feedback** - Obvious errors caught immediately
- **Good error messages** - All compile errors collected
- **Runtime efficiency** - No validation overhead in hot path
- **IDE friendly** - Panics in builder show in call stack

### Negative
- Two error mechanisms (panic, error) to understand
- Must remember to call Compile()

### Risks
- User confused by panic vs error → Mitigate: Clear documentation

---

## Error Messages

### Build-Time (Panic)
```
panic: duplicate node ID: process

goroutine 1 [running]:
flowgraph.(*Graph[...]).AddNode(...)
    /path/to/graph.go:45
main.buildGraph(...)
    /path/to/main.go:23
```

### Compile-Time (Error)
```
graph compilation failed:
  - edge target 'review' does not exist
  - edge target 'submit' does not exist
  - no path to END from entry point 'start'
```

### Runtime (Error)
```
execution failed at node 'process': context canceled
```

---

## Validation Implementation

```go
// hasPathToEnd uses BFS to verify at least one path terminates
func (g *Graph[S]) hasPathToEnd() bool {
    if g.entryPoint == "" {
        return false
    }

    visited := make(map[string]bool)
    queue := []string{g.entryPoint}

    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]

        if visited[current] {
            continue
        }
        visited[current] = true

        // Check conditional edges first
        if router, ok := g.conditionalEdges[current]; ok {
            // For conditional edges, we can't statically verify all paths
            // We assume router can return END (documented requirement)
            return true  // Optimistic: conditional edge might go to END
        }

        // Check simple edges
        for _, target := range g.edges[current] {
            if target == END {
                return true
            }
            queue = append(queue, target)
        }
    }

    return false
}

// findUnreachableNodes returns nodes not reachable from entry
func (g *Graph[S]) findUnreachableNodes() []string {
    reachable := make(map[string]bool)
    g.walkFromEntry(g.entryPoint, reachable)

    var unreachable []string
    for id := range g.nodes {
        if !reachable[id] {
            unreachable = append(unreachable, id)
        }
    }
    return unreachable
}
```
