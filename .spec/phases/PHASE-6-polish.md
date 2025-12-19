# Phase 6: Polish & Documentation

**Status**: Blocked (Depends on All Previous Phases)
**Estimated Effort**: 2-3 days
**Dependencies**: Phases 1-5 Complete

---

## Goal

Production readiness: comprehensive documentation, examples, benchmarks, and final polish.

---

## Files to Create/Modify

```
flowgraph/
├── README.md                    # Project README
├── CONTRIBUTING.md              # Contribution guide
├── SECURITY.md                  # Security policy
├── CHANGELOG.md                 # Version history
├── doc.go                       # Package documentation
├── examples/
│   ├── linear/main.go           # Simple linear flow
│   ├── conditional/main.go      # Branching example
│   ├── loop/main.go             # Retry/loop example
│   ├── checkpointing/main.go    # Checkpoint/resume example
│   └── llm/main.go              # LLM integration example
├── benchmarks/
│   ├── graph_test.go            # Graph construction benchmarks
│   ├── execute_test.go          # Execution benchmarks
│   └── checkpoint_test.go       # Checkpoint benchmarks
└── pkg/flowgraph/
    └── *.go                     # Godoc improvements
```

---

## Implementation Order

### Step 1: Package Documentation (~2 hours)

**doc.go**
```go
/*
Package flowgraph provides graph-based orchestration for LLM workflows.

# Overview

flowgraph is a Go library for building and executing directed graphs
where nodes perform work and edges define flow. It's designed for
orchestrating LLM-powered workflows with features like checkpointing,
conditional branching, and crash recovery.

# Basic Usage

Create a graph with nodes and edges, then compile and run:

    type State struct {
        Input  string
        Output string
    }

    graph := flowgraph.NewGraph[State]().
        AddNode("process", processNode).
        AddEdge("process", flowgraph.END).
        SetEntry("process")

    compiled, err := graph.Compile()
    if err != nil {
        log.Fatal(err)
    }

    ctx := flowgraph.NewContext(context.Background())
    result, err := compiled.Run(ctx, State{Input: "hello"})

# Conditional Branching

Use conditional edges for decision points:

    graph.AddConditionalEdge("review", func(ctx flowgraph.Context, s State) string {
        if s.Approved {
            return "create-pr"
        }
        return "fix-issues"
    })

# Checkpointing

Enable crash recovery with checkpointing:

    store := checkpoint.NewSQLiteStore("./checkpoints.db")
    defer store.Close()

    result, err := compiled.Run(ctx, state,
        flowgraph.WithCheckpointing(store),
        flowgraph.WithRunID("run-123"))

    // Resume after crash
    result, err = compiled.Resume(ctx, store, "run-123")

# LLM Integration

Use the LLM client interface for AI calls:

    func generateSpec(ctx flowgraph.Context, s State) (State, error) {
        resp, err := ctx.LLM().Complete(ctx, llm.CompletionRequest{
            Messages: []llm.Message{{Role: llm.RoleUser, Content: s.Input}},
        })
        if err != nil {
            return s, err
        }
        s.Output = resp.Content
        return s, nil
    }

# Error Handling

Errors include context about which node failed:

    result, err := compiled.Run(ctx, state)
    var nodeErr *flowgraph.NodeError
    if errors.As(err, &nodeErr) {
        log.Printf("Node %s failed: %v", nodeErr.NodeID, nodeErr.Err)
    }

See the examples directory for complete working examples.
*/
package flowgraph
```

### Step 2: README (~2 hours)

**README.md**
```markdown
# flowgraph

[![Go Reference](https://pkg.go.dev/badge/github.com/yourorg/flowgraph.svg)](https://pkg.go.dev/github.com/yourorg/flowgraph)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourorg/flowgraph)](https://goreportcard.com/report/github.com/yourorg/flowgraph)
[![Coverage](https://codecov.io/gh/yourorg/flowgraph/branch/main/graph/badge.svg)](https://codecov.io/gh/yourorg/flowgraph)

Graph-based orchestration for LLM workflows in Go.

## Features

- **Type-safe graphs** - Generic state type with compile-time checking
- **Conditional branching** - Route based on state
- **Crash recovery** - Checkpoint and resume from failure
- **LLM integration** - Claude CLI support out of the box
- **Observable** - Structured logging, metrics, tracing
- **Tested** - 85%+ coverage, production-ready

## Installation

```bash
go get github.com/yourorg/flowgraph
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "github.com/yourorg/flowgraph"
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

    compiled, _ := graph.Compile()
    ctx := flowgraph.NewContext(context.Background())

    result, _ := compiled.Run(ctx, Counter{Value: 0})
    fmt.Println(result.Value) // 2
}
```

## Examples

See the [examples](./examples) directory:

- [Linear flow](./examples/linear) - Basic sequential execution
- [Conditional](./examples/conditional) - Branching based on state
- [Loop](./examples/loop) - Retry patterns
- [Checkpointing](./examples/checkpointing) - Crash recovery
- [LLM](./examples/llm) - Claude CLI integration

## Documentation

- [API Reference](https://pkg.go.dev/github.com/yourorg/flowgraph)
- [Architecture](./docs/ARCHITECTURE.md)
- [Testing Guide](./docs/TESTING.md)

## Performance

Execution overhead is minimal:

| Operation | Time |
|-----------|------|
| Per-node overhead | < 1µs |
| Context creation | < 100ns |
| Checkpoint (SQLite) | < 1ms |

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT License - see [LICENSE](./LICENSE).
```

