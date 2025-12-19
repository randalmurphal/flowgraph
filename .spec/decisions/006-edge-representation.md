# ADR-006: Edge Representation

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should edges between nodes be represented? Options include adjacency lists, edge structs, or implicit function chaining.

## Decision

**Three edge types with separate storage:**

```go
type Graph[S any] struct {
    nodes           map[string]NodeFunc[S]
    edges           map[string][]string          // from -> [to, ...]
    conditionalEdges map[string]RouterFunc[S]    // from -> router
    entryPoint      string
}

// Simple edge: A always goes to B
func (g *Graph[S]) AddEdge(from, to string) *Graph[S]

// Conditional edge: A goes to result of router(state)
func (g *Graph[S]) AddConditionalEdge(from string, router RouterFunc[S]) *Graph[S]

// Special constant for terminal edge
const END = "__end__"

// Router returns next node ID or END
type RouterFunc[S any] func(ctx Context, state S) string
```

### Edge Types

#### 1. Simple Edge
```go
graph.AddEdge("a", "b")  // a -> b always
```

#### 2. Conditional Edge
```go
graph.AddConditionalEdge("validate", func(ctx Context, state S) string {
    if state.Valid {
        return "process"
    }
    return "reject"
})
```

#### 3. Terminal Edge
```go
graph.AddEdge("final", flowgraph.END)  // Terminates execution
```

### Validation Rules

1. **No duplicate edges from same source** - Simple edges only allow one target per source
2. **Conditional edge replaces simple edge** - If node has conditional, simple edges ignored
3. **All targets must exist** - Compile fails if edge points to non-existent node
4. **Entry must be set** - Compile fails without entry point
5. **Path to END required** - At least one path must terminate

## Alternatives Considered

### 1. Edge Structs

```go
type Edge struct {
    From      string
    To        string
    Condition func(S) bool
}

graph.AddEdge(Edge{From: "a", To: "b"})
```

**Rejected**: Verbose for common case (simple edges).

### 2. Implicit Chaining

```go
graph.Chain("a", "b", "c")  // a -> b -> c
```

**Rejected**: Doesn't compose well with conditionals.

### 3. Priority-Based Edges

```go
graph.AddEdge("a", "b", Priority(1))
graph.AddEdge("a", "c", Priority(2), When(state.X))
```

**Rejected**: Complex to understand; conditionals are cleaner.

### 4. Edge Objects with Methods

```go
graph.Node("a").To("b").When(func(s S) bool { return s.X })
```

**Rejected**: Different paradigm from node-first approach.

## Consequences

### Positive
- **Clear semantics** - Simple edges are simple, conditionals are explicit
- **Type-safe routing** - Router functions are typed
- **Easy to validate** - Graph structure is explicit

### Negative
- Two maps to maintain (edges, conditionalEdges)
- Can't have both simple and conditional from same node

### Risks
- Complex routing logic in routers → Mitigate: Document best practices

---

## Usage Examples

### Linear Flow
```go
graph := flowgraph.NewGraph[State]().
    AddNode("start", startNode).
    AddNode("middle", middleNode).
    AddNode("end", endNode).
    AddEdge("start", "middle").
    AddEdge("middle", "end").
    AddEdge("end", flowgraph.END).
    SetEntry("start")
```

### Conditional Branching
```go
graph := flowgraph.NewGraph[ReviewState]().
    AddNode("review", reviewNode).
    AddNode("approve", approveNode).
    AddNode("reject", rejectNode).
    AddConditionalEdge("review", func(ctx Context, s ReviewState) string {
        switch s.Decision {
        case "approve":
            return "approve"
        case "reject":
            return "reject"
        default:
            return flowgraph.END  // No decision, stop
        }
    }).
    AddEdge("approve", flowgraph.END).
    AddEdge("reject", flowgraph.END).
    SetEntry("review")
```

### Loop with Exit
```go
graph := flowgraph.NewGraph[IterState]().
    AddNode("process", processNode).
    AddNode("check", checkNode).
    AddConditionalEdge("check", func(ctx Context, s IterState) string {
        if s.Iteration >= s.MaxIterations {
            return flowgraph.END
        }
        if s.Done {
            return flowgraph.END
        }
        return "process"  // Loop back
    }).
    AddEdge("process", "check").
    SetEntry("process")
```

### Multiple Paths to Same Node
```go
graph := flowgraph.NewGraph[State]().
    AddNode("start", startNode).
    AddNode("pathA", pathANode).
    AddNode("pathB", pathBNode).
    AddNode("merge", mergeNode).
    AddConditionalEdge("start", func(ctx Context, s State) string {
        if s.UsePathA {
            return "pathA"
        }
        return "pathB"
    }).
    AddEdge("pathA", "merge").
    AddEdge("pathB", "merge").
    AddEdge("merge", flowgraph.END).
    SetEntry("start")
```

---

## Validation Test Cases

```go
func TestEdgeValidation(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(*Graph[testState])
        wantErr string
    }{
        {
            name: "edge to non-existent node",
            setup: func(g *Graph[testState]) {
                g.AddNode("a", testNode)
                g.AddEdge("a", "b")  // b doesn't exist
            },
            wantErr: "edge target 'b' does not exist",
        },
        {
            name: "no entry point",
            setup: func(g *Graph[testState]) {
                g.AddNode("a", testNode)
                // No SetEntry
            },
            wantErr: "entry point not set",
        },
        {
            name: "no path to END",
            setup: func(g *Graph[testState]) {
                g.AddNode("a", testNode)
                g.AddNode("b", testNode)
                g.AddEdge("a", "b")
                g.AddEdge("b", "a")  // Infinite loop
                g.SetEntry("a")
            },
            wantErr: "no path to END from entry",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            g := flowgraph.NewGraph[testState]()
            tt.setup(g)
            _, err := g.Compile()
            require.Error(t, err)
            assert.Contains(t, err.Error(), tt.wantErr)
        })
    }
}
```
