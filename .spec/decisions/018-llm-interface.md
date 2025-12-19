# ADR-018: LLM Client Interface

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

What interface should LLM clients implement? Need to support:
- Claude CLI
- Future: Claude API direct
- Future: Other providers (OpenAI, etc.)
- Testing with mocks

## Decision

**Simple interface with Complete and Stream methods.**

```go
// LLMClient provides LLM completion capabilities
type LLMClient interface {
    // Complete sends a prompt and waits for full response
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

    // Stream sends a prompt and returns a channel of response chunks
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

// CompletionRequest configures an LLM call
type CompletionRequest struct {
    // Required
    Prompt string

    // Optional
    SystemPrompt string
    MaxTokens    int
    Temperature  float64
    Model        string        // Provider-specific model ID
    Timeout      time.Duration // Per-request timeout

    // Files/context
    Files        []File        // Files to include in context
    WorkDir      string        // Working directory for file references
}

type File struct {
    Path    string
    Content []byte  // If set, use this instead of reading from Path
}

// CompletionResponse contains the full response
type CompletionResponse struct {
    Text       string
    TokensIn   int
    TokensOut  int
    Model      string
    Duration   time.Duration
    FinishReason string  // "end_turn", "max_tokens", etc.
}

// StreamChunk is a piece of a streaming response
type StreamChunk struct {
    Text     string
    Done     bool
    Error    error
    TokensIn int  // Only set on final chunk
    TokensOut int // Only set on final chunk
}
```

## Alternatives Considered

### 1. Separate Methods Per Feature

```go
type LLMClient interface {
    Complete(prompt string) (string, error)
    CompleteWithSystem(system, prompt string) (string, error)
    CompleteWithFiles(prompt string, files []string) (string, error)
    // ... many methods
}
```

**Rejected**: Combinatorial explosion. Single method with options is cleaner.

### 2. Functional Options

```go
type LLMClient interface {
    Complete(ctx context.Context, prompt string, opts ...Option) (string, error)
}

client.Complete(ctx, prompt,
    WithSystemPrompt(system),
    WithMaxTokens(4000),
)
```

**Rejected**: Harder to extend, options get complex. Request struct is clearer.

### 3. Provider-Specific Interfaces

```go
type ClaudeClient interface {
    Complete(req ClaudeRequest) (*ClaudeResponse, error)
}

type OpenAIClient interface {
    Complete(req OpenAIRequest) (*OpenAIResponse, error)
}
```

**Rejected**: Forces provider-specific code throughout. Common interface enables swapping.

### 4. Chat-Based Interface

```go
type Message struct {
    Role    string  // "user", "assistant", "system"
    Content string
}

type LLMClient interface {
    Chat(ctx context.Context, messages []Message) (*Message, error)
}
```

**Rejected for v1**: Over-engineered for current use cases. Can add later if needed.

## Consequences

### Positive
- **Simple** - Two methods cover all use cases
- **Extensible** - Request struct can grow without breaking interface
- **Mockable** - Easy to implement for testing
- **Provider-agnostic** - Same interface for Claude, OpenAI, etc.

### Negative
- Request struct may have fields irrelevant to some providers
- Streaming complexity (channel management)

### Risks
- Provider differences → Mitigate: Provider returns error for unsupported features

---

## Implementation: Claude CLI Client

```go
type ClaudeCLI struct {
    binaryPath string
    model      string
    timeout    time.Duration
}

func NewClaudeCLI(opts ...ClaudeCLIOption) *ClaudeCLI {
    c := &ClaudeCLI{
        binaryPath: "claude",
        model:      "claude-sonnet-4-20250514",
        timeout:    5 * time.Minute,
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}

func (c *ClaudeCLI) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    // Build command
    args := []string{"--print", "-p", req.Prompt}

    if req.SystemPrompt != "" {
        args = append(args, "--system-prompt", req.SystemPrompt)
    }
    if req.MaxTokens > 0 {
        args = append(args, "--max-tokens", strconv.Itoa(req.MaxTokens))
    }
    for _, f := range req.Files {
        args = append(args, "--file", f.Path)
    }

    // Set timeout
    timeout := c.timeout
    if req.Timeout > 0 {
        timeout = req.Timeout
    }
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    // Execute
    cmd := exec.CommandContext(ctx, c.binaryPath, args...)
    if req.WorkDir != "" {
        cmd.Dir = req.WorkDir
    }

    start := time.Now()
    output, err := cmd.Output()
    duration := time.Since(start)

    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return nil, fmt.Errorf("claude CLI timed out after %v", timeout)
        }
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            return nil, fmt.Errorf("claude CLI failed: %s", exitErr.Stderr)
        }
        return nil, fmt.Errorf("claude CLI error: %w", err)
    }

    return &CompletionResponse{
        Text:     string(output),
        Duration: duration,
        Model:    c.model,
        // Token counts would need to be parsed from output
    }, nil
}

func (c *ClaudeCLI) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
    // Claude CLI streaming implementation
    chunks := make(chan StreamChunk)

    go func() {
        defer close(chunks)

        // Build and execute command with streaming
        args := []string{"--stream", "-p", req.Prompt}
        // ... build args

        cmd := exec.CommandContext(ctx, c.binaryPath, args...)
        stdout, _ := cmd.StdoutPipe()
        cmd.Start()

        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            chunks <- StreamChunk{Text: scanner.Text()}
        }

        cmd.Wait()
        chunks <- StreamChunk{Done: true}
    }()

    return chunks, nil
}
```

