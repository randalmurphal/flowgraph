package flowgraph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAcceptanceCriteria tests the exact example from SESSION_PROMPT.md
// This must work for Phase 1 to be considered complete.
func TestAcceptanceCriteria(t *testing.T) {
	type CounterState struct {
		Count int
	}

	increment := func(ctx Context, s CounterState) (CounterState, error) {
		s.Count++
		return s, nil
	}

	graph := NewGraph[CounterState]().
		AddNode("inc1", increment).
		AddNode("inc2", increment).
		AddNode("inc3", increment).
		AddEdge("inc1", "inc2").
		AddEdge("inc2", "inc3").
		AddEdge("inc3", END).
		SetEntry("inc1")

	compiled, err := graph.Compile()
	require.NoError(t, err, "graph should compile successfully")

	ctx := NewContext(context.Background())
	result, err := compiled.Run(ctx, CounterState{Count: 0})
	require.NoError(t, err, "graph should execute successfully")

	assert.Equal(t, 3, result.Count, "final count should be 3 after three increments")
}

// TestAcceptanceCriteria_WithInitialValue tests with non-zero initial state.
func TestAcceptanceCriteria_WithInitialValue(t *testing.T) {
	type CounterState struct {
		Count int
	}

	increment := func(ctx Context, s CounterState) (CounterState, error) {
		s.Count++
		return s, nil
	}

	graph := NewGraph[CounterState]().
		AddNode("inc1", increment).
		AddNode("inc2", increment).
		AddEdge("inc1", "inc2").
		AddEdge("inc2", END).
		SetEntry("inc1")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(NewContext(context.Background()), CounterState{Count: 10})
	require.NoError(t, err)

	assert.Equal(t, 12, result.Count) // 10 + 2 increments
}

// TestAcceptanceCriteria_ConditionalLoop tests conditional looping.
func TestAcceptanceCriteria_ConditionalLoop(t *testing.T) {
	type LoopState struct {
		Count  int
		Target int
	}

	increment := func(ctx Context, s LoopState) (LoopState, error) {
		s.Count++
		return s, nil
	}

	router := func(ctx Context, s LoopState) string {
		if s.Count >= s.Target {
			return END
		}
		return "inc"
	}

	graph := NewGraph[LoopState]().
		AddNode("inc", increment).
		AddConditionalEdge("inc", router).
		SetEntry("inc")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	result, err := compiled.Run(NewContext(context.Background()), LoopState{Count: 0, Target: 5})
	require.NoError(t, err)

	assert.Equal(t, 5, result.Count)
}

// TestAcceptanceCriteria_BranchAndJoin tests branching and joining paths.
func TestAcceptanceCriteria_BranchAndJoin(t *testing.T) {
	type BranchState struct {
		Path  string
		Value int
	}

	start := func(ctx Context, s BranchState) (BranchState, error) {
		s.Value = 1
		return s, nil
	}

	leftPath := func(ctx Context, s BranchState) (BranchState, error) {
		s.Path = "left"
		s.Value *= 2
		return s, nil
	}

	rightPath := func(ctx Context, s BranchState) (BranchState, error) {
		s.Path = "right"
		s.Value *= 3
		return s, nil
	}

	finish := func(ctx Context, s BranchState) (BranchState, error) {
		s.Value += 10
		return s, nil
	}

	router := func(ctx Context, s BranchState) string {
		if s.Value%2 == 0 {
			return "left"
		}
		return "right"
	}

	graph := NewGraph[BranchState]().
		AddNode("start", start).
		AddNode("left", leftPath).
		AddNode("right", rightPath).
		AddNode("finish", finish).
		AddConditionalEdge("start", router).
		AddEdge("left", "finish").
		AddEdge("right", "finish").
		AddEdge("finish", END).
		SetEntry("start")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	// Value=1 is odd, so goes right, value becomes 3, then +10 = 13
	result, err := compiled.Run(NewContext(context.Background()), BranchState{})
	require.NoError(t, err)

	assert.Equal(t, "right", result.Path)
	assert.Equal(t, 13, result.Value)
}

// TestAcceptanceCriteria_ReusableCompiledGraph tests compiled graph reuse.
func TestAcceptanceCriteria_ReusableCompiledGraph(t *testing.T) {
	type Counter struct {
		Value int
	}

	increment := func(ctx Context, s Counter) (Counter, error) {
		s.Value++
		return s, nil
	}

	graph := NewGraph[Counter]().
		AddNode("inc", increment).
		AddEdge("inc", END).
		SetEntry("inc")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	// Run multiple times with different initial states
	results := make([]int, 3)
	for i := 0; i < 3; i++ {
		result, err := compiled.Run(NewContext(context.Background()), Counter{Value: i * 10})
		require.NoError(t, err)
		results[i] = result.Value
	}

	assert.Equal(t, []int{1, 11, 21}, results)
}
