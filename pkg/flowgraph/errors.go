// Package flowgraph provides a graph-based LLM workflow orchestration engine.
package flowgraph

import (
	"errors"
	"fmt"
)

// Sentinel errors for graph building and compilation.
var (
	// ErrNoEntryPoint indicates SetEntry() was not called before Compile().
	ErrNoEntryPoint = errors.New("entry point not set")

	// ErrEntryNotFound indicates the entry point references a non-existent node.
	ErrEntryNotFound = errors.New("entry point node not found")

	// ErrNodeNotFound indicates an edge references a non-existent node.
	ErrNodeNotFound = errors.New("node not found")

	// ErrNoPathToEnd indicates no path exists from the entry point to END.
	ErrNoPathToEnd = errors.New("no path to END from entry")
)

// Sentinel errors for execution.
var (
	// ErrMaxIterations indicates the execution loop exceeded the configured limit.
	ErrMaxIterations = errors.New("exceeded maximum iterations")

	// ErrNilContext indicates Run() was called with a nil context.
	ErrNilContext = errors.New("context cannot be nil")

	// ErrInvalidRouterResult indicates a router function returned an empty string.
	ErrInvalidRouterResult = errors.New("router returned empty string")

	// ErrRouterTargetNotFound indicates a router function returned an unknown node ID.
	ErrRouterTargetNotFound = errors.New("router returned unknown node")
)

// Sentinel errors for checkpointing and resume.
var (
	// ErrRunIDRequired indicates checkpointing was enabled without a run ID.
	ErrRunIDRequired = errors.New("run ID required for checkpointing")

	// ErrSerializeState indicates state serialization failed.
	ErrSerializeState = errors.New("failed to serialize state")

	// ErrDeserializeState indicates state deserialization failed.
	ErrDeserializeState = errors.New("failed to deserialize state")

	// ErrNoCheckpoints indicates no checkpoints exist for the run.
	ErrNoCheckpoints = errors.New("no checkpoints found for run")

	// ErrInvalidResumeNode indicates the resume node doesn't exist in the graph.
	ErrInvalidResumeNode = errors.New("invalid resume node")

	// ErrCheckpointVersionMismatch indicates the checkpoint version is incompatible.
	ErrCheckpointVersionMismatch = errors.New("checkpoint version mismatch")
)

// CheckpointError wraps errors from checkpoint operations.
type CheckpointError struct {
	// NodeID is the node where checkpointing failed.
	NodeID string
	// Op is the operation that failed ("save", "load", "serialize").
	Op string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *CheckpointError) Error() string {
	return fmt.Sprintf("checkpoint %s at node %s: %v", e.Op, e.NodeID, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *CheckpointError) Unwrap() error {
	return e.Err
}

// NodeError wraps an error with node context.
// It provides information about which node failed and what operation was attempted.
type NodeError struct {
	// NodeID is the identifier of the node that failed.
	NodeID string
	// Op is the operation that failed (e.g., "execute").
	Op string
	// Err is the underlying error from the node.
	Err error
}

// Error implements the error interface.
func (e *NodeError) Error() string {
	return fmt.Sprintf("node %s: %s: %v", e.NodeID, e.Op, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *NodeError) Unwrap() error {
	return e.Err
}

// PanicError captures panic information from node execution.
// It includes the stack trace for debugging.
type PanicError struct {
	// NodeID is the identifier of the node that panicked.
	NodeID string
	// Value is the value passed to panic().
	Value any
	// Stack is the full stack trace at the point of panic.
	Stack string
}

// Error implements the error interface.
func (e *PanicError) Error() string {
	return fmt.Sprintf("node %s panicked: %v", e.NodeID, e.Value)
}

// CancellationError captures the state when execution was cancelled.
// It preserves the state at the point of cancellation for recovery.
type CancellationError struct {
	// NodeID is the node that was about to execute or was executing.
	NodeID string
	// State is the state at cancellation (can type-assert to the actual type).
	State any
	// Cause is the underlying cancellation cause (context.Canceled or context.DeadlineExceeded).
	Cause error
	// WasExecuting is true if cancellation occurred during node execution.
	WasExecuting bool
}

// Error implements the error interface.
func (e *CancellationError) Error() string {
	if e.WasExecuting {
		return fmt.Sprintf("cancelled during node %s: %v", e.NodeID, e.Cause)
	}
	return fmt.Sprintf("cancelled before node %s: %v", e.NodeID, e.Cause)
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *CancellationError) Unwrap() error {
	return e.Cause
}

// RouterError wraps errors from conditional edge routing.
// It provides context about which router failed and what it returned.
type RouterError struct {
	// FromNode is the node with the conditional edge.
	FromNode string
	// Returned is the value the router returned.
	Returned string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *RouterError) Error() string {
	return fmt.Sprintf("router from %s returned %q: %v", e.FromNode, e.Returned, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *RouterError) Unwrap() error {
	return e.Err
}

// MaxIterationsError provides context when the loop limit is exceeded.
// It includes the state at termination for inspection.
type MaxIterationsError struct {
	// Max is the configured iteration limit.
	Max int
	// LastNodeID is the node that would have executed next.
	LastNodeID string
	// State is the state at termination (can type-assert to the actual type).
	State any
}

// Error implements the error interface.
func (e *MaxIterationsError) Error() string {
	return fmt.Sprintf("exceeded maximum iterations (%d) at node %s", e.Max, e.LastNodeID)
}

// Unwrap returns ErrMaxIterations for errors.Is support.
func (e *MaxIterationsError) Unwrap() error {
	return ErrMaxIterations
}
