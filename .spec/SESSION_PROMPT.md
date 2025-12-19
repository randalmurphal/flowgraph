# flowgraph Implementation Session

**Purpose**: Implement flowgraph Phase 5 (Observability)

**Philosophy**: Write production-quality Go code. Follow the specs exactly. Test as you go. No shortcuts.

---

## Context

flowgraph is a Go library for graph-based LLM workflow orchestration. Phases 1-4 are complete. Your job is to implement observability features: structured logging, metrics, and tracing.

### What's Complete

- **Phase 1**: Core graph engine - `pkg/flowgraph/*.go` (87.8% coverage)
- **Phase 2**: Conditional edges - included in Phase 1
- **Phase 3**: Checkpointing - `pkg/flowgraph/checkpoint/` (91.3% coverage)
- **Phase 4**: LLM Clients - `pkg/flowgraph/llm/` (74.7% coverage)
- **27 ADRs** in `decisions/` - all architectural decisions locked
- **10 Feature Specs** in `features/` - detailed behavior specifications
- **6 Phase Specs** in `phases/` - implementation plans with code skeletons

### What's Ready to Build

- **Phase 5**: Observability (structured logging, OpenTelemetry metrics/tracing)

---

## Your Task: Implement Phase 5 Observability

**Goal**: Add production-grade observability: structured logging via slog, metrics and tracing via OpenTelemetry.

**Estimated Effort**: 2-3 days

### Files to Create

```
pkg/flowgraph/observability/
├── logger.go       # slog integration helpers
├── metrics.go      # OpenTelemetry metrics
├── tracing.go      # OpenTelemetry tracing
├── noop.go         # No-op implementations
├── logger_test.go
├── metrics_test.go
├── tracing_test.go
└── noop_test.go
```

### Files to Modify

```
pkg/flowgraph/
├── options.go      # Add WithLogger, WithMetrics, WithTracing
├── execute.go      # Add observability hooks
```

---

## Implementation Order

### Step 1: Logger Helpers (~2 hours)

Create `observability/logger.go` with slog enrichment helpers:

```go
// EnrichLogger adds flowgraph context to a logger
func EnrichLogger(logger *slog.Logger, runID, nodeID string, attempt int) *slog.Logger

// LogNodeStart/Complete/Error for structured logging
func LogNodeStart(logger *slog.Logger, nodeID string)
func LogNodeComplete(logger *slog.Logger, nodeID string, durationMs float64)
func LogNodeError(logger *slog.Logger, nodeID string, err error)
func LogCheckpoint(logger *slog.Logger, nodeID string, sizeBytes int)
```

### Step 2: OpenTelemetry Metrics (~3 hours)

Create `observability/metrics.go`:

```go
// Metrics to emit:
// - flowgraph.node.executions{node_id="..."}
// - flowgraph.node.latency_ms{node_id="..."}
// - flowgraph.node.errors{node_id="..."}
// - flowgraph.graph.runs{success="true|false"}
// - flowgraph.checkpoint.size_bytes{node_id="..."}

func RecordNodeExecution(ctx context.Context, nodeID string, duration time.Duration, err error)
func RecordGraphRun(ctx context.Context, success bool)
func RecordCheckpoint(ctx context.Context, nodeID string, sizeBytes int64)
```

### Step 3: OpenTelemetry Tracing (~3 hours)

Create `observability/tracing.go`:

```go
// Spans created:
// flowgraph.run (parent span)
//   ├── flowgraph.node.a
//   ├── flowgraph.node.b
//   └── flowgraph.node.c

func StartRunSpan(ctx context.Context, graphName, runID string) (context.Context, trace.Span)
func StartNodeSpan(ctx context.Context, nodeID string) (context.Context, trace.Span)
func EndSpanWithError(span trace.Span, err error)
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue)
```

### Step 4: No-op Implementations (~30 min)

Create `observability/noop.go` for when observability is disabled:

```go
type NoopMetrics struct{}
func (NoopMetrics) RecordNodeExecution(...) {}
func (NoopMetrics) RecordGraphRun(...) {}
func (NoopMetrics) RecordCheckpoint(...) {}
```

### Step 5: Options Integration (~2 hours)

Add to `options.go`:

```go
func WithLogger(logger *slog.Logger) RunOption
func WithMetrics(enabled bool) RunOption
func WithTracing(enabled bool) RunOption
```

### Step 6: Execute Integration (~2 hours)

Modify `execute.go` to:
- Start/end run span if tracing enabled
- Start/end node spans around each node execution
- Record metrics after each node
- Log with enriched logger

### Step 7: Tests (~2 hours)

