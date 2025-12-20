// Example: Loop/Retry pattern
//
// This example demonstrates a retry loop with max attempts.
// The graph attempts an operation up to 3 times before giving up.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
)

// State holds retry tracking data.
type State struct {
	Attempts int
	Success  bool
	Message  string
}

// attempt tries an operation that may fail.
func attempt(ctx flowgraph.Context, s State) (State, error) {
	s.Attempts++
	fmt.Printf("Attempt %d: ", s.Attempts)

	// Simulate operation with 40% success rate
	if rand.Float32() < 0.4 {
		s.Success = true
		fmt.Println("Success!")
	} else {
		fmt.Println("Failed, will retry...")
	}
	return s, nil
}

// handleSuccess processes successful completion.
func handleSuccess(ctx flowgraph.Context, s State) (State, error) {
	if s.Success {
		s.Message = fmt.Sprintf("Completed successfully after %d attempt(s)", s.Attempts)
	} else {
		s.Message = fmt.Sprintf("Failed after %d attempts, giving up", s.Attempts)
	}
	fmt.Println("Final:", s.Message)
	return s, nil
}

// retryRouter decides whether to retry or finish.
func retryRouter(ctx flowgraph.Context, s State) string {
	if s.Success {
		return "done"
	}
	if s.Attempts >= 3 {
		return "done" // Max retries reached
	}
	return "attempt" // Loop back
}

func main() {
	// Build the graph with retry loop.
	graph := flowgraph.NewGraph[State]().
		AddNode("attempt", attempt).
		AddNode("done", handleSuccess).
		AddConditionalEdge("attempt", retryRouter).
		AddEdge("done", flowgraph.END).
		SetEntry("attempt")

	compiled, err := graph.Compile()
	if err != nil {
		log.Fatal("compile error:", err)
	}

	ctx := flowgraph.NewContext(context.Background())

	// Run multiple times to show different outcomes
	for i := 1; i <= 3; i++ {
		fmt.Printf("\n=== Run %d ===\n", i)
		result, err := compiled.Run(ctx, State{})
		if err != nil {
			log.Fatal("run error:", err)
		}
		fmt.Printf("Final state: success=%v, attempts=%d\n", result.Success, result.Attempts)
	}
}
