package flowgraph

import (
	"log/slog"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/rmurphy/flowgraph/pkg/flowgraph/observability"
)

// runConfig holds configuration for graph execution.
type runConfig struct {
	maxIterations int

	// Checkpointing
	checkpointStore        checkpoint.Store
	runID                  string
	checkpointFailureFatal bool
	sequence               int

	// Resume
	stateOverride func(any) any
	validateState func(any) error
	replayNode    bool

	// Observability
	logger         *slog.Logger
	metricsEnabled bool
	tracingEnabled bool
	metrics        observability.MetricsRecorder
	spans          observability.SpanManager
}

// defaultRunConfig returns the default execution configuration.
func defaultRunConfig() runConfig {
	return runConfig{
		maxIterations:          1000,
		checkpointFailureFatal: false,
		sequence:               0,
		// Observability disabled by default (no overhead)
		metrics: observability.NoopMetrics{},
		spans:   observability.NoopSpanManager{},
	}
}

// RunOption configures execution behavior.
type RunOption func(*runConfig)

// MaxIterationsLimit is the maximum allowed value for WithMaxIterations.
// This prevents accidental resource exhaustion from extremely high values.
const MaxIterationsLimit = 100000

// WithMaxIterations sets the maximum number of node executions.
// Default: 1000
//
// This prevents infinite loops from hanging forever. If a graph
// exceeds this limit, Run returns ErrMaxIterations.
//
// Panics if n <= 0 or n > MaxIterationsLimit (100000).
//
// Example:
//
//	result, err := compiled.Run(ctx, state, flowgraph.WithMaxIterations(100))
func WithMaxIterations(n int) RunOption {
	if n <= 0 {
		panic("flowgraph: max iterations must be > 0")
	}
	if n > MaxIterationsLimit {
		panic("flowgraph: max iterations exceeds limit (100000)")
	}
	return func(c *runConfig) {
		c.maxIterations = n
	}
}

// WithCheckpointing enables checkpoint saving during execution.
// Checkpoints are saved after each node completes successfully.
//
// Must be used with WithRunID to identify the run.
//
// Example:
//
//	store := checkpoint.NewMemoryStore()
//	result, err := compiled.Run(ctx, state,
//	    flowgraph.WithCheckpointing(store),
//	    flowgraph.WithRunID("run-123"))
func WithCheckpointing(store checkpoint.Store) RunOption {
	return func(c *runConfig) {
		c.checkpointStore = store
	}
}

// WithRunID sets the run identifier for checkpointing.
// Required when checkpointing is enabled.
//
// Example:
//
//	result, err := compiled.Run(ctx, state,
//	    flowgraph.WithCheckpointing(store),
//	    flowgraph.WithRunID("run-123"))
func WithRunID(id string) RunOption {
	return func(c *runConfig) {
		c.runID = id
	}
}

// WithCheckpointFailureFatal makes checkpoint failures stop execution.
// By default, checkpoint failures are logged but don't stop execution.
//
// Example:
//
//	result, err := compiled.Run(ctx, state,
//	    flowgraph.WithCheckpointing(store),
//	    flowgraph.WithRunID("run-123"),
//	    flowgraph.WithCheckpointFailureFatal(true))
func WithCheckpointFailureFatal(fatal bool) RunOption {
	return func(c *runConfig) {
		c.checkpointFailureFatal = fatal
	}
}

// WithObservabilityLogger sets a logger for execution observability.
// When set, flowgraph logs node executions, completions, errors, and checkpoints.
//
// This is separate from the context logger - the context logger is for
// your node code to use. This logger is for flowgraph's internal logging.
//
// Example:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	result, err := compiled.Run(ctx, state,
//	    flowgraph.WithObservabilityLogger(logger),
//	    flowgraph.WithRunID("run-123"))
func WithObservabilityLogger(logger *slog.Logger) RunOption {
	return func(c *runConfig) {
		c.logger = logger
	}
}

// WithMetrics enables OpenTelemetry metrics collection.
// When enabled, flowgraph records metrics for node executions, latency,
// errors, and checkpoint sizes.
//
// Requires a global OTel meter provider to be configured:
//
//	import "go.opentelemetry.io/otel"
//	otel.SetMeterProvider(yourProvider)
//
// Example:
//
//	result, err := compiled.Run(ctx, state,
//	    flowgraph.WithMetrics(true))
//
// Metrics emitted:
//   - flowgraph.node.executions{node_id="..."}
//   - flowgraph.node.latency_ms{node_id="..."}
//   - flowgraph.node.errors{node_id="..."}
//   - flowgraph.graph.runs{success="true|false"}
//   - flowgraph.checkpoint.size_bytes{node_id="..."}
func WithMetrics(enabled bool) RunOption {
	return func(c *runConfig) {
		c.metricsEnabled = enabled
		if enabled {
			c.metrics = observability.NewMetricsRecorder()
		} else {
			c.metrics = observability.NoopMetrics{}
		}
	}
}

// WithTracing enables OpenTelemetry distributed tracing.
// When enabled, flowgraph creates spans for graph runs and node executions.
//
// Requires a global OTel tracer provider to be configured:
//
//	import "go.opentelemetry.io/otel"
//	otel.SetTracerProvider(yourProvider)
//
// Example:
//
//	result, err := compiled.Run(ctx, state,
//	    flowgraph.WithTracing(true))
//
// Spans created:
//
//	flowgraph.run (parent span)
//	  ├── flowgraph.node.a
//	  ├── flowgraph.node.b
//	  └── flowgraph.node.c
func WithTracing(enabled bool) RunOption {
	return func(c *runConfig) {
		c.tracingEnabled = enabled
		if enabled {
			c.spans = observability.NewSpanManager()
		} else {
			c.spans = observability.NoopSpanManager{}
		}
	}
}

// resumeConfig holds configuration for resume operations.
type resumeConfig struct {
	stateOverride func(any) any
	validateState func(any) error
	replayNode    bool
}

// ResumeOption configures resume behavior.
type ResumeOption func(*resumeConfig)

// WithStateOverride allows modifying the loaded state before resuming.
// Use this to fix data issues or update external references.
//
// Example:
//
//	result, err := compiled.Resume(ctx, store, runID,
//	    flowgraph.WithStateOverride(func(s any) any {
//	        state := s.(MyState)
//	        state.FixedField = "corrected"
//	        return state
//	    }))
func WithStateOverride(fn func(any) any) ResumeOption {
	return func(c *resumeConfig) {
		c.stateOverride = fn
	}
}

// WithStateValidation validates the loaded state before resuming.
// If validation fails, Resume returns the error without executing.
//
// Example:
//
//	result, err := compiled.Resume(ctx, store, runID,
//	    flowgraph.WithStateValidation(func(s any) error {
//	        state := s.(MyState)
//	        if state.Expired() {
//	            return errors.New("state expired")
//	        }
//	        return nil
//	    }))
func WithStateValidation(fn func(any) error) ResumeOption {
	return func(c *resumeConfig) {
		c.validateState = fn
	}
}

// WithReplayNode causes the resume to re-execute the checkpointed node.
// By default, resume starts from the node AFTER the checkpoint.
// Use this when the checkpointed node is idempotent and you want to retry it.
//
// Example:
//
//	// Node C crashed mid-execution, but it's idempotent (e.g., LLM call)
//	result, err := compiled.Resume(ctx, store, runID,
//	    flowgraph.WithReplayNode())
func WithReplayNode() ResumeOption {
	return func(c *resumeConfig) {
		c.replayNode = true
	}
}
