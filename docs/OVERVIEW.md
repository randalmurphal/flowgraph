# flowgraph Overview

## Purpose

flowgraph is a Go library for building graph-based LLM orchestration workflows. It provides the foundation for complex AI-powered automation with:

- **Declarative graph definition** - Define workflows as nodes and edges
- **Type-safe state management** - Generic state flows through the graph
- **Checkpointing/recovery** - Persist state for crash recovery and replay
- **Multi-model support** - Pluggable LLM client interface
- **Production-grade robustness** - Timeouts, retries, observability

---

## Why flowgraph?

### The Problem

Building AI-powered workflows requires:
1. Orchestrating multiple LLM calls in sequence
2. Handling conditional branching based on LLM outputs
3. Recovering from failures mid-workflow
4. Managing context across long-running operations
5. Testing workflows reliably

Existing solutions (LangChain, LangGraph) are Python-only and often over-abstracted.

### The Solution

flowgraph provides LangGraph-equivalent functionality for Go:
- **Graph-based orchestration** - Model workflows as directed graphs
- **Explicit state** - Typed structs instead of magic dictionaries
- **Checkpointing** - Resume workflows after crashes
- **Minimal dependencies** - No framework lock-in
- **Testable** - Mock LLM clients, deterministic execution

---

## Core Concepts

### Graph

A directed graph of nodes connected by edges. The graph defines the structure of the workflow.

```go
graph := flowgraph.NewGraph[TicketState]().
    AddNode("fetch-ticket", fetchTicketNode).
    AddNode("generate-spec", generateSpecNode).
    AddNode("implement", implementNode).
    AddEdge("fetch-ticket", "generate-spec").
    AddEdge("generate-spec", "implement").
    AddEdge("implement", flowgraph.END).
    SetEntry("fetch-ticket")
```

**Key properties**:
- Generic over state type `S`
- Fluent builder API
- Validated at compile time
- Immutable after compilation

### State

A typed struct that flows through the graph, accumulating data at each node.

```go
type TicketState struct {
    TicketID       string
    Ticket         *Ticket
    Spec           *Spec
    Implementation *Implementation
    Review         *ReviewResult
    PR             *PullRequest
}
```

**Key properties**:
- User-defined struct
- Passed by value (nodes return updated state)
- JSON-serializable for checkpointing
- Type-safe access (no `interface{}` maps)

### Node

A function that receives state and context, performs work, and returns updated state.

```go
type NodeFunc[S any] func(ctx Context, state S) (S, error)

func fetchTicketNode(ctx flowgraph.Context, state TicketState) (TicketState, error) {
    ticket, err := ticketService.Get(ctx, state.TicketID)
    if err != nil {
        return state, fmt.Errorf("fetch ticket: %w", err)
    }
    state.Ticket = ticket
    return state, nil
}
```

**Key properties**:
- Pure function signature
- Context provides logger, LLM client, checkpoint access
- Errors propagate with node ID context
- Panics are recovered and converted to errors

### Edge

A connection between nodes. Can be unconditional or conditional.

```go
// Unconditional
graph.AddEdge("fetch", "process")

// Conditional
graph.AddConditionalEdge("review", func(s TicketState) string {
    if s.Review.Approved {
        return "create-pr"
    }
    return "fix-findings"
})

// Terminal
graph.AddEdge("create-pr", flowgraph.END)
```

**Key properties**:
- Unconditional edges always followed
- Conditional edges use router function to pick next node
- `flowgraph.END` is special terminal node
- Cycles allowed (for retry loops) with exit conditions

### Checkpoint

A persisted snapshot of state at a point in the graph. Enables recovery and replay.

```go
type CheckpointStore interface {
    Save(runID string, nodeID string, state []byte) error
    Load(runID string, nodeID string) ([]byte, error)
    List(runID string) ([]Checkpoint, error)
}
```

**Available stores**:
- `MemoryStore` - Testing and development
- `SQLiteStore` - Single-process persistence
- `PostgresStore` - Production multi-process
- `TemporalStore` - Durable execution (optional)

---

## Execution Model

### Compilation

Before execution, the graph is compiled and validated:

