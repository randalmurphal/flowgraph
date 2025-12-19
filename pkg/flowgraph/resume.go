package flowgraph

import (
	"encoding/json"
	"fmt"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/checkpoint"
)

// Resume continues execution from the last checkpoint for a run.
// It loads the latest checkpoint and starts execution from the next node.
//
// Example:
//
//	// Previous run crashed after node B
//	// Resume continues from node C with state from B's checkpoint
//	result, err := compiled.Resume(ctx, store, "run-123")
func (cg *CompiledGraph[S]) Resume(ctx Context, store checkpoint.Store, runID string, opts ...ResumeOption) (S, error) {
	var zero S

	if ctx == nil {
		return zero, ErrNilContext
	}

	// Apply resume options
	cfg := resumeConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Find latest checkpoint
	infos, err := store.List(runID)
	if err != nil {
		return zero, fmt.Errorf("list checkpoints: %w", err)
	}
	if len(infos) == 0 {
		return zero, fmt.Errorf("%w: %s", ErrNoCheckpoints, runID)
	}

	// Load the latest checkpoint (last in sequence)
	latest := infos[len(infos)-1]
	data, err := store.Load(runID, latest.NodeID)
	if err != nil {
		return zero, fmt.Errorf("load checkpoint: %w", err)
	}

	// Unmarshal checkpoint
	cp, err := checkpoint.Unmarshal(data)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrDeserializeState, err)
	}

	// Check version compatibility
	if cp.Version != checkpoint.Version {
		return zero, fmt.Errorf("%w: got %d, expected %d",
			ErrCheckpointVersionMismatch, cp.Version, checkpoint.Version)
	}

	// Deserialize state
	var state S
	if err := json.Unmarshal(cp.State, &state); err != nil {
		return zero, fmt.Errorf("%w: %v", ErrDeserializeState, err)
	}

	// Apply state override if configured
	if cfg.stateOverride != nil {
		modified := cfg.stateOverride(state)
		if typed, ok := modified.(S); ok {
			state = typed
		}
	}

	// Validate state if configured
	if cfg.validateState != nil {
		if err := cfg.validateState(state); err != nil {
			return state, fmt.Errorf("state validation failed: %w", err)
		}
	}

	// Determine start node
	startNode := cp.NextNode
	if cfg.replayNode {
		// Re-execute the checkpointed node
		startNode = cp.NodeID
	}

	// Continue execution from determined node
	runCfg := defaultRunConfig()
	runCfg.checkpointStore = store
	runCfg.runID = runID
	runCfg.sequence = cp.Sequence

	return cg.runFrom(ctx, state, startNode, &runCfg)
}

// ResumeFrom continues execution from a specific checkpoint.
// Unlike Resume, this loads the checkpoint at a specific node rather than the latest.
//
// Example:
//
//	// Retry from a specific node
//	result, err := compiled.ResumeFrom(ctx, store, "run-123", "process-node")
func (cg *CompiledGraph[S]) ResumeFrom(ctx Context, store checkpoint.Store, runID, nodeID string, opts ...ResumeOption) (S, error) {
	var zero S

	if ctx == nil {
		return zero, ErrNilContext
	}

	// Apply resume options
	cfg := resumeConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Load checkpoint at specified node
	data, err := store.Load(runID, nodeID)
	if err != nil {
		if err == checkpoint.ErrNotFound {
			return zero, fmt.Errorf("%w: %s at node %s", ErrNoCheckpoints, runID, nodeID)
		}
		return zero, fmt.Errorf("load checkpoint: %w", err)
	}

	// Unmarshal checkpoint
	cp, err := checkpoint.Unmarshal(data)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrDeserializeState, err)
	}

	// Check version compatibility
	if cp.Version != checkpoint.Version {
		return zero, fmt.Errorf("%w: got %d, expected %d",
			ErrCheckpointVersionMismatch, cp.Version, checkpoint.Version)
	}

	// Deserialize state
	var state S
	if err := json.Unmarshal(cp.State, &state); err != nil {
		return zero, fmt.Errorf("%w: %v", ErrDeserializeState, err)
	}

	// Apply state override if configured
	if cfg.stateOverride != nil {
		modified := cfg.stateOverride(state)
		if typed, ok := modified.(S); ok {
			state = typed
		}
	}

	// Validate state if configured
	if cfg.validateState != nil {
		if err := cfg.validateState(state); err != nil {
			return state, fmt.Errorf("state validation failed: %w", err)
		}
	}

	// Determine start node
	startNode := cp.NextNode
	if cfg.replayNode {
		// Re-execute the checkpointed node
		startNode = nodeID
	}

	// Validate start node exists (unless it's END)
	if startNode != END && !cg.HasNode(startNode) {
		return zero, fmt.Errorf("%w: %s", ErrInvalidResumeNode, startNode)
	}

	// Continue execution from determined node
	runCfg := defaultRunConfig()
	runCfg.checkpointStore = store
	runCfg.runID = runID
	runCfg.sequence = cp.Sequence

	return cg.runFrom(ctx, state, startNode, &runCfg)
}
