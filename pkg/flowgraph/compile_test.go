package flowgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompile_LinearGraph tests successful compilation of a linear graph.
func TestCompile_LinearGraph(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddNode("b", increment).
		AddNode("c", increment).
		AddEdge("a", "b").
		AddEdge("b", "c").
		AddEdge("c", END).
		SetEntry("a")

	compiled, err := graph.Compile()

	require.NoError(t, err)
	assert.NotNil(t, compiled)
	assert.Equal(t, "a", compiled.EntryPoint())
	assert.ElementsMatch(t, []string{"a", "b", "c"}, compiled.NodeIDs())
}

// TestCompile_SingleNodeGraph tests graph with single node.
func TestCompile_SingleNodeGraph(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("only", increment).
		AddEdge("only", END).
		SetEntry("only")

	compiled, err := graph.Compile()

	require.NoError(t, err)
	assert.Equal(t, []string{"only"}, compiled.NodeIDs())
}

// TestCompile_BranchingGraph tests graph with conditional branching.
func TestCompile_BranchingGraph(t *testing.T) {
	router := func(ctx Context, s State) string {
		if s.GoLeft {
			return "left"
		}
		return "right"
	}

	graph := NewGraph[State]().
		AddNode("start", passthrough[State]).
		AddNode("left", passthrough[State]).
		AddNode("right", passthrough[State]).
		AddNode("join", passthrough[State]).
		AddConditionalEdge("start", router).
		AddEdge("left", "join").
		AddEdge("right", "join").
		AddEdge("join", END).
		SetEntry("start")

	compiled, err := graph.Compile()

	require.NoError(t, err)
	assert.True(t, compiled.IsConditional("start"))
	assert.False(t, compiled.IsConditional("left"))
	assert.False(t, compiled.IsConditional("right"))
}

// TestCompile_ValidCycle tests that cycles with conditional exit compile.
func TestCompile_ValidCycle(t *testing.T) {
	router := func(ctx Context, s State) string {
		if s.Done {
			return END
		}
		return "process"
	}

	graph := NewGraph[State]().
		AddNode("check", passthrough[State]).
		AddNode("process", passthrough[State]).
		AddConditionalEdge("check", router).
		AddEdge("process", "check").
		SetEntry("check")

	compiled, err := graph.Compile()

	require.NoError(t, err)
	assert.NotNil(t, compiled)
}

// TestCompile_SelfLoop_WithExit tests self-loop with conditional exit.
func TestCompile_SelfLoop_WithExit(t *testing.T) {
	router := func(ctx Context, s State) string {
		if s.Done {
			return END
		}
		return "loop"
	}

	graph := NewGraph[State]().
		AddNode("loop", passthrough[State]).
		AddConditionalEdge("loop", router).
		SetEntry("loop")

	compiled, err := graph.Compile()

	require.NoError(t, err)
	assert.NotNil(t, compiled)
}

// TestCompile_NoEntryPoint_Error tests missing entry point error.
func TestCompile_NoEntryPoint_Error(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddEdge("a", END)
	// No SetEntry()

	_, err := graph.Compile()

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoEntryPoint)
}

// TestCompile_EntryNotFound_Error tests entry point referencing missing node.
func TestCompile_EntryNotFound_Error(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddEdge("a", END).
		SetEntry("nonexistent")

	_, err := graph.Compile()

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrEntryNotFound)
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestCompile_MissingEdgeTarget_Error tests edge to missing node.
func TestCompile_MissingEdgeTarget_Error(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddEdge("a", "nonexistent").
		SetEntry("a")

	_, err := graph.Compile()

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNodeNotFound)
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestCompile_MissingEdgeSource_Error tests edge from missing node.
func TestCompile_MissingEdgeSource_Error(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddEdge("nonexistent", "a").
		AddEdge("a", END).
		SetEntry("a")

	_, err := graph.Compile()

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNodeNotFound)
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestCompile_NoPathToEnd_Error tests dead-end node error.
func TestCompile_NoPathToEnd_Error(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddNode("b", increment).
		AddEdge("a", "b").
		// b has no outgoing edge - dead end
		SetEntry("a")

	_, err := graph.Compile()

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoPathToEnd)
}

