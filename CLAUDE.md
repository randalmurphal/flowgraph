# flowgraph

**Go library for graph-based LLM orchestration workflows.** LangGraph-equivalent with checkpointing, conditional branching, and multi-model support.

---

## Current Status: Phase 1 Complete, Phase 3-4 Ready

**Core graph engine is implemented and tested.** Ready to add checkpointing and LLM clients.

| Phase | Status | Spec |
|-------|--------|------|
| Phase 1: Core Graph | ✅ Complete (98.2% coverage) | `.spec/phases/PHASE-1-core.md` |
| Phase 2: Conditional | ✅ Complete (included in P1) | `.spec/phases/PHASE-2-conditional.md` |
| Phase 3: Checkpointing | **Ready to Start** | `.spec/phases/PHASE-3-checkpointing.md` |
| Phase 4: LLM Clients | **Ready to Start** | `.spec/phases/PHASE-4-llm.md` |
| Phase 5: Observability | Blocked (needs P3-4) | `.spec/phases/PHASE-5-observability.md` |
| Phase 6: Polish | Blocked (needs all) | `.spec/phases/PHASE-6-polish.md` |

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
| `execute.go` | Run() execution loop | ✅ |
| `options.go` | RunOption (WithMaxIterations) | ✅ |

### Working Features

- ✅ Fluent graph builder API
- ✅ Type-safe generics for state
- ✅ Linear and conditional execution
- ✅ Loops with conditional exit
- ✅ Panic recovery with stack traces
- ✅ Context cancellation/timeout
- ✅ Max iterations protection
- ✅ Error wrapping with node context

### Usage Example

```go
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

---

## What's Next

### Phase 3: Checkpointing

Add persistent state checkpoints for crash recovery.

**Files to create**:
```
pkg/flowgraph/checkpoint/
├── store.go       # CheckpointStore interface
├── checkpoint.go  # Checkpoint type, serialization
├── memory.go      # MemoryStore implementation
├── sqlite.go      # SQLiteStore implementation
└── *_test.go
```

**Key ADRs**: ADR-014 (format), ADR-015 (store), ADR-016 (resume)

### Phase 4: LLM Clients

Add LLM client interface and implementations.

**Files to create**:
```
pkg/flowgraph/llm/
├── client.go      # LLMClient interface
├── request.go     # CompletionRequest, Response
├── claude_cli.go  # Claude CLI implementation
├── mock.go        # MockLLM for testing
└── *_test.go
```

**Key ADRs**: ADR-018 (interface), ADR-020 (streaming)

**Note**: Phases 3 and 4 can run in parallel.

---

## Project Structure

```
flowgraph/
├── CLAUDE.md              # This file
├── go.mod                 # Module definition
├── pkg/flowgraph/         # Main package
│   ├── *.go               # Core implementation (Phase 1)
│   ├── *_test.go          # Tests (97 tests, 98.2% coverage)
│   ├── checkpoint/        # TODO: Phase 3
│   └── llm/               # TODO: Phase 4
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
| LLM interface | Complete + Stream methods | ADR-018 |

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

**Current Coverage**: 98.2% (target: 90%)

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
| `.spec/phases/PHASE-3-checkpointing.md` | Next phase spec |
| `.spec/phases/PHASE-4-llm.md` | Parallel phase spec |
| `.spec/knowledge/API_SURFACE.md` | Complete public API |
| `.spec/knowledge/TESTING_STRATEGY.md` | Test patterns |
