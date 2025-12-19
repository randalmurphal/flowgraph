# ADR-008: Cycle Handling

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

Should graphs allow cycles (loops)? If so, how do we prevent infinite loops?

## Decision

**Allow cycles with mandatory exit conditions via conditional edges.**

### Rules

1. **Cycles allowed** - Nodes can form loops
2. **Exit required** - Every cycle must have a conditional edge that can exit
3. **Static detection** - Compile-time detection of "pure" cycles (no conditional exit)
4. **Runtime protection** - Max iteration limit as safety net

### Implementation

```go
// Compile-time: Detect pure cycles
func (g *Graph[S]) detectPureCycles() [][]string {
    // Find strongly connected components (SCCs)
    // Return any SCC that has no conditional edge out
    sccs := tarjanSCC(g.edges)

    var pureCycles [][]string
    for _, scc := range sccs {
        if len(scc) > 1 && !g.hasConditionalExit(scc) {
            pureCycles = append(pureCycles, scc)
        }
    }
    return pureCycles
}

// Compile-time: Check that conditional edge exists
func (g *Graph[S]) hasConditionalExit(cycle []string) bool {
    cycleSet := toSet(cycle)
    for _, node := range cycle {
        if _, ok := g.conditionalEdges[node]; ok {
            return true  // Has potential exit
        }
    }
    return false
}

// Runtime: Safety limit
type ExecutionOptions struct {
    MaxIterations int  // Default: 1000
}

func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...ExecutionOption) (S, error) {
    cfg := defaultOptions()
    for _, opt := range opts {
        opt(&cfg)
    }

    iterations := 0
    currentNode := cg.entryPoint

    for currentNode != END {
        iterations++
        if iterations > cfg.MaxIterations {
            return state, fmt.Errorf("%w: exceeded %d iterations",
                ErrMaxIterations, cfg.MaxIterations)
        }

        // Execute node and advance
        // ...
    }

    return state, nil
}
```

### Cycle Categories

| Type | Example | Compile Behavior |
|------|---------|------------------|
| Pure cycle | A → B → A (no conditionals) | Error |
| Guarded cycle | A → B → Router → A or END | Allowed |
| Self-loop | A → A (no conditional) | Error |
| Conditional self-loop | A → Router → A or B | Allowed |

## Alternatives Considered

### 1. No Cycles Allowed (DAG Only)

```go
// Compile rejects any cycle
func (g *Graph[S]) Compile() (*CompiledGraph[S], error) {
    if hasCycle(g.edges) {
        return nil, ErrCycleDetected
    }
}
```

**Rejected**: Many workflows need iteration (retry, refinement loops).

### 2. Explicit Loop Construct

```go
graph.AddLoop("retry-loop",
    loopNodes...,
    LoopUntil(func(s S) bool { return s.Success }),
    MaxIterations(5),
)
```

**Rejected**: Over-complicated API. Conditional edges handle this naturally.

### 3. No Static Detection

```go
// Only runtime iteration limit
// Trust user to add proper exit conditions
```

**Rejected**: Pure cycles are always bugs; catch early.

### 4. Unrolling Loops

```go
// Compile time: Expand loops into linear sequence
// process → check → process_1 → check_1 → process_2 → ...
```

**Rejected**: Doesn't work for dynamic iteration counts.

## Consequences

### Positive
- **Expressive** - Can model retry, refinement, review loops
- **Safe** - Pure cycles caught at compile time
- **Bounded** - Runtime limit prevents runaway execution
- **Natural** - Uses same conditional edge mechanism

### Negative
- SCC detection adds compile-time complexity
- MaxIterations default might be too low/high for some use cases

### Risks
- Conditional edge never returns END → Runtime limit catches, but late
- Mitigate: Document pattern for "always eventually exit"

---

## Loop Patterns

### Retry Loop
```go
graph := flowgraph.NewGraph[RetryState]().
    AddNode("attempt", attemptNode).
    AddNode("evaluate", evaluateNode).
    AddConditionalEdge("evaluate", func(ctx Context, s RetryState) string {
        if s.Success {
            return flowgraph.END
        }
        if s.Attempts >= s.MaxAttempts {
            return flowgraph.END  // Give up
        }
        return "attempt"  // Retry
    }).
    AddEdge("attempt", "evaluate").
    SetEntry("attempt")
```

### Refinement Loop
```go
graph := flowgraph.NewGraph[DraftState]().
    AddNode("draft", draftNode).
    AddNode("review", reviewNode).
    AddNode("refine", refineNode).
    AddNode("finalize", finalizeNode).
    AddConditionalEdge("review", func(ctx Context, s DraftState) string {
        if s.ApprovalScore >= 0.9 {
            return "finalize"
        }
        if s.Iterations >= 3 {
            return "finalize"  // Best effort
        }
        return "refine"  // Need more work
    }).
    AddEdge("draft", "review").
    AddEdge("refine", "review").
    AddEdge("finalize", flowgraph.END).
    SetEntry("draft")
```

### Polling Loop
```go
graph := flowgraph.NewGraph[PollState]().
    AddNode("check", checkStatusNode).
    AddNode("wait", waitNode).
    AddConditionalEdge("check", func(ctx Context, s PollState) string {
        if s.Status == "complete" {
            return flowgraph.END
        }
        if s.Status == "failed" {
            return flowgraph.END
        }
        return "wait"  // Keep polling
    }).
    AddEdge("wait", "check").
    SetEntry("check")
```

---

## Compile Error Examples

### Pure Cycle Detected
```
graph compilation failed:
  cycle detected with no exit condition: [process, validate, process]
  hint: add a conditional edge from one of these nodes that can return END
```

### Self-Loop Without Condition
```
graph compilation failed:
  self-loop detected on node 'retry' with no exit condition
  hint: use AddConditionalEdge instead of AddEdge for loops
```

---

## Test Cases

```go
func TestCycleDetection(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(*Graph[testState])
        wantErr bool
        errMsg  string
    }{
        {
            name: "pure cycle rejected",
            setup: func(g *Graph[testState]) {
                g.AddNode("a", testNode)
                g.AddNode("b", testNode)
                g.AddEdge("a", "b")
                g.AddEdge("b", "a")  // Pure cycle
                g.SetEntry("a")
            },
            wantErr: true,
            errMsg:  "cycle detected with no exit",
        },
        {
            name: "guarded cycle allowed",
            setup: func(g *Graph[testState]) {
                g.AddNode("a", testNode)
                g.AddNode("b", testNode)
                g.AddEdge("a", "b")
                g.AddConditionalEdge("b", func(ctx Context, s testState) string {
                    if s.Done {
                        return flowgraph.END
                    }
                    return "a"
                })
                g.SetEntry("a")
            },
            wantErr: false,
        },
        {
            name: "self-loop rejected",
            setup: func(g *Graph[testState]) {
                g.AddNode("a", testNode)
                g.AddEdge("a", "a")  // Self-loop
                g.SetEntry("a")
            },
            wantErr: true,
            errMsg:  "self-loop detected",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            g := flowgraph.NewGraph[testState]()
            tt.setup(g)
            _, err := g.Compile()

            if tt.wantErr {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```
