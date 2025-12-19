# flowgraph Implementation Session

**Purpose**: Implement flowgraph Phase 1 - Core Graph Engine

**Philosophy**: Write production-quality Go code. Follow the specs exactly. Test as you go. No shortcuts.

---

## Context

flowgraph is a Go library for graph-based LLM workflow orchestration. All architectural decisions are made and documented. Your job is to implement, not design.

### What's Complete

- **27 ADRs** in `decisions/` - all architectural decisions locked
- **10 Feature Specs** in `features/` - detailed behavior specifications
- **6 Phase Specs** in `phases/` - implementation plans with code skeletons
- **API Surface** in `knowledge/API_SURFACE.md` - frozen public API
- **Testing Strategy** in `knowledge/TESTING_STRATEGY.md` - patterns and targets

### What's Not Started

- No Go code exists yet
- No `go.mod` exists yet
- No tests exist yet

---

## Your Task: Implement Phase 1

**Goal**: Build the core graph engine - definition, compilation, and linear execution.

**Estimated Effort**: 2-3 days of focused work

### Files to Create

```
pkg/flowgraph/
├── errors.go          # Error types and sentinels
├── node.go            # NodeFunc[S], END constant
├── context.go         # Context interface and implementation
├── graph.go           # Graph[S] builder
├── compile.go         # Compile() and validation
├── compiled.go        # CompiledGraph[S] type
├── execute.go         # Run() execution loop
├── options.go         # RunOption, ContextOption
├── graph_test.go      # Graph builder tests
├── compile_test.go    # Compilation tests
├── execute_test.go    # Execution tests
└── testutil_test.go   # Shared test helpers
```

### Implementation Order

Follow this order exactly (dependencies flow down):

1. **errors.go** (~30 min) - All error types first
2. **node.go** (~15 min) - NodeFunc and END
3. **context.go** (~1 hour) - Context interface and basic impl
4. **graph.go** (~2 hours) - Builder with validation panics
5. **compile.go** (~3 hours) - Validation logic, path checking
6. **compiled.go** (~1 hour) - Immutable compiled type
7. **execute.go** (~3 hours) - Run loop, cancellation, panic recovery
8. **options.go** (~30 min) - Functional options
9. **Tests** (~4 hours) - Comprehensive test coverage

### Detailed Instructions

Read `.spec/phases/PHASE-1-core.md` for:
- Complete code skeletons
- Exact function signatures
- Validation rules
- Test case examples

Read `.spec/features/` for behavior details:
- `graph-builder.md` - Builder API, panic conditions
- `compilation.md` - Validation order, error aggregation
- `linear-execution.md` - Run loop, panic recovery, cancellation
- `error-handling.md` - Error types, wrapping patterns
- `context-interface.md` - Context services and metadata

---

## Key Decisions (Don't Re-Decide)

| Topic | Decision | Reference |
|-------|----------|-----------|
| State | Pass by value, return new | ADR-001 |
| Errors | Sentinel + typed + wrapping | ADR-002 |
| Context | Custom wrapping context.Context | ADR-003 |
| Graph | Mutable builder, immutable compiled | ADR-004 |
| Node signature | `func(Context, S) (S, error)` | ADR-005 |
| Validation | Panic at build, error at compile | ADR-007 |
| Panics | Recover, convert to PanicError | ADR-011 |
| Cancellation | Check between nodes | ADR-012 |

---

## Quality Requirements

### Code Quality

- All public types have godoc comments
- All functions handle errors explicitly
- No `_` for ignored errors
- Use `fmt.Errorf("operation: %w", err)` for wrapping

### Testing

- Table-driven tests using testify
- 90% coverage for core package
- Race detection: `go test -race ./...`
- Test both happy path and error cases

### Style

- `gofmt -s -w .` before commit
- `go vet ./...` clean
- Follow patterns from existing ADRs

---

## First Steps

1. **Create go.mod**:
   ```bash
   cd /home/rmurphy/repos/flowgraph
   mkdir -p pkg/flowgraph
   cd pkg/flowgraph
   go mod init github.com/yourusername/flowgraph
   go get github.com/stretchr/testify
   ```

2. **Create errors.go** following the skeleton in PHASE-1-core.md

3. **Write tests as you implement** - don't defer testing

4. **Run frequently**:
   ```bash
   go test -race ./...
   go vet ./...
   ```

---

## Acceptance Criteria

Phase 1 is complete when this code works:

```go
package main

import (
    "context"
    "fmt"
    "github.com/yourusername/flowgraph"
)

type Counter struct {
    Value int
}

func increment(ctx flowgraph.Context, s Counter) (Counter, error) {
    s.Value++
    return s, nil
}

func main() {
    graph := flowgraph.NewGraph[Counter]().
        AddNode("inc1", increment).
        AddNode("inc2", increment).
        AddNode("inc3", increment).
        AddEdge("inc1", "inc2").
        AddEdge("inc2", "inc3").
        AddEdge("inc3", flowgraph.END).
        SetEntry("inc1")

    compiled, err := graph.Compile()
    if err != nil {
        panic(err)
    }

    ctx := flowgraph.NewContext(context.Background())
    result, err := compiled.Run(ctx, Counter{Value: 0})
    if err != nil {
        panic(err)
    }

    fmt.Printf("Final count: %d\n", result.Value)  // Output: 3
}
```

### Checklist

- [ ] go.mod created with dependencies
- [ ] errors.go with all error types
- [ ] node.go with NodeFunc and END
- [ ] context.go with Context interface
- [ ] graph.go with builder methods
- [ ] compile.go with validation
- [ ] compiled.go with CompiledGraph
- [ ] execute.go with Run()
- [ ] options.go with functional options
- [ ] All tests passing
- [ ] 90% coverage achieved
- [ ] No race conditions
- [ ] Godoc for all public types

---

## After Phase 1

When Phase 1 is complete:

1. Update `.spec/tracking/PROGRESS.md` to mark Phase 1 complete
2. Phase 2 (Conditional) can start - adds `AddConditionalEdge`
3. Phase 4 (LLM) can start in parallel - adds LLM client

See `.spec/PLANNING.md` for the full roadmap.

---

## Reference Documents

| Document | Use For |
|----------|---------|
| `.spec/phases/PHASE-1-core.md` | Code skeletons, implementation order |
| `.spec/features/*.md` | Detailed behavior specifications |
| `.spec/decisions/*.md` | Why decisions were made |
| `.spec/knowledge/API_SURFACE.md` | Exact public API |
| `.spec/knowledge/TESTING_STRATEGY.md` | Test patterns |
| `docs/GO_PATTERNS.md` | Go idioms to follow |
