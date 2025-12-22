package flowgraph

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// executeForkJoin handles parallel execution of a fork node.
// It clones state for each branch, executes branches in goroutines,
// waits for completion, and merges the results.
//
// Returns the merged state and the join node to continue from.
func (cg *CompiledGraph[S]) executeForkJoin(
	ctx Context,
	forkNode *ForkNode,
	state S,
	cfg *runConfig,
) (mergedState S, joinNode string, err error) {
	startTime := time.Now()
	hook := cg.getBranchHook()
	fjConfig := cg.getForkJoinConfig()

	// Set up concurrency control
	var sem chan struct{}
	if fjConfig.MaxConcurrency > 0 {
		sem = make(chan struct{}, fjConfig.MaxConcurrency)
	}

	// Context with optional timeout for cancellation checking
	// We keep the flowgraph Context for execution but use a derived context.Context for timeout
	var timeoutCtx context.Context = ctx
	var cancel context.CancelFunc
	if fjConfig.MergeTimeout > 0 {
		timeoutCtx, cancel = context.WithTimeout(ctx, fjConfig.MergeTimeout)
		defer cancel()
	}

	// Clone state for each branch
	branchStates := make(map[string]S)
	for _, branchID := range forkNode.Branches {
		cloned, cloneErr := cloneState(state, branchID)
		if cloneErr != nil {
			return state, "", fmt.Errorf("fork node %s: clone state for branch %s: %w",
				forkNode.NodeID, branchID, cloneErr)
		}

		// Call OnFork hook if available
		if hook != nil {
			var hookErr error
			cloned, hookErr = hook.OnFork(ctx, branchID, cloned)
			if hookErr != nil {
				return state, "", fmt.Errorf("fork node %s: OnFork hook for branch %s: %w",
					forkNode.NodeID, branchID, hookErr)
			}
		}

		branchStates[branchID] = cloned
	}

	// Execute branches in parallel
	results := make(chan BranchResult[S], len(forkNode.Branches))
	var wg sync.WaitGroup

	for _, branchID := range forkNode.Branches {
		wg.Add(1)
		go func(bID string, bState S) {
			defer wg.Done()

			// Acquire semaphore if concurrency is limited
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-timeoutCtx.Done():
					results <- BranchResult[S]{
						BranchID: bID,
						Error:    timeoutCtx.Err(),
					}
					return
				}
			}

			// Execute this branch (pass timeoutCtx for tracing, ctx for flowgraph context)
			result := cg.executeBranch(timeoutCtx, ctx, bID, bState, forkNode.JoinNodeID, cfg)
			results <- result

			// Notify hook on error
			if result.Error != nil && hook != nil {
				hook.OnBranchError(ctx, bID, bState, result.Error)
			}
		}(branchID, branchStates[branchID])
	}

	// Wait for all branches to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	branchResults := make([]BranchResult[S], 0, len(forkNode.Branches))
	var firstError error
	successfulStates := make(map[string]S)

	for result := range results {
		branchResults = append(branchResults, result)

		if result.Error != nil {
			if firstError == nil {
				firstError = result.Error
			}
			// If fail-fast, we could cancel here, but we've already started all branches
			// The context cancellation from timeout handles this
		} else {
			successfulStates[result.BranchID] = result.State
		}
	}

	// Check for errors
	if firstError != nil {
		return state, "", &ForkJoinError{
			ForkNodeID: forkNode.NodeID,
			BranchID:   branchResults[0].BranchID, // First failed branch
			Err:        firstError,
		}
	}

	// Call OnJoin hook if available
	if hook != nil {
		if joinErr := hook.OnJoin(ctx, successfulStates); joinErr != nil {
			return state, "", fmt.Errorf("fork node %s: OnJoin hook: %w",
				forkNode.NodeID, joinErr)
		}
	}

	// Merge states
	mergedState = mergeStates(state, successfulStates)

	// Log completion
	duration := time.Since(startTime)
	ctx.Logger().Info("fork/join completed",
		"fork_node", forkNode.NodeID,
		"join_node", forkNode.JoinNodeID,
		"branches", len(forkNode.Branches),
		"duration_ms", duration.Milliseconds())

	return mergedState, forkNode.JoinNodeID, nil
}

// executeBranch executes a single branch from its start node until it reaches the join node.
func (cg *CompiledGraph[S]) executeBranch(
	tracingCtx context.Context,
	fgCtx Context,
	branchID string,
	state S,
	joinNodeID string,
	cfg *runConfig,
) BranchResult[S] {
	startTime := time.Now()
	current := branchID
	iterations := 0

	for current != joinNodeID && current != END {
		iterations++
		if iterations > cfg.maxIterations {
			return BranchResult[S]{
				BranchID: branchID,
				Error: &MaxIterationsError{
					Max:        cfg.maxIterations,
					LastNodeID: current,
					State:      state,
				},
				Duration: time.Since(startTime),
			}
		}

		// Check for cancellation
		select {
		case <-fgCtx.Done():
			return BranchResult[S]{
				BranchID: branchID,
				Error: &CancellationError{
					NodeID:       current,
					State:        state,
					Cause:        fgCtx.Err(),
					WasExecuting: false,
				},
				Duration: time.Since(startTime),
			}
		default:
		}

		// Execute the node
		var nodeErr error
		state, nodeErr = cg.executeNode(fgCtx, current, state)
		if nodeErr != nil {
			return BranchResult[S]{
				BranchID: branchID,
				State:    state,
				Error:    nodeErr,
				Duration: time.Since(startTime),
			}
		}

		// Determine next node
		next, routeErr := cg.nextNode(fgCtx, state, current)
		if routeErr != nil {
			return BranchResult[S]{
				BranchID: branchID,
				State:    state,
				Error:    routeErr,
				Duration: time.Since(startTime),
			}
		}

		current = next
	}

	return BranchResult[S]{
		BranchID: branchID,
		State:    state,
		Duration: time.Since(startTime),
	}
}

// ForkJoinError represents an error during fork/join execution.
type ForkJoinError struct {
	ForkNodeID string
	BranchID   string
	Err        error
}

func (e *ForkJoinError) Error() string {
	return fmt.Sprintf("fork/join error at %s (branch %s): %v", e.ForkNodeID, e.BranchID, e.Err)
}

func (e *ForkJoinError) Unwrap() error {
	return e.Err
}
