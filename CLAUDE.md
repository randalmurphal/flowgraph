# flowgraph

**Go library for graph-based LLM orchestration workflows.** LangGraph-equivalent with checkpointing, conditional branching, and multi-model support.

---

## Current Status: Phases 1-4 Complete, Phase 5 Ready

**Core graph engine, checkpointing, and LLM clients are implemented and tested.** Ready to add observability.

| Phase | Status | Spec |
|-------|--------|------|
| Phase 1: Core Graph | ✅ Complete (87.8% coverage) | `.spec/phases/PHASE-1-core.md` |
| Phase 2: Conditional | ✅ Complete (included in P1) | `.spec/phases/PHASE-2-conditional.md` |
| Phase 3: Checkpointing | ✅ Complete (91.3% coverage) | `.spec/phases/PHASE-3-checkpointing.md` |
| Phase 4: LLM Clients | ✅ Complete (74.7% coverage) | `.spec/phases/PHASE-4-llm.md` |
| Phase 5: Observability | **Ready to Start** | `.spec/phases/PHASE-5-observability.md` |
| Phase 6: Polish | Blocked (needs P5) | `.spec/phases/PHASE-6-polish.md` |

**Start here**: `.spec/SESSION_PROMPT.md` for implementation handoff.

---

## What's Implemented

### Core Package (`pkg/flowgraph/`)

| File | Purpose | Status |
|------|---------|--------|
| `errors.go` | All error types (NodeError, PanicError, etc.) | ✅ |
| `node.go` | NodeFunc[S], RouterFunc[S], END constant | ✅ |
| `context.go` | Context interface and implementation | ✅ |
| `graph.go` | Graph[S] builder (AddNode, AddEdge, etc.) | ✅ |
| `compile.go` | Compile() with validation | ✅ |
| `compiled.go` | CompiledGraph[S] immutable type | ✅ |
| `execute.go` | Run() execution loop with checkpointing | ✅ |
| `options.go` | RunOptions (WithCheckpointing, WithRunID, etc.) | ✅ |
| `resume.go` | Resume() and ResumeFrom() methods | ✅ |

### Checkpoint Package (`pkg/flowgraph/checkpoint/`)

| File | Purpose | Status |
|------|---------|--------|
| `store.go` | CheckpointStore interface | ✅ |
| `checkpoint.go` | Checkpoint type, JSON serialization | ✅ |
| `memory.go` | MemoryStore (in-memory, for testing) | ✅ |
| `sqlite.go` | SQLiteStore (persistent, for production) | ✅ |

### LLM Package (`pkg/flowgraph/llm/`)

| File | Purpose | Status |
|------|---------|--------|
| `client.go` | Client interface (Complete, Stream) | ✅ |
| `request.go` | CompletionRequest, CompletionResponse types | ✅ |
| `errors.go` | Error type with Retryable flag | ✅ |
| `mock.go` | MockClient for testing | ✅ |
| `claude_cli.go` | ClaudeCLI implementation | ✅ |

### Working Features

- ✅ Fluent graph builder API
- ✅ Type-safe generics for state
- ✅ Linear and conditional execution
- ✅ Loops with conditional exit
- ✅ Panic recovery with stack traces
- ✅ Context cancellation/timeout
- ✅ Max iterations protection
- ✅ Error wrapping with node context
- ✅ Checkpoint persistence (SQLite, Memory)
- ✅ Resume from checkpoint after crash
- ✅ LLM client interface with streaming
- ✅ Claude CLI integration
- ✅ MockClient for testing

### Usage Examples

```go
// Basic graph execution
graph := flowgraph.NewGraph[Counter]().
    AddNode("inc1", increment).
    AddNode("inc2", increment).
    AddEdge("inc1", "inc2").
    AddEdge("inc2", flowgraph.END).
    SetEntry("inc1")

compiled, _ := graph.Compile()
ctx := flowgraph.NewContext(context.Background())
result, _ := compiled.Run(ctx, Counter{Value: 0})
// result.Value == 2
```

