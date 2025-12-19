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

// TestState for checkpoint integration tests.
type CheckpointState struct {
	Value    int      `json:"value"`
	Messages []string `json:"messages"`
}

func TestCheckpointing_BasicExecution(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	increment := func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
		s.Value++
		s.Messages = append(s.Messages, "incremented")
		return s, nil
	}

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("inc1", increment).
		AddNode("inc2", increment).
		AddEdge("inc1", "inc2").
		AddEdge("inc2", flowgraph.END).
		SetEntry("inc1")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())
	result, err := compiled.Run(ctx, CheckpointState{Value: 0},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID("test-run-1"))

	require.NoError(t, err)
	assert.Equal(t, 2, result.Value)
	assert.Equal(t, []string{"incremented", "incremented"}, result.Messages)

	// Verify checkpoints were created
	infos, err := store.List("test-run-1")
	require.NoError(t, err)
	assert.Len(t, infos, 2) // One checkpoint per node
}

func TestCheckpointing_RequiresRunID(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	noop := func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
		return s, nil
	}

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("noop", noop).
		AddEdge("noop", flowgraph.END).
		SetEntry("noop")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())
	_, err = compiled.Run(ctx, CheckpointState{},
		flowgraph.WithCheckpointing(store)) // No WithRunID!

	assert.ErrorIs(t, err, flowgraph.ErrRunIDRequired)
}

func TestCheckpointing_Resume(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	var executedNodes []string
	makeNode := func(name string, fail bool) flowgraph.NodeFunc[CheckpointState] {
		return func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			executedNodes = append(executedNodes, name)
			s.Value++
			if fail {
				return s, errors.New("intentional failure")
			}
			return s, nil
		}
	}

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("a", makeNode("a", false)).
		AddNode("b", makeNode("b", false)).
		AddNode("c", makeNode("c", false)).
		AddEdge("a", "b").
		AddEdge("b", "c").
		AddEdge("c", flowgraph.END).
		SetEntry("a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	// Simulate first run that completes successfully
	ctx := flowgraph.NewContext(context.Background())
	_, err = compiled.Run(ctx, CheckpointState{},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID("resume-test"))

	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, executedNodes)

	// Clear and resume from checkpoint
	executedNodes = nil

	// Resume should start from after the last checkpoint (c -> END)
	result, err := compiled.Resume(ctx, store, "resume-test")
	require.NoError(t, err)

	// Since last checkpoint was at "c" with next node as END, nothing should execute
	assert.Empty(t, executedNodes)
	assert.Equal(t, 3, result.Value)
}

func TestCheckpointing_ResumeAfterCrash(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	var executedNodes []string
	crashOnB := true

	makeNode := func(name string) flowgraph.NodeFunc[CheckpointState] {
		return func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			executedNodes = append(executedNodes, name)
			s.Value++
			if name == "b" && crashOnB {
				return s, errors.New("crash")
			}
			return s, nil
		}
	}

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("a", makeNode("a")).
		AddNode("b", makeNode("b")).
		AddNode("c", makeNode("c")).
		AddEdge("a", "b").
		AddEdge("b", "c").
		AddEdge("c", flowgraph.END).
		SetEntry("a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// First run crashes on node b
	_, err = compiled.Run(ctx, CheckpointState{},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID("crash-test"))

	require.Error(t, err)
	assert.Equal(t, []string{"a", "b"}, executedNodes)

	// Checkpoint at "a" should exist (b failed, so no checkpoint for b)
	infos, _ := store.List("crash-test")
	require.Len(t, infos, 1)
	assert.Equal(t, "a", infos[0].NodeID)

	// Fix the crash and resume
	crashOnB = false
	executedNodes = nil

	result, err := compiled.Resume(ctx, store, "crash-test")
	require.NoError(t, err)

	// Should resume from node b (after checkpoint at a)
	assert.Equal(t, []string{"b", "c"}, executedNodes)
	assert.Equal(t, 3, result.Value) // a(1) + b(2) + c(3) - but state from checkpoint was 1
}

func TestCheckpointing_ResumeFrom(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	var executedNodes []string
	makeNode := func(name string) flowgraph.NodeFunc[CheckpointState] {
		return func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			executedNodes = append(executedNodes, name)
			s.Value++
			return s, nil
		}
	}

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("a", makeNode("a")).
		AddNode("b", makeNode("b")).
		AddNode("c", makeNode("c")).
		AddEdge("a", "b").
		AddEdge("b", "c").
		AddEdge("c", flowgraph.END).
		SetEntry("a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// Run to completion
	_, err = compiled.Run(ctx, CheckpointState{},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID("resume-from-test"))
	require.NoError(t, err)

	// Resume from a specific checkpoint (node "a")
	executedNodes = nil
	result, err := compiled.ResumeFrom(ctx, store, "resume-from-test", "a")
	require.NoError(t, err)

	// Should start from node after "a" checkpoint (which is "b")
	assert.Equal(t, []string{"b", "c"}, executedNodes)
	assert.Equal(t, 3, result.Value)
}

