// Example: Linear flow execution
//
// This example demonstrates basic sequential graph execution.
// Three nodes process state in sequence: step1 -> step2 -> step3.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
)

// State holds data that flows through the graph.
type State struct {
	Input string
	Step1 string
	Step2 string
	Step3 string
}

// step1 processes the initial input.
func step1(ctx flowgraph.Context, s State) (State, error) {
	s.Step1 = "Processed: " + s.Input
	fmt.Println("Step 1:", s.Step1)
	return s, nil
}

// step2 validates the processed input.
func step2(ctx flowgraph.Context, s State) (State, error) {
	s.Step2 = "Validated: " + s.Step1
	fmt.Println("Step 2:", s.Step2)
	return s, nil
}

// step3 completes the workflow.
func step3(ctx flowgraph.Context, s State) (State, error) {
	s.Step3 = "Completed: " + s.Step2
	fmt.Println("Step 3:", s.Step3)
	return s, nil
}

func main() {
	// Build the graph with three nodes connected linearly.
	graph := flowgraph.NewGraph[State]().
		AddNode("step1", step1).
		AddNode("step2", step2).
		AddNode("step3", step3).
		AddEdge("step1", "step2").
		AddEdge("step2", "step3").
		AddEdge("step3", flowgraph.END).
		SetEntry("step1")

	// Compile validates the graph structure.
	compiled, err := graph.Compile()
	if err != nil {
		log.Fatal("compile error:", err)
	}

	// Create execution context.
	ctx := flowgraph.NewContext(context.Background())

	// Run the graph with initial state.
	result, err := compiled.Run(ctx, State{Input: "hello"})
	if err != nil {
		log.Fatal("run error:", err)
	}

	fmt.Println("\nFinal state:")
	fmt.Printf("  Input:  %s\n", result.Input)
	fmt.Printf("  Step1:  %s\n", result.Step1)
	fmt.Printf("  Step2:  %s\n", result.Step2)
	fmt.Printf("  Step3:  %s\n", result.Step3)
}
