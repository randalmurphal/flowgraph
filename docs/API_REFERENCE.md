# flowgraph API Reference

## Graph Builder

### NewGraph

```go
func NewGraph[S any]() *Graph[S]
```

Creates a new graph builder with state type `S`.

**Example**:
```go
type MyState struct {
    Input  string
    Output string
}

graph := flowgraph.NewGraph[MyState]()
```

---

### AddNode

```go
func (g *Graph[S]) AddNode(id string, fn NodeFunc[S]) *Graph[S]
```

Adds a node to the graph.

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | `string` | Unique node identifier |
| `fn` | `NodeFunc[S]` | Function to execute |

**Returns**: `*Graph[S]` for chaining

**Errors** (at compile time):
- `ErrInvalidNodeID` - Empty ID or contains spaces
- `ErrDuplicateNode` - ID already exists
- `ErrReservedNodeID` - ID is "END" or "START"

**Example**:
```go
graph.AddNode("fetch", func(ctx flowgraph.Context, s State) (State, error) {
    s.Data, _ = fetch(ctx, s.URL)
    return s, nil
})
```

---

### AddEdge

```go
func (g *Graph[S]) AddEdge(from, to string) *Graph[S]
```

Adds an unconditional edge between nodes.

| Parameter | Type | Description |
|-----------|------|-------------|
| `from` | `string` | Source node ID |
| `to` | `string` | Target node ID (or `flowgraph.END`) |

**Example**:
```go
graph.AddEdge("fetch", "process")
graph.AddEdge("process", flowgraph.END)
```

---

### AddConditionalEdge

```go
func (g *Graph[S]) AddConditionalEdge(from string, router RouterFunc[S]) *Graph[S]
```

Adds a conditional edge that selects the next node based on state.

| Parameter | Type | Description |
|-----------|------|-------------|
| `from` | `string` | Source node ID |
| `router` | `RouterFunc[S]` | Function returning target node ID |

**Example**:
```go
graph.AddConditionalEdge("review", func(s State) string {
    if s.Approved {
        return "publish"
    }
    return "revise"
})
```

---

### SetEntry

```go
func (g *Graph[S]) SetEntry(id string) *Graph[S]
```

Sets the entry point node for the graph.

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | `string` | Entry node ID |

**Example**:
```go
graph.SetEntry("fetch")
```

---

### Compile

```go
func (g *Graph[S]) Compile() (*CompiledGraph[S], error)
```

Validates and compiles the graph for execution.

**Returns**: `(*CompiledGraph[S], error)`

**Errors**:
- `ErrNoEntryPoint` - No entry node set
- `ErrNodeNotFound` - Edge references non-existent node
- `ErrNoPathToEnd` - Node cannot reach END

**Example**:
```go
compiled, err := graph.Compile()
if err != nil {
    log.Fatal(err)
}
```

---

## Compiled Graph

### Run

```go
func (cg *CompiledGraph[S]) Run(ctx context.Context, initial S) (S, error)
```

Executes the graph with initial state.

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Execution context (cancellation, timeout) |
| `initial` | `S` | Initial state |

**Returns**: `(S, error)` - Final state and any error

**Example**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

