# ADR-019: Context Window Management

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

LLMs have limited context windows. How should flowgraph help manage context across multi-turn conversations or large inputs?

## Decision

**Context management is the responsibility of the user/devflow layer, not flowgraph core.**

### Rationale

1. **flowgraph is low-level** - It executes graphs, not LLM-specific logic
2. **Context strategies vary** - Different workflows need different approaches
3. **State contains history** - Users can track conversation in state
4. **devflow provides patterns** - Higher-level library handles this

### What flowgraph Provides

```go
// LLM response includes token counts
type CompletionResponse struct {
    Text      string
    TokensIn  int   // Tokens consumed by prompt
    TokensOut int   // Tokens in response
    // ...
}

// State can track cumulative usage
type MyState struct {
    TotalTokensIn  int
    TotalTokensOut int
    Conversation   []Message  // User manages this
}

func myNode(ctx flowgraph.Context, state MyState) (MyState, error) {
    resp, err := ctx.LLM().Complete(ctx, request)
    if err != nil {
        return state, err
    }

    state.TotalTokensIn += resp.TokensIn
    state.TotalTokensOut += resp.TokensOut
    state.Conversation = append(state.Conversation, Message{
        Role:    "assistant",
        Content: resp.Text,
    })

    return state, nil
}
```

### Helper Utilities (Optional)

flowgraph MAY provide utilities, not core features:

```go
package llmutil

// TokenCounter estimates tokens without API call
type TokenCounter interface {
    Count(text string) int
}

// Pruner removes old messages to fit context
type Pruner interface {
    Prune(messages []Message, maxTokens int) []Message
}

// Strategies
func PruneOldest(messages []Message, maxTokens int) []Message
func PruneSlidingWindow(messages []Message, maxTokens int, keepLast int) []Message
func PruneSummarize(ctx context.Context, llm LLMClient, messages []Message, maxTokens int) []Message
```

## Alternatives Considered

### 1. Automatic Context Pruning

```go
// flowgraph automatically prunes context before LLM calls
type Graph[S any] struct {
    contextStrategy ContextStrategy
}

graph.SetContextStrategy(PruneOldest{MaxTokens: 100000})
```

**Rejected**: Too opinionated. Different workflows need different approaches.

### 2. Context Window in Request

```go
type CompletionRequest struct {
    MaxContextTokens int  // flowgraph enforces this
}
```

**Rejected**: flowgraph doesn't know how to prune user's state.

### 3. Pre/Post Hooks

```go
graph.SetPreLLMHook(func(req *CompletionRequest) {
    // Prune context
})
```

**Rejected for v1**: Adds complexity. Users can do this in nodes.

## Consequences

### Positive
- **Simple core** - flowgraph stays focused on graph execution
- **Flexible** - Users choose their own context strategy
- **Explicit** - No magic, token usage is visible

### Negative
- Users must implement context management
- Risk of hitting context limits if not careful

### Risks
- Context overflow → Mitigate: Document patterns, provide utilities in devflow

---

## Recommended Patterns

### Pattern 1: Track Token Usage

```go
type ConversationState struct {
    Messages     []Message
    TokenCount   int
    MaxTokens    int
}

func chatNode(ctx flowgraph.Context, state ConversationState) (ConversationState, error) {
    // Check before calling LLM
    if state.TokenCount > state.MaxTokens*0.8 {
        state = pruneOldMessages(state)
    }

    // Build prompt from messages
    prompt := formatMessages(state.Messages)

    resp, err := ctx.LLM().Complete(ctx, llm.CompletionRequest{
        Prompt: prompt,
    })
    if err != nil {
        return state, err
    }

    state.TokenCount += resp.TokensIn + resp.TokensOut
    state.Messages = append(state.Messages, Message{
        Role:    "assistant",
        Content: resp.Text,
    })

    return state, nil
}
```

### Pattern 2: Summarize When Full

```go
func summarizeIfNeeded(ctx flowgraph.Context, state ConversationState) (ConversationState, error) {
    if state.TokenCount < state.MaxTokens*0.9 {
        return state, nil  // Not full yet
    }

    // Keep last few messages
    keep := state.Messages[len(state.Messages)-3:]

    // Summarize older messages
    older := state.Messages[:len(state.Messages)-3]
    summary, err := summarize(ctx, older)
    if err != nil {
        return state, err
    }

    state.Messages = append(
        []Message{{Role: "system", Content: "Previous conversation summary: " + summary}},
        keep...,
    )
    state.TokenCount = estimateTokens(state.Messages)

    return state, nil
}
```

### Pattern 3: Sliding Window

```go
func slidingWindow(messages []Message, maxTokens int) []Message {
    total := 0
    start := len(messages)

    // Walk backward until we hit limit
    for i := len(messages) - 1; i >= 0; i-- {
        tokens := estimateTokens(messages[i].Content)
        if total+tokens > maxTokens {
            break
        }
        total += tokens
        start = i
    }

    return messages[start:]
}
```

### Pattern 4: Important Message Preservation

```go
type Message struct {
    Role      string
    Content   string
    Important bool  // Never prune
}

func pruneWithImportant(messages []Message, maxTokens int) []Message {
    var important, regular []Message
    for _, m := range messages {
        if m.Important {
            important = append(important, m)
        } else {
            regular = append(regular, m)
        }
    }

    // Keep all important, prune regular
    importantTokens := sumTokens(important)
    remainingBudget := maxTokens - importantTokens

    pruned := slidingWindow(regular, remainingBudget)
    return append(important, pruned...)
}
```

---

## devflow Responsibility

The devflow library should provide:

```go
package devflow

// ContextManager handles context window management
type ContextManager struct {
    MaxTokens int
    Strategy  PruningStrategy
}

type PruningStrategy interface {
    Prune(messages []Message, maxTokens int) []Message
}

// Built-in strategies
type OldestFirst struct{}
type SlidingWindow struct{ KeepLast int }
type SummarizingPruner struct{ LLM LLMClient }

// Usage
cm := devflow.NewContextManager(100000, devflow.SlidingWindow{KeepLast: 5})
messages = cm.Prune(messages)
```

---

## Test Cases

```go
func TestTokenTracking(t *testing.T) {
    mock := &MockLLM{
        CompleteFunc: func(req llm.CompletionRequest) (*llm.CompletionResponse, error) {
            return &llm.CompletionResponse{
                Text:      "response",
                TokensIn:  100,
                TokensOut: 50,
            }, nil
        },
    }

    state := ConversationState{MaxTokens: 1000}

    // Simulate 5 turns
    for i := 0; i < 5; i++ {
        state, _ = chatNode(mockContext(mock), state)
    }

    assert.Equal(t, 750, state.TokenCount)  // 5 * (100 + 50)
}

func TestSlidingWindow(t *testing.T) {
    messages := []Message{
        {Content: strings.Repeat("a", 100)},  // ~25 tokens
        {Content: strings.Repeat("b", 100)},
        {Content: strings.Repeat("c", 100)},
        {Content: strings.Repeat("d", 100)},
    }

    result := slidingWindow(messages, 50)

    // Should keep last 2 messages (50 tokens)
    assert.Len(t, result, 2)
    assert.Contains(t, result[0].Content, "c")
    assert.Contains(t, result[1].Content, "d")
}
```
