package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testHandler captures log records for testing.
type testHandler struct {
	buf    *bytes.Buffer
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

func newTestHandler() *testHandler {
	return &testHandler{
		buf:   &bytes.Buffer{},
		level: slog.LevelDebug,
	}
}

func (h *testHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *testHandler) Handle(_ context.Context, r slog.Record) error {
	// Build a map from the record
	data := map[string]any{
		"level": r.Level.String(),
		"msg":   r.Message,
	}

	// Add pre-configured attrs
	for _, attr := range h.attrs {
		data[attr.Key] = attr.Value.Any()
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		data[a.Key] = a.Value.Any()
		return true
	})

	// Encode as JSON
	enc := json.NewEncoder(h.buf)
	if err := enc.Encode(data); err != nil {
		return err
	}
	return nil
}

func (h *testHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := &testHandler{
		buf:    h.buf,
		level:  h.level,
		attrs:  make([]slog.Attr, len(h.attrs)+len(attrs)),
		groups: h.groups,
	}
	copy(newH.attrs, h.attrs)
	copy(newH.attrs[len(h.attrs):], attrs)
	return newH
}

func (h *testHandler) WithGroup(name string) slog.Handler {
	newH := &testHandler{
		buf:    h.buf,
		level:  h.level,
		attrs:  h.attrs,
		groups: append(h.groups, name),
	}
	return newH
}

func (h *testHandler) getLastRecord() map[string]any {
	lines := bytes.Split(h.buf.Bytes(), []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		if len(lines[i]) > 0 {
			var m map[string]any
			if err := json.Unmarshal(lines[i], &m); err == nil {
				return m
			}
		}
	}
	return nil
}

func (h *testHandler) getAllRecords() []map[string]any {
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

func TestEnrichLogger(t *testing.T) {
	t.Run("adds run_id, node_id, and attempt", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)

		enriched := EnrichLogger(logger, "run-123", "process", 2)
		enriched.Info("test message")

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "run-123", record["run_id"])
		assert.Equal(t, "process", record["node_id"])
		assert.Equal(t, float64(2), record["attempt"]) // JSON decodes ints as float64
		assert.Equal(t, "test message", record["msg"])
	})

	t.Run("nil logger returns nil", func(t *testing.T) {
		enriched := EnrichLogger(nil, "run-123", "process", 1)
		assert.Nil(t, enriched)
	})

	t.Run("empty values are included", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)

		enriched := EnrichLogger(logger, "", "", 0)
		enriched.Info("test")

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "", record["run_id"])
		assert.Equal(t, "", record["node_id"])
		assert.Equal(t, float64(0), record["attempt"])
	})
}

func TestLogRunStart(t *testing.T) {
	t.Run("logs run_id at INFO level", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)

		LogRunStart(logger, "run-456")

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "INFO", record["level"])
		assert.Equal(t, "graph run starting", record["msg"])
		assert.Equal(t, "run-456", record["run_id"])
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			LogRunStart(nil, "run-123")
		})
	})
}

func TestLogRunComplete(t *testing.T) {
	t.Run("logs run completion with metrics", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)

		LogRunComplete(logger, "run-789", 123.5, 5)

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "INFO", record["level"])
		assert.Equal(t, "graph run completed", record["msg"])
		assert.Equal(t, "run-789", record["run_id"])
		assert.Equal(t, 123.5, record["duration_ms"])
		assert.Equal(t, float64(5), record["nodes_executed"])
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			LogRunComplete(nil, "run-123", 100.0, 3)
		})
	})
}

func TestLogRunError(t *testing.T) {
	t.Run("logs run error with context", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)
		testErr := errors.New("connection failed")

		LogRunError(logger, "run-err", testErr, 50.0, "process")

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "ERROR", record["level"])
		assert.Equal(t, "graph run failed", record["msg"])
		assert.Equal(t, "run-err", record["run_id"])
		assert.Equal(t, "connection failed", record["error"])
		assert.Equal(t, 50.0, record["duration_ms"])
		assert.Equal(t, "process", record["last_node"])
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			LogRunError(nil, "run", errors.New("err"), 0, "node")
		})
	})
}

func TestLogNodeStart(t *testing.T) {
	t.Run("logs at DEBUG level", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)

		LogNodeStart(logger, "fetch")

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "DEBUG", record["level"])
		assert.Equal(t, "node starting", record["msg"])
		assert.Equal(t, "fetch", record["node_id"])
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			LogNodeStart(nil, "node")
		})
	})
}

func TestLogNodeComplete(t *testing.T) {
	t.Run("logs completion with duration", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)

		LogNodeComplete(logger, "transform", 45.7)

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "DEBUG", record["level"])
		assert.Equal(t, "node completed", record["msg"])
		assert.Equal(t, "transform", record["node_id"])
		assert.Equal(t, 45.7, record["duration_ms"])
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			LogNodeComplete(nil, "node", 100.0)
		})
	})
}

func TestLogNodeError(t *testing.T) {
	t.Run("logs at ERROR level", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)
		testErr := errors.New("validation failed")

		LogNodeError(logger, "validate", testErr)

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "ERROR", record["level"])
		assert.Equal(t, "node failed", record["msg"])
		assert.Equal(t, "validate", record["node_id"])
		assert.Equal(t, "validation failed", record["error"])
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			LogNodeError(nil, "node", errors.New("err"))
		})
	})
}

func TestLogCheckpoint(t *testing.T) {
	t.Run("logs checkpoint size", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)

		LogCheckpoint(logger, "process", 1024)

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "DEBUG", record["level"])
		assert.Equal(t, "checkpoint saved", record["msg"])
		assert.Equal(t, "process", record["node_id"])
		assert.Equal(t, float64(1024), record["size_bytes"])
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			LogCheckpoint(nil, "node", 100)
		})
	})
}

func TestLogCheckpointError(t *testing.T) {
	t.Run("logs at WARN level", func(t *testing.T) {
		h := newTestHandler()
		logger := slog.New(h)
		testErr := errors.New("disk full")

		LogCheckpointError(logger, "save", "serialize", testErr)

		record := h.getLastRecord()
		require.NotNil(t, record)
		assert.Equal(t, "WARN", record["level"])
		assert.Equal(t, "checkpoint failed", record["msg"])
		assert.Equal(t, "save", record["node_id"])
		assert.Equal(t, "serialize", record["operation"])
		assert.Equal(t, "disk full", record["error"])
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			LogCheckpointError(nil, "node", "op", errors.New("err"))
		})
	})
}

func TestTimedOperation(t *testing.T) {
	t.Run("measures duration", func(t *testing.T) {
		done := TimedOperation()
		time.Sleep(10 * time.Millisecond)
		duration := done()

		// Should be at least 10ms
		assert.GreaterOrEqual(t, duration, 10.0)
		// Should be less than 100ms (reasonable upper bound)
		assert.Less(t, duration, 100.0)
	})

	t.Run("returns zero for immediate call", func(t *testing.T) {
		done := TimedOperation()
		duration := done()

		// Should be very small (less than 1ms)
		assert.Less(t, duration, 1.0)
	})

	t.Run("can be called multiple times", func(t *testing.T) {
		done := TimedOperation()
		time.Sleep(5 * time.Millisecond)
		d1 := done()
		time.Sleep(5 * time.Millisecond)
		d2 := done()

		// Second call should have larger duration
		assert.Greater(t, d2, d1)
	})
}
