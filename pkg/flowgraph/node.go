package flowgraph

// END is the terminal node identifier.
// Use this as an edge target to indicate the graph should terminate.
const END = "__end__"

// NodeFunc is the signature for all node functions.
// Nodes receive the execution context and current state,
// and return the updated state (or the same state) and any error.
//
// The state parameter is passed by value. Nodes should modify and return
// a new state value, not rely on pointer mutation.
//
// Example:
//
//	func increment(ctx flowgraph.Context, s Counter) (Counter, error) {
//	    s.Value++
//	    return s, nil
//	}
type NodeFunc[S any] func(ctx Context, state S) (S, error)

// RouterFunc determines the next node based on state.
// It is used for conditional edges where the next node depends on runtime state.
//
// The router should return a valid node ID or flowgraph.END.
// Returning an empty string or an unknown node ID will cause a runtime error.
//
// Example:
//
//	func router(ctx flowgraph.Context, s State) string {
//	    if s.Done {
//	        return flowgraph.END
//	    }
//	    return "process"
//	}
type RouterFunc[S any] func(ctx Context, state S) string
