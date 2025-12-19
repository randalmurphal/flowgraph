# flowgraph Planning Document

**Status**: Phase 0 Complete - Ready for Implementation
**Last Updated**: 2025-12-19

---

## Vision

Build an industry-grade, production-ready graph orchestration library for Go that:

1. **Is genuinely useful** - Solves real problems for LLM workflow orchestration
2. **Is robust** - Handles failures gracefully, recovers from crashes
3. **Is testable** - Every component can be tested in isolation
4. **Is extensible** - Easy to add new node types, stores, clients
5. **Is observable** - Logging, metrics, tracing built-in
6. **Is documented** - Code, API, patterns all clearly documented

---

## Specification Structure

```
.spec/
├── PLANNING.md              # This file - overall planning
├── DECISIONS.md             # Decision log (quick reference)
├── decisions/               # Architecture Decision Records (27 complete)
│   ├── 001-state-management.md
│   ├── 002-error-handling.md
│   └── ... (001-027 complete)
├── phases/                  # Implementation phases
│   ├── PHASE-1-core.md
│   └── ...
├── features/                # Feature specifications
├── tracking/                # Progress tracking
│   ├── PROGRESS.md
│   ├── BLOCKERS.md
│   └── CHANGELOG.md
└── knowledge/               # Learnings and patterns
```

---

## Architectural Decisions (Complete)

All 27 ADRs have been written and accepted. See `DECISIONS.md` for quick reference.

### Core Architecture ✅

| # | Decision | Status | Key Choice |
|---|----------|--------|------------|
| 001 | State management | ✅ Accepted | Pass by value, return new state |
| 002 | Error handling | ✅ Accepted | Hybrid: sentinel + typed + wrapping |
| 003 | Context interface | ✅ Accepted | Custom Context wrapping context.Context |
| 004 | Graph immutability | ✅ Accepted | Mutable builder, immutable after Compile() |
| 005 | Node signature | ✅ Accepted | `func(Context, S) (S, error)` |
| 006 | Edge representation | ✅ Accepted | Three types: simple, conditional, END |

### Validation & Compilation ✅

| # | Decision | Status | Key Choice |
|---|----------|--------|------------|
| 007 | Validation timing | ✅ Accepted | Progressive: build panics, compile errors |
| 008 | Cycle handling | ✅ Accepted | Cycles allowed with conditional exits |
| 009 | Compilation output | ✅ Accepted | CompiledGraph with pre-computed metadata |

### Execution ✅

| # | Decision | Status | Key Choice |
|---|----------|--------|------------|
| 010 | Execution model | ✅ Accepted | Synchronous, parallel in v2 |
| 011 | Panic recovery | ✅ Accepted | Recover, convert to PanicError with stack |
| 012 | Cancellation | ✅ Accepted | Check between nodes, nodes check during |
| 013 | Timeouts | ✅ Accepted | Graph-level via context, optional per-node |

### Checkpointing ✅

| # | Decision | Status | Key Choice |
|---|----------|--------|------------|
| 014 | Checkpoint format | ✅ Accepted | JSON with metadata, optional compression |
| 015 | Checkpoint store | ✅ Accepted | Simple CRUD interface |
| 016 | Resume strategy | ✅ Accepted | Resume from node after last checkpoint |
| 017 | State serialization | ✅ Accepted | JSON with exported fields |

### LLM Integration ✅

| # | Decision | Status | Key Choice |
|---|----------|--------|------------|
| 018 | LLM interface | ✅ Accepted | Complete + Stream methods |
| 019 | Context window | ✅ Accepted | User/devflow responsibility |
| 020 | Streaming | ✅ Accepted | Optional via Stream(), node decides |
| 021 | Token tracking | ✅ Accepted | In response, aggregate in state/hooks |

### Observability ✅

| # | Decision | Status | Key Choice |
|---|----------|--------|------------|
| 022 | Logging | ✅ Accepted | slog with context injection |
| 023 | Metrics | ✅ Accepted | OpenTelemetry with hooks fallback |
| 024 | Tracing | ✅ Accepted | OpenTelemetry automatic spans |

