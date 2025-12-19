package benchmarks

import (
	"context"
	"testing"

	"github.com/rmurphy/flowgraph/pkg/flowgraph"
)

// BenchmarkRun_Linear_5 runs a 5-node linear graph.
func BenchmarkRun_Linear_5(b *testing.B) {
	compiled := mustCompile(buildLinearGraph(5))
	ctx := flowgraph.NewContext(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Run(ctx, State{})
	}
}

// BenchmarkRun_Linear_10 runs a 10-node linear graph.
func BenchmarkRun_Linear_10(b *testing.B) {
	compiled := mustCompile(buildLinearGraph(10))
	ctx := flowgraph.NewContext(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Run(ctx, State{})
	}
}

// BenchmarkRun_Linear_50 runs a 50-node linear graph.
func BenchmarkRun_Linear_50(b *testing.B) {
	compiled := mustCompile(buildLinearGraph(50))
	ctx := flowgraph.NewContext(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Run(ctx, State{})
	}
}

// BenchmarkRun_Linear_100 runs a 100-node linear graph.
func BenchmarkRun_Linear_100(b *testing.B) {
	compiled := mustCompile(buildLinearGraph(100))
	ctx := flowgraph.NewContext(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Run(ctx, State{})
	}
}

// BenchmarkRun_Branching runs a graph with conditional edges.
func BenchmarkRun_Branching(b *testing.B) {
	compiled := mustCompile(buildBranchingGraph())
	ctx := flowgraph.NewContext(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Run(ctx, State{Value: i})
	}
}

// BenchmarkRun_Loop runs a looping graph (3 iterations).
func BenchmarkRun_Loop(b *testing.B) {
	compiled := mustCompile(buildLoopGraph(3))
	ctx := flowgraph.NewContext(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Run(ctx, State{})
	}
}

// BenchmarkRun_Loop_10 runs a looping graph (10 iterations).
func BenchmarkRun_Loop_10(b *testing.B) {
	compiled := mustCompile(buildLoopGraph(10))
	ctx := flowgraph.NewContext(context.Background())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Run(ctx, State{})
	}
}

// BenchmarkContextCreation measures context creation overhead.
func BenchmarkContextCreation(b *testing.B) {
	bg := context.Background()
	for i := 0; i < b.N; i++ {
		flowgraph.NewContext(bg)
	}
}

// Helper functions

func mustCompile(g *flowgraph.Graph[State]) *flowgraph.CompiledGraph[State] {
	compiled, err := g.Compile()
	if err != nil {
		panic(err)
	}
	return compiled
}

func buildLoopGraph(maxIterations int) *flowgraph.Graph[State] {
	counter := 0
	loopNode := func(ctx flowgraph.Context, s State) (State, error) {
		s.Value++
		return s, nil
	}

	router := func(ctx flowgraph.Context, s State) string {
		counter++
		if counter >= maxIterations {
			counter = 0 // Reset for next run
			return "done"
		}
		return "loop"
	}

	return flowgraph.NewGraph[State]().
		AddNode("loop", loopNode).
		AddNode("done", noopNode).
		AddConditionalEdge("loop", router).
		AddEdge("done", flowgraph.END).
		SetEntry("loop")
}