result, err := compiled.Run(ctx, State{Input: "hello"})
```

---

### RunWithCheckpointing

```go
func (cg *CompiledGraph[S]) RunWithCheckpointing(
    ctx context.Context,
    initial S,
    store CheckpointStore,
) (S, error)
```

Executes with checkpointing after each node.

| Parameter | Type | Description |
|-----------|------|-------------|
| `ctx` | `context.Context` | Execution context |
| `initial` | `S` | Initial state |
| `store` | `CheckpointStore` | Checkpoint storage backend |

**Example**:
```go
store := flowgraph.NewPostgresStore(db)
result, err := compiled.RunWithCheckpointing(ctx, initial, store)
```

---

### RunWithOptions

```go
func (cg *CompiledGraph[S]) RunWithOptions(
    ctx context.Context,
    initial S,
    opts ...RunOption,
) (S, error)
```

Executes with configurable options.

**Options**:
```go
WithCheckpointing(store CheckpointStore)
WithRunID(id string)
WithLLMClient(client LLMClient)
WithLogger(logger *slog.Logger)
WithHooks(hooks Hooks[S])
```

**Example**:
```go
result, err := compiled.RunWithOptions(ctx, initial,
    flowgraph.WithCheckpointing(store),
    flowgraph.WithLLMClient(claude),
    flowgraph.WithLogger(logger),
)
```

---

## Context Interface

### Context

```go
type Context interface {
    context.Context

    // Logger returns the configured logger
    Logger() *slog.Logger

    // Checkpoint saves the current state
    Checkpoint(state any) error

    // LLM returns the LLM client
    LLM() LLMClient
}
```

**Usage in nodes**:
```go
func myNode(ctx flowgraph.Context, state State) (State, error) {
    ctx.Logger().Info("processing", "id", state.ID)

    response, err := ctx.LLM().Complete(ctx, flowgraph.CompletionRequest{
        System: "You are helpful",
        Messages: []flowgraph.Message{
            {Role: "user", Content: state.Input},
        },
    })
    if err != nil {
        return state, err
    }

    state.Output = response.Content
    return state, nil
}
```

---

## Checkpoint Stores

### CheckpointStore Interface

```go
type CheckpointStore interface {
    Save(runID string, nodeID string, state []byte) error
    Load(runID string, nodeID string) ([]byte, error)
    List(runID string) ([]Checkpoint, error)
}
```

### NewMemoryStore

```go
func NewMemoryStore() *MemoryStore
```

Creates an in-memory checkpoint store. Use for testing.

### NewSQLiteStore

```go
func NewSQLiteStore(path string) (*SQLiteStore, error)
```

Creates a SQLite-backed checkpoint store.

| Parameter | Type | Description |
|-----------|------|-------------|
| `path` | `string` | Database file path |

### NewPostgresStore

```go
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore
```

Creates a Postgres-backed checkpoint store.

| Parameter | Type | Description |
|-----------|------|-------------|
| `pool` | `*pgxpool.Pool` | Connection pool |

---

## LLM Clients

### LLMClient Interface

```go
type LLMClient interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
```

### CompletionRequest

```go
type CompletionRequest struct {
    Model         string        // Model identifier
    System        string        // System prompt
    Messages      []Message     // Conversation messages
    MaxTokens     int           // Max response tokens
    Temperature   float64       // Sampling temperature
    Tools         []Tool        // Available tools
    ContextLimit  int           // Max context tokens
    PruneStrategy PruneStrategy // How to handle overflow
}
```

### CompletionResponse

```go
type CompletionResponse struct {
    Content    string // Response text
    TokensIn   int    // Input tokens used
    TokensOut  int    // Output tokens generated
    StopReason string // Why generation stopped
}
```

### NewClaudeCLIClient

```go
func NewClaudeCLIClient(opts ...ClaudeOption) *ClaudeCLIClient
```

Creates a Claude CLI client.

**Options**:
```go
WithClaudeBinary(path string)
WithClaudeModel(model string)
WithClaudeTimeout(duration time.Duration)
```

**Example**:
```go
client := flowgraph.NewClaudeCLIClient(
    flowgraph.WithClaudeTimeout(5*time.Minute),
)
```

### NewMockClient

```go
func NewMockClient(responses ...MockResponse) *MockClient
```

Creates a mock client for testing.

**Example**:
```go
mock := flowgraph.NewMockClient(
    flowgraph.MockResponse{Content: "Hello!"},
    flowgraph.MockResponse{Content: "Goodbye!"},
)
```

---

## Error Types

### Sentinel Errors

```go
var (
    ErrInvalidNodeID  = errors.New("invalid node ID")
    ErrDuplicateNode  = errors.New("duplicate node")
    ErrReservedNodeID = errors.New("reserved node ID")
    ErrNodeNotFound   = errors.New("node not found")
    ErrNoEntryPoint   = errors.New("no entry point set")
    ErrNoPathToEnd    = errors.New("no path to END")
    ErrNodeExecution  = errors.New("node execution failed")
)
```

### Error Checking

```go
if errors.Is(err, flowgraph.ErrNodeNotFound) {
    // Handle missing node
}

var execErr *flowgraph.NodeExecutionError
if errors.As(err, &execErr) {
    fmt.Printf("Node %s failed: %v\n", execErr.NodeID, execErr.Cause)
}
```

---

## Constants

```go
const (
    END = "__END__" // Terminal node identifier
)
```

---

## Hooks (Future)

```go
type Hooks[S any] struct {
    BeforeNode func(nodeID string, state S)
    AfterNode  func(nodeID string, state S, err error)
    OnError    func(nodeID string, err error) error
}
```

---

## Complete Example

```go
package main

import (
    "context"
    "log/slog"
    "time"

    "github.com/yourorg/flowgraph"
)

type State struct {
    Query    string
    Response string
    Approved bool
}

func main() {
    // Build graph
    graph := flowgraph.NewGraph[State]().
        AddNode("generate", generateNode).
        AddNode("review", reviewNode).
        AddNode("revise", reviseNode).
        AddNode("publish", publishNode).
        AddEdge("generate", "review").
        AddConditionalEdge("review", func(s State) string {
            if s.Approved {
                return "publish"
            }
            return "revise"
        }).
        AddEdge("revise", "review").
        AddEdge("publish", flowgraph.END).
        SetEntry("generate")

    // Compile
    compiled, err := graph.Compile()
    if err != nil {
        slog.Error("compile failed", "error", err)
        return
    }

    // Execute
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()

    store := flowgraph.NewSQLiteStore("checkpoints.db")
    result, err := compiled.RunWithCheckpointing(ctx, State{Query: "Hello"}, store)
    if err != nil {
        slog.Error("execution failed", "error", err)
        return
    }

    slog.Info("completed", "response", result.Response)
}

func generateNode(ctx flowgraph.Context, s State) (State, error) {
    resp, err := ctx.LLM().Complete(ctx, flowgraph.CompletionRequest{
        Messages: []flowgraph.Message{{Role: "user", Content: s.Query}},
    })
    if err != nil {
        return s, err
    }
    s.Response = resp.Content
    return s, nil
}

func reviewNode(ctx flowgraph.Context, s State) (State, error) {
    // Auto-approve for demo
    s.Approved = len(s.Response) > 10
    return s, nil
}

func reviseNode(ctx flowgraph.Context, s State) (State, error) {
    s.Response = s.Response + " (revised)"
    return s, nil
}

func publishNode(ctx flowgraph.Context, s State) (State, error) {
    ctx.Logger().Info("publishing", "response", s.Response)
    return s, nil
}
```
