package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// setupMetricsTest creates a test meter provider and returns a function to collect metrics.
func setupMetricsTest(t *testing.T) (*sdkmetric.ManualReader, func()) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	// Save the original provider
	originalProvider := otel.GetMeterProvider()

	// Set test provider
	otel.SetMeterProvider(provider)

	// Return cleanup function
	cleanup := func() {
		otel.SetMeterProvider(originalProvider)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Error shutting down meter provider: %v", err)
		}
	}

	return reader, cleanup
}

// collectMetrics collects all metrics from the reader.
func collectMetrics(t *testing.T, reader *sdkmetric.ManualReader) *metricdata.ResourceMetrics {
	var rm metricdata.ResourceMetrics
	err := reader.Collect(context.Background(), &rm)
	require.NoError(t, err)
	return &rm
}

// findMetric finds a metric by name in the collected data.
func findMetric(rm *metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}

func TestNewMetricsRecorder(t *testing.T) {
	_, cleanup := setupMetricsTest(t)
	defer cleanup()

	// NewMetricsRecorder uses the global provider
	recorder := NewMetricsRecorder()
	require.NotNil(t, recorder)

	// Should not be a noop (since we set up a real provider)
	_, isNoop := recorder.(NoopMetrics)
	assert.False(t, isNoop, "Expected real metrics recorder, got noop")
}

func TestRecordNodeExecution(t *testing.T) {
	reader, cleanup := setupMetricsTest(t)
	defer cleanup()

	// Create a fresh metrics instance using the test provider
	m, err := newOtelMetrics()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("records execution count", func(t *testing.T) {
		m.RecordNodeExecution(ctx, "process", 50*time.Millisecond, nil)

		rm := collectMetrics(t, reader)
		metric := findMetric(rm, "flowgraph.node.executions")
		require.NotNil(t, metric)

		sum, ok := metric.Data.(metricdata.Sum[int64])
		require.True(t, ok, "Expected Sum type")
		require.NotEmpty(t, sum.DataPoints)

		// Find the datapoint for our node
		found := false
		for _, dp := range sum.DataPoints {
			for _, attr := range dp.Attributes.ToSlice() {
				if attr.Key == "node_id" && attr.Value.AsString() == "process" {
					found = true
					assert.GreaterOrEqual(t, dp.Value, int64(1))
				}
			}
		}
		assert.True(t, found, "Expected to find datapoint for node_id=process")
	})

	t.Run("records latency", func(t *testing.T) {
		m.RecordNodeExecution(ctx, "transform", 100*time.Millisecond, nil)

		rm := collectMetrics(t, reader)
		metric := findMetric(rm, "flowgraph.node.latency_ms")
		require.NotNil(t, metric)

		hist, ok := metric.Data.(metricdata.Histogram[float64])
		require.True(t, ok, "Expected Histogram type")
		require.NotEmpty(t, hist.DataPoints)
	})

	t.Run("records errors when present", func(t *testing.T) {
		testErr := errors.New("node failed")
		m.RecordNodeExecution(ctx, "failing", 10*time.Millisecond, testErr)

		rm := collectMetrics(t, reader)
		metric := findMetric(rm, "flowgraph.node.errors")
		require.NotNil(t, metric)

		sum, ok := metric.Data.(metricdata.Sum[int64])
		require.True(t, ok, "Expected Sum type")
		require.NotEmpty(t, sum.DataPoints)

		// Find the datapoint for our node
		found := false
		for _, dp := range sum.DataPoints {
			for _, attr := range dp.Attributes.ToSlice() {
				if attr.Key == "node_id" && attr.Value.AsString() == "failing" {
					found = true
					assert.GreaterOrEqual(t, dp.Value, int64(1))
				}
			}
		}
		assert.True(t, found, "Expected to find error datapoint")
	})

	t.Run("does not record error when nil", func(t *testing.T) {
		// Record success for a unique node
		m.RecordNodeExecution(ctx, "success_only", 10*time.Millisecond, nil)

		rm := collectMetrics(t, reader)
		metric := findMetric(rm, "flowgraph.node.errors")
		if metric != nil {
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if ok {
				// Check that success_only has no error recorded
				for _, dp := range sum.DataPoints {
					for _, attr := range dp.Attributes.ToSlice() {
						if attr.Key == "node_id" && attr.Value.AsString() == "success_only" {
							// If found, value should be 0
							assert.Equal(t, int64(0), dp.Value, "Expected no errors for success_only node")
						}
					}
				}
			}
		}
		// If metric is nil, that's fine - no errors recorded
	})
}