```go
// With checkpointing
store := checkpoint.NewMemoryStore()
result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))

// Resume after crash
result, err := compiled.Resume(ctx, store, "run-123")
```

```go
// With LLM client
client := llm.NewClaudeCLI()
ctx := flowgraph.NewContext(context.Background(), flowgraph.WithLLM(client))

func myNode(ctx flowgraph.Context, s State) (State, error) {
    resp, err := ctx.LLM().Complete(ctx, llm.CompletionRequest{
        Messages: []llm.Message{{Role: llm.RoleUser, Content: "Hello"}},
    })
    // ...
}
```

---

## What's Next

### Phase 5: Observability

Add production-grade observability: structured logging, metrics, and tracing.

**Files to create**:
```
pkg/flowgraph/observability/
├── logger.go     # slog integration helpers
├── metrics.go    # OpenTelemetry metrics
├── tracing.go    # OpenTelemetry tracing
├── noop.go       # No-op implementations
└── *_test.go
```

**Key features**:
- Structured logging via slog
- OpenTelemetry metrics (node executions, latency, errors)
- OpenTelemetry tracing (spans for runs and nodes)
- No-op implementations for disabled state

---

## Project Structure

```
flowgraph/
├── CLAUDE.md              # This file
├── go.mod                 # Module definition
├── pkg/flowgraph/         # Main package
│   ├── *.go               # Core implementation
│   ├── *_test.go          # Core tests
│   ├── checkpoint/        # ✅ Checkpoint package
│   ├── llm/               # ✅ LLM client package
│   └── observability/     # TODO: Phase 5
├── docs/                  # User documentation
│   ├── OVERVIEW.md
│   ├── ARCHITECTURE.md
│   └── ...
└── .spec/                 # Implementation specs
    ├── SESSION_PROMPT.md  # Start here for next phase
    ├── phases/            # Phase specifications
    ├── features/          # Feature specifications
    ├── decisions/         # 27 ADRs
    └── tracking/
        └── PROGRESS.md    # Detailed progress
```

---

## Key Decisions (Already Made)

| Decision | Choice | ADR |
|----------|--------|-----|
| State management | Pass by value, return new | ADR-001 |
| Error handling | Sentinel + typed + wrapping | ADR-002 |
| Context | Custom wrapping context.Context | ADR-003 |
| Validation | Panic at build, error at compile | ADR-007 |
| Execution | Synchronous (parallel in v2) | ADR-010 |
| Panics | Recover, convert to PanicError | ADR-011 |
| Checkpoint format | JSON with metadata | ADR-014 |
| Checkpoint store | Simple CRUD interface | ADR-015 |
| Resume strategy | From node after last checkpoint | ADR-016 |
| LLM interface | Complete + Stream methods | ADR-018 |
| Streaming | Optional, node decides | ADR-020 |

All 27 ADRs are in `.spec/decisions/`.

---

## Testing

```bash
# Run all tests
go test -race ./pkg/flowgraph/...

# With coverage
go test -coverprofile=coverage.out ./pkg/flowgraph/...
go tool cover -func=coverage.out
```

**Current Coverage**:
- flowgraph: 87.8%
- checkpoint: 91.3%
- llm: 74.7%

---

## Quality Gates

Before any phase is complete:

- [ ] All tests pass (`go test -race ./...`)
- [ ] Coverage meets target
- [ ] No race conditions detected
- [ ] `gofmt -s -w .` clean
- [ ] `go vet ./...` clean
- [ ] Godoc for all public types

---

## References

| Doc | Purpose |
|-----|---------|
| `.spec/SESSION_PROMPT.md` | Next implementation handoff |
| `.spec/tracking/PROGRESS.md` | Detailed progress tracking |
| `.spec/phases/PHASE-5-observability.md` | Next phase spec |
| `.spec/knowledge/API_SURFACE.md` | Complete public API |
| `.spec/knowledge/TESTING_STRATEGY.md` | Test patterns |
