# Checkpoint Package

**Crash recovery and state persistence for flowgraph workflows.**

---

## Overview

This package provides checkpoint storage for saving and resuming graph execution state. Use checkpointing for:

- Crash recovery (resume after failure)
- Long-running workflows
- Debugging (inspect state at each node)

---

## Key Types

| Type | Purpose |
|------|---------|
| `Store` | Interface for checkpoint persistence |
| `Checkpoint` | Saved state with metadata |
| `MemoryStore` | In-memory store (testing) |
| `SQLiteStore` | SQLite-backed store (production) |

---

## Usage

### Enable Checkpointing

```go
store := checkpoint.NewSQLiteStore("./checkpoints.db")
defer store.Close()

result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))
```

### Resume After Crash

```go
// Resume from last checkpoint
result, err := compiled.Resume(ctx, store, "run-123")

// Resume from specific node
result, err := compiled.ResumeFrom(ctx, store, "run-123", "node-id")
```

### List Checkpoints

```go
checkpoints, err := store.List("run-123")
for _, cp := range checkpoints {
    fmt.Printf("Node: %s, Time: %v\n", cp.NodeID, cp.CreatedAt)
}
```

---

## Store Interface

```go
type Store interface {
    Save(ctx context.Context, checkpoint *Checkpoint) error
    Load(ctx context.Context, runID string) (*Checkpoint, error)
    List(runID string) ([]*Checkpoint, error)
    Delete(ctx context.Context, runID string) error
    Close() error
}
```

---

## Checkpoint Format

Checkpoints are stored as JSON:

```json
{
  "run_id": "run-123",
  "node_id": "process",
  "state": {...},
  "created_at": "2025-01-15T10:30:00Z",
  "metadata": {
    "graph_id": "my-workflow",
    "version": 1
  }
}
```

---

## Files

| File | Purpose |
|------|---------|
| `store.go` | Store interface definition |
| `checkpoint.go` | Checkpoint type and serialization |
| `memory.go` | MemoryStore implementation |
| `sqlite.go` | SQLiteStore implementation |

---

## Testing

Use `MemoryStore` for tests:

```go
store := checkpoint.NewMemoryStore()
// No cleanup needed - data is in-memory
```

Use `SQLiteStore` for production:

```go
store, err := checkpoint.NewSQLiteStore("./data/checkpoints.db")
if err != nil {
    return err
}
defer store.Close()
```

---

## Design Decisions

- **JSON format** - Human-readable, debuggable (ADR-014)
- **Simple CRUD interface** - Easy to implement new stores (ADR-015)
- **Resume from node after checkpoint** - Not before (ADR-016)
- **State must be JSON-serializable** - Required for checkpointing (ADR-017)
