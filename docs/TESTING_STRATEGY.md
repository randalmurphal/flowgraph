# flowgraph Testing Strategy

## Philosophy

1. **Test behavior, not implementation** - Verify what code does, not how
2. **Fail fast, fail clearly** - Immediate failure with clear messages
3. **Isolation** - Each test independent
4. **Determinism** - Same result every time
5. **Coverage is a floor, not a ceiling** - 80% minimum, focus on critical paths

---

## Coverage Requirements

| Package | Minimum | Focus |
|---------|---------|-------|
| `flowgraph` (core) | 90% | Graph construction, execution |
| `flowgraph/checkpoint` | 85% | All store implementations |
| `flowgraph/llm` | 80% | Client implementations |

---

## Test Categories

### Unit Tests

Location: Same package, `*_test.go` files

```go
// graph_test.go
func TestGraph_AddNode(t *testing.T) {
    tests := []struct {
        name    string
        nodeID  string
        wantErr bool
        errType error
    }{
        {"valid node", "my-node", false, nil},
        {"empty id", "", true, ErrInvalidNodeID},
        {"duplicate id", "duplicate", true, ErrDuplicateNode},
        {"reserved id END", "END", true, ErrReservedNodeID},
        {"id with spaces", "my node", true, ErrInvalidNodeID},
        {"id with underscore", "my_node", false, nil},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            g := NewGraph[TestState]()

            // Add first node for duplicate test
            if tt.name == "duplicate id" {
                g.AddNode(tt.nodeID, noopNode)
            }

            g.AddNode(tt.nodeID, noopNode)
            _, err := g.Compile()

            if tt.wantErr {
                require.Error(t, err)
                if tt.errType != nil {
                    assert.ErrorIs(t, err, tt.errType)
                }
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Integration Tests

Location: `tests/integration/`

Build tag: `//go:build integration`

```go
//go:build integration

func TestCheckpointStore_Postgres(t *testing.T) {
    pg := startPostgres(t) // testcontainers
    defer pg.Terminate(context.Background())

    store := NewPostgresStore(pg.Pool())

    // Run interface compliance tests
    testCheckpointStore(t, store)
}
```

### Interface Compliance Tests

Test all implementations against interface contract:

```go
func testCheckpointStore(t *testing.T, store CheckpointStore) {
    t.Run("SaveAndLoad", func(t *testing.T) {
        runID := "test-run-1"
        nodeID := "node-a"
        data := []byte(`{"value": 42}`)

        err := store.Save(runID, nodeID, data)
        require.NoError(t, err)

        loaded, err := store.Load(runID, nodeID)
        require.NoError(t, err)
        assert.Equal(t, data, loaded)
    })

    t.Run("NotFound", func(t *testing.T) {
        _, err := store.Load("missing", "missing")
        require.Error(t, err)
    })

    t.Run("LargeState", func(t *testing.T) {
        largeData := make([]byte, 10*1024*1024) // 10MB
        rand.Read(largeData)

        err := store.Save("run", "node", largeData)
        require.NoError(t, err)

        loaded, err := store.Load("run", "node")
        require.NoError(t, err)
        assert.Equal(t, largeData, loaded)
    })

    t.Run("Concurrent", func(t *testing.T) {
        var wg sync.WaitGroup
        errCh := make(chan error, 100)

        for i := 0; i < 100; i++ {
            wg.Add(1)
            go func(i int) {
                defer wg.Done()
                runID := fmt.Sprintf("run-%d", i%10)
                nodeID := fmt.Sprintf("node-%d", i)

                if err := store.Save(runID, nodeID, []byte("data")); err != nil {
                    errCh <- err
                }
                if _, err := store.Load(runID, nodeID); err != nil {
                    errCh <- err
                }
            }(i)
        }

        wg.Wait()
        close(errCh)

        for err := range errCh {
            t.Errorf("concurrent error: %v", err)
        }
    })
}
```

---

## Graph Tests

### Construction Tests

```go
func TestGraph_AddEdge(t *testing.T) {
    tests := []struct {
        name    string
        from    string
        to      string
        setup   func(*Graph[TestState])
        wantErr bool
        errType error
    }{
        {"valid edge", "a", "b", setupTwoNodes, false, nil},
        {"missing from", "missing", "b", setupOneNode, true, ErrNodeNotFound},
        {"missing to", "a", "missing", setupOneNode, true, ErrNodeNotFound},
        {"self loop", "a", "a", setupOneNode, false, nil},
        {"edge to END", "a", END, setupOneNode, false, nil},
    }
    // ...
}
```

### Compilation Tests

