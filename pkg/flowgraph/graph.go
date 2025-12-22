package flowgraph

import (
	"fmt"
	"strings"
	"sync"
)

// Graph is a mutable builder for creating execution graphs.
// Use NewGraph to create a new graph, then chain AddNode, AddEdge,
// and SetEntry calls to define the workflow.
//
// Graph is NOT thread-safe during building. Use a single goroutine
// to construct the graph, then call Compile() to create an immutable
// CompiledGraph that can be safely shared.
//
// Example:
//
//	graph := flowgraph.NewGraph[MyState]().
//	    AddNode("fetch", fetchNode).
//	    AddNode("process", processNode).
//	    AddEdge("fetch", "process").
//	    AddEdge("process", flowgraph.END).
//	    SetEntry("fetch")
//
//	compiled, err := graph.Compile()
//
// For parallel execution (fork/join), add multiple edges from a single node:
//
//	graph.AddEdge("dispatch", "workerA").
//	    AddEdge("dispatch", "workerB").
//	    AddEdge("workerA", "collect").
//	    AddEdge("workerB", "collect")
//
// The compiler will detect "dispatch" as a fork node and "collect" as the join.
type Graph[S any] struct {
	mu               sync.RWMutex
	nodes            map[string]NodeFunc[S]
	edges            map[string][]string
	conditionalEdges map[string]RouterFunc[S]
	entryPoint       string
	branchHook       BranchHook[S]
	forkJoinConfig   ForkJoinConfig
}

// NewGraph creates a new graph builder for state type S.
// The type parameter S defines the state that flows through the graph.
func NewGraph[S any]() *Graph[S] {
	return &Graph[S]{
		nodes:            make(map[string]NodeFunc[S]),
		edges:            make(map[string][]string),
		conditionalEdges: make(map[string]RouterFunc[S]),
	}
}

// AddNode adds a named node to the graph.
// Returns the graph for method chaining.
//
// Panics if:
//   - id is empty
//   - id is the reserved word "END" or "__end__" (case-insensitive)
//   - id contains whitespace (space, tab, newline)
//   - fn is nil
//   - id already exists in the graph
func (g *Graph[S]) AddNode(id string, fn NodeFunc[S]) *Graph[S] {
	// Validation (panics per ADR-007)
	if id == "" {
		panic("flowgraph: node ID cannot be empty")
	}

	// Check reserved words (case-insensitive)
	idLower := strings.ToLower(id)
	if idLower == "end" || idLower == "__end__" {
		panic("flowgraph: node ID cannot be reserved word 'END'")
	}

	if strings.ContainsAny(id, " \t\n\r") {
		panic("flowgraph: node ID cannot contain whitespace")
	}

	if fn == nil {
		panic("flowgraph: node function cannot be nil")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[id]; exists {
		panic(fmt.Sprintf("flowgraph: duplicate node ID: %s", id))
	}

	g.nodes[id] = fn
	return g
}

// AddEdge adds an unconditional edge from one node to another.
// The target can be a node ID or flowgraph.END.
// Returns the graph for method chaining.
//
// Edge validation happens at Compile() time, not here.
// This allows edges to be added in any order.
func (g *Graph[S]) AddEdge(from, to string) *Graph[S] {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.edges[from] = append(g.edges[from], to)
	return g
}

// AddConditionalEdge adds a conditional edge where a RouterFunc
// determines the next node at runtime based on state.
// Returns the graph for method chaining.
//
// The router function should return a valid node ID or flowgraph.END.
// Returning an empty string or unknown node ID will cause a runtime error.
//
// A node can have either simple edges or a conditional edge, not both.
// If both are present, the conditional edge takes precedence.
func (g *Graph[S]) AddConditionalEdge(from string, router RouterFunc[S]) *Graph[S] {
	if router == nil {
		panic("flowgraph: router function cannot be nil")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.conditionalEdges[from] = router
	return g
}

// SetEntry designates the entry point node.
// This must be called before Compile().
// Returns the graph for method chaining.
//
// Entry point validation happens at Compile() time.
func (g *Graph[S]) SetEntry(id string) *Graph[S] {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.entryPoint = id
	return g
}

// SetBranchHook sets the lifecycle hook for parallel branch execution.
// The hook is called during fork/join operations to allow custom setup,
// validation, and cleanup.
//
// This is optional - if not set, the executor uses sensible defaults:
//   - OnFork: state is cloned using ParallelState.Clone or JSON fallback
//   - OnJoin: branch states are merged using ParallelState.Merge
//   - OnBranchError: error is logged, no additional cleanup
//
// Example use case (git worktrees):
//
//	hook := &WorktreeBranchHook{Manager: worktreeManager}
//	graph.SetBranchHook(hook)
func (g *Graph[S]) SetBranchHook(hook BranchHook[S]) *Graph[S] {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.branchHook = hook
	return g
}

// SetForkJoinConfig sets the configuration for parallel execution.
// This controls concurrency limits, failure handling, and timeouts.
//
// This is optional - if not set, defaults are used:
//   - MaxConcurrency: 0 (unlimited)
//   - FailFast: false (wait for all branches)
//   - MergeTimeout: 0 (no timeout)
//
// Example:
//
//	graph.SetForkJoinConfig(flowgraph.ForkJoinConfig{
//	    MaxConcurrency: 4,      // Limit to 4 concurrent branches
//	    FailFast:       true,   // Cancel others on first failure
//	    MergeTimeout:   time.Minute * 5,
//	})
func (g *Graph[S]) SetForkJoinConfig(cfg ForkJoinConfig) *Graph[S] {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.forkJoinConfig = cfg
	return g
}