### Testing ✅

| # | Decision | Status | Key Choice |
|---|----------|--------|------------|
| 025 | Testing philosophy | ✅ Accepted | Table-driven, unit/integration/e2e split |
| 026 | Mocks | ✅ Accepted | Hand-written with helpers |
| 027 | Integration tests | ✅ Accepted | Build tags, real internals |

---

## Implementation Phases

### Phase 0: Decisions ✅ COMPLETE

**Goal**: Lock in all architectural decisions before writing code.

**Deliverables**:
- [x] All 27 ADRs written and accepted
- [ ] Feature specifications complete
- [ ] Test strategy defined
- [ ] Dependency list finalized

### Phase 1: Core Graph

**Goal**: Basic graph definition and linear execution.

**Dependencies**: Phase 0 complete ✅

**Estimated Effort**: 2-3 days

**Files to Create**:
```
pkg/flowgraph/
├── graph.go           # Graph[S] type, builder methods
├── node.go            # NodeFunc[S], node configuration
├── edge.go            # Edge types, END constant
├── compile.go         # Compile(), validation logic
├── compiled.go        # CompiledGraph[S], introspection
├── execute.go         # Run(), execution loop
├── context.go         # Context interface, implementation
├── errors.go          # Error types (NodeError, etc.)
├── options.go         # RunOption, functional options
└── *_test.go          # Tests for each file
```

**Deliverables**:
- [ ] Graph[S] type with AddNode, AddEdge, SetEntry
- [ ] Compile() with validation (per ADR-007)
- [ ] Run() for linear flows (per ADR-010)
- [ ] Context interface (per ADR-003)
- [ ] Error types (per ADR-002)
- [ ] 90% test coverage

**Acceptance Criteria**:
```go
// This must work after Phase 1
graph := flowgraph.NewGraph[MyState]().
    AddNode("a", nodeA).
    AddNode("b", nodeB).
    AddEdge("a", "b").
    AddEdge("b", flowgraph.END).
    SetEntry("a")

compiled, err := graph.Compile()
result, err := compiled.Run(ctx, initialState)
```

### Phase 2: Conditional Execution

**Goal**: Conditional edges and loops.

**Dependencies**: Phase 1 complete

**Estimated Effort**: 1-2 days

**Deliverables**:
- [ ] AddConditionalEdge with RouterFunc[S]
- [ ] Cycle detection with conditional exit validation (per ADR-008)
- [ ] Loop execution with max iteration limit
- [ ] 90% test coverage

**Acceptance Criteria**:
```go
// Conditional branching
graph.AddConditionalEdge("review", func(ctx Context, s State) string {
    if s.Approved { return "approve" }
    return "reject"
})

// Loop with exit
graph.AddConditionalEdge("check", func(ctx Context, s State) string {
    if s.Done { return flowgraph.END }
    return "process"
})
```

### Phase 3: Checkpointing

**Goal**: Persistent checkpoints and resume.

**Dependencies**: Phase 2 complete

**Estimated Effort**: 2-3 days

**Files to Create**:
```
pkg/flowgraph/
├── checkpoint/
│   ├── store.go       # CheckpointStore interface
│   ├── memory.go      # MemoryStore implementation
│   ├── sqlite.go      # SQLiteStore implementation
│   ├── checkpoint.go  # Checkpoint type, serialization
│   └── *_test.go
```

**Deliverables**:
- [ ] CheckpointStore interface (per ADR-015)
- [ ] Checkpoint format (per ADR-014)
- [ ] MemoryStore implementation
- [ ] SQLiteStore implementation
- [ ] Resume() method (per ADR-016)
- [ ] 85% test coverage

**Acceptance Criteria**:
```go
// Checkpointing enabled
result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"),
)

// Resume after crash
result, err := compiled.Resume(ctx, store, "run-123")
```

### Phase 4: LLM Clients

**Goal**: LLM client interface and implementations.

**Dependencies**: Phase 1 complete (can parallel with 2-3)

