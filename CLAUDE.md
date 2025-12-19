# flowgraph

**Go library for graph-based LLM orchestration workflows.** LangGraph-equivalent with checkpointing, conditional branching, and multi-model support.

---

## Status: v0.1.0 Release Ready

All phases complete. Release branch `release/v0.1.0` created.

| Phase | Status | Coverage |
|-------|--------|----------|
| Phase 1: Core Graph | Complete | 89.1% |
| Phase 2: Conditional | Complete | (in P1) |
| Phase 3: Checkpointing | Complete | 91.3% |
| Phase 4: LLM Clients | Complete | 83.8% |
| Phase 5: Observability | Complete | 90.6% |
| Phase 6: Polish | Complete | 88.2% overall |

---

## Quick Start

```go
// Build a graph
graph := flowgraph.NewGraph[MyState]().
    AddNode("process", processNode).
    AddNode("validate", validateNode).
    AddEdge("process", "validate").
    AddEdge("validate", flowgraph.END).
    SetEntry("process")

compiled, _ := graph.Compile()

// Run it
ctx := flowgraph.NewContext(context.Background())
result, err := compiled.Run(ctx, initialState)
```

See `examples/` for complete runnable examples.

---

## Project Structure

```
flowgraph/
├── CLAUDE.md              # This file - start here
├── README.md              # User-facing documentation
├── CONTRIBUTING.md        # Development guide
├── go.mod
├── pkg/flowgraph/         # Main package
│   ├── doc.go             # Package documentation
│   ├── *.go               # Core implementation
│   ├── checkpoint/        # Checkpoint persistence
│   │   └── CLAUDE.md      # Checkpoint package guide
│   ├── llm/               # LLM client integration
│   │   └── CLAUDE.md      # LLM package guide
│   └── observability/     # Logging, metrics, tracing
│       └── CLAUDE.md      # Observability guide
├── examples/              # Runnable examples
│   ├── linear/
│   ├── conditional/
│   ├── loop/
│   ├── checkpointing/
│   ├── llm/
│   └── observability/
├── benchmarks/            # Performance benchmarks
├── docs/                  # Additional documentation
│   ├── ARCHITECTURE.md    # System design
│   └── DECISIONS.md       # Key ADRs summary
└── .spec/                 # Implementation specs (historical)
    ├── decisions/         # 27 ADRs
    ├── phases/            # Phase specifications
    └── features/          # Feature specifications
```

---

## Key Packages

### `pkg/flowgraph/` - Core Engine

Graph building, compilation, and execution.

```go
graph := flowgraph.NewGraph[S]()     // Create builder
graph.AddNode("id", fn)              // Add node
graph.AddEdge("from", "to")          // Add edge
graph.AddConditionalEdge("from", fn) // Conditional routing
graph.SetEntry("id")                 // Set entry point
compiled, err := graph.Compile()     // Validate and compile
result, err := compiled.Run(ctx, s)  // Execute
```

### `pkg/flowgraph/checkpoint/` - Persistence

Crash recovery via checkpointing.

```go
store := checkpoint.NewSQLiteStore("./checkpoints.db")
result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))

// Resume after crash
result, err := compiled.Resume(ctx, store, "run-123")
```

### `pkg/flowgraph/llm/` - LLM Integration

Claude CLI integration with full feature support.

```go
client := llm.NewClaudeCLI(
    llm.WithModel("sonnet"),
    llm.WithOutputFormat(llm.OutputFormatJSON),
    llm.WithDangerouslySkipPermissions(),
    llm.WithMaxBudgetUSD(1.0),
)

ctx := flowgraph.NewContext(context.Background(), flowgraph.WithLLM(client))

// In node:
resp, err := ctx.LLM().Complete(ctx, llm.CompletionRequest{...})
// resp.SessionID, resp.CostUSD, resp.Usage available
```

### `pkg/flowgraph/observability/` - Observability

Structured logging, OpenTelemetry metrics and tracing.

```go
result, err := compiled.Run(ctx, state,
    flowgraph.WithObservabilityLogger(logger),
    flowgraph.WithMetrics(true),
    flowgraph.WithTracing(true))
```

---

## Testing

```bash
go test -race ./pkg/flowgraph/...              # All tests
go test -coverprofile=coverage.out ./...       # With coverage
go test -bench=. ./benchmarks/...              # Benchmarks
```

---

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| State | Pass by value | Immutability, predictability |
| Errors | Sentinel + typed | Go idioms, `errors.Is/As` support |
| Context | Custom interface | Wrap context.Context with services |
| Panics | Recover to PanicError | Safety, debugging |
| Checkpoints | JSON + SQLite | Simplicity, debuggability |
| LLM | Interface + CLI | Flexibility, Claude Code integration |

Full ADRs in `.spec/decisions/`.

---

## Claude CLI Integration

The `llm.ClaudeCLI` client wraps the Claude Code CLI for LLM operations.

**Reference Implementation**: See `~/repos/ai-devtools/ensemble/core/runner.py` for battle-tested Python patterns.

### Key Patterns

```go
// Production configuration
client := llm.NewClaudeCLI(
    llm.WithOutputFormat(llm.OutputFormatJSON),  // Always use JSON for token tracking
    llm.WithDangerouslySkipPermissions(),        // Non-interactive execution
    llm.WithMaxBudgetUSD(1.0),                   // Cap spending
    llm.WithMaxTurns(10),                        // Limit agentic turns
)
```

### Available Options

| Option | Purpose |
|--------|---------|
| `WithOutputFormat(format)` | text, json, stream-json |
| `WithSessionID(id)` | Track multi-turn conversations |
| `WithContinue()` | Continue last session |
| `WithResume(id)` | Resume specific session |
| `WithSystemPrompt(s)` | Set system prompt |
| `WithAppendSystemPrompt(s)` | Append to system prompt |
| `WithDisallowedTools(tools)` | Blacklist tools |
| `WithDangerouslySkipPermissions()` | Skip interactive prompts |
| `WithMaxBudgetUSD(amount)` | Cap spending |
| `WithMaxTurns(n)` | Limit agentic turns |

### Response Data

When using JSON output format:
- `resp.SessionID` - Session identifier for multi-turn
- `resp.CostUSD` - Total cost in USD
- `resp.NumTurns` - Number of agentic turns
- `resp.Usage.InputTokens` - Input token count
- `resp.Usage.OutputTokens` - Output token count
- `resp.Usage.CacheCreationInputTokens` - Cache write tokens
- `resp.Usage.CacheReadInputTokens` - Cache read tokens

---

## For Agents

### When modifying this codebase:

1. **Read the relevant CLAUDE.md** in the package you're modifying
2. **Check `.spec/decisions/`** for architectural decisions
3. **Run tests with race detection** before committing
4. **Follow existing patterns** - consistency over novelty

### Quality gates:

```bash
go test -race ./...     # Must pass
go vet ./...            # Must be clean
gofmt -s -w .           # Must be applied
```

### Coverage targets:

- Core packages: 85%+
- Overall: 80%+
