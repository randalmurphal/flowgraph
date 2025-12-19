# Phase 3: Checkpointing

**Status**: Blocked (Depends on Phase 2)
**Estimated Effort**: 2-3 days
**Dependencies**: Phase 2 Complete

---

## Goal

Enable persistent checkpoints and crash recovery through pluggable storage backends.

---

## Files to Create

```
pkg/flowgraph/
├── checkpoint/
│   ├── store.go         # CheckpointStore interface
│   ├── checkpoint.go    # Checkpoint type, serialization
│   ├── memory.go        # MemoryStore implementation
│   ├── sqlite.go        # SQLiteStore implementation
│   ├── store_test.go    # Contract tests for all stores
│   ├── memory_test.go   # MemoryStore-specific tests
│   └── sqlite_test.go   # SQLiteStore-specific tests
├── options.go           # MODIFY: Add checkpoint options
├── execute.go           # MODIFY: Add checkpoint hooks
├── resume.go            # NEW: Resume logic
└── checkpoint_test.go   # Integration tests
```

---

## Implementation Order

### Step 1: Core Types (~2 hours)

**checkpoint/store.go**
```go
package checkpoint

import (
    "errors"
    "time"
)

// CheckpointStore persists checkpoints for crash recovery
type CheckpointStore interface {
    // Save stores a checkpoint for a run at a specific node
    Save(runID, nodeID string, data []byte) error

    // Load retrieves a checkpoint
    Load(runID, nodeID string) ([]byte, error)

    // List returns all checkpoints for a run, ordered by sequence
    List(runID string) ([]CheckpointInfo, error)

    // Delete removes a specific checkpoint
    Delete(runID, nodeID string) error

    // DeleteRun removes all checkpoints for a run
    DeleteRun(runID string) error

    // Close releases any resources
    Close() error
}

// CheckpointInfo provides metadata without loading full state
type CheckpointInfo struct {
    RunID     string
    NodeID    string
    Sequence  int
    Timestamp time.Time
    Size      int64
}

// Sentinel errors
var (
    ErrCheckpointNotFound = errors.New("checkpoint not found")
)
```

**checkpoint/checkpoint.go**
```go
package checkpoint

import (
    "encoding/json"
    "time"
)

const Version = "1.0"

// Checkpoint is the persisted snapshot
type Checkpoint struct {
    // Metadata
    RunID     string    `json:"run_id"`
    NodeID    string    `json:"node_id"`
    Sequence  int       `json:"sequence"`
    Timestamp time.Time `json:"timestamp"`
    Version   string    `json:"version"`

    // Execution state
    State    json.RawMessage `json:"state"`
    NextNode string          `json:"next_node"`
}

// Marshal serializes a checkpoint
func (c *Checkpoint) Marshal() ([]byte, error) {
    return json.Marshal(c)
}

// Unmarshal deserializes a checkpoint
func Unmarshal(data []byte) (*Checkpoint, error) {
    var c Checkpoint
    if err := json.Unmarshal(data, &c); err != nil {
        return nil, err
    }
    return &c, nil
}
```

### Step 2: Memory Store (~1 hour)

