package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupTracingTest creates a test tracer provider with an in-memory span recorder.
func setupTracingTest(t *testing.T) (*tracetest.InMemoryExporter, func()) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)

	// Save the original provider
	originalProvider := otel.GetTracerProvider()

	// Set test provider
	otel.SetTracerProvider(tp)

	// Update the package-level tracer
	tracer = otel.Tracer("flowgraph")

	cleanup := func() {
		otel.SetTracerProvider(originalProvider)
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Logf("Error shutting down tracer provider: %v", err)
		}
	}

	return exporter, cleanup
}

func TestStartRunSpan(t *testing.T) {
	exporter, cleanup := setupTracingTest(t)
	defer cleanup()

	t.Run("creates span with correct name and attributes", func(t *testing.T) {
		ctx := context.Background()
		ctx, span := StartRunSpan(ctx, "my-graph", "run-123")
		require.NotNil(t, span)

		// End the span to flush it to the exporter
		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		s := spans[0]
		assert.Equal(t, "flowgraph.run", s.Name)

		// Check attributes
		attrs := s.Attributes
		var graphName, runID string
		for _, attr := range attrs {
			switch attr.Key {
			case "graph.name":
				graphName = attr.Value.AsString()
			case "run.id":
				runID = attr.Value.AsString()
			}
		}
		assert.Equal(t, "my-graph", graphName)
		assert.Equal(t, "run-123", runID)
	})

	t.Run("returns context with span", func(t *testing.T) {
		exporter.Reset()

		ctx := context.Background()
		newCtx, span := StartRunSpan(ctx, "test", "run-456")

		// Context should be different
		assert.NotEqual(t, ctx, newCtx)

		span.End()

		// Should still have recorded the span
		spans := exporter.GetSpans()
		require.Len(t, spans, 1)
	})
}

func TestStartNodeSpan(t *testing.T) {
	exporter, cleanup := setupTracingTest(t)
	defer cleanup()

	t.Run("creates span with node name suffix", func(t *testing.T) {
		ctx := context.Background()
		ctx, span := StartNodeSpan(ctx, "process")
		require.NotNil(t, span)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		s := spans[0]
		assert.Equal(t, "flowgraph.node.process", s.Name)

		// Check node.id attribute
		var nodeID string
		for _, attr := range s.Attributes {
			if attr.Key == "node.id" {
				nodeID = attr.Value.AsString()
			}
		}
		assert.Equal(t, "process", nodeID)
	})

	t.Run("child spans have correct parent", func(t *testing.T) {
		exporter.Reset()

		ctx := context.Background()
		ctx, runSpan := StartRunSpan(ctx, "graph", "run-1")

		ctx, nodeSpan := StartNodeSpan(ctx, "node1")
		nodeSpan.End()

		runSpan.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 2)

		// Find node span
		var nodeSpanData *tracetest.SpanStub
		for i := range spans {
			if spans[i].Name == "flowgraph.node.node1" {
				nodeSpanData = &spans[i]
				break
			}
		}
		require.NotNil(t, nodeSpanData)

		// Verify parent-child relationship
		assert.True(t, nodeSpanData.Parent.IsValid())
	})
}

func TestEndSpanWithError(t *testing.T) {
	exporter, cleanup := setupTracingTest(t)
	defer cleanup()

	t.Run("sets OK status for nil error", func(t *testing.T) {
		ctx := context.Background()
		_, span := StartRunSpan(ctx, "test", "run-1")

		EndSpanWithError(span, nil)

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		assert.Equal(t, codes.Ok, spans[0].Status.Code)
		assert.Equal(t, "", spans[0].Status.Description)
	})

	t.Run("sets Error status and records error", func(t *testing.T) {
		exporter.Reset()

		ctx := context.Background()
		_, span := StartRunSpan(ctx, "test", "run-2")
		testErr := errors.New("something went wrong")

		EndSpanWithError(span, testErr)

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		s := spans[0]
		assert.Equal(t, codes.Error, s.Status.Code)
		assert.Equal(t, "something went wrong", s.Status.Description)

		// Check that error was recorded as an event
		require.NotEmpty(t, s.Events)
		found := false
		for _, event := range s.Events {
			if event.Name == "exception" {
				found = true
			}
		}
		assert.True(t, found, "Expected exception event")
	})

	t.Run("nil span does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			EndSpanWithError(nil, nil)
		})
		assert.NotPanics(t, func() {
			EndSpanWithError(nil, errors.New("test"))
		})
	})
}

