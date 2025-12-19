# flowgraph Implementation Session

**Purpose**: Implement flowgraph Phase 3 (Checkpointing) and Phase 4 (LLM Clients)

**Philosophy**: Write production-quality Go code. Follow the specs exactly. Test as you go. No shortcuts.

---

## Context

flowgraph is a Go library for graph-based LLM workflow orchestration. Phase 1 (Core Graph) and Phase 2 (Conditional) are complete. Your job is to implement checkpointing and LLM clients.

### What's Complete

- **Phase 1**: Core graph engine - `pkg/flowgraph/*.go` (98.2% coverage)
- **Phase 2**: Conditional edges - included in Phase 1
- **27 ADRs** in `decisions/` - all architectural decisions locked
- **10 Feature Specs** in `features/` - detailed behavior specifications
- **6 Phase Specs** in `phases/` - implementation plans with code skeletons
- **API Surface** in `knowledge/API_SURFACE.md` - frozen public API

### What's Ready to Build

- **Phase 3**: Checkpointing (checkpoint store, state persistence, resume)
- **Phase 4**: LLM Clients (interface, Claude CLI, mock)

These phases can be implemented in parallel.

---

## Your Task: Implement Phase 3 and/or Phase 4

**Goal**: Add checkpointing for crash recovery AND/OR LLM client interface for AI calls.

**Estimated Effort**: 2-3 days per phase

### Phase 3: Checkpointing

**Files to Create**:
```
pkg/flowgraph/checkpoint/
├── store.go       # CheckpointStore interface
├── checkpoint.go  # Checkpoint type, metadata, serialization
├── memory.go      # MemoryStore implementation
├── sqlite.go      # SQLiteStore implementation
├── store_test.go
├── memory_test.go
└── sqlite_test.go
```

**Key ADRs**:
- ADR-014: Checkpoint format (JSON with metadata)
- ADR-015: Checkpoint store (simple CRUD interface)
- ADR-016: Resume strategy (resume from node after last checkpoint)
- ADR-017: State serialization (JSON with exported fields)

**Acceptance Criteria**:
```go
// Checkpointing enabled
store := checkpoint.NewMemoryStore()
result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))

// Resume after crash
result, err := compiled.Resume(ctx, store, "run-123")
```

### Phase 4: LLM Clients

**Files to Create**:
```
pkg/flowgraph/llm/
├── client.go      # LLMClient interface
├── request.go     # CompletionRequest, Response types
├── message.go     # Message, Role types
├── claude_cli.go  # Claude CLI implementation
├── mock.go        # MockLLM for testing
├── client_test.go
├── claude_cli_test.go
└── mock_test.go
```

**Key ADRs**:
- ADR-018: LLM interface (Complete + Stream methods)
- ADR-019: Context window (user responsibility)
- ADR-020: Streaming (optional via Stream())
- ADR-021: Token tracking (in response, aggregate in state)

**Acceptance Criteria**:
```go
// LLM client usage in nodes
client := llm.NewClaudeCLI()
ctx := flowgraph.NewContext(context.Background(), flowgraph.WithLLM(client))

func myNode(ctx flowgraph.Context, s State) (State, error) {
    resp, err := ctx.LLM().Complete(ctx, llm.CompletionRequest{
        Messages: []llm.Message{{Role: llm.RoleUser, Content: "Hello"}},
    })
    if err != nil {
        return s, err
    }
    s.Output = resp.Content
    return s, nil
}
```

---

## Implementation Order

### Option A: Phase 3 First (Checkpointing)

1. **checkpoint/store.go** (~1 hour) - Interface definition
2. **checkpoint/checkpoint.go** (~2 hours) - Checkpoint type, serialization
3. **checkpoint/memory.go** (~2 hours) - In-memory implementation
4. **checkpoint/sqlite.go** (~3 hours) - SQLite implementation
5. **execute.go modifications** (~2 hours) - WithCheckpointing, Resume
6. **Tests** (~3 hours) - Store tests, integration tests

### Option B: Phase 4 First (LLM Clients)

1. **llm/client.go** (~1 hour) - Interface definition
2. **llm/request.go** (~1 hour) - Request/Response types
3. **llm/message.go** (~30 min) - Message types
4. **llm/mock.go** (~1 hour) - Mock for testing
5. **llm/claude_cli.go** (~3 hours) - Claude CLI implementation
6. **Tests** (~2 hours) - Mock tests, CLI tests

### Option C: Parallel Implementation

Run both phases in parallel if you have the context budget.

---

## Detailed Instructions

### Phase 3: Read These First

- `.spec/phases/PHASE-3-checkpointing.md` - Complete code skeletons
- `.spec/features/checkpointing.md` - Checkpoint behavior
- `.spec/features/resume.md` - Resume behavior
- `.spec/decisions/014-checkpoint-format.md`
- `.spec/decisions/015-checkpoint-store.md`
- `.spec/decisions/016-resume-strategy.md`

