# Feature: LLM Client Interface

**Related ADRs**: 018-llm-interface, 019-context-window, 020-streaming, 021-token-tracking

---

## Problem Statement

flowgraph orchestrates LLM workflows. Nodes need to:
1. Call LLMs with prompts
2. Stream responses for long completions
3. Track token usage for cost management
4. Work with different LLM providers

The client interface must be simple, mockable, and provider-agnostic.

## User Stories

- As a developer, I want a simple interface for LLM calls so that nodes are easy to write
- As a developer, I want to stream responses so that long completions feel responsive
- As a developer, I want token counts so that I can track costs
- As a developer, I want to mock LLM calls so that I can test nodes without API calls
- As a developer, I want to use Claude CLI so that I get tool use and file access

---

## API Design

### LLMClient Interface

```go
// LLMClient is the interface for LLM providers
type LLMClient interface {
    // Complete sends a request and returns the full response
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

    // Stream sends a request and returns a channel of chunks
    // The channel is closed when streaming completes
    // Errors are returned via the final chunk
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
```

### Request Types

```go
// CompletionRequest is the input to Complete/Stream
type CompletionRequest struct {
    // Prompt configuration
    SystemPrompt string    // System message
    Messages     []Message // Conversation history

    // Model configuration
    Model       string  // Model identifier (optional, provider decides default)
    MaxTokens   int     // Max tokens in response (0 = provider default)
    Temperature float64 // 0.0-1.0 (0 = provider default = typically 1.0)

    // Tool use (optional)
    Tools []Tool // Available tools

    // Provider-specific options
    Options map[string]any
}

// Message is a conversation turn
type Message struct {
    Role    Role   // User, Assistant, Tool
    Content string
    Name    string // For tool results
}

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
    Parameters  json.RawMessage // JSON Schema
}
```

### Response Types

```go
// CompletionResponse is the output of Complete
type CompletionResponse struct {
    // Content
    Content string // Text response

    // Tool use (if any)
    ToolCalls []ToolCall

    // Usage
    Usage TokenUsage

    // Metadata
    Model      string        // Actual model used
    FinishReason string      // stop, tool_use, max_tokens
    Duration   time.Duration // Request duration
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
    Content   string      // Incremental content
    ToolCalls []ToolCall  // Partial tool calls
    Usage     *TokenUsage // Only in final chunk
    Done      bool        // True for final chunk
    Error     error       // Non-nil if streaming failed
}
```

### Claude CLI Client

```go
// NewClaudeCLI creates a client that shells out to Claude CLI
func NewClaudeCLI(opts ...ClaudeOption) *ClaudeCLI

type ClaudeOption func(*ClaudeCLI)

// WithClaudePath sets the path to the claude binary
func WithClaudePath(path string) ClaudeOption

// WithModel sets the default model
func WithModel(model string) ClaudeOption

// WithWorkdir sets the working directory for Claude
func WithWorkdir(dir string) ClaudeOption

// WithAllowedTools configures which tools Claude can use
func WithAllowedTools(tools []string) ClaudeOption
```

### Mock Client

```go
// MockLLM is a test double for LLMClient
type MockLLM struct {
    CompleteFunc func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    StreamFunc   func(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

// NewMockLLM creates a mock that returns fixed responses
func NewMockLLM(response string) *MockLLM

// WithResponses creates a mock that returns responses in sequence
func (m *MockLLM) WithResponses(responses ...string) *MockLLM

// WithError creates a mock that always returns an error
func (m *MockLLM) WithError(err error) *MockLLM
```

---

## Behavior Specification

### Complete Behavior

```
Complete(ctx, req) called
        │
        ▼
Validate request
        │
        ├── invalid ──► return error
        │
        │ valid
        ▼
Call LLM provider
        │
        ├── error ──► return wrapped error
        │
        │ success
        ▼
Parse response
        │
        ▼
Return CompletionResponse
```

### Stream Behavior

```
Stream(ctx, req) called
        │
        ▼
Validate request
        │
        ├── invalid ──► return nil, error
        │
        │ valid
        ▼
Start streaming (return channel immediately)
        │
        ▼
┌─► Read chunk from provider
│       │
│       ├── error ──► send chunk with Error, close channel
│       │
│       ├── done ──► send final chunk with Done=true, close channel
│       │
│       └── ok ──► send chunk
│               │
└───────────────┘
```

### Claude CLI Integration

The Claude CLI client:
1. Builds command arguments from request
2. Executes `claude` binary with `--print` mode
3. Captures stdout as response
4. Parses structured output for tool calls

```bash
# Generated command
claude --print --system "You are helpful" "User prompt here"
```

### Token Tracking (ADR-021)

Tokens are tracked in the response, not globally:

```go
resp, _ := client.Complete(ctx, req)
log.Printf("Used %d tokens", resp.Usage.TotalTokens)

// Accumulate in state if needed
state.TotalTokens += resp.Usage.TotalTokens
```

### Context Window (ADR-019)

flowgraph does NOT manage context windows. User/devflow responsibility:

```go
// User manages conversation history
if tokenCount(messages) > contextLimit {
    messages = pruneMessages(messages)
}

resp, _ := client.Complete(ctx, CompletionRequest{
    Messages: messages,
})
```

---

## Error Cases

```go
var (
    ErrLLMUnavailable = errors.New("LLM service unavailable")
    ErrContextTooLong = errors.New("context exceeds maximum length")
    ErrRateLimited    = errors.New("rate limited")
    ErrInvalidRequest = errors.New("invalid request")
)

// LLMError wraps errors with context
type LLMError struct {
    Op      string // "complete", "stream"
    Err     error
    Retryable bool
}
```

