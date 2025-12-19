// Package llm provides interfaces and implementations for LLM clients.
package llm

import "context"

// Client is the interface for LLM providers.
// Implementations must be safe for concurrent use.
type Client interface {
	// Complete sends a request and returns the full response.
	// The context controls cancellation and timeouts.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Stream sends a request and returns a channel of response chunks.
	// The channel is closed when streaming completes (check chunk.Done).
	// Errors during streaming are returned via chunk.Error.
	Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