func TestCheckpointing_WithStateOverride(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	noop := func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
		return s, nil
	}

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("noop", noop).
		AddEdge("noop", flowgraph.END).
		SetEntry("noop")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// Create initial state with checkpoints
	_, err = compiled.Run(ctx, CheckpointState{Value: 10},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID("override-test"))
	require.NoError(t, err)

	// Resume with state override
	result, err := compiled.Resume(ctx, store, "override-test",
		flowgraph.WithStateOverride(func(s any) any {
			state := s.(CheckpointState)
			state.Value = 999
			return state
		}))
	require.NoError(t, err)
	assert.Equal(t, 999, result.Value)
}

func TestCheckpointing_WithStateValidation(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	noop := func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
		return s, nil
	}

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("noop", noop).
		AddEdge("noop", flowgraph.END).
		SetEntry("noop")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// Create checkpoint
	_, err = compiled.Run(ctx, CheckpointState{Value: 10},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID("validate-test"))
	require.NoError(t, err)

	// Resume with validation that fails
	_, err = compiled.Resume(ctx, store, "validate-test",
		flowgraph.WithStateValidation(func(s any) error {
			state := s.(CheckpointState)
			if state.Value < 100 {
				return errors.New("value too small")
			}
			return nil
		}))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "value too small")
}

func TestCheckpointing_WithReplayNode(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	var executedNodes []string
	makeNode := func(name string) flowgraph.NodeFunc[CheckpointState] {
		return func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			executedNodes = append(executedNodes, name)
			s.Value++
			return s, nil
		}
	}

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("a", makeNode("a")).
		AddNode("b", makeNode("b")).
		AddEdge("a", "b").
		AddEdge("b", flowgraph.END).
		SetEntry("a")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())

	// Run to completion
	_, err = compiled.Run(ctx, CheckpointState{},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID("replay-test"))
	require.NoError(t, err)

	// Resume with replay (should re-execute the checkpointed node)
	executedNodes = nil
	result, err := compiled.Resume(ctx, store, "replay-test",
		flowgraph.WithReplayNode())
	require.NoError(t, err)

	// Should replay "b" (latest checkpoint) even though next node is END
	assert.Equal(t, []string{"b"}, executedNodes)
	assert.Equal(t, 3, result.Value) // Original 2 + replay 1
}

func TestCheckpointing_NoCheckpoints(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	ctx := flowgraph.NewContext(context.Background())
	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("noop", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			return s, nil
		}).
		AddEdge("noop", flowgraph.END).
		SetEntry("noop")

	compiled, _ := graph.Compile()

	_, err := compiled.Resume(ctx, store, "nonexistent-run")
	assert.ErrorIs(t, err, flowgraph.ErrNoCheckpoints)
}

func TestCheckpointing_CheckpointData(t *testing.T) {
	store := checkpoint.NewMemoryStore()

	graph := flowgraph.NewGraph[CheckpointState]().
		AddNode("process", func(ctx flowgraph.Context, s CheckpointState) (CheckpointState, error) {
			s.Value = 42
			s.Messages = []string{"processed"}
			return s, nil
		}).
		AddEdge("process", flowgraph.END).
		SetEntry("process")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	ctx := flowgraph.NewContext(context.Background())
	_, err = compiled.Run(ctx, CheckpointState{},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID("data-test"))
	require.NoError(t, err)

	// Load and verify checkpoint data
	data, err := store.Load("data-test", "process")
	require.NoError(t, err)

	cp, err := checkpoint.Unmarshal(data)
	require.NoError(t, err)

	assert.Equal(t, "data-test", cp.RunID)
	assert.Equal(t, "process", cp.NodeID)
	assert.Equal(t, flowgraph.END, cp.NextNode)
	assert.Equal(t, 1, cp.Sequence)

	// Verify state in checkpoint
	var state CheckpointState
	err = json.Unmarshal(cp.State, &state)
	require.NoError(t, err)
	assert.Equal(t, 42, state.Value)
	assert.Equal(t, []string{"processed"}, state.Messages)
}
