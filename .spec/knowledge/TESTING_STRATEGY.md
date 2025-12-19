# flowgraph Testing Strategy

**Related ADRs**: 025-testing-philosophy, 026-mocks, 027-integration-tests

---

## Test File Organization

### Package Structure

```
pkg/flowgraph/
├── graph.go
├── graph_test.go           # Unit tests for graph.go
├── compile.go
├── compile_test.go         # Unit tests for compile.go
├── execute.go
├── execute_test.go         # Unit tests for execute.go
├── context.go
├── context_test.go
├── errors.go
├── errors_test.go
├── testutil_test.go        # Shared test helpers (internal)
├── integration_test.go     # Cross-component integration tests
│
├── checkpoint/
│   ├── store.go
│   ├── store_test.go       # Contract tests for all stores
│   ├── memory.go
│   ├── memory_test.go      # MemoryStore-specific tests
│   ├── sqlite.go
│   └── sqlite_test.go      # SQLiteStore-specific tests
│
└── llm/
    ├── client.go
    ├── client_test.go
    ├── mock.go
    ├── mock_test.go
    └── claude_cli_test.go  # Claude CLI tests (skipped in CI)
```

### File Naming Conventions

| Pattern | Purpose |
|---------|---------|
| `*_test.go` | Unit tests for corresponding source file |
| `testutil_test.go` | Shared test helpers, not exported |
| `integration_test.go` | Cross-component tests |
| `*_bench_test.go` | Benchmarks (optional separate file) |

---

## Table-Driven Test Patterns

