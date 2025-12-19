# ADR-027: Integration Test Approach

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should integration tests be structured? What should they test vs unit tests?

## Decision

**Integration tests focus on component interaction, use real implementations where possible, with build tags for external dependencies.**

### Test Categories

| Category | Uses Real | Uses Mock | Build Tag |
|----------|-----------|-----------|-----------|
| Unit | - | All deps | (none) |
| Integration | Internal components | External APIs | (none) |
| External | Internal + DB | External APIs | integration |
| E2E | Everything | Nothing | e2e |

### Integration Test Structure

```go
// integration_test.go
// Tests interaction between flowgraph components

func TestIntegration_GraphWithCheckpointing(t *testing.T) {
    // Real MemoryStore, real execution
    store := NewMemoryStore()

    graph := NewGraph[testState]().
        AddNode("a", nodeA).
        AddNode("b", nodeB).
        AddEdge("a", "b").
        AddEdge("b", END).
        SetEntry("a")

    compiled, err := graph.Compile()
    require.NoError(t, err)

    // Run with checkpointing
    result, err := compiled.Run(context.Background(), testState{Input: "test"},
        WithCheckpointing(store),
        WithRunID("integration-test-1"),
    )

    require.NoError(t, err)
    assert.Equal(t, "processed", result.Output)

    // Verify checkpoints were saved
    checkpoints, err := store.List("integration-test-1")
    require.NoError(t, err)
    assert.Len(t, checkpoints, 2)  // One per node
}

func TestIntegration_ResumeFromCheckpoint(t *testing.T) {
    store := NewMemoryStore()

    // First run: save checkpoints
    compiled := buildTestGraph()
    _, _ = compiled.Run(context.Background(), testState{},
        WithCheckpointing(store),
        WithRunID("resume-test"),
    )

    // Second run: resume
    result, err := compiled.Resume(context.Background(), store, "resume-test")

    require.NoError(t, err)
    // Should skip completed nodes
}
```

### External Integration Tests

```go
//go:build integration

// sqlite_test.go
package flowgraph_test

import (
    "testing"
    _ "github.com/mattn/go-sqlite3"
)

func TestIntegration_SQLiteStore(t *testing.T) {
    store, err := NewSQLiteStore(":memory:")
    require.NoError(t, err)
    defer store.Close()

    // Test with real SQLite
    err = store.Save("run-1", "node-a", []byte("data"))
    require.NoError(t, err)

    data, err := store.Load("run-1", "node-a")
    require.NoError(t, err)
    assert.Equal(t, []byte("data"), data)
}
```

### E2E Tests

```go
//go:build e2e

// e2e_test.go
package flowgraph_test

func TestE2E_RealClaudeCLI(t *testing.T) {
    if os.Getenv("CLAUDE_CLI_AVAILABLE") != "true" {
        t.Skip("Claude CLI not available")
    }

    client := NewClaudeCLI()
    resp, err := client.Complete(context.Background(), CompletionRequest{
        Prompt: "Say 'hello world' and nothing else",
    })

    require.NoError(t, err)
    assert.Contains(t, strings.ToLower(resp.Text), "hello world")
}
```

## Alternatives Considered

### 1. All-in-One Tests

```go
// Every test can use real or mock based on env
func TestGraph(t *testing.T) {
    store := getStore()  // Returns real or mock based on env
}
```

**Rejected**: Hard to reason about what's being tested.

### 2. Separate Test Packages

```go
package integration_test
package e2e_test
```

**Rejected**: Loses access to internal types. Build tags are cleaner.

### 3. Docker-Based Integration

```go
// Spin up containers for every test
func TestWithPostgres(t *testing.T) {
    container := startPostgres(t)
    defer container.Stop()
}
```

**Rejected for v1**: Adds complexity. SQLite in-memory is sufficient.

## Consequences

### Positive
- **Clear separation** - Know what each test exercises
- **Fast CI** - Unit tests run by default
- **Comprehensive** - Can run full suite locally
- **Isolated** - External tests don't fail normal builds

### Negative
- Multiple test commands to run everything
- Build tags can be forgotten

### Risks
- Integration tests not run often → CI runs them daily

---

## Test Commands

```bash
# Unit tests (default, fast)
go test ./...

# Include integration tests
go test -tags=integration ./...

# Include E2E tests (requires real services)
go test -tags=e2e ./...

# All tests
go test -tags=integration,e2e ./...

# With race detector
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
```

