# flowgraph

**Go library for graph-based LLM orchestration workflows.** LangGraph-equivalent with checkpointing, conditional branching, and multi-model support.

---

## Current Status: Ready for Implementation

**All specifications are complete.** The `.spec/` directory contains everything needed to implement flowgraph v1.0 without making architectural decisions.

| Phase | Status | Spec |
|-------|--------|------|
| Phase 1: Core Graph | **Ready to Start** | `.spec/phases/PHASE-1-core.md` |
| Phase 2: Conditional | Blocked (needs P1) | `.spec/phases/PHASE-2-conditional.md` |
| Phase 3: Checkpointing | Blocked (needs P2) | `.spec/phases/PHASE-3-checkpointing.md` |
| Phase 4: LLM Clients | Ready (after P1) | `.spec/phases/PHASE-4-llm.md` |
| Phase 5: Observability | Blocked (needs P2) | `.spec/phases/PHASE-5-observability.md` |
| Phase 6: Polish | Blocked (needs all) | `.spec/phases/PHASE-6-polish.md` |

**Start here**: `.spec/SESSION_PROMPT.md` for implementation handoff.

---

## Specification Structure

```
.spec/
├── PLANNING.md              # Implementation phases overview
├── DECISIONS.md             # Quick reference for 27 ADRs
├── SESSION_PROMPT.md        # Implementation handoff prompt
├── decisions/               # 27 Architecture Decision Records
├── features/                # 10 feature specifications
│   ├── graph-builder.md
│   ├── compilation.md
│   ├── linear-execution.md
│   ├── conditional-edges.md
│   ├── loop-execution.md
│   ├── checkpointing.md
│   ├── resume.md
│   ├── llm-client.md
│   ├── context-interface.md
│   └── error-handling.md
├── phases/                  # 6 implementation phases
├── knowledge/               # Supporting documents
│   ├── API_SURFACE.md       # Frozen public API
│   ├── TESTING_STRATEGY.md  # Test patterns
│   └── DECISIONS-REVISITED.md  # Open questions resolved
└── tracking/
    └── PROGRESS.md          # Implementation progress
```

---

## Vision

Foundation layer for AI workflow systems. Generic, reusable, suitable for any LLM application. Part of a three-layer ecosystem:

| Layer | Purpose | Repo |
|-------|---------|------|
| **flowgraph** | Graph orchestration engine (this repo) | Open source |
| devflow | Dev workflow primitives (git, Claude CLI, transcripts) | Open source |
| task-keeper | Commercial SaaS product | Commercial |

**Design Philosophy**: Accept interfaces, return structs. Context everywhere. Explicit errors. Small interfaces.

---

## Core Concepts

| Concept | Description | Key Type |
|---------|-------------|----------|
| **Graph** | Directed graph of nodes connected by edges | `Graph[S any]` |
| **State** | Typed struct flowing through graph, accumulating data | Generic `S` |
| **Node** | Function receiving state+context, performs work, returns updated state | `NodeFunc[S]` |
| **Edge** | Connection between nodes (unconditional or conditional) | `Edge` |
| **Checkpoint** | Persisted state snapshot for recovery/replay | `CheckpointStore` |

---

## API Quick Reference

```go
// Build graph
graph := flowgraph.NewGraph[MyState]().
    AddNode("fetch", fetchNode).
    AddNode("process", processNode).
    AddEdge("fetch", "process").
    AddConditionalEdge("process", routerFunc).
    SetEntry("fetch")

// Compile and run
compiled, err := graph.Compile()
result, err := compiled.Run(ctx, initialState)

// With checkpointing
result, err := compiled.Run(ctx, initialState,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))
```

**Complete API**: `.spec/knowledge/API_SURFACE.md`

---

## Implementation Guide

### Starting Phase 1

1. Read `.spec/phases/PHASE-1-core.md` for exact file list and order
2. Start with `errors.go` - defines all error types
3. Continue with `node.go`, `context.go`, `graph.go`, etc.
4. Follow the code skeletons in the phase spec
5. Write tests as you go (see `.spec/knowledge/TESTING_STRATEGY.md`)

### Key Decisions (Already Made)

| Decision | Choice | ADR |
|----------|--------|-----|
| State management | Pass by value, return new | ADR-001 |
| Error handling | Sentinel + typed + wrapping | ADR-002 |
| Validation timing | Panic at build, error at compile | ADR-007 |
| Execution model | Synchronous (parallel in v2) | ADR-010 |
| Checkpoint format | JSON with metadata | ADR-014 |

All 27 ADRs are in `.spec/decisions/`.

### Testing Requirements

| Package | Coverage Target |
|---------|-----------------|
| flowgraph (core) | 90% |
| flowgraph/checkpoint | 85% |
| flowgraph/llm | 80% |

See `.spec/knowledge/TESTING_STRATEGY.md` for patterns.

---

## Project Structure (Target)

```
pkg/flowgraph/
├── graph.go           # Graph definition, AddNode, AddEdge
├── node.go            # Node types, NodeFunc
├── edge.go            # Edge types, conditional edges
├── compile.go         # Graph compilation, validation
├── execute.go         # Run, Resume
├── context.go         # Execution context
├── errors.go          # Error types
├── options.go         # RunOption, ContextOption
├── checkpoint/        # Checkpoint store implementations
│   ├── store.go       # Interface
│   ├── memory.go
│   └── sqlite.go
└── llm/               # LLM client implementations
    ├── client.go      # Interface
    ├── claude_cli.go
    └── mock.go
```

---

## Error Handling

| Error Type | When | Handling |
|------------|------|----------|
| Panic | Empty/reserved node ID, nil function | Fail at AddNode |
| `ErrNoEntryPoint` | No entry node set | Fail at Compile |
| `ErrNodeNotFound` | Edge references missing node | Fail at Compile |
| `ErrNoPathToEnd` | No path to END | Fail at Compile |
| `NodeError` | Node returns error | Wrapped with node ID |
| `PanicError` | Node panics | Captured with stack |

---

## Dependencies

```bash
# Core (minimal)
go get github.com/stretchr/testify  # Testing

# Checkpoint stores (Phase 3)
go get modernc.org/sqlite           # Pure Go SQLite

# Observability (Phase 5)
go get go.opentelemetry.io/otel     # Metrics/Tracing
```

---

## References

| Doc | Purpose |
|-----|---------|
| `.spec/PLANNING.md` | Implementation phases |
| `.spec/DECISIONS.md` | Quick ADR reference |
| `.spec/knowledge/API_SURFACE.md` | Complete public API |
| `.spec/knowledge/TESTING_STRATEGY.md` | Test patterns |
| `docs/ARCHITECTURE.md` | Design decisions, data flow |
| `docs/GO_PATTERNS.md` | Go idioms and gotchas |

---

## Related Repos

- **devflow**: Dev workflow primitives built on flowgraph
- **task-keeper**: Commercial product built on devflow
