# Feature: Checkpointing

**Related ADRs**: 014-checkpoint-format, 015-checkpoint-store, 017-state-serialization

---

## Problem Statement

Long-running workflows need crash recovery:
1. If the process crashes, resume from the last checkpoint
2. If a node fails, retry from a known-good state
3. Audit and replay historical executions

Checkpointing saves state snapshots at defined points during execution.

## User Stories

- As a developer, I want automatic checkpointing so that crashes don't lose progress
- As a developer, I want to resume from checkpoints so that I can recover from failures
- As a developer, I want pluggable storage so that I can use SQLite locally, Postgres in production
- As a developer, I want checkpoints to include metadata so that I can debug and audit

---

## API Design

### CheckpointStore Interface

```go
// CheckpointStore persists checkpoints for crash recovery
type CheckpointStore interface {
    // Save stores a checkpoint for a run at a specific node
    // Overwrites if checkpoint for (runID, nodeID) already exists
    Save(runID, nodeID string, data []byte) error

    // Load retrieves a checkpoint
    // Returns ErrCheckpointNotFound if checkpoint doesn't exist
    Load(runID, nodeID string) ([]byte, error)

    // List returns all checkpoints for a run, ordered by sequence
    List(runID string) ([]CheckpointInfo, error)

    // Delete removes a specific checkpoint
    Delete(runID, nodeID string) error

    // DeleteRun removes all checkpoints for a run
    DeleteRun(runID string) error

    // Close releases any resources (connections, files)
    Close() error
}

// CheckpointInfo provides metadata without loading full state
type CheckpointInfo struct {
    RunID     string
    NodeID    string
    Sequence  int       // Execution order
    Timestamp time.Time
    Size      int64     // Bytes
}
```

### Checkpoint Type

```go
// Checkpoint is the persisted snapshot
type Checkpoint struct {
    // Metadata
    RunID     string    `json:"run_id"`
    NodeID    string    `json:"node_id"`
    Sequence  int       `json:"sequence"`
    Timestamp time.Time `json:"timestamp"`
    Version   string    `json:"version"`  // Schema version

    // Execution state
    State     json.RawMessage `json:"state"`
    NextNode  string          `json:"next_node"`

    // Optional compression
    Compressed bool   `json:"compressed,omitempty"`
}
```

### Run Options

```go
// WithCheckpointing enables checkpoint saving during execution
func WithCheckpointing(store CheckpointStore) RunOption

// WithRunID sets the run identifier for checkpointing
// Required when checkpointing is enabled
func WithRunID(id string) RunOption

// WithCheckpointAfter specifies when to checkpoint
// Default: after every node
func WithCheckpointAfter(strategy CheckpointStrategy) RunOption

type CheckpointStrategy int
const (
    CheckpointEveryNode  CheckpointStrategy = iota  // Default
    CheckpointOnSuccess                              // Only on successful completion
    CheckpointOnError                                // Only when errors occur
)
```

### Store Constructors

```go
// MemoryStore for testing
func NewMemoryStore() *MemoryStore

// SQLiteStore for single-node production
func NewSQLiteStore(path string) (*SQLiteStore, error)

// PostgresStore for multi-node production
func NewPostgresStore(db *sql.DB) (*PostgresStore, error)
```

---

## Behavior Specification

### Checkpoint Timing

By default, checkpoints are saved **after** each node executes successfully:

```
Node A executes
    │
    ▼
State updated
    │
    ▼
Checkpoint saved (runID, "a", state)
    │
    ▼
Move to next node
```

### Checkpoint Contents

Each checkpoint contains:

| Field | Description |
|-------|-------------|
| `run_id` | Unique identifier for this execution |
| `node_id` | Node that just completed |
| `sequence` | Monotonically increasing counter |
| `timestamp` | When checkpoint was created |
| `version` | Schema version for migration |
| `state` | JSON-serialized state |
| `next_node` | Next node to execute |

### State Serialization (ADR-017)

