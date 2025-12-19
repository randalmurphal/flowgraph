# Feature: Resume from Checkpoint

**Related ADRs**: 016-resume-strategy

---

## Problem Statement

After a crash or failure, workflows need to resume from their last checkpoint:
1. Load the saved state
2. Determine where to continue
3. Re-execute from that point
4. Handle state that may have changed externally

## User Stories

- As a developer, I want to resume crashed workflows so that progress isn't lost
- As a developer, I want to resume from specific nodes so that I can retry failed steps
- As a developer, I want clear feedback on where resume starts so that I can debug issues
- As a developer, I want to skip already-completed work so that resume is efficient

---

## API Design

### Resume Method

```go
// Resume continues execution from the last checkpoint
// Loads state from store and continues from the next node
func (cg *CompiledGraph[S]) Resume(ctx Context, store CheckpointStore, runID string, opts ...RunOption) (S, error)

// ResumeFrom continues from a specific node
// Useful for retrying a failed step
func (cg *CompiledGraph[S]) ResumeFrom(ctx Context, store CheckpointStore, runID, nodeID string, opts ...RunOption) (S, error)
```

### Resume Options

```go
// WithStateOverride allows modifying the loaded state before resuming
// Useful when external systems have changed
func WithStateOverride[S any](fn func(S) S) RunOption

// WithRevalidate re-runs validation on loaded state
func WithRevalidate(fn func(S) error) RunOption
```

---

## Behavior Specification

### Resume Flow

```
Resume(ctx, store, runID) called
        │
        ▼
Find latest checkpoint for runID
        │
        ├── not found ──► return ErrNoCheckpointFound
        │
        │ found
        ▼
Deserialize state
        │
        ├── error ──► return ErrDeserializeState
        │
        │ ok
        ▼
Apply state override (if configured)
        │
        ▼
Run validation (if configured)
        │
        ├── error ──► return validation error
        │
        │ ok
        ▼
Continue from checkpoint.NextNode
        │
        ▼
Normal execution continues
```

### Determining Resume Point

The checkpoint stores which node completed and what comes next:

```go
type Checkpoint struct {
    NodeID   string  // Node that completed
    NextNode string  // Where to resume
    State    []byte  // State after NodeID
}
```

Resume starts at `NextNode`, not re-executing `NodeID`.

### State Deserialization

State is deserialized back into the type parameter:

```go
// Checkpoint saved with:
//   State: {"count": 5, "status": "processing"}

// Resume loads:
var state MyState
json.Unmarshal(checkpoint.State, &state)
// state.Count == 5, state.Status == "processing"
```

### Handling External Changes

Use `WithStateOverride` when external state may have changed:

```go
result, err := compiled.Resume(ctx, store, runID,
    flowgraph.WithStateOverride(func(s State) State {
        // Refresh from database
        s.Order = db.GetOrder(s.OrderID)
        return s
    }))
```

### Resume vs Fresh Run

| Aspect | Fresh Run | Resume |
|--------|-----------|--------|
| Starting point | Entry node | Last checkpoint |
| Initial state | Caller provides | Loaded from checkpoint |
| Checkpoints | Creates new | Appends to existing |

---

## Error Cases

```go
var (
    ErrNoCheckpointFound  = errors.New("no checkpoint found for run")
    ErrDeserializeState   = errors.New("failed to deserialize state")
    ErrInvalidResumeNode  = errors.New("resume node not in graph")
    ErrResumeNodeCompleted = errors.New("resume node was the last completed")
)
```

### ResumeFrom Errors

```go
// ResumeFrom specific node
result, err := compiled.ResumeFrom(ctx, store, runID, "process")

if errors.Is(err, flowgraph.ErrInvalidResumeNode) {
    // Node doesn't exist in graph
}

if errors.Is(err, flowgraph.ErrNoCheckpointFound) {
    // No checkpoint at that node
}
```

---

## Test Cases

### Basic Resume

```go
func TestResume_FromLastCheckpoint(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    var executed []string
    track := func(name string) flowgraph.NodeFunc[State] {
        return func(ctx flowgraph.Context, s State) (State, error) {
            executed = append(executed, name)
            return s, nil
        }
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("a", track("a")).
        AddNode("b", track("b")).
        AddNode("c", track("c")).
        AddEdge("a", "b").
        AddEdge("b", "c").
        AddEdge("c", flowgraph.END).
        SetEntry("a")

    compiled, _ := graph.Compile()

    // Simulate: Run completed A and B, crashed before C
    checkpoint := flowgraph.Checkpoint{
        RunID:    "run-1",
        NodeID:   "b",
        NextNode: "c",
        State:    []byte(`{}`),
    }
    data, _ := json.Marshal(checkpoint)
    store.Save("run-1", "b", data)

    // Resume
    executed = nil
    result, err := compiled.Resume(ctx, store, "run-1")

    require.NoError(t, err)
    assert.Equal(t, []string{"c"}, executed)  // Only C ran
}
```

