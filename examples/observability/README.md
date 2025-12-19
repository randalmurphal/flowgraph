# Observability Example

This example demonstrates enabling logging, metrics, and tracing for monitoring graph execution.

## What It Shows

- Configuring structured logging with `WithObservabilityLogger()`
- Enabling OpenTelemetry metrics with `WithMetrics(true)`
- Enabling distributed tracing with `WithTracing(true)`
- Run ID tracking with `WithRunID()`

## Graph Structure

```
process -> validate -> finalize -> END
   │          │           │
[logged, metered, and traced at each step]
```

## Running

```bash
go run main.go
```

## Expected Output

```json
{"time":"...","level":"INFO","msg":"run_start","run_id":"obs-demo-001"}
{"time":"...","level":"DEBUG","msg":"node_start","run_id":"obs-demo-001","node_id":"process"}
{"time":"...","level":"DEBUG","msg":"node_complete","run_id":"obs-demo-001","node_id":"process","duration_ms":50}
{"time":"...","level":"DEBUG","msg":"node_start","run_id":"obs-demo-001","node_id":"validate"}
{"time":"...","level":"DEBUG","msg":"node_complete","run_id":"obs-demo-001","node_id":"validate","duration_ms":30}
{"time":"...","level":"DEBUG","msg":"node_start","run_id":"obs-demo-001","node_id":"finalize"}
{"time":"...","level":"DEBUG","msg":"node_complete","run_id":"obs-demo-001","node_id":"finalize","duration_ms":20}
{"time":"...","level":"INFO","msg":"run_complete","run_id":"obs-demo-001","duration_ms":100}
```

## Key Concepts

1. **Structured logging**: JSON format with consistent fields
2. **OpenTelemetry metrics**: Standard metrics for monitoring
3. **Distributed tracing**: Span hierarchy for request tracing
4. **Run IDs**: Correlate all observability data

## Observability Features

### Logging Fields

| Field | Description |
|-------|-------------|
| `run_id` | Unique run identifier |
| `node_id` | Current node being executed |
| `duration_ms` | Time taken in milliseconds |
| `attempt` | Attempt number (for retries) |

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `flowgraph.node.executions` | Counter | Node execution count |
| `flowgraph.node.latency_ms` | Histogram | Node execution time |
| `flowgraph.node.errors` | Counter | Node error count |
| `flowgraph.graph.runs` | Counter | Graph run count |
| `flowgraph.graph.latency_ms` | Histogram | Total run time |

### Trace Spans

```
flowgraph.run
├── flowgraph.node.process
├── flowgraph.node.validate
└── flowgraph.node.finalize
```

## Integration

Connect to your observability stack:
- **Logging**: stdout, file, or log aggregator
- **Metrics**: Prometheus, Datadog, etc.
- **Tracing**: Jaeger, Zipkin, etc.