// TestCompile_MultipleErrors_AllReturned tests error aggregation.
func TestCompile_MultipleErrors_AllReturned(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddEdge("a", "missing1").
		AddEdge("missing2", END)
	// No entry point

	_, err := graph.Compile()

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoEntryPoint)
	assert.ErrorIs(t, err, ErrNodeNotFound)
	// Should contain info about both missing nodes
	assert.Contains(t, err.Error(), "missing1")
	assert.Contains(t, err.Error(), "missing2")
}

// TestCompile_ConditionalEdgeSourceNotFound_Error tests missing conditional edge source.
func TestCompile_ConditionalEdgeSourceNotFound_Error(t *testing.T) {
	router := func(ctx Context, s Counter) string { return END }

	graph := NewGraph[Counter]().
		AddConditionalEdge("nonexistent", router).
		SetEntry("nonexistent")

	_, err := graph.Compile()

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNodeNotFound)
}

// TestCompiledGraph_Introspection tests compiled graph introspection methods.
func TestCompiledGraph_Introspection(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("start", increment).
		AddNode("middle", increment).
		AddNode("finish", increment).
		AddEdge("start", "middle").
		AddEdge("middle", "finish").
		AddEdge("finish", END).
		SetEntry("start")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	// EntryPoint
	assert.Equal(t, "start", compiled.EntryPoint())

	// NodeIDs
	assert.Len(t, compiled.NodeIDs(), 3)
	assert.ElementsMatch(t, []string{"start", "middle", "finish"}, compiled.NodeIDs())

	// HasNode
	assert.True(t, compiled.HasNode("start"))
	assert.True(t, compiled.HasNode("middle"))
	assert.True(t, compiled.HasNode("finish"))
	assert.False(t, compiled.HasNode("nonexistent"))

	// Successors
	assert.Equal(t, []string{"middle"}, compiled.Successors("start"))
	assert.Equal(t, []string{"finish"}, compiled.Successors("middle"))
	assert.Equal(t, []string{END}, compiled.Successors("finish"))
	assert.Nil(t, compiled.Successors(END))
	assert.Nil(t, compiled.Successors("nonexistent"))

	// Predecessors
	assert.Equal(t, []string{"start"}, compiled.Predecessors("middle"))
	assert.Equal(t, []string{"middle"}, compiled.Predecessors("finish"))
	assert.Nil(t, compiled.Predecessors("start")) // Entry has no predecessors

	// IsConditional
	assert.False(t, compiled.IsConditional("start"))
	assert.False(t, compiled.IsConditional("middle"))
}

// TestCompiledGraph_IsConditional tests conditional node detection.
func TestCompiledGraph_IsConditional(t *testing.T) {
	router := func(ctx Context, s Counter) string { return END }

	graph := NewGraph[Counter]().
		AddNode("start", increment).
		AddNode("loop", increment).
		AddEdge("start", "loop").
		AddConditionalEdge("loop", router).
		SetEntry("start")

	compiled, err := graph.Compile()
	require.NoError(t, err)

	assert.False(t, compiled.IsConditional("start"))
	assert.True(t, compiled.IsConditional("loop"))
}

// TestCompile_RecompilingDoesNotAffectPrevious tests immutability.
func TestCompile_RecompilingDoesNotAffectPrevious(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddEdge("a", END).
		SetEntry("a")

	compiled1, err := graph.Compile()
	require.NoError(t, err)

	// Modify the builder
	graph.AddNode("b", increment).
		AddEdge("a", "b").
		AddEdge("b", END)

	compiled2, err := graph.Compile()
	require.NoError(t, err)

	// compiled1 should be unchanged
	assert.Equal(t, 1, len(compiled1.NodeIDs()))
	assert.Equal(t, 2, len(compiled2.NodeIDs()))
}

// TestCompile_EmptyGraph_Error tests compiling empty graph.
func TestCompile_EmptyGraph_Error(t *testing.T) {
	graph := NewGraph[Counter]()

	_, err := graph.Compile()

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoEntryPoint)
}

// TestCompile_OnlyEntrySet_Error tests graph with only entry set.
func TestCompile_OnlyEntrySet_Error(t *testing.T) {
	graph := NewGraph[Counter]().
		SetEntry("nonexistent")

	_, err := graph.Compile()

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrEntryNotFound)
}

// TestCompile_NodeToEND tests direct edge to END.
func TestCompile_NodeToEND(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddEdge("a", END).
		SetEntry("a")

	compiled, err := graph.Compile()

	require.NoError(t, err)
	assert.Equal(t, []string{END}, compiled.Successors("a"))
}
