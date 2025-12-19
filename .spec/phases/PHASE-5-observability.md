# Phase 5: Observability

**Status**: Blocked (Depends on Phase 2)
**Estimated Effort**: 2-3 days
**Dependencies**: Phase 2 Complete

---

## Goal

Add production-grade observability: structured logging, metrics, and distributed tracing.

---

## Files to Create/Modify

```
pkg/flowgraph/
├── observability/
│   ├── logger.go        # slog integration helpers
│   ├── metrics.go       # OpenTelemetry metrics
│   ├── tracing.go       # OpenTelemetry tracing
│   └── noop.go          # No-op implementations
├── context.go           # MODIFY: Enhanced logger
├── options.go           # MODIFY: Observability options
├── execute.go           # MODIFY: Add observability hooks
└── observability_test.go
```

---

## Implementation Order

### Step 1: Logger Enhancement (~2 hours)

**observability/logger.go**
```go
package observability

import (
    "log/slog"
)

// EnrichLogger adds flowgraph context to a logger
func EnrichLogger(logger *slog.Logger, runID, nodeID string, attempt int) *slog.Logger {
    return logger.With(
        slog.String("run_id", runID),
        slog.String("node_id", nodeID),
        slog.Int("attempt", attempt),
    )
}

// LogNodeStart logs node execution start
func LogNodeStart(logger *slog.Logger, nodeID string) {
    logger.Debug("node starting", slog.String("node_id", nodeID))
}

// LogNodeComplete logs successful node completion
func LogNodeComplete(logger *slog.Logger, nodeID string, durationMs float64) {
    logger.Debug("node completed",
        slog.String("node_id", nodeID),
        slog.Float64("duration_ms", durationMs),
    )
}

// LogNodeError logs node execution error
func LogNodeError(logger *slog.Logger, nodeID string, err error) {
    logger.Error("node failed",
        slog.String("node_id", nodeID),
        slog.String("error", err.Error()),
    )
}

// LogCheckpoint logs checkpoint creation
func LogCheckpoint(logger *slog.Logger, nodeID string, sizeBytes int) {
    logger.Debug("checkpoint saved",
        slog.String("node_id", nodeID),
        slog.Int("size_bytes", sizeBytes),
    )
}
```

### Step 2: Metrics (~3 hours)

**observability/metrics.go**
```go
package observability

import (
    "context"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

var (
    meter = otel.Meter("flowgraph")

    nodeExecutions metric.Int64Counter
    nodeLatency    metric.Float64Histogram
    nodeErrors     metric.Int64Counter
    graphRuns      metric.Int64Counter
    checkpointSize metric.Int64Histogram
)

func init() {
    var err error

    nodeExecutions, err = meter.Int64Counter("flowgraph.node.executions",
        metric.WithDescription("Number of node executions"),
    )
    if err != nil {
        panic(err)
    }

    nodeLatency, err = meter.Float64Histogram("flowgraph.node.latency_ms",
        metric.WithDescription("Node execution latency in milliseconds"),
        metric.WithUnit("ms"),
    )
    if err != nil {
        panic(err)
    }

    nodeErrors, err = meter.Int64Counter("flowgraph.node.errors",
        metric.WithDescription("Number of node execution errors"),
    )
    if err != nil {
        panic(err)
    }

    graphRuns, err = meter.Int64Counter("flowgraph.graph.runs",
        metric.WithDescription("Number of graph runs"),
    )
    if err != nil {
        panic(err)
    }

    checkpointSize, err = meter.Int64Histogram("flowgraph.checkpoint.size_bytes",
        metric.WithDescription("Checkpoint size in bytes"),
        metric.WithUnit("By"),
    )
    if err != nil {
        panic(err)
    }
}

// RecordNodeExecution records a node execution
func RecordNodeExecution(ctx context.Context, nodeID string, duration time.Duration, err error) {
    attrs := []attribute.KeyValue{
        attribute.String("node_id", nodeID),
    }

    nodeExecutions.Add(ctx, 1, metric.WithAttributes(attrs...))
    nodeLatency.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))

    if err != nil {
        nodeErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
    }
}

// RecordGraphRun records a graph run
func RecordGraphRun(ctx context.Context, success bool) {
    attrs := []attribute.KeyValue{
        attribute.Bool("success", success),
    }
    graphRuns.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordCheckpoint records a checkpoint save
func RecordCheckpoint(ctx context.Context, nodeID string, sizeBytes int64) {
    attrs := []attribute.KeyValue{
        attribute.String("node_id", nodeID),
    }
    checkpointSize.Record(ctx, sizeBytes, metric.WithAttributes(attrs...))
}
```

### Step 3: Tracing (~3 hours)

**observability/tracing.go**
```go
package observability

import (
    "context"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("flowgraph")

// StartRunSpan starts a span for the entire graph run
func StartRunSpan(ctx context.Context, graphName, runID string) (context.Context, trace.Span) {
    return tracer.Start(ctx, "flowgraph.run",
        trace.WithAttributes(
            attribute.String("graph.name", graphName),
            attribute.String("run.id", runID),
        ),
    )
}

// StartNodeSpan starts a span for a node execution
func StartNodeSpan(ctx context.Context, nodeID string) (context.Context, trace.Span) {
    return tracer.Start(ctx, "flowgraph.node."+nodeID,
        trace.WithAttributes(
            attribute.String("node.id", nodeID),
        ),
    )
}

// EndSpanWithError completes a span, optionally recording an error
func EndSpanWithError(span trace.Span, err error) {
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    } else {
        span.SetStatus(codes.Ok, "")
    }
    span.End()
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
    span := trace.SpanFromContext(ctx)
    span.AddEvent(name, trace.WithAttributes(attrs...))
}
```

