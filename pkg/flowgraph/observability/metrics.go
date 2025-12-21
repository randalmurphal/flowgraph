package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsRecorder records flowgraph metrics.
// Use NewMetricsRecorder() for OTel metrics or NoopMetrics{} when disabled.
type MetricsRecorder interface {
	// RecordNodeExecution records a node execution with its duration and error status.
	RecordNodeExecution(ctx context.Context, nodeID string, duration time.Duration, err error)

	// RecordGraphRun records a graph run completion.
	RecordGraphRun(ctx context.Context, success bool, duration time.Duration)

	// RecordCheckpoint records a checkpoint save operation.
	RecordCheckpoint(ctx context.Context, nodeID string, sizeBytes int64)
}

// otelMetrics implements MetricsRecorder using OpenTelemetry.
type otelMetrics struct {
	nodeExecutions metric.Int64Counter
	nodeLatency    metric.Float64Histogram
	nodeErrors     metric.Int64Counter
	graphRuns      metric.Int64Counter
	graphLatency   metric.Float64Histogram
	checkpointSize metric.Int64Histogram
}

var (
	defaultMetrics     *otelMetrics
	defaultMetricsOnce sync.Once
	defaultMetricsErr  error
)

// getDefaultMetrics returns the default OTel metrics instance.
// Lazily initializes the metrics on first call.
func getDefaultMetrics() (*otelMetrics, error) {
	defaultMetricsOnce.Do(func() {
		defaultMetrics, defaultMetricsErr = newOtelMetrics()
	})
	return defaultMetrics, defaultMetricsErr
}

// newOtelMetrics creates a new OTel metrics instance.
func newOtelMetrics() (*otelMetrics, error) {
	meter := otel.Meter("flowgraph")

	nodeExecutions, err := meter.Int64Counter("flowgraph.node.executions",
		metric.WithDescription("Number of node executions"),
	)
	if err != nil {
		return nil, err
	}

	nodeLatency, err := meter.Float64Histogram("flowgraph.node.latency_ms",
		metric.WithDescription("Node execution latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	nodeErrors, err := meter.Int64Counter("flowgraph.node.errors",
		metric.WithDescription("Number of node execution errors"),
	)
	if err != nil {
		return nil, err
	}

	graphRuns, err := meter.Int64Counter("flowgraph.graph.runs",
		metric.WithDescription("Number of graph runs"),
	)
	if err != nil {
		return nil, err
	}

	graphLatency, err := meter.Float64Histogram("flowgraph.graph.latency_ms",
		metric.WithDescription("Graph run latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	checkpointSize, err := meter.Int64Histogram("flowgraph.checkpoint.size_bytes",
		metric.WithDescription("Checkpoint size in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	return &otelMetrics{
		nodeExecutions: nodeExecutions,
		nodeLatency:    nodeLatency,
		nodeErrors:     nodeErrors,
		graphRuns:      graphRuns,
		graphLatency:   graphLatency,
		checkpointSize: checkpointSize,
	}, nil
}

// NewMetricsRecorder returns a MetricsRecorder that uses OpenTelemetry.
// If metrics initialization fails, returns a no-op recorder.
//
// The recorder uses the global OTel meter provider. Configure the provider
// before calling this function:
//
//	import "go.opentelemetry.io/otel"
//	otel.SetMeterProvider(yourProvider)
func NewMetricsRecorder() MetricsRecorder {
	m, err := getDefaultMetrics()
	if err != nil {
		slog.Warn("metrics initialization failed, using no-op recorder",
			slog.String("error", err.Error()))
		return NoopMetrics{}
	}
	return m
}

// RecordNodeExecution records a node execution.
func (m *otelMetrics) RecordNodeExecution(ctx context.Context, nodeID string, duration time.Duration, err error) {
	attrs := []attribute.KeyValue{
		attribute.String("node_id", nodeID),
	}

	m.nodeExecutions.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.nodeLatency.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))

	if err != nil {
		m.nodeErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordGraphRun records a graph run.
func (m *otelMetrics) RecordGraphRun(ctx context.Context, success bool, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.Bool("success", success),
	}
	m.graphRuns.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.graphLatency.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
}

// RecordCheckpoint records a checkpoint save.
func (m *otelMetrics) RecordCheckpoint(ctx context.Context, nodeID string, sizeBytes int64) {
	attrs := []attribute.KeyValue{
		attribute.String("node_id", nodeID),
	}
	m.checkpointSize.Record(ctx, sizeBytes, metric.WithAttributes(attrs...))
}
