# Phase 4: LLM Clients

**Status**: Ready (Can start after Phase 1, parallel with 2-3)
**Estimated Effort**: 2-3 days
**Dependencies**: Phase 1 Complete

---

## Goal

Implement the LLM client interface with Claude CLI as the primary implementation, plus a mock for testing.

---

## Files to Create

```
pkg/flowgraph/
├── llm/
│   ├── client.go        # LLMClient interface
│   ├── request.go       # CompletionRequest, Response types
│   ├── claude_cli.go    # Claude CLI implementation
│   ├── mock.go          # MockLLM for testing
│   ├── client_test.go   # Interface contract tests
│   ├── claude_cli_test.go  # Claude CLI tests
│   └── mock_test.go     # Mock helper tests
└── context.go           # MODIFY: Add LLM() method implementation
```

---

## Implementation Order

### Step 1: Types (~2 hours)

**llm/client.go**
```go
package llm

import "context"

// LLMClient is the interface for LLM providers
type LLMClient interface {
    // Complete sends a request and returns the full response
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

    // Stream sends a request and returns a channel of chunks
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
```

**llm/request.go**
```go
package llm

import (
    "encoding/json"
    "time"
)

// CompletionRequest is the input to Complete/Stream
type CompletionRequest struct {
    // Prompt configuration
    SystemPrompt string    `json:"system_prompt,omitempty"`
    Messages     []Message `json:"messages"`

    // Model configuration
    Model       string  `json:"model,omitempty"`
    MaxTokens   int     `json:"max_tokens,omitempty"`
    Temperature float64 `json:"temperature,omitempty"`

    // Tool use
    Tools []Tool `json:"tools,omitempty"`

    // Provider-specific
    Options map[string]any `json:"options,omitempty"`
}

// Message is a conversation turn
type Message struct {
    Role    Role   `json:"role"`
    Content string `json:"content"`
    Name    string `json:"name,omitempty"`  // For tool results
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
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"`  // JSON Schema
}

// CompletionResponse is the output of Complete
type CompletionResponse struct {
    Content      string        `json:"content"`
    ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
    Usage        TokenUsage    `json:"usage"`
    Model        string        `json:"model"`
    FinishReason string        `json:"finish_reason"`
    Duration     time.Duration `json:"duration"`
}

// ToolCall represents a tool invocation request
type ToolCall struct {
    ID        string          `json:"id"`
    Name      string          `json:"name"`
    Arguments json.RawMessage `json:"arguments"`
}

// TokenUsage tracks token consumption
type TokenUsage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
    TotalTokens  int `json:"total_tokens"`
}

// StreamChunk is a piece of a streaming response
type StreamChunk struct {
    Content   string      `json:"content,omitempty"`
    ToolCalls []ToolCall  `json:"tool_calls,omitempty"`
    Usage     *TokenUsage `json:"usage,omitempty"`  // Only in final chunk
    Done      bool        `json:"done"`
    Error     error       `json:"-"`  // Non-nil if streaming failed
}
```

### Step 2: Mock Client (~1 hour)

**llm/mock.go**
```go
package llm

import (
    "context"
    "sync"
)

// MockLLM is a test double for LLMClient
type MockLLM struct {
    mu           sync.Mutex
    responses    []string
    responseIdx  int
    err          error
    CompleteFunc func(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    StreamFunc   func(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)

    // Call tracking
    Calls []CompletionRequest
}

// NewMockLLM creates a mock that returns a fixed response
func NewMockLLM(response string) *MockLLM {
    return &MockLLM{responses: []string{response}}
}

// WithResponses configures sequential responses
func (m *MockLLM) WithResponses(responses ...string) *MockLLM {
    m.responses = responses
    return m
}

// WithError configures the mock to always return an error
func (m *MockLLM) WithError(err error) *MockLLM {
    m.err = err
    return m
}

// Complete implements LLMClient
func (m *MockLLM) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.Calls = append(m.Calls, req)

    if m.CompleteFunc != nil {
        return m.CompleteFunc(ctx, req)
    }

    if m.err != nil {
        return nil, m.err
    }

    response := ""
    if len(m.responses) > 0 {
        response = m.responses[m.responseIdx%len(m.responses)]
        m.responseIdx++
    }

    return &CompletionResponse{
        Content:      response,
        Usage:        TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
        FinishReason: "stop",
    }, nil
}