```go
func TestGraph_Compile_Validation(t *testing.T) {
    tests := []struct {
        name    string
        setup   func() *Graph[TestState]
        wantErr bool
        errType error
    }{
        {
            name: "valid linear flow",
            setup: func() *Graph[TestState] {
                return NewGraph[TestState]().
                    AddNode("a", noopNode).
                    AddNode("b", noopNode).
                    AddEdge("a", "b").
                    AddEdge("b", END).
                    SetEntry("a")
            },
            wantErr: false,
        },
        {
            name: "no entry point",
            setup: func() *Graph[TestState] {
                return NewGraph[TestState]().AddNode("a", noopNode)
            },
            wantErr: true,
            errType: ErrNoEntryPoint,
        },
        {
            name: "no path to END",
            setup: func() *Graph[TestState] {
                return NewGraph[TestState]().
                    AddNode("a", noopNode).
                    AddNode("b", noopNode).
                    AddEdge("a", "b").
                    SetEntry("a")
                // b has no edge to END
            },
            wantErr: true,
            errType: ErrNoPathToEnd,
        },
    }
    // ...
}
```

### Execution Tests

```go
func TestCompiledGraph_Run_LinearFlow(t *testing.T) {
    // Setup: a -> b -> c -> END
    // Each node appends to state.Path
    g := NewGraph[TestState]().
        AddNode("a", appendNode("a")).
        AddNode("b", appendNode("b")).
        AddNode("c", appendNode("c")).
        AddEdge("a", "b").
        AddEdge("b", "c").
        AddEdge("c", END).
        SetEntry("a")

    compiled, err := g.Compile()
    require.NoError(t, err)

    result, err := compiled.Run(context.Background(), TestState{})
    require.NoError(t, err)
    assert.Equal(t, []string{"a", "b", "c"}, result.Path)
}

func TestCompiledGraph_Run_ConditionalBranching(t *testing.T) {
    tests := []struct {
        name         string
        condition    bool
        expectedPath []string
    }{
        {"takes true branch", true, []string{"start", "true-branch"}},
        {"takes false branch", false, []string{"start", "false-branch"}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            g := NewGraph[TestState]().
                AddNode("start", appendNode("start")).
                AddNode("true-branch", appendNode("true-branch")).
                AddNode("false-branch", appendNode("false-branch")).
                AddEdge("start", "router").
                AddConditionalEdge("start", func(s TestState) string {
                    if s.Condition {
                        return "true-branch"
                    }
                    return "false-branch"
                }).
                AddEdge("true-branch", END).
                AddEdge("false-branch", END).
                SetEntry("start")

            compiled, _ := g.Compile()
            result, _ := compiled.Run(context.Background(), TestState{Condition: tt.condition})
            assert.Equal(t, tt.expectedPath, result.Path)
        })
    }
}

func TestCompiledGraph_Run_Loop(t *testing.T) {
    // Counter increments each iteration, exits when > 3
    g := NewGraph[TestState]().
        AddNode("increment", func(ctx Context, s TestState) (TestState, error) {
            s.Counter++
            return s, nil
        }).
        AddConditionalEdge("increment", func(s TestState) string {
            if s.Counter > 3 {
                return END
            }
            return "increment"
        }).
        SetEntry("increment")

    compiled, _ := g.Compile()
    result, err := compiled.Run(context.Background(), TestState{})
    require.NoError(t, err)
    assert.Equal(t, 4, result.Counter)
}

func TestCompiledGraph_Run_Timeout(t *testing.T) {
    g := NewGraph[TestState]().
        AddNode("slow", func(ctx Context, s TestState) (TestState, error) {
            time.Sleep(5 * time.Second)
            return s, nil
        }).
        AddEdge("slow", END).
        SetEntry("slow")

    compiled, _ := g.Compile()

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    _, err := compiled.Run(ctx, TestState{})
    require.Error(t, err)
    assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestCompiledGraph_Run_Cancellation(t *testing.T) {
    started := make(chan struct{})
    g := NewGraph[TestState]().
        AddNode("blocking", func(ctx Context, s TestState) (TestState, error) {
            close(started)
            <-ctx.Done()
            return s, ctx.Err()
        }).
        AddEdge("blocking", END).
        SetEntry("blocking")

    compiled, _ := g.Compile()

    ctx, cancel := context.WithCancel(context.Background())

    go func() {
        <-started
        cancel()
    }()

    _, err := compiled.Run(ctx, TestState{})
    require.Error(t, err)
    assert.ErrorIs(t, err, context.Canceled)
}

func TestCompiledGraph_Run_PanicRecovery(t *testing.T) {
    g := NewGraph[TestState]().
        AddNode("panics", func(ctx Context, s TestState) (TestState, error) {
            panic("unexpected error")
        }).
        AddEdge("panics", END).
        SetEntry("panics")

    compiled, _ := g.Compile()
    _, err := compiled.Run(context.Background(), TestState{})

    require.Error(t, err)
    assert.Contains(t, err.Error(), "panic")
    assert.Contains(t, err.Error(), "unexpected error")
}
```

