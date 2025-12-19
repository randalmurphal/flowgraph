# flowgraph Architecture

## System Design

### Core Components

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Code                                 │
│  graph := NewGraph[S]().AddNode(...).AddEdge(...).SetEntry(...) │
└────────────────────────────────┬────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Graph[S any]                                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │   nodes     │  │   edges     │  │  entryNode  │             │
│  │ map[string] │  │  []Edge     │  │   string    │             │
│  │  NodeFunc   │  │             │  │             │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└────────────────────────────────┬────────────────────────────────┘
                                 │ Compile()
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                    CompiledGraph[S]                              │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                   Execution Engine                       │    │
│  │  • Node ordering (topological when possible)            │    │
│  │  • Edge resolution (unconditional + conditional)        │    │
│  │  • State propagation                                    │    │
│  │  • Error handling + panic recovery                      │    │
│  └─────────────────────────────────────────────────────────┘    │
└────────────────────────────────┬────────────────────────────────┘
                                 │ Run(ctx, state)
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Execution                                  │
│  ┌────────┐    ┌────────┐    ┌────────┐    ┌─────┐             │
│  │ Node A │───▶│ Node B │───▶│ Node C │───▶│ END │             │
│  └────────┘    └────────┘    └────────┘    └─────┘             │
│       │             │             │                              │
│       ▼             ▼             ▼                              │
│  ┌─────────────────────────────────────────────────┐            │
│  │              CheckpointStore                     │            │
│  │  Save state after each node for recovery        │            │
│  └─────────────────────────────────────────────────┘            │
└─────────────────────────────────────────────────────────────────┘
```

---

## Type Definitions

### Graph Types

```go
// Graph is the builder for defining workflows
type Graph[S any] struct {
    nodes     map[string]Node[S]
    edges     []Edge
    entryNode string
    validated bool
}

// Node wraps a NodeFunc with metadata
type Node[S any] struct {
    ID   string
    Func NodeFunc[S]
}

// NodeFunc is the signature for node implementations
type NodeFunc[S any] func(ctx Context, state S) (S, error)

// Edge connects two nodes
type Edge struct {
    From      string
    To        string
    Condition *RouterFunc // nil for unconditional
}

// RouterFunc selects next node based on state
type RouterFunc[S any] func(state S) string

// CompiledGraph is ready for execution
type CompiledGraph[S any] struct {
    nodes      map[string]Node[S]
    edges      map[string][]Edge  // from -> edges
    entryNode  string
    reachable  map[string]bool    // for validation warnings
}
```

### Context Types

```go
// Context provides resources to nodes
type Context interface {
    context.Context
    Logger() *slog.Logger
    Checkpoint(state any) error
    LLM() LLMClient
}

// executionContext implements Context
type executionContext struct {
    context.Context
    logger *slog.Logger
    store  CheckpointStore
    runID  string
    nodeID string
    llm    LLMClient
}
```

### Checkpoint Types

```go
// CheckpointStore persists state snapshots
type CheckpointStore interface {
    Save(runID string, nodeID string, state []byte) error
    Load(runID string, nodeID string) ([]byte, error)
    List(runID string) ([]Checkpoint, error)
}

// Checkpoint represents a saved state
type Checkpoint struct {
    RunID     string
    NodeID    string
    State     []byte
    CreatedAt time.Time
}
```

### LLM Types

```go
// LLMClient abstracts LLM operations
type LLMClient interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

// CompletionRequest is the input for LLM calls
type CompletionRequest struct {
    Model         string
    System        string
    Messages      []Message
    MaxTokens     int
    Temperature   float64
    Tools         []Tool
    ContextLimit  int
    PruneStrategy PruneStrategy
}

// Message is a conversation turn
type Message struct {
    Role    string // "user", "assistant", "system"
    Content string
}

// CompletionResponse is the LLM output
type CompletionResponse struct {
    Content   string
    TokensIn  int
    TokensOut int
    StopReason string
}

// PruneStrategy determines how to handle context overflow
type PruneStrategy int

