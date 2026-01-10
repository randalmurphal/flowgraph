# LLM Integration Example

This example demonstrates using LLM clients with flowgraph via context injection.

## What It Shows

- Using `context.WithValue` to inject LLM clients
- Defining context key and accessor functions
- Building completion requests with `claude.CompletionRequest`
- Handling responses with token tracking
- Using `MockClient` for testing

## Graph Structure

```
[question] -> generate -> [answer] -> END
```

## Running

```bash
go run main.go
```

## Expected Output

```
Question: What is 6 times 7?
Answer: The answer to your question is 42.
Tokens: 10, Model: mock

Question: What is the meaning of life?
Answer: I understand you're asking about the meaning of life.
Tokens: 10, Model: mock

Question: Can you help me debug this code?
Answer: Let me help you with that problem.
Tokens: 10, Model: mock

Total LLM calls made: 3

=== Production Configuration ===
...
```

## Key Concepts

1. **LLM interface**: `claude.Client` with `Complete()` and `Stream()` methods
2. **Context injection**: Pass client via `context.WithValue` pattern
3. **MockClient**: For testing without actual API calls
4. **ClaudeCLI**: Production client for Claude CLI

## Production Configuration

```go
import "github.com/randalmurphal/llmkit/claude"

// Context key and accessor (define once per package)
type llmKey struct{}
func WithLLM(ctx context.Context, c claude.Client) context.Context {
    return context.WithValue(ctx, llmKey{}, c)
}

// Full-featured Claude CLI client
client := claude.NewClaudeCLI(
    claude.WithModel("sonnet"),
    claude.WithOutputFormat(claude.OutputFormatJSON),
    claude.WithDangerouslySkipPermissions(),
    claude.WithMaxBudgetUSD(1.0),
)

// Inject into context
baseCtx := WithLLM(context.Background(), client)
ctx := flowgraph.NewContext(baseCtx)
```

## Available Client Options

| Option | Description |
|--------|-------------|
| `WithModel` | Set default model |
| `WithOutputFormat` | JSON for token tracking |
| `WithDangerouslySkipPermissions` | Non-interactive mode |
| `WithMaxBudgetUSD` | Spending limit |
| `WithSessionID` | Session tracking |
| `WithContinue` | Continue last session |
| `WithDisallowedTools` | Tool blacklist |