**checkpoint/memory.go**
```go
package checkpoint

import (
    "sync"
    "time"
)

// MemoryStore is an in-memory checkpoint store for testing
type MemoryStore struct {
    mu          sync.RWMutex
    data        map[string]map[string][]byte  // runID -> nodeID -> data
    sequences   map[string]int                 // runID -> next sequence
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
    return &MemoryStore{
        data:      make(map[string]map[string][]byte),
        sequences: make(map[string]int),
    }
}

func (m *MemoryStore) Save(runID, nodeID string, data []byte) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.data[runID] == nil {
        m.data[runID] = make(map[string][]byte)
    }

    // Copy data to avoid retaining caller's slice
    stored := make([]byte, len(data))
    copy(stored, data)
    m.data[runID][nodeID] = stored

    m.sequences[runID]++
    return nil
}

func (m *MemoryStore) Load(runID, nodeID string) ([]byte, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    run, ok := m.data[runID]
    if !ok {
        return nil, ErrCheckpointNotFound
    }

    data, ok := run[nodeID]
    if !ok {
        return nil, ErrCheckpointNotFound
    }

    // Return a copy
    result := make([]byte, len(data))
    copy(result, data)
    return result, nil
}

func (m *MemoryStore) List(runID string) ([]CheckpointInfo, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    run, ok := m.data[runID]
    if !ok {
        return nil, nil
    }

    var infos []CheckpointInfo
    seq := 1
    for nodeID, data := range run {
        infos = append(infos, CheckpointInfo{
            RunID:     runID,
            NodeID:    nodeID,
            Sequence:  seq,
            Timestamp: time.Now(),
            Size:      int64(len(data)),
        })
        seq++
    }
    return infos, nil
}

func (m *MemoryStore) Delete(runID, nodeID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if run, ok := m.data[runID]; ok {
        delete(run, nodeID)
    }
    return nil
}

func (m *MemoryStore) DeleteRun(runID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    delete(m.data, runID)
    delete(m.sequences, runID)
    return nil
}

func (m *MemoryStore) Close() error {
    return nil
}
```

### Step 3: SQLite Store (~3 hours)

**checkpoint/sqlite.go**
```go
package checkpoint

import (
    "database/sql"
    "time"

    _ "modernc.org/sqlite"  // Pure Go SQLite
)

// SQLiteStore persists checkpoints to SQLite
type SQLiteStore struct {
    db *sql.DB
}

// NewSQLiteStore creates a new SQLite checkpoint store
func NewSQLiteStore(path string) (*SQLiteStore, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }

    // Create table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS checkpoints (
            run_id TEXT NOT NULL,
            node_id TEXT NOT NULL,
            sequence INTEGER NOT NULL,
            timestamp TEXT NOT NULL,
            data BLOB NOT NULL,
            PRIMARY KEY (run_id, node_id)
        );
        CREATE INDEX IF NOT EXISTS idx_checkpoints_run_id
            ON checkpoints(run_id);
    `)
    if err != nil {
        db.Close()
        return nil, err
    }

    return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Save(runID, nodeID string, data []byte) error {
    _, err := s.db.Exec(`
        INSERT INTO checkpoints (run_id, node_id, sequence, timestamp, data)
        VALUES (
            ?, ?,
            COALESCE((SELECT MAX(sequence) FROM checkpoints WHERE run_id = ?), 0) + 1,
            ?, ?
        )
        ON CONFLICT(run_id, node_id) DO UPDATE SET
            sequence = excluded.sequence,
            timestamp = excluded.timestamp,
            data = excluded.data
    `, runID, nodeID, runID, time.Now().UTC().Format(time.RFC3339Nano), data)
    return err
}

func (s *SQLiteStore) Load(runID, nodeID string) ([]byte, error) {
    var data []byte
    err := s.db.QueryRow(`
        SELECT data FROM checkpoints
        WHERE run_id = ? AND node_id = ?
    `, runID, nodeID).Scan(&data)

    if err == sql.ErrNoRows {
        return nil, ErrCheckpointNotFound
    }
    return data, err
}

