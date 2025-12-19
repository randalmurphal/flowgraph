// Example: Observability - logging, metrics, and tracing
//
// This example demonstrates enabling observability features for monitoring.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/rmurphy/flowgraph/pkg/flowgraph"
)

// State holds processing data.
type State struct {
	Input     string
	Processed bool
	Output    string
}

func process(ctx flowgraph.Context, s State) (State, error) {
	// Simulate some work
	time.Sleep(50 * time.Millisecond)
	s.Processed = true
	s.Output = "Processed: " + s.Input
	return s, nil
}

func validate(ctx flowgraph.Context, s State) (State, error) {
	time.Sleep(30 * time.Millisecond)
	if !s.Processed {
		return s, fmt.Errorf("validation failed: not processed")
	}
	return s, nil
}

func finalize(ctx flowgraph.Context, s State) (State, error) {
	time.Sleep(20 * time.Millisecond)
	s.Output += " [finalized]"
	return s, nil
}

func main() {
	// Configure structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Build the graph
	graph := flowgraph.NewGraph[State]().
		AddNode("process", process).
		AddNode("validate", validate).
		AddNode("finalize", finalize).
		AddEdge("process", "validate").
		AddEdge("validate", "finalize").
		AddEdge("finalize", flowgraph.END).
		SetEntry("process")

	compiled, err := graph.Compile()
	if err != nil {
		log.Fatal("compile error:", err)
	}

	ctx := flowgraph.NewContext(context.Background())

	fmt.Println("=== Running with full observability ===")
	fmt.Println("(JSON logs will include run_id, node_id, duration_ms)")
	fmt.Println()

	// Run with all observability features enabled
	result, err := compiled.Run(ctx, State{Input: "hello"},
		flowgraph.WithObservabilityLogger(logger),
		flowgraph.WithMetrics(true),
		flowgraph.WithTracing(true),
		flowgraph.WithRunID("obs-demo-001"),
	)
	if err != nil {
		log.Fatal("run error:", err)
	}

	fmt.Println()
	fmt.Println("=== Result ===")
	fmt.Printf("Output: %s\n", result.Output)
	fmt.Println()

	// Show what observability provides
	fmt.Println("=== Observability Features ===")
	fmt.Println()
	fmt.Println("1. LOGGING")
	fmt.Println("   - Structured JSON with consistent fields")
	fmt.Println("   - Fields: run_id, node_id, duration_ms, attempt")
	fmt.Println("   - Events: run_start, node_start, node_complete, run_complete")
	fmt.Println()
	fmt.Println("2. METRICS (OpenTelemetry)")
	fmt.Println("   - flowgraph.node.executions (counter)")
	fmt.Println("   - flowgraph.node.latency_ms (histogram)")
	fmt.Println("   - flowgraph.node.errors (counter)")
	fmt.Println("   - flowgraph.graph.runs (counter)")
	fmt.Println("   - flowgraph.graph.latency_ms (histogram)")
	fmt.Println()
	fmt.Println("3. TRACING (OpenTelemetry)")
	fmt.Println("   - Parent span: flowgraph.run")
	fmt.Println("   - Child spans: flowgraph.node.{id}")
	fmt.Println("   - Attributes: run_id, node_id, graph_id")
}