---

## Test Cases

### Complete

```go
func TestComplete_BasicRequest(t *testing.T) {
    mock := flowgraph.NewMockLLM("Hello, world!")

    resp, err := mock.Complete(ctx, flowgraph.CompletionRequest{
        SystemPrompt: "You are helpful",
        Messages: []flowgraph.Message{
            {Role: flowgraph.RoleUser, Content: "Say hello"},
        },
    })

    require.NoError(t, err)
    assert.Equal(t, "Hello, world!", resp.Content)
}

func TestComplete_WithTokenTracking(t *testing.T) {
    mock := &flowgraph.MockLLM{
        CompleteFunc: func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
            return &CompletionResponse{
                Content: "Response",
                Usage: TokenUsage{
                    InputTokens:  10,
                    OutputTokens: 5,
                    TotalTokens:  15,
                },
            }, nil
        },
    }

    resp, _ := mock.Complete(ctx, flowgraph.CompletionRequest{})

    assert.Equal(t, 15, resp.Usage.TotalTokens)
}
```

### Streaming

```go
func TestStream_ReceivesChunks(t *testing.T) {
    chunks := []string{"Hello", ", ", "world", "!"}
    mock := &flowgraph.MockLLM{
        StreamFunc: func(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
            ch := make(chan StreamChunk)
            go func() {
                defer close(ch)
                for i, content := range chunks {
                    ch <- StreamChunk{
                        Content: content,
                        Done:    i == len(chunks)-1,
                    }
                }
            }()
            return ch, nil
        },
    }

    ch, err := mock.Stream(ctx, flowgraph.CompletionRequest{})
    require.NoError(t, err)

    var collected []string
    for chunk := range ch {
        collected = append(collected, chunk.Content)
    }

    assert.Equal(t, chunks, collected)
}

func TestStream_ErrorInChunk(t *testing.T) {
    mock := &flowgraph.MockLLM{
        StreamFunc: func(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
            ch := make(chan StreamChunk)
            go func() {
                defer close(ch)
                ch <- StreamChunk{Content: "partial"}
                ch <- StreamChunk{Error: errors.New("connection lost")}
            }()
            return ch, nil
        },
    }

    ch, _ := mock.Stream(ctx, flowgraph.CompletionRequest{})

    var lastChunk StreamChunk
    for chunk := range ch {
        lastChunk = chunk
    }

    assert.Error(t, lastChunk.Error)
}
```

### Mock Helpers

```go
func TestMock_SequentialResponses(t *testing.T) {
    mock := flowgraph.NewMockLLM("").
        WithResponses("first", "second", "third")

    r1, _ := mock.Complete(ctx, flowgraph.CompletionRequest{})
    r2, _ := mock.Complete(ctx, flowgraph.CompletionRequest{})
    r3, _ := mock.Complete(ctx, flowgraph.CompletionRequest{})

    assert.Equal(t, "first", r1.Content)
    assert.Equal(t, "second", r2.Content)
    assert.Equal(t, "third", r3.Content)
}

func TestMock_AlwaysError(t *testing.T) {
    mock := flowgraph.NewMockLLM("").
        WithError(errors.New("API down"))

    _, err := mock.Complete(ctx, flowgraph.CompletionRequest{})

    assert.Error(t, err)
}
```

### Node Integration

```go
func TestNode_UsesContextLLM(t *testing.T) {
    mock := flowgraph.NewMockLLM("Generated spec")

    generateSpec := func(ctx flowgraph.Context, s State) (State, error) {
        resp, err := ctx.LLM().Complete(ctx, flowgraph.CompletionRequest{
            SystemPrompt: "Generate a spec",
            Messages: []flowgraph.Message{
                {Role: flowgraph.RoleUser, Content: s.Ticket.Description},
            },
        })
        if err != nil {
            return s, err
        }
        s.Spec = resp.Content
        return s, nil
    }

    ctx := flowgraph.NewContext(context.Background(),
        flowgraph.WithLLM(mock))

    result, err := generateSpec(ctx, State{Ticket: &Ticket{Description: "test"}})

    require.NoError(t, err)
    assert.Equal(t, "Generated spec", result.Spec)
}
```

---

## Performance Requirements

| Operation | Target |
|-----------|--------|
| Mock Complete | < 1 microsecond |
| Claude CLI start | < 500ms |
| Streaming first chunk | < 1s |
| Channel overhead | < 1 microsecond per chunk |

---

## Security Considerations

1. **API keys**: Never log or checkpoint API keys
2. **Prompt injection**: User must sanitize inputs
3. **Claude CLI permissions**: Runs with process permissions; use sandboxing if needed
4. **Tool execution**: Tools can execute arbitrary code; restrict carefully

---

## Simplicity Check

**What we included**:
- Simple two-method interface (Complete, Stream)
- Request/Response types that work with any provider
- Token tracking in response
- Claude CLI implementation
- Mock for testing

**What we did NOT include**:
- Context window management - User/devflow responsibility (ADR-019)
- Automatic retry - User implements or uses options
- Rate limiting - Provider handles; user can layer on
- Prompt templating - Use Go templates externally
- Message history management - User maintains in state
- Multiple model backends - One client per model; compose externally

**Is this the simplest solution?** Yes. Two methods cover all use cases. Everything else is layered on top.
