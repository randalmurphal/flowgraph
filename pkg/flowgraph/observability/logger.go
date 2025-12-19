// Package observability provides production-grade observability features
// for flowgraph: structured logging, metrics, and distributed tracing.
//
// Features:
//   - Structured logging via slog (Go stdlib)
//   - Metrics via OpenTelemetry
//   - Tracing via OpenTelemetry
//
// All features are opt-in and have no-op implementations when disabled.
package observability

import (
	"log/slog"
	"time"
)

// EnrichLogger adds flowgraph context to a logger.
// Returns a new logger with run_id, node_id, and attempt fields.
//
// Example:
//
//	enriched := EnrichLogger(logger, "run-123", "process", 1)
//	enriched.Info("doing work") // includes run_id, node_id, attempt
func EnrichLogger(logger *slog.Logger, runID, nodeID string, attempt int) *slog.Logger {
	if logger == nil {
		return nil
	}
	return logger.With(
		slog.String("run_id", runID),
		slog.String("node_id", nodeID),
		slog.Int("attempt", attempt),
	)
}

// LogRunStart logs the start of a graph run.
func LogRunStart(logger *slog.Logger, runID string) {
	if logger == nil {
		return
	}
	logger.Info("graph run starting",
		slog.String("run_id", runID),
	)
}

// LogRunComplete logs successful graph run completion.
func LogRunComplete(logger *slog.Logger, runID string, durationMs float64, nodeCount int) {
	if logger == nil {
		return
	}
	logger.Info("graph run completed",
		slog.String("run_id", runID),
		slog.Float64("duration_ms", durationMs),
		slog.Int("nodes_executed", nodeCount),
	)
}

// LogRunError logs graph run failure.
func LogRunError(logger *slog.Logger, runID string, err error, durationMs float64, lastNode string) {
	if logger == nil {
		return
	}
	logger.Error("graph run failed",
		slog.String("run_id", runID),
		slog.String("error", err.Error()),
		slog.Float64("duration_ms", durationMs),
		slog.String("last_node", lastNode),
	)
}

// LogNodeStart logs node execution start.
func LogNodeStart(logger *slog.Logger, nodeID string) {
	if logger == nil {
		return
	}
	logger.Debug("node starting",
		slog.String("node_id", nodeID),
	)
}

// LogNodeComplete logs successful node completion.
func LogNodeComplete(logger *slog.Logger, nodeID string, durationMs float64) {
	if logger == nil {
		return
	}
	logger.Debug("node completed",
		slog.String("node_id", nodeID),
		slog.Float64("duration_ms", durationMs),
	)
}

// LogNodeError logs node execution error.
func LogNodeError(logger *slog.Logger, nodeID string, err error) {
	if logger == nil {
		return
	}
	logger.Error("node failed",
		slog.String("node_id", nodeID),
		slog.String("error", err.Error()),
	)
}

// LogCheckpoint logs checkpoint creation.
func LogCheckpoint(logger *slog.Logger, nodeID string, sizeBytes int) {
	if logger == nil {
		return
	}
	logger.Debug("checkpoint saved",
		slog.String("node_id", nodeID),
		slog.Int("size_bytes", sizeBytes),
	)
}

// LogCheckpointError logs checkpoint failure (non-fatal).
func LogCheckpointError(logger *slog.Logger, nodeID string, op string, err error) {
	if logger == nil {
		return
	}
	logger.Warn("checkpoint failed",
		slog.String("node_id", nodeID),
		slog.String("operation", op),
		slog.String("error", err.Error()),
	)
}

// TimedOperation measures the duration of an operation.
// Returns a function that, when called, returns the elapsed time in milliseconds.
//
// Example:
//
//	done := TimedOperation()
//	// ... do work ...
//	durationMs := done()
func TimedOperation() func() float64 {
	start := time.Now()
	return func() float64 {
		return float64(time.Since(start).Milliseconds())
	}
}
