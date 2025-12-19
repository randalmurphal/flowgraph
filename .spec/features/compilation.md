# Feature: Compilation

**Related ADRs**: 007-validation-timing, 008-cycle-handling, 009-compilation-output

---

## Problem Statement

Graph compilation transforms a mutable builder into an immutable, executable form. This step must:
1. Validate the graph structure is correct
2. Detect potential issues (cycles without exits, unreachable nodes)
3. Pre-compute metadata to optimize execution
4. Produce an immutable result safe for concurrent use

## User Stories

- As a developer, I want Compile() to catch all structural problems so that invalid graphs never run
- As a developer, I want clear error messages pointing to exactly what's wrong
- As a developer, I want compilation to be fast (sub-millisecond for reasonable graphs)
- As a developer, I want the compiled graph to be reusable for multiple runs

---

## API Design

### Compile Method

```go
// Compile validates the graph and returns an immutable CompiledGraph
// Returns error if validation fails (multiple errors joined)
func (g *Graph[S]) Compile() (*CompiledGraph[S], error)
```

### CompiledGraph Type

```go
// CompiledGraph is an immutable, executable graph
// Safe for concurrent use
type CompiledGraph[S any] struct {
    // All fields unexported
}

// EntryPoint returns the entry node ID
func (cg *CompiledGraph[S]) EntryPoint() string

// NodeIDs returns all node identifiers (unordered)
func (cg *CompiledGraph[S]) NodeIDs() []string

// HasNode checks if a node exists
func (cg *CompiledGraph[S]) HasNode(id string) bool

// Successors returns nodes reachable from the given node
// Returns nil for END or unknown node
func (cg *CompiledGraph[S]) Successors(id string) []string

// Predecessors returns nodes that can reach the given node
// Returns nil for entry or unknown node
func (cg *CompiledGraph[S]) Predecessors(id string) []string

// IsConditional returns true if node has a conditional edge
func (cg *CompiledGraph[S]) IsConditional(id string) bool
```

---

## Behavior Specification

### Validation Order

Compile() performs validation in this order:

1. **Entry Point Validation**
   - Entry point must be set
   - Entry point must reference existing node

2. **Edge Validation**
   - All edge sources must reference existing nodes (or be implicit from nodes with conditional edges)
   - All edge targets must reference existing nodes or END

3. **Reachability Validation**
   - All nodes must have a path to END (via simple or conditional edges)
   - Unreachable nodes logged as warning (not error)

4. **Metadata Computation** (only if validation passes)
   - Pre-compute successors map
   - Pre-compute predecessors map
   - Identify conditional nodes

### Cycle Handling (ADR-008)

Cycles are **allowed** if they have conditional exit to END:

```go
// Valid: Loop with conditional exit
graph.AddNode("check", checkNode).
    AddConditionalEdge("check", func(ctx Context, s State) string {
        if s.Done { return flowgraph.END }
        return "process"
    }).
    AddNode("process", processNode).
    AddEdge("process", "check")
```

Cycles are **invalid** if there's no path out:

```go
// Invalid: Unconditional infinite loop
graph.AddNode("a", nodeA).
    AddNode("b", nodeB).
    AddEdge("a", "b").
    AddEdge("b", "a")  // No exit!
```

### Unreachable Node Detection

Nodes not reachable from entry are logged as warnings:

```go
graph.AddNode("a", nodeA).
    AddNode("b", nodeB).
    AddNode("orphan", orphanNode).  // Not connected
    AddEdge("a", "b").
    AddEdge("b", flowgraph.END).
    SetEntry("a")

// Compiles successfully, but logs:
// WARN: node "orphan" is unreachable from entry
```

---

## Error Cases

### Sentinel Errors

```go
var (
    ErrNoEntryPoint  = errors.New("entry point not set")
    ErrEntryNotFound = errors.New("entry point node not found")
    ErrNodeNotFound  = errors.New("node not found")
    ErrNoPathToEnd   = errors.New("no path to END from entry")
)
```

### Error Aggregation

Compile returns all errors found, not just the first:

```go
_, err := graph.Compile()
// err contains joined errors:
// "entry point not set; edge source 'x' does not exist; edge target 'y' does not exist"
```

Use `errors.Is()` to check for specific errors:

```go
if errors.Is(err, flowgraph.ErrNoEntryPoint) {
    // Handle missing entry
}
```

---

## Test Cases

### Successful Compilation

