# ADR-020: Streaming Support

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

Should flowgraph support streaming LLM responses? If so, how should it integrate with the graph execution model?

## Decision

**Support streaming via optional Stream method on LLMClient. Node implementation decides whether to use it.**

### LLMClient Interface (from ADR-018)

```go
type LLMClient interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

type StreamChunk struct {
    Text      string
    Done      bool
    Error     error
    TokensIn  int  // Set on final chunk
    TokensOut int  // Set on final chunk
}
```

### Usage in Nodes

Nodes choose whether to stream:

```go
// Non-streaming (most nodes)
func processNode(ctx flowgraph.Context, state State) (State, error) {
    resp, err := ctx.LLM().Complete(ctx, llm.CompletionRequest{
        Prompt: state.Prompt,
    })
    if err != nil {
        return state, err
    }
    state.Output = resp.Text
    return state, nil
}

// Streaming (when user visibility needed)
func streamingNode(ctx flowgraph.Context, state State) (State, error) {
    chunks, err := ctx.LLM().Stream(ctx, llm.CompletionRequest{
        Prompt: state.Prompt,
    })
    if err != nil {
        return state, err
    }

    var builder strings.Builder
    for chunk := range chunks {
        if chunk.Error != nil {
            return state, chunk.Error
        }
        if chunk.Done {
            state.TokensIn = chunk.TokensIn
            state.TokensOut = chunk.TokensOut
            break
        }

        builder.WriteString(chunk.Text)

        // Optional: emit progress
        if progress := ctx.ProgressEmitter(); progress != nil {
            progress.Emit(chunk.Text)
        }
    }

    state.Output = builder.String()
    return state, nil
}
```

### Progress Emission

For visibility during long-running nodes:

```go
// Context provides optional progress emitter
type Context interface {
    // ... other methods
    ProgressEmitter() ProgressEmitter  // nil if not configured
}

type ProgressEmitter interface {
    Emit(text string)
    EmitJSON(v any)
}

// Usage with run options
result, err := compiled.Run(ctx, state,
    flowgraph.WithProgress(func(nodeID string, text string) {
        fmt.Printf("[%s] %s", nodeID, text)
    }),
)
```

## Alternatives Considered

### 1. Automatic Streaming

```go
// Graph configuration
graph.SetStreaming(true)  // All LLM calls stream automatically
```

**Rejected**: Nodes may need to process response before it's complete. Let nodes choose.

### 2. Streaming State Updates

```go
// State updates stream to caller
func Run(ctx, state) <-chan StateUpdate {
    updates := make(chan StateUpdate)
    go func() {
        // Execute, send state updates as they happen
    }()
    return updates
}
```

**Rejected for v1**: Changes fundamental API. Can add later if needed.

### 3. Callback-Based Streaming

```go
type StreamCallback func(chunk string) error

func Complete(ctx context.Context, req CompletionRequest, cb StreamCallback) error
```

**Rejected**: Channel-based is more idiomatic Go, easier to compose.

### 4. No Streaming

```go
// Only Complete, no Stream method
type LLMClient interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
}
```

**Rejected**: Streaming is valuable for user experience and long operations.

## Consequences

### Positive
- **Flexible** - Nodes choose streaming behavior
- **Progressive** - Users see output as it generates
- **Compatible** - Non-streaming remains simple

### Negative
- More complex node implementations when streaming
- Channel management overhead

### Risks
- Goroutine leaks if channel not fully consumed → Mitigate: Context cancellation closes channel

---

## Implementation Details

### Channel Lifecycle

```go
func (c *ClaudeCLI) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
    chunks := make(chan StreamChunk)

    go func() {
        defer close(chunks)  // Always close when done

        cmd := exec.CommandContext(ctx, c.binaryPath, args...)
        stdout, _ := cmd.StdoutPipe()

        if err := cmd.Start(); err != nil {
            chunks <- StreamChunk{Error: err}
            return
        }

        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            select {
            case <-ctx.Done():
                return  // Context cancelled, stop
            case chunks <- StreamChunk{Text: scanner.Text()}:
            }
        }

        if err := cmd.Wait(); err != nil {
            chunks <- StreamChunk{Error: err}
            return
        }

        chunks <- StreamChunk{Done: true}
    }()

    return chunks, nil
}
```

