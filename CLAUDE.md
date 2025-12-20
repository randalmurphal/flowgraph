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
| `pkg/flowgraph/model/` | Model selection & cost | `ModelName`, `Selector`, `EscalationChain`, `CostTracker` |
| `pkg/flowgraph/errors/` | Error handling strategies | `Category`, `RetryConfig`, `Handler` |

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
store, err := checkpoint.NewSQLiteStore("./checkpoints.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))

// Resume after crash
result, err = compiled.Resume(ctx, store, "run-123")
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

### Model Selection
```go
import "github.com/rmurphy/flowgraph/pkg/flowgraph/model"

selector := model.NewSelector(
    model.WithThinkingModel(model.ModelOpus),
    model.WithDefaultModel(model.ModelSonnet),
    model.WithFastModel(model.ModelHaiku),
)
m := selector.SelectForTier(model.TierThinking) // returns ModelOpus
```

### Error Handling with Retry & Escalation
```go
import "github.com/rmurphy/flowgraph/pkg/flowgraph/errors"

handler := errors.NewHandler(
    errors.WithRetryConfig(errors.DefaultRetry),
    errors.WithEscalation(&model.DefaultEscalation),
)
result := handler.Execute(ctx, model.ModelSonnet, func(ctx context.Context, m model.ModelName) error {
    // Your LLM operation here - will retry transient errors, escalate model on failures
    return nil
})
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

## Important Behavior

| Behavior | Default | Override |
|----------|---------|----------|
| Checkpoint failures | **Fatal** (stops execution) | `WithCheckpointFailureFatal(false)` |
| Max iterations | 1000 | `WithMaxIterations(n)` |
| JSON parse fallback | Logs warning, returns zero tokens | Expected when CLI returns text |

**Error Philosophy**: Errors are surfaced, not swallowed. If something fails, you'll know.

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

**Coverage**: flowgraph: 95.4%, checkpoint: 90.1%, llm: 93.8%, observability: 90.6%, model: 92.8%, errors: 84.8%

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
