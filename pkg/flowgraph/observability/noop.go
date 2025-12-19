package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// NoopMetrics is a MetricsRecorder that does nothing.
// Use when metrics are disabled to avoid overhead.
type NoopMetrics struct{}

// Compile-time interface check.
var _ MetricsRecorder = NoopMetrics{}

// RecordNodeExecution does nothing.
func (NoopMetrics) RecordNodeExecution(_ context.Context, _ string, _ time.Duration, _ error) {}

// RecordGraphRun does nothing.
func (NoopMetrics) RecordGraphRun(_ context.Context, _ bool, _ time.Duration) {}

// RecordCheckpoint does nothing.
func (NoopMetrics) RecordCheckpoint(_ context.Context, _ string, _ int64) {}

// NoopSpanManager is a SpanManager that does nothing.
// Use when tracing is disabled to avoid overhead.
type NoopSpanManager struct{}

// Compile-time interface check.
var _ SpanManager = NoopSpanManager{}

// noopSpan is a span that does nothing.
// We use the OTel noop package for a proper no-op span implementation.
var noopSpan = noop.Span{}

// StartRunSpan returns the context unchanged and a no-op span.
func (NoopSpanManager) StartRunSpan(ctx context.Context, _, _ string) (context.Context, trace.Span) {
	return ctx, noopSpan
}

// StartNodeSpan returns the context unchanged and a no-op span.
func (NoopSpanManager) StartNodeSpan(ctx context.Context, _ string) (context.Context, trace.Span) {
	return ctx, noopSpan
}

// EndSpanWithError does nothing.
func (NoopSpanManager) EndSpanWithError(_ trace.Span, _ error) {}

// AddSpanEvent does nothing.
func (NoopSpanManager) AddSpanEvent(_ context.Context, _ string, _ ...attribute.KeyValue) {}
