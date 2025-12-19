# Observability Package

**Logging, metrics, and tracing for flowgraph workflows.**

---

## Overview

This package provides observability integration using:
- **Logging**: Structured logging via `slog`
- **Metrics**: OpenTelemetry metrics (counters, histograms)
- **Tracing**: OpenTelemetry distributed tracing

---

## Key Types

| Type | Purpose |
|------|---------|
| `MetricsRecorder` | Interface for recording metrics |
| `SpanManager` | Interface for managing trace spans |
| `DefaultMetricsRecorder` | OpenTelemetry metrics implementation |
| `DefaultSpanManager` | OpenTelemetry tracing implementation |
| `NoOpMetricsRecorder` | No-op for disabled metrics |
| `NoOpSpanManager` | No-op for disabled tracing |

---

## Usage

### Enable All Observability

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

result, err := compiled.Run(ctx, state,
    flowgraph.WithObservabilityLogger(logger),
    flowgraph.WithMetrics(true),
    flowgraph.WithTracing(true),
    flowgraph.WithRunID("run-123"))
```

### Logging Only

```go
result, err := compiled.Run(ctx, state,
    flowgraph.WithObservabilityLogger(logger))
```

---

## Logged Events

| Event | Fields |
|-------|--------|
| `run.start` | run_id, graph_id |
| `run.complete` | run_id, duration_ms, final_node |
| `run.error` | run_id, error, node_id |
| `node.start` | run_id, node_id |
| `node.complete` | run_id, node_id, duration_ms |
| `node.error` | run_id, node_id, error |

---

## Metrics

| Metric | Type | Labels |
|--------|------|--------|
| `flowgraph.node.executions` | Counter | graph_id, node_id, status |
| `flowgraph.node.latency_ms` | Histogram | graph_id, node_id |
| `flowgraph.run.duration_ms` | Histogram | graph_id, status |
| `flowgraph.errors` | Counter | graph_id, node_id, error_type |

---

## Tracing

Spans created:
- `flowgraph.run` - Parent span for entire execution
  - `flowgraph.node.{id}` - Child span for each node

Span attributes:
- `flowgraph.run_id`
- `flowgraph.graph_id`
- `flowgraph.node_id`
- `flowgraph.duration_ms`

---

## Files

| File | Purpose |
|------|---------|
| `logger.go` | slog enrichment helpers |
| `metrics.go` | MetricsRecorder interface and implementation |
| `tracing.go` | SpanManager interface and implementation |
| `noop.go` | No-op implementations |

---

## Integration with OpenTelemetry

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
)

// Set up OpenTelemetry exporter
exporter, _ := otlptrace.New(ctx, otlptrace.WithEndpoint("localhost:4317"))
tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
otel.SetTracerProvider(tp)

// flowgraph will use the global tracer provider
result, err := compiled.Run(ctx, state,
    flowgraph.WithTracing(true))
```

---

## Testing

Observability is optional - tests can run without it:

```go
// No observability
result, err := compiled.Run(ctx, state)

// With logging only (useful for debugging tests)
result, err := compiled.Run(ctx, state,
    flowgraph.WithObservabilityLogger(slog.Default()))
```
