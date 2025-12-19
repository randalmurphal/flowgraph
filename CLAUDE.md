# flowgraph

**Go library for graph-based LLM orchestration workflows.** LangGraph-equivalent with checkpointing, conditional branching, and multi-model support.

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
result, err := compiled.RunWithCheckpointing(ctx, initialState, store)
```

**See**: `docs/API_REFERENCE.md` for complete API

---

## Architecture

```
Graph Definition → Compile() → CompiledGraph → Run(ctx, state)
                      ↓
              Validation:
              - Entry point exists
              - All edges reference valid nodes
              - Path to END exists
              - Unreachable nodes (warning)
```

**Checkpoint Stores**: Memory, SQLite, Postgres, Temporal (optional)

**LLM Clients**: Claude CLI, Claude API, OpenAI, Ollama

**See**: `docs/ARCHITECTURE.md` for detailed design

---

## Key Patterns

| Pattern | When | Example |
|---------|------|---------|
| Linear flow | Sequential steps | `a → b → c → END` |
| Conditional branch | Decision points | `review → (approved ? create-pr : fix)` |
| Loop with exit | Retry logic | `implement → review → (pass ? END : implement)` |
| Parallel nodes | Independent work | Future: fan-out/fan-in |

---

## Project Structure

```
flowgraph/
├── graph.go           # Graph definition, AddNode, AddEdge
├── node.go            # Node types, NodeFunc
├── edge.go            # Edge types, conditional edges
├── compile.go         # Graph compilation, validation
├── execute.go         # Run, RunWithCheckpointing
├── context.go         # Execution context
├── errors.go          # Error types
├── checkpoint/        # Checkpoint store implementations
│   ├── store.go       # Interface
│   ├── memory.go
│   ├── sqlite.go
│   └── postgres.go
└── llm/               # LLM client implementations
    ├── client.go      # Interface
    ├── claude_cli.go
    └── mock.go
```

---

## Error Handling

| Error Type | When | Handling |
|------------|------|----------|
| `ErrInvalidNodeID` | Empty/reserved node ID | Fail at AddNode |
| `ErrDuplicateNode` | Same ID added twice | Fail at AddNode |
| `ErrNodeNotFound` | Edge references missing node | Fail at Compile |
| `ErrNoEntryPoint` | No entry node set | Fail at Compile |
| `ErrNoPathToEnd` | Node can't reach END | Fail at Compile |
| `ErrNodeExecution` | Node returns error | Wrapped with node ID |

---

## Testing

```bash
go test -race ./...                    # All tests
go test -race -tags=integration ./...  # With real DBs
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

**Coverage targets**: 90% for core, 85% for checkpoint stores, 80% for LLM clients

**See**: `docs/TESTING_STRATEGY.md` for test patterns

---

## Implementation Status

| Component | Status | Notes |
|-----------|--------|-------|
| Graph definition | Planned | Core types |
| Compilation/validation | Planned | |
| Linear execution | Planned | |
| Conditional edges | Planned | |
| Checkpointing | Planned | Memory first, then SQLite/Postgres |
| LLM clients | Planned | Claude CLI first |
| Temporal backend | Future | Durable execution |
| Parallel nodes | Future | Fan-out/fan-in |

---

## Dependencies

```bash
# Core (minimal)
go get github.com/stretchr/testify  # Testing

# Checkpoint stores
go get github.com/mattn/go-sqlite3  # SQLite
go get github.com/jackc/pgx/v5      # Postgres

# Optional
go get go.temporal.io/sdk           # Temporal backend
```

---

## References

| Doc | Purpose |
|-----|---------|
| `docs/OVERVIEW.md` | Detailed vision and concepts |
| `docs/ARCHITECTURE.md` | Design decisions, data flow |
| `docs/API_REFERENCE.md` | Complete public API |
| `docs/IMPLEMENTATION_GUIDE.md` | Porting from Python patterns |
| `docs/TESTING_STRATEGY.md` | Test patterns and requirements |
| `docs/GO_PATTERNS.md` | Go idioms and gotchas |

---

## Related Repos

- **devflow**: Dev workflow primitives built on flowgraph
- **task-keeper**: Commercial product built on devflow
- **ai-devtools/ensemble**: Python reference implementation
