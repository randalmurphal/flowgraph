# ADR-015: Checkpoint Store Interface

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

What interface should checkpoint stores implement? What operations are needed?

## Decision

**Simple CRUD interface with list and cleanup operations.**

```go
// CheckpointStore persists checkpoints for crash recovery
type CheckpointStore interface {
    // Save stores a checkpoint for a run at a specific node
    // Overwrites if checkpoint for (runID, nodeID) already exists
    Save(runID, nodeID string, data []byte) error

    // Load retrieves a checkpoint
    // Returns ErrNotFound if checkpoint doesn't exist
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
    Sequence  int
    Timestamp time.Time
    Size      int64
}

// Sentinel errors
var (
    ErrCheckpointNotFound = errors.New("checkpoint not found")
)
```

### Store Implementations

| Store | Use Case | Durability |
|-------|----------|------------|
| MemoryStore | Testing, short-lived processes | None |
| SQLiteStore | Single-node production | Disk |
| PostgresStore | Multi-node production | Database |
| FileStore | Simple persistence | Disk |

## Alternatives Considered

### 1. Key-Value Only Interface

```go
type CheckpointStore interface {
    Set(key string, data []byte) error
    Get(key string) ([]byte, error)
}
```

**Rejected**: Too generic. Loses semantic meaning, harder to implement List.

### 2. Streaming Interface

```go
type CheckpointStore interface {
    Save(runID, nodeID string) (io.WriteCloser, error)
    Load(runID, nodeID string) (io.ReadCloser, error)
}
```

**Rejected for v1**: Over-engineered. Checkpoints are typically < 10MB.

### 3. Versioned Interface

```go
type CheckpointStore interface {
    Save(runID, nodeID string, data []byte, version int) error
    Load(runID, nodeID string, version int) ([]byte, error)
    ListVersions(runID, nodeID string) ([]int, error)
}
```

**Rejected for v1**: Adds complexity. Single checkpoint per (run, node) is sufficient.

### 4. Batch Interface

```go
type CheckpointStore interface {
    SaveBatch(checkpoints []Checkpoint) error
}
```

**Rejected for v1**: Not needed for sequential execution. Can add later for parallel.

## Consequences

### Positive
- **Simple** - Easy to implement for any storage backend
- **Testable** - MemoryStore for unit tests
- **Complete** - CRUD + list + cleanup covers all needs
- **Efficient** - CheckpointInfo avoids loading full data for listing

### Negative
- No streaming (large checkpoints must fit in memory)
- No batch operations (one at a time)

### Risks
- Very large states → Mitigate: Compress, warn on size

---

## Implementation: MemoryStore

```go
type MemoryStore struct {
    mu          sync.RWMutex
    checkpoints map[string]map[string][]byte  // runID -> nodeID -> data
    info        map[string][]CheckpointInfo   // runID -> infos
}

func NewMemoryStore() *MemoryStore {
    return &MemoryStore{
        checkpoints: make(map[string]map[string][]byte),
        info:        make(map[string][]CheckpointInfo),
    }
}

func (m *MemoryStore) Save(runID, nodeID string, data []byte) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.checkpoints[runID] == nil {
        m.checkpoints[runID] = make(map[string][]byte)
    }

    // Make a copy to avoid retaining caller's slice
    stored := make([]byte, len(data))
    copy(stored, data)
    m.checkpoints[runID][nodeID] = stored

    // Update info
    m.updateInfo(runID, nodeID, len(data))

    return nil
}

func (m *MemoryStore) Load(runID, nodeID string) ([]byte, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    run, ok := m.checkpoints[runID]
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

func (m *MemoryStore) Close() error {
    return nil  // Nothing to close
}
```

---

## Implementation: SQLiteStore

```go
type SQLiteStore struct {
    db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
    db, err := sql.Open("sqlite3", path)
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
        CREATE INDEX IF NOT EXISTS idx_run_id ON checkpoints(run_id);
    `)
    if err != nil {
        return nil, err
    }

    return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Save(runID, nodeID string, data []byte) error {
    _, err := s.db.Exec(`
        INSERT OR REPLACE INTO checkpoints (run_id, node_id, sequence, timestamp, data)
        VALUES (?, ?, COALESCE((SELECT MAX(sequence) FROM checkpoints WHERE run_id = ?), 0) + 1, ?, ?)
    `, runID, nodeID, runID, time.Now().UTC().Format(time.RFC3339), data)
    return err
}

func (s *SQLiteStore) Load(runID, nodeID string) ([]byte, error) {
    var data []byte
    err := s.db.QueryRow(`
        SELECT data FROM checkpoints WHERE run_id = ? AND node_id = ?
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
        err := rows.Scan(&info.NodeID, &info.Sequence, &timestamp, &info.Size)
        if err != nil {
            return nil, err
        }
        info.RunID = runID
        info.Timestamp, _ = time.Parse(time.RFC3339, timestamp)
        infos = append(infos, info)
    }
    return infos, nil
}

func (s *SQLiteStore) Close() error {
    return s.db.Close()
}
```

---

## Usage Examples

### Basic Checkpointing

```go
store := flowgraph.NewSQLiteStore("./checkpoints.db")
defer store.Close()

result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
)
```

### Resume from Checkpoint

```go
// Find latest checkpoint
checkpoints, _ := store.List(runID)
if len(checkpoints) > 0 {
    latest := checkpoints[len(checkpoints)-1]

    // Load state
    data, _ := store.Load(runID, latest.NodeID)
    var state MyState
    json.Unmarshal(data, &state)

    // Resume from next node
    result, err := compiled.Run(ctx, state,
        flowgraph.WithResume(latest.NodeID),
        flowgraph.WithCheckpointing(store),
    )
}
```

### Cleanup Old Checkpoints

```go
// Delete checkpoints older than 7 days
cutoff := time.Now().Add(-7 * 24 * time.Hour)

runs, _ := store.ListRuns()  // hypothetical
for _, runID := range runs {
    checkpoints, _ := store.List(runID)
    if len(checkpoints) > 0 && checkpoints[0].Timestamp.Before(cutoff) {
        store.DeleteRun(runID)
    }
}
```

---

## Test Cases

```go
func TestCheckpointStore(t *testing.T) {
    stores := []struct {
        name  string
        store CheckpointStore
    }{
        {"memory", NewMemoryStore()},
        {"sqlite", mustSQLiteStore(t)},
    }

    for _, tt := range stores {
        t.Run(tt.name, func(t *testing.T) {
            store := tt.store
            defer store.Close()

            // Save
            err := store.Save("run-1", "node-a", []byte("data-a"))
            require.NoError(t, err)

            // Load
            data, err := store.Load("run-1", "node-a")
            require.NoError(t, err)
            assert.Equal(t, []byte("data-a"), data)

            // Not found
            _, err = store.Load("run-1", "node-x")
            assert.ErrorIs(t, err, ErrCheckpointNotFound)

            // List
            store.Save("run-1", "node-b", []byte("data-b"))
            infos, err := store.List("run-1")
            require.NoError(t, err)
            assert.Len(t, infos, 2)

            // Delete
            err = store.Delete("run-1", "node-a")
            require.NoError(t, err)
            _, err = store.Load("run-1", "node-a")
            assert.ErrorIs(t, err, ErrCheckpointNotFound)

            // DeleteRun
            err = store.DeleteRun("run-1")
            require.NoError(t, err)
            infos, _ = store.List("run-1")
            assert.Empty(t, infos)
        })
    }
}
```
