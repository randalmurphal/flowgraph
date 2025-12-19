# ADR-021: Token Tracking

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should token usage be tracked across graph execution? Needed for:
- Cost monitoring
- Context window management
- Usage analytics
- Billing/quota enforcement

## Decision

**Track tokens in CompletionResponse. Aggregate in state or via hooks.**

### Token Counts in Response

```go
type CompletionResponse struct {
    Text         string
    TokensIn     int    // Prompt tokens
    TokensOut    int    // Completion tokens
    Model        string
    Duration     time.Duration
    FinishReason string
}
```

### Aggregation Options

#### Option 1: In State (User Responsibility)

```go
type MyState struct {
    // ... other fields

    // Token tracking
    TotalTokensIn  int
    TotalTokensOut int
    TokensByNode   map[string]TokenUsage
}

type TokenUsage struct {
    In  int
    Out int
}

func myNode(ctx flowgraph.Context, state MyState) (MyState, error) {
    resp, err := ctx.LLM().Complete(ctx, req)
    if err != nil {
        return state, err
    }

    // Track in state
    state.TotalTokensIn += resp.TokensIn
    state.TotalTokensOut += resp.TokensOut
    if state.TokensByNode == nil {
        state.TokensByNode = make(map[string]TokenUsage)
    }
    state.TokensByNode[ctx.NodeID()] = TokenUsage{
        In:  resp.TokensIn,
        Out: resp.TokensOut,
    }

    return state, nil
}
```

#### Option 2: Via Execution Hook

```go
result, err := compiled.Run(ctx, state,
    flowgraph.WithLLMHook(func(nodeID string, req llm.CompletionRequest, resp *llm.CompletionResponse, err error) {
        if resp != nil {
            metrics.TokensIn.WithLabelValues(nodeID).Add(float64(resp.TokensIn))
            metrics.TokensOut.WithLabelValues(nodeID).Add(float64(resp.TokensOut))
        }
    }),
)
```

#### Option 3: Wrapper Client

```go
type TokenTrackingClient struct {
    wrapped LLMClient
    usage   *UsageTracker
}

func (t *TokenTrackingClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    resp, err := t.wrapped.Complete(ctx, req)
    if err == nil {
        t.usage.Record(resp.TokensIn, resp.TokensOut)
    }
    return resp, err
}
```

## Alternatives Considered

### 1. Automatic State Tracking

```go
// flowgraph automatically adds tokens to state
type Graph[S TokenTracker] struct { ... }

type TokenTracker interface {
    AddTokens(in, out int)
}
```

**Rejected**: Forces constraint on state type. Too opinionated.

### 2. Separate Token Counter

```go
// Returned alongside result
result, tokens, err := compiled.Run(ctx, state)
// tokens.In, tokens.Out, tokens.ByNode
```

**Rejected**: Changes API, complicates return value.

### 3. Context-Based Tracking

```go
// Tokens recorded in context
type Context interface {
    RecordTokens(in, out int)
    GetTokenUsage() TokenUsage
}
```

**Rejected for v1**: Context should be immutable. Recording mutates.

## Consequences

### Positive
- **Flexible** - Multiple aggregation strategies supported
- **Accurate** - Tokens come from actual API response
- **Observable** - Easy to emit metrics

### Negative
- User must implement aggregation
- Easy to forget tracking

### Risks
- Token counts unavailable from some providers → Return 0, document

---

## Token Estimation

When exact counts unavailable (e.g., Claude CLI without --token-usage flag):

```go
package tokenutil

// Estimate tokens using character-based heuristic
// Claude: ~4 characters per token on average
func EstimateTokens(text string) int {
    return len(text) / 4
}

// More accurate: use tiktoken for OpenAI compatibility
import "github.com/pkoukk/tiktoken-go"

func CountTokensClaude(text string) int {
    // Claude uses similar tokenization to GPT
    enc, _ := tiktoken.GetEncoding("cl100k_base")
    return len(enc.Encode(text, nil, nil))
}
```

---

## Usage Examples

### Simple Tracking

```go
type SimpleState struct {
    Input     string
    Output    string
    TokensIn  int
    TokensOut int
}

func processNode(ctx flowgraph.Context, state SimpleState) (SimpleState, error) {
    resp, err := ctx.LLM().Complete(ctx, llm.CompletionRequest{
        Prompt: state.Input,
    })
    if err != nil {
        return state, err
    }

    state.Output = resp.Text
    state.TokensIn += resp.TokensIn
    state.TokensOut += resp.TokensOut
    return state, nil
}
```

