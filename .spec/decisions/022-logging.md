# ADR-022: Logging Approach

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should flowgraph handle logging? Options:
- Use standard library slog
- Accept interface for user's logger
- No logging (let users handle)
- Structured vs unstructured

## Decision

**Use slog with context-based logger injection.**

### Approach

```go
// Logger access via Context
type Context interface {
    context.Context
    Logger() *slog.Logger
    // ... other methods
}

// Default: slog.Default()
// User can override via RunOption

func WithLogger(logger *slog.Logger) RunOption {
    return func(c *runConfig) {
        c.logger = logger
    }
}
```

### Logging Levels

| Level | Use Case | Example |
|-------|----------|---------|
| Debug | Internal details | "entering node", "state size" |
| Info | Significant events | "node completed", "checkpoint saved" |
| Warn | Recoverable issues | "unreachable node", "slow execution" |
| Error | Failures | "node failed", "checkpoint failed" |

### Automatic Log Enrichment

```go
func (cg *CompiledGraph[S]) executeNode(ctx Context, nodeID string, state S) (S, error) {
    // Logger automatically includes run context
    logger := ctx.Logger().With(
        "node_id", nodeID,
        "run_id", ctx.RunID(),
        "attempt", ctx.Attempt(),
    )

    logger.Debug("node starting")
    start := time.Now()

    result, err := cg.runWithRecovery(ctx, nodeID, state)
    duration := time.Since(start)

    if err != nil {
        logger.Error("node failed",
            "duration", duration,
            "error", err,
        )
    } else {
        logger.Info("node completed",
            "duration", duration,
        )
    }

    return result, err
}
```

## Alternatives Considered

### 1. Logger Interface

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    // ...
}
```

**Rejected**: slog is the Go standard. No need to abstract.

### 2. No Logging

```go
// User adds logging in their nodes
func myNode(ctx Context, state State) (State, error) {
    log.Printf("processing...")
}
```

**Rejected**: Loses graph-level visibility. Users want execution logs.

### 3. Logging Hooks

```go
graph.OnNodeStart(func(nodeID string) {
    log.Printf("starting %s", nodeID)
})
```

**Rejected**: Already have hooks (ADR-010). Logging should be built-in.

### 4. Zerolog/Zap

```go
import "github.com/rs/zerolog"
```

**Rejected**: slog is standard library, no dependency. Performance is sufficient.

## Consequences

### Positive
- **Standard** - slog is Go 1.21+ standard
- **Structured** - JSON-compatible output
- **Contextual** - Automatic enrichment with run/node info
- **Configurable** - Users can provide their own handler

### Negative
- Requires Go 1.21+
- slog is newer, less familiar to some

### Risks
- Too verbose → Users can set level

---

## Log Output Examples

### JSON Handler (Production)

```json
{"time":"2024-01-19T15:30:00Z","level":"INFO","msg":"node completed","run_id":"abc123","node_id":"generate-spec","duration":"2.5s"}
{"time":"2024-01-19T15:30:02Z","level":"INFO","msg":"node completed","run_id":"abc123","node_id":"implement","duration":"8.3s"}
{"time":"2024-01-19T15:30:05Z","level":"ERROR","msg":"node failed","run_id":"abc123","node_id":"review","duration":"3.1s","error":"LLM timeout"}
```

### Text Handler (Development)

```
2024/01/19 15:30:00 INFO node completed run_id=abc123 node_id=generate-spec duration=2.5s
2024/01/19 15:30:02 INFO node completed run_id=abc123 node_id=implement duration=8.3s
2024/01/19 15:30:05 ERROR node failed run_id=abc123 node_id=review duration=3.1s error="LLM timeout"
```

---

## Usage Examples

### Default Logging

```go
// Uses slog.Default()
result, err := compiled.Run(ctx, state)
```

### Custom Logger

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

result, err := compiled.Run(ctx, state,
    flowgraph.WithLogger(logger),
)
```

### File Logging

```go
file, _ := os.OpenFile("graph.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
logger := slog.New(slog.NewJSONHandler(file, nil))

result, err := compiled.Run(ctx, state,
    flowgraph.WithLogger(logger),
)
```

### Logging in Nodes

```go
func myNode(ctx flowgraph.Context, state State) (State, error) {
    logger := ctx.Logger()

    logger.Debug("processing input", "size", len(state.Input))

    result, err := process(state.Input)
    if err != nil {
        logger.Error("processing failed", "error", err)
        return state, err
    }

    logger.Info("processing complete", "result_size", len(result))
    state.Output = result
    return state, nil
}
```

### Suppressing Logs

```go
// For tests or when logging not wanted
logger := slog.New(slog.NewTextHandler(io.Discard, nil))
result, err := compiled.Run(ctx, state,
    flowgraph.WithLogger(logger),
)
```

---

## Standard Log Events

| Event | Level | Attributes |
|-------|-------|------------|
| graph.run.start | Info | run_id, entry_point |
| graph.run.complete | Info | run_id, duration, nodes_executed |
| graph.run.failed | Error | run_id, duration, error |
| node.start | Debug | run_id, node_id |
| node.complete | Info | run_id, node_id, duration |
| node.failed | Error | run_id, node_id, duration, error |
| node.panic | Error | run_id, node_id, panic, stack |
| checkpoint.save | Debug | run_id, node_id, size |
| checkpoint.load | Debug | run_id, node_id |
| llm.request | Debug | run_id, node_id, prompt_length |
| llm.response | Debug | run_id, node_id, tokens_in, tokens_out |

---

## Test Cases

```go
func TestLogging_NodeExecution(t *testing.T) {
    var buf bytes.Buffer
    logger := slog.New(slog.NewJSONHandler(&buf, nil))

    compiled, _ := graph.Compile()
    _, _ = compiled.Run(context.Background(), state,
        flowgraph.WithLogger(logger),
    )

    logs := buf.String()
    assert.Contains(t, logs, `"node_id":"`)
    assert.Contains(t, logs, `"msg":"node completed"`)
}

func TestLogging_NodeFailure(t *testing.T) {
    var buf bytes.Buffer
    logger := slog.New(slog.NewJSONHandler(&buf, nil))

    failingNode := func(ctx flowgraph.Context, s testState) (testState, error) {
        return s, errors.New("intentional failure")
    }

    compiled, _ := flowgraph.NewGraph[testState]().
        AddNode("fail", failingNode).
        AddEdge("fail", flowgraph.END).
        SetEntry("fail").
        Compile()

    _, _ = compiled.Run(context.Background(), testState{},
        flowgraph.WithLogger(logger),
    )

    logs := buf.String()
    assert.Contains(t, logs, `"level":"ERROR"`)
    assert.Contains(t, logs, `"msg":"node failed"`)
    assert.Contains(t, logs, `"error":"intentional failure"`)
}
```
