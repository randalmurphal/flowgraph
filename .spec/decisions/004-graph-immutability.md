# ADR-004: Graph Immutability

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

Should `Graph[S]` be mutable (builder pattern mutates self) or immutable (builder pattern returns new instance)?

## Decision

**Mutable builder, immutable after compilation.**

```go
// Builder phase - mutable, chainable
graph := flowgraph.NewGraph[MyState]().
    AddNode("a", nodeA).
    AddNode("b", nodeB).
    AddEdge("a", "b").
    SetEntry("a")

// After compilation - immutable
compiled, err := graph.Compile()
// compiled cannot be modified
// graph can still be modified (for reuse)
```

### Design

```go
type Graph[S any] struct {
    nodes       map[string]NodeFunc[S]
    edges       map[string][]string
    conditional map[string]RouterFunc[S]
    entryPoint  string
    mu          sync.RWMutex  // Thread-safe building
}

// Builder methods modify and return self for chaining
func (g *Graph[S]) AddNode(id string, fn NodeFunc[S]) *Graph[S] {
    g.mu.Lock()
    defer g.mu.Unlock()
    g.nodes[id] = fn
    return g
}

// Compile creates an immutable execution plan
func (g *Graph[S]) Compile() (*CompiledGraph[S], error) {
    g.mu.RLock()
    defer g.mu.RUnlock()

    // Deep copy all data
    nodes := make(map[string]NodeFunc[S], len(g.nodes))
    for k, v := range g.nodes {
        nodes[k] = v
    }
    // ... copy edges, conditional, etc.

    // Validate and build execution plan
    return &CompiledGraph[S]{
        nodes:      nodes,
        edges:      edges,
        entryPoint: g.entryPoint,
        // execution plan computed here
    }, nil
}

// CompiledGraph is immutable
type CompiledGraph[S any] struct {
    nodes      map[string]NodeFunc[S]  // private, no setters
    edges      map[string][]string
    entryPoint string
    // computed execution order, validated references, etc.
}
```

## Alternatives Considered

### 1. Fully Immutable Builder

```go
graph := flowgraph.NewGraph[MyState]()
graph = graph.AddNode("a", nodeA)  // Returns new graph
graph = graph.AddNode("b", nodeB)
```

**Rejected**: Awkward API, lots of allocations, no benefit since graphs are typically built in a single goroutine.

### 2. Fully Mutable (No Compile)

```go
graph := flowgraph.NewGraph[MyState]()
graph.AddNode("a", nodeA)
graph.Run(ctx, state)  // Validates on each run
```

**Rejected**: Validation on every run is wasteful; errors caught at compile time are better.

### 3. Frozen Builder

```go
graph.Freeze()  // After this, modifications panic
```

**Rejected**: Runtime panics are worse than compile-time type safety.

## Consequences

### Positive
- Familiar builder pattern (chainable methods)
- Type safety: CompiledGraph has no mutation methods
- Thread-safe: Can build graph while another goroutine runs a compiled version
- Validation happens once at compile time

### Negative
- Two types to understand (Graph, CompiledGraph)
- Must call Compile() before Run()

### Risks
- User forgets to Compile() → Mitigate: Graph has no Run() method

---

## API Examples

### Basic Usage
```go
compiled, err := flowgraph.NewGraph[State]().
    AddNode("start", startNode).
    AddNode("end", endNode).
    AddEdge("start", "end").
    SetEntry("start").
    Compile()
if err != nil {
    log.Fatal(err)
}

result, err := compiled.Run(ctx, initialState)
```

### Reusing Builder
```go
// Base graph template
base := flowgraph.NewGraph[State]().
    AddNode("start", startNode).
    AddNode("end", endNode)

// Variant A
variantA, _ := base.Clone().
    AddNode("middle", middlewareA).
    AddEdge("start", "middle").
    AddEdge("middle", "end").
    SetEntry("start").
    Compile()

// Variant B
variantB, _ := base.Clone().
    AddNode("middle", middlewareB).
    AddEdge("start", "middle").
    AddEdge("middle", "end").
    SetEntry("start").
    Compile()
```

### Thread Safety
```go
// Safe: Build and run concurrently
var wg sync.WaitGroup

wg.Add(1)
go func() {
    defer wg.Done()
    graph.AddNode("extra", extraNode)  // Safe: mutex protected
}()

wg.Add(1)
go func() {
    defer wg.Done()
    compiled.Run(ctx, state)  // Safe: CompiledGraph is immutable
}()

wg.Wait()
```
