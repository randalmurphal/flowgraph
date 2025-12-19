package flowgraph

import (
	"context"
)

// Test state types used across tests

// Counter is a simple state for testing incrementing.
type Counter struct {
	Value int
}

// State is a more complex state for testing various scenarios.
type State struct {
	Step      int
	Progress  []string
	Initial   string
	Output    string
	Done      bool
	GoLeft    bool
	Completed []string
	Count     int
}

// Helper node functions

// increment is a node that increments the counter.
func increment(ctx Context, s Counter) (Counter, error) {
	s.Value++
	return s, nil
}

// passthrough returns the state unchanged.
func passthrough[S any](ctx Context, s S) (S, error) {
	return s, nil
}

// makeTrackingNode creates a node that records its execution.
func makeTrackingNode(name string, tracker *[]string) NodeFunc[State] {
	return func(ctx Context, s State) (State, error) {
		*tracker = append(*tracker, name)
		s.Progress = append(s.Progress, name)
		return s, nil
	}
}

// makeFailingNode creates a node that returns the given error.
func makeFailingNode(err error) NodeFunc[State] {
	return func(ctx Context, s State) (State, error) {
		return s, err
	}
}

// makePanicNode creates a node that panics with the given value.
func makePanicNode(value any) NodeFunc[State] {
	return func(ctx Context, s State) (State, error) {
		panic(value)
	}
}

// testCtx creates a simple test context.
func testCtx() Context {
	return NewContext(context.Background())
}
