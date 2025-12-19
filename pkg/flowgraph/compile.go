package flowgraph

import (
	"errors"
	"fmt"
	"log/slog"
)

// Compile validates the graph and creates an executable CompiledGraph.
// Returns an error if validation fails. Multiple errors are joined together.
//
// Validation checks (in order):
//  1. Entry point must be set
//  2. Entry point must reference an existing node
//  3. All edge sources must reference existing nodes
//  4. All edge targets must reference existing nodes or END
//  5. All nodes must have a path to END
//
// Unreachable nodes (not reachable from entry) are logged as warnings
// but do not cause compilation to fail.
func (g *Graph[S]) Compile() (*CompiledGraph[S], error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var errs []error

	// 1. Validate entry point is set
	if g.entryPoint == "" {
		errs = append(errs, ErrNoEntryPoint)
	} else if _, exists := g.nodes[g.entryPoint]; !exists {
		// 2. Validate entry point references existing node
		errs = append(errs, fmt.Errorf("%w: %s", ErrEntryNotFound, g.entryPoint))
	}

	// 3 & 4. Validate edge references
	for from, targets := range g.edges {
		// Check source exists (unless it's a node that only has conditional edges)
		if from != END {
			if _, exists := g.nodes[from]; !exists {
				if _, hasConditional := g.conditionalEdges[from]; !hasConditional {
					errs = append(errs, fmt.Errorf("%w: edge source '%s' does not exist", ErrNodeNotFound, from))
				}
			}
		}

		// Check all targets exist
		for _, to := range targets {
			if to != END {
				if _, exists := g.nodes[to]; !exists {
					errs = append(errs, fmt.Errorf("%w: edge target '%s' does not exist", ErrNodeNotFound, to))
				}
			}
		}
	}

	// Also check conditional edge sources
	for from := range g.conditionalEdges {
		if _, exists := g.nodes[from]; !exists {
			errs = append(errs, fmt.Errorf("%w: conditional edge source '%s' does not exist", ErrNodeNotFound, from))
		}
	}

	// 5. Validate path to END exists from entry
	if g.entryPoint != "" {
		if _, exists := g.nodes[g.entryPoint]; exists {
			if !g.hasPathToEnd() {
				errs = append(errs, ErrNoPathToEnd)
			}
		}
	}

	// Check for unreachable nodes (warning only)
	g.warnUnreachableNodes()

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return g.buildCompiledGraph(), nil
}

// hasPathToEnd checks if there's a path from entry to END.
// This uses a simple reachability analysis.
// Nodes with conditional edges are assumed to potentially reach any of their
// possible targets, including END.
func (g *Graph[S]) hasPathToEnd() bool {
	// Find all nodes that can reach END using reverse traversal
	canReachEnd := make(map[string]bool)
	canReachEnd[END] = true

	// Keep propagating until no changes
	changed := true
	for changed {
		changed = false

		// Check simple edges
		for from, targets := range g.edges {
			if canReachEnd[from] {
				continue
			}
			for _, to := range targets {
				if canReachEnd[to] {
					canReachEnd[from] = true
					changed = true
					break
				}
			}
		}

		// Check conditional edges - assume they can reach END if they have a router
		// (since the router might return END)
		for from := range g.conditionalEdges {
			if !canReachEnd[from] {
				// A conditional edge can potentially reach END
				canReachEnd[from] = true
				changed = true
			}
		}
	}

	return canReachEnd[g.entryPoint]
}

// warnUnreachableNodes logs warnings for nodes not reachable from entry.
func (g *Graph[S]) warnUnreachableNodes() {
	if g.entryPoint == "" {
		return
	}

	reachable := g.findReachableNodes()

	for nodeID := range g.nodes {
		if !reachable[nodeID] {
			slog.Warn("node is unreachable from entry", "node_id", nodeID)
		}
	}
}

// findReachableNodes returns the set of nodes reachable from the entry point.
func (g *Graph[S]) findReachableNodes() map[string]bool {
	reachable := make(map[string]bool)

	if g.entryPoint == "" {
		return reachable
	}

	// BFS from entry
	queue := []string{g.entryPoint}
	reachable[g.entryPoint] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Follow simple edges
		for _, target := range g.edges[current] {
			if target != END && !reachable[target] {
				reachable[target] = true
				queue = append(queue, target)
			}
		}

		// For conditional edges, we can't know the actual targets at compile time
		// since they depend on runtime state. The router function could potentially
		// return any node ID, so we must assume ALL nodes are reachable.
		if _, hasConditional := g.conditionalEdges[current]; hasConditional {
			for nodeID := range g.nodes {
				if !reachable[nodeID] {
					reachable[nodeID] = true
					queue = append(queue, nodeID)
				}
			}
		}
	}

	return reachable
}

// buildCompiledGraph creates the immutable CompiledGraph from the builder state.
func (g *Graph[S]) buildCompiledGraph() *CompiledGraph[S] {
	// Deep copy nodes
	nodes := make(map[string]NodeFunc[S], len(g.nodes))
	for id, fn := range g.nodes {
		nodes[id] = fn
	}

	// Deep copy edges
	edges := make(map[string][]string, len(g.edges))
	for from, targets := range g.edges {
		edges[from] = make([]string, len(targets))
		copy(edges[from], targets)
	}

	// Deep copy conditional edges
	conditionalEdges := make(map[string]RouterFunc[S], len(g.conditionalEdges))
	for from, router := range g.conditionalEdges {
		conditionalEdges[from] = router
	}

	// Pre-compute successors
	successors := make(map[string][]string)
	for from, targets := range edges {
		successors[from] = targets
	}

	// Pre-compute predecessors
	predecessors := make(map[string][]string)
	for from, targets := range edges {
		for _, to := range targets {
			if to != END {
				predecessors[to] = append(predecessors[to], from)
			}
		}
	}

	// Identify conditional nodes
	isConditional := make(map[string]bool)
	for from := range conditionalEdges {
		isConditional[from] = true
	}

	return &CompiledGraph[S]{
		nodes:            nodes,
		edges:            edges,
		conditionalEdges: conditionalEdges,
		entryPoint:       g.entryPoint,
		successors:       successors,
		predecessors:     predecessors,
		isConditional:    isConditional,
	}
}
