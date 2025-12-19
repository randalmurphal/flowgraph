# Checkpointing Example

This example demonstrates saving checkpoints for crash recovery using SQLite storage.

## What It Shows

- Creating a checkpoint store with `checkpoint.NewSQLiteStore(path)`
- Enabling checkpointing with `WithCheckpointing(store)`
- Assigning run IDs with `WithRunID(id)`
- Listing saved checkpoints
- Resuming from checkpoints with `Resume()`

## Graph Structure

```
step1 -> step2 -> step3 -> END
  ↓        ↓        ↓
[checkpoint saved after each node]
```

## Running

```bash
go run main.go
```

## Expected Output

```
=== First Run: Normal execution with checkpointing ===
Step 1: Processing input...
Step 2: Transforming data...
Step 3: Finalizing...
Result: Processed: hello world
All steps complete: step1=true, step2=true, step3=true

=== Checkpoints saved ===
  - After node: step1 (at 2024-01-15T10:30:00Z)
  - After node: step2 (at 2024-01-15T10:30:00Z)
  - After node: step3 (at 2024-01-15T10:30:00Z)

=== Simulating Resume Scenario ===
...
```

## Key Concepts

1. **Checkpoint stores**: Implement `checkpoint.CheckpointStore` interface
2. **Built-in stores**: `MemoryStore` for testing, `SQLiteStore` for production
3. **Run IDs**: Unique identifiers for tracking runs
4. **Automatic checkpointing**: Saves state after each successful node execution
5. **Resume**: Continues from the node after the last checkpoint

## Storage Options

| Store | Use Case |
|-------|----------|
| `MemoryStore` | Testing, ephemeral workflows |
| `SQLiteStore` | Production, persistent storage |
| Custom | Implement `CheckpointStore` interface |

## Resume Pattern

```go
// Initial run
result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))

// After crash, resume
result, err := compiled.Resume(ctx, store, "run-123")
```
