# LLM Package

**Claude CLI integration for flowgraph workflows.**

---

## Overview

This package provides the `Client` interface and `ClaudeCLI` implementation for LLM operations within flowgraph nodes.

---

## Key Types

| Type | Purpose |
|------|---------|
| `Client` | Interface for LLM operations (Complete, Stream) |
| `ClaudeCLI` | Claude Code CLI implementation |
| `MockClient` | Testing mock with configurable responses |
| `CompletionRequest` | Request configuration |
| `CompletionResponse` | Response with content, tokens, cost |
| `Message` | Conversation turn (role + content) |
| `TokenUsage` | Token counts including cache tokens |

---

## Claude CLI Integration

### Reference Implementation

See `~/repos/ai-devtools/ensemble/core/runner.py` for battle-tested Python patterns. Key patterns to match:

1. **Always use JSON output** - `--output-format json` for token/cost tracking
2. **Skip permissions** - `--dangerously-skip-permissions` for non-interactive
3. **Parse modelUsage** - Full per-model token breakdown from CLI response

### JSON Response Format

When using `--output-format json`, Claude CLI returns:

```json
{
  "type": "result",
  "subtype": "success",
  "is_error": false,
  "result": "The actual response content",
  "session_id": "6de4b9fa-874e-4c4a-a7a1-d552f5b774d2",
  "duration_ms": 2695,
  "num_turns": 1,
  "total_cost_usd": 0.060663,
  "usage": {
    "input_tokens": 2,
    "cache_creation_input_tokens": 8360,
    "cache_read_input_tokens": 0,
    "output_tokens": 5
  },
  "modelUsage": {
    "claude-opus-4-5-20251101": {
      "inputTokens": 2,
      "outputTokens": 5,
      "cacheReadInputTokens": 0,
      "cacheCreationInputTokens": 8360,
      "costUSD": 0.052385
    }
  }
}
```

### Available Options

| Option | CLI Flag | Purpose |
|--------|----------|---------|
| `WithModel(m)` | `--model` | Model selection |
| `WithOutputFormat(f)` | `--output-format` | text, json, stream-json |
| `WithSessionID(id)` | `--session-id` | Track sessions |
| `WithContinue()` | `--continue` | Continue last session |
| `WithResume(id)` | `--resume` | Resume specific session |
| `WithSystemPrompt(s)` | `--system-prompt` | Set system prompt |
| `WithAppendSystemPrompt(s)` | `--append-system-prompt` | Append to prompt |
| `WithAllowedTools(t)` | `--allowed-tools` | Whitelist tools |
| `WithDisallowedTools(t)` | `--disallowed-tools` | Blacklist tools |
| `WithDangerouslySkipPermissions()` | `--dangerously-skip-permissions` | Non-interactive |
| `WithMaxBudgetUSD(n)` | `--max-budget-usd` | Cap spending |
| `WithMaxTurns(n)` | `--max-turns` | Limit agentic turns |
| `WithAddDirs(dirs)` | `--add-dir` | Additional directories |

---

## Testing

Use `MockClient` for testing:

```go
// Single response
mock := llm.NewMockClient("response text")

// Multiple responses (sequential)
mock := llm.NewMockClient("").WithResponses("first", "second", "third")

// Custom handler
mock := llm.NewMockClient("").WithHandler(func(req CompletionRequest) (CompletionResponse, error) {
    return CompletionResponse{Content: "custom"}, nil
})

// Track calls
count := mock.CallCount()
lastReq := mock.LastRequest()
```

---

## Files

| File | Purpose |
|------|---------|
| `client.go` | Client interface definition |
| `request.go` | Request/response types |
| `errors.go` | Error type with Retryable flag |
| `mock.go` | MockClient for testing |
| `claude_cli.go` | ClaudeCLI implementation |

---

## Patterns

### Production Configuration

```go
client := llm.NewClaudeCLI(
    llm.WithModel("sonnet"),
    llm.WithOutputFormat(llm.OutputFormatJSON),
    llm.WithDangerouslySkipPermissions(),
    llm.WithMaxBudgetUSD(1.0),
    llm.WithMaxTurns(10),
)
```

### In a Node

```go
func myNode(ctx flowgraph.Context, s State) (State, error) {
    resp, err := ctx.LLM().Complete(ctx, llm.CompletionRequest{
        Messages: []llm.Message{
            {Role: llm.RoleUser, Content: "Process: " + s.Input},
        },
    })
    if err != nil {
        return s, err
    }
    s.Output = resp.Content
    s.TokensUsed += resp.Usage.TotalTokens
    s.Cost += resp.CostUSD
    return s, nil
}
```
