# ADR-014: Checkpoint Format

**Status**: Accepted
**Date**: 2025-01-19
**Deciders**: Architecture Team

---

## Context

What format should checkpoints be stored in? Considerations:
- Serialization format (JSON, Protobuf, Gob, MessagePack)
- What data to include
- Versioning for schema evolution
- Compression

## Decision

**JSON with metadata wrapper, optional compression, version field.**

### Checkpoint Structure

```go
type Checkpoint struct {
    // Metadata
    Version     int       `json:"version"`      // Schema version
    RunID       string    `json:"run_id"`
    NodeID      string    `json:"node_id"`      // Node that produced this state
    Timestamp   time.Time `json:"timestamp"`
    Sequence    int       `json:"sequence"`     // Order within run

    // State
    State       json.RawMessage `json:"state"`  // User's state, serialized

    // Execution context
    Attempt     int    `json:"attempt"`         // Which attempt (for retries)
    PrevNodeID  string `json:"prev_node_id"`    // Previous node (for debugging)

    // Optional: compressed state for large payloads
    Compressed  bool   `json:"compressed,omitempty"`
}

const CurrentCheckpointVersion = 1
```

### Serialization

```go
func (cg *CompiledGraph[S]) checkpoint(store CheckpointStore, runID, nodeID string, state S, seq int) error {
    stateBytes, err := json.Marshal(state)
    if err != nil {
        return fmt.Errorf("marshal state: %w", err)
    }

    // Compress if large
    compressed := false
    if len(stateBytes) > 1024*1024 { // 1MB threshold
        stateBytes = compress(stateBytes)
        compressed = true
    }

    cp := Checkpoint{
        Version:    CurrentCheckpointVersion,
        RunID:      runID,
        NodeID:     nodeID,
        Timestamp:  time.Now().UTC(),
        Sequence:   seq,
        State:      stateBytes,
        Attempt:    1,
        Compressed: compressed,
    }

    data, err := json.Marshal(cp)
    if err != nil {
        return fmt.Errorf("marshal checkpoint: %w", err)
    }

    return store.Save(runID, nodeID, data)
}
```

### Deserialization

```go
func loadCheckpoint[S any](store CheckpointStore, runID, nodeID string) (*S, error) {
    data, err := store.Load(runID, nodeID)
    if err != nil {
        return nil, err
    }

    var cp Checkpoint
    if err := json.Unmarshal(data, &cp); err != nil {
        return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
    }

    // Version migration if needed
    if cp.Version != CurrentCheckpointVersion {
        return nil, fmt.Errorf("unsupported checkpoint version: %d", cp.Version)
    }

    // Decompress if needed
    stateBytes := cp.State
    if cp.Compressed {
        stateBytes = decompress(stateBytes)
    }

    var state S
    if err := json.Unmarshal(stateBytes, &state); err != nil {
        return nil, fmt.Errorf("unmarshal state: %w", err)
    }

    return &state, nil
}
```

## Alternatives Considered

### 1. Protocol Buffers

```protobuf
message Checkpoint {
    int32 version = 1;
    string run_id = 2;
    bytes state = 3;  // Serialized state
}
```

**Rejected for v1**: Adds protobuf dependency, build complexity. JSON is sufficient for checkpoint sizes.

### 2. Gob (Go's native encoding)

```go
var buf bytes.Buffer
enc := gob.NewEncoder(&buf)
enc.Encode(state)
```

**Rejected**: Not human-readable, not cross-language, no schema evolution story.

### 3. MessagePack

```go
data, _ := msgpack.Marshal(state)
```

**Rejected for v1**: Marginal benefits over JSON. Would add dependency for little gain.

### 4. Raw State Only

```go
// Just store the state, no wrapper
json.Marshal(state)
```

**Rejected**: No versioning, no metadata for debugging, no compression flag.

## Consequences

### Positive
- **Debuggable** - JSON is human-readable
- **Versionable** - Version field enables migration
- **Portable** - JSON works everywhere
- **Compressible** - Large states handled efficiently