State is serialized to JSON:
- Exported fields only
- Must be JSON-serializable
- Pointers to non-serializable types (channels, funcs) will error

```go
// Good - fully serializable
type GoodState struct {
    ID     string
    Count  int
    Items  []Item
}

// Bad - will fail
type BadState struct {
    ID     string
    done   chan bool  // Unexported, ignored
    Notify func()     // func cannot be serialized
}
```

### Checkpoint Storage Patterns

**Memory Store**: Testing only. Lost on restart.

**SQLite Store**: Single process. Fast. Survives restart.

**Postgres Store**: Multi-process. Distributed. Production-grade.

### Error Handling

Checkpoint failures are **non-fatal warnings** by default:

```go
// If Save fails:
// 1. Log warning
// 2. Continue execution
// Rationale: Losing a checkpoint is better than failing the workflow
```

Can be made fatal with option:

```go
result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithCheckpointFailureFatal(true),
)
```

---

## Error Cases

```go
var (
    ErrCheckpointNotFound = errors.New("checkpoint not found")
    ErrRunIDRequired      = errors.New("run ID required for checkpointing")
    ErrSerializeState     = errors.New("failed to serialize state")
)
```

---

## Test Cases

### Basic Checkpointing

```go
func TestCheckpointing_SavesAfterEachNode(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    graph := flowgraph.NewGraph[State]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddNode("c", nodeC).
        AddEdge("a", "b").
        AddEdge("b", "c").
        AddEdge("c", flowgraph.END).
        SetEntry("a")

    compiled, _ := graph.Compile()

    _, err := compiled.Run(ctx, State{},
        flowgraph.WithCheckpointing(store),
        flowgraph.WithRunID("run-1"))

    require.NoError(t, err)

    // Verify checkpoints
    infos, _ := store.List("run-1")
    assert.Len(t, infos, 3)  // One per node
    assert.Equal(t, "a", infos[0].NodeID)
    assert.Equal(t, "b", infos[1].NodeID)
    assert.Equal(t, "c", infos[2].NodeID)
}

func TestCheckpointing_StatePreserved(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    nodeB := func(ctx Context, s State) (State, error) {
        s.Progress = "at-b"
        return s, nil
    }

    // ... build graph ...

    compiled.Run(ctx, State{},
        flowgraph.WithCheckpointing(store),
        flowgraph.WithRunID("run-1"))

    // Load checkpoint
    data, _ := store.Load("run-1", "b")
    var checkpoint flowgraph.Checkpoint
    json.Unmarshal(data, &checkpoint)

    var state State
    json.Unmarshal(checkpoint.State, &state)

    assert.Equal(t, "at-b", state.Progress)
}
```

### Checkpoint Store Implementations

```go
func TestCheckpointStore_Contract(t *testing.T) {
    stores := []struct {
        name  string
        store func(t *testing.T) flowgraph.CheckpointStore
    }{
        {"memory", func(t *testing.T) flowgraph.CheckpointStore {
            return flowgraph.NewMemoryStore()
        }},
        {"sqlite", func(t *testing.T) flowgraph.CheckpointStore {
            store, err := flowgraph.NewSQLiteStore(t.TempDir() + "/test.db")
            require.NoError(t, err)
            return store
        }},
    }

    for _, tt := range stores {
        t.Run(tt.name, func(t *testing.T) {
            store := tt.store(t)
            defer store.Close()

            // Save
            err := store.Save("run-1", "node-a", []byte(`{"value":1}`))
            require.NoError(t, err)

            // Load
            data, err := store.Load("run-1", "node-a")
            require.NoError(t, err)
            assert.JSONEq(t, `{"value":1}`, string(data))

            // Not found
            _, err = store.Load("run-1", "nonexistent")
            assert.ErrorIs(t, err, flowgraph.ErrCheckpointNotFound)

            // List
            store.Save("run-1", "node-b", []byte(`{"value":2}`))
            infos, err := store.List("run-1")
            require.NoError(t, err)
            assert.Len(t, infos, 2)

            // Delete
            err = store.Delete("run-1", "node-a")
            require.NoError(t, err)
            _, err = store.Load("run-1", "node-a")
            assert.ErrorIs(t, err, flowgraph.ErrCheckpointNotFound)

            // DeleteRun
            err = store.DeleteRun("run-1")
            require.NoError(t, err)
            infos, _ = store.List("run-1")
            assert.Empty(t, infos)
        })
    }
}
```