**Estimated Effort**: 2-3 days

**Files to Create**:
```
pkg/flowgraph/
├── llm/
│   ├── client.go      # LLMClient interface
│   ├── request.go     # CompletionRequest, Response
│   ├── claude_cli.go  # Claude CLI implementation
│   ├── mock.go        # MockLLM for testing
│   └── *_test.go
```

**Deliverables**:
- [ ] LLMClient interface (per ADR-018)
- [ ] CompletionRequest/Response types
- [ ] ClaudeCLI implementation
- [ ] Streaming support (per ADR-020)
- [ ] MockLLM for testing
- [ ] 80% test coverage

**Acceptance Criteria**:
```go
client := llm.NewClaudeCLI()
resp, err := client.Complete(ctx, llm.CompletionRequest{
    Prompt: "Hello",
    SystemPrompt: "You are helpful",
})

// In nodes
func myNode(ctx flowgraph.Context, s State) (State, error) {
    resp, _ := ctx.LLM().Complete(ctx, req)
    s.Output = resp.Text
    return s, nil
}
```

### Phase 5: Observability

**Goal**: Production observability.

**Dependencies**: Phase 2 complete

**Estimated Effort**: 2-3 days

**Deliverables**:
- [ ] slog integration (per ADR-022)
- [ ] Automatic log enrichment (run_id, node_id)
- [ ] OpenTelemetry metrics (per ADR-023)
- [ ] OpenTelemetry tracing (per ADR-024)
- [ ] WithLogger, WithMetrics, WithTracing options

**Acceptance Criteria**:
```go
result, err := compiled.Run(ctx, state,
    flowgraph.WithLogger(slog.Default()),
    flowgraph.WithMetrics(otelMetrics),
)

// Automatic spans created:
// flowgraph.run -> flowgraph.node.a -> flowgraph.node.b
```

### Phase 6: Polish & Documentation

**Goal**: Production readiness.

**Dependencies**: All other phases complete

**Estimated Effort**: 2-3 days

**Deliverables**:
- [ ] Comprehensive godoc
- [ ] Example applications (2-3)
- [ ] Performance benchmarks
- [ ] README with quick start
- [ ] CONTRIBUTING guide
- [ ] Security considerations documented

---

## Quality Gates

Before any phase is considered complete:

### Code Quality
- [ ] All tests pass (`go test -race ./...`)
- [ ] Coverage meets target (90% core, 80% integrations)
- [ ] No linter warnings (`golangci-lint run`)
- [ ] No race conditions detected
- [ ] Benchmarks exist for critical paths

### Documentation
- [ ] All public APIs documented with godoc
- [ ] Code comments explain "why"
- [ ] Examples for common use cases

### Review
- [ ] Self-review against ADRs
- [ ] Code structure matches design decisions
- [ ] Error handling is comprehensive (per ADR-002)
- [ ] Edge cases are tested

---

## Dependencies

### Required (Core)

| Dependency | Purpose | Version |
|------------|---------|---------|
| Go | Language | 1.22+ |
| testify | Testing assertions | Latest |

### Optional (Phase 3+)

| Dependency | Purpose | When |
|------------|---------|------|
| modernc.org/sqlite | Pure-Go SQLite | Phase 3 |
| go.opentelemetry.io/otel | Observability | Phase 5 |

---

## Open Questions

Deferred to v2 or later discussion:

1. **Parallel node execution** - Fan-out/fan-in → v2
2. **Sub-graphs** - Graph composition → v2
3. **Dynamic graphs** - Runtime edge changes → v2
4. **State versioning** - Schema migration → v2
5. **Distributed execution** - Multi-node → out of scope

---

## Success Criteria

flowgraph v1.0 is ready when:

1. **Functional**: Can run the ticket-to-pr workflow end-to-end
2. **Reliable**: Crashes recover correctly, no data loss
3. **Tested**: 85%+ coverage, all edge cases covered
4. **Documented**: Someone can use it from docs alone
5. **Performant**: Sub-millisecond overhead per node
6. **Observable**: Can debug production issues from logs/traces