### Step 4: No-op Implementations (~30 min)

**observability/noop.go**
```go
package observability

import (
    "context"
    "time"
)

// NoopMetrics provides no-op metric recording
type NoopMetrics struct{}

func (NoopMetrics) RecordNodeExecution(ctx context.Context, nodeID string, duration time.Duration, err error) {}
func (NoopMetrics) RecordGraphRun(ctx context.Context, success bool) {}
func (NoopMetrics) RecordCheckpoint(ctx context.Context, nodeID string, sizeBytes int64) {}
```

### Step 5: Options Integration (~2 hours)

**options.go additions**
```go
// WithLogger sets the logger for execution
func WithLogger(logger *slog.Logger) RunOption {
    return func(c *runConfig) {
        c.logger = logger
    }
}

// WithMetrics enables OpenTelemetry metrics
func WithMetrics(enabled bool) RunOption {
    return func(c *runConfig) {
        c.metricsEnabled = enabled
    }
}

// WithTracing enables OpenTelemetry tracing
func WithTracing(enabled bool) RunOption {
    return func(c *runConfig) {
        c.tracingEnabled = enabled
    }
}
```

### Step 6: Execute Integration (~2 hours)

**execute.go modifications**
```go
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error) {
    cfg := defaultRunConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    // Start run span if tracing enabled
    var runSpan trace.Span
    if cfg.tracingEnabled {
        ctx, runSpan = observability.StartRunSpan(ctx, cg.name, cfg.runID)
        defer func() {
            // err captured from closure
            observability.EndSpanWithError(runSpan, err)
        }()
    }

    // Record run metric
    defer func() {
        if cfg.metricsEnabled {
            observability.RecordGraphRun(ctx, err == nil)
        }
    }()

    // ... existing execution loop ...

    // Inside execution loop, around executeNode:
    if cfg.tracingEnabled {
        nodeCtx, nodeSpan := observability.StartNodeSpan(ctx, current)
        defer observability.EndSpanWithError(nodeSpan, nodeErr)
        // Use nodeCtx for node execution
    }

    start := time.Now()
    state, nodeErr = cg.executeNode(nodeCtx, current, state)
    duration := time.Since(start)

    if cfg.metricsEnabled {
        observability.RecordNodeExecution(ctx, current, duration, nodeErr)
    }

    if cfg.logger != nil {
        if nodeErr != nil {
            observability.LogNodeError(cfg.logger, current, nodeErr)
        } else {
            observability.LogNodeComplete(cfg.logger, current, float64(duration.Milliseconds()))
        }
    }
}
```

### Step 7: Tests (~2 hours)

**observability_test.go**
```go
func TestLogger_Enrichment(t *testing.T)
func TestMetrics_NodeExecution(t *testing.T)
func TestTracing_SpanCreation(t *testing.T)
func TestObservability_Disabled(t *testing.T)
```

---

## Acceptance Criteria

```go
// Structured logging
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

result, err := compiled.Run(ctx, state,
    flowgraph.WithLogger(logger),
    flowgraph.WithRunID("run-123"))

// Logs include: run_id, node_id, duration, errors
```

```go
// Metrics (with OTel provider configured)
result, err := compiled.Run(ctx, state,
    flowgraph.WithMetrics(true))

// Metrics emitted:
// - flowgraph.node.executions{node_id="..."}
// - flowgraph.node.latency_ms{node_id="..."}
// - flowgraph.node.errors{node_id="..."}
// - flowgraph.graph.runs{success="true|false"}
```

```go
// Tracing (with OTel provider configured)
result, err := compiled.Run(ctx, state,
    flowgraph.WithTracing(true))

// Spans created:
// flowgraph.run (parent)
//   ├── flowgraph.node.a
//   ├── flowgraph.node.b
//   └── flowgraph.node.c
```

---

## Test Coverage Targets

| File | Target |
|------|--------|
| observability/logger.go | 95% |
| observability/metrics.go | 80% |
| observability/tracing.go | 80% |
| observability/noop.go | 100% |
| Overall Phase 5 | 85% |

---

## Checklist

- [ ] Logger enrichment helpers
- [ ] slog integration
- [ ] OpenTelemetry metrics
- [ ] OpenTelemetry tracing
- [ ] No-op implementations
- [ ] WithLogger, WithMetrics, WithTracing options
- [ ] Execute integration
- [ ] Tests with mock collectors
- [ ] 85% coverage achieved

---

## Dependencies

```go
// go.mod additions
require (
    go.opentelemetry.io/otel v1.24.0
    go.opentelemetry.io/otel/metric v1.24.0
    go.opentelemetry.io/otel/trace v1.24.0
)
```

---

## Notes

- slog is stdlib (Go 1.21+), no dependency
- OpenTelemetry is optional - disabled by default
- Metrics/tracing require user to configure OTel provider
- No-op implementations prevent overhead when disabled
- All observability is opt-in via RunOptions
