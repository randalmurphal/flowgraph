package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Tracer is the flowgraph tracer instance.
// Uses the global OTel tracer provider.
var tracer = otel.Tracer("flowgraph")

// SpanManager handles trace span lifecycle.
// Use NewSpanManager() for OTel tracing or NoopSpanManager{} when disabled.
type SpanManager interface {
	// StartRunSpan starts a span for the entire graph run.
	// Returns the context with span and the span itself.
	StartRunSpan(ctx context.Context, graphName, runID string) (context.Context, trace.Span)

	// StartNodeSpan starts a span for a node execution.
	// The node span should be a child of the run span.
	StartNodeSpan(ctx context.Context, nodeID string) (context.Context, trace.Span)

	// EndSpanWithError completes a span, optionally recording an error.
	EndSpanWithError(span trace.Span, err error)

	// AddSpanEvent adds an event to the current span in context.
	AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue)
}

// otelSpanManager implements SpanManager using OpenTelemetry.
type otelSpanManager struct{}

// NewSpanManager returns a SpanManager that uses OpenTelemetry.
//
// The span manager uses the global OTel tracer provider. Configure the provider
// before calling this function:
//
//	import "go.opentelemetry.io/otel"
//	otel.SetTracerProvider(yourProvider)
func NewSpanManager() SpanManager {
	return &otelSpanManager{}
}

// StartRunSpan starts a span for the entire graph run.
func (m *otelSpanManager) StartRunSpan(ctx context.Context, graphName, runID string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "flowgraph.run",
		trace.WithAttributes(
			attribute.String("graph.name", graphName),
			attribute.String("run.id", runID),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// StartNodeSpan starts a span for a node execution.
func (m *otelSpanManager) StartNodeSpan(ctx context.Context, nodeID string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "flowgraph.node."+nodeID,
		trace.WithAttributes(
			attribute.String("node.id", nodeID),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// EndSpanWithError completes a span, optionally recording an error.
func (m *otelSpanManager) EndSpanWithError(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

// AddSpanEvent adds an event to the current span.
func (m *otelSpanManager) AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return
	}
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// Convenience functions that operate on the global tracer.
// These are useful for simple cases where you don't need the interface.

// StartRunSpan starts a span for the entire graph run.
// Uses the global OTel tracer.
func StartRunSpan(ctx context.Context, graphName, runID string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "flowgraph.run",
		trace.WithAttributes(
			attribute.String("graph.name", graphName),
			attribute.String("run.id", runID),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// StartNodeSpan starts a span for a node execution.
// Uses the global OTel tracer.
func StartNodeSpan(ctx context.Context, nodeID string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "flowgraph.node."+nodeID,
		trace.WithAttributes(
			attribute.String("node.id", nodeID),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// EndSpanWithError completes a span, optionally recording an error.
func EndSpanWithError(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

// AddSpanEvent adds an event to the current span in context.
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.IsRecording() {
		return
	}
	span.AddEvent(name, trace.WithAttributes(attrs...))
}
