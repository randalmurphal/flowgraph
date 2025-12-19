# flowgraph Examples

Working examples demonstrating flowgraph features. Each example is self-contained and runnable.

## Examples

| Example | Description | Key Features |
|---------|-------------|--------------|
| [linear](./linear) | Sequential node execution | Graph building, basic execution |
| [conditional](./conditional) | Branching based on state | Router functions, conditional edges |
| [loop](./loop) | Retry patterns with exit conditions | Loops, max iterations protection |
| [checkpointing](./checkpointing) | Crash recovery and resume | SQLiteStore, Resume(), checkpoint listing |
| [llm](./llm) | LLM integration | MockClient, ClaudeCLI configuration |
| [observability](./observability) | Monitoring and debugging | Logging, metrics, tracing |

## Running Examples

```bash
cd examples/<name>
go run main.go
```

## Progression

Examples are ordered by complexity:

1. **linear** - Start here. Simplest graph with two sequential nodes.
2. **conditional** - Adds branching logic with router functions.
3. **loop** - Shows retry patterns with conditional exit.
4. **checkpointing** - Persisting and recovering from crashes.
5. **llm** - Integrating with Claude CLI for AI workflows.
6. **observability** - Adding production monitoring.

## Common Patterns

### Basic Graph Structure
```go
graph := flowgraph.NewGraph[State]().
    AddNode("step1", step1Func).
    AddNode("step2", step2Func).
    AddEdge("step1", "step2").
    AddEdge("step2", flowgraph.END).
    SetEntry("step1")

compiled, _ := graph.Compile()
result, _ := compiled.Run(ctx, initialState)
```

### Conditional Branching
```go
graph.AddConditionalEdge("decision", func(ctx flowgraph.Context, s State) string {
    if s.Condition {
        return "pathA"
    }
    return "pathB"
})
```

### With Checkpointing
```go
store, _ := checkpoint.NewSQLiteStore("./checkpoints.db")
defer store.Close()

result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithRunID("run-123"))
```

### With LLM
```go
client := llm.NewClaudeCLI(llm.WithDangerouslySkipPermissions())
ctx := flowgraph.NewContext(context.Background(), flowgraph.WithLLM(client))
```

## See Also

- [docs/OVERVIEW.md](../docs/OVERVIEW.md) - Conceptual documentation
- [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md) - Technical details
- [README.md](../README.md) - Main project README