---

## Implementation: Mock Client

```go
type MockLLM struct {
    // Fixed response for all calls
    Response string
    Error    error

    // Or dynamic responses
    CompleteFunc func(CompletionRequest) (*CompletionResponse, error)

    // Track calls for assertions
    Calls []CompletionRequest
    mu    sync.Mutex
}

func (m *MockLLM) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    m.mu.Lock()
    m.Calls = append(m.Calls, req)
    m.mu.Unlock()

    if m.CompleteFunc != nil {
        return m.CompleteFunc(req)
    }

    if m.Error != nil {
        return nil, m.Error
    }

    return &CompletionResponse{
        Text:      m.Response,
        TokensIn:  len(req.Prompt) / 4,  // Rough estimate
        TokensOut: len(m.Response) / 4,
        Model:     "mock",
        Duration:  10 * time.Millisecond,
    }, nil
}

func (m *MockLLM) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
    chunks := make(chan StreamChunk)

    go func() {
        defer close(chunks)

        resp, err := m.Complete(ctx, req)
        if err != nil {
            chunks <- StreamChunk{Error: err}
            return
        }

        // Simulate streaming by splitting response
        words := strings.Split(resp.Text, " ")
        for _, word := range words {
            select {
            case <-ctx.Done():
                return
            case chunks <- StreamChunk{Text: word + " "}:
            }
            time.Sleep(10 * time.Millisecond)
        }
        chunks <- StreamChunk{Done: true, TokensIn: resp.TokensIn, TokensOut: resp.TokensOut}
    }()

    return chunks, nil
}
```

---

## Usage Examples

### Basic Completion

```go
client := llm.NewClaudeCLI()

resp, err := client.Complete(ctx, llm.CompletionRequest{
    Prompt: "What is the capital of France?",
})
if err != nil {
    return err
}

fmt.Println(resp.Text)  // "Paris"
```

### With System Prompt and Files

```go
resp, err := client.Complete(ctx, llm.CompletionRequest{
    SystemPrompt: "You are a code reviewer. Be thorough but constructive.",
    Prompt:       "Review this code for issues.",
    Files: []llm.File{
        {Path: "main.go"},
        {Path: "handler.go"},
    },
    MaxTokens: 4000,
})
```

### Streaming

```go
chunks, err := client.Stream(ctx, llm.CompletionRequest{
    Prompt: "Write a poem about Go programming.",
})
if err != nil {
    return err
}

for chunk := range chunks {
    if chunk.Error != nil {
        return chunk.Error
    }
    if chunk.Done {
        fmt.Printf("\n[Tokens: in=%d, out=%d]\n", chunk.TokensIn, chunk.TokensOut)
        break
    }
    fmt.Print(chunk.Text)
}
```

### In a Node

```go
func generateSpecNode(ctx flowgraph.Context, state TicketState) (TicketState, error) {
    llm := ctx.LLM()
    if llm == nil {
        return state, errors.New("LLM client not configured")
    }

    resp, err := llm.Complete(ctx, llm.CompletionRequest{
        SystemPrompt: specGenerationSystemPrompt,
        Prompt:       fmt.Sprintf(specPromptTemplate, state.Ticket.Description),
        MaxTokens:    8000,
    })
    if err != nil {
        return state, fmt.Errorf("generate spec: %w", err)
    }

    state.Spec = resp.Text
    state.TokensUsed += resp.TokensIn + resp.TokensOut
    return state, nil
}
```

---

## Test Cases

```go
func TestLLMClient_Complete(t *testing.T) {
    mock := &MockLLM{
        Response: "The capital of France is Paris.",
    }

    resp, err := mock.Complete(context.Background(), llm.CompletionRequest{
        Prompt: "What is the capital of France?",
    })

    require.NoError(t, err)
    assert.Equal(t, "The capital of France is Paris.", resp.Text)
    assert.Len(t, mock.Calls, 1)
    assert.Equal(t, "What is the capital of France?", mock.Calls[0].Prompt)
}

func TestLLMClient_Timeout(t *testing.T) {
    client := llm.NewClaudeCLI(
        llm.WithBinary("sleep"),  // Mock with sleep command
    )

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
    defer cancel()

    _, err := client.Complete(ctx, llm.CompletionRequest{
        Prompt: "100",  // sleep 100 seconds
    })

    require.Error(t, err)
    assert.Contains(t, err.Error(), "timed out")
}
```
