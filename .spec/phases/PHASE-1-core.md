# Phase 1: Core Graph

**Status**: Ready to Start
**Estimated Effort**: 2-3 days
**Dependencies**: Phase 0 (ADRs) ✅

---

## Goal

Build the core graph engine: definition, compilation, and linear execution.

---

## Files to Create

```
pkg/flowgraph/
├── graph.go           # Graph[S] builder
├── node.go            # Node types
├── edge.go            # Edge types
├── compile.go         # Compilation logic
├── compiled.go        # CompiledGraph
├── execute.go         # Execution engine
├── context.go         # Context interface
├── errors.go          # Error types
├── options.go         # Functional options
├── graph_test.go
├── compile_test.go
├── execute_test.go
└── testutil_test.go   # Test helpers
```

---

## Implementation Order

### Step 1: Types and Errors (~2 hours)

**errors.go**
```go
package flowgraph

import "errors"

// Sentinel errors
var (
    ErrNoEntryPoint   = errors.New("entry point not set")
    ErrEntryNotFound  = errors.New("entry point node not found")
    ErrNodeNotFound   = errors.New("node not found")
    ErrNoPathToEnd    = errors.New("no path to END from entry")
    ErrMaxIterations  = errors.New("exceeded maximum iterations")
    ErrNilContext     = errors.New("context cannot be nil")
)

// NodeError wraps an error with node context
type NodeError struct {
    NodeID string
    Op     string
    Err    error
}

// PanicError captures panic information
type PanicError struct {
    NodeID string
    Value  any
    Stack  string
}

// CancellationError captures state at cancellation
type CancellationError struct {
    NodeID       string
    State        any
    Cause        error
    WasExecuting bool
}
```

**node.go**
```go
package flowgraph

// NodeFunc is the signature for all node functions
type NodeFunc[S any] func(ctx Context, state S) (S, error)

// END is the terminal node identifier
const END = "__end__"
```

### Step 2: Context Interface (~2 hours)

**context.go**
```go
package flowgraph

import (
    "context"
    "log/slog"
)

// Context provides execution context to nodes
type Context interface {
    context.Context
    Logger() *slog.Logger
    LLM() LLMClient
    Checkpointer() CheckpointStore
    RunID() string
    NodeID() string
    Attempt() int
}

// LLMClient interface (minimal for Phase 1)
type LLMClient interface {
    // Defined in Phase 4, placeholder here
}

// CheckpointStore interface (minimal for Phase 1)
type CheckpointStore interface {
    // Defined in Phase 3, placeholder here
}
```

### Step 3: Graph Builder (~3 hours)

**graph.go**
```go
package flowgraph

import (
    "fmt"
    "strings"
    "sync"
)

// Graph is a builder for creating execution graphs
type Graph[S any] struct {
    mu              sync.RWMutex
    nodes           map[string]NodeFunc[S]
    edges           map[string][]string
    conditionalEdges map[string]RouterFunc[S]
    entryPoint      string
}

// RouterFunc determines the next node based on state
type RouterFunc[S any] func(ctx Context, state S) string

// NewGraph creates a new graph builder
func NewGraph[S any]() *Graph[S] {
    return &Graph[S]{
        nodes:           make(map[string]NodeFunc[S]),
        edges:           make(map[string][]string),
        conditionalEdges: make(map[string]RouterFunc[S]),
    }
}

// AddNode adds a node to the graph
func (g *Graph[S]) AddNode(id string, fn NodeFunc[S]) *Graph[S] {
    // Validation (panics per ADR-007)
    if id == "" {
        panic("flowgraph: node ID cannot be empty")
    }
    if id == END {
        panic("flowgraph: node ID cannot be reserved word 'END'")
    }
    if strings.ContainsAny(id, " \t\n") {
        panic("flowgraph: node ID cannot contain whitespace")
    }
    if fn == nil {
        panic("flowgraph: node function cannot be nil")
    }

    g.mu.Lock()
    defer g.mu.Unlock()

    if _, exists := g.nodes[id]; exists {
        panic(fmt.Sprintf("flowgraph: duplicate node ID: %s", id))
    }

    g.nodes[id] = fn
    return g
}

// AddEdge adds a simple edge between nodes
func (g *Graph[S]) AddEdge(from, to string) *Graph[S] {
    g.mu.Lock()
    defer g.mu.Unlock()
    g.edges[from] = append(g.edges[from], to)
    return g
}

// SetEntry sets the entry point node
func (g *Graph[S]) SetEntry(id string) *Graph[S] {
    g.mu.Lock()
    defer g.mu.Unlock()
    g.entryPoint = id
    return g
}
```

### Step 4: Compilation (~4 hours)