func (s *SQLiteStore) List(runID string) ([]CheckpointInfo, error) {
    rows, err := s.db.Query(`
        SELECT node_id, sequence, timestamp, LENGTH(data)
        FROM checkpoints
        WHERE run_id = ?
        ORDER BY sequence
    `, runID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var infos []CheckpointInfo
    for rows.Next() {
        var info CheckpointInfo
        var timestamp string
        if err := rows.Scan(&info.NodeID, &info.Sequence, &timestamp, &info.Size); err != nil {
            return nil, err
        }
        info.RunID = runID
        info.Timestamp, _ = time.Parse(time.RFC3339Nano, timestamp)
        infos = append(infos, info)
    }
    return infos, rows.Err()
}

func (s *SQLiteStore) Delete(runID, nodeID string) error {
    _, err := s.db.Exec(`
        DELETE FROM checkpoints
        WHERE run_id = ? AND node_id = ?
    `, runID, nodeID)
    return err
}

func (s *SQLiteStore) DeleteRun(runID string) error {
    _, err := s.db.Exec(`
        DELETE FROM checkpoints WHERE run_id = ?
    `, runID)
    return err
}

func (s *SQLiteStore) Close() error {
    return s.db.Close()
}
```

### Step 4: Execution Integration (~2 hours)

**options.go additions**
```go
// WithCheckpointing enables checkpoint saving during execution
func WithCheckpointing(store CheckpointStore) RunOption {
    return func(c *runConfig) {
        c.checkpointStore = store
    }
}

// WithRunID sets the run identifier for checkpointing
func WithRunID(id string) RunOption {
    return func(c *runConfig) {
        c.runID = id
    }
}

// WithCheckpointFailureFatal makes checkpoint failures stop execution
func WithCheckpointFailureFatal(fatal bool) RunOption {
    return func(c *runConfig) {
        c.checkpointFailureFatal = fatal
    }
}
```

**execute.go additions**
```go
// After node execution, before moving to next node:
func (cg *CompiledGraph[S]) checkpoint(ctx Context, cfg *runConfig, nodeID string, state S, nextNode string) error {
    if cfg.checkpointStore == nil {
        return nil
    }

    if cfg.runID == "" {
        return ErrRunIDRequired
    }

    // Serialize state
    stateBytes, err := json.Marshal(state)
    if err != nil {
        if cfg.checkpointFailureFatal {
            return fmt.Errorf("%w: %v", ErrSerializeState, err)
        }
        ctx.Logger().Warn("checkpoint serialization failed",
            "node", nodeID, "error", err)
        return nil
    }

    // Create checkpoint
    cp := &checkpoint.Checkpoint{
        RunID:     cfg.runID,
        NodeID:    nodeID,
        Sequence:  cfg.sequence,
        Timestamp: time.Now(),
        Version:   checkpoint.Version,
        State:     stateBytes,
        NextNode:  nextNode,
    }

    data, err := cp.Marshal()
    if err != nil {
        if cfg.checkpointFailureFatal {
            return err
        }
        ctx.Logger().Warn("checkpoint marshal failed",
            "node", nodeID, "error", err)
        return nil
    }

    // Save
    if err := cfg.checkpointStore.Save(cfg.runID, nodeID, data); err != nil {
        if cfg.checkpointFailureFatal {
            return err
        }
        ctx.Logger().Warn("checkpoint save failed",
            "node", nodeID, "error", err)
    }

    cfg.sequence++
    return nil
}
```

### Step 5: Resume Logic (~2 hours)

**resume.go**
```go
package flowgraph

import (
    "encoding/json"
    "fmt"

    "github.com/yourorg/flowgraph/checkpoint"
)

// Resume continues execution from the last checkpoint
func (cg *CompiledGraph[S]) Resume(ctx Context, store checkpoint.CheckpointStore, runID string, opts ...RunOption) (S, error) {
    var zero S

    // Find latest checkpoint
    infos, err := store.List(runID)
    if err != nil {
        return zero, err
    }
    if len(infos) == 0 {
        return zero, fmt.Errorf("%w: %s", checkpoint.ErrCheckpointNotFound, runID)
    }

    // Load latest
    latest := infos[len(infos)-1]
    data, err := store.Load(runID, latest.NodeID)
    if err != nil {
        return zero, err
    }

    cp, err := checkpoint.Unmarshal(data)
    if err != nil {
        return zero, fmt.Errorf("%w: %v", ErrDeserializeState, err)
    }

    // Deserialize state
    var state S
    if err := json.Unmarshal(cp.State, &state); err != nil {
        return zero, fmt.Errorf("%w: %v", ErrDeserializeState, err)
    }

    // Apply options
    cfg := defaultRunConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    // Apply state override if configured
    if cfg.stateOverride != nil {
        state = cfg.stateOverride(state)
    }

    // Apply validation if configured
    if cfg.validateState != nil {
        if err := cfg.validateState(state); err != nil {
            return state, err
        }
    }

    // Continue from next node
    return cg.runFrom(ctx, state, cp.NextNode, &cfg)
}

// ResumeFrom continues from a specific node
func (cg *CompiledGraph[S]) ResumeFrom(ctx Context, store checkpoint.CheckpointStore, runID, nodeID string, opts ...RunOption) (S, error) {
    var zero S

    // Validate node exists
    if !cg.HasNode(nodeID) && nodeID != END {
        return zero, fmt.Errorf("%w: %s", ErrInvalidResumeNode, nodeID)
    }

    // Load checkpoint at specified node
    data, err := store.Load(runID, nodeID)
    if err != nil {
        return zero, err
    }

    cp, err := checkpoint.Unmarshal(data)
    if err != nil {
        return zero, fmt.Errorf("%w: %v", ErrDeserializeState, err)
    }

    var state S
    if err := json.Unmarshal(cp.State, &state); err != nil {
        return zero, fmt.Errorf("%w: %v", ErrDeserializeState, err)
    }

    cfg := defaultRunConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    if cfg.stateOverride != nil {
        state = cfg.stateOverride(state)
    }

    if cfg.validateState != nil {
        if err := cfg.validateState(state); err != nil {
            return state, err
        }
    }

    return cg.runFrom(ctx, state, cp.NextNode, &cfg)
}
```

### Step 6: Tests (~4 hours)

**checkpoint/store_test.go** - Contract tests run against all implementations
**checkpoint/memory_test.go** - MemoryStore-specific edge cases
**checkpoint/sqlite_test.go** - SQLiteStore-specific tests (file handling, concurrency)
**checkpoint_test.go** - Integration tests with graph execution

---

## Acceptance Criteria

```go
// Basic checkpointing
store := flowgraph.NewSQLiteStore("./checkpoints.db")
defer store.Close()

result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))