func TestRecordGraphRun(t *testing.T) {
	reader, cleanup := setupMetricsTest(t)
	defer cleanup()

	m, err := newOtelMetrics()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("records successful runs", func(t *testing.T) {
		m.RecordGraphRun(ctx, true, 500*time.Millisecond)

		rm := collectMetrics(t, reader)
		metric := findMetric(rm, "flowgraph.graph.runs")
		require.NotNil(t, metric)

		sum, ok := metric.Data.(metricdata.Sum[int64])
		require.True(t, ok)
		require.NotEmpty(t, sum.DataPoints)
	})

	t.Run("records failed runs", func(t *testing.T) {
		m.RecordGraphRun(ctx, false, 100*time.Millisecond)

		rm := collectMetrics(t, reader)
		metric := findMetric(rm, "flowgraph.graph.runs")
		require.NotNil(t, metric)
	})

	t.Run("records graph latency", func(t *testing.T) {
		m.RecordGraphRun(ctx, true, 200*time.Millisecond)

		rm := collectMetrics(t, reader)
		metric := findMetric(rm, "flowgraph.graph.latency_ms")
		require.NotNil(t, metric)

		hist, ok := metric.Data.(metricdata.Histogram[float64])
		require.True(t, ok, "Expected Histogram type")
		require.NotEmpty(t, hist.DataPoints)
	})
}

func TestRecordCheckpoint(t *testing.T) {
	reader, cleanup := setupMetricsTest(t)
	defer cleanup()

	m, err := newOtelMetrics()
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("records checkpoint size", func(t *testing.T) {
		m.RecordCheckpoint(ctx, "save_node", 2048)

		rm := collectMetrics(t, reader)
		metric := findMetric(rm, "flowgraph.checkpoint.size_bytes")
		require.NotNil(t, metric)

		hist, ok := metric.Data.(metricdata.Histogram[int64])
		require.True(t, ok, "Expected Histogram[int64] type")
		require.NotEmpty(t, hist.DataPoints)

		// Verify attribute
		found := false
		for _, dp := range hist.DataPoints {
			for _, attr := range dp.Attributes.ToSlice() {
				if attr.Key == "node_id" && attr.Value.AsString() == "save_node" {
					found = true
					assert.Greater(t, dp.Count, uint64(0))
				}
			}
		}
		assert.True(t, found, "Expected to find datapoint for save_node")
	})
}

func TestOtelMetrics_AllMethods(t *testing.T) {
	reader, cleanup := setupMetricsTest(t)
	defer cleanup()

	m, err := newOtelMetrics()
	require.NoError(t, err)
	require.NotNil(t, m)

	ctx := context.Background()

	// Call all methods to ensure they work
	m.RecordNodeExecution(ctx, "test_node", 25*time.Millisecond, nil)
	m.RecordNodeExecution(ctx, "error_node", 10*time.Millisecond, errors.New("test"))
	m.RecordGraphRun(ctx, true, 100*time.Millisecond)
	m.RecordGraphRun(ctx, false, 50*time.Millisecond)
	m.RecordCheckpoint(ctx, "cp_node", 1024)

	// Collect and verify all metrics exist
	rm := collectMetrics(t, reader)

	assert.NotNil(t, findMetric(rm, "flowgraph.node.executions"))
	assert.NotNil(t, findMetric(rm, "flowgraph.node.latency_ms"))
	assert.NotNil(t, findMetric(rm, "flowgraph.node.errors"))
	assert.NotNil(t, findMetric(rm, "flowgraph.graph.runs"))
	assert.NotNil(t, findMetric(rm, "flowgraph.graph.latency_ms"))
	assert.NotNil(t, findMetric(rm, "flowgraph.checkpoint.size_bytes"))
}

func TestNewOtelMetrics_Creation(t *testing.T) {
	reader, cleanup := setupMetricsTest(t)
	defer cleanup()

	m, err := newOtelMetrics()
	require.NoError(t, err)
	require.NotNil(t, m)

	// Verify all metric instruments were created
	assert.NotNil(t, m.nodeExecutions)
	assert.NotNil(t, m.nodeLatency)
	assert.NotNil(t, m.nodeErrors)
	assert.NotNil(t, m.graphRuns)
	assert.NotNil(t, m.graphLatency)
	assert.NotNil(t, m.checkpointSize)

	// Use the reader to avoid unused warning
	_ = reader
}
