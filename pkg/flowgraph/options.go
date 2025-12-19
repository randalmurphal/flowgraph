package flowgraph

// runConfig holds configuration for graph execution.
type runConfig struct {
	maxIterations int
	// Additional options will be added in later phases:
	// - checkpointStore for Phase 3
	// - runID for checkpointing
}

// defaultRunConfig returns the default execution configuration.
func defaultRunConfig() runConfig {
	return runConfig{
		maxIterations: 1000,
	}
}

// RunOption configures execution behavior.
type RunOption func(*runConfig)

// WithMaxIterations sets the maximum number of node executions.
// Default: 1000
//
// This prevents infinite loops from hanging forever. If a graph
// exceeds this limit, Run returns ErrMaxIterations.
//
// Example:
//
//	result, err := compiled.Run(ctx, state, flowgraph.WithMaxIterations(100))
func WithMaxIterations(n int) RunOption {
	return func(c *runConfig) {
		if n > 0 {
			c.maxIterations = n
		}
	}
}