// Checkpoints should exist for each node
infos, _ := store.List("run-123")
// len(infos) == number of nodes executed
```

```go
// Resume after crash
// Previous run: A -> B -> [crash] -> C -> END
// Checkpoint exists at B

result, err := compiled.Resume(ctx, store, "run-123")
// Should continue from C, not re-run A and B
```

```go
// Resume from specific node (retry failed step)
result, err := compiled.ResumeFrom(ctx, store, "run-123", "b",
    flowgraph.WithStateOverride(func(s State) State {
        s.FixedData = "corrected"
        return s
    }))
```

---

## Test Coverage Targets

| File | Target |
|------|--------|
| checkpoint/store.go | 100% (interface, errors only) |
| checkpoint/checkpoint.go | 95% |
| checkpoint/memory.go | 95% |
| checkpoint/sqlite.go | 85% |
| resume.go | 90% |
| Overall Phase 3 | 85% |

---

## Checklist

- [ ] CheckpointStore interface defined
- [ ] Checkpoint type with serialization
- [ ] MemoryStore implementation
- [ ] SQLiteStore implementation
- [ ] WithCheckpointing, WithRunID options
- [ ] Checkpoint integration in Run()
- [ ] Resume() method
- [ ] ResumeFrom() method
- [ ] WithStateOverride option
- [ ] Contract tests for all stores
- [ ] Integration tests
- [ ] 85% coverage achieved
- [ ] No race conditions

---

## Dependencies

```go
// go.mod additions
require (
    modernc.org/sqlite v1.29.0  // Pure Go SQLite, no CGO
)
```

---

## Notes

- MemoryStore for testing only - data lost on restart
- SQLiteStore for single-process production
- Postgres store deferred to later (higher complexity, can use SQLiteStore for v1)
- Checkpoints are saved after successful node execution, before moving to next
- Failed nodes don't get checkpointed (resume from previous node)