### Phase 4: Read These First

- `.spec/phases/PHASE-4-llm.md` - Complete code skeletons
- `.spec/features/llm-client.md` - LLM client behavior
- `.spec/decisions/018-llm-interface.md`
- `.spec/decisions/020-streaming.md`

---

## Key Decisions (Don't Re-Decide)

| Topic | Decision | Reference |
|-------|----------|-----------|
| Checkpoint format | JSON with metadata | ADR-014 |
| Checkpoint timing | After each node | ADR-015 |
| Resume strategy | From node after last checkpoint | ADR-016 |
| State serialization | JSON, exported fields only | ADR-017 |
| LLM interface | Complete + Stream methods | ADR-018 |
| Context window | User/devflow responsibility | ADR-019 |
| Streaming | Optional, node decides | ADR-020 |
| Token tracking | In response, aggregate in state | ADR-021 |

---

## Quality Requirements

### Code Quality

- All public types have godoc comments
- All functions handle errors explicitly
- No `_` for ignored errors
- Use `fmt.Errorf("operation: %w", err)` for wrapping

### Testing

- Table-driven tests using testify
- 85% coverage for checkpoint, 80% for llm
- Race detection: `go test -race ./...`
- Test both happy path and error cases

### Style

- `gofmt -s -w .` before commit
- `go vet ./...` clean
- Follow patterns from existing core code

---

## Existing Code to Reference

The core package is complete and well-tested. Use it as reference:

- **Error patterns**: See `pkg/flowgraph/errors.go`
- **Interface patterns**: See `pkg/flowgraph/context.go` (Context interface)
- **Options patterns**: See `pkg/flowgraph/options.go` (functional options)
- **Test patterns**: See `pkg/flowgraph/*_test.go`

---

## First Steps

1. **Decide which phase to start** (3 or 4, or parallel)

2. **Create directory structure**:
   ```bash
   mkdir -p pkg/flowgraph/checkpoint  # Phase 3
   mkdir -p pkg/flowgraph/llm         # Phase 4
   ```

3. **Start with interfaces** - define the contract first

4. **Write tests as you implement** - don't defer testing

5. **Run frequently**:
   ```bash
   go test -race ./...
   go vet ./...
   ```

---

## Acceptance Criteria Summary

### Phase 3 Complete When:

```go
// This works
store := checkpoint.NewMemoryStore()
result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))

// And this works
result, err := compiled.Resume(ctx, store, "run-123")
```

### Phase 4 Complete When:

```go
// This works
client := llm.NewClaudeCLI()
resp, err := client.Complete(ctx, llm.CompletionRequest{
    Messages: []llm.Message{{Role: llm.RoleUser, Content: "Hello"}},
})

// And in nodes
ctx := flowgraph.NewContext(context.Background(), flowgraph.WithLLM(client))
// ctx.LLM().Complete(...) works
```

---

## Checklist

### Phase 3: Checkpointing

- [ ] checkpoint/store.go with CheckpointStore interface
- [ ] checkpoint/checkpoint.go with Checkpoint type
- [ ] checkpoint/memory.go with MemoryStore
- [ ] checkpoint/sqlite.go with SQLiteStore
- [ ] WithCheckpointing RunOption in options.go
- [ ] Resume() method on CompiledGraph
- [ ] All tests passing
- [ ] 85% coverage achieved
- [ ] No race conditions

### Phase 4: LLM Clients

- [ ] llm/client.go with LLMClient interface
- [ ] llm/request.go with CompletionRequest/Response
- [ ] llm/message.go with Message types
- [ ] llm/mock.go with MockLLM
- [ ] llm/claude_cli.go with ClaudeCLI
- [ ] Update context.go to use real LLMClient
- [ ] All tests passing
- [ ] 80% coverage achieved
- [ ] No race conditions

---

## After These Phases

When Phases 3 and 4 are complete:

1. Update `.spec/tracking/PROGRESS.md` to mark phases complete
2. Phase 5 (Observability) can start - adds logging, metrics, tracing
3. Phase 6 (Polish) comes after all other phases

See `.spec/PLANNING.md` for the full roadmap.

---

## Reference Documents

| Document | Use For |
|----------|---------|
| `.spec/phases/PHASE-3-checkpointing.md` | Checkpoint code skeletons |
| `.spec/phases/PHASE-4-llm.md` | LLM client code skeletons |
| `.spec/features/*.md` | Detailed behavior specifications |
| `.spec/decisions/*.md` | Why decisions were made |
| `.spec/knowledge/API_SURFACE.md` | Exact public API |
| `.spec/knowledge/TESTING_STRATEGY.md` | Test patterns |
| `pkg/flowgraph/*.go` | Reference implementation |
