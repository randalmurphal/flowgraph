package flowgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewGraph verifies basic graph creation.
func TestNewGraph(t *testing.T) {
	graph := NewGraph[Counter]()
	assert.NotNil(t, graph)
	assert.NotNil(t, graph.nodes)
	assert.NotNil(t, graph.edges)
	assert.NotNil(t, graph.conditionalEdges)
	assert.Empty(t, graph.entryPoint)
}

// TestGraph_AddNode tests successful node addition.
func TestGraph_AddNode(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddNode("b", increment)

	assert.Len(t, graph.nodes, 2)
	assert.Contains(t, graph.nodes, "a")
	assert.Contains(t, graph.nodes, "b")
}

// TestGraph_AddNode_Chaining tests fluent API chaining.
func TestGraph_AddNode_Chaining(t *testing.T) {
	graph := NewGraph[Counter]()
	result := graph.AddNode("a", increment)
	assert.Same(t, graph, result) // Should return same pointer for chaining
}

// TestGraph_AddNode_EmptyID_Panics tests that empty node ID panics.
func TestGraph_AddNode_EmptyID_Panics(t *testing.T) {
	assert.PanicsWithValue(t, "flowgraph: node ID cannot be empty", func() {
		NewGraph[Counter]().AddNode("", increment)
	})
}

// TestGraph_AddNode_ReservedID_Panics tests that reserved IDs panic.
func TestGraph_AddNode_ReservedID_Panics(t *testing.T) {
	testCases := []struct {
		name string
		id   string
	}{
		{"END uppercase", "END"},
		{"end lowercase", "end"},
		{"End mixed case", "End"},
		{"__end__ literal", "__end__"},
		{"__END__ uppercase", "__END__"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.PanicsWithValue(t, "flowgraph: node ID cannot be reserved word 'END'", func() {
				NewGraph[Counter]().AddNode(tc.id, increment)
			})
		})
	}
}

// TestGraph_AddNode_WhitespaceID_Panics tests that IDs with whitespace panic.
func TestGraph_AddNode_WhitespaceID_Panics(t *testing.T) {
	testCases := []struct {
		name string
		id   string
	}{
		{"space", "node a"},
		{"tab", "node\ta"},
		{"newline", "node\na"},
		{"leading space", " node"},
		{"trailing space", "node "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.PanicsWithValue(t, "flowgraph: node ID cannot contain whitespace", func() {
				NewGraph[Counter]().AddNode(tc.id, increment)
			})
		})
	}
}

// TestGraph_AddNode_NilFunc_Panics tests that nil function panics.
func TestGraph_AddNode_NilFunc_Panics(t *testing.T) {
	assert.PanicsWithValue(t, "flowgraph: node function cannot be nil", func() {
		NewGraph[Counter]().AddNode("a", nil)
	})
}

// TestGraph_AddNode_DuplicateID_Panics tests that duplicate IDs panic.
func TestGraph_AddNode_DuplicateID_Panics(t *testing.T) {
	assert.PanicsWithValue(t, "flowgraph: duplicate node ID: a", func() {
		NewGraph[Counter]().
			AddNode("a", increment).
			AddNode("a", increment)
	})
}

// TestGraph_AddNode_ValidIDs tests various valid node IDs.
func TestGraph_AddNode_ValidIDs(t *testing.T) {
	validIDs := []string{
		"a",
		"node1",
		"fetch-data",
		"process_input",
		"CamelCase",
		"node-with-many-dashes",
		"123",
		"_underscore",
	}

	for _, id := range validIDs {
		t.Run(id, func(t *testing.T) {
			graph := NewGraph[Counter]().AddNode(id, increment)
			assert.Contains(t, graph.nodes, id)
		})
	}
}

// TestGraph_AddEdge tests edge addition.
func TestGraph_AddEdge(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddNode("b", increment).
		AddEdge("a", "b").
		AddEdge("b", END)

	assert.Equal(t, []string{"b"}, graph.edges["a"])
	assert.Equal(t, []string{END}, graph.edges["b"])
}

// TestGraph_AddEdge_Chaining tests fluent API chaining.
func TestGraph_AddEdge_Chaining(t *testing.T) {
	graph := NewGraph[Counter]()
	result := graph.AddEdge("a", "b")
	assert.Same(t, graph, result)
}

// TestGraph_AddEdge_MultipleFromSameNode tests multiple edges from one node.
func TestGraph_AddEdge_MultipleFromSameNode(t *testing.T) {
	graph := NewGraph[Counter]().
		AddEdge("a", "b").
		AddEdge("a", "c")

	assert.Equal(t, []string{"b", "c"}, graph.edges["a"])
}

// TestGraph_AddConditionalEdge tests conditional edge addition.
func TestGraph_AddConditionalEdge(t *testing.T) {
	router := func(ctx Context, s Counter) string {
		if s.Value > 0 {
			return END
		}
		return "loop"
	}

	graph := NewGraph[Counter]().
		AddNode("check", increment).
		AddConditionalEdge("check", router)

	assert.NotNil(t, graph.conditionalEdges["check"])
}

// TestGraph_AddConditionalEdge_NilRouter_Panics tests that nil router panics.
func TestGraph_AddConditionalEdge_NilRouter_Panics(t *testing.T) {
	assert.PanicsWithValue(t, "flowgraph: router function cannot be nil", func() {
		NewGraph[Counter]().AddConditionalEdge("check", nil)
	})
}

// TestGraph_SetEntry tests entry point setting.
func TestGraph_SetEntry(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("start", increment).
		SetEntry("start")

	assert.Equal(t, "start", graph.entryPoint)
}

// TestGraph_SetEntry_Chaining tests fluent API chaining.
func TestGraph_SetEntry_Chaining(t *testing.T) {
	graph := NewGraph[Counter]()
	result := graph.SetEntry("start")
	assert.Same(t, graph, result)
}

// TestGraph_SetEntry_CanBeOverwritten tests that entry can be changed.
func TestGraph_SetEntry_CanBeOverwritten(t *testing.T) {
	graph := NewGraph[Counter]().
		SetEntry("first").
		SetEntry("second")

	assert.Equal(t, "second", graph.entryPoint)
}

// TestGraph_FluentAPI tests full fluent API usage.
func TestGraph_FluentAPI(t *testing.T) {
	graph := NewGraph[Counter]().
		AddNode("a", increment).
		AddNode("b", increment).
		AddNode("c", increment).
		AddEdge("a", "b").
		AddEdge("b", "c").
		AddEdge("c", END).
		SetEntry("a")

	assert.Len(t, graph.nodes, 3)
	assert.Equal(t, "a", graph.entryPoint)
	assert.Len(t, graph.edges, 3)
}
