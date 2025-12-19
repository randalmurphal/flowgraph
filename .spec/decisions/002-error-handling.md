# ADR-002: Error Handling Strategy

**Status**: Proposed
**Date**: 2025-01-19
**Deciders**: @rmurphy

---

## Context

Error handling is critical for a workflow orchestration library. Errors can occur at multiple points:
- Graph construction (invalid node ID)
- Compilation (missing edges)
- Execution (node failure, timeout)
- Checkpointing (serialization failure)

We need a consistent strategy that:
- Provides clear, actionable error messages
- Allows programmatic error handling
- Supports wrapping for context
- Works well with Go idioms

## Problem Statement

How should flowgraph handle and communicate errors?

## Decision Drivers

- **Clarity**: Errors must be clear and actionable
- **Programmatic handling**: Code must be able to check error types
- **Context preservation**: Errors must carry context through the stack
- **Go idioms**: Follow standard Go error patterns
- **Debugging**: Errors must help identify root cause

## Considered Options

### Option 1: Sentinel Errors Only

```go
var (
    ErrInvalidNodeID = errors.New("invalid node ID")
    ErrNodeNotFound  = errors.New("node not found")
)

// Usage
if errors.Is(err, ErrNodeNotFound) {
    // handle
}
```

**Pros**:
- Simple
- Standard Go pattern
- Easy to check

**Cons**:
- No additional context
- Can't carry dynamic data

### Option 2: Typed Errors Only

```go
type NodeNotFoundError struct {
    NodeID string
}

func (e *NodeNotFoundError) Error() string {
    return fmt.Sprintf("node not found: %s", e.NodeID)
}

// Usage
var notFound *NodeNotFoundError
if errors.As(err, &notFound) {
    fmt.Println("Missing node:", notFound.NodeID)
}
```

**Pros**:
- Carries context
- Programmatically inspectable

**Cons**:
- More verbose
- May be overkill for simple errors

### Option 3: Hybrid (Sentinel + Typed + Wrapping)

```go
// Sentinel errors for categories
var (
    ErrConstruction = errors.New("graph construction error")
    ErrCompilation  = errors.New("graph compilation error")
    ErrExecution    = errors.New("graph execution error")
)

// Typed errors for specific cases with context
type NodeError struct {
    NodeID string
    Op     string
    Err    error
}

func (e *NodeError) Error() string {
    return fmt.Sprintf("node %s: %s: %v", e.NodeID, e.Op, e.Err)
}

func (e *NodeError) Unwrap() error { return e.Err }

// Wrapping for context
return fmt.Errorf("compile: %w", &NodeError{NodeID: id, Op: "validate", Err: ErrNodeNotFound})

// Usage - check category
if errors.Is(err, ErrExecution) { ... }

// Usage - get specifics
var nodeErr *NodeError
if errors.As(err, &nodeErr) {
    fmt.Printf("Node %s failed during %s\n", nodeErr.NodeID, nodeErr.Op)
}
```

**Pros**:
- Best of both worlds
- Category checking with `errors.Is`
- Context access with `errors.As`
- Full context chain with `Unwrap`

**Cons**:
- More code
- Need to be consistent

## Decision

We will use **Option 3: Hybrid** because:

1. **Sentinel errors for categories** - Easy to check "is this a compilation error?"
2. **Typed errors for context** - Get the node ID, operation, etc.
3. **Wrapping for full chain** - Understand the full error path
4. **Standard Go patterns** - Works with `errors.Is` and `errors.As`

## Error Taxonomy

### Category Errors (Sentinel)

```go
var (
    // Graph building
    ErrConstruction = errors.New("graph construction error")

    // Compilation/validation
    ErrCompilation = errors.New("graph compilation error")

    // Runtime execution
    ErrExecution = errors.New("graph execution error")

    // Checkpointing
    ErrCheckpoint = errors.New("checkpoint error")
)
```

### Specific Errors (Sentinel)

```go
var (
    // Construction
    ErrInvalidNodeID  = errors.New("invalid node ID")
    ErrDuplicateNode  = errors.New("duplicate node")
    ErrReservedNodeID = errors.New("reserved node ID")

    // Compilation
    ErrNoEntryPoint  = errors.New("no entry point set")
    ErrNodeNotFound  = errors.New("node not found")
    ErrNoPathToEnd   = errors.New("no path to END")
    ErrInvalidEdge   = errors.New("invalid edge")

    // Execution
    ErrNodePanic     = errors.New("node panicked")
    ErrNodeTimeout   = errors.New("node timed out")

    // Checkpoint
    ErrCheckpointNotFound = errors.New("checkpoint not found")
    ErrStateCorrupted     = errors.New("state corrupted")
)
```

### Contextual Errors (Typed)

```go
// NodeError wraps errors with node context
type NodeError struct {
    NodeID string
    Op     string // "execute", "validate", "checkpoint"
    Err    error
}

// EdgeError wraps errors with edge context
type EdgeError struct {
    From string
    To   string
    Err  error
}

// RunError wraps errors with run context
type RunError struct {
    RunID    string
    NodeID   string
    Duration time.Duration
    Err      error
}
```

## Consequences

### Positive

- Clear error messages with full context
- Programmatic error handling with type checks
- Debugging is straightforward
- Works with standard Go tooling

### Negative

- **More code**: Mitigate with helper constructors:
  ```go
  func nodeErr(nodeID, op string, err error) error {
      return &NodeError{NodeID: nodeID, Op: op, Err: err}
  }
  ```

### Neutral

- Need to be consistent about wrapping throughout codebase

## Implementation Notes

### Error Construction Helpers

```go
func nodeErr(nodeID, op string, err error) error {
    return &NodeError{NodeID: nodeID, Op: op, Err: err}
}

func wrapf(err error, format string, args ...any) error {
    return fmt.Errorf(format+": %w", append(args, err)...)
}
```

### Usage in Code

```go
func (g *Graph[S]) Compile() (*CompiledGraph[S], error) {
    if g.entryNode == "" {
        return nil, fmt.Errorf("%w: %w", ErrCompilation, ErrNoEntryPoint)
    }

    if _, ok := g.nodes[g.entryNode]; !ok {
        return nil, fmt.Errorf("%w: entry %w: %s",
            ErrCompilation, ErrNodeNotFound, g.entryNode)
    }
    // ...
}

func (cg *CompiledGraph[S]) executeNode(ctx context.Context, nodeID string, state S) (S, error) {
    result, err := node.Func(ctx, state)
    if err != nil {
        return state, nodeErr(nodeID, "execute", err)
    }
    return result, nil
}
```

### Error Checking in User Code

```go
result, err := compiled.Run(ctx, state)
if err != nil {
    // Check category
    if errors.Is(err, flowgraph.ErrExecution) {
        // Execution failed
    }

    // Get node details
    var nodeErr *flowgraph.NodeError
    if errors.As(err, &nodeErr) {
        log.Printf("Node %s failed during %s: %v",
            nodeErr.NodeID, nodeErr.Op, nodeErr.Err)
    }

    // Check specific error
    if errors.Is(err, flowgraph.ErrNodeTimeout) {
        // Handle timeout specifically
    }
}
```

## Validation

This decision is correct if:
- [ ] All errors include enough context for debugging
- [ ] Category checking works with `errors.Is`
- [ ] Specific error extraction works with `errors.As`
- [ ] Error messages are clear to end users

## References

- [Go Blog: Error handling and Go](https://go.dev/blog/error-handling-and-go)
- [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)
