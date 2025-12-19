# flowgraph API Surface

**Version**: 1.0 (Frozen)
**Date**: 2025-12-19

This document defines the complete public API of flowgraph v1.0. Changes to this API after freeze are considered breaking changes.

---

## Package flowgraph

### Constants

```go
// END is the terminal node identifier
const END = "__end__"
```

### Types

#### Graph Builder

```go
// Graph is the mutable builder for creating execution graphs
type Graph[S any] struct {
    // unexported fields
}

// NewGraph creates a new graph builder for state type S
func NewGraph[S any]() *Graph[S]

// AddNode adds a named node to the graph
// Panics if id is empty, reserved, contains whitespace, fn is nil, or id exists
func (g *Graph[S]) AddNode(id string, fn NodeFunc[S]) *Graph[S]

// AddEdge adds an unconditional edge from one node to another
func (g *Graph[S]) AddEdge(from, to string) *Graph[S]

// AddConditionalEdge adds a conditional edge with a router function
func (g *Graph[S]) AddConditionalEdge(from string, router RouterFunc[S]) *Graph[S]

// SetEntry designates the entry point node
func (g *Graph[S]) SetEntry(id string) *Graph[S]

// Compile validates the graph and returns an immutable CompiledGraph
func (g *Graph[S]) Compile() (*CompiledGraph[S], error)
```

#### Compiled Graph

```go
// CompiledGraph is an immutable, executable graph
type CompiledGraph[S any] struct {
    // unexported fields
}

// EntryPoint returns the entry node ID
func (cg *CompiledGraph[S]) EntryPoint() string

// NodeIDs returns all node identifiers
func (cg *CompiledGraph[S]) NodeIDs() []string

// HasNode checks if a node exists
func (cg *CompiledGraph[S]) HasNode(id string) bool

// Successors returns nodes reachable from the given node
func (cg *CompiledGraph[S]) Successors(id string) []string

// Predecessors returns nodes that can reach the given node
func (cg *CompiledGraph[S]) Predecessors(id string) []string

// IsConditional returns true if node has a conditional edge
func (cg *CompiledGraph[S]) IsConditional(id string) bool

// Run executes the graph with the given initial state
func (cg *CompiledGraph[S]) Run(ctx Context, state S, opts ...RunOption) (S, error)

// Resume continues execution from the last checkpoint
func (cg *CompiledGraph[S]) Resume(ctx Context, store CheckpointStore, runID string, opts ...RunOption) (S, error)

// ResumeFrom continues from a specific node
func (cg *CompiledGraph[S]) ResumeFrom(ctx Context, store CheckpointStore, runID, nodeID string, opts ...RunOption) (S, error)
```

#### Node and Router Functions

```go
// NodeFunc is the signature for all node functions
type NodeFunc[S any] func(ctx Context, state S) (S, error)

// RouterFunc determines the next node based on context and state
type RouterFunc[S any] func(ctx Context, state S) string
```

#### Context

```go
// Context provides execution context to nodes
type Context interface {
    context.Context

    // Services
    Logger() *slog.Logger
    LLM() LLMClient
    Checkpointer() CheckpointStore

    // Metadata
    RunID() string
    NodeID() string
    Attempt() int
}

// NewContext creates an execution context
func NewContext(ctx context.Context, opts ...ContextOption) Context

// ContextOption configures the context
type ContextOption func(*contextConfig)

func WithLogger(logger *slog.Logger) ContextOption
func WithLLM(client LLMClient) ContextOption
func WithCheckpointer(store CheckpointStore) ContextOption
func WithRunID(id string) ContextOption
```

#### Run Options

```go
// RunOption configures execution behavior
type RunOption func(*runConfig)

func WithMaxIterations(n int) RunOption
func WithCheckpointing(store CheckpointStore) RunOption
func WithRunID(id string) RunOption
func WithCheckpointFailureFatal(fatal bool) RunOption
func WithStateOverride[S any](fn func(S) S) RunOption
func WithRevalidate[S any](fn func(S) error) RunOption
func WithLogger(logger *slog.Logger) RunOption
func WithMetrics(enabled bool) RunOption
func WithTracing(enabled bool) RunOption
```

### Errors

#### Sentinel Errors

```go
// Graph building/compilation errors
var (
    ErrNoEntryPoint  = errors.New("entry point not set")
    ErrEntryNotFound = errors.New("entry point node not found")
    ErrNodeNotFound  = errors.New("node not found")
    ErrNoPathToEnd   = errors.New("no path to END from entry")
)

// Execution errors
var (
    ErrMaxIterations        = errors.New("exceeded maximum iterations")
    ErrNilContext           = errors.New("context cannot be nil")
    ErrInvalidRouterResult  = errors.New("router returned empty string")
    ErrRouterTargetNotFound = errors.New("router returned unknown node")
    ErrRunIDRequired        = errors.New("run ID required for checkpointing")
    ErrSerializeState       = errors.New("failed to serialize state")
    ErrDeserializeState     = errors.New("failed to deserialize state")
    ErrInvalidResumeNode    = errors.New("resume node not in graph")
)
```

#### Error Types

