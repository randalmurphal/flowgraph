package flowgraph

import "github.com/rmurphy/flowgraph/pkg/flowgraph/checkpoint"

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
}

// defaultRunConfig returns the default execution configuration.
func defaultRunConfig() runConfig {
	return runConfig{
		maxIterations:          1000,
		checkpointFailureFatal: false,
		sequence:               0,
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
