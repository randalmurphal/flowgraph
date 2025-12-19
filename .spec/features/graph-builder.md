# Feature: Graph Builder

**Related ADRs**: 004-graph-immutability, 005-node-signature, 006-edge-representation, 007-validation-timing

---

## Problem Statement

Users need a way to define execution graphs that:
1. Are type-safe (compile-time guarantees on state)
2. Use a fluent, chainable API
3. Validate correctness at the right times (panic early, error at compile)
4. Are immutable after compilation (safe to share/reuse)

## User Stories

- As a developer, I want to build a graph with a fluent API so that I can define workflows in readable, chainable code
- As a developer, I want type-safe state so that the compiler catches state mismatches before runtime
- As a developer, I want immediate feedback on invalid operations (empty node ID, nil function) so that I catch mistakes early
- As a developer, I want to reuse a compiled graph for multiple runs so that I don't pay compilation cost repeatedly

---

## API Design

### Core Types

```go
// Graph is the mutable builder for creating execution graphs
type Graph[S any] struct {
    // All fields unexported
}

// NewGraph creates a new graph builder for state type S
func NewGraph[S any]() *Graph[S]
```

### Builder Methods

All builder methods return `*Graph[S]` for chaining and panic on programmer errors (per ADR-007).

```go
// AddNode adds a named node to the graph
// Panics if:
//   - id is empty
//   - id is reserved word "END" (case-insensitive)
//   - id contains whitespace
//   - fn is nil
//   - id already exists
func (g *Graph[S]) AddNode(id string, fn NodeFunc[S]) *Graph[S]

// AddEdge adds an unconditional edge from one node to another
// Target can be a node ID or flowgraph.END
func (g *Graph[S]) AddEdge(from, to string) *Graph[S]

// AddConditionalEdge adds a conditional edge where RouterFunc determines next node
// The router returns a node ID or flowgraph.END
func (g *Graph[S]) AddConditionalEdge(from string, router RouterFunc[S]) *Graph[S]

// SetEntry designates the entry point node
// Must be called before Compile()
func (g *Graph[S]) SetEntry(id string) *Graph[S]

// Compile validates the graph and returns an immutable CompiledGraph
// Returns error if:
//   - No entry point set
//   - Entry point references non-existent node
//   - Edges reference non-existent nodes
//   - No path to END from entry
func (g *Graph[S]) Compile() (*CompiledGraph[S], error)
```

### Constants

```go
// END is the terminal node identifier
const END = "__end__"
```

---

## Behavior Specification

### State Machine: Graph Builder

```
                     ┌──────────────────────────────────────────────┐
                     │                                              │
                     ▼                                              │
    NewGraph() → [Empty] ──AddNode()──► [HasNodes] ──AddEdge()──────┤
                     │                       │                      │
                     │                       │                      │
                     ▼                       ▼                      │
              AddConditionalEdge()    SetEntry()                    │
                     │                       │                      │
                     │                       ▼                      │
                     │              [HasEntry]                      │
                     │                       │                      │
                     └───────────────────────┘                      │
                                             │                      │
                                             ▼                      │
                                    Compile() ──► [Compiled]        │
                                             │                      │
                                             │     (immutable)      │
                                             │                      │
                                             └──────────────────────┘
```

### Validation Timing (ADR-007)

| When | What | Response |
|------|------|----------|
| AddNode() | Empty ID | panic |
| AddNode() | Reserved ID ("END") | panic |
| AddNode() | Whitespace in ID | panic |
| AddNode() | nil function | panic |
| AddNode() | Duplicate ID | panic |
| Compile() | No entry point | error |
| Compile() | Entry references missing node | error |
| Compile() | Edge references missing node | error |
| Compile() | No path to END | error |
| Compile() | Unreachable nodes | warning (logged, not error) |

### Node ID Rules

- Must not be empty
- Must not equal "END" or "__end__" (case-insensitive match)
- Must not contain whitespace (space, tab, newline)
- Must be unique within the graph
- Recommended: alphanumeric with hyphens (e.g., "fetch-ticket", "validate-input")

### Thread Safety

- `*Graph[S]` is NOT thread-safe during building (single builder pattern)
- `*CompiledGraph[S]` IS thread-safe (immutable, can run concurrently)

---

## Error Cases

| Scenario | Error Type | Message |
|----------|------------|---------|
| No entry point | `ErrNoEntryPoint` | "entry point not set" |
| Entry node not found | `ErrEntryNotFound` | "entry point node not found: {id}" |
| Edge source not found | `ErrNodeNotFound` | "edge source '{from}' does not exist" |
| Edge target not found | `ErrNodeNotFound` | "edge target '{to}' does not exist" |
| No path to END | `ErrNoPathToEnd` | "no path to END from entry" |

Compile returns `errors.Join()` of all errors found (not fail-fast).

---

## Test Cases

### Happy Path

