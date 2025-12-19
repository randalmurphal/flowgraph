# ADR-009: Compilation Output

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

What should `Compile()` return? Just a validated graph reference, or a pre-computed execution plan?

## Decision

**Return a `CompiledGraph` with pre-computed execution metadata.**

```go
type CompiledGraph[S any] struct {
    // Immutable copies from Graph
    nodes           map[string]NodeFunc[S]
    edges           map[string][]string
    conditionalEdges map[string]RouterFunc[S]
    entryPoint      string

    // Pre-computed execution metadata
    nodeOrder       []string              // Topological order (for linear paths)
    successors      map[string][]string   // Quick lookup: node → possible next nodes
    predecessors    map[string][]string   // Quick lookup: node → previous nodes
    isConditional   map[string]bool       // Quick check: does this node have conditional edge?
    hasCycle        bool                  // Does graph contain any cycle?
    reachable       map[string]bool       // Nodes reachable from entry
}

func (g *Graph[S]) Compile() (*CompiledGraph[S], error) {
    if err := g.validate(); err != nil {
        return nil, err
    }

    return &CompiledGraph[S]{
        nodes:           copyNodes(g.nodes),
        edges:           copyEdges(g.edges),
        conditionalEdges: copyConditional(g.conditionalEdges),
        entryPoint:      g.entryPoint,

        // Pre-compute for O(1) runtime lookups
        nodeOrder:     computeTopologicalOrder(g),
        successors:    computeSuccessors(g),
        predecessors:  computePredecessors(g),
        isConditional: computeConditionalMap(g),
        hasCycle:      detectCycles(g),
        reachable:     computeReachable(g),
    }, nil
}
```

### Benefits of Pre-Computation

| Metadata | Runtime Use | Compile Cost |
|----------|-------------|--------------|
| successors | O(1) next node lookup | O(E) once |
| predecessors | Resume: find previous checkpoint | O(E) once |
| isConditional | Skip edge lookup if no conditional | O(N) once |
| hasCycle | Optimization hints | O(N+E) once |
| reachable | Debugging, visualization | O(N+E) once |

## Alternatives Considered

### 1. Return Validated Graph Reference

```go
func (g *Graph[S]) Compile() error {
    return g.validate()  // Just validate, no new struct
}

func (g *Graph[S]) Run(ctx Context, state S) (S, error) {
    // Execute on original graph
}
```

**Rejected**: Per ADR-004, compiled graph must be immutable. Can't guarantee immutability with same type.

### 2. Return Interface Instead of Concrete Type

```go
type Executable[S any] interface {
    Run(ctx Context, state S) (S, error)
}

func (g *Graph[S]) Compile() (Executable[S], error)
```

**Rejected**: Concrete type gives better IDE support, clearer API. Interface can be added later if needed.

### 3. Return Serializable Plan

```go
type ExecutionPlan struct {
    Steps []Step `json:"steps"`
}

func (g *Graph[S]) Compile() (*ExecutionPlan, error)
```

**Rejected**: Loses type safety. Would need reflection or code generation to execute.

## Consequences

### Positive
- **Fast runtime** - All lookups O(1)
- **Immutable** - CompiledGraph has no setters
- **Debuggable** - All metadata accessible for inspection
- **Introspectable** - Can visualize compiled graph

### Negative
- Memory for metadata (but small: O(N+E))
- Compilation takes O(N+E) time

### Risks
- Metadata gets stale → N/A, graph is immutable

---

## CompiledGraph API

```go
// Execution
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error)
func (cg *CompiledGraph[S]) RunWithCheckpoint(ctx Context, state S, store CheckpointStore) (S, error)

// Introspection
func (cg *CompiledGraph[S]) NodeIDs() []string
func (cg *CompiledGraph[S]) EntryPoint() string
func (cg *CompiledGraph[S]) HasCycle() bool
func (cg *CompiledGraph[S]) Successors(nodeID string) []string
func (cg *CompiledGraph[S]) Predecessors(nodeID string) []string

// Visualization (optional, could be separate package)
func (cg *CompiledGraph[S]) DOT() string  // GraphViz format
func (cg *CompiledGraph[S]) Mermaid() string  // Mermaid diagram
```

---

## Usage Examples

### Basic Compilation
```go
graph := flowgraph.NewGraph[State]().
    AddNode("a", nodeA).
    AddNode("b", nodeB).
    AddEdge("a", "b").
    AddEdge("b", flowgraph.END).
    SetEntry("a")

compiled, err := graph.Compile()
if err != nil {
    log.Fatalf("compilation failed: %v", err)
}

// Run multiple times with same compiled graph
for i := 0; i < 10; i++ {
    result, err := compiled.Run(ctx, State{Input: i})
    if err != nil {
        log.Printf("run %d failed: %v", i, err)
        continue
    }
    log.Printf("run %d result: %v", i, result.Output)
}
```

### Introspection for Debugging
```go
compiled, _ := graph.Compile()

fmt.Println("Graph structure:")
fmt.Printf("  Entry: %s\n", compiled.EntryPoint())
fmt.Printf("  Nodes: %v\n", compiled.NodeIDs())
fmt.Printf("  Has cycle: %v\n", compiled.HasCycle())

for _, node := range compiled.NodeIDs() {
    fmt.Printf("  %s → %v\n", node, compiled.Successors(node))
}
```

### Visualization
```go
compiled, _ := graph.Compile()

// Generate DOT for GraphViz
dot := compiled.DOT()
/*
digraph G {
    a -> b;
    b -> __end__;
}
*/

// Generate Mermaid for markdown
mermaid := compiled.Mermaid()
/*
graph TD
    a --> b
    b --> END
*/
```

---

## Test Cases

```go
func TestCompilationOutput(t *testing.T) {
    graph := flowgraph.NewGraph[testState]().
        AddNode("start", testNode).
        AddNode("middle", testNode).
        AddNode("end", testNode).
        AddEdge("start", "middle").
        AddEdge("middle", "end").
        AddEdge("end", flowgraph.END).
        SetEntry("start")

    compiled, err := graph.Compile()
    require.NoError(t, err)

    // Verify metadata
    assert.Equal(t, "start", compiled.EntryPoint())
    assert.ElementsMatch(t, []string{"start", "middle", "end"}, compiled.NodeIDs())
    assert.False(t, compiled.HasCycle())

    // Verify successors
    assert.Equal(t, []string{"middle"}, compiled.Successors("start"))
    assert.Equal(t, []string{"end"}, compiled.Successors("middle"))
    assert.Equal(t, []string{flowgraph.END}, compiled.Successors("end"))

    // Verify predecessors
    assert.Empty(t, compiled.Predecessors("start"))
    assert.Equal(t, []string{"start"}, compiled.Predecessors("middle"))
    assert.Equal(t, []string{"middle"}, compiled.Predecessors("end"))
}

func TestCompilationImmutability(t *testing.T) {
    graph := flowgraph.NewGraph[testState]().
        AddNode("a", testNode).
        AddEdge("a", flowgraph.END).
        SetEntry("a")

    compiled, _ := graph.Compile()

    // Modify original graph
    graph.AddNode("b", testNode)

    // Compiled graph should be unchanged
    assert.Equal(t, []string{"a"}, compiled.NodeIDs())
}
```
