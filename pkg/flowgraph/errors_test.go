package flowgraph

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNodeError_Error tests NodeError formatting.
func TestNodeError_Error(t *testing.T) {
	err := &NodeError{
		NodeID: "process",
		Op:     "execute",
		Err:    errors.New("connection failed"),
	}

	assert.Equal(t, "node process: execute: connection failed", err.Error())
}

// TestNodeError_Unwrap tests NodeError unwrapping.
func TestNodeError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying")
	err := &NodeError{
		NodeID: "test",
		Op:     "execute",
		Err:    underlying,
	}

	assert.ErrorIs(t, err, underlying)
}

// TestPanicError_Error tests PanicError formatting.
func TestPanicError_Error(t *testing.T) {
	err := &PanicError{
		NodeID: "crash",
		Value:  "unexpected nil",
		Stack:  "goroutine 1 [running]:\n...",
	}

	assert.Equal(t, "node crash panicked: unexpected nil", err.Error())
}

// TestCancellationError_Error_BeforeExecution tests cancellation error before execution.
func TestCancellationError_Error_BeforeExecution(t *testing.T) {
	err := &CancellationError{
		NodeID:       "pending",
		State:        nil,
		Cause:        context.Canceled,
		WasExecuting: false,
	}

	assert.Equal(t, "cancelled before node pending: context canceled", err.Error())
}

// TestCancellationError_Error_DuringExecution tests cancellation error during execution.
func TestCancellationError_Error_DuringExecution(t *testing.T) {
	err := &CancellationError{
		NodeID:       "running",
		State:        nil,
		Cause:        context.DeadlineExceeded,
		WasExecuting: true,
	}

	assert.Equal(t, "cancelled during node running: context deadline exceeded", err.Error())
}

// TestCancellationError_Unwrap tests CancellationError unwrapping.
func TestCancellationError_Unwrap(t *testing.T) {
	err := &CancellationError{
		NodeID:       "test",
		Cause:        context.Canceled,
		WasExecuting: false,
	}

	assert.ErrorIs(t, err, context.Canceled)
}

// TestRouterError_Error tests RouterError formatting.
func TestRouterError_Error(t *testing.T) {
	err := &RouterError{
		FromNode: "route",
		Returned: "unknown",
		Err:      ErrRouterTargetNotFound,
	}

	assert.Equal(t, "router from route returned \"unknown\": router returned unknown node", err.Error())
}

// TestRouterError_Unwrap tests RouterError unwrapping.
func TestRouterError_Unwrap(t *testing.T) {
	err := &RouterError{
		FromNode: "test",
		Returned: "",
		Err:      ErrInvalidRouterResult,
	}

	assert.ErrorIs(t, err, ErrInvalidRouterResult)
}

// TestMaxIterationsError_Error tests MaxIterationsError formatting.
func TestMaxIterationsError_Error(t *testing.T) {
	err := &MaxIterationsError{
		Max:        1000,
		LastNodeID: "loop",
		State:      nil,
	}

	assert.Equal(t, "exceeded maximum iterations (1000) at node loop", err.Error())
}

// TestMaxIterationsError_Unwrap tests MaxIterationsError unwrapping.
func TestMaxIterationsError_Unwrap(t *testing.T) {
	err := &MaxIterationsError{
		Max:        100,
		LastNodeID: "test",
	}

	assert.ErrorIs(t, err, ErrMaxIterations)
}
