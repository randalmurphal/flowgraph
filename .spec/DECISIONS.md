# flowgraph Architectural Decisions

Quick reference for all ADRs. See individual files in `decisions/` for full context.

---

## Decision Summary

| # | Decision | Status | Key Choice |
|---|----------|--------|------------|
| 001 | State Management | ✅ Accepted | Pass by value, return new state |
| 002 | Error Handling | ✅ Accepted | Hybrid: sentinel + typed + wrapping |
| 003 | Context Interface | ✅ Accepted | Custom Context wrapping context.Context |
| 004 | Graph Immutability | ✅ Accepted | Mutable builder, immutable after Compile() |
| 005 | Node Signature | ✅ Accepted | `func(Context, S) (S, error)` |
| 006 | Edge Representation | ✅ Accepted | Three types: simple, conditional, END |
| 007 | Validation Timing | ✅ Accepted | Progressive: build panics, compile errors |
| 008 | Cycle Handling | ✅ Accepted | Cycles allowed with conditional exits |
| 009 | Compilation Output | ✅ Accepted | CompiledGraph with pre-computed metadata |
| 010 | Execution Model | ✅ Accepted | Synchronous, parallel fan-out in v2 |
| 011 | Panic Recovery | ✅ Accepted | Recover, convert to PanicError with stack |
| 012 | Cancellation | ✅ Accepted | Check between nodes, nodes check during |
| 013 | Timeouts | ✅ Accepted | Graph-level via context, optional per-node |
| 014 | Checkpoint Format | ✅ Accepted | JSON with metadata, optional compression |
| 015 | Checkpoint Store | ✅ Accepted | Simple CRUD interface |
| 016 | Resume Strategy | ✅ Accepted | Resume from node after last checkpoint |
| 017 | State Serialization | ✅ Accepted | JSON with exported fields |
| 018 | LLM Interface | ✅ Accepted | Complete + Stream methods |
| 019 | Context Window | ✅ Accepted | User/devflow responsibility |
| 020 | Streaming | ✅ Accepted | Optional via Stream(), node decides |
| 021 | Token Tracking | ✅ Accepted | In response, aggregate in state/hooks |
| 022 | Logging | ✅ Accepted | slog with context injection |
| 023 | Metrics | ✅ Accepted | OpenTelemetry with hooks fallback |
| 024 | Tracing | ✅ Accepted | OpenTelemetry automatic spans |
| 025 | Testing Philosophy | ✅ Accepted | Table-driven, unit/integration/e2e split |
| 026 | Mocks | ✅ Accepted | Hand-written with helpers |
| 027 | Integration Tests | ✅ Accepted | Build tags, real internals |

---

## Core Type Decisions

### State Type
```go
type NodeFunc[S any] func(ctx Context, state S) (S, error)
```
- Generic over user-defined state `S`
- Immutable: receive value, return new value
- Must be JSON-serializable for checkpointing

### Context Interface
```go
type Context interface {
    context.Context
    Logger() *slog.Logger
    LLM() LLMClient
    Checkpointer() CheckpointStore
    RunID() string
    NodeID() string
}
```

### Error Types
```go
var ErrCompilation = errors.New("compilation error")  // Sentinel

type NodeError struct {  // Typed with context
    NodeID string
    Err    error
}

return fmt.Errorf("operation X: %w", err)  // Wrapping
```

---

## Key Patterns

### Graph Construction
```go
graph := flowgraph.NewGraph[MyState]().
    AddNode("a", nodeA).
    AddNode("b", nodeB).
    AddEdge("a", "b").
    AddConditionalEdge("b", router).
    SetEntry("a")

compiled, err := graph.Compile()
```

### Execution
```go
result, err := compiled.Run(ctx, initialState,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithLogger(logger),
)
```

### Testing
```go
mock := &MockLLM{Response: "test"}
ctx := NewMockContext().WithLLM(mock)
result, err := myNode(ctx, inputState)
```

---

## What's NOT Decided (Open Questions)

These require discussion before implementation:

1. **Parallel node execution** - Fan-out/fan-in in v1 or defer to v2?
2. **Sub-graphs** - Can graphs compose other graphs?
3. **Dynamic graphs** - Can edges be added at runtime?
4. **State versioning** - How handle schema changes on resume?
5. **Distributed execution** - In scope for v1?

---

## Decision Principles

These principles guided all decisions:

1. **Explicit over implicit** - No magic, clear data flow
2. **Simple over clever** - Boring code that works
3. **Testable** - Every component mockable
4. **Go-idiomatic** - Follow standard patterns
5. **Production-ready** - Error handling, observability built-in
