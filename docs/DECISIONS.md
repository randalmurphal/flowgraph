# Key Design Decisions

Summary of architectural decisions for flowgraph. Full ADRs in `.spec/decisions/`.

---

## Core Design

### ADR-001: State Management
**Decision**: Pass state by value, return new state from nodes.

**Rationale**:
- Immutability makes debugging easier
- No shared state between nodes
- Clear data flow

```go
func myNode(ctx Context, s State) (State, error) {
    s.Value = s.Value + 1  // Modify copy
    return s, nil          // Return new state
}
```

### ADR-002: Error Handling
**Decision**: Sentinel errors + typed errors + wrapping.

**Rationale**:
- Follow Go idioms (`errors.Is`, `errors.As`)
- Preserve context via wrapping
- Enable programmatic error handling

```go
if errors.Is(err, flowgraph.ErrNoEntryNode) { ... }

var nodeErr *flowgraph.NodeError
if errors.As(err, &nodeErr) {
    fmt.Printf("Node %s failed: %v\n", nodeErr.NodeID, nodeErr.Cause)
}
```

### ADR-003: Context Interface
**Decision**: Custom `Context` interface wrapping `context.Context`.

**Rationale**:
- Carry services (LLM, logger, etc.)
- Preserve Go context semantics
- Enable dependency injection

```go
type Context interface {
    context.Context
    LLM() llm.Client
    Logger() *slog.Logger
    // ...
}
```

### ADR-007: Validation Timing
**Decision**: Panic at build time, error at compile time.

**Rationale**:
- Fast feedback during development
- Clear separation of concerns
- Build-time panics catch programmer errors

```go
graph.AddNode("", fn)    // Panics: empty ID
graph.Compile()          // Returns error: no entry
```

### ADR-010: Execution Model
**Decision**: Synchronous execution (parallel in v2).

**Rationale**:
- Simpler implementation
- Easier debugging
- Parallel adds complexity without clear need yet

### ADR-011: Panic Recovery
**Decision**: Recover panics, convert to `PanicError`.

**Rationale**:
- Safety: don't crash the caller
- Debugging: preserve stack trace
- Consistency: all failures return errors

---

## Checkpointing

### ADR-014: Checkpoint Format
**Decision**: JSON with metadata.

**Rationale**:
- Human-readable
- Debuggable
- Language-agnostic

### ADR-015: Checkpoint Store Interface
**Decision**: Simple CRUD interface.

**Rationale**:
- Easy to implement new stores
- Minimal abstraction
- Clear semantics

### ADR-016: Resume Strategy
**Decision**: Resume FROM node after last checkpoint.

**Rationale**:
- Re-run the node that was interrupted
- Simpler than partial node execution
- Matches user expectations

### ADR-017: State Serialization
**Decision**: State must be JSON-serializable.

**Rationale**:
- Required for checkpointing
- Matches checkpoint format
- Clear contract

---

## LLM Integration

### ADR-018: LLM Interface
**Decision**: `Complete` + `Stream` methods.

**Rationale**:
- Streaming for UI responsiveness
- Non-streaming for simplicity
- Single interface for both

### ADR-020: Streaming Strategy
**Decision**: Optional, node decides.

**Rationale**:
- Not all nodes need streaming
- Node knows best
- Keep interface simple

### ADR-021: Token Tracking
**Decision**: Track in `CompletionResponse`.

**Rationale**:
- Cost management
- Budget enforcement
- Usage analytics

---

## Observability

### ADR-022: Logging
**Decision**: Structured logging via `slog`.

**Rationale**:
- Standard library (Go 1.21+)
- Structured by default
- Pluggable handlers

### ADR-023: Metrics
**Decision**: OpenTelemetry metrics.

**Rationale**:
- Industry standard
- Wide ecosystem
- Vendor-agnostic

### ADR-024: Tracing
**Decision**: OpenTelemetry tracing.

**Rationale**:
- Same as metrics
- Consistent with metrics choice
- Distributed tracing support

---

## Testing

### ADR-025: Testing Philosophy
**Decision**: Table-driven tests, high coverage.

**Rationale**:
- Go convention
- Easy to add cases
- Clear test structure

### ADR-026: Mocks
**Decision**: Interface-based mocks.

**Rationale**:
- No mock generation tools
- Explicit control
- Easy to understand

### ADR-027: Integration Tests
**Decision**: Real stores, skip if unavailable.

**Rationale**:
- Test real behavior
- CI can run full suite
- Local dev works without deps
