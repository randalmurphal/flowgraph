# Phase 2: Conditional Execution

**Status**: Ready (Depends on Phase 1)
**Estimated Effort**: 1-2 days
**Dependencies**: Phase 1 Complete

---

## Goal

Add conditional edges and loop support to enable branching workflows.

---

## Files to Create/Modify

```
pkg/flowgraph/
├── router.go           # NEW: RouterFunc type, routing logic
├── graph.go            # MODIFY: AddConditionalEdge
├── compile.go          # MODIFY: Cycle validation
├── execute.go          # MODIFY: Conditional edge handling
├── router_test.go      # NEW: Router tests
├── conditional_test.go # NEW: Conditional edge tests
└── loop_test.go        # NEW: Loop execution tests
```

---

## Implementation Order

### Step 1: Router Types (~1 hour)

**router.go**
```go
package flowgraph

// RouterFunc determines the next node based on context and state
// Must return a valid node ID or flowgraph.END
type RouterFunc[S any] func(ctx Context, state S) string

// RouterError wraps errors from conditional edge routing
type RouterError struct {
    FromNode string
    Returned string
    Err      error
}

func (e *RouterError) Error() string {
    return fmt.Sprintf("router from %s returned %q: %v",
        e.FromNode, e.Returned, e.Err)
}

func (e *RouterError) Unwrap() error {
    return e.Err
}

// Sentinel errors
var (
    ErrInvalidRouterResult    = errors.New("router returned empty string")
    ErrRouterTargetNotFound   = errors.New("router returned unknown node")
)
```

### Step 2: Graph Builder Extension (~1 hour)

**graph.go additions**
```go
// AddConditionalEdge adds a conditional edge from a node
// The RouterFunc is called during execution to determine the target
// Panics if from node already has simple edges
func (g *Graph[S]) AddConditionalEdge(from string, router RouterFunc[S]) *Graph[S] {
    g.mu.Lock()
    defer g.mu.Unlock()

    // Validate no mixing of edge types
    if len(g.edges[from]) > 0 {
        panic(fmt.Sprintf("flowgraph: node %s already has simple edges", from))
    }

    g.conditionalEdges[from] = router
    return g
}
```

### Step 3: Cycle Validation (~2 hours)

**compile.go additions**
```go
// validateCycles checks that all cycles have conditional exits
func (g *Graph[S]) validateCycles() error {
    // Find strongly connected components
    sccs := g.findSCCs()

    for _, scc := range sccs {
        if len(scc) <= 1 {
            continue  // Single node, no cycle (self-loops handled separately)
        }

        // Check if SCC has conditional exit
        hasConditionalExit := false
        for _, nodeID := range scc {
            if _, ok := g.conditionalEdges[nodeID]; ok {
                // Check if conditional can exit the SCC
                // (goes to END or node outside SCC)
                hasConditionalExit = true
                break
            }
        }

        if !hasConditionalExit {
            return fmt.Errorf("%w: cycle through nodes %v has no conditional exit",
                ErrNoPathToEnd, scc)
        }
    }

    return nil
}

// findSCCs returns strongly connected components using Tarjan's algorithm
func (g *Graph[S]) findSCCs() [][]string {
    // Standard Tarjan implementation
    // Returns list of SCCs, each SCC is a list of node IDs
}

// hasPathToEnd updated to consider conditional edges
func (g *Graph[S]) hasPathToEnd() bool {
    // BFS/DFS from entry
    // Consider both simple edges and all possible outputs from conditional edges
    // For conditional edges, assume router can return any registered node or END
}
```

### Step 4: Execution Extension (~2 hours)

**execute.go additions**
```go
// nextNode determines the next node after current
func (cg *CompiledGraph[S]) nextNode(ctx Context, state S, current string) (string, error) {
    // Check for conditional edge first
    if router, ok := cg.conditionalEdges[current]; ok {
        return cg.executeRouter(ctx, state, current, router)
    }

    // Simple edges
    targets := cg.edges[current]
    if len(targets) == 0 {
        return "", fmt.Errorf("no outgoing edge from node %s", current)
    }
    if len(targets) > 1 {
        return "", fmt.Errorf("node %s has multiple simple edges (use conditional)", current)
    }

    return targets[0], nil
}

// executeRouter calls the RouterFunc with panic recovery
func (cg *CompiledGraph[S]) executeRouter(ctx Context, state S, from string, router RouterFunc[S]) (next string, err error) {
    // Panic recovery for router
    defer func() {
        if r := recover(); r != nil {
            err = &PanicError{
                NodeID: from,
                Value:  r,
                Stack:  string(debug.Stack()),
            }
        }
    }()

    next = router(ctx, state)

    // Validate result
    if next == "" {
        return "", &RouterError{
            FromNode: from,
            Returned: next,
            Err:      ErrInvalidRouterResult,
        }
    }

    if next != END && !cg.HasNode(next) {
        return "", &RouterError{
            FromNode: from,
            Returned: next,
            Err:      ErrRouterTargetNotFound,
        }
    }

    return next, nil
}
```

