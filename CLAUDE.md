# flowgraph

**Go library for graph-based LLM orchestration workflows.** LangGraph-equivalent with checkpointing, conditional branching, and multi-model support.

**Status**: Production-ready. Coverage: 93%+. See `.spec/tracking/PROGRESS.md` for history.

---

## Package Overview

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `pkg/flowgraph/` | Core orchestration | `Graph[S]`, `CompiledGraph[S]`, `Context`, `NodeFunc[S]` |
| `pkg/flowgraph/checkpoint/` | State persistence | `Store`, `MemoryStore`, `SQLiteStore` |
| `pkg/flowgraph/llm/` | LLM client interface | `Client`, `ClaudeCLI`, `MockClient` |
| `pkg/flowgraph/observability/` | Logging/metrics/tracing | `MetricsRecorder`, `SpanManager` |

---

## Quick Reference

### Basic Execution
```go
graph := flowgraph.NewGraph[Counter]().
    AddNode("inc", increment).
    AddEdge("inc", flowgraph.END).
    SetEntry("inc")

compiled, _ := graph.Compile()
result, _ := compiled.Run(ctx, Counter{})
```

### With Checkpointing
```go
store := checkpoint.NewSQLiteStore("./checkpoints.db")
result, _ := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))

// Resume after crash
result, _ := compiled.Resume(ctx, store, "run-123")
```

### With LLM
```go
client := llm.NewClaudeCLI()
ctx := flowgraph.NewContext(context.Background(), flowgraph.WithLLM(client))
```

### With Observability
```go
result, _ := compiled.Run(ctx, state,
    flowgraph.WithObservabilityLogger(slog.Default()),
    flowgraph.WithMetrics(true),
    flowgraph.WithTracing(true))
```

See `examples/` for complete working examples.

---

## Key Features

- **Type-safe graphs** with Go generics
- **Conditional branching** via router functions
- **Loops** with max iterations protection
- **Crash recovery** via checkpointing
- **LLM integration** with Claude CLI (token/cost tracking)
- **Observability** via slog + OpenTelemetry

---

## Project Structure

```
flowgraph/
├── pkg/flowgraph/         # Main package + subpackages
├── examples/              # 6 working examples
├── benchmarks/            # Performance benchmarks
├── docs/                  # OVERVIEW.md, ARCHITECTURE.md
└── .spec/                 # ADRs, phase specs, progress tracking
```

---

## Key Decisions

| Decision | Choice | ADR |
|----------|--------|-----|
| State management | Pass by value, return new | ADR-001 |
| Error handling | Sentinel + typed + wrapping | ADR-002 |
| Context | Custom wrapping context.Context | ADR-003 |
| Execution | Synchronous (parallel in v2) | ADR-010 |
| Panics | Recover → PanicError | ADR-011 |
| LLM interface | Complete + Stream methods | ADR-018 |

All 27 ADRs in `.spec/decisions/`.

---

## Testing

```bash
go test -race ./pkg/flowgraph/...
go test -coverprofile=coverage.out ./pkg/flowgraph/...
```

**Coverage**: flowgraph: 95.4%, checkpoint: 91.7%, llm: 93.2%, observability: 90.6%

---

## Quality Gates

- All tests pass (`go test -race ./...`)
- `gofmt -s -w .` clean
- `go vet ./...` clean
- Godoc for all public types

---

## References

| Doc | Purpose |
|-----|---------|
| `docs/OVERVIEW.md` | Conceptual documentation |
| `docs/ARCHITECTURE.md` | Technical details |
| `.spec/decisions/` | Architecture Decision Records |
| `.spec/tracking/PROGRESS.md` | Implementation history |