```go
func TestCompile_LinearGraph(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddNode("c", nodeC).
        AddEdge("a", "b").
        AddEdge("b", "c").
        AddEdge("c", flowgraph.END).
        SetEntry("a")

    compiled, err := graph.Compile()

    require.NoError(t, err)
    assert.Equal(t, "a", compiled.EntryPoint())
    assert.True(t, compiled.HasNode("b"))
    assert.Equal(t, []string{"b"}, compiled.Successors("a"))
    assert.Equal(t, []string{"a"}, compiled.Predecessors("b"))
}

func TestCompile_BranchingGraph(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("start", startNode).
        AddNode("left", leftNode).
        AddNode("right", rightNode).
        AddNode("join", joinNode).
        AddConditionalEdge("start", func(ctx Context, s State) string {
            if s.GoLeft { return "left" }
            return "right"
        }).
        AddEdge("left", "join").
        AddEdge("right", "join").
        AddEdge("join", flowgraph.END).
        SetEntry("start")

    compiled, err := graph.Compile()

    require.NoError(t, err)
    assert.True(t, compiled.IsConditional("start"))
    assert.False(t, compiled.IsConditional("left"))
}

func TestCompile_ValidCycle(t *testing.T) {
    // Loop with conditional exit is valid
    graph := flowgraph.NewGraph[State]().
        AddNode("check", checkNode).
        AddNode("process", processNode).
        AddConditionalEdge("check", router).  // returns END or "process"
        AddEdge("process", "check").
        SetEntry("check")

    compiled, err := graph.Compile()

    require.NoError(t, err)
    assert.NotNil(t, compiled)
}
```

### Compilation Errors

```go
func TestCompile_NoEntryPoint(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddEdge("a", flowgraph.END)
        // No SetEntry()

    _, err := graph.Compile()

    assert.ErrorIs(t, err, flowgraph.ErrNoEntryPoint)
}

func TestCompile_EntryNotFound(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddEdge("a", flowgraph.END).
        SetEntry("nonexistent")

    _, err := graph.Compile()

    assert.ErrorIs(t, err, flowgraph.ErrEntryNotFound)
}

func TestCompile_MissingEdgeTarget(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddEdge("a", "nonexistent").  // Target doesn't exist
        SetEntry("a")

    _, err := graph.Compile()

    assert.ErrorIs(t, err, flowgraph.ErrNodeNotFound)
    assert.Contains(t, err.Error(), "nonexistent")
}

func TestCompile_NoPathToEnd(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddEdge("a", "b").
        // b has no outgoing edge - dead end
        SetEntry("a")

    _, err := graph.Compile()

    assert.ErrorIs(t, err, flowgraph.ErrNoPathToEnd)
}

func TestCompile_MultipleErrors(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddEdge("a", "missing1").
        AddEdge("missing2", "a")
        // No entry point

    _, err := graph.Compile()

    // All errors should be reported
    assert.ErrorIs(t, err, flowgraph.ErrNoEntryPoint)
    assert.ErrorIs(t, err, flowgraph.ErrNodeNotFound)
}
```

### Edge Cases

```go
func TestCompile_SingleNodeGraph(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("only", onlyNode).
        AddEdge("only", flowgraph.END).
        SetEntry("only")

    compiled, err := graph.Compile()

    require.NoError(t, err)
    assert.Equal(t, []string{"only"}, compiled.NodeIDs())
}

func TestCompile_SelfLoop_WithExit(t *testing.T) {
    // Node loops to itself but has conditional exit
    graph := flowgraph.NewGraph[State]().
        AddNode("loop", loopNode).
        AddConditionalEdge("loop", func(ctx Context, s State) string {
            if s.Done { return flowgraph.END }
            return "loop"  // Loop to self
        }).
        SetEntry("loop")

    compiled, err := graph.Compile()

    require.NoError(t, err)  // Valid - has exit condition
}

func TestCompile_RecompilingDoesNotAffectPrevious(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddEdge("a", flowgraph.END).
        SetEntry("a")

    compiled1, _ := graph.Compile()

    // Add more nodes
    graph.AddNode("b", nodeB).
        AddEdge("a", "b").
        AddEdge("b", flowgraph.END)

    compiled2, _ := graph.Compile()

    // compiled1 should be unchanged
    assert.Equal(t, 1, len(compiled1.NodeIDs()))
    assert.Equal(t, 2, len(compiled2.NodeIDs()))
}
```

---

## Performance Requirements

| Graph Size | Compile Time | Notes |
|------------|--------------|-------|
| 10 nodes | < 100 microseconds | Common case |
| 100 nodes | < 1 millisecond | Large workflow |
| 1000 nodes | < 10 milliseconds | Very large graph |

Memory: O(V + E) - linear in graph size.

### Benchmark

```go
func BenchmarkCompile_10Nodes(b *testing.B) {
    graph := buildLinearGraph(10)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        graph.Compile()
    }
}

func BenchmarkCompile_100Nodes(b *testing.B) {
    graph := buildLinearGraph(100)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        graph.Compile()
    }
}
```

---

## Security Considerations

None specific to compilation. Graph structure is developer-controlled.

---

## Simplicity Check

**What we included**:
- Single Compile() method
- All-or-nothing validation (returns error or valid CompiledGraph)
- Basic introspection (entry point, node list, successors/predecessors)
- Immutability guarantee

**What we did NOT include**:
- Partial compilation (compile what's valid) - Too confusing. Fix the errors.
- Warnings as return values - Logged instead. Don't clutter the API.
- Compilation options - No knobs to turn. One way to compile.
- Graph visualization/DOT export - Layer on top if needed.
- Incremental compilation - Compile is fast enough. Simpler to recompile.

**Is this the simplest solution?** Yes. Compile() does one thing: validate and freeze.
