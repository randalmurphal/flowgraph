package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestNoopMetrics_ImplementsInterface(t *testing.T) {
	var _ MetricsRecorder = NoopMetrics{}
}

func TestNoopMetrics_RecordNodeExecution(t *testing.T) {
	m := NoopMetrics{}

	t.Run("does not panic with valid args", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordNodeExecution(context.Background(), "node", 100*time.Millisecond, nil)
		})
	})

	t.Run("does not panic with error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordNodeExecution(context.Background(), "node", 100*time.Millisecond, errors.New("test"))
		})
	})

	t.Run("does not panic with nil context", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordNodeExecution(nil, "node", 0, nil)
		})
	})

	t.Run("does not panic with empty node ID", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordNodeExecution(context.Background(), "", 0, nil)
		})
	})
}

func TestNoopMetrics_RecordGraphRun(t *testing.T) {
	m := NoopMetrics{}

	t.Run("does not panic with success=true", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordGraphRun(context.Background(), true, 500*time.Millisecond)
		})
	})

	t.Run("does not panic with success=false", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordGraphRun(context.Background(), false, 100*time.Millisecond)
		})
	})

	t.Run("does not panic with nil context", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordGraphRun(nil, true, 0)
		})
	})
}

func TestNoopMetrics_RecordCheckpoint(t *testing.T) {
	m := NoopMetrics{}

	t.Run("does not panic with valid args", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordCheckpoint(context.Background(), "node", 1024)
		})
	})

	t.Run("does not panic with zero size", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordCheckpoint(context.Background(), "node", 0)
		})
	})

	t.Run("does not panic with negative size", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordCheckpoint(context.Background(), "node", -1)
		})
	})

	t.Run("does not panic with nil context", func(t *testing.T) {
		assert.NotPanics(t, func() {
			m.RecordCheckpoint(nil, "node", 1024)
		})
	})
}

func TestNoopSpanManager_ImplementsInterface(t *testing.T) {
	var _ SpanManager = NoopSpanManager{}
}

func TestNoopSpanManager_StartRunSpan(t *testing.T) {
	sm := NoopSpanManager{}

	t.Run("returns same context", func(t *testing.T) {
		ctx := context.Background()
		newCtx, span := sm.StartRunSpan(ctx, "graph", "run-1")

		assert.Equal(t, ctx, newCtx, "Context should be unchanged")
		assert.NotNil(t, span, "Span should not be nil")
	})

	t.Run("span is valid noop span", func(t *testing.T) {
		ctx := context.Background()
		_, span := sm.StartRunSpan(ctx, "graph", "run-1")

		// Noop spans are not recording
		assert.False(t, span.IsRecording())
	})

	t.Run("does not panic with empty args", func(t *testing.T) {
		assert.NotPanics(t, func() {
			sm.StartRunSpan(context.Background(), "", "")
		})
	})
}

func TestNoopSpanManager_StartNodeSpan(t *testing.T) {
	sm := NoopSpanManager{}

	t.Run("returns same context", func(t *testing.T) {
		ctx := context.Background()
		newCtx, span := sm.StartNodeSpan(ctx, "process")

		assert.Equal(t, ctx, newCtx, "Context should be unchanged")
		assert.NotNil(t, span, "Span should not be nil")
	})

	t.Run("span is valid noop span", func(t *testing.T) {
		ctx := context.Background()
		_, span := sm.StartNodeSpan(ctx, "process")

		assert.False(t, span.IsRecording())
	})

	t.Run("does not panic with empty node ID", func(t *testing.T) {
		assert.NotPanics(t, func() {
			sm.StartNodeSpan(context.Background(), "")
		})
	})
}

func TestNoopSpanManager_EndSpanWithError(t *testing.T) {
	sm := NoopSpanManager{}

	t.Run("does not panic with nil span", func(t *testing.T) {
		assert.NotPanics(t, func() {
			sm.EndSpanWithError(nil, nil)
		})
	})

	t.Run("does not panic with nil error", func(t *testing.T) {
		_, span := sm.StartRunSpan(context.Background(), "g", "r")
		assert.NotPanics(t, func() {
			sm.EndSpanWithError(span, nil)
		})
	})

	t.Run("does not panic with error", func(t *testing.T) {
		_, span := sm.StartRunSpan(context.Background(), "g", "r")
		assert.NotPanics(t, func() {
			sm.EndSpanWithError(span, errors.New("test error"))
		})
	})
}

func TestNoopSpanManager_AddSpanEvent(t *testing.T) {
	sm := NoopSpanManager{}

	t.Run("does not panic with valid args", func(t *testing.T) {
		ctx := context.Background()
		assert.NotPanics(t, func() {
			sm.AddSpanEvent(ctx, "test_event", attribute.String("key", "value"))
		})
	})

	t.Run("does not panic with no attributes", func(t *testing.T) {
		ctx := context.Background()
		assert.NotPanics(t, func() {
			sm.AddSpanEvent(ctx, "test_event")
		})
	})

	t.Run("does not panic with nil context", func(t *testing.T) {
		assert.NotPanics(t, func() {
			sm.AddSpanEvent(nil, "test_event")
		})
	})

	t.Run("does not panic with empty event name", func(t *testing.T) {
		assert.NotPanics(t, func() {
			sm.AddSpanEvent(context.Background(), "")
		})
	})
}

func TestNoopImplementations_NoSideEffects(t *testing.T) {
	// This test verifies that noop implementations can be used
	// in a realistic scenario without any side effects

	metrics := NoopMetrics{}
	spans := NoopSpanManager{}

	ctx := context.Background()

	// Simulate a graph run
	ctx, runSpan := spans.StartRunSpan(ctx, "test-graph", "run-123")

	// Simulate node executions
	for i, nodeID := range []string{"fetch", "process", "save"} {
		ctx, nodeSpan := spans.StartNodeSpan(ctx, nodeID)

		start := time.Now()
		// Simulate work
		time.Sleep(1 * time.Millisecond)
		duration := time.Since(start)

		var err error
		if i == 1 {
			err = errors.New("simulated error")
		}

		metrics.RecordNodeExecution(ctx, nodeID, duration, err)

		if i == 2 {
			metrics.RecordCheckpoint(ctx, nodeID, 512)
			spans.AddSpanEvent(ctx, "checkpoint_saved", attribute.Int64("size", 512))
		}

		spans.EndSpanWithError(nodeSpan, err)
	}

	metrics.RecordGraphRun(ctx, true, 100*time.Millisecond)
	spans.EndSpanWithError(runSpan, nil)

	// If we get here without panicking, the test passes
}