const (
    PruneOldest PruneStrategy = iota
    PruneSlidingWindow
    PruneSummarize
)
```

---

## Compilation

### Validation Rules

| Rule | Severity | Check |
|------|----------|-------|
| Entry exists | Error | `entryNode` is set and exists in `nodes` |
| Edge targets exist | Error | All `To` values exist in `nodes` |
| Path to END | Error | BFS from each node can reach `END` |
| No duplicate nodes | Error | Same ID not added twice |
| Reserved IDs | Error | "END", "START" are reserved |
| Valid node IDs | Error | Non-empty, no spaces |
| Unreachable nodes | Warning | Nodes not reachable from entry |

### Compilation Algorithm

```go
func (g *Graph[S]) Compile() (*CompiledGraph[S], error) {
    // 1. Validate entry point
    if g.entryNode == "" {
        return nil, ErrNoEntryPoint
    }
    if _, ok := g.nodes[g.entryNode]; !ok {
        return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, g.entryNode)
    }

    // 2. Validate all edges
    for _, edge := range g.edges {
        if _, ok := g.nodes[edge.From]; !ok && edge.From != "START" {
            return nil, fmt.Errorf("%w: edge from %s", ErrNodeNotFound, edge.From)
        }
        if _, ok := g.nodes[edge.To]; !ok && edge.To != END {
            return nil, fmt.Errorf("%w: edge to %s", ErrNodeNotFound, edge.To)
        }
    }

    // 3. Build edge map for fast lookup
    edgeMap := make(map[string][]Edge)
    for _, edge := range g.edges {
        edgeMap[edge.From] = append(edgeMap[edge.From], edge)
    }

    // 4. Check path to END exists for each node
    for nodeID := range g.nodes {
        if !canReachEnd(nodeID, edgeMap) {
            return nil, fmt.Errorf("%w: %s", ErrNoPathToEnd, nodeID)
        }
    }

    // 5. Detect unreachable nodes (warning only)
    reachable := findReachable(g.entryNode, edgeMap)
    for nodeID := range g.nodes {
        if !reachable[nodeID] {
            slog.Warn("unreachable node", "node", nodeID)
        }
    }

    return &CompiledGraph[S]{
        nodes:     g.nodes,
        edges:     edgeMap,
        entryNode: g.entryNode,
        reachable: reachable,
    }, nil
}
```

---

## Execution

### Run Algorithm

```go
func (cg *CompiledGraph[S]) Run(ctx context.Context, initial S) (S, error) {
    state := initial
    currentNode := cg.entryNode

    for currentNode != END {
        // Check context cancellation
        select {
        case <-ctx.Done():
            return state, ctx.Err()
        default:
        }

        // Get node
        node, ok := cg.nodes[currentNode]
        if !ok {
            return state, fmt.Errorf("%w: %s", ErrNodeNotFound, currentNode)
        }

        // Execute with panic recovery
        var err error
        state, err = cg.executeNode(ctx, node, state)
        if err != nil {
            return state, fmt.Errorf("node %s: %w", currentNode, err)
        }

        // Determine next node
        currentNode, err = cg.nextNode(currentNode, state)
        if err != nil {
            return state, err
        }
    }

    return state, nil
}

func (cg *CompiledGraph[S]) executeNode(
    ctx context.Context,
    node Node[S],
    state S,
) (result S, err error) {
    // Panic recovery
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("panic in node %s: %v\n%s", node.ID, r, debug.Stack())
        }
    }()

    return node.Func(ctx, state)
}

func (cg *CompiledGraph[S]) nextNode(current string, state S) (string, error) {
    edges := cg.edges[current]
    if len(edges) == 0 {
        return END, nil
    }

    for _, edge := range edges {
        if edge.Condition == nil {
            // Unconditional edge
            return edge.To, nil
        }
        // Conditional edge - evaluate router
        next := (*edge.Condition)(state)
        return next, nil
    }

    return END, nil
}
```

### Checkpointed Execution

```go
func (cg *CompiledGraph[S]) RunWithCheckpointing(
    ctx context.Context,
    initial S,
    store CheckpointStore,
) (S, error) {
    runID := generateRunID()
    state := initial
    currentNode := cg.entryNode

    // Check for existing checkpoints (resume)
    checkpoints, err := store.List(runID)
    if err != nil {
        return state, fmt.Errorf("list checkpoints: %w", err)
    }

    // Find last checkpoint and resume from there
    if len(checkpoints) > 0 {
        last := checkpoints[len(checkpoints)-1]
        if err := json.Unmarshal(last.State, &state); err != nil {
            return state, fmt.Errorf("unmarshal checkpoint: %w", err)
        }
        currentNode = cg.nextNodeAfter(last.NodeID)
    }

    for currentNode != END {
        // ... (same execution logic)

        // Checkpoint after successful node execution
        stateBytes, err := json.Marshal(state)
        if err != nil {
            return state, fmt.Errorf("marshal state: %w", err)
        }
        if err := store.Save(runID, currentNode, stateBytes); err != nil {
            return state, fmt.Errorf("save checkpoint: %w", err)
        }

        currentNode, err = cg.nextNode(currentNode, state)
        if err != nil {
            return state, err
        }
    }

    return state, nil
}
```

---

## Checkpoint Stores

### Memory Store

```go
type MemoryStore struct {
    mu          sync.RWMutex
    checkpoints map[string][]Checkpoint // runID -> checkpoints
}

func NewMemoryStore() *MemoryStore {
    return &MemoryStore{
        checkpoints: make(map[string][]Checkpoint),
    }
}
```

**Use case**: Testing, short-lived processes

### SQLite Store

```go
type SQLiteStore struct {
    db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, err
    }

    // Create table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS checkpoints (
            run_id TEXT NOT NULL,
            node_id TEXT NOT NULL,
            state BLOB NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (run_id, node_id)
        )
    `)
    if err != nil {
        return nil, err
    }

    return &SQLiteStore{db: db}, nil
}
```

**Use case**: Single-process persistence, development

### Postgres Store

```go
type PostgresStore struct {
    pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
    return &PostgresStore{pool: pool}
}
```

