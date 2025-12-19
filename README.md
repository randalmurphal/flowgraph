# flowgraph

[![Go Reference](https://pkg.go.dev/badge/github.com/yourorg/flowgraph.svg)](https://pkg.go.dev/github.com/yourorg/flowgraph)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourorg/flowgraph)](https://goreportcard.com/report/github.com/yourorg/flowgraph)

**Graph-based LLM orchestration for Go.** Define workflows as directed graphs with typed state, conditional branching, and checkpointing.

## Features

- **Type-safe state** - Generic state type flows through the graph
- **Conditional branching** - Route based on state values
- **Checkpointing** - Resume workflows after crashes
- **Multi-model support** - Pluggable LLM client interface
- **Production-ready** - Timeouts, retries, observability

## Installation

```bash
go get github.com/yourorg/flowgraph
```

## Quick Start

```go
package main

import (
    "context"
    "github.com/yourorg/flowgraph"
)

type State struct {
    Input  string
    Output string
}

func main() {
    graph := flowgraph.NewGraph[State]().
        AddNode("process", func(ctx flowgraph.Context, s State) (State, error) {
            s.Output = "Processed: " + s.Input
            return s, nil
        }).
        AddEdge("process", flowgraph.END).
        SetEntry("process")

    compiled, _ := graph.Compile()
    result, _ := compiled.Run(context.Background(), State{Input: "hello"})

    fmt.Println(result.Output) // "Processed: hello"
}
```

## Documentation

- [CLAUDE.md](CLAUDE.md) - AI-readable project reference
- [docs/OVERVIEW.md](docs/OVERVIEW.md) - Detailed concepts
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - Design decisions
- [docs/API_REFERENCE.md](docs/API_REFERENCE.md) - Full API

## Ecosystem

flowgraph is the foundation layer of a three-layer ecosystem:

| Layer | Repo | Purpose |
|-------|------|---------|
| **flowgraph** | This repo | Graph orchestration engine |
| devflow | [github.com/yourorg/devflow](https://github.com/yourorg/devflow) | Dev workflow primitives |
| task-keeper | [github.com/yourorg/task-keeper](https://github.com/yourorg/task-keeper) | Commercial SaaS product |

## License

MIT License - see [LICENSE](LICENSE) for details.
