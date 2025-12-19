package flowgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/rmurphy/flowgraph/pkg/flowgraph/observability"
	"go.opentelemetry.io/otel/trace"
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
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (result S, runErr error) {
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

	// Get run ID for observability (from config or context)
	runID := cfg.runID
	if runID == "" {
		runID = ctx.RunID()
	}

	// Start timing
	startTime := time.Now()

	// Log run start
	observability.LogRunStart(cfg.logger, runID)

	// Start run span if tracing enabled
	var execCtx context.Context = ctx
	var runSpan trace.Span
	if cfg.tracingEnabled {
		execCtx, runSpan = cfg.spans.StartRunSpan(ctx, "flowgraph", runID)
		defer func() {
			cfg.spans.EndSpanWithError(runSpan, runErr)
		}()
	}

	// Execute the graph
	var nodeCount int
	result, nodeCount, runErr = cg.runFromWithObservability(execCtx, ctx, state, cg.entryPoint, &cfg)

	// Calculate duration
	duration := time.Since(startTime)
	durationMs := float64(duration.Milliseconds())

	// Record graph run metric
	cfg.metrics.RecordGraphRun(ctx, runErr == nil, duration)

	// Log run completion or error
	if runErr != nil {
		// Get last node from error if available
		lastNode := ""
		if nodeErr, ok := runErr.(*NodeError); ok {
			lastNode = nodeErr.NodeID
		} else if maxErr, ok := runErr.(*MaxIterationsError); ok {
			lastNode = maxErr.LastNodeID
		} else if cancelErr, ok := runErr.(*CancellationError); ok {
			lastNode = cancelErr.NodeID
		}
		observability.LogRunError(cfg.logger, runID, runErr, durationMs, lastNode)
	} else {
		observability.LogRunComplete(cfg.logger, runID, durationMs, nodeCount)
	}

	return result, runErr
}

// runFrom executes the graph starting from a specific node.
// This is used by Resume() - does not include run-level observability.
func (cg *CompiledGraph[S]) runFrom(ctx Context, state S, startNode string, cfg *runConfig) (S, error) {
	result, _, err := cg.runFromWithObservability(ctx, ctx, state, startNode, cfg)
	return result, err
}

// runFromWithObservability executes the graph with full observability.
// tracingCtx carries span context; fgCtx is the flowgraph Context.
// Returns the final state, node count, and any error.
func (cg *CompiledGraph[S]) runFromWithObservability(tracingCtx context.Context, fgCtx Context, state S, startNode string, cfg *runConfig) (S, int, error) {
	current := startNode
	iterations := 0
	prevNode := ""
	nodeCount := 0

	for current != END {
		iterations++
		if iterations > cfg.maxIterations {
			return state, nodeCount, &MaxIterationsError{
				Max:        cfg.maxIterations,
				LastNodeID: current,
				State:      state,
			}
		}

		// Check for cancellation before executing node
		select {
		case <-fgCtx.Done():
			return state, nodeCount, &CancellationError{
				NodeID:       current,
				State:        state,
				Cause:        fgCtx.Err(),
				WasExecuting: false,
			}
		default:
		}

		// Log node start
		observability.LogNodeStart(cfg.logger, current)

		// Start node span if tracing enabled
		nodeTracingCtx := tracingCtx
		var nodeSpan trace.Span
		if cfg.tracingEnabled {
			nodeTracingCtx, nodeSpan = cfg.spans.StartNodeSpan(tracingCtx, current)
		}

		// Time the node execution
		nodeStart := time.Now()

		// Execute the node
		var nodeErr error
		state, nodeErr = cg.executeNode(fgCtx, current, state)

		// Calculate duration
		nodeDuration := time.Since(nodeStart)
		nodeDurationMs := float64(nodeDuration.Milliseconds())

		// Record node metrics
		cfg.metrics.RecordNodeExecution(nodeTracingCtx, current, nodeDuration, nodeErr)

		// End node span with error status
		if cfg.tracingEnabled {
			cfg.spans.EndSpanWithError(nodeSpan, nodeErr)
		}

		// Log node completion or error
		if nodeErr != nil {
			observability.LogNodeError(cfg.logger, current, nodeErr)
			return state, nodeCount, nodeErr
		}
		observability.LogNodeComplete(cfg.logger, current, nodeDurationMs)
		nodeCount++

		// Determine next node
		next, err := cg.nextNode(fgCtx, state, current)
		if err != nil {
			return state, nodeCount, err
		}

		// Checkpoint after successful node execution
		if cfg.checkpointStore != nil {
			if err := cg.saveCheckpointWithObservability(fgCtx, cfg, current, prevNode, state, next); err != nil {
				return state, nodeCount, err
			}
		}

		prevNode = current
		current = next
	}

	return state, nodeCount, nil
}

// saveCheckpoint persists the current state after node execution.
// Used by Resume() which doesn't have full observability config.
func (cg *CompiledGraph[S]) saveCheckpoint(ctx Context, cfg *runConfig, nodeID, prevNodeID string, state S, nextNode string) error {
	return cg.saveCheckpointWithObservability(ctx, cfg, nodeID, prevNodeID, state, nextNode)
}

// saveCheckpointWithObservability persists the current state with observability.
func (cg *CompiledGraph[S]) saveCheckpointWithObservability(ctx Context, cfg *runConfig, nodeID, prevNodeID string, state S, nextNode string) error {
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
		observability.LogCheckpointError(cfg.logger, nodeID, "serialize", err)
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
		observability.LogCheckpointError(cfg.logger, nodeID, "marshal", err)
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
		observability.LogCheckpointError(cfg.logger, nodeID, "save", err)
		return nil
	}

	// Log and record successful checkpoint
	sizeBytes := len(data)
	observability.LogCheckpoint(cfg.logger, nodeID, sizeBytes)
	cfg.metrics.RecordCheckpoint(ctx, nodeID, int64(sizeBytes))

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
