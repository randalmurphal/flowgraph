# flowgraph

**Go library for graph-based LLM orchestration workflows.** LangGraph-equivalent with checkpointing, conditional branching, and multi-model support.

**Status**: Production-ready. Coverage: 93%+. See `.spec/tracking/PROGRESS.md` for history.

---

## Package Overview

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `pkg/flowgraph/` | Core orchestration | `Graph[S]`, `CompiledGraph[S]`, `Context`, `NodeFunc[S]` |
| `pkg/flowgraph/checkpoint/` | State persistence | `Store`, `MemoryStore`, `SQLiteStore` |
| `pkg/flowgraph/config/` | Type-safe config extraction | `Config`, `FromFile`, `FromYAML`, `FromJSON` |
| `pkg/flowgraph/errors/` | Error handling strategies | `Category`, `RetryConfig`, `Handler` |
| `pkg/flowgraph/event/` | Event-driven architecture | `Event`, `Router`, `Bus`, `DLQ`, `PoisonPillDetector` |
| `pkg/flowgraph/expr/` | Expression evaluation | `Evaluator`, `Eval`, `BinaryOp` |
| `pkg/flowgraph/llm/` | LLM client interface + credentials | `Client`, `ClaudeCLI`, `Credentials`, `MockClient` |
| `pkg/flowgraph/llm/tokens/` | Token counting, budget, model limits | `Counter`, `Budget`, `ModelLimits` |
| `pkg/flowgraph/llm/truncate/` | Truncation strategies (FromEnd, FromMiddle, FromStart) | `Strategy`, `Truncator`, `Options` |
| `pkg/flowgraph/llm/template/` | Prompt templates with Handlebars syntax | `Engine`, `Template`, `Render` |
| `pkg/flowgraph/llm/parser/` | Extract JSON, YAML, code blocks from responses | `Parser`, `Extract`, `CodeBlock` |
| `pkg/flowgraph/model/` | Model selection & cost | `ModelName`, `Selector`, `EscalationChain`, `CostTracker` |
| `pkg/flowgraph/observability/` | Logging/metrics/tracing | `MetricsRecorder`, `SpanManager` |
| `pkg/flowgraph/query/` | Read-only workflow inspection (Temporal-inspired) | `Handler`, `Registry`, `Executor`, `State` |
| `pkg/flowgraph/registry/` | Generic thread-safe registry | `Registry[K,V]`, `GetOrCreate`, `Range` |
| `pkg/flowgraph/saga/` | Distributed transactions with compensation | `Step`, `Definition`, `Execution`, `Orchestrator` |
| `pkg/flowgraph/signal/` | Fire-and-forget workflow signals (Temporal-inspired) | `Signal`, `Registry`, `Store`, `Dispatcher` |
| `pkg/flowgraph/template/` | Variable expansion | `Expander`, `Expand`, `ExpandAll`, `ExpandMap` |

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

### LLM in Containers
```go
// For containers where credentials are mounted to a custom location
client := llm.NewClaudeCLI(
    llm.WithHomeDir("/home/worker"),        // Where ~/.claude is mounted
    llm.WithDangerouslySkipPermissions(),   // Non-interactive mode
)

// Load and validate credentials
creds, err := llm.LoadCredentialsFromDir("/home/worker/.claude")
if err != nil {
    log.Fatal(err)
}
if creds.IsExpiringSoon(10 * time.Minute) {
    log.Warn("credentials expiring soon", "in", creds.ExpiresIn())
}
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
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/model"

selector := model.NewSelector(
    model.WithThinkingModel(model.ModelOpus),
    model.WithDefaultModel(model.ModelSonnet),
    model.WithFastModel(model.ModelHaiku),
)
m := selector.SelectForTier(model.TierThinking) // returns ModelOpus
```

### Error Handling with Retry & Escalation
```go
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/errors"

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

## Event-Driven Architecture

The `event` package provides infrastructure for event-driven systems:

```go
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/event"

// Create typed events
evt := event.New[UserCreated]("user.created", "auth-service", "tenant-1", payload)

// Router with middleware
router := event.NewRouter(event.RouterConfig{MaxDepth: 10})
router.Use(event.RecoveryMiddleware())
router.Use(event.LoggingMiddleware(logFn))
router.Register(myHandler)

derived, err := router.Route(ctx, evt)

// Pub/sub bus
bus := event.NewBus(event.BusConfig{BufferSize: 100})
sub := bus.Subscribe([]string{"user.*"}, handler)
bus.Publish(ctx, evt)