- Logger tests with mock handlers
- Metrics tests with test meter provider
- Tracing tests with mock span processor
- Integration test with full graph execution

---

## Key Decisions (Don't Re-Decide)

| Topic | Decision | Notes |
|-------|----------|-------|
| Logging library | slog (stdlib) | Go 1.21+, no dependency |
| Metrics/Tracing | OpenTelemetry | Industry standard |
| Default state | Disabled | Opt-in via WithLogger/WithMetrics/WithTracing |
| Overhead | No-op when disabled | No performance impact |

---

## Quality Requirements

### Code Quality

- All public types have godoc comments
- All functions handle errors explicitly
- No `_` for ignored errors
- Use `fmt.Errorf("operation: %w", err)` for wrapping

### Testing

- Table-driven tests using testify
- 85% coverage target
- Race detection: `go test -race ./...`
- Test both enabled and disabled paths

### Style

- `gofmt -s -w .` before commit
- `go vet ./...` clean
- Follow patterns from existing core code

---

## Dependencies to Add

```go
// go.mod additions
require (
    go.opentelemetry.io/otel v1.24.0
    go.opentelemetry.io/otel/metric v1.24.0
    go.opentelemetry.io/otel/trace v1.24.0
)
```

Note: slog is stdlib (Go 1.21+), no dependency needed.

---

## Acceptance Criteria

### Structured Logging Works

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

result, err := compiled.Run(ctx, state,
    flowgraph.WithLogger(logger),
    flowgraph.WithRunID("run-123"))

// Logs include: run_id, node_id, duration_ms, errors
```

### Metrics Work (with OTel provider configured)

```go
result, err := compiled.Run(ctx, state,
    flowgraph.WithMetrics(true))

// Metrics emitted:
// - flowgraph.node.executions{node_id="process"}
// - flowgraph.node.latency_ms{node_id="process"}
// - flowgraph.graph.runs{success="true"}
```

### Tracing Works (with OTel provider configured)

```go
result, err := compiled.Run(ctx, state,
    flowgraph.WithTracing(true))

// Spans created in hierarchy:
// flowgraph.run
//   ├── flowgraph.node.start
//   ├── flowgraph.node.process
//   └── flowgraph.node.end
```

### No Overhead When Disabled

```go
// Default - no observability overhead
result, err := compiled.Run(ctx, state)
```

---

## Checklist

- [ ] observability/logger.go with slog helpers
- [ ] observability/metrics.go with OTel metrics
- [ ] observability/tracing.go with OTel tracing
- [ ] observability/noop.go with no-op implementations
- [ ] WithLogger RunOption
- [ ] WithMetrics RunOption
- [ ] WithTracing RunOption
- [ ] Execute integration (spans, metrics, logging hooks)
- [ ] All tests passing
- [ ] 85% coverage achieved
- [ ] No race conditions
- [ ] go.mod updated with OTel dependencies

---

## Reference Code

### Existing Options Pattern (options.go)

```go
func WithCheckpointing(store checkpoint.Store) RunOption {
    return func(c *runConfig) {
        c.checkpointStore = store
    }
}
```

### Existing Execute Loop (execute.go)

```go
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error) {
    cfg := defaultRunConfig()
    for _, opt := range opts {
        opt(&cfg)
    }
    // ... execution loop
}
```

### Error Wrapping Pattern

```go
return s, fmt.Errorf("node %s: %w", current, err)
```

---

## First Steps

1. **Create directory**: `mkdir -p pkg/flowgraph/observability`

2. **Start with logger.go** - simplest, no dependencies

3. **Add OTel dependencies** when ready for metrics/tracing:
   ```bash
   go get go.opentelemetry.io/otel@v1.24.0
   go get go.opentelemetry.io/otel/metric@v1.24.0
   go get go.opentelemetry.io/otel/trace@v1.24.0
   ```

4. **Write tests as you implement** - don't defer testing

5. **Run frequently**:
   ```bash
   go test -race ./...
   go vet ./...
   ```

---

## After This Phase

When Phase 5 is complete:

1. Update `.spec/tracking/PROGRESS.md` to mark phase complete
2. Phase 6 (Polish) can start - examples, documentation, API review
3. See `.spec/phases/PHASE-6-polish.md` for final phase spec

---

## Reference Documents

| Document | Use For |
|----------|---------|
| `.spec/phases/PHASE-5-observability.md` | Complete code skeletons |
| `.spec/tracking/PROGRESS.md` | Progress tracking |
| `pkg/flowgraph/*.go` | Reference implementation patterns |
| `pkg/flowgraph/checkpoint/` | Reference for subpackage structure |
| `pkg/flowgraph/llm/` | Reference for subpackage structure |
