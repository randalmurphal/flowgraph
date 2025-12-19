package flowgraph

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// Context provides execution context to nodes.
// It extends context.Context with flowgraph-specific services and metadata.
//
// Context is immutable after creation. The executor creates derived contexts
// for each node with updated NodeID and enriched logger.
type Context interface {
	context.Context

	// Services

	// Logger returns the configured logger, enriched with run and node context.
	// Never returns nil - defaults to slog.Default() if not configured.
	Logger() *slog.Logger

	// LLM returns the LLM client, or nil if not configured.
	// Nodes should check for nil before using.
	LLM() LLMClient

	// Checkpointer returns the checkpoint store, or nil if not configured.
	// Nodes should check for nil before using.
	Checkpointer() CheckpointStore

	// Metadata

	// RunID returns the unique identifier for this execution run.
	// Auto-generated if not configured.
	RunID() string

	// NodeID returns the current node being executed.
	// Empty string before execution starts.
	NodeID() string

	// Attempt returns the retry attempt number (1 = first attempt).
	Attempt() int
}

// LLMClient is the interface for LLM providers.
// Defined here as a placeholder; full implementation in Phase 4.
type LLMClient interface {
	// Methods will be defined in Phase 4
}

// CheckpointStore is the interface for checkpoint persistence.
// Defined here as a placeholder; full implementation in Phase 3.
type CheckpointStore interface {
	// Methods will be defined in Phase 3
}

// executionContext is the internal implementation of Context.
type executionContext struct {
	context.Context

	logger       *slog.Logger
	llm          LLMClient
	checkpointer CheckpointStore
	runID        string
	nodeID       string
	attempt      int
}

// Logger returns the configured logger.
func (c *executionContext) Logger() *slog.Logger {
	return c.logger
}

// LLM returns the LLM client.
func (c *executionContext) LLM() LLMClient {
	return c.llm
}

// Checkpointer returns the checkpoint store.
func (c *executionContext) Checkpointer() CheckpointStore {
	return c.checkpointer
}

// RunID returns the run identifier.
func (c *executionContext) RunID() string {
	return c.runID
}

// NodeID returns the current node identifier.
func (c *executionContext) NodeID() string {
	return c.nodeID
}

// Attempt returns the retry attempt number.
func (c *executionContext) Attempt() int {
	return c.attempt
}

// ContextOption configures a Context.
type ContextOption func(*executionContext)

// WithLogger sets the logger for the context.
// The logger will be enriched with run_id, node_id, and attempt during execution.
func WithLogger(logger *slog.Logger) ContextOption {
	return func(c *executionContext) {
		c.logger = logger
	}
}

// WithLLM sets the LLM client for the context.
func WithLLM(client LLMClient) ContextOption {
	return func(c *executionContext) {
		c.llm = client
	}
}

// WithCheckpointer sets the checkpoint store for the context.
func WithCheckpointer(store CheckpointStore) ContextOption {
	return func(c *executionContext) {
		c.checkpointer = store
	}
}

// WithRunID sets the run identifier for the context.
// If not set, a UUID will be auto-generated.
func WithRunID(id string) ContextOption {
	return func(c *executionContext) {
		c.runID = id
	}
}

// NewContext creates an execution context from a standard context.
// The returned Context wraps the provided context.Context and adds
// flowgraph-specific services and metadata.
//
// Example:
//
//	ctx := flowgraph.NewContext(context.Background(),
//	    flowgraph.WithLogger(myLogger),
//	    flowgraph.WithRunID("run-123"))
func NewContext(ctx context.Context, opts ...ContextOption) Context {
	ec := &executionContext{
		Context: ctx,
		logger:  slog.Default(),
		runID:   uuid.New().String(),
		attempt: 1,
	}

	for _, opt := range opts {
		opt(ec)
	}

	return ec
}

// withNodeID returns a new context with the given node ID set.
// Used internally by the executor to enrich the context per-node.
func (c *executionContext) withNodeID(nodeID string) *executionContext {
	return &executionContext{
		Context:      c.Context,
		logger:       c.logger.With("run_id", c.runID, "node_id", nodeID, "attempt", c.attempt),
		llm:          c.llm,
		checkpointer: c.checkpointer,
		runID:        c.runID,
		nodeID:       nodeID,
		attempt:      c.attempt,
	}
}

// withAttempt returns a new context with the given attempt number set.
// Used internally for retry logic.
func (c *executionContext) withAttempt(attempt int) *executionContext {
	return &executionContext{
		Context:      c.Context,
		logger:       c.logger,
		llm:          c.llm,
		checkpointer: c.checkpointer,
		runID:        c.runID,
		nodeID:       c.nodeID,
		attempt:      attempt,
	}
}
