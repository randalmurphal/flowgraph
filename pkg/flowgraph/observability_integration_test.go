package flowgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogHandler captures log records for testing.
type testLogHandler struct {
	buf   *bytes.Buffer
	level slog.Level
}

func newTestLogHandler() *testLogHandler {
	return &testLogHandler{
		buf:   &bytes.Buffer{},
		level: slog.LevelDebug,
	}
}

func (h *testLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	data := map[string]any{
		"level": r.Level.String(),
		"msg":   r.Message,
	}
	r.Attrs(func(a slog.Attr) bool {
		data[a.Key] = a.Value.Any()
		return true
	})
	enc := json.NewEncoder(h.buf)
	return enc.Encode(data)
}

func (h *testLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *testLogHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *testLogHandler) getRecords() []map[string]any {
	var records []map[string]any
	lines := bytes.Split(h.buf.Bytes(), []byte("\n"))
	for _, line := range lines {
		if len(line) > 0 {
			var m map[string]any
			if err := json.Unmarshal(line, &m); err == nil {
				records = append(records, m)
			}
		}
	}
	return records
}

func TestRun_WithObservabilityLogger(t *testing.T) {
	h := newTestLogHandler()
	logger := slog.New(h)

	graph := NewGraph[Counter]().
		AddNode("inc1", increment).
		AddNode("inc2", increment).
		AddEdge("inc1", "inc2").
		AddEdge("inc2", END).
		SetEntry("inc1")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := NewContext(context.Background(), WithContextRunID("test-run-123"))
	result, err := compiled.Run(ctx, Counter{Value: 0},
		WithObservabilityLogger(logger))

	require.NoError(t, err)
	assert.Equal(t, 2, result.Value)

	// Check log records
	records := h.getRecords()
	require.NotEmpty(t, records, "Expected log records")

	// Should have: run start, node1 start/complete, node2 start/complete, run complete
	var foundRunStart, foundRunComplete bool
	var nodeStarts, nodeCompletes int

	for _, r := range records {
		msg, _ := r["msg"].(string)
		switch msg {
		case "graph run starting":
			foundRunStart = true
			assert.Equal(t, "test-run-123", r["run_id"])
		case "graph run completed":
			foundRunComplete = true
			assert.Equal(t, "test-run-123", r["run_id"])
		case "node starting":
			nodeStarts++
		case "node completed":
			nodeCompletes++
		}
	}

	assert.True(t, foundRunStart, "Expected 'graph run starting' log")
	assert.True(t, foundRunComplete, "Expected 'graph run completed' log")
	assert.Equal(t, 2, nodeStarts, "Expected 2 'node starting' logs")
	assert.Equal(t, 2, nodeCompletes, "Expected 2 'node completed' logs")
}

func TestRun_WithObservabilityLogger_Error(t *testing.T) {
	h := newTestLogHandler()
	logger := slog.New(h)

	errBoom := errors.New("boom")
	failingNode := func(ctx Context, s Counter) (Counter, error) {
		return s, errBoom
	}

	graph := NewGraph[Counter]().
		AddNode("ok", increment).
		AddNode("fail", failingNode).
		AddEdge("ok", "fail").
		AddEdge("fail", END).
		SetEntry("ok")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := NewContext(context.Background(), WithContextRunID("error-run"))
	_, err = compiled.Run(ctx, Counter{Value: 0},
		WithObservabilityLogger(logger))

	require.Error(t, err)

	// Check log records
	records := h.getRecords()

	var foundNodeError, foundRunError bool
	for _, r := range records {
		msg, _ := r["msg"].(string)
		switch msg {
		case "node failed":
			foundNodeError = true
			assert.Equal(t, "fail", r["node_id"])
		case "graph run failed":
			foundRunError = true
			assert.Equal(t, "error-run", r["run_id"])
		}
	}

	assert.True(t, foundNodeError, "Expected 'node failed' log")
	assert.True(t, foundRunError, "Expected 'graph run failed' log")
}

func TestRun_WithMetrics_Disabled(t *testing.T) {
	// Metrics disabled by default - should not panic
	graph := NewGraph[Counter]().
		AddNode("inc", increment).
		AddEdge("inc", END).
		SetEntry("inc")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), Counter{Value: 0})

	require.NoError(t, err)
	assert.Equal(t, 1, result.Value)
}

func TestRun_WithMetrics_Enabled(t *testing.T) {
	// Enable metrics - should not panic even without provider
	graph := NewGraph[Counter]().
		AddNode("inc", increment).
		AddEdge("inc", END).
		SetEntry("inc")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), Counter{Value: 0},
		WithMetrics(true))

	require.NoError(t, err)
	assert.Equal(t, 1, result.Value)
}

func TestRun_WithTracing_Disabled(t *testing.T) {
	// Tracing disabled by default - should not panic
	graph := NewGraph[Counter]().
		AddNode("inc", increment).
		AddEdge("inc", END).
		SetEntry("inc")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), Counter{Value: 0})

	require.NoError(t, err)
	assert.Equal(t, 1, result.Value)
}

func TestRun_WithTracing_Enabled(t *testing.T) {
	// Enable tracing - should not panic even without provider
	graph := NewGraph[Counter]().
		AddNode("inc", increment).
		AddEdge("inc", END).
		SetEntry("inc")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(testCtx(), Counter{Value: 0},
		WithTracing(true))

	require.NoError(t, err)
	assert.Equal(t, 1, result.Value)
}

func TestRun_WithAllObservability(t *testing.T) {
	h := newTestLogHandler()
	logger := slog.New(h)

	graph := NewGraph[Counter]().
		AddNode("inc1", increment).
		AddNode("inc2", increment).
		AddEdge("inc1", "inc2").
		AddEdge("inc2", END).
		SetEntry("inc1")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := NewContext(context.Background(), WithContextRunID("full-obs-run"))
	result, err := compiled.Run(ctx, Counter{Value: 0},
		WithObservabilityLogger(logger),
		WithMetrics(true),
		WithTracing(true))

	require.NoError(t, err)
	assert.Equal(t, 2, result.Value)

	// Verify logs were captured
	records := h.getRecords()
	assert.NotEmpty(t, records)
}

func TestRun_ObservabilityOptions_AreApplied(t *testing.T) {
	// Test that options actually set the config values
	t.Run("WithMetrics sets metricsEnabled", func(t *testing.T) {
		cfg := defaultRunConfig()
		opt := WithMetrics(true)
		opt(&cfg)
		assert.True(t, cfg.metricsEnabled)
		assert.NotNil(t, cfg.metrics)
	})

	t.Run("WithMetrics false sets noop", func(t *testing.T) {
		cfg := defaultRunConfig()
		opt := WithMetrics(false)
		opt(&cfg)
		assert.False(t, cfg.metricsEnabled)
	})

	t.Run("WithTracing sets tracingEnabled", func(t *testing.T) {
		cfg := defaultRunConfig()
		opt := WithTracing(true)
		opt(&cfg)
		assert.True(t, cfg.tracingEnabled)
		assert.NotNil(t, cfg.spans)
	})

	t.Run("WithTracing false sets noop", func(t *testing.T) {
		cfg := defaultRunConfig()
		opt := WithTracing(false)
		opt(&cfg)
		assert.False(t, cfg.tracingEnabled)
	})

	t.Run("WithObservabilityLogger sets logger", func(t *testing.T) {
		cfg := defaultRunConfig()
		logger := slog.Default()
		opt := WithObservabilityLogger(logger)
		opt(&cfg)
		assert.Equal(t, logger, cfg.logger)
	})
}