```go
// NodeError wraps errors from node execution
type NodeError struct {
    NodeID string
    Op     string
    Err    error
}

func (e *NodeError) Error() string
func (e *NodeError) Unwrap() error

// PanicError captures panic information
type PanicError struct {
    NodeID string
    Value  any
    Stack  string
}

func (e *PanicError) Error() string

// CancellationError captures state at cancellation
type CancellationError struct {
    NodeID       string
    State        any
    Cause        error
    WasExecuting bool
}

func (e *CancellationError) Error() string
func (e *CancellationError) Unwrap() error

// RouterError wraps errors from conditional edge routing
type RouterError struct {
    FromNode string
    Returned string
    Err      error
}

func (e *RouterError) Error() string
func (e *RouterError) Unwrap() error

// MaxIterationsError provides context when loop limit exceeded
type MaxIterationsError struct {
    Max        int
    LastNodeID string
    State      any
}

func (e *MaxIterationsError) Error() string
func (e *MaxIterationsError) Unwrap() error
```

---

## Package flowgraph/checkpoint

### Types

```go
// CheckpointStore persists checkpoints for crash recovery
type CheckpointStore interface {
    Save(runID, nodeID string, data []byte) error
    Load(runID, nodeID string) ([]byte, error)
    List(runID string) ([]CheckpointInfo, error)
    Delete(runID, nodeID string) error
    DeleteRun(runID string) error
    Close() error
}

// CheckpointInfo provides metadata without loading full state
type CheckpointInfo struct {
    RunID     string
    NodeID    string
    Sequence  int
    Timestamp time.Time
    Size      int64
}

// Checkpoint is the persisted snapshot
type Checkpoint struct {
    RunID     string          `json:"run_id"`
    NodeID    string          `json:"node_id"`
    Sequence  int             `json:"sequence"`
    Timestamp time.Time       `json:"timestamp"`
    Version   string          `json:"version"`
    State     json.RawMessage `json:"state"`
    NextNode  string          `json:"next_node"`
}

func (c *Checkpoint) Marshal() ([]byte, error)
func Unmarshal(data []byte) (*Checkpoint, error)
```

### Constructors

```go
func NewMemoryStore() *MemoryStore
func NewSQLiteStore(path string) (*SQLiteStore, error)
```

### Errors

```go
var ErrCheckpointNotFound = errors.New("checkpoint not found")
```

---

## Package flowgraph/llm

### Types

```go
// LLMClient is the interface for LLM providers
type LLMClient interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

// CompletionRequest is the input to Complete/Stream
type CompletionRequest struct {
    SystemPrompt string
    Messages     []Message
    Model        string
    MaxTokens    int
    Temperature  float64
    Tools        []Tool
    Options      map[string]any
}

// Message is a conversation turn
type Message struct {
    Role    Role
    Content string
    Name    string
}

// Role identifies the message sender
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

// Tool defines an available tool
type Tool struct {
    Name        string
    Description string
    Parameters  json.RawMessage
}

// CompletionResponse is the output of Complete
type CompletionResponse struct {
    Content      string
    ToolCalls    []ToolCall
    Usage        TokenUsage
    Model        string
    FinishReason string
    Duration     time.Duration
}

// ToolCall represents a tool invocation request
type ToolCall struct {
    ID        string
    Name      string
    Arguments json.RawMessage
}

// TokenUsage tracks token consumption
type TokenUsage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
}

// StreamChunk is a piece of a streaming response
type StreamChunk struct {
    Content   string
    ToolCalls []ToolCall
    Usage     *TokenUsage
    Done      bool
    Error     error
}
```

### Constructors

```go
// ClaudeCLI
func NewClaudeCLI(opts ...ClaudeOption) *ClaudeCLI

type ClaudeOption func(*ClaudeCLI)

func WithClaudePath(path string) ClaudeOption
func WithModel(model string) ClaudeOption
func WithWorkdir(dir string) ClaudeOption
func WithAllowedTools(tools []string) ClaudeOption

// MockLLM
func NewMockLLM(response string) *MockLLM

func (m *MockLLM) WithResponses(responses ...string) *MockLLM
func (m *MockLLM) WithError(err error) *MockLLM
```

### Errors

```go
var (
    ErrLLMUnavailable = errors.New("LLM service unavailable")
    ErrContextTooLong = errors.New("context exceeds maximum length")
    ErrRateLimited    = errors.New("rate limited")
    ErrInvalidRequest = errors.New("invalid request")
)

type LLMError struct {
    Op        string
    Err       error
    Retryable bool
}

func (e *LLMError) Error() string
func (e *LLMError) Unwrap() error
```

---

## API Stability Guarantees

### Stable (will not change in v1.x)

- All types and functions listed above
- Error messages (for `errors.Is` compatibility)
- JSON field names in Checkpoint
- Method signatures

### May Change (with deprecation notice)

- Performance characteristics
- Log message formats
- Metric names
- Span names

### Not Part of Public API

- Unexported fields and methods
- Package `internal/`
- Test utilities (`*_test.go`)
- Build tags and constraints

---

## Deprecation Policy

1. Deprecated items marked with `// Deprecated:` comment
2. Deprecated items work for at least 2 minor versions
3. Migration guide provided in CHANGELOG
4. Removal only in major version

---

## Version Compatibility

| Go Version | flowgraph Version |
|------------|-------------------|
| 1.22+ | v1.x |
| 1.21 | Not supported (requires slog) |
| < 1.21 | Not supported |
