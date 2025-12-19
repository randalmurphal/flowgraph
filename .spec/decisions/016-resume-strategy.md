# ADR-016: Resume Strategy

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should execution resume after a crash or failure? Options:
- Resume from failed node
- Resume from last successful node
- Resume from specific checkpoint
- Replay from beginning with cached results

## Decision

**Resume from the node AFTER the last checkpoint, with configurable behavior.**

### Default Resume Behavior

```go
// If run crashed after node B completed:
// A → B → [CRASH] → C → END
//
// Resume starts at C with state from B's checkpoint

func (cg *CompiledGraph[S]) Resume(
    ctx Context,
    store CheckpointStore,
    runID string,
    opts ...RunOption,
) (S, error) {
    // Find latest checkpoint
    checkpoints, err := store.List(runID)
    if err != nil {
        return zero[S](), fmt.Errorf("list checkpoints: %w", err)
    }
    if len(checkpoints) == 0 {
        return zero[S](), ErrNoCheckpointFound
    }

    latest := checkpoints[len(checkpoints)-1]

    // Load state from checkpoint
    data, err := store.Load(runID, latest.NodeID)
    if err != nil {
        return zero[S](), fmt.Errorf("load checkpoint: %w", err)
    }

    var cp Checkpoint
    if err := json.Unmarshal(data, &cp); err != nil {
        return zero[S](), fmt.Errorf("unmarshal checkpoint: %w", err)
    }

    var state S
    if err := json.Unmarshal(cp.State, &state); err != nil {
        return zero[S](), fmt.Errorf("unmarshal state: %w", err)
    }

    // Find next node after checkpoint
    nextNode, err := cg.nextNode(ctx, state, latest.NodeID)
    if err != nil {
        return zero[S](), fmt.Errorf("determine next node: %w", err)
    }

    // Continue execution from next node
    return cg.runFrom(ctx, state, nextNode, store, runID, opts...)
}
```

### Resume Options

```go
type ResumeOption func(*resumeConfig)

type resumeConfig struct {
    fromNode      string  // Override: start from specific node
    replayNode    bool    // Re-execute the checkpoint node
    validateState func(S) error  // Validate state before resuming
}

// Resume from a specific node (not necessarily latest checkpoint)
func ResumeFrom(nodeID string) ResumeOption {
    return func(c *resumeConfig) { c.fromNode = nodeID }
}

// Re-execute the checkpointed node (useful if node was idempotent but crashed mid-execution)
func ReplayCheckpointNode() ResumeOption {
    return func(c *resumeConfig) { c.replayNode = true }
}

// Validate state is still valid before resuming
func WithStateValidation(fn func(S) error) ResumeOption {
    return func(c *resumeConfig) { c.validateState = fn }
}
```

## Alternatives Considered

### 1. Always Replay Failed Node

```go
// If crashed during C:
// Resume re-executes C with state from B
```

**Rejected as default**: Non-idempotent nodes would cause issues. Made optional.

### 2. Full Replay with Caching

```go
// Re-run from beginning, but use cached results for completed nodes
for _, nodeID := range order {
    if cached := loadCache(nodeID); cached != nil {
        state = cached
    } else {
        state = execute(nodeID, state)
    }
}
```

**Rejected**: More complex, doesn't handle side effects well.

### 3. User Chooses Checkpoint

```go
// Present list of checkpoints, user picks one
checkpoints := store.List(runID)
// UI to select...
selected := getUserChoice(checkpoints)
Resume(ctx, store, runID, ResumeFrom(selected.NodeID))
```

**Supported**: User can pass ResumeFrom option. Default is latest.

### 4. Transaction Log Replay

```go
// Store every state mutation, replay like database WAL
log := loadTransactionLog(runID)
state := initialState
for _, tx := range log {
    state = apply(tx, state)
}
```

**Rejected**: Over-engineered for v1. Checkpoint per node is sufficient.

## Consequences

### Positive
- **Simple default** - Resume from where you left off
- **Flexible** - Options for other behaviors
- **Safe** - Skips completed nodes (avoids side effect duplication)

### Negative
- Non-idempotent nodes may have partial side effects
- State must be serializable

### Risks
- State changes between crash and resume → Mitigate: StateValidation option

---

## Resume Scenarios

