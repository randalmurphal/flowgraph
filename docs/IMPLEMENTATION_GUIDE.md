# flowgraph Implementation Guide

Guide for implementing flowgraph, porting patterns from Python (ai-devtools/ensemble).

---

## Context

This codebase is being ported from a working Python orchestration system. The Python code:
1. Works and has proven patterns
2. Shells out to Claude CLI for LLM operations
3. Lacks robustness (error handling, testing, checkpointing)
4. Uses dicts for state (type-unsafe)

The goal is to:
1. Port to Go with proper architecture
2. Add type safety with generics
3. Add comprehensive testing
4. Add checkpointing for recovery
5. Make it production-grade

---

## Working Principles

### 1. Understand Before Implementing

Before writing Go code:
1. Read corresponding Python code completely
2. Identify core logic vs incidental complexity
3. Note what works well vs needs improvement
4. Understand data flow
5. Identify implicit assumptions

### 2. Don't Directly Translate

Python idioms don't map to Go:

```python
# Python
result = [x.upper() for x in items if x]
```

```go
// Go - be explicit
var result []string
for _, x := range items {
    if x != "" {
        result = append(result, strings.ToUpper(x))
    }
}
```

### 3. Type Everything

Python dicts become Go structs:

```python
# Python
def process_ticket(ticket):
    return {
        "id": ticket["id"],
        "spec": generate_spec(ticket),
        "status": "processed"
    }
```

```go
// Go
type ProcessedTicket struct {
    ID     string `json:"id"`
    Spec   *Spec  `json:"spec"`
    Status string `json:"status"`
}

func ProcessTicket(ticket *Ticket) (*ProcessedTicket, error) {
    spec, err := GenerateSpec(ticket)
    if err != nil {
        return nil, fmt.Errorf("generate spec: %w", err)
    }
    return &ProcessedTicket{
        ID:     ticket.ID,
        Spec:   spec,
        Status: "processed",
    }, nil
}
```

### 4. Explicit Error Handling

Every error must be handled:

```python
# Python
try:
    result = run_claude(prompt)
except TimeoutError:
    result = None
except ClaudeError as e:
    logger.error(f"Claude failed: {e}")
    raise
```

```go
// Go
result, err := runClaude(ctx, prompt)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return nil, nil // Timeout case
    }
    return nil, fmt.Errorf("run claude: %w", err)
}
```

### 5. Add What Python Lacks

The Python version lacks:
- Proper context cancellation
- Timeouts at every I/O boundary
- Retry logic with backoff
- Checkpointing for crash recovery
- Comprehensive input validation
- Structured logging

Add these in Go.

---

## Implementation Order

### Phase 1: Core Graph

```
flowgraph/
├── graph.go         # Graph definition, AddNode, AddEdge
├── node.go          # Node types, NodeFunc
├── edge.go          # Edge types, conditional edges
├── compile.go       # Graph compilation, validation
├── execute.go       # Run, RunWithCheckpointing
├── context.go       # Execution context
├── errors.go        # Error types
└── graph_test.go    # Tests
```

**Start with**:
1. Define `Graph[S any]` struct
2. Implement `AddNode`, `AddEdge`, `SetEntry`
3. Implement `Compile()` with validation
4. Implement `Run()` for linear flows
5. Add tests for each step

**Then add**:
1. Conditional edges
2. Loops
3. Checkpointing
4. Error handling improvements

### Phase 2: Checkpointing

```
flowgraph/checkpoint/
├── store.go         # Interface
├── memory.go        # In-memory (testing)
├── sqlite.go        # SQLite (development)
├── postgres.go      # Postgres (production)
└── store_test.go    # Interface compliance tests
```

### Phase 3: LLM Clients

```
flowgraph/llm/
├── client.go        # Interface
├── claude_cli.go    # Shell out to claude
├── mock.go          # For testing
└── client_test.go
```

---

## Porting Patterns

### Subprocess Calls

Python:
```python
result = subprocess.run(
    ["claude", "--print", "-p", prompt],
    capture_output=True,
    timeout=120
)
```

Go:
```go
func runClaude(ctx context.Context, prompt string) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
    defer cancel()

    cmd := exec.CommandContext(ctx, "claude", "--print", "-p", prompt)

    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return "", fmt.Errorf("claude timed out after 2m")
        }
        return "", fmt.Errorf("claude failed: %w\nstderr: %s", err, stderr.String())
    }

    return stdout.String(), nil
}
```

### Dict-Heavy Code

Python:
```python
state = {
    "ticket": ticket,
    "spec": None,
    "implementation": None,
    "review": None,
}

def generate_spec(state):
    state["spec"] = call_claude(...)
    return state
```

