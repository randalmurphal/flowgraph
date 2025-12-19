# LLM Integration Example

This example demonstrates using the LLM client interface for AI-powered nodes.

## What It Shows

- Configuring LLM client with `flowgraph.WithLLM(client)`
- Accessing LLM from context with `ctx.LLM()`
- Building completion requests with `llm.CompletionRequest`
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

1. **LLM interface**: `llm.Client` with `Complete()` and `Stream()` methods
2. **Context integration**: Pass client via `WithLLM()` option
3. **MockClient**: For testing without actual API calls
4. **ClaudeCLI**: Production client for Claude CLI

## Production Configuration

```go
// Full-featured Claude CLI client
client := llm.NewClaudeCLI(
    llm.WithModel("sonnet"),
    llm.WithOutputFormat(llm.OutputFormatJSON),
    llm.WithDangerouslySkipPermissions(),
    llm.WithMaxBudgetUSD(1.0),
    llm.WithSessionID("workflow-123"),
    llm.WithSettingSources([]string{"project", "local"}),
)
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
