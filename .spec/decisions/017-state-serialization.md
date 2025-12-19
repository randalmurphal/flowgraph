# ADR-017: State Serialization

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

How should state be serialized for checkpointing? The state type is user-defined and generic.

## Decision

**JSON serialization with compile-time check for json.Marshaler requirement.**

### Approach

State types must be JSON-serializable:

```go
// User's state must have exported fields or implement json.Marshaler
type MyState struct {
    Input     string            `json:"input"`
    Output    string            `json:"output"`
    Metadata  map[string]any    `json:"metadata"`
    Timestamp time.Time         `json:"timestamp"`
}

// Or implement custom marshaling
type CustomState struct {
    internal string
}

func (c CustomState) MarshalJSON() ([]byte, error) {
    return json.Marshal(map[string]string{"value": c.internal})
}

func (c *CustomState) UnmarshalJSON(data []byte) error {
    var m map[string]string
    if err := json.Unmarshal(data, &m); err != nil {
        return err
    }
    c.internal = m["value"]
    return nil
}
```

### Validation

```go
// At compile time, we can't enforce json.Marshaler
// But at checkpoint time, we fail fast with clear error

func (cg *CompiledGraph[S]) checkpoint(store CheckpointStore, runID, nodeID string, state S) error {
    data, err := json.Marshal(state)
    if err != nil {
        return &StateSerializationError{
            NodeID:   nodeID,
            StateType: fmt.Sprintf("%T", state),
            Cause:    err,
        }
    }
    // ... save checkpoint
}

type StateSerializationError struct {
    NodeID    string
    StateType string
    Cause     error
}

func (e *StateSerializationError) Error() string {
    return fmt.Sprintf("cannot serialize state type %s at node %s: %v",
        e.StateType, e.NodeID, e.Cause)
}
```

### Documentation Requirement

State types documentation must emphasize:

```go
// State types must be JSON-serializable for checkpointing.
// All fields must be:
//   - Exported (capitalized)
//   - JSON-serializable (no channels, functions, unexported fields)
//
// For custom serialization, implement json.Marshaler and json.Unmarshaler.
//
// Example:
//   type MyState struct {
//       Input  string `json:"input"`
//       Output string `json:"output"`
//   }
```

## Alternatives Considered

### 1. Interface Constraint

```go
type Serializable interface {
    json.Marshaler
    json.Unmarshaler
}

type Graph[S Serializable] struct { ... }
```

**Rejected**: Overly restrictive. Most structs are JSON-serializable without explicit interface implementation.

### 2. Gob Encoding

```go
func checkpoint(state any) ([]byte, error) {
    var buf bytes.Buffer
    enc := gob.NewEncoder(&buf)
    err := enc.Encode(state)
    return buf.Bytes(), err
}
```

**Rejected**: Not human-readable, harder to debug, less portable.

### 3. Reflection-Based Validation

```go
// At compile time of Graph, validate state type
func NewGraph[S any]() *Graph[S] {
    var zero S
    // Use reflection to check all fields are serializable
    validateSerializable(reflect.TypeOf(zero))
    return &Graph[S]{}
}
```

**Rejected**: Reflection is slow, can't catch all issues (custom MarshalJSON).

### 4. Code Generation

```go
//go:generate flowgraph-gen -state MyState
```

**Rejected for v1**: Adds build complexity. JSON is good enough.

## Consequences

### Positive
- **Simple** - Standard library JSON
- **Debuggable** - Human-readable checkpoints
- **Flexible** - Custom Marshaler for complex types

### Negative
- Runtime error if state isn't serializable (caught at first checkpoint)
- Can't serialize channels, functions, unexported fields

### Risks
- User forgets to make fields exported → Clear error message at checkpoint time

---

## Common Patterns

### Basic Struct

```go
type OrderState struct {
    OrderID   string    `json:"order_id"`
    Items     []Item    `json:"items"`
    Total     float64   `json:"total"`
    CreatedAt time.Time `json:"created_at"`
}

// Serializes automatically
```