Go:
```go
type TicketState struct {
    Ticket         *Ticket
    Spec           *Spec
    Implementation *Implementation
    Review         *ReviewResult
}

func generateSpecNode(ctx flowgraph.Context, state TicketState) (TicketState, error) {
    spec, err := callClaude(ctx, ...)
    if err != nil {
        return state, err
    }
    state.Spec = spec
    return state, nil
}
```

### File Operations

Python:
```python
with open(f".orchestrator/runs/{run_id}/transcript.json", "w") as f:
    json.dump(transcript, f)
```

Go:
```go
func (m *TranscriptManager) Save(runID string, transcript *Transcript) error {
    dir := filepath.Join(m.baseDir, "runs", runID)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("create run dir: %w", err)
    }

    path := filepath.Join(dir, "transcript.json")

    data, err := json.MarshalIndent(transcript, "", "  ")
    if err != nil {
        return fmt.Errorf("marshal transcript: %w", err)
    }

    if err := os.WriteFile(path, data, 0644); err != nil {
        return fmt.Errorf("write transcript: %w", err)
    }

    return nil
}
```

### Async/Concurrent Code

Python:
```python
async def run_parallel(tasks):
    return await asyncio.gather(*[process(t) for t in tasks])
```

Go:
```go
func runParallel(ctx context.Context, tasks []*Task) ([]*Result, error) {
    results := make([]*Result, len(tasks))
    errs := make([]error, len(tasks))

    var wg sync.WaitGroup
    for i, task := range tasks {
        wg.Add(1)
        go func(i int, task *Task) {
            defer wg.Done()
            result, err := process(ctx, task)
            results[i] = result
            errs[i] = err
        }(i, task)
    }
    wg.Wait()

    // Collect errors
    var multiErr error
    for _, err := range errs {
        if err != nil {
            multiErr = errors.Join(multiErr, err)
        }
    }

    return results, multiErr
}
```

---

## Common Patterns

### Functional Options

```go
type ClientOption func(*Client)

func WithTimeout(d time.Duration) ClientOption {
    return func(c *Client) {
        c.timeout = d
    }
}

func NewClient(opts ...ClientOption) *Client {
    c := &Client{
        timeout: 30 * time.Second, // default
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}
```

### Context Propagation

```go
// Always accept context as first parameter
func DoThing(ctx context.Context, input Input) (Output, error) {
    // Check context before expensive operations
    select {
    case <-ctx.Done():
        return Output{}, ctx.Err()
    default:
    }

    // Pass context to downstream calls
    return downstream(ctx, ...)
}
```

### Error Wrapping

```go
// Always add context to errors
if err != nil {
    return fmt.Errorf("operation X with input %s: %w", input.ID, err)
}

// Use sentinel errors for known conditions
var ErrNotFound = errors.New("not found")

// Check with errors.Is
if errors.Is(err, ErrNotFound) {
    // Handle not found
}
```

### Structured Logging

```go
// Use slog
logger := slog.With(
    "component", "flowgraph",
    "run_id", runID,
)

logger.Info("node started",
    "node_id", nodeID,
    "input_size", len(input),
)

logger.Error("node failed",
    "node_id", nodeID,
    "error", err,
)
```

---

## Red Flags

### In Python Code

| Pattern | Go Handling |
|---------|-------------|
| `try/except: pass` | Explicit error handling |
| Global state | Explicit dependencies |
| Magic strings | Constants or types |
| Implicit conversions | Explicit type conversions |
| Missing error handling | Every error has a path |

### In Your Go Code

| Anti-Pattern | Fix |
|--------------|-----|
| `_ = err` | Handle or return |
| Naked returns | Explicit returns |
| Deep nesting | Extract functions |
| Long functions | Break up (50 lines max) |
| Missing tests | Write them now |

---

## Component Checklist

Before considering a component done:

- [ ] All functions have doc comments
- [ ] All exported types have doc comments
- [ ] Errors are wrapped with context
- [ ] Context is accepted and respected
- [ ] Timeouts are configurable
- [ ] Tests exist and pass
- [ ] Tests cover error cases
- [ ] No data races (`go test -race`)
- [ ] Logging is structured
- [ ] Metrics/tracing hooks exist (if applicable)

---

## Questions to Ask When Stuck

1. **What is the Python code trying to do?** - Strip implementation, find the goal
2. **What are the inputs and outputs?** - Define types for them
3. **What can go wrong?** - Each failure mode needs handling
4. **Is this the right abstraction level?** - flowgraph, devflow, or task-keeper?
5. **How will this be tested?** - Hard to test = wrong design
6. **What would an API user expect?** - Design outside-in