// Dead Letter Queue with poison pill detection
dlq := event.NewInMemoryDLQ(event.DLQConfig{MaxRetries: 5})
detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
    FailureThreshold: 3,
})
dlqWithDetection := event.NewDLQWithPoisonPillDetection(dlq, detector)
```

**Key Features**:
- Generic `BaseEvent[T]` with correlation IDs and versioning
- Schema registry with validation and version compatibility
- Middleware support (logging, recovery, metrics, correlation)
- Fan-out pub/sub with deduplication
- Fan-in aggregation (correlation, count, time-window based)
- DLQ/PLQ with retry, exponential backoff, poison pill detection

---

## Temporal-Inspired Patterns

### Signals (Fire-and-Forget)
```go
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/signal"

// Registry for signal handlers
registry := signal.NewRegistry()
registry.Register("cancel", func(ctx context.Context, targetID string, sig *signal.Signal) error {
    // Handle cancel signal
    return nil
})

// Store + Dispatcher for signal delivery
store := signal.NewMemoryStore()
dispatcher := signal.NewDispatcher(registry, store)

// Send signal to a running workflow
sig := &signal.Signal{Name: "cancel", TargetID: "run-123", Payload: map[string]any{"reason": "user request"}}
dispatcher.Send(ctx, sig)

// Process pending signals
dispatcher.Process(ctx, "run-123")
```

### Queries (Read-Only Inspection)
```go
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/query"

// Registry with built-in queries
registry := query.NewRegistry()
query.RegisterBuiltins(registry, func(ctx context.Context, targetID string) (*query.State, error) {
    // Load state from your storage
    return &query.State{TargetID: targetID, Status: "running", Progress: 0.5}, nil
})

// Execute queries
executor := query.NewExecutor(registry, stateLoader)
status, _ := executor.Execute(ctx, "run-123", query.QueryStatus, nil)
progress, _ := executor.Execute(ctx, "run-123", query.QueryProgress, nil)

// Built-in queries: status, progress, current_node, variables, pending_task, state
```

### Saga (Distributed Transactions)
```go
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/saga"

// In-memory orchestrator (default)
orch := saga.NewOrchestrator()

// Or with persistent store for durability
store := saga.NewMemoryStore()  // Or implement saga.Store for PostgreSQL, etc.
orch := saga.NewOrchestrator(saga.WithStore(store), saga.WithLogger(logger))

orch.Register(&saga.Definition{
    Name: "order-saga",
    Steps: []saga.Step{
        {Name: "create-order", Handler: createOrder, Compensation: cancelOrder},
        {Name: "reserve-inventory", Handler: reserveInventory, Compensation: releaseInventory},
        {Name: "charge-payment", Handler: chargePayment, Compensation: refundPayment},
    },
})

// Start saga - runs async, compensates on failure
exec, _ := orch.Start(ctx, "order-saga", orderInput)

// Check status
exec = orch.Get(exec.ID)
fmt.Println(exec.Status) // completed, compensated, or failed

// Manual compensation
orch.Compensate(ctx, exec.ID, "manual rollback requested")

// List executions with filter
execs, _ := orch.ListContext(ctx, &saga.ListFilter{Status: saga.StatusRunning})
```

**Saga Store Interface** (for custom persistence):
```go
type Store interface {
    Create(ctx context.Context, execution *Execution) error
    Update(ctx context.Context, execution *Execution) error
    Get(ctx context.Context, executionID string) (*Execution, error)
    List(ctx context.Context, filter *ListFilter) ([]*Execution, error)
    Delete(ctx context.Context, executionID string) error
}
```

---

## Parallel Execution (Fork/Join)

Flowgraph supports parallel branch execution with goroutine orchestration:

```go
// State must implement ParallelState for custom clone/merge
type ParallelState[S any] interface {
    CloneForBranch(branchID string) S
    Merge(branchStates map[string]S) S
}

// BranchHook provides lifecycle callbacks for parallel branches
type BranchHook[S any] interface {
    OnFork(ctx Context, branchID string, state S) (S, error)
    OnJoin(ctx Context, branchStates map[string]S) error
    OnBranchError(ctx Context, branchID string, state S, err error)
}

// Configure parallel execution
graph := flowgraph.NewGraph[State]().
    SetBranchHook(myHook).
    SetForkJoinConfig(flowgraph.ForkJoinConfig{
        MaxConcurrency: 4,      // 0 = unlimited
        FailFast:       false,  // Wait for all branches
        MergeTimeout:   0,      // 0 = no timeout
    })
```

**Checkpoint branch fields** (for resuming mid-fork):
```go
checkpoint.New(runID, nodeID, seq, state, nextNode).
    WithBranch(branchID, forkNodeID)  // Track branch context
```

---

## Key Features

- **Type-safe graphs** with Go generics
- **Conditional branching** via router functions
- **Parallel execution** via fork/join with goroutines
- **Loops** with max iterations protection
- **Crash recovery** via checkpointing (including branch state)
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