// Stream implements LLMClient
func (m *MockLLM) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.Calls = append(m.Calls, req)

    if m.StreamFunc != nil {
        return m.StreamFunc(ctx, req)
    }

    if m.err != nil {
        return nil, m.err
    }

    ch := make(chan StreamChunk)
    go func() {
        defer close(ch)
        response := ""
        if len(m.responses) > 0 {
            response = m.responses[m.responseIdx%len(m.responses)]
            m.responseIdx++
        }
        // Send as single chunk
        ch <- StreamChunk{
            Content: response,
            Done:    true,
            Usage:   &TokenUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
        }
    }()

    return ch, nil
}
```

### Step 3: Claude CLI Client (~4 hours)

**llm/claude_cli.go**
```go
package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
    "strings"
    "time"
)

// ClaudeCLI calls the Claude CLI binary
type ClaudeCLI struct {
    path         string
    model        string
    workdir      string
    allowedTools []string
}

// ClaudeOption configures ClaudeCLI
type ClaudeOption func(*ClaudeCLI)

// NewClaudeCLI creates a new Claude CLI client
func NewClaudeCLI(opts ...ClaudeOption) *ClaudeCLI {
    c := &ClaudeCLI{
        path: "claude",  // Assume in PATH
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}

// WithClaudePath sets the path to the claude binary
func WithClaudePath(path string) ClaudeOption {
    return func(c *ClaudeCLI) { c.path = path }
}

// WithModel sets the default model
func WithModel(model string) ClaudeOption {
    return func(c *ClaudeCLI) { c.model = model }
}

// WithWorkdir sets the working directory
func WithWorkdir(dir string) ClaudeOption {
    return func(c *ClaudeCLI) { c.workdir = dir }
}

// WithAllowedTools sets allowed tools
func WithAllowedTools(tools []string) ClaudeOption {
    return func(c *ClaudeCLI) { c.allowedTools = tools }
}

// Complete implements LLMClient
func (c *ClaudeCLI) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    start := time.Now()

    args := c.buildArgs(req)
    cmd := exec.CommandContext(ctx, c.path, args...)

    if c.workdir != "" {
        cmd.Dir = c.workdir
    }

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return nil, &LLMError{
            Op:        "complete",
            Err:       fmt.Errorf("%w: %s", err, stderr.String()),
            Retryable: isRetryable(err),
        }
    }

    resp, err := c.parseResponse(stdout.Bytes())
    if err != nil {
        return nil, err
    }

    resp.Duration = time.Since(start)
    return resp, nil
}

// Stream implements LLMClient
func (c *ClaudeCLI) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
    args := append(c.buildArgs(req), "--stream")
    cmd := exec.CommandContext(ctx, c.path, args...)

    if c.workdir != "" {
        cmd.Dir = c.workdir
    }

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, err
    }

    if err := cmd.Start(); err != nil {
        return nil, err
    }

    ch := make(chan StreamChunk)
    go func() {
        defer close(ch)
        defer cmd.Wait()

        // Parse streaming output
        decoder := json.NewDecoder(stdout)
        for decoder.More() {
            var chunk StreamChunk
            if err := decoder.Decode(&chunk); err != nil {
                ch <- StreamChunk{Error: err}
                return
            }
            ch <- chunk
        }
    }()

    return ch, nil
}

func (c *ClaudeCLI) buildArgs(req CompletionRequest) []string {
    args := []string{"--print"}

    if req.SystemPrompt != "" {
        args = append(args, "--system", req.SystemPrompt)
    }

    if c.model != "" {
        args = append(args, "--model", c.model)
    }
    if req.Model != "" {
        args = append(args, "--model", req.Model)
    }

    if req.MaxTokens > 0 {
        args = append(args, "--max-tokens", fmt.Sprintf("%d", req.MaxTokens))
    }

    // Build prompt from messages
    var prompt strings.Builder
    for _, msg := range req.Messages {
        if msg.Role == RoleUser {
            prompt.WriteString(msg.Content)
            prompt.WriteString("\n")
        }
    }

    args = append(args, prompt.String())

    return args
}