### Step 5: Tests (~3 hours)

**router_test.go**
```go
func TestRouterFunc_ReturnValues(t *testing.T) {
    tests := []struct {
        name    string
        router  RouterFunc[State]
        wantErr error
    }{
        {
            name:    "valid node",
            router:  func(ctx Context, s State) string { return "next" },
            wantErr: nil,
        },
        {
            name:    "END",
            router:  func(ctx Context, s State) string { return END },
            wantErr: nil,
        },
        {
            name:    "empty string",
            router:  func(ctx Context, s State) string { return "" },
            wantErr: ErrInvalidRouterResult,
        },
        {
            name:    "unknown node",
            router:  func(ctx Context, s State) string { return "nonexistent" },
            wantErr: ErrRouterTargetNotFound,
        },
    }
    // ... test implementation
}
```

**conditional_test.go**
```go
func TestConditionalEdge_Branching(t *testing.T) {
    // Test two-way branching
}

func TestConditionalEdge_MultiWay(t *testing.T) {
    // Test switch-like branching
}

func TestConditionalEdge_ToEND(t *testing.T) {
    // Test conditional termination
}

func TestConditionalEdge_StatePassedToRouter(t *testing.T) {
    // Verify router receives node's output state
}

func TestConditionalEdge_MixedEdgeTypes_Panics(t *testing.T) {
    // Test that adding simple + conditional panics
}
```

**loop_test.go**
```go
func TestLoop_ReviewCycle(t *testing.T) {
    // Implement -> Review -> (approved ? END : Implement)
}

func TestLoop_RetryPattern(t *testing.T) {
    // Self-loop with exit condition
}

func TestLoop_MaxIterations(t *testing.T) {
    // Verify loop terminates at limit
}

func TestLoop_NestedCycles(t *testing.T) {
    // Multiple interconnected loops
}

func TestLoop_InvalidCycle_NoExit(t *testing.T) {
    // Compile should fail
}
```

---

## Acceptance Criteria

After Phase 2, this code must work:

```go
// Branching based on state
graph := flowgraph.NewGraph[ReviewState]().
    AddNode("review", reviewNode).
    AddNode("approve", approveNode).
    AddNode("reject", rejectNode).
    AddConditionalEdge("review", func(ctx Context, s ReviewState) string {
        if s.Score >= 80 {
            return "approve"
        }
        return "reject"
    }).
    AddEdge("approve", flowgraph.END).
    AddEdge("reject", flowgraph.END).
    SetEntry("review")

compiled, _ := graph.Compile()

// Test approval path
result, _ := compiled.Run(ctx, ReviewState{Score: 90})
// Should go: review -> approve -> END

// Test rejection path
result, _ = compiled.Run(ctx, ReviewState{Score: 50})
// Should go: review -> reject -> END
```

```go
// Loop with exit condition
graph := flowgraph.NewGraph[WorkState]().
    AddNode("work", workNode).
    AddNode("check", checkNode).
    AddEdge("work", "check").
    AddConditionalEdge("check", func(ctx Context, s WorkState) string {
        if s.Done {
            return flowgraph.END
        }
        return "work"  // Loop back
    }).
    SetEntry("work")

compiled, _ := graph.Compile()
result, _ := compiled.Run(ctx, WorkState{MaxIterations: 3})
// Should loop 3 times then exit
```

---

## Test Coverage Targets

| File | Target |
|------|--------|
| router.go | 95% |
| graph.go (new code) | 95% |
| compile.go (new code) | 90% |
| execute.go (new code) | 95% |
| Overall Phase 2 | 90% |

---

## Checklist

- [ ] RouterFunc type defined
- [ ] RouterError type with Unwrap
- [ ] Sentinel errors (ErrInvalidRouterResult, ErrRouterTargetNotFound)
- [ ] AddConditionalEdge method
- [ ] Cycle validation in Compile
- [ ] Conditional edge handling in Run
- [ ] Router panic recovery
- [ ] All tests passing
- [ ] 90% coverage achieved
- [ ] No race conditions
- [ ] Godoc for all public types

---

## Risks

| Risk | Mitigation |
|------|------------|
| Tarjan's algorithm complexity | Use well-tested reference implementation |
| Router panic handling | Same pattern as node panics (already tested) |
| Edge type mixing confusion | Clear panic message, document in godoc |

---

## Notes

- Conditional edges are evaluated AFTER the source node executes
- Router receives the OUTPUT state from the node, not the input
- Self-loops (node → same node) are valid if conditional
- At most one conditional edge per node (enforced by panic)
