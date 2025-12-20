// Example: Conditional branching
//
// This example demonstrates routing execution based on state values.
// A review node examines the score and routes to either approve or reject.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
)

// State holds review data.
type State struct {
	Code    string
	Score   int
	Outcome string
	Message string
}

// review examines the code and assigns a score.
func review(ctx flowgraph.Context, s State) (State, error) {
	// Simulate scoring based on code content
	if len(s.Code) > 50 {
		s.Score = 90
	} else if len(s.Code) > 20 {
		s.Score = 75
	} else {
		s.Score = 40
	}
	fmt.Printf("Review: Code scored %d/100\n", s.Score)
	return s, nil
}

// approve handles high-scoring code.
func approve(ctx flowgraph.Context, s State) (State, error) {
	s.Outcome = "approved"
	s.Message = "Code meets quality standards"
	fmt.Println("Approve: Code approved!")
	return s, nil
}

// requestChanges handles low-scoring code.
func requestChanges(ctx flowgraph.Context, s State) (State, error) {
	s.Outcome = "changes_requested"
	s.Message = "Please improve code quality"
	fmt.Println("Request Changes: Improvements needed")
	return s, nil
}

// router decides the next node based on score.
func router(ctx flowgraph.Context, s State) string {
	if s.Score >= 80 {
		return "approve"
	}
	return "request-changes"
}

func main() {
	// Build the graph with conditional branching.
	graph := flowgraph.NewGraph[State]().
		AddNode("review", review).
		AddNode("approve", approve).
		AddNode("request-changes", requestChanges).
		AddConditionalEdge("review", router).
		AddEdge("approve", flowgraph.END).
		AddEdge("request-changes", flowgraph.END).
		SetEntry("review")

	compiled, err := graph.Compile()
	if err != nil {
		log.Fatal("compile error:", err)
	}

	ctx := flowgraph.NewContext(context.Background())

	// Test with high-quality code (will be approved)
	fmt.Println("=== Test 1: High-quality code ===")
	result, err := compiled.Run(ctx, State{
		Code: "This is a well-written function with proper documentation and error handling patterns.",
	})
	if err != nil {
		log.Fatal("run error:", err)
	}
	fmt.Printf("Result: %s - %s\n\n", result.Outcome, result.Message)

	// Test with low-quality code (will request changes)
	fmt.Println("=== Test 2: Low-quality code ===")
	result, err = compiled.Run(ctx, State{
		Code: "short code",
	})
	if err != nil {
		log.Fatal("run error:", err)
	}
	fmt.Printf("Result: %s - %s\n", result.Outcome, result.Message)
}
