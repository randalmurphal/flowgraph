package flowgraph

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/checkpoint"
)

// Run executes the graph with the given initial state.
// Returns the final state and any error encountered.
//
// On success, returns the state after the last node executed before END.
// On error, returns the state at the point of failure (useful for debugging).
//
// Execution flow:
//  1. Start at the entry point node
//  2. Check for cancellation
//  3. Execute the current node
//  4. Determine the next node (via simple or conditional edge)
//  5. Repeat until END is reached or an error occurs
//
// Example:
//
//	ctx := flowgraph.NewContext(context.Background())
//	result, err := compiled.Run(ctx, initialState)
//	if err != nil {
//	    // result contains state at point of failure
//	}
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error) {
	if ctx == nil {
		return state, ErrNilContext
	}

	cfg := defaultRunConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	// Validate checkpointing configuration
	if cfg.checkpointStore != nil && cfg.runID == "" {
		return state, ErrRunIDRequired
	}

	return cg.runFrom(ctx, state, cg.entryPoint, &cfg)
}

// runFrom executes the graph starting from a specific node.
// This is used both by Run() and Resume().
func (cg *CompiledGraph[S]) runFrom(ctx Context, state S, startNode string, cfg *runConfig) (S, error) {
	current := startNode
	iterations := 0
	prevNode := ""

	for current != END {
		iterations++
		if iterations > cfg.maxIterations {
			return state, &MaxIterationsError{
				Max:        cfg.maxIterations,
				LastNodeID: current,
				State:      state,
			}
		}

		// Check for cancellation before executing node
		select {
		case <-ctx.Done():
			return state, &CancellationError{
				NodeID:       current,
				State:        state,
				Cause:        ctx.Err(),
				WasExecuting: false,
			}
		default:
		}

		// Execute the node
		var err error
		state, err = cg.executeNode(ctx, current, state)
		if err != nil {
			return state, err
		}

		// Determine next node
		next, err := cg.nextNode(ctx, state, current)
		if err != nil {
			return state, err
		}

		// Checkpoint after successful node execution
		if cfg.checkpointStore != nil {
			if err := cg.saveCheckpoint(ctx, cfg, current, prevNode, state, next); err != nil {
				return state, err
			}
		}

		prevNode = current
		current = next
	}

	return state, nil
}

// saveCheckpoint persists the current state after node execution.
func (cg *CompiledGraph[S]) saveCheckpoint(ctx Context, cfg *runConfig, nodeID, prevNodeID string, state S, nextNode string) error {
	// Serialize state
	stateBytes, err := json.Marshal(state)
	if err != nil {
		if cfg.checkpointFailureFatal {
			return &CheckpointError{
				NodeID: nodeID,
				Op:     "serialize",
				Err:    err,
			}
		}
		ctx.Logger().Warn("checkpoint serialization failed",
			"node", nodeID, "error", err)
		return nil
	}

	// Create checkpoint
	cfg.sequence++
	cp := checkpoint.New(cfg.runID, nodeID, cfg.sequence, stateBytes, nextNode).
		WithPrevNode(prevNodeID)

	if ec, ok := ctx.(*executionContext); ok {
		cp = cp.WithAttempt(ec.attempt)
	}

	data, err := cp.Marshal()
	if err != nil {
		if cfg.checkpointFailureFatal {
			return &CheckpointError{
				NodeID: nodeID,
				Op:     "marshal",
				Err:    err,
			}
		}
		ctx.Logger().Warn("checkpoint marshal failed",
			"node", nodeID, "error", err)
		return nil
	}

	// Save to store
	if err := cfg.checkpointStore.Save(cfg.runID, nodeID, data); err != nil {
		if cfg.checkpointFailureFatal {
			return &CheckpointError{
				NodeID: nodeID,
				Op:     "save",
				Err:    err,
			}
		}
		ctx.Logger().Warn("checkpoint save failed",
			"node", nodeID, "error", err)
	}

	return nil
}

// executeNode executes a single node with panic recovery.
// Returns the new state and any error (including wrapped panics).
func (cg *CompiledGraph[S]) executeNode(ctx Context, nodeID string, state S) (result S, err error) {
	fn, exists := cg.getNode(nodeID)
	if !exists {
		// This shouldn't happen if compilation was successful
		return state, &NodeError{
			NodeID: nodeID,
			Op:     "lookup",
			Err:    fmt.Errorf("node not found: %s", nodeID),
		}
	}

	// Create node-specific context with enriched logger
	nodeCtx := ctx
	if ec, ok := ctx.(*executionContext); ok {
		nodeCtx = ec.withNodeID(nodeID)
	}

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			result = state
			err = &PanicError{
				NodeID: nodeID,
				Value:  r,
				Stack:  string(debug.Stack()),
			}
		}
	}()

	result, err = fn(nodeCtx, state)
	if err != nil {
		return result, &NodeError{
			NodeID: nodeID,
			Op:     "execute",
			Err:    err,
		}
	}

	return result, nil
}

// nextNode determines the next node to execute.
// Checks conditional edges first, then simple edges.
func (cg *CompiledGraph[S]) nextNode(ctx Context, state S, current string) (string, error) {
	// Check for conditional edge first
	if router, exists := cg.getRouter(current); exists {
		// Create node-specific context for the router
		routerCtx := ctx
		if ec, ok := ctx.(*executionContext); ok {
			routerCtx = ec.withNodeID(current)
		}

		next := router(routerCtx, state)

		// Validate router result
		if next == "" {
			return "", &RouterError{
				FromNode: current,
				Returned: next,
				Err:      ErrInvalidRouterResult,
			}
		}

		if next != END {
			if _, exists := cg.getNode(next); !exists {
				return "", &RouterError{
					FromNode: current,
					Returned: next,
					Err:      ErrRouterTargetNotFound,
				}
			}
		}

		return next, nil
	}

	// Use simple edges
	edges := cg.getEdges(current)
	if len(edges) == 0 {
		// No outgoing edges - this shouldn't happen if compilation was successful
		return "", &NodeError{
			NodeID: current,
			Op:     "routing",
			Err:    fmt.Errorf("no outgoing edge from node %s", current),
		}
	}

	// For simple edges, take the first one
	// (Multiple simple edges from one node isn't really supported in Phase 1)
	return edges[0], nil
}