1. **Entry point exists** - Graph has a starting node
2. **All references valid** - Edge targets exist
3. **Path to END** - Every node can reach terminal
4. **Cycle detection** - Identify loops (warning, not error)
5. **Unreachable nodes** - Nodes with no path from entry (warning)

```go
compiled, err := graph.Compile()
if err != nil {
    // Validation failed
}
```

### Execution

The compiled graph executes nodes in order, following edges based on state:

```go
result, err := compiled.Run(ctx, TicketState{TicketID: "TK-421"})
```

**Execution flow**:
1. Start at entry node
2. Execute node function with current state
3. If error, propagate (with node ID context)
4. Follow edge to next node (or evaluate conditional)
5. Repeat until END reached
6. Return final state

### Checkpointed Execution

For long-running workflows, checkpoint after each node:

```go
store := flowgraph.NewPostgresStore(db)
result, err := compiled.RunWithCheckpointing(ctx, initialState, store)
```

**Recovery**:
1. On crash, restart with same `runID`
2. Load checkpoints for completed nodes
3. Resume from last checkpoint
4. Skip already-completed nodes

---

## Context Interface

Nodes receive a context that provides access to shared resources:

```go
type Context interface {
    context.Context

    // Logging
    Logger() *slog.Logger

    // Checkpointing
    Checkpoint(state any) error

    // LLM operations
    LLM() LLMClient
}
```

**Usage in nodes**:

```go
func generateSpecNode(ctx flowgraph.Context, state TicketState) (TicketState, error) {
    ctx.Logger().Info("generating spec", "ticket", state.TicketID)

    response, err := ctx.LLM().Complete(ctx, flowgraph.CompletionRequest{
        System:   specSystemPrompt,
        Messages: []flowgraph.Message{{Role: "user", Content: prompt}},
    })
    if err != nil {
        return state, err
    }

    state.Spec = parseSpec(response.Content)
    return state, nil
}
```

---

## LLM Client Interface

Pluggable interface for LLM operations:

```go
type LLMClient interface {
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}
```

**Implementations**:
- `ClaudeCLIClient` - Shells out to `claude` binary
- `ClaudeAPIClient` - Direct API calls
- `OpenAIClient` - OpenAI-compatible APIs
- `OllamaClient` - Local models via Ollama
- `MockClient` - Testing

**Context management**:
```go
type CompletionRequest struct {
    Model         string
    System        string
    Messages      []Message
    MaxTokens     int
    Temperature   float64
    Tools         []Tool
    ContextLimit  int           // Auto-prune if exceeded
    PruneStrategy PruneStrategy // Oldest, SlidingWindow, Summarize
}
```

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Generics for state | `Graph[S any]` | Type safety, no runtime casts |
| State by value | Return updated state | Immutability, testability |
| Explicit edges | `AddEdge()` calls | Visible structure, validated |
| Interface for stores | `CheckpointStore` | Pluggable backends |
| Context not stored | Passed to each node | No hidden state |
| Panics recovered | Converted to errors | Robustness |

---

## Relationship to LangGraph

flowgraph is inspired by LangGraph (Python) but differs:

| Aspect | LangGraph | flowgraph |
|--------|-----------|-----------|
| Language | Python | Go |
| State | Dict-based | Typed structs |
| Typing | Runtime | Compile-time |
| Async | asyncio | goroutines/channels |
| Dependencies | Heavy (LangChain) | Minimal |
| Error handling | Exceptions | Explicit returns |

---

## Use Cases

### AI-Powered Development Workflows

The primary use case (via devflow layer):
- Parse ticket → Generate spec → Implement → Review → Create PR
- Loops for review/fix cycles
- Checkpointing for long operations

### Data Processing Pipelines

- Fetch → Transform → Enrich (via LLM) → Store
- Conditional routing based on content
- Parallel processing (future)

### Chatbot Orchestration

- Intent detection → Route → Handle → Respond
- Tool use with result incorporation
- Multi-turn conversation management

### Document Processing

- Extract → Classify → Summarize → Index
- Conditional paths based on document type
- Batch processing with checkpointing
