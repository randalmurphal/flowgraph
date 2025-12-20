// Example: Checkpointing and crash recovery
//
// This example demonstrates saving checkpoints and resuming after crashes.
// Uses SQLite for persistent storage across process restarts.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
	"github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
)

// State tracks pipeline progress.
type State struct {
	Input       string
	Step1Done   bool
	Step2Done   bool
	Step3Done   bool
	FinalResult string
}

func step1(ctx flowgraph.Context, s State) (State, error) {
	fmt.Println("Step 1: Processing input...")
	time.Sleep(100 * time.Millisecond) // Simulate work
	s.Step1Done = true
	return s, nil
}

func step2(ctx flowgraph.Context, s State) (State, error) {
	fmt.Println("Step 2: Transforming data...")
	time.Sleep(100 * time.Millisecond)
	s.Step2Done = true
	return s, nil
}

func step3(ctx flowgraph.Context, s State) (State, error) {
	fmt.Println("Step 3: Finalizing...")
	time.Sleep(100 * time.Millisecond)
	s.Step3Done = true
	s.FinalResult = "Processed: " + s.Input
	return s, nil
}

func buildGraph() *flowgraph.CompiledGraph[State] {
	graph := flowgraph.NewGraph[State]().
		AddNode("step1", step1).
		AddNode("step2", step2).
		AddNode("step3", step3).
		AddEdge("step1", "step2").
		AddEdge("step2", "step3").
		AddEdge("step3", flowgraph.END).
		SetEntry("step1")

	compiled, err := graph.Compile()
	if err != nil {
		log.Fatal("compile error:", err)
	}
	return compiled
}

func main() {
	// Create SQLite checkpoint store
	dbPath := "./checkpoints.db"
	store, err := checkpoint.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatal("failed to create store:", err)
	}
	defer store.Close()
	defer os.Remove(dbPath) // Clean up for demo

	compiled := buildGraph()
	ctx := flowgraph.NewContext(context.Background())
	runID := "demo-run-001"

	fmt.Println("=== First Run: Normal execution with checkpointing ===")
	result, err := compiled.Run(ctx, State{Input: "hello world"},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID(runID),
	)
	if err != nil {
		log.Fatal("run error:", err)
	}
	fmt.Printf("Result: %s\n", result.FinalResult)
	fmt.Printf("All steps complete: step1=%v, step2=%v, step3=%v\n\n",
		result.Step1Done, result.Step2Done, result.Step3Done)

	// List checkpoints
	fmt.Println("=== Checkpoints saved ===")
	checkpoints, err := store.List(runID)
	if err != nil {
		log.Fatal("list error:", err)
	}
	for _, cp := range checkpoints {
		fmt.Printf("  - After node: %s (at %s)\n", cp.NodeID, cp.Timestamp.Format(time.RFC3339))
	}

	// Demonstrate resume (simulating a crash scenario)
	fmt.Println("\n=== Simulating Resume Scenario ===")
	fmt.Println("In a real crash, you would call Resume() to continue from last checkpoint")

	// Create a new run to demonstrate resume
	runID2 := "demo-run-002"
	result2, err := compiled.Run(ctx, State{Input: "another input"},
		flowgraph.WithCheckpointing(store),
		flowgraph.WithRunID(runID2),
	)
	if err != nil {
		log.Fatal("run error:", err)
	}
	fmt.Printf("Second run result: %s\n", result2.FinalResult)

	// Show how Resume would work
	fmt.Println("\n=== Resume Example ===")
	fmt.Println("To resume after crash:")
	fmt.Println("  result, err := compiled.Resume(ctx, store, \"run-id\")")
	fmt.Println("This continues execution from the node after the last checkpoint.")
}