### Negative
- JSON overhead (~30% larger than binary)
- Compression adds CPU cost for large states

### Risks
- State too large for JSON → Mitigate: Compression, streaming in future

---

## Compression Strategy

```go
import "github.com/klauspost/compress/zstd"

var (
    encoder, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
    decoder, _ = zstd.NewReader(nil)
)

func compress(data []byte) []byte {
    return encoder.EncodeAll(data, nil)
}

func decompress(data []byte) ([]byte, error) {
    return decoder.DecodeAll(data, nil)
}
```

### Compression Thresholds

| State Size | Action |
|------------|--------|
| < 1 KB | No compression |
| 1 KB - 1 MB | Optional, disabled by default |
| > 1 MB | Automatic compression |
| > 100 MB | Warning logged |

---

## Version Migration

```go
func migrateCheckpoint(data []byte) ([]byte, error) {
    var raw map[string]any
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, err
    }

    version, ok := raw["version"].(float64)
    if !ok {
        version = 0  // Pre-versioning
    }

    switch int(version) {
    case 0:
        // Migrate from v0 to v1
        raw["version"] = 1
        raw["attempt"] = 1  // Default
        raw["prev_node_id"] = ""
        return json.Marshal(raw)

    case 1:
        return data, nil  // Current version

    default:
        return nil, fmt.Errorf("unknown checkpoint version: %d", int(version))
    }
}
```

---

## Usage Examples

### Checkpoint After Each Node

```go
result, err := compiled.Run(ctx, state,
    flowgraph.WithCheckpointing(store),
)
```

### List Checkpoints for Run

```go
checkpoints, err := store.List(runID)
for _, cp := range checkpoints {
    fmt.Printf("Node: %s, Time: %s, Size: %d\n",
        cp.NodeID, cp.Timestamp, len(cp.State))
}
```

### Debug Checkpoint Contents

```go
// Load and pretty-print checkpoint
data, _ := store.Load(runID, nodeID)

var cp flowgraph.Checkpoint
json.Unmarshal(data, &cp)

fmt.Printf("Checkpoint version: %d\n", cp.Version)
fmt.Printf("Node: %s\n", cp.NodeID)
fmt.Printf("Timestamp: %s\n", cp.Timestamp)
fmt.Printf("State:\n%s\n", string(cp.State))
```

---

## Test Cases

```go
func TestCheckpointFormat(t *testing.T) {
    state := TestState{
        Input:  "hello",
        Output: "world",
        Count:  42,
    }

    // Create checkpoint
    cp := Checkpoint{
        Version:   CurrentCheckpointVersion,
        RunID:     "run-123",
        NodeID:    "process",
        Timestamp: time.Now().UTC(),
        Sequence:  1,
        State:     mustMarshal(state),
    }

    // Serialize
    data, err := json.Marshal(cp)
    require.NoError(t, err)

    // Verify JSON structure
    var raw map[string]any
    json.Unmarshal(data, &raw)
    assert.Equal(t, float64(1), raw["version"])
    assert.Equal(t, "run-123", raw["run_id"])

    // Deserialize
    var loaded Checkpoint
    err = json.Unmarshal(data, &loaded)
    require.NoError(t, err)
    assert.Equal(t, cp.Version, loaded.Version)
    assert.Equal(t, cp.RunID, loaded.RunID)
}

func TestCheckpointCompression(t *testing.T) {
    // Large state
    state := TestState{
        Data: strings.Repeat("x", 2*1024*1024),  // 2MB
    }

    stateBytes, _ := json.Marshal(state)
    original := len(stateBytes)

    compressed := compress(stateBytes)
    compressedSize := len(compressed)

    t.Logf("Original: %d, Compressed: %d, Ratio: %.2f%%",
        original, compressedSize, float64(compressedSize)/float64(original)*100)

    // Verify round-trip
    decompressed, err := decompress(compressed)
    require.NoError(t, err)
    assert.Equal(t, stateBytes, decompressed)
}
```