### Standard Template

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name    string
        input   Input
        want    Output
        wantErr error
    }{
        {
            name:  "happy path",
            input: Input{Value: "valid"},
            want:  Output{Result: "processed"},
        },
        {
            name:    "invalid input",
            input:   Input{Value: ""},
            wantErr: ErrInvalidInput,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Something(tt.input)

            if tt.wantErr != nil {
                require.Error(t, err)
                assert.ErrorIs(t, err, tt.wantErr)
                return
            }

            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Graph Building Tests

```go
func TestGraph_Compile(t *testing.T) {
    noop := func(ctx Context, s State) (State, error) { return s, nil }

    tests := []struct {
        name      string
        buildFn   func() *Graph[State]
        wantErr   error
        wantNodes int
    }{
        {
            name: "valid linear graph",
            buildFn: func() *Graph[State] {
                return NewGraph[State]().
                    AddNode("a", noop).
                    AddNode("b", noop).
                    AddEdge("a", "b").
                    AddEdge("b", END).
                    SetEntry("a")
            },
            wantNodes: 2,
        },
        {
            name: "missing entry point",
            buildFn: func() *Graph[State] {
                return NewGraph[State]().
                    AddNode("a", noop).
                    AddEdge("a", END)
                    // No SetEntry
            },
            wantErr: ErrNoEntryPoint,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            graph := tt.buildFn()
            compiled, err := graph.Compile()

            if tt.wantErr != nil {
                require.Error(t, err)
                assert.ErrorIs(t, err, tt.wantErr)
                return
            }

            require.NoError(t, err)
            assert.Equal(t, tt.wantNodes, len(compiled.NodeIDs()))
        })
    }
}
```

### Execution Tests

```go
func TestRun(t *testing.T) {
    tests := []struct {
        name      string
        nodes     map[string]NodeFunc[State]
        edges     [][2]string
        entry     string
        initial   State
        want      State
        wantErr   error
    }{
        {
            name: "increments counter",
            nodes: map[string]NodeFunc[State]{
                "inc": func(ctx Context, s State) (State, error) {
                    s.Count++
                    return s, nil
                },
            },
            edges:   [][2]string{{"inc", END}},
            entry:   "inc",
            initial: State{Count: 0},
            want:    State{Count: 1},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Build graph
            graph := NewGraph[State]()
            for id, fn := range tt.nodes {
                graph.AddNode(id, fn)
            }
            for _, edge := range tt.edges {
                graph.AddEdge(edge[0], edge[1])
            }
            graph.SetEntry(tt.entry)

            compiled, err := graph.Compile()
            require.NoError(t, err)

            // Execute
            ctx := NewContext(context.Background())
            got, err := compiled.Run(ctx, tt.initial)

            if tt.wantErr != nil {
                require.Error(t, err)
                assert.ErrorIs(t, err, tt.wantErr)
                return
            }

            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

---

## Mock Patterns

### MockLLM Usage

```go
func TestNodeUsesLLM(t *testing.T) {
    // Create mock
    mock := llm.NewMockLLM("expected response")

    // Create context with mock
    ctx := flowgraph.NewContext(context.Background(),
        flowgraph.WithLLM(mock))

    // Run node
    result, err := myLLMNode(ctx, State{Input: "test"})

    // Verify
    require.NoError(t, err)
    assert.Equal(t, "expected response", result.Output)

    // Verify mock was called
    require.Len(t, mock.Calls, 1)
    assert.Equal(t, "test", mock.Calls[0].Messages[0].Content)
}
```

### MockCheckpointStore

```go
func TestCheckpointingFlow(t *testing.T) {
    store := checkpoint.NewMemoryStore()

    // Run with checkpointing
    result, err := compiled.Run(ctx, State{},
        flowgraph.WithCheckpointing(store),
        flowgraph.WithRunID("test-run"))

    require.NoError(t, err)

    // Verify checkpoints exist
    infos, _ := store.List("test-run")
    assert.Len(t, infos, expectedNodeCount)
}
```

### Custom Mock Behavior

```go
func TestLLMRetryLogic(t *testing.T) {
    callCount := 0
    mock := &llm.MockLLM{
        CompleteFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
            callCount++
            if callCount < 3 {
                return nil, errors.New("temporary error")
            }
            return &llm.CompletionResponse{Content: "success"}, nil
        },
    }

    // Test retry logic in node
    result, err := retryingNode(ctx, State{})

    require.NoError(t, err)
    assert.Equal(t, 3, callCount)
    assert.Equal(t, "success", result.Output)
}
```

---

## Integration Test Scenarios

### Full Graph Execution

```go
func TestIntegration_LinearFlow(t *testing.T) {
    // Build realistic graph
    graph := buildTicketProcessingGraph()
    compiled, err := graph.Compile()
    require.NoError(t, err)

    // Run with all features
    store := checkpoint.NewMemoryStore()
    ctx := flowgraph.NewContext(context.Background(),
        flowgraph.WithLLM(llm.NewMockLLM("generated")),
        flowgraph.WithRunID("int-test-1"))

    result, err := compiled.Run(ctx, TicketState{TicketID: "TK-123"},
        flowgraph.WithCheckpointing(store))

    require.NoError(t, err)
    assert.NotEmpty(t, result.GeneratedSpec)
    assert.NotEmpty(t, result.CreatedPR)
}
```

### Crash Recovery

```go
func TestIntegration_ResumeAfterCrash(t *testing.T) {
    store := checkpoint.NewSQLiteStore(t.TempDir() + "/test.db")
    defer store.Close()

    // Simulate partial execution
    // ... run until checkpoint at node B

    // Resume
    result, err := compiled.Resume(ctx, store, "run-123")

    require.NoError(t, err)
    // Verify only remaining nodes executed
}
```

### Concurrent Runs

```go
func TestIntegration_ConcurrentRuns(t *testing.T) {
    compiled, _ := graph.Compile()
    store := checkpoint.NewMemoryStore()

    var wg sync.WaitGroup
    errors := make(chan error, 10)

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(runID string) {
            defer wg.Done()
            ctx := flowgraph.NewContext(context.Background())
            _, err := compiled.Run(ctx, State{},
                flowgraph.WithCheckpointing(store),
                flowgraph.WithRunID(runID))
            if err != nil {
                errors <- err
            }
        }(fmt.Sprintf("run-%d", i))
    }

    wg.Wait()
    close(errors)

    for err := range errors {
        t.Errorf("concurrent run failed: %v", err)
    }
}
```

---

## Benchmark Requirements

### What to Benchmark

| Component | Benchmark | Target |
|-----------|-----------|--------|
| Graph | NewGraph() | < 1µs |
| Graph | AddNode (100 nodes) | < 100µs |
| Graph | Compile (10 nodes) | < 100µs |
| Graph | Compile (100 nodes) | < 1ms |
| Execute | Per-node overhead | < 1µs |
| Execute | 10-node linear | < 10µs |
| Checkpoint | MemoryStore Save | < 10µs |
| Checkpoint | SQLiteStore Save | < 1ms |
| Context | NewContext() | < 100ns |

### Benchmark Template

```go
func BenchmarkNewGraph(b *testing.B) {
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        _ = NewGraph[State]()
    }
}

