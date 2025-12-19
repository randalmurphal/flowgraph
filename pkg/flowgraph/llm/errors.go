package llm

import (
	"errors"
	"fmt"
)

// Sentinel errors for LLM operations.
var (
	// ErrUnavailable indicates the LLM service is unavailable.
	ErrUnavailable = errors.New("LLM service unavailable")

	// ErrContextTooLong indicates the input exceeds the context window.
	ErrContextTooLong = errors.New("context exceeds maximum length")

	// ErrRateLimited indicates the request was rate limited.
	ErrRateLimited = errors.New("rate limited")

	// ErrInvalidRequest indicates the request is malformed.
	ErrInvalidRequest = errors.New("invalid request")

	// ErrTimeout indicates the request timed out.
	ErrTimeout = errors.New("request timed out")
)

// Error wraps LLM errors with context.
type Error struct {
	Op        string // Operation that failed ("complete", "stream")
	Err       error  // Underlying error
	Retryable bool   // Whether the error is likely transient
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("llm %s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *Error) Unwrap() error {
	return e.Err
}

// NewError creates a new LLM error.
func NewError(op string, err error, retryable bool) *Error {
	return &Error{
		Op:        op,
		Err:       err,
		Retryable: retryable,
	}
}
