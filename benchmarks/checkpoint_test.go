package benchmarks

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/rmurphy/flowgraph/pkg/flowgraph"
	"github.com/rmurphy/flowgraph/pkg/flowgraph/checkpoint"
)

// LargeState represents a larger state for realistic benchmarks.
type LargeState struct {
	ID       string
	Values   []int
	Metadata map[string]string
	Nested   struct {
		A string
		B int
		C []string
	}
}

// BenchmarkMemoryStore_Save measures in-memory checkpoint save.
func BenchmarkMemoryStore_Save(b *testing.B) {
	store := checkpoint.NewMemoryStore()
	state := createLargeState()
	data, _ := json.Marshal(state)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Save("run-1", "node-1", data)
	}
}

// BenchmarkMemoryStore_Load measures in-memory checkpoint load.
func BenchmarkMemoryStore_Load(b *testing.B) {
	store := checkpoint.NewMemoryStore()
	state := createLargeState()
	data, _ := json.Marshal(state)
	_ = store.Save("run-1", "node-1", data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Load("run-1", "node-1")
	}
}

// BenchmarkSQLiteStore_Save measures SQLite checkpoint save.
func BenchmarkSQLiteStore_Save(b *testing.B) {
	store, cleanup := createSQLiteStore(b)
	defer cleanup()

	state := createLargeState()
	data, _ := json.Marshal(state)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = store.Save("run-1", nodeID(i%100), data)
	}
}

// BenchmarkSQLiteStore_Load measures SQLite checkpoint load.
func BenchmarkSQLiteStore_Load(b *testing.B) {
	store, cleanup := createSQLiteStore(b)
	defer cleanup()

	state := createLargeState()
	data, _ := json.Marshal(state)
	_ = store.Save("run-1", "node-1", data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Load("run-1", "node-1")
	}
}

// BenchmarkRun_WithCheckpointing measures execution with checkpointing enabled.
func BenchmarkRun_WithCheckpointing(b *testing.B) {
	store := checkpoint.NewMemoryStore()
	compiled := mustCompileState(buildLinearGraphState(5))
	ctx := flowgraph.NewContext(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Run(ctx, LargeState{},
			flowgraph.WithCheckpointing(store),
			flowgraph.WithRunID("run-"+nodeID(i)),
		)
	}
}

// BenchmarkRun_WithoutCheckpointing baseline without checkpointing.
func BenchmarkRun_WithoutCheckpointing(b *testing.B) {
	compiled := mustCompileState(buildLinearGraphState(5))
	ctx := flowgraph.NewContext(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Run(ctx, LargeState{})
	}
}

// BenchmarkJSONMarshal measures state serialization overhead.
func BenchmarkJSONMarshal(b *testing.B) {
	state := createLargeState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(state)
	}
}

// BenchmarkJSONUnmarshal measures state deserialization overhead.
func BenchmarkJSONUnmarshal(b *testing.B) {
	state := createLargeState()
	data, _ := json.Marshal(state)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var s LargeState
		_ = json.Unmarshal(data, &s)
	}
}

// Helper functions

func createLargeState() LargeState {
	return LargeState{
		ID:     "test-id",
		Values: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
		Nested: struct {
			A string
			B int
			C []string
		}{
			A: "nested-a",
			B: 42,
			C: []string{"c1", "c2", "c3"},
		},
	}
}

func createSQLiteStore(b *testing.B) (*checkpoint.SQLiteStore, func()) {
	b.Helper()
	tmpFile, err := os.CreateTemp("", "bench-*.db")
	if err != nil {
		b.Fatal(err)
	}
	tmpFile.Close()

	store, err := checkpoint.NewSQLiteStore(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		b.Fatal(err)
	}

	return store, func() {
		store.Close()
		os.Remove(tmpFile.Name())
	}
}

func noopNodeState(ctx flowgraph.Context, s LargeState) (LargeState, error) {
	return s, nil
}

func buildLinearGraphState(n int) *flowgraph.Graph[LargeState] {
	graph := flowgraph.NewGraph[LargeState]()
	for i := 0; i < n; i++ {
		graph.AddNode(nodeID(i), noopNodeState)
	}
	for i := 0; i < n-1; i++ {
		graph.AddEdge(nodeID(i), nodeID(i+1))
	}
	graph.AddEdge(nodeID(n-1), flowgraph.END)
	graph.SetEntry(nodeID(0))
	return graph
}

func mustCompileState(g *flowgraph.Graph[LargeState]) *flowgraph.CompiledGraph[LargeState] {
	compiled, err := g.Compile()
	if err != nil {
		panic(err)
	}
	return compiled
}