**Use case**: Production, multi-process

---

## LLM Clients

### Claude CLI Client

```go
type ClaudeCLIClient struct {
    binaryPath string
    model      string
    timeout    time.Duration
}

func NewClaudeCLIClient(opts ...ClaudeOption) *ClaudeCLIClient {
    c := &ClaudeCLIClient{
        binaryPath: "claude",
        model:      "claude-sonnet-4-20250514",
        timeout:    5 * time.Minute,
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}

func (c *ClaudeCLIClient) Complete(
    ctx context.Context,
    req CompletionRequest,
) (*CompletionResponse, error) {
    ctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()

    args := []string{"--print", "-p", formatPrompt(req)}
    if req.System != "" {
        args = append(args, "--system", req.System)
    }

    cmd := exec.CommandContext(ctx, c.binaryPath, args...)

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return nil, fmt.Errorf("claude timed out after %v", c.timeout)
        }
        return nil, fmt.Errorf("claude failed: %w\nstderr: %s", err, stderr.String())
    }

    return &CompletionResponse{
        Content: stdout.String(),
        // Token counts parsed from stderr if available
    }, nil
}
```

### Mock Client

```go
type MockClient struct {
    responses []MockResponse
    index     int
    mu        sync.Mutex
}

type MockResponse struct {
    Content   string
    Error     error
    Delay     time.Duration
}

func NewMockClient(responses ...MockResponse) *MockClient {
    return &MockClient{responses: responses}
}

func (m *MockClient) Complete(
    ctx context.Context,
    req CompletionRequest,
) (*CompletionResponse, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.index >= len(m.responses) {
        return nil, errors.New("no more mock responses")
    }

    resp := m.responses[m.index]
    m.index++

    if resp.Delay > 0 {
        select {
        case <-time.After(resp.Delay):
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }

    if resp.Error != nil {
        return nil, resp.Error
    }

    return &CompletionResponse{Content: resp.Content}, nil
}
```

---

## Error Handling

### Error Types

```go
var (
    // Graph construction errors
    ErrInvalidNodeID  = errors.New("invalid node ID")
    ErrDuplicateNode  = errors.New("duplicate node")
    ErrReservedNodeID = errors.New("reserved node ID")

    // Compilation errors
    ErrNodeNotFound  = errors.New("node not found")
    ErrNoEntryPoint  = errors.New("no entry point set")
    ErrNoPathToEnd   = errors.New("no path to END")

    // Execution errors
    ErrNodeExecution = errors.New("node execution failed")
)
```

### Error Wrapping

All errors wrap the original with context:

```go
// In node execution
if err != nil {
    return state, fmt.Errorf("node %s: %w", nodeID, err)
}

// Checking error types
if errors.Is(err, ErrNodeNotFound) {
    // Handle missing node
}

var execErr *NodeExecutionError
if errors.As(err, &execErr) {
    fmt.Printf("Node %s failed: %v\n", execErr.NodeID, execErr.Cause)
}
```

---

## Concurrency Model

### Current: Sequential Execution

Nodes execute one at a time in sequence. This is:
- Simple to reason about
- Easy to checkpoint
- Sufficient for most workflows

### Future: Parallel Execution

Fan-out/fan-in pattern for independent nodes:

```go
graph.AddParallelNodes("fan-out", []string{"worker-1", "worker-2", "worker-3"})
graph.AddEdge("fan-out", "aggregate") // Waits for all workers
```

**Implementation considerations**:
- State must be mergeable
- Checkpointing becomes complex
- Error handling (fail-fast vs wait-all)

---

## Observability

### Logging

All execution is logged with structured logging:

```go
logger.Info("node started", "node", nodeID, "run", runID)
logger.Info("node completed", "node", nodeID, "run", runID, "duration", duration)
logger.Error("node failed", "node", nodeID, "run", runID, "error", err)
```

### Metrics (Future)

```go
type Metrics interface {
    NodeStarted(nodeID string)
    NodeCompleted(nodeID string, duration time.Duration)
    NodeFailed(nodeID string, err error)
    CheckpointSaved(runID string, nodeID string)
}
```

### Tracing (Future)

OpenTelemetry integration:

```go
func (cg *CompiledGraph[S]) Run(ctx context.Context, initial S) (S, error) {
    ctx, span := tracer.Start(ctx, "flowgraph.run")
    defer span.End()
    // ...
}
```

---

## Performance Considerations

### State Serialization

State is serialized for checkpointing. Large state impacts:
- Checkpoint save time
- Memory usage
- Storage requirements

**Recommendations**:
- Keep state focused (IDs, not full objects)
- Use lazy loading for large data
- Consider compression for large checkpoints

### LLM Latency

LLM calls dominate execution time. Strategies:
- Generous timeouts (minutes, not seconds)
- Checkpointing before long LLM calls
- Streaming for progress feedback

### Checkpoint Storage

| Store | Write Latency | Use Case |
|-------|--------------|----------|
| Memory | <1ms | Testing |
| SQLite | 1-10ms | Development |
| Postgres | 5-50ms | Production |
