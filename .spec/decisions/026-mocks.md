# ADR-026: Mock Strategy

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should mocks be created and managed? Options:
- Hand-written mocks
- Code generation (mockgen, moq)
- Interface-based dependency injection
- Fake implementations

## Decision

**Hand-written mocks for core interfaces, with helper constructors.**

### Core Mocks Provided

```go
package flowgraph_test

// MockContext implements Context for testing
type MockContext struct {
    context.Context
    logger       *slog.Logger
    llm          LLMClient
    checkpointer CheckpointStore
    runID        string
    nodeID       string
}

func NewMockContext() *MockContext {
    return &MockContext{
        Context: context.Background(),
        logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
        runID:   "test-run",
    }
}

func (m *MockContext) WithLLM(llm LLMClient) *MockContext {
    m.llm = llm
    return m
}

func (m *MockContext) WithCheckpointer(store CheckpointStore) *MockContext {
    m.checkpointer = store
    return m
}

func (m *MockContext) Logger() *slog.Logger { return m.logger }
func (m *MockContext) LLM() LLMClient { return m.llm }
func (m *MockContext) RunID() string { return m.runID }
// ... other methods
```

```go
// MockLLM implements LLMClient for testing
type MockLLM struct {
    // Fixed responses
    Response string
    Error    error

    // Dynamic behavior
    CompleteFunc func(CompletionRequest) (*CompletionResponse, error)
    StreamFunc   func(CompletionRequest) (<-chan StreamChunk, error)

    // Call tracking
    Calls []CompletionRequest
    mu    sync.Mutex
}

func (m *MockLLM) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
    m.mu.Lock()
    m.Calls = append(m.Calls, req)
    m.mu.Unlock()

    if m.CompleteFunc != nil {
        return m.CompleteFunc(req)
    }

    if m.Error != nil {
        return nil, m.Error
    }

    return &CompletionResponse{
        Text:      m.Response,
        TokensIn:  len(req.Prompt) / 4,
        TokensOut: len(m.Response) / 4,
        Model:     "mock",
    }, nil
}
```

### Fake Implementations

For stores that need real behavior:

```go
// MemoryStore is a real implementation, usable in tests
store := NewMemoryStore()
store.Save("run-1", "node-a", data)
loaded, _ := store.Load("run-1", "node-a")
```

## Alternatives Considered

### 1. Code Generation (mockgen)

```go
//go:generate mockgen -source=llm.go -destination=mock_llm.go -package=flowgraph

type LLMClient interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
}
```

**Rejected**:
- Generated mocks are verbose
- Requires mockgen installation
- Less readable than hand-written

### 2. Interface Embedding

```go
type MockLLM struct {
    LLMClient  // Embed interface
    CompleteFunc func(...) (...)
}
```

**Rejected**: Panics on unimplemented methods. Hand-written is clearer.

### 3. Test Doubles Package

```go
package testdoubles

type MockLLM struct { ... }
type FakeCheckpointStore struct { ... }
```

**Rejected for v1**: Internal package is fine. Can move if widely reused.

### 4. Recording Mocks

```go
// Automatically record all calls, replay later
mock := NewRecordingLLM(realClient)
result := RunTest(mock)
mock.SaveRecording("fixtures/test1.json")
```

**Rejected for v1**: Over-engineered. Simple mocks sufficient.

## Consequences

### Positive
- **Simple** - No code generation
- **Readable** - Mocks are clear Go code
- **Flexible** - Easy to customize per test
- **No dependencies** - No mockgen/moq tooling

### Negative
- Must maintain mocks manually
- Could get out of sync with interfaces

### Risks
- Interface changes break mocks → Tests fail, update mock

---

## Mock Patterns

### Pattern 1: Configurable Response

```go
func TestNode_LLMSuccess(t *testing.T) {
    mock := &MockLLM{Response: "expected output"}
    ctx := NewMockContext().WithLLM(mock)

    result, err := myNode(ctx, inputState)

    require.NoError(t, err)
    assert.Equal(t, "expected output", result.Output)
}
```