```go
// Test: Basic linear graph compiles successfully
func TestGraph_Compile_LinearFlow(t *testing.T) {
    graph := flowgraph.NewGraph[CounterState]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddEdge("a", "b").
        AddEdge("b", flowgraph.END).
        SetEntry("a")

    compiled, err := graph.Compile()

    require.NoError(t, err)
    assert.NotNil(t, compiled)
    assert.Equal(t, "a", compiled.EntryPoint())
    assert.ElementsMatch(t, []string{"a", "b"}, compiled.NodeIDs())
}
```

### Validation Panics

```go
// Test: Empty node ID panics
func TestGraph_AddNode_EmptyID_Panics(t *testing.T) {
    assert.Panics(t, func() {
        flowgraph.NewGraph[State]().AddNode("", fn)
    })
}

// Test: Reserved word END panics
func TestGraph_AddNode_ReservedID_Panics(t *testing.T) {
    assert.Panics(t, func() {
        flowgraph.NewGraph[State]().AddNode("END", fn)
    })
    assert.Panics(t, func() {
        flowgraph.NewGraph[State]().AddNode("__end__", fn)
    })
}

// Test: Nil function panics
func TestGraph_AddNode_NilFunc_Panics(t *testing.T) {
    assert.Panics(t, func() {
        flowgraph.NewGraph[State]().AddNode("a", nil)
    })
}

// Test: Duplicate ID panics
func TestGraph_AddNode_DuplicateID_Panics(t *testing.T) {
    assert.Panics(t, func() {
        flowgraph.NewGraph[State]().
            AddNode("a", fn).
            AddNode("a", fn)
    })
}
```

### Compile Errors

```go
// Test: No entry point returns error
func TestGraph_Compile_NoEntryPoint_Error(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddEdge("a", flowgraph.END)

    _, err := graph.Compile()

    assert.ErrorIs(t, err, flowgraph.ErrNoEntryPoint)
}

// Test: Missing node reference returns error
func TestGraph_Compile_MissingNode_Error(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddEdge("a", "nonexistent").
        SetEntry("a")

    _, err := graph.Compile()

    assert.ErrorIs(t, err, flowgraph.ErrNodeNotFound)
}

// Test: No path to END returns error
func TestGraph_Compile_NoPathToEnd_Error(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddEdge("a", "b").
        // b has no outgoing edge
        SetEntry("a")

    _, err := graph.Compile()

    assert.ErrorIs(t, err, flowgraph.ErrNoPathToEnd)
}

// Test: Multiple errors returned together
func TestGraph_Compile_MultipleErrors_AllReturned(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddEdge("a", "missing1").
        AddEdge("missing2", flowgraph.END)
        // No entry point set

    _, err := graph.Compile()

    assert.ErrorIs(t, err, flowgraph.ErrNoEntryPoint)
    assert.ErrorIs(t, err, flowgraph.ErrNodeNotFound)
}
```

### CompiledGraph Introspection

```go
// Test: CompiledGraph provides introspection
func TestCompiledGraph_Introspection(t *testing.T) {
    graph := flowgraph.NewGraph[State]().
        AddNode("start", nodeA).
        AddNode("process", nodeB).
        AddNode("finish", nodeC).
        AddEdge("start", "process").
        AddEdge("process", "finish").
        AddEdge("finish", flowgraph.END).
        SetEntry("start")

    compiled, _ := graph.Compile()

    assert.Equal(t, "start", compiled.EntryPoint())
    assert.Len(t, compiled.NodeIDs(), 3)
    assert.ElementsMatch(t, []string{"start", "process", "finish"}, compiled.NodeIDs())
}
```

---

## Performance Requirements

| Operation | Target | Notes |
|-----------|--------|-------|
| AddNode | O(1) | Hash map insertion |
| AddEdge | O(1) | Append to slice |
| Compile | O(V + E) | Graph traversal for validation |
| Memory | O(V + E) | Linear in graph size |

Where V = number of nodes, E = number of edges.

---

## Security Considerations

1. **Node ID injection**: Node IDs are user-provided strings. They're used as map keys, not in file paths or SQL queries. No security risk.

2. **Node function trust**: Node functions are provided by the user and execute with full process permissions. This is by design - flowgraph is a library, not a sandbox.

3. **Panic recovery**: Builder panics are for programmer errors (empty ID). They should not be caught - they indicate bugs.

---

## Simplicity Check

**What we included**:
- Minimal builder API (4 methods + Compile)
- Type-safe generics for state
- Fluent chainable interface
- Panic for programmer errors, error for runtime validation

**What we did NOT include**:
- `RemoveNode()` / `RemoveEdge()` - Graphs are built once. Mutation adds complexity and use case is unclear.
- `Clone()` - Build another graph instead. Cloning mutable state is error-prone.
- Named edge IDs - Edges are identified by (from, to). No need for separate names.
- Default entry point - Explicit is better. `SetEntry()` is one line.
- Graph metadata (name, description) - Not needed for execution. Layer on top if needed.

**Is this the simplest solution?** Yes. The API has one way to do everything. Builder → Compile → Run.