func (c *ClaudeCLI) parseResponse(data []byte) (*CompletionResponse, error) {
    // Claude CLI output format parsing
    // This will need adjustment based on actual claude CLI output format
    return &CompletionResponse{
        Content:      string(data),
        FinishReason: "stop",
        Usage: TokenUsage{
            // Token counts may come from stderr or not at all
            InputTokens:  0,
            OutputTokens: 0,
            TotalTokens:  0,
        },
    }, nil
}

func isRetryable(err error) bool {
    // Check for transient errors
    return strings.Contains(err.Error(), "rate limit") ||
           strings.Contains(err.Error(), "timeout")
}
```

### Step 4: Error Types (~30 min)

**llm/errors.go**
```go
package llm

import "errors"

var (
    ErrLLMUnavailable = errors.New("LLM service unavailable")
    ErrContextTooLong = errors.New("context exceeds maximum length")
    ErrRateLimited    = errors.New("rate limited")
    ErrInvalidRequest = errors.New("invalid request")
)

// LLMError wraps errors with context
type LLMError struct {
    Op        string  // "complete", "stream"
    Err       error
    Retryable bool
}

func (e *LLMError) Error() string {
    return fmt.Sprintf("llm %s: %v", e.Op, e.Err)
}

func (e *LLMError) Unwrap() error {
    return e.Err
}
```

### Step 5: Tests (~3 hours)

**llm/mock_test.go**
```go
func TestMockLLM_SequentialResponses(t *testing.T)
func TestMockLLM_AlwaysError(t *testing.T)
func TestMockLLM_CallTracking(t *testing.T)
func TestMockLLM_CustomCompleteFunc(t *testing.T)
```

**llm/claude_cli_test.go**
```go
func TestClaudeCLI_BuildArgs(t *testing.T)
func TestClaudeCLI_ParseResponse(t *testing.T)
// Integration tests with actual binary (if available)
func TestClaudeCLI_Integration(t *testing.T)  // Skip if claude not installed
```

---

## Acceptance Criteria

```go
// Mock usage in tests
mock := llm.NewMockLLM("Generated output")

ctx := flowgraph.NewContext(context.Background(),
    flowgraph.WithLLM(mock))

result, _ := compiled.Run(ctx, state)
// Node called mock.Complete()
```

```go
// Claude CLI in production
client := llm.NewClaudeCLI(
    llm.WithModel("claude-3-opus"),
    llm.WithWorkdir("/path/to/project"),
)

resp, err := client.Complete(ctx, llm.CompletionRequest{
    SystemPrompt: "You are a helpful assistant",
    Messages: []llm.Message{
        {Role: llm.RoleUser, Content: "Hello"},
    },
})
```

---

## Test Coverage Targets

| File | Target |
|------|--------|
| llm/client.go | 100% (interface only) |
| llm/request.go | 90% |
| llm/mock.go | 95% |
| llm/claude_cli.go | 80% |
| Overall Phase 4 | 80% |

---

## Checklist

- [ ] LLMClient interface defined
- [ ] CompletionRequest/Response types
- [ ] Message, Tool, ToolCall types
- [ ] TokenUsage type
- [ ] StreamChunk type
- [ ] MockLLM with helpers
- [ ] ClaudeCLI implementation
- [ ] Error types
- [ ] Mock tests
- [ ] Claude CLI tests (with skip for CI)
- [ ] 80% coverage achieved

---

## Notes

- Claude CLI is the primary implementation for v1
- API-based clients (Anthropic API, OpenAI) can be added later
- Token tracking from Claude CLI may be limited (depends on output format)
- Streaming implementation depends on Claude CLI's streaming format
- Mock is essential for testing - nodes should always work with mock
