package benchmarks

import (
	"testing"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
)

// State for benchmarks.
type State struct {
	Value int
}

// noopNode does minimal work to measure framework overhead.
func noopNode(ctx flowgraph.Context, s State) (State, error) {
	return s, nil
}

// BenchmarkNewGraph measures graph creation overhead.
func BenchmarkNewGraph(b *testing.B) {
	for i := 0; i < b.N; i++ {
		flowgraph.NewGraph[State]()
	}
}

// BenchmarkAddNode measures node addition overhead.
func BenchmarkAddNode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		graph := flowgraph.NewGraph[State]()
		graph.AddNode("node", noopNode)
	}
}

// BenchmarkAddNode_10 measures adding 10 nodes.
func BenchmarkAddNode_10(b *testing.B) {
	for i := 0; i < b.N; i++ {
		graph := flowgraph.NewGraph[State]()
		for j := 0; j < 10; j++ {
			graph.AddNode(nodeID(j), noopNode)
		}
	}
}

// BenchmarkAddNode_100 measures adding 100 nodes.
func BenchmarkAddNode_100(b *testing.B) {
	for i := 0; i < b.N; i++ {
		graph := flowgraph.NewGraph[State]()
		for j := 0; j < 100; j++ {
			graph.AddNode(nodeID(j), noopNode)
		}
	}
}

// BenchmarkCompile_Linear_5 compiles a 5-node linear graph.
func BenchmarkCompile_Linear_5(b *testing.B) {
	graph := buildLinearGraph(5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = graph.Compile()
	}
}

// BenchmarkCompile_Linear_10 compiles a 10-node linear graph.
func BenchmarkCompile_Linear_10(b *testing.B) {
	graph := buildLinearGraph(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = graph.Compile()
	}
}

// BenchmarkCompile_Linear_50 compiles a 50-node linear graph.
func BenchmarkCompile_Linear_50(b *testing.B) {
	graph := buildLinearGraph(50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = graph.Compile()
	}
}

// BenchmarkCompile_Linear_100 compiles a 100-node linear graph.
func BenchmarkCompile_Linear_100(b *testing.B) {
	graph := buildLinearGraph(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = graph.Compile()
	}
}

// BenchmarkCompile_Branching compiles a graph with conditional edges.
func BenchmarkCompile_Branching(b *testing.B) {
	graph := buildBranchingGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = graph.Compile()
	}
}

// Helper functions

func nodeID(n int) string {
	return string(rune('a'+n%26)) + string(rune('0'+n/26%10))
}

func buildLinearGraph(n int) *flowgraph.Graph[State] {
	graph := flowgraph.NewGraph[State]()
	for i := 0; i < n; i++ {
		graph.AddNode(nodeID(i), noopNode)
	}
	for i := 0; i < n-1; i++ {
		graph.AddEdge(nodeID(i), nodeID(i+1))
	}
	graph.AddEdge(nodeID(n-1), flowgraph.END)
	graph.SetEntry(nodeID(0))
	return graph
}

func buildBranchingGraph() *flowgraph.Graph[State] {
	router := func(ctx flowgraph.Context, s State) string {
		if s.Value%2 == 0 {
			return "even"
		}
		return "odd"
	}

	return flowgraph.NewGraph[State]().
		AddNode("start", noopNode).
		AddNode("even", noopNode).
		AddNode("odd", noopNode).
		AddNode("merge", noopNode).
		AddConditionalEdge("start", router).
		AddEdge("even", "merge").
		AddEdge("odd", "merge").
		AddEdge("merge", flowgraph.END).
		SetEntry("start")
}