**compile.go**
```go
package flowgraph

import (
    "errors"
    "fmt"
)

// Compile validates the graph and creates an executable CompiledGraph
func (g *Graph[S]) Compile() (*CompiledGraph[S], error) {
    g.mu.RLock()
    defer g.mu.RUnlock()

    var errs []error

    // Validate entry point
    if g.entryPoint == "" {
        errs = append(errs, ErrNoEntryPoint)
    } else if _, exists := g.nodes[g.entryPoint]; !exists {
        errs = append(errs, fmt.Errorf("%w: %s", ErrEntryNotFound, g.entryPoint))
    }

    // Validate edge references
    for from, targets := range g.edges {
        if _, exists := g.nodes[from]; !exists && from != END {
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

    // Validate path to END
    if !g.hasPathToEnd() {
        errs = append(errs, ErrNoPathToEnd)
    }

    if len(errs) > 0 {
        return nil, errors.Join(errs...)
    }

    return g.buildCompiledGraph(), nil
}
```

**compiled.go**
```go
package flowgraph

// CompiledGraph is an immutable, executable graph
type CompiledGraph[S any] struct {
    nodes           map[string]NodeFunc[S]
    edges           map[string][]string
    conditionalEdges map[string]RouterFunc[S]
    entryPoint      string

    // Pre-computed
    successors   map[string][]string
    predecessors map[string][]string
    isConditional map[string]bool
}

// NodeIDs returns all node identifiers
func (cg *CompiledGraph[S]) NodeIDs() []string {
    ids := make([]string, 0, len(cg.nodes))
    for id := range cg.nodes {
        ids = append(ids, id)
    }
    return ids
}

// EntryPoint returns the entry node
func (cg *CompiledGraph[S]) EntryPoint() string {
    return cg.entryPoint
}
```

### Step 5: Execution (~4 hours)

**execute.go**
```go
package flowgraph

import (
    "context"
    "fmt"
    "runtime/debug"
)

// Run executes the graph with the given initial state
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error) {
    if ctx == nil {
        return state, ErrNilContext
    }

    cfg := defaultRunConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    current := cg.entryPoint
    iterations := 0

    for current != END {
        iterations++
        if iterations > cfg.maxIterations {
            return state, fmt.Errorf("%w: exceeded %d iterations",
                ErrMaxIterations, cfg.maxIterations)
        }

        // Check cancellation
        select {
        case <-ctx.Done():
            return state, &CancellationError{
                NodeID:       current,
                State:        state,
                Cause:        ctx.Err(),
                WasExecuting: false,
            }
        default:
        }

        // Execute node
        var err error
        state, err = cg.executeNode(ctx, current, state)
        if err != nil {
            return state, err
        }

        // Determine next node
        current, err = cg.nextNode(ctx, state, current)
        if err != nil {
            return state, err
        }
    }

    return state, nil
}

func (cg *CompiledGraph[S]) executeNode(ctx Context, nodeID string, state S) (result S, err error) {
    fn := cg.nodes[nodeID]

    // Panic recovery
    defer func() {
        if r := recover(); r != nil {
            err = &PanicError{
                NodeID: nodeID,
                Value:  r,
                Stack:  string(debug.Stack()),
            }
        }
    }()

    result, err = fn(ctx, state)
    if err != nil {
        return result, &NodeError{NodeID: nodeID, Err: err}
    }
    return result, nil
}
```

**options.go**
```go
package flowgraph

type runConfig struct {
    maxIterations int
    // More options added in later phases
}

func defaultRunConfig() runConfig {
    return runConfig{
        maxIterations: 1000,
    }
}

type RunOption func(*runConfig)

func WithMaxIterations(n int) RunOption {
    return func(c *runConfig) { c.maxIterations = n }
}
```

### Step 6: Tests (~4 hours)

See ADR-025 and ADR-026 for test patterns.

---

## Acceptance Criteria

After Phase 1, this code must work:

```go
package main

import (
    "context"
    "fmt"
    "github.com/yourusername/flowgraph"
)

type CounterState struct {
    Count int
}

func increment(ctx flowgraph.Context, s CounterState) (CounterState, error) {
    s.Count++
    return s, nil
}

func main() {
    graph := flowgraph.NewGraph[CounterState]().
        AddNode("inc1", increment).
        AddNode("inc2", increment).
        AddNode("inc3", increment).
        AddEdge("inc1", "inc2").
        AddEdge("inc2", "inc3").
        AddEdge("inc3", flowgraph.END).
        SetEntry("inc1")

    compiled, err := graph.Compile()
    if err != nil {
        panic(err)
    }

    ctx := flowgraph.NewContext(context.Background())
    result, err := compiled.Run(ctx, CounterState{Count: 0})
    if err != nil {
        panic(err)
    }

    fmt.Printf("Final count: %d\n", result.Count)  // Output: 3
}
```

---

## Test Coverage Target

| File | Target |
|------|--------|
| graph.go | 95% |
| compile.go | 95% |
| execute.go | 90% |
| errors.go | 100% |
| context.go | 90% |
| Overall | 90% |

---

## Checklist

- [ ] errors.go with all error types
- [ ] node.go with NodeFunc and END
- [ ] context.go with Context interface
- [ ] graph.go with builder methods
- [ ] compile.go with validation
- [ ] compiled.go with CompiledGraph
- [ ] execute.go with Run()
- [ ] options.go with RunOption
- [ ] All tests passing
- [ ] 90% coverage achieved
- [ ] No race conditions (go test -race)
- [ ] Godoc for all public types