---

## Checkpoint Tests

```go
func TestCompiledGraph_ResumeFromCheckpoint(t *testing.T) {
    store := NewMemoryStore()
    executedNodes := []string{}

    g := NewGraph[TestState]().
        AddNode("a", func(ctx Context, s TestState) (TestState, error) {
            executedNodes = append(executedNodes, "a")
            return s, nil
        }).
        AddNode("b", func(ctx Context, s TestState) (TestState, error) {
            executedNodes = append(executedNodes, "b")
            return s, nil
        }).
        AddNode("c", func(ctx Context, s TestState) (TestState, error) {
            executedNodes = append(executedNodes, "c")
            return s, nil
        }).
        AddEdge("a", "b").
        AddEdge("b", "c").
        AddEdge("c", END).
        SetEntry("a")

    compiled, _ := g.Compile()

    // Simulate crash after node b by saving checkpoint
    state := TestState{Value: "at-b"}
    stateBytes, _ := json.Marshal(state)
    store.Save("run-1", "b", stateBytes)

    // Resume should skip a and b, execute only c
    result, err := compiled.RunWithCheckpointing(
        context.Background(),
        TestState{},
        store,
        WithRunID("run-1"),
    )
    require.NoError(t, err)
    assert.Equal(t, []string{"c"}, executedNodes)
    assert.Equal(t, "at-b", result.Value)
}
```

---

## LLM Client Tests

```go
func TestClaudeCLIClient_Complete(t *testing.T) {
    tests := []struct {
        name         string
        request      CompletionRequest
        mockOutput   string
        mockExitCode int
        wantErr      bool
    }{
        {
            name: "successful completion",
            request: CompletionRequest{
                System:   "You are helpful",
                Messages: []Message{{Role: "user", Content: "Hello"}},
            },
            mockOutput:   "Hello! How can I help?",
            mockExitCode: 0,
            wantErr:      false,
        },
        {
            name:         "rate limit error",
            request:      CompletionRequest{},
            mockOutput:   "rate_limit_error: ...",
            mockExitCode: 1,
            wantErr:      true,
        },
    }
    // Use mock binary or exec mock
}

func TestMockClient_SequentialResponses(t *testing.T) {
    mock := NewMockClient(
        MockResponse{Content: "First"},
        MockResponse{Content: "Second"},
        MockResponse{Error: errors.New("third fails")},
    )

    ctx := context.Background()

    resp1, err := mock.Complete(ctx, CompletionRequest{})
    require.NoError(t, err)
    assert.Equal(t, "First", resp1.Content)

    resp2, err := mock.Complete(ctx, CompletionRequest{})
    require.NoError(t, err)
    assert.Equal(t, "Second", resp2.Content)

    _, err = mock.Complete(ctx, CompletionRequest{})
    require.Error(t, err)
    assert.Contains(t, err.Error(), "third fails")
}
```

---

## Test Utilities

### Test State

```go
type TestState struct {
    Value     string
    Counter   int
    Path      []string
    Condition bool
}

func noopNode(ctx Context, s TestState) (TestState, error) {
    return s, nil
}

func appendNode(name string) NodeFunc[TestState] {
    return func(ctx Context, s TestState) (TestState, error) {
        s.Path = append(s.Path, name)
        return s, nil
    }
}
```

### Test Containers

```go
func startPostgres(t *testing.T) *PostgresContainer {
    ctx := context.Background()
    req := testcontainers.ContainerRequest{
        Image:        "postgres:15",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "test",
        },
        WaitingFor: wait.ForListeningPort("5432/tcp"),
    }

    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)

    t.Cleanup(func() { container.Terminate(ctx) })

    return &PostgresContainer{Container: container}
}
```

---

## Running Tests

```bash
# All tests
go test -race ./...

# Verbose
go test -race -v ./...

# Specific package
go test -race -v ./checkpoint/...

# Integration tests (requires Docker)
go test -race -tags=integration ./...

# Coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out

# Benchmark
go test -bench=. -benchmem ./...
```

---

## CI Configuration

```yaml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Test
        run: go test -race -coverprofile=coverage.out ./...

      - name: Coverage
        run: |
          go tool cover -func=coverage.out | grep total
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          if (( $(echo "$COVERAGE < 80" | bc -l) )); then
            echo "Coverage $COVERAGE% is below 80%"
            exit 1
          fi

  integration:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
          POSTGRES_DB: test
        ports:
          - 5432:5432
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test -race -tags=integration ./...
```