### Step 3: Examples (~3 hours)

**examples/linear/main.go**
```go
// Example: Linear flow execution
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/yourorg/flowgraph"
)

type State struct {
    Input    string
    Step1    string
    Step2    string
    Step3    string
}

func step1(ctx flowgraph.Context, s State) (State, error) {
    s.Step1 = "Processed: " + s.Input
    return s, nil
}

func step2(ctx flowgraph.Context, s State) (State, error) {
    s.Step2 = "Validated: " + s.Step1
    return s, nil
}

func step3(ctx flowgraph.Context, s State) (State, error) {
    s.Step3 = "Completed: " + s.Step2
    return s, nil
}

func main() {
    graph := flowgraph.NewGraph[State]().
        AddNode("step1", step1).
        AddNode("step2", step2).
        AddNode("step3", step3).
        AddEdge("step1", "step2").
        AddEdge("step2", "step3").
        AddEdge("step3", flowgraph.END).
        SetEntry("step1")

    compiled, err := graph.Compile()
    if err != nil {
        log.Fatal(err)
    }

    ctx := flowgraph.NewContext(context.Background())
    result, err := compiled.Run(ctx, State{Input: "hello"})
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Final state: %+v\n", result)
}
```

**examples/conditional/main.go**
```go
// Example: Conditional branching
```

**examples/loop/main.go**
```go
// Example: Retry loop pattern
```

**examples/checkpointing/main.go**
```go
// Example: Checkpoint and resume
```

**examples/llm/main.go**
```go
// Example: LLM integration
```

### Step 4: Benchmarks (~2 hours)

**benchmarks/graph_test.go**
```go
package benchmarks

import (
    "testing"
    "github.com/yourorg/flowgraph"
)

func BenchmarkNewGraph(b *testing.B) {
    for i := 0; i < b.N; i++ {
        flowgraph.NewGraph[State]()
    }
}

func BenchmarkAddNode(b *testing.B) {
    graph := flowgraph.NewGraph[State]()
    node := func(ctx flowgraph.Context, s State) (State, error) { return s, nil }
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        graph = flowgraph.NewGraph[State]()
        for j := 0; j < 100; j++ {
            graph.AddNode(fmt.Sprintf("node-%d", j), node)
        }
    }
}

func BenchmarkCompile_10Nodes(b *testing.B) {
    graph := buildLinearGraph(10)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        graph.Compile()
    }
}

func BenchmarkCompile_100Nodes(b *testing.B) {
    graph := buildLinearGraph(100)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        graph.Compile()
    }
}
```

**benchmarks/execute_test.go**
```go
func BenchmarkRun_Linear10(b *testing.B) {
    compiled := mustCompile(buildLinearGraph(10))
    ctx := flowgraph.NewContext(context.Background())
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        compiled.Run(ctx, State{})
    }
}

func BenchmarkRun_Conditional(b *testing.B) {
    // Branching graph
}

func BenchmarkRun_Loop10Iterations(b *testing.B) {
    // Loop that runs 10 times
}
```

### Step 5: Contributing Guide (~1 hour)

**CONTRIBUTING.md**
```markdown
# Contributing to flowgraph

## Development Setup

1. Clone the repository
2. Install Go 1.22+
3. Run tests: `go test -race ./...`

## Code Style

- Run `gofmt -s -w .` before committing
- Run `golangci-lint run` for linting
- All public APIs must have godoc

## Testing

- Write table-driven tests
- Target 90% coverage for core packages
- Include benchmarks for performance-critical code

## Pull Requests

1. Fork and create a feature branch
2. Write tests for new functionality
3. Update documentation if needed
4. Submit PR with clear description

## Reporting Issues

Use GitHub Issues. Include:
- Go version
- flowgraph version
- Minimal reproduction code
- Expected vs actual behavior
```

### Step 6: Godoc Review (~2 hours)

Review and improve all public API documentation:

- Every public type has a doc comment
- Every public function has a doc comment
- Examples included where helpful
- Error conditions documented
- Thread safety noted

---

## Acceptance Criteria

- [ ] All examples compile and run
- [ ] README is complete and accurate
- [ ] Godoc renders correctly on pkg.go.dev
- [ ] Benchmarks pass and show reasonable performance
- [ ] No linter warnings
- [ ] No race conditions in any tests

---

## Checklist

- [ ] doc.go package documentation
- [ ] README.md
- [ ] CONTRIBUTING.md
- [ ] SECURITY.md
- [ ] CHANGELOG.md
- [ ] examples/linear
- [ ] examples/conditional
- [ ] examples/loop
- [ ] examples/checkpointing
- [ ] examples/llm
- [ ] benchmarks/graph_test.go
- [ ] benchmarks/execute_test.go
- [ ] benchmarks/checkpoint_test.go
- [ ] All godoc reviewed
- [ ] golangci-lint clean
- [ ] go vet clean
- [ ] Examples tested

---

## Quality Gates

Before v1.0 release:

- [ ] 85%+ overall test coverage
- [ ] All examples work
- [ ] Documentation complete
- [ ] No known bugs
- [ ] Performance benchmarks documented
- [ ] Security review complete
- [ ] License file present

---

## Notes

- Examples should be copy-pasteable
- README should get users started in < 5 minutes
- Benchmarks establish baseline for future optimization
- godoc is the primary API documentation