func BenchmarkRun_Linear10(b *testing.B) {
    compiled := mustCompile(buildLinearGraph(10))
    ctx := NewContext(context.Background())
    state := State{}

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        _, _ = compiled.Run(ctx, state)
    }
}
```

---

## Coverage Enforcement

### Targets by Package

| Package | Target | Rationale |
|---------|--------|-----------|
| flowgraph (core) | 90% | Critical path, must be reliable |
| flowgraph/checkpoint | 85% | Storage is well-defined |
| flowgraph/llm | 80% | External integration |
| flowgraph/observability | 80% | Optional functionality |

### CI Configuration

```yaml
# .github/workflows/test.yml
- name: Run tests with coverage
  run: |
    go test -race -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

- name: Check coverage threshold
  run: |
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | tr -d '%')
    if (( $(echo "$COVERAGE < 85" | bc -l) )); then
      echo "Coverage $COVERAGE% is below threshold 85%"
      exit 1
    fi
```

### Excluding from Coverage

```go
// Code that shouldn't count against coverage:
// - Generated code
// - Pure type definitions
// - Main packages

//go:build ignore
// or use build tags for test utilities
```

---

## Test Helpers

### testutil_test.go

```go
package flowgraph

import (
    "context"
    "testing"
)

// State is a common test state type
type State struct {
    Count    int
    Values   []string
    Error    error
    Progress string
}

// noopNode returns state unchanged
func noopNode(ctx Context, s State) (State, error) {
    return s, nil
}

// incrementNode increments Count
func incrementNode(ctx Context, s State) (State, error) {
    s.Count++
    return s, nil
}

// failingNode always returns an error
func failingNode(err error) NodeFunc[State] {
    return func(ctx Context, s State) (State, error) {
        return s, err
    }
}

// panicNode panics with the given value
func panicNode(v any) NodeFunc[State] {
    return func(ctx Context, s State) (State, error) {
        panic(v)
    }
}

// mustCompile compiles a graph, panicking on error
func mustCompile[S any](g *Graph[S]) *CompiledGraph[S] {
    c, err := g.Compile()
    if err != nil {
        panic(err)
    }
    return c
}

// testContext creates a context for testing
func testContext(t *testing.T) Context {
    return NewContext(context.Background(),
        WithRunID(t.Name()))
}

// buildLinearGraph creates a linear graph with n nodes
func buildLinearGraph(n int) *Graph[State] {
    g := NewGraph[State]()
    for i := 0; i < n; i++ {
        id := fmt.Sprintf("node-%d", i)
        g.AddNode(id, noopNode)
        if i > 0 {
            g.AddEdge(fmt.Sprintf("node-%d", i-1), id)
        }
    }
    g.AddEdge(fmt.Sprintf("node-%d", n-1), END)
    g.SetEntry("node-0")
    return g
}
```

---

## Running Tests

### Commands

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package
go test ./pkg/flowgraph/...

# Run specific test
go test -run TestGraph_Compile ./pkg/flowgraph

# Run benchmarks
go test -bench=. ./pkg/flowgraph

# Verbose output
go test -v ./...

# Integration tests (if tagged)
go test -tags=integration ./...
```

### CI Pipeline

1. `go vet ./...` - Static analysis
2. `golangci-lint run` - Linting
3. `go test -race ./...` - Tests with race detector
4. `go test -coverprofile=...` - Coverage check
5. `go test -bench=...` - Benchmarks (optional)
