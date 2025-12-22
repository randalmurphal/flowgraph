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

	// Detect fork/join nodes
	forkNodes, joinNodes := detectForkJoinNodes(edges, predecessors, isConditional)

	return &CompiledGraph[S]{
		nodes:            nodes,
		edges:            edges,
		conditionalEdges: conditionalEdges,
		entryPoint:       g.entryPoint,
		successors:       successors,
		predecessors:     predecessors,
		isConditional:    isConditional,
		branchHook:       g.branchHook,
		forkJoinConfig:   g.forkJoinConfig,
		forkNodes:        forkNodes,
		joinNodes:        joinNodes,
	}
}

// detectForkJoinNodes identifies fork and join nodes in the graph.
// A fork node has multiple outgoing edges (and is not conditional).
// A join node is found using a simple heuristic: the first node where all
// branches from a fork converge (post-dominator).
//
// This is a simplified algorithm that works for basic fork/join patterns.
// More complex DAGs may require additional validation.
func detectForkJoinNodes(edges map[string][]string, predecessors map[string][]string, isConditional map[string]bool) (map[string]*ForkNode, map[string]*JoinNode) {
	forkNodes := make(map[string]*ForkNode)
	joinNodes := make(map[string]*JoinNode)

	// Find fork nodes: nodes with multiple outgoing non-conditional edges
	for from, targets := range edges {
		if len(targets) > 1 && !isConditional[from] {
			// This is a fork node
			fork := &ForkNode{
				NodeID:   from,
				Branches: make([]string, len(targets)),
			}
			copy(fork.Branches, targets)

			// Find the join node using post-dominator analysis
			joinNodeID := findJoinNode(from, targets, edges, predecessors)
			fork.JoinNodeID = joinNodeID

			forkNodes[from] = fork

			// Create the join node entry
			if joinNodeID != "" && joinNodeID != END {
				joinNodes[joinNodeID] = &JoinNode{
					NodeID:           joinNodeID,
					ForkNodeID:       from,
					ExpectedBranches: fork.Branches,
				}
			}
		}
	}

	return forkNodes, joinNodes
}

// findJoinNode finds the join point for a fork using simplified post-dominator analysis.
// It finds the first node that all branches must pass through to reach END.
func findJoinNode(forkNode string, branches []string, edges map[string][]string, predecessors map[string][]string) string {
	if len(branches) == 0 {
		return ""
	}

	// For each branch, compute all nodes reachable from it
	branchReachable := make([]map[string]bool, len(branches))
	for i, branch := range branches {
		branchReachable[i] = computeReachable(branch, edges)
	}

	// Find nodes reachable from ALL branches (intersection)
	// Start with the first branch's reachable set
	common := make(map[string]bool)
	for node := range branchReachable[0] {
		common[node] = true
	}

	// Intersect with other branches
	for i := 1; i < len(branches); i++ {
		for node := range common {
			if !branchReachable[i][node] {
				delete(common, node)
			}
		}
	}

	// Find the first common node (closest to branches)
	// by finding the one with the shortest path from any branch
	if len(common) == 0 {
		return ""
	}

	// Use BFS from any branch to find the closest common node
	return findClosestNode(branches[0], common, edges)
}

// computeReachable returns all nodes reachable from the given start node.
func computeReachable(start string, edges map[string][]string) map[string]bool {
	reachable := make(map[string]bool)
	queue := []string{start}
	reachable[start] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range edges[current] {
			if next != END && !reachable[next] {
				reachable[next] = true
				queue = append(queue, next)
			}
		}
	}

	return reachable
}

// findClosestNode finds the closest node in targets reachable from start using BFS.
func findClosestNode(start string, targets map[string]bool, edges map[string][]string) string {
	if targets[start] {
		return start
	}

	visited := make(map[string]bool)
	queue := []string{start}
	visited[start] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range edges[current] {
			if next == END {
				continue
			}
			if targets[next] {
				return next
			}
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	return ""
}
