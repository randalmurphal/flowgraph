# ADR-025: Testing Philosophy

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

What testing philosophy should flowgraph follow? How should we structure tests?

## Decision

**Table-driven tests with clear separation of unit, integration, and example tests.**

### Test Categories

| Category | Location | Purpose | Coverage Target |
|----------|----------|---------|-----------------|
| Unit | `*_test.go` | Test individual functions | 90% |
| Integration | `integration_test.go` | Test component interactions | 80% |
| Examples | `example_test.go` | Demonstrate usage | N/A |
| Benchmarks | `*_bench_test.go` | Performance verification | Critical paths |

### Test Structure

```go
// Table-driven tests
func TestGraph_AddNode(t *testing.T) {
    tests := []struct {
        name    string
        nodeID  string
        fn      NodeFunc[testState]
        wantErr bool
        errMsg  string
    }{
        {
            name:    "valid node",
            nodeID:  "process",
            fn:      testNode,
            wantErr: false,
        },
        {
            name:    "empty ID panics",
            nodeID:  "",
            fn:      testNode,
            wantErr: true,  // Actually panics, tested separately
        },
        {
            name:    "nil function panics",
            nodeID:  "process",
            fn:      nil,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Test Helpers

```go
// Common test state
type testState struct {
    Input   string
    Output  string
    Counter int
}

// Common test node
func testNode(ctx Context, s testState) (testState, error) {
    s.Counter++
    return s, nil
}

// Assert helpers
func assertGraphHasNodes(t *testing.T, g *Graph[testState], nodeIDs ...string) {
    t.Helper()
    for _, id := range nodeIDs {
        _, exists := g.nodes[id]
        assert.True(t, exists, "expected node %s to exist", id)
    }
}
```

### Naming Conventions

```
TestTypeName_MethodName
TestTypeName_MethodName_Scenario
TestTypeName_MethodName_EdgeCase

Example:
TestGraph_Compile
TestGraph_Compile_NoEntryPoint
TestGraph_Run_ContextCancellation
```

## Alternatives Considered

### 1. BDD-Style Tests

```go
Describe("Graph", func() {
    Context("when adding a node", func() {
        It("should succeed with valid ID", func() {
            // ...
        })
    })
})
```

**Rejected**: Requires external framework (Ginkgo). Standard testing is sufficient.

### 2. Test Per File

```go
// graph_addnode_test.go
// graph_addedge_test.go
// graph_compile_test.go
```

**Rejected**: Too fragmented. Group related tests in fewer files.

### 3. Integration-First

```go
// Focus on end-to-end tests
func TestCompleteWorkflow(t *testing.T) {
    // Test full graph execution
}
```

**Rejected**: Harder to isolate failures. Unit tests first.

## Consequences

### Positive
- **Standard** - Uses Go's testing package
- **Readable** - Table-driven tests are self-documenting
- **Fast** - Unit tests run quickly
- **Maintainable** - Clear structure

### Negative
- Table-driven setup can be verbose
- Need discipline to maintain coverage

### Risks
- Coverage drops over time → CI enforces minimum

---

## Coverage Requirements

| Package | Minimum | Target |
|---------|---------|--------|
| flowgraph (core) | 85% | 90% |
| flowgraph/llm | 80% | 85% |
| flowgraph/checkpoint | 80% | 90% |

### Exclusions

- Generated code (protobuf, mocks)
- Main packages (CLI entry points)
- Pure I/O wrappers

---

## Test Patterns

### Pattern 1: Testing Panics

```go
func TestGraph_AddNode_EmptyID(t *testing.T) {
    assert.Panics(t, func() {
        NewGraph[testState]().AddNode("", testNode)
    })
}
```

### Pattern 2: Testing Errors

```go
func TestGraph_Compile_NoEntryPoint(t *testing.T) {
    g := NewGraph[testState]().
        AddNode("a", testNode)

    _, err := g.Compile()

    require.Error(t, err)
    assert.ErrorIs(t, err, ErrNoEntryPoint)
}
```

### Pattern 3: Testing Async Behavior

```go
func TestGraph_Run_ContextCancellation(t *testing.T) {
    slowNode := func(ctx Context, s testState) (testState, error) {
        select {
        case <-ctx.Done():
            return s, ctx.Err()
        case <-time.After(time.Hour):
            return s, nil
        }
    }

    compiled, _ := NewGraph[testState]().
        AddNode("slow", slowNode).
        AddEdge("slow", END).
        SetEntry("slow").
        Compile()

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
    defer cancel()

    _, err := compiled.Run(ctx, testState{})

    require.Error(t, err)
    assert.True(t, errors.Is(err, context.DeadlineExceeded))
}
```

### Pattern 4: Testing with Mocks

```go
func TestNode_LLMCall(t *testing.T) {
    mockLLM := &MockLLM{
        CompleteFunc: func(req CompletionRequest) (*CompletionResponse, error) {
            return &CompletionResponse{Text: "mocked response"}, nil
        },
    }

    ctx := NewMockContext().WithLLM(mockLLM)
    state := testState{Input: "test"}

    result, err := myNode(ctx, state)

    require.NoError(t, err)
    assert.Equal(t, "mocked response", result.Output)
    assert.Len(t, mockLLM.Calls, 1)
}
```

### Pattern 5: Testing Race Conditions

```go
func TestGraph_ConcurrentRuns(t *testing.T) {
    compiled, _ := buildGraph().Compile()

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            _, _ = compiled.Run(context.Background(), testState{Input: fmt.Sprintf("%d", i)})
        }(i)
    }
    wg.Wait()
}

// Run with: go test -race ./...
```

---

## Example Tests

```go
// example_test.go
func ExampleGraph_basic() {
    // Create a simple graph
    graph := NewGraph[OrderState]().
        AddNode("validate", validateOrder).
        AddNode("process", processOrder).
        AddEdge("validate", "process").
        AddEdge("process", END).
        SetEntry("validate")

    compiled, _ := graph.Compile()
    result, _ := compiled.Run(context.Background(), OrderState{
        OrderID: "123",
        Items:   []string{"book", "pen"},
    })

    fmt.Println(result.Status)
    // Output: processed
}
```

---

## CI Configuration

```yaml
# .github/workflows/test.yml
test:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: '1.22'

    - name: Run tests with race detector
      run: go test -race -coverprofile=coverage.out ./...

    - name: Check coverage
      run: |
        go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//' | \
        awk '{if ($1 < 85) exit 1}'
```

---

## Benchmarks

```go
// graph_bench_test.go
func BenchmarkGraph_Compile(b *testing.B) {
    graph := buildLargeGraph(100)  // 100 nodes

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = graph.Compile()
    }
}

func BenchmarkCompiledGraph_Run(b *testing.B) {
    compiled, _ := buildLargeGraph(10).Compile()
    state := testState{}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = compiled.Run(context.Background(), state)
    }
}
```