### Struct with Pointer

```go
type TicketState struct {
    Ticket *Ticket `json:"ticket"`  // nil becomes null
    Spec   *Spec   `json:"spec"`
}

// nil fields serialize as null, deserialize as nil
```

### Struct with Interface

```go
type ProcessState struct {
    // BAD: interface{} loses type info on round-trip
    Result any `json:"result"`
}

// GOOD: Use concrete types or type wrapper
type ProcessState struct {
    ResultType string          `json:"result_type"`
    Result     json.RawMessage `json:"result"`
}

func (p *ProcessState) SetResult(v any) error {
    data, err := json.Marshal(v)
    if err != nil {
        return err
    }
    p.ResultType = fmt.Sprintf("%T", v)
    p.Result = data
    return nil
}
```

### Large Binary Data

```go
type ImageState struct {
    ImageID  string `json:"image_id"`
    // BAD: Large binary in JSON
    // ImageData []byte `json:"image_data"`

    // GOOD: Store reference, not data
    ImagePath string `json:"image_path"`
}
```

### Excluding Fields

```go
type DebugState struct {
    Input   string `json:"input"`
    Output  string `json:"output"`

    // Exclude from serialization
    Logger  *slog.Logger `json:"-"`
    TempDir string       `json:"-"`
}

// Logger and TempDir won't be checkpointed
// They need to be re-initialized on resume
```

---

## Error Examples

### Non-Serializable Field

```go
type BadState struct {
    Input string
    done  chan struct{}  // Unexported, but also channels can't serialize
}

// Error: cannot serialize state type BadState at node process:
//   json: unsupported type: chan struct {}
```

### Unexported Field

```go
type BadState struct {
    Input  string
    output string  // Unexported
}

// Warning: unexported field 'output' will be omitted from checkpoint
// State after resume may differ from state before crash
```

---

## Test Cases

```go
func TestStateSerialization_Basic(t *testing.T) {
    state := TestState{
        Input:  "hello",
        Output: "world",
        Count:  42,
    }

    data, err := json.Marshal(state)
    require.NoError(t, err)

    var loaded TestState
    err = json.Unmarshal(data, &loaded)
    require.NoError(t, err)

    assert.Equal(t, state, loaded)
}

func TestStateSerialization_CustomMarshaler(t *testing.T) {
    state := CustomState{internal: "secret"}

    data, err := json.Marshal(state)
    require.NoError(t, err)
    assert.JSONEq(t, `{"value":"secret"}`, string(data))

    var loaded CustomState
    err = json.Unmarshal(data, &loaded)
    require.NoError(t, err)
    assert.Equal(t, "secret", loaded.internal)
}

func TestStateSerialization_NonSerializable(t *testing.T) {
    type BadState struct {
        Ch chan int
    }

    state := BadState{Ch: make(chan int)}
    _, err := json.Marshal(state)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "unsupported type")
}

func TestStateSerialization_PointerNil(t *testing.T) {
    type PtrState struct {
        Data *string `json:"data"`
    }

    // Nil pointer
    state := PtrState{Data: nil}
    data, _ := json.Marshal(state)
    assert.JSONEq(t, `{"data":null}`, string(data))

    // Non-nil pointer
    s := "hello"
    state = PtrState{Data: &s}
    data, _ = json.Marshal(state)
    assert.JSONEq(t, `{"data":"hello"}`, string(data))
}
```

---

## Best Practices

1. **Use exported fields** - All fields that need checkpointing must be capitalized
2. **Add JSON tags** - Explicit tags prevent field name changes from breaking checkpoints
3. **Avoid interfaces** - Use concrete types or json.RawMessage for polymorphism
4. **Exclude transient data** - Use `json:"-"` for loggers, connections, temp files
5. **Handle nil** - Pointer fields may be nil after resume
6. **Test serialization** - Unit test that your state round-trips correctly
