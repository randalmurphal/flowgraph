package flowgraph

import (
	"encoding/json"
	"fmt"
	"time"
)

// ParallelState is an optional interface for state types that want custom
// clone/merge behavior during parallel execution.
//
// If your state type does not implement this interface, the executor will
// fall back to JSON marshaling/unmarshaling for cloning and a simple
// map-based merge for combining branch states.
//
// Implement this interface when you need:
//   - Deep copying of complex nested structures
//   - Custom merge logic (conflict resolution, aggregation, etc.)
//   - Efficient cloning without JSON serialization overhead
//
// Example:
//
//	func (s MyState) Clone(branchID string) MyState {
//	    clone := s
//	    clone.BranchID = branchID
//	    clone.Data = make(map[string]any)
//	    for k, v := range s.Data {
//	        clone.Data[k] = v
//	    }
//	    return clone
//	}
//
//	func (s MyState) Merge(branches map[string]MyState) MyState {
//	    merged := s
//	    for branchID, branchState := range branches {
//	        merged.Data[branchID+"_result"] = branchState.Result
//	    }
//	    return merged
//	}
type ParallelState[S any] interface {
	// Clone creates an independent copy of the state for a parallel branch.
	// The branchID identifies which branch this clone is for.
	Clone(branchID string) S

	// Merge combines the states from all completed branches.
	// The receiver is the original state at the fork point.
	// The branches map contains branchID -> final state from that branch.
	Merge(branches map[string]S) S
}

// BranchHook provides lifecycle callbacks for fork/join execution.
// All methods are optional - the executor uses sensible defaults if nil.
//
// Hooks are called in this order:
//  1. OnFork - called once per branch, before branch execution starts
//  2. (branch nodes execute)
//  3. OnJoin - called once after all branches complete (or OnBranchError if any failed)
//
// Hooks can modify state and abort execution by returning errors.
// The executor will cancel remaining branches if OnFork returns an error.
//
// Example use cases:
//   - Create git worktrees for each branch (OnFork)
//   - Validate all branches committed before merge (OnJoin)
//   - Clean up failed branch resources (OnBranchError)
type BranchHook[S any] interface {
	// OnFork is called before each branch starts executing.
	// The returned state will be used as the initial state for that branch.
	// Return an error to abort the fork - all branches will be cancelled.
	//
	// This is where you set up per-branch resources (e.g., git worktrees).
	OnFork(ctx Context, branchID string, state S) (S, error)

	// OnJoin is called after all branches complete successfully.
	// Use this to validate branch results before merging or to clean up.
	// Return an error to fail the entire fork/join operation.
	OnJoin(ctx Context, branchStates map[string]S) error

	// OnBranchError is called when a branch fails.
	// This is for cleanup - the error has already been recorded.
	// The state is the branch state at the point of failure.
	OnBranchError(ctx Context, branchID string, state S, err error)
}

// ForkJoinConfig configures parallel execution behavior.
// All fields have sensible defaults (zero values are valid).
//
// Note: BranchHook is set via Graph.SetBranchHook() rather than this config
// to maintain proper generic typing.
type ForkJoinConfig struct {
	// MaxConcurrency limits the number of branches executing simultaneously.
	// 0 = unlimited (all branches start immediately).
	// Use this to prevent resource exhaustion with many branches.
	MaxConcurrency int

	// FailFast stops all branches when any branch fails.
	// false = wait for all branches to complete (default).
	// true = cancel remaining branches on first error.
	FailFast bool

	// MergeTimeout is the maximum time to wait for branch completion.
	// 0 = no timeout (wait indefinitely).
	// If timeout is reached, remaining branches are cancelled.
	MergeTimeout time.Duration
}

// DefaultForkJoinConfig returns the default configuration.
// Unlimited concurrency, wait for all branches, no timeout.
func DefaultForkJoinConfig() ForkJoinConfig {
	return ForkJoinConfig{
		MaxConcurrency: 0,     // Unlimited
		FailFast:       false, // Wait for all
		MergeTimeout:   0,     // No timeout
	}
}

// ForkNode represents a point where execution splits into parallel branches.
// This is computed during graph compilation from nodes with multiple outgoing edges.
type ForkNode struct {
	// NodeID is the ID of the fork node in the graph.
	NodeID string

	// Branches are the IDs of the first node in each branch.
	// These are the targets of the outgoing edges from the fork node.
	Branches []string

	// JoinNodeID is where all branches must converge.
	// Computed using post-dominator analysis at compile time.
	JoinNodeID string
}

// JoinNode represents a point where parallel branches converge.
type JoinNode struct {
	// NodeID is the ID of the join node in the graph.
	NodeID string

	// ForkNodeID is the corresponding fork node.
	ForkNodeID string

	// ExpectedBranches are the branch entry nodes that must complete.
	ExpectedBranches []string
}

// BranchResult holds the outcome of a single branch execution.
type BranchResult[S any] struct {
	// BranchID identifies this branch (same as the first node ID).
	BranchID string

	// State is the final state when the branch reached the join point.
	// Nil if the branch failed.
	State S

	// Error is set if the branch failed.
	Error error

	// Duration is how long the branch took to execute.
	Duration time.Duration
}

// ForkJoinResult holds the combined results of parallel execution.
type ForkJoinResult[S any] struct {
	// ForkNodeID is the fork point.
	ForkNodeID string

	// JoinNodeID is the join point.
	JoinNodeID string

	// Branches contains results from all branches.
	Branches []BranchResult[S]

	// MergedState is the combined state after merging all branch states.
	// Only set if all branches succeeded.
	MergedState S

	// TotalDuration is the wall-clock time from fork start to join complete.
	TotalDuration time.Duration

	// Success is true if all branches completed without error.
	Success bool
}

// cloneState creates a copy of state for a parallel branch.
// Uses ParallelState.Clone if available, otherwise falls back to JSON.
func cloneState[S any](state S, branchID string) (S, error) {
	// Check if state implements ParallelState
	if ps, ok := any(state).(ParallelState[S]); ok {
		return ps.Clone(branchID), nil
	}

	// Fallback: JSON marshal/unmarshal
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		var zero S
		return zero, fmt.Errorf("clone state for branch %s: marshal: %w", branchID, marshalErr)
	}

	var clone S
	if unmarshalErr := json.Unmarshal(data, &clone); unmarshalErr != nil {
		var zero S
		return zero, fmt.Errorf("clone state for branch %s: unmarshal: %w", branchID, unmarshalErr)
	}

	return clone, nil
}

// mergeStates combines branch states back into a single state.
// Uses ParallelState.Merge if available, otherwise returns the first branch state.
func mergeStates[S any](originalState S, branchStates map[string]S) S {
	// Check if state implements ParallelState
	if ps, ok := any(originalState).(ParallelState[S]); ok {
		return ps.Merge(branchStates)
	}

	// Fallback: return original state (branches' side effects are lost)
	// This is intentional - without ParallelState, we can't know how to merge.
	// The hook's OnJoin can handle custom merge logic if needed.
	return originalState
}