### Resume with State

```go
func TestResume_LoadsState(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    var receivedState State
    nodeC := func(ctx flowgraph.Context, s State) (State, error) {
        receivedState = s
        return s, nil
    }

    // ... build graph ...

    // Checkpoint with state
    checkpoint := flowgraph.Checkpoint{
        RunID:    "run-1",
        NodeID:   "b",
        NextNode: "c",
        State:    []byte(`{"count": 42, "status": "processing"}`),
    }
    data, _ := json.Marshal(checkpoint)
    store.Save("run-1", "b", data)

    compiled.Resume(ctx, store, "run-1")

    assert.Equal(t, 42, receivedState.Count)
    assert.Equal(t, "processing", receivedState.Status)
}
```

### Resume from Specific Node

```go
func TestResumeFrom_SpecificNode(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    // Checkpoints at A and B
    saveCheckpoint(store, "run-1", "a", State{Step: 1})
    saveCheckpoint(store, "run-1", "b", State{Step: 2})

    // Resume from A (re-run B and C)
    result, err := compiled.ResumeFrom(ctx, store, "run-1", "a")

    require.NoError(t, err)
    // B and C should have executed
}
```

### No Checkpoint Found

```go
func TestResume_NoCheckpoint_Error(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    _, err := compiled.Resume(ctx, store, "nonexistent-run")

    assert.ErrorIs(t, err, flowgraph.ErrNoCheckpointFound)
}
```

### State Override

```go
func TestResume_WithStateOverride(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    // Checkpoint with old data
    saveCheckpoint(store, "run-1", "fetch", State{
        OrderID: "123",
        Order:   Order{Amount: 100},  // Old value
    })

    var resumedState State
    nodeProcess := func(ctx flowgraph.Context, s State) (State, error) {
        resumedState = s
        return s, nil
    }

    result, err := compiled.Resume(ctx, store, "run-1",
        flowgraph.WithStateOverride(func(s State) State {
            s.Order.Amount = 200  // Updated value
            return s
        }))

    require.NoError(t, err)
    assert.Equal(t, 200, resumedState.Order.Amount)
}
```

### Validation on Resume

```go
func TestResume_WithValidation(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    saveCheckpoint(store, "run-1", "fetch", State{
        OrderID: "123",
        Status:  "cancelled",  // Invalid for processing
    })

    _, err := compiled.Resume(ctx, store, "run-1",
        flowgraph.WithRevalidate(func(s State) error {
            if s.Status == "cancelled" {
                return errors.New("order was cancelled")
            }
            return nil
        }))

    assert.ErrorContains(t, err, "order was cancelled")
}
```

### Invalid Resume Node

```go
func TestResumeFrom_InvalidNode_Error(t *testing.T) {
    store := flowgraph.NewMemoryStore()
    saveCheckpoint(store, "run-1", "a", State{})

    // Try to resume from node not in graph
    _, err := compiled.ResumeFrom(ctx, store, "run-1", "nonexistent")

    assert.ErrorIs(t, err, flowgraph.ErrInvalidResumeNode)
}
```

### Resume Continues Checkpointing

```go
func TestResume_ContinuesCheckpointing(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    // Existing checkpoint at B
    saveCheckpoint(store, "run-1", "b", State{})

    // Resume and run C
    compiled.Resume(ctx, store, "run-1",
        flowgraph.WithCheckpointing(store),
        flowgraph.WithRunID("run-1"))

    // Should now have checkpoint for C too
    infos, _ := store.List("run-1")
    nodeIDs := []string{}
    for _, info := range infos {
        nodeIDs = append(nodeIDs, info.NodeID)
    }
    assert.Contains(t, nodeIDs, "c")
}
```

---

## Performance Requirements

| Operation | Target |
|-----------|--------|
| Find latest checkpoint | O(n) where n = checkpoints for run |
| State deserialization | < 1ms for 100KB state |
| Resume overhead | < 1ms beyond normal execution |

---

## Security Considerations

1. **State tampering**: Verify checkpoint integrity before resume
2. **Stale state**: Use `WithRevalidate` for time-sensitive data
3. **Access control**: Ensure only authorized users can resume runs

---

## Simplicity Check

**What we included**:
- `Resume()` from last checkpoint
- `ResumeFrom()` from specific node
- State override for external changes
- Validation hook

**What we did NOT include**:
- Automatic retry logic - User decides when to resume
- Checkpoint selection UI - User provides run ID
- State migration - User handles schema changes via override
- Automatic state refresh - User implements in override function
- Resume policies - Keep it simple: load and continue

**Is this the simplest solution?** Yes. Find checkpoint, load state, continue execution.
