package checkpoint_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckpoint_New(t *testing.T) {
	state := []byte(`{"value": 42}`)
	cp := checkpoint.New("run-123", "node-a", 1, state, "node-b")

	assert.Equal(t, checkpoint.Version, cp.Version)
	assert.Equal(t, "run-123", cp.RunID)
	assert.Equal(t, "node-a", cp.NodeID)
	assert.Equal(t, 1, cp.Sequence)
	assert.Equal(t, "node-b", cp.NextNode)
	assert.Equal(t, json.RawMessage(state), cp.State)
	assert.Equal(t, 1, cp.Attempt) // Default attempt
	assert.Empty(t, cp.PrevNodeID) // Not set by default
	assert.False(t, cp.Timestamp.IsZero())
}

func TestCheckpoint_WithAttempt(t *testing.T) {
	cp := checkpoint.New("run-1", "node-a", 1, []byte("{}"), "node-b").
		WithAttempt(3)

	assert.Equal(t, 3, cp.Attempt)
}

func TestCheckpoint_WithPrevNode(t *testing.T) {
	cp := checkpoint.New("run-1", "node-b", 2, []byte("{}"), "node-c").
		WithPrevNode("node-a")

	assert.Equal(t, "node-a", cp.PrevNodeID)
}

func TestCheckpoint_MarshalUnmarshal(t *testing.T) {
	state := []byte(`{"counter":10}`)
	original := checkpoint.New("run-123", "process", 5, state, "validate").
		WithAttempt(2).
		WithPrevNode("start")

	// Marshal
	data, err := original.Marshal()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Unmarshal
	loaded, err := checkpoint.Unmarshal(data)
	require.NoError(t, err)

	// Compare fields
	assert.Equal(t, original.Version, loaded.Version)
	assert.Equal(t, original.RunID, loaded.RunID)
	assert.Equal(t, original.NodeID, loaded.NodeID)
	assert.Equal(t, original.Sequence, loaded.Sequence)
	assert.Equal(t, original.NextNode, loaded.NextNode)
	assert.Equal(t, original.Attempt, loaded.Attempt)
	assert.Equal(t, original.PrevNodeID, loaded.PrevNodeID)
	assert.JSONEq(t, string(original.State), string(loaded.State))

	// Timestamp should be preserved (within a small margin due to JSON serialization)
	assert.WithinDuration(t, original.Timestamp, loaded.Timestamp, time.Second)
}

func TestCheckpoint_UnmarshalInvalidJSON(t *testing.T) {
	_, err := checkpoint.Unmarshal([]byte("not json"))
	assert.Error(t, err)
}

func TestCheckpoint_JSONFormat(t *testing.T) {
	cp := checkpoint.New("run-1", "node-a", 1, []byte(`{"value":42}`), "node-b")

	data, err := cp.Marshal()
	require.NoError(t, err)

	// Verify it's valid JSON
	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Verify expected fields exist
	assert.Equal(t, float64(checkpoint.Version), raw["version"])
	assert.Equal(t, "run-1", raw["run_id"])
	assert.Equal(t, "node-a", raw["node_id"])
	assert.Equal(t, float64(1), raw["sequence"])
	assert.Equal(t, "node-b", raw["next_node"])
	assert.NotEmpty(t, raw["timestamp"])

	// State should be nested JSON
	stateMap, ok := raw["state"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(42), stateMap["value"])
}

func TestCheckpoint_LargeState(t *testing.T) {
	// Test with a larger state payload
	state := make(map[string]string)
	for i := 0; i < 1000; i++ {
		state[string(rune('a'+i%26))+string(rune('0'+i%10))] = "value"
	}

	stateBytes, err := json.Marshal(state)
	require.NoError(t, err)

	cp := checkpoint.New("run-1", "node-a", 1, stateBytes, "node-b")
	data, err := cp.Marshal()
	require.NoError(t, err)

	loaded, err := checkpoint.Unmarshal(data)
	require.NoError(t, err)
	assert.Equal(t, string(stateBytes), string(loaded.State))
}