### Per-Node Breakdown

```go
type DetailedState struct {
    // ... data fields

    TokenUsage map[string]NodeTokens `json:"token_usage"`
}

type NodeTokens struct {
    Calls    int `json:"calls"`
    TokensIn int `json:"tokens_in"`
    TokensOut int `json:"tokens_out"`
}

func trackingNode(nodeID string) NodeFunc[DetailedState] {
    return func(ctx flowgraph.Context, state DetailedState) (DetailedState, error) {
        resp, err := ctx.LLM().Complete(ctx, req)
        if err != nil {
            return state, err
        }

        if state.TokenUsage == nil {
            state.TokenUsage = make(map[string]NodeTokens)
        }

        existing := state.TokenUsage[nodeID]
        existing.Calls++
        existing.TokensIn += resp.TokensIn
        existing.TokensOut += resp.TokensOut
        state.TokenUsage[nodeID] = existing

        return state, nil
    }
}

// After run, inspect usage:
fmt.Printf("Token usage by node:\n")
for nodeID, usage := range result.TokenUsage {
    fmt.Printf("  %s: %d calls, %d in, %d out\n",
        nodeID, usage.Calls, usage.TokensIn, usage.TokensOut)
}
```

### Metrics Emission

```go
// With Prometheus
var (
    llmTokensIn = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "flowgraph_llm_tokens_in_total",
            Help: "Total input tokens consumed",
        },
        []string{"graph", "node", "model"},
    )
    llmTokensOut = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "flowgraph_llm_tokens_out_total",
            Help: "Total output tokens generated",
        },
        []string{"graph", "node", "model"},
    )
)

// Tracking client wrapper
type MetricsClient struct {
    wrapped   LLMClient
    graphName string
}

func (m *MetricsClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    resp, err := m.wrapped.Complete(ctx, req)
    if err == nil {
        nodeID := ctx.Value("node_id").(string)
        llmTokensIn.WithLabelValues(m.graphName, nodeID, resp.Model).Add(float64(resp.TokensIn))
        llmTokensOut.WithLabelValues(m.graphName, nodeID, resp.Model).Add(float64(resp.TokensOut))
    }
    return resp, err
}
```

### Cost Calculation

```go
type CostTracker struct {
    TokensIn  int
    TokensOut int
}

func (c *CostTracker) Cost() float64 {
    // Claude Sonnet pricing (example)
    inCost := float64(c.TokensIn) / 1_000_000 * 3.0   // $3/MTok
    outCost := float64(c.TokensOut) / 1_000_000 * 15.0 // $15/MTok
    return inCost + outCost
}

// After run
fmt.Printf("Estimated cost: $%.4f\n", state.CostTracker.Cost())
```

---

## Test Cases

```go
func TestTokenTracking_InState(t *testing.T) {
    mock := &MockLLM{
        CompleteFunc: func(req llm.CompletionRequest) (*llm.CompletionResponse, error) {
            return &llm.CompletionResponse{
                Text:      "response",
                TokensIn:  100,
                TokensOut: 50,
            }, nil
        },
    }

    state := SimpleState{Input: "test"}
    state, err := processNode(mockContext(mock), state)

    require.NoError(t, err)
    assert.Equal(t, 100, state.TokensIn)
    assert.Equal(t, 50, state.TokensOut)
}

func TestTokenTracking_Cumulative(t *testing.T) {
    mock := &MockLLM{
        CompleteFunc: func(req llm.CompletionRequest) (*llm.CompletionResponse, error) {
            return &llm.CompletionResponse{
                TokensIn:  100,
                TokensOut: 50,
            }, nil
        },
    }

    state := SimpleState{}

    // Multiple nodes
    for i := 0; i < 5; i++ {
        state, _ = processNode(mockContext(mock), state)
    }

    assert.Equal(t, 500, state.TokensIn)
    assert.Equal(t, 250, state.TokensOut)
}

func TestTokenEstimation(t *testing.T) {
    text := "Hello, world! This is a test."
    estimated := tokenutil.EstimateTokens(text)

    // ~30 characters / 4 = ~7-8 tokens
    assert.InDelta(t, 7, estimated, 2)
}
```
