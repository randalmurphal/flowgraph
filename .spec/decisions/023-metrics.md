# ADR-023: Metrics Interface

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should flowgraph expose metrics? Options:
- Built-in Prometheus
- OpenTelemetry metrics
- Custom interface
- Hooks only (no built-in)

## Decision

**OpenTelemetry metrics with fallback to hooks for custom implementations.**

### Approach

```go
// Optional metrics provider
type MetricsProvider interface {
    // Counters
    NodeExecutions(nodeID string) Counter
    NodeFailures(nodeID string) Counter

    // Histograms
    NodeDuration(nodeID string) Histogram
    LLMTokensIn(nodeID string) Histogram
    LLMTokensOut(nodeID string) Histogram

    // Gauges
    ActiveRuns() Gauge
}

// Minimal interfaces (OTel compatible)
type Counter interface {
    Add(ctx context.Context, value int64, attrs ...attribute.KeyValue)
}

type Histogram interface {
    Record(ctx context.Context, value float64, attrs ...attribute.KeyValue)
}

type Gauge interface {
    Set(ctx context.Context, value int64, attrs ...attribute.KeyValue)
}
```

### Built-in OTel Implementation

```go
package flowgraph

import (
    "go.opentelemetry.io/otel/metric"
)

type otelMetrics struct {
    nodeExecutions metric.Int64Counter
    nodeFailures   metric.Int64Counter
    nodeDuration   metric.Float64Histogram
    llmTokensIn    metric.Int64Histogram
    llmTokensOut   metric.Int64Histogram
    activeRuns     metric.Int64UpDownCounter
}

func NewOTelMetrics(meter metric.Meter) (*otelMetrics, error) {
    m := &otelMetrics{}
    var err error

    m.nodeExecutions, err = meter.Int64Counter("flowgraph.node.executions",
        metric.WithDescription("Number of node executions"),
    )
    if err != nil {
        return nil, err
    }

    m.nodeDuration, err = meter.Float64Histogram("flowgraph.node.duration",
        metric.WithDescription("Node execution duration in seconds"),
        metric.WithUnit("s"),
    )
    // ... create other metrics

    return m, nil
}
```

### Integration with Execution

```go
func (cg *CompiledGraph[S]) executeNode(ctx Context, nodeID string, state S) (S, error) {
    if metrics := ctx.Metrics(); metrics != nil {
        metrics.NodeExecutions(nodeID).Add(ctx, 1)
        defer func(start time.Time) {
            metrics.NodeDuration(nodeID).Record(ctx, time.Since(start).Seconds())
        }(time.Now())
    }

    result, err := cg.runWithRecovery(ctx, nodeID, state)

    if err != nil && metrics != nil {
        metrics.NodeFailures(nodeID).Add(ctx, 1)
    }

    return result, err
}
```

## Alternatives Considered

### 1. Direct Prometheus

```go
import "github.com/prometheus/client_golang/prometheus"

var nodeExecutions = prometheus.NewCounterVec(...)
```

**Rejected**: Vendor lock-in. OTel is vendor-neutral and can export to Prometheus.

### 2. Custom Interface Only

```go
type MetricsCollector interface {
    RecordNodeExecution(nodeID string, duration time.Duration, err error)
    RecordLLMCall(nodeID string, tokensIn, tokensOut int)
}
```

**Rejected**: Would need to reinvent metric types. OTel is standard.

### 3. Hooks Only

```go
// No built-in metrics, users add via hooks
graph.Run(ctx, state,
    WithNodeHooks(start, func(nodeID string, state any, err error) {
        myMetrics.Record(...)
    }),
)
```

**Rejected**: Too low-level. Metrics are common enough to warrant built-in support.

### 4. Statsd

```go
import "github.com/DataDog/datadog-go/statsd"
```

**Rejected**: Push-based metrics, less flexible than OTel.

## Consequences

### Positive
- **Standard** - OpenTelemetry is industry standard
- **Vendor-neutral** - Export to Prometheus, Datadog, etc.
- **Optional** - No metrics if not configured
- **Extensible** - Users can add custom metrics

### Negative
- OTel adds dependency
- More complex setup than simple counters