### Consuming Streams Safely

```go
func consumeStream(ctx context.Context, chunks <-chan StreamChunk) (string, error) {
    var builder strings.Builder
    var lastErr error

    for {
        select {
        case <-ctx.Done():
            return builder.String(), ctx.Err()

        case chunk, ok := <-chunks:
            if !ok {
                // Channel closed unexpectedly
                return builder.String(), lastErr
            }

            if chunk.Error != nil {
                lastErr = chunk.Error
                continue  // May get more chunks or Done
            }

            if chunk.Done {
                return builder.String(), nil
            }

            builder.WriteString(chunk.Text)
        }
    }
}
```

---

## Usage Examples

### CLI Output During Execution

```go
result, err := compiled.Run(ctx, state,
    flowgraph.WithProgress(func(nodeID, text string) {
        // Print to terminal as LLM generates
        fmt.Print(text)
    }),
)
```

### Web UI Progress

```go
// WebSocket handler
func handleRun(ws *websocket.Conn, runRequest RunRequest) {
    result, err := compiled.Run(ctx, state,
        flowgraph.WithProgress(func(nodeID, text string) {
            ws.WriteJSON(ProgressUpdate{
                NodeID: nodeID,
                Text:   text,
            })
        }),
    )

    ws.WriteJSON(CompletionUpdate{
        Result: result,
        Error:  err,
    })
}
```

### Streaming with Validation

```go
func validatingStreamNode(ctx flowgraph.Context, state State) (State, error) {
    chunks, err := ctx.LLM().Stream(ctx, llm.CompletionRequest{
        Prompt: state.Prompt,
    })
    if err != nil {
        return state, err
    }

    var builder strings.Builder
    for chunk := range chunks {
        if chunk.Error != nil {
            return state, chunk.Error
        }
        if chunk.Done {
            break
        }

        builder.WriteString(chunk.Text)

        // Validate as we receive
        partial := builder.String()
        if containsInvalidContent(partial) {
            // Cancel and report
            return state, errors.New("invalid content detected")
        }
    }

    state.Output = builder.String()
    return state, nil
}
```

---

## Test Cases

```go
func TestStreaming_Basic(t *testing.T) {
    mock := &MockLLM{
        CompleteFunc: func(req llm.CompletionRequest) (*llm.CompletionResponse, error) {
            return &llm.CompletionResponse{Text: "hello world"}, nil
        },
    }

    chunks, err := mock.Stream(context.Background(), llm.CompletionRequest{
        Prompt: "test",
    })
    require.NoError(t, err)

    var received []string
    for chunk := range chunks {
        if chunk.Done {
            break
        }
        received = append(received, chunk.Text)
    }

    assert.NotEmpty(t, received)
}

func TestStreaming_Cancellation(t *testing.T) {
    // Slow streaming mock
    mock := &SlowStreamingMock{delay: 100 * time.Millisecond}

    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    chunks, _ := mock.Stream(ctx, llm.CompletionRequest{})

    var received int
    for range chunks {
        received++
    }

    // Should have stopped early due to cancellation
    assert.Less(t, received, 10)
}

func TestProgressEmitter(t *testing.T) {
    var progressText []string
    var mu sync.Mutex

    compiled, _ := graph.Compile()
    _, err := compiled.Run(context.Background(), state,
        flowgraph.WithProgress(func(nodeID, text string) {
            mu.Lock()
            progressText = append(progressText, text)
            mu.Unlock()
        }),
    )

    require.NoError(t, err)
    assert.NotEmpty(t, progressText)
}
```