### Scenario 1: Clean Crash

```
A ✓ → B ✓ → [CRASH] → C → D → END
                       ↑
                    Resume here
```

State from B's checkpoint, execute C, D, END.

### Scenario 2: Mid-Node Crash

```
A ✓ → B ✓ → C [CRASH MID-EXECUTION] → D → END
             ↑
          Resume here (default: skip C, use B's state)
          Or: ReplayCheckpointNode() to re-run C
```

Decision depends on whether C is idempotent:
- **Idempotent** (e.g., LLM call): Re-run is safe
- **Non-idempotent** (e.g., payment): Skip and investigate

### Scenario 3: Loop Resume

```
A ✓ → B ✓ → C ✓ → [loop check] → B ✓ → C [CRASH]
                                       ↑
                                Resume here
```

Resume from latest checkpoint of C (second iteration).

---

## Usage Examples

### Basic Resume

```go
// First run crashes
runID := uuid.NewString()
result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID(runID),
)
// err != nil, crashed

// Later: Resume
result, err = compiled.Resume(ctx, store, runID)
if err != nil {
    log.Fatalf("resume failed: %v", err)
}
```

### Resume from Specific Node

```go
// User wants to re-run from a specific point
result, err := compiled.Resume(ctx, store, runID,
    flowgraph.ResumeFrom("review"),
)
```

### Resume with State Validation

```go
// Validate external state hasn't changed
result, err := compiled.Resume(ctx, store, runID,
    flowgraph.WithStateValidation(func(s MyState) error {
        // Check ticket still exists
        ticket, err := jira.Get(s.TicketID)
        if err != nil {
            return fmt.Errorf("ticket not found: %w", err)
        }
        if ticket.Status == "Closed" {
            return errors.New("ticket was closed, cannot resume")
        }
        return nil
    }),
)
```

### Resume with Replay

```go
// Node was idempotent, safe to re-run
result, err := compiled.Resume(ctx, store, runID,
    flowgraph.ReplayCheckpointNode(),
)
```

---

## Test Cases

```go
func TestResume_FromLatestCheckpoint(t *testing.T) {
    var executed []string
    makeNode := func(name string) NodeFunc[testState] {
        return func(ctx Context, s testState) (testState, error) {
            executed = append(executed, name)
            return s, nil
        }
    }

    store := NewMemoryStore()
    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("a", makeNode("a")).
        AddNode("b", makeNode("b")).
        AddNode("c", makeNode("c")).
        AddEdge("a", "b").AddEdge("b", "c").AddEdge("c", flowgraph.END).
        SetEntry("a").
        Compile()

    // Simulate: a and b completed, crashed before c
    runID := "test-run"
    store.Save(runID, "a", mustMarshalCheckpoint(testState{}))
    store.Save(runID, "b", mustMarshalCheckpoint(testState{}))

    // Resume
    _, err := compiled.Resume(context.Background(), store, runID)

    require.NoError(t, err)
    assert.Equal(t, []string{"c"}, executed)  // Only c ran
}

func TestResume_ReplayNode(t *testing.T) {
    var executed []string
    makeNode := func(name string) NodeFunc[testState] {
        return func(ctx Context, s testState) (testState, error) {
            executed = append(executed, name)
            return s, nil
        }
    }

    store := NewMemoryStore()
    compiled, _ := graph.Compile()

    runID := "test-run"
    store.Save(runID, "a", mustMarshalCheckpoint(testState{}))
    store.Save(runID, "b", mustMarshalCheckpoint(testState{}))

    // Resume with replay
    _, err := compiled.Resume(context.Background(), store, runID,
        flowgraph.ReplayCheckpointNode(),
    )

    require.NoError(t, err)
    assert.Equal(t, []string{"b", "c"}, executed)  // b replayed, then c
}

func TestResume_ValidationFailure(t *testing.T) {
    store := NewMemoryStore()
    compiled, _ := graph.Compile()

    runID := "test-run"
    store.Save(runID, "a", mustMarshalCheckpoint(testState{}))

    _, err := compiled.Resume(context.Background(), store, runID,
        flowgraph.WithStateValidation(func(s testState) error {
            return errors.New("external state changed")
        }),
    )

    require.Error(t, err)
    assert.Contains(t, err.Error(), "external state changed")
}
```
