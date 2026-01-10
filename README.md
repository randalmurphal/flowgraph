# flowgraph

[![Go Reference](https://pkg.go.dev/badge/github.com/randalmurphal/flowgraph.svg)](https://pkg.go.dev/github.com/randalmurphal/flowgraph)
[![Go Report Card](https://goreportcard.com/badge/github.com/randalmurphal/flowgraph)](https://goreportcard.com/report/github.com/randalmurphal/flowgraph)

**Graph-based LLM orchestration for Go.** Define workflows as directed graphs with typed state, conditional branching, checkpointing, and observability.

## Features

- **Type-safe graphs** - Generic state type with compile-time checking
- **Conditional branching** - Route execution based on state values
- **Crash recovery** - Checkpoint and resume from any failure point
- **LLM integration** - Claude CLI support with token tracking and budget controls
- **Observable** - Structured logging, OpenTelemetry metrics, distributed tracing
- **Production-ready** - 90%+ test coverage, race-condition free, well-documented

## Installation

```bash
go get github.com/randalmurphal/flowgraph
```

Requires Go 1.22 or later.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/randalmurphal/flowgraph/pkg/flowgraph"
)

type Counter struct {
    Value int
}

func increment(ctx flowgraph.Context, s Counter) (Counter, error) {
    s.Value++
    return s, nil
}

func main() {
    graph := flowgraph.NewGraph[Counter]().
        AddNode("inc1", increment).
        AddNode("inc2", increment).
        AddEdge("inc1", "inc2").
        AddEdge("inc2", flowgraph.END).
        SetEntry("inc1")

    compiled, err := graph.Compile()
    if err != nil {
        log.Fatal(err)
    }

    ctx := flowgraph.NewContext(context.Background())
    result, err := compiled.Run(ctx, Counter{Value: 0})
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Value) // 2
}
```

## Conditional Branching

Route execution based on state values:

```go
graph := flowgraph.NewGraph[ReviewState]().
    AddNode("review", reviewCode).
    AddNode("approve", approve).
    AddNode("request-changes", requestChanges).
    AddConditionalEdge("review", func(ctx flowgraph.Context, s ReviewState) string {
        if s.Score >= 80 {
            return "approve"
        }
        return "request-changes"
    }).
    AddEdge("approve", flowgraph.END).
    AddEdge("request-changes", flowgraph.END).
    SetEntry("review")
```

## Loops with Retry

Create retry patterns with conditional loops:

```go
graph := flowgraph.NewGraph[RetryState]().
    AddNode("attempt", tryOperation).
    AddNode("success", handleSuccess).
    AddConditionalEdge("attempt", func(ctx flowgraph.Context, s RetryState) string {
        if s.Success || s.Attempts >= 3 {
            return "success"
        }
        return "attempt" // Loop back
    }).
    AddEdge("success", flowgraph.END).
    SetEntry("attempt")
```

## Checkpointing

Enable crash recovery with SQLite or in-memory storage:

```go
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"

store, err := checkpoint.NewSQLiteStore("./checkpoints.db")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))

// Later: resume after crash
result, err = compiled.Resume(ctx, store, "run-123")
```

## LLM Integration

Use Claude CLI with full token tracking via context injection:

```go
import "github.com/randalmurphal/llmkit/claude"

// Context key for LLM client
type llmKey struct{}
func WithLLM(ctx context.Context, c claude.Client) context.Context {
    return context.WithValue(ctx, llmKey{}, c)
}
func LLM(ctx context.Context) claude.Client {
    if c, ok := ctx.Value(llmKey{}).(claude.Client); ok { return c }
    return nil
}

// Configure client
client := claude.NewClaudeCLI(
    claude.WithModel("sonnet"),
    claude.WithOutputFormat(claude.OutputFormatJSON),
    claude.WithDangerouslySkipPermissions(),
    claude.WithMaxBudgetUSD(1.0),
)

baseCtx := WithLLM(context.Background(), client)
ctx := flowgraph.NewContext(baseCtx)

// In a node:
func generateSpec(ctx flowgraph.Context, s State) (State, error) {
    client := LLM(ctx) // Access via context.Value
    if client == nil {
        return s, fmt.Errorf("LLM client not configured")
    }

    resp, err := client.Complete(ctx, claude.CompletionRequest{
        Messages: []claude.Message{{Role: claude.RoleUser, Content: s.Prompt}},
    })
    if err != nil {
        return s, err
    }

    s.Response = resp.Content
    s.TokensUsed = resp.Usage.TotalTokens
    s.CostUSD = resp.CostUSD
    return s, nil
}
```

## Observability

Enable logging, metrics, and tracing:

```go
import "log/slog"

logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

result, err := compiled.Run(ctx, state,
    flowgraph.WithObservabilityLogger(logger),
    flowgraph.WithMetrics(true),
    flowgraph.WithTracing(true),
    flowgraph.WithRunID("run-123"))
```

Produces:
- **Logs**: Structured JSON with run_id, node_id, duration_ms, attempt
- **Metrics**: `flowgraph.node.executions`, `flowgraph.node.latency_ms`, `flowgraph.node.errors`
- **Traces**: `flowgraph.run` > `flowgraph.node.{id}` span hierarchy

## Examples

See the [examples](./examples) directory:

| Example | Description |
|---------|-------------|
| [linear](./examples/linear) | Basic sequential execution |
| [conditional](./examples/conditional) | Branching based on state |
| [loop](./examples/loop) | Retry patterns with max attempts |
| [checkpointing](./examples/checkpointing) | Crash recovery with SQLite |
| [llm](./examples/llm) | LLM integration with Claude CLI |
| [observability](./examples/observability) | Logging, metrics, and tracing |

## Performance

Execution overhead is minimal:

| Operation | Time |
|-----------|------|
| Per-node overhead | < 1μs |
| Context creation | < 100ns |
| Checkpoint (SQLite) | < 1ms |

## Package Structure

```
github.com/randalmurphal/flowgraph/
├── pkg/flowgraph/            # Core orchestration
│   ├── checkpoint/           # Checkpoint storage
│   ├── config/               # Type-safe configuration extraction
│   ├── errors/               # Error categorization (Transient, Permanent, etc.)
│   ├── event/                # Event infrastructure (bus, router, DLQ, aggregator)
│   ├── expr/                 # Expression evaluation for conditions
│   ├── llm/                  # LLM client interface
│   │   ├── tokens/           # Token counting and budget management
│   │   ├── truncate/         # Text truncation strategies
│   │   ├── template/         # Prompt template engine
│   │   └── parser/           # LLM response parsing
│   ├── model/                # Model interfaces
│   ├── observability/        # Logging, metrics, tracing
│   ├── query/                # Read-only workflow inspection (Temporal-inspired)
│   ├── registry/             # Generic thread-safe registry
│   ├── saga/                 # Saga pattern for distributed transactions
│   ├── signal/               # Fire-and-forget signals (Temporal-inspired)
│   └── template/             # Variable expansion (${var}, $var)
├── examples/                 # Working examples
└── benchmarks/               # Performance benchmarks
```

## Documentation

- [API Reference](https://pkg.go.dev/github.com/randalmurphal/flowgraph) - godoc
- [CLAUDE.md](./CLAUDE.md) - AI-readable project reference
- [docs/](./docs) - Detailed architecture and concepts

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup and guidelines.

## License

MIT License - see [LICENSE](./LICENSE) for details.