### Serialization

```go
func TestCheckpointing_SerializableState(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    type State struct {
        ID      string
        Count   int
        Items   []string
        Nested  *NestedStruct
    }

    // ... run graph with this state ...

    // Should work - all fields serializable
    require.NoError(t, err)
}

func TestCheckpointing_UnserializableState_Error(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    type BadState struct {
        Callback func()  // Cannot serialize
    }

    nodeA := func(ctx Context, s BadState) (BadState, error) {
        s.Callback = func() {}
        return s, nil
    }

    // ... build graph ...

    _, err := compiled.Run(ctx, BadState{},
        flowgraph.WithCheckpointing(store),
        flowgraph.WithRunID("run-1"),
        flowgraph.WithCheckpointFailureFatal(true))

    assert.ErrorIs(t, err, flowgraph.ErrSerializeState)
}
```

### RunID Required

```go
func TestCheckpointing_RequiresRunID(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    _, err := compiled.Run(ctx, State{},
        flowgraph.WithCheckpointing(store))
        // No WithRunID!

    assert.ErrorIs(t, err, flowgraph.ErrRunIDRequired)
}
```

### Checkpoint Strategies

```go
func TestCheckpointing_OnlyOnSuccess(t *testing.T) {
    store := flowgraph.NewMemoryStore()

    failingNode := func(ctx Context, s State) (State, error) {
        return s, errors.New("failed")
    }

    // ... graph: a -> fail -> END ...

    compiled.Run(ctx, State{},
        flowgraph.WithCheckpointing(store),
        flowgraph.WithRunID("run-1"),
        flowgraph.WithCheckpointAfter(flowgraph.CheckpointOnSuccess))

    // Only successful nodes checkpointed
    infos, _ := store.List("run-1")
    assert.Len(t, infos, 1)  // Only "a"
}
```

---

## Performance Requirements

| Operation | Target |
|-----------|--------|
| Checkpoint serialization | < 1ms for 100KB state |
| MemoryStore Save | < 10 microseconds |
| SQLiteStore Save | < 1 millisecond |
| PostgresStore Save | < 10 milliseconds |

### Benchmarks

```go
func BenchmarkCheckpoint_Serialize(b *testing.B) {
    state := createLargeState(1000)  // 1000 items
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        json.Marshal(state)
    }
}

func BenchmarkMemoryStore_Save(b *testing.B) {
    store := flowgraph.NewMemoryStore()
    data := make([]byte, 10000)  // 10KB
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        store.Save("run", fmt.Sprintf("node-%d", i), data)
    }
}
```

---

## Security Considerations

1. **State contains secrets**: Use encrypted stores for sensitive data
2. **Checkpoint tampering**: SQLite/Postgres provide integrity; add HMAC for file stores
3. **Access control**: Store implementations should handle auth (Postgres roles, file permissions)

---

## Simplicity Check

**What we included**:
- Simple CRUD interface
- Three store implementations (memory, SQLite, Postgres)
- JSON serialization
- Metadata (run ID, node ID, sequence, timestamp)
- Configurable checkpoint strategy

**What we did NOT include**:
- Streaming for large states - States should be reasonably sized (< 10MB)
- Automatic compression - Layer on top if needed
- Encryption - Layer on top if needed
- Checkpoint diffing - Full state each time is simpler
- Automatic cleanup/retention - User manages or implements
- Event sourcing - Checkpoints are snapshots, not events

**Is this the simplest solution?** Yes. Save bytes, load bytes, list, delete. JSON handles serialization.