---

## CI Configuration

```yaml
# .github/workflows/test.yml
jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test -race -coverprofile=coverage.out ./...

  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test -tags=integration -race ./...

  e2e:
    runs-on: ubuntu-latest
    if: github.event_name == 'schedule' || github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Install Claude CLI
        run: npm install -g @anthropic-ai/claude-cli
      - run: go test -tags=e2e ./...
        env:
          CLAUDE_CLI_AVAILABLE: 'true'
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

---

## Integration Test Scenarios

### Scenario 1: Full Workflow Execution

```go
func TestIntegration_FullWorkflow(t *testing.T) {
    // Build a realistic workflow graph
    graph := NewGraph[WorkflowState]().
        AddNode("parse", parseNode).
        AddNode("validate", validateNode).
        AddNode("process", processNode).
        AddNode("complete", completeNode).
        AddConditionalEdge("validate", validateRouter).
        AddEdge("parse", "validate").
        AddEdge("process", "complete").
        AddEdge("complete", END).
        SetEntry("parse")

    compiled, err := graph.Compile()
    require.NoError(t, err)

    // Test happy path
    result, err := compiled.Run(context.Background(), WorkflowState{
        Input: validInput,
    })

    require.NoError(t, err)
    assert.Equal(t, "completed", result.Status)
}
```

### Scenario 2: Error Recovery

```go
func TestIntegration_ErrorRecovery(t *testing.T) {
    failureCount := 0
    unreliableNode := func(ctx Context, s testState) (testState, error) {
        failureCount++
        if failureCount < 3 {
            return s, errors.New("transient error")
        }
        return s, nil
    }

    // Test with retry wrapper
    graph := NewGraph[testState]().
        AddNode("unreliable", WithRetry(unreliableNode, 3)).
        AddEdge("unreliable", END).
        SetEntry("unreliable")

    compiled, _ := graph.Compile()
    result, err := compiled.Run(context.Background(), testState{})

    require.NoError(t, err)
    assert.Equal(t, 3, failureCount)  // Failed twice, succeeded on third
}
```

### Scenario 3: Checkpoint Resume After Failure

```go
func TestIntegration_CheckpointResume(t *testing.T) {
    store := NewMemoryStore()
    runID := "resume-integration-test"

    // Node that fails on first attempt
    attempts := 0
    failOnce := func(ctx Context, s testState) (testState, error) {
        attempts++
        if attempts == 1 {
            return s, errors.New("first attempt fails")
        }
        s.Output = "success"
        return s, nil
    }

    graph := NewGraph[testState]().
        AddNode("setup", setupNode).
        AddNode("flaky", failOnce).
        AddNode("finish", finishNode).
        AddEdge("setup", "flaky").
        AddEdge("flaky", "finish").
        AddEdge("finish", END).
        SetEntry("setup")

    compiled, _ := graph.Compile()

    // First run: fails at flaky node
    _, err := compiled.Run(context.Background(), testState{},
        WithCheckpointing(store),
        WithRunID(runID),
    )
    require.Error(t, err)

    // Resume: should start at flaky node with setup's state
    result, err := compiled.Resume(context.Background(), store, runID)
    require.NoError(t, err)
    assert.Equal(t, "success", result.Output)
}
```

### Scenario 4: Concurrent Execution

```go
func TestIntegration_ConcurrentRuns(t *testing.T) {
    compiled := buildTestGraph()

    var wg sync.WaitGroup
    results := make(chan testState, 10)
    errors := make(chan error, 10)

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            result, err := compiled.Run(context.Background(), testState{
                Input: fmt.Sprintf("input-%d", i),
            })
            if err != nil {
                errors <- err
            } else {
                results <- result
            }
        }(i)
    }

    wg.Wait()
    close(results)
    close(errors)

    // All should succeed
    assert.Empty(t, errors)
    assert.Len(t, results, 10)
}
```

---

## Fixtures and Test Data

```go
// testdata/fixtures.go
package testdata

var ValidWorkflowInput = WorkflowState{
    Input:    "valid input",
    Metadata: map[string]string{"source": "test"},
}

var InvalidWorkflowInput = WorkflowState{
    Input: "",  // Will fail validation
}

// Load fixture from file
func LoadFixture(t *testing.T, name string) []byte {
    data, err := os.ReadFile(filepath.Join("testdata", name))
    require.NoError(t, err)
    return data
}
```