### Risks
- OTel overhead → Benchmarked, <1% impact

---

## Metrics Reference

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| flowgraph.node.executions | Counter | node_id, graph | Number of node executions |
| flowgraph.node.failures | Counter | node_id, graph, error_type | Number of node failures |
| flowgraph.node.duration | Histogram | node_id, graph | Execution duration (seconds) |
| flowgraph.node.panics | Counter | node_id, graph | Number of panics |
| flowgraph.run.duration | Histogram | graph | Total run duration |
| flowgraph.run.active | Gauge | graph | Currently running graphs |
| flowgraph.checkpoint.saves | Counter | node_id, graph | Checkpoints saved |
| flowgraph.checkpoint.size | Histogram | node_id, graph | Checkpoint size (bytes) |
| flowgraph.llm.tokens_in | Histogram | node_id, graph, model | Input tokens per call |
| flowgraph.llm.tokens_out | Histogram | node_id, graph, model | Output tokens per call |
| flowgraph.llm.duration | Histogram | node_id, graph, model | LLM call duration |

---

## Usage Examples

### Enable OTel Metrics

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/prometheus"
    "go.opentelemetry.io/otel/sdk/metric"
)

// Setup OTel with Prometheus exporter
exporter, _ := prometheus.New()
provider := metric.NewMeterProvider(metric.WithReader(exporter))
otel.SetMeterProvider(provider)

// Create flowgraph metrics
meter := otel.Meter("flowgraph")
metrics, _ := flowgraph.NewOTelMetrics(meter)

// Use in runs
result, err := compiled.Run(ctx, state,
    flowgraph.WithMetrics(metrics),
)
```

### Custom Metrics Implementation

```go
type MyMetrics struct {
    // Your metrics system
}

func (m *MyMetrics) NodeExecutions(nodeID string) flowgraph.Counter {
    return &myCounter{/* ... */}
}

// Use your implementation
result, err := compiled.Run(ctx, state,
    flowgraph.WithMetrics(myMetrics),
)
```

### No Metrics (Default)

```go
// Metrics not configured, no overhead
result, err := compiled.Run(ctx, state)
```

---

## Prometheus Dashboard

Example queries:

```promql
# Node execution rate
rate(flowgraph_node_executions_total[5m])

# Node failure rate
rate(flowgraph_node_failures_total[5m]) /
rate(flowgraph_node_executions_total[5m])

# 99th percentile node duration
histogram_quantile(0.99, rate(flowgraph_node_duration_bucket[5m]))

# LLM cost estimate (tokens)
sum(rate(flowgraph_llm_tokens_in_total[1h])) +
sum(rate(flowgraph_llm_tokens_out_total[1h])) * 5  # Output costs more
```

---

## Test Cases

```go
func TestMetrics_NodeExecution(t *testing.T) {
    // Create test metrics
    testMetrics := &TestMetrics{}

    compiled, _ := graph.Compile()
    _, _ = compiled.Run(context.Background(), state,
        flowgraph.WithMetrics(testMetrics),
    )

    assert.Equal(t, 3, testMetrics.TotalExecutions())
    assert.Equal(t, 0, testMetrics.TotalFailures())
}

func TestMetrics_NodeFailure(t *testing.T) {
    testMetrics := &TestMetrics{}

    failingCompiled, _ := failingGraph.Compile()
    _, _ = failingCompiled.Run(context.Background(), state,
        flowgraph.WithMetrics(testMetrics),
    )

    assert.Equal(t, 1, testMetrics.TotalExecutions())
    assert.Equal(t, 1, testMetrics.TotalFailures())
}

func TestMetrics_Duration(t *testing.T) {
    testMetrics := &TestMetrics{}

    slowNode := func(ctx flowgraph.Context, s testState) (testState, error) {
        time.Sleep(100 * time.Millisecond)
        return s, nil
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("slow", slowNode).
        AddEdge("slow", flowgraph.END).
        SetEntry("slow").
        Compile()

    _, _ = compiled.Run(context.Background(), testState{},
        flowgraph.WithMetrics(testMetrics),
    )

    duration := testMetrics.NodeDurations["slow"]
    assert.GreaterOrEqual(t, duration, 100*time.Millisecond)
}
```
