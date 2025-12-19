package flowgraph_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rmurphy/flowgraph/pkg/flowgraph"
	"github.com/rmurphy/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResumeFrom_EdgeCases tests edge cases for ResumeFrom function.
func TestResumeFrom_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, store checkpoint.Store) (runID, nodeID string)
		wantErr   error
		errMsg    string
		skipSetup bool
	}{
		{
			name: "checkpoint version mismatch",
			setup: func(t *testing.T, store checkpoint.Store) (string, string) {
				// Create a checkpoint with wrong version
				state, _ := json.Marshal(CheckpointState{Value: 10})
				cp := &checkpoint.Checkpoint{
					Version:  999, // Wrong version
					RunID:    "version-test",
					NodeID:   "node-a",
					Sequence: 1,
					State:    state,
					NextNode: flowgraph.END,
				}
				data, _ := cp.Marshal()
				_ = store.Save("version-test", "node-a", data)
				return "version-test", "node-a"
			},
			wantErr: flowgraph.ErrCheckpointVersionMismatch,
			errMsg:  "checkpoint version mismatch",
		},
		{
			name: "state deserialization fails",
			setup: func(t *testing.T, store checkpoint.Store) (string, string) {
				// Create a checkpoint with invalid JSON in State field
				cp := &checkpoint.Checkpoint{
					Version:  checkpoint.Version,
					RunID:    "deserialize-test",
					NodeID:   "node-a",
					Sequence: 1,
					State:    []byte(`{invalid json`), // Malformed JSON
					NextNode: flowgraph.END,
				}
				data, _ := cp.Marshal()
				_ = store.Save("deserialize-test", "node-a", data)
				return "deserialize-test", "node-a"
			},
			wantErr: flowgraph.ErrDeserializeState,
			errMsg:  "failed to deserialize state",
		},
		{
			name: "invalid start node (node doesn't exist)",
			setup: func(t *testing.T, store checkpoint.Store) (string, string) {
				// Create a valid checkpoint but with NextNode pointing to non-existent node
				state, _ := json.Marshal(CheckpointState{Value: 10})
				cp := &checkpoint.Checkpoint{
					Version:  checkpoint.Version,
					RunID:    "invalid-node-test",
					NodeID:   "node-a",
					Sequence: 1,
					State:    state,
					NextNode: "nonexistent-node",
				}
				data, _ := cp.Marshal()
				_ = store.Save("invalid-node-test", "node-a", data)
				return "invalid-node-test", "node-a"
			},
			wantErr: flowgraph.ErrInvalidResumeNode,
			errMsg:  "invalid resume node: nonexistent-node",
		},
		{
			name: "resume from nonexistent checkpoint",
			setup: func(t *testing.T, store checkpoint.Store) (string, string) {
				// Don't create any checkpoint
				return "no-checkpoint-run", "nonexistent-node"
			},
			wantErr: flowgraph.ErrNoCheckpoints,
			errMsg:  "no checkpoints found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := checkpoint.NewMemoryStore()

			// Setup checkpoint data
			runID, nodeID := tt.setup(t, store)

			// Create a simple graph
			graph := flowgraph.NewGraph[CheckpointState]().
				AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
					s.Value++
					return s, nil
				}).
				AddNode("node-b", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
					s.Value++
					return s, nil
				}).
				AddEdge("node-a", "node-b").
				AddEdge("node-b", flowgraph.END).
				SetEntry("node-a")

			compiled, err := graph.Compile()
			require.NoError(t, err)

			ctx := flowgraph.NewContext(context.Background())

			// Attempt to resume from the checkpoint
			_, err = compiled.ResumeFrom(ctx, store, runID, nodeID)

			// Verify error
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// TestResumeFrom_WithStateValidationFailure tests ResumeFrom with state validation that fails.
func TestResumeFrom_WithStateValidationFailure(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	// Create a valid checkpoint
	state, _ := json.Marshal(CheckpointState{Value: 5})
	cp := checkpoint.New("validation-test", "node-a", 1, state, "node-b")
	data, _ := cp.Marshal()
	_ = store.Save("validation-test", "node-a", data)

	// Create graph
	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddNode("node-b", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", "node-b").
		AddEdge("node-b", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// Resume with validation that fails
	_, err = compiled.ResumeFrom(ctx, store, "validation-test", "node-a",
		flowgraph.WithStateValidation(func(s any) error {
			state := s.(CheckpointState)
			if state.Value < 100 {
				return errors.New("value must be at least 100")
			}
			return nil
		}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "state validation failed")
	assert.Contains(t, err.Error(), "value must be at least 100")
}

// TestResumeFrom_WithReplayInvalidNode tests ResumeFrom with replay option when node doesn't exist.
func TestResumeFrom_WithReplayInvalidNode(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	// Create a checkpoint with NextNode = END
	state, _ := json.Marshal(CheckpointState{Value: 10})
	cp := checkpoint.New("replay-test", "node-b", 2, state, flowgraph.END)
	data, _ := cp.Marshal()
	_ = store.Save("replay-test", "node-b", data)

	// Create graph WITHOUT node-b
	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// Attempt to resume with replay (startNode will be "node-b" which doesn't exist)
	_, err = compiled.ResumeFrom(ctx, store, "replay-test", "node-b",
		flowgraph.WithReplayNode())

	require.Error(t, err)
	assert.ErrorIs(t, err, flowgraph.ErrInvalidResumeNode)
	assert.Contains(t, err.Error(), "node-b")
}

// TestResumeFrom_CorruptedCheckpointJSON tests handling of corrupted checkpoint JSON.
func TestResumeFrom_CorruptedCheckpointJSON(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	// Save completely invalid JSON as checkpoint
	_ = store.Save("corrupt-test", "node-a", []byte(`{not valid json at all`))

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	_, err = compiled.ResumeFrom(ctx, store, "corrupt-test", "node-a")

	require.Error(t, err)
	assert.ErrorIs(t, err, flowgraph.ErrDeserializeState)
}

// TestResumeFrom_ValidENDAsNextNode tests that END is allowed as next node.
func TestResumeFrom_ValidENDAsNextNode(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	// Create checkpoint with NextNode = END (valid case)
	state, _ := json.Marshal(CheckpointState{Value: 10})
	cp := checkpoint.New("end-test", "node-a", 1, state, flowgraph.END)
	data, _ := cp.Marshal()
	_ = store.Save("end-test", "node-a", data)

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// This should succeed - END is a valid next node
	result, err := compiled.ResumeFrom(ctx, store, "end-test", "node-a")

	require.NoError(t, err)
	assert.Equal(t, 10, result.Value) // State should be unchanged since next node is END
}

// TestResume_NoCheckpoints tests Resume when no checkpoints exist for run ID.
func TestResume_NoCheckpoints(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// Attempt to resume from non-existent run
	_, err = compiled.Resume(ctx, store, "nonexistent-run-id")

	require.Error(t, err)
	assert.ErrorIs(t, err, flowgraph.ErrNoCheckpoints)
	assert.Contains(t, err.Error(), "nonexistent-run-id")
}

// TestResume_VersionMismatch tests Resume with version mismatch.
func TestResume_VersionMismatch(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	// Create checkpoint with wrong version
	state, _ := json.Marshal(CheckpointState{Value: 10})
	cp := &checkpoint.Checkpoint{
		Version:  42, // Wrong version
		RunID:    "version-mismatch",
		NodeID:   "node-a",
		Sequence: 1,
		State:    state,
		NextNode: flowgraph.END,
	}
	data, _ := cp.Marshal()
	_ = store.Save("version-mismatch", "node-a", data)

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	_, err = compiled.Resume(ctx, store, "version-mismatch")

	require.Error(t, err)
	assert.ErrorIs(t, err, flowgraph.ErrCheckpointVersionMismatch)
	assert.Contains(t, err.Error(), "got 42")
	assert.Contains(t, err.Error(), "expected 1")
}

// TestResume_StateDeserializationFailure tests Resume when state JSON is invalid.
func TestResume_StateDeserializationFailure(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	// Create checkpoint with invalid state JSON
	cp := &checkpoint.Checkpoint{
		Version:  checkpoint.Version,
		RunID:    "bad-state",
		NodeID:   "node-a",
		Sequence: 1,
		State:    []byte(`{"value": "not a number"}`), // value should be int
		NextNode: flowgraph.END,
	}
	data, _ := cp.Marshal()
	_ = store.Save("bad-state", "node-a", data)

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	_, err = compiled.Resume(ctx, store, "bad-state")

	require.Error(t, err)
	assert.ErrorIs(t, err, flowgraph.ErrDeserializeState)
}

// TestResume_WithInvalidResumeNode tests Resume when next node doesn't exist in graph.
func TestResume_WithInvalidResumeNode(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	// Create checkpoint with next node that doesn't exist
	state, _ := json.Marshal(CheckpointState{Value: 10})
	cp := checkpoint.New("invalid-next", "node-a", 1, state, "nonexistent-node")
	data, _ := cp.Marshal()
	_ = store.Save("invalid-next", "node-a", data)

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// Resume should fail because "nonexistent-node" doesn't exist
	// Note: Resume doesn't validate before execution, so we get a NodeError from executeNode
	_, err = compiled.Resume(ctx, store, "invalid-next")

	require.Error(t, err)
	// The error is wrapped in a NodeError from executeNode
	var nodeErr *flowgraph.NodeError
	assert.ErrorAs(t, err, &nodeErr)
	assert.Equal(t, "nonexistent-node", nodeErr.NodeID)
	assert.Contains(t, err.Error(), "node not found")
}

// TestResumeFrom_NilContext tests ResumeFrom with nil context.
func TestResumeFrom_NilContext(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.ResumeFrom(nil, store, "test-run", "node-a")

	require.Error(t, err)
	assert.ErrorIs(t, err, flowgraph.ErrNilContext)
}

// TestResume_NilContext tests Resume with nil context.
func TestResume_NilContext(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("node-a", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value++
			return s, nil
		}).
		AddEdge("node-a", flowgraph.END).
		SetEntry("node-a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	_, err = compiled.Resume(nil, store, "test-run")

	require.Error(t, err)
	assert.ErrorIs(t, err, flowgraph.ErrNilContext)
}