func TestAddSpanEvent(t *testing.T) {
	exporter, cleanup := setupTracingTest(t)
	defer cleanup()

	t.Run("adds event to current span", func(t *testing.T) {
		ctx := context.Background()
		ctx, span := StartRunSpan(ctx, "test", "run-1")

		AddSpanEvent(ctx, "checkpoint_saved",
			attribute.String("node_id", "process"),
			attribute.Int64("size_bytes", 1024),
		)

		span.End()

		spans := exporter.GetSpans()
		require.Len(t, spans, 1)

		s := spans[0]
		require.NotEmpty(t, s.Events)

		// Find our event
		var found bool
		for _, event := range s.Events {
			if event.Name == "checkpoint_saved" {
				found = true
				// Check attributes
				var nodeID string
				var sizeBytes int64
				for _, attr := range event.Attributes {
					switch attr.Key {
					case "node_id":
						nodeID = attr.Value.AsString()
					case "size_bytes":
						sizeBytes = attr.Value.AsInt64()
					}
				}
				assert.Equal(t, "process", nodeID)
				assert.Equal(t, int64(1024), sizeBytes)
			}
		}
		assert.True(t, found, "Expected to find checkpoint_saved event")
	})

	t.Run("no panic with no current span", func(t *testing.T) {
		ctx := context.Background()
		assert.NotPanics(t, func() {
			AddSpanEvent(ctx, "test_event")
		})
	})
}

func TestSpanManager_Interface(t *testing.T) {
	exporter, cleanup := setupTracingTest(t)
	defer cleanup()

	sm := NewSpanManager()
	require.NotNil(t, sm)

	t.Run("StartRunSpan via interface", func(t *testing.T) {
		ctx := context.Background()
		ctx, span := sm.StartRunSpan(ctx, "interface-graph", "run-if")
		require.NotNil(t, span)

		sm.EndSpanWithError(span, nil)

		spans := exporter.GetSpans()
		require.NotEmpty(t, spans)
	})

	t.Run("StartNodeSpan via interface", func(t *testing.T) {
		exporter.Reset()

		ctx := context.Background()
		ctx, span := sm.StartNodeSpan(ctx, "interface-node")
		require.NotNil(t, span)

		sm.EndSpanWithError(span, nil)

		spans := exporter.GetSpans()
		require.NotEmpty(t, spans)
		assert.Equal(t, "flowgraph.node.interface-node", spans[0].Name)
	})

	t.Run("AddSpanEvent via interface", func(t *testing.T) {
		exporter.Reset()

		ctx := context.Background()
		ctx, span := sm.StartRunSpan(ctx, "test", "run-1")

		sm.AddSpanEvent(ctx, "custom_event", attribute.String("key", "value"))

		sm.EndSpanWithError(span, nil)

		spans := exporter.GetSpans()
		require.NotEmpty(t, spans)
		require.NotEmpty(t, spans[0].Events)
	})
}

func TestOtelSpanManager_EndSpanWithError_Scenarios(t *testing.T) {
	exporter, cleanup := setupTracingTest(t)
	defer cleanup()

	sm := &otelSpanManager{}

	t.Run("wrapped error message is preserved", func(t *testing.T) {
		ctx := context.Background()
		_, span := sm.StartRunSpan(ctx, "test", "run-1")

		wrappedErr := errors.New("wrapped: inner error")
		sm.EndSpanWithError(span, wrappedErr)

		spans := exporter.GetSpans()
		require.NotEmpty(t, spans)
		assert.Contains(t, spans[0].Status.Description, "wrapped: inner error")
	})
}