### Pattern 2: Configurable Error

```go
func TestNode_LLMFailure(t *testing.T) {
    mock := &MockLLM{Error: errors.New("API error")}
    ctx := NewMockContext().WithLLM(mock)

    _, err := myNode(ctx, inputState)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "API error")
}
```

### Pattern 3: Dynamic Behavior

```go
func TestNode_LLMDynamicResponse(t *testing.T) {
    callCount := 0
    mock := &MockLLM{
        CompleteFunc: func(req CompletionRequest) (*CompletionResponse, error) {
            callCount++
            if strings.Contains(req.Prompt, "error") {
                return nil, errors.New("bad prompt")
            }
            return &CompletionResponse{Text: fmt.Sprintf("response %d", callCount)}, nil
        },
    }

    // Test with different inputs
}
```

### Pattern 4: Call Verification

```go
func TestNode_LLMCallsCorrectly(t *testing.T) {
    mock := &MockLLM{Response: "ok"}
    ctx := NewMockContext().WithLLM(mock)

    _, _ = myNode(ctx, inputState)

    require.Len(t, mock.Calls, 1)
    assert.Contains(t, mock.Calls[0].Prompt, "expected text")
    assert.Equal(t, "system prompt", mock.Calls[0].SystemPrompt)
}
```

### Pattern 5: Sequence of Responses

```go
func TestNode_MultipleCallsDifferentResponses(t *testing.T) {
    responses := []string{"first", "second", "third"}
    index := 0

    mock := &MockLLM{
        CompleteFunc: func(req CompletionRequest) (*CompletionResponse, error) {
            resp := responses[index]
            index++
            return &CompletionResponse{Text: resp}, nil
        },
    }

    // Test node that makes multiple LLM calls
}
```

### Pattern 6: Delay Simulation

```go
func TestNode_LLMTimeout(t *testing.T) {
    mock := &MockLLM{
        CompleteFunc: func(req CompletionRequest) (*CompletionResponse, error) {
            time.Sleep(100 * time.Millisecond)
            return &CompletionResponse{Text: "slow"}, nil
        },
    }

    ctx := NewMockContext().WithLLM(mock)
    ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
    defer cancel()

    _, err := myNode(ctx, inputState)

    require.Error(t, err)
    assert.True(t, errors.Is(err, context.DeadlineExceeded))
}
```

---

## Provided Mocks

| Interface | Mock | Location |
|-----------|------|----------|
| Context | MockContext | flowgraph_test |
| LLMClient | MockLLM | flowgraph_test |
| CheckpointStore | MemoryStore | flowgraph (real implementation) |
| MetricsProvider | TestMetrics | flowgraph_test |

---

## Test Helper Functions

```go
// mustMarshal panics on error, for test setup
func mustMarshal(v any) []byte {
    data, err := json.Marshal(v)
    if err != nil {
        panic(err)
    }
    return data
}

// mustCompile panics on error
func mustCompile[S any](g *Graph[S]) *CompiledGraph[S] {
    compiled, err := g.Compile()
    if err != nil {
        panic(err)
    }
    return compiled
}

// buildTestGraph creates a standard test graph
func buildTestGraph() *Graph[testState] {
    return NewGraph[testState]().
        AddNode("start", testNode).
        AddNode("end", testNode).
        AddEdge("start", "end").
        AddEdge("end", END).
        SetEntry("start")
}
```

---

## Test File Organization

```
flowgraph/
├── graph.go
├── graph_test.go       # Unit tests for graph.go
├── compile.go
├── compile_test.go
├── execute.go
├── execute_test.go
├── testutil_test.go    # Shared test utilities (unexported)
├── mock_test.go        # Mock implementations (unexported)
├── integration_test.go # Cross-component tests
└── example_test.go     # Examples for documentation
```
