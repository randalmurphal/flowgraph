package flowgraph

// CompiledGraph is an immutable, executable graph.
// It is created by calling Compile() on a Graph builder.
//
// CompiledGraph is thread-safe and can be used concurrently for multiple
// Run() calls. The graph structure cannot be modified after compilation.
//
// Use the introspection methods (NodeIDs, Successors, etc.) to examine
// the graph structure for debugging or visualization.
type CompiledGraph[S any] struct {
	nodes            map[string]NodeFunc[S]
	edges            map[string][]string
	conditionalEdges map[string]RouterFunc[S]
	entryPoint       string

	// Pre-computed for efficient lookup
	successors    map[string][]string
	predecessors  map[string][]string
	isConditional map[string]bool

	// Parallel execution support
	branchHook     BranchHook[S]
	forkJoinConfig ForkJoinConfig
	forkNodes      map[string]*ForkNode // nodeID -> fork info (nodes with multiple outgoing edges)
	joinNodes      map[string]*JoinNode // nodeID -> join info (nodes with multiple incoming from same fork)
}

// EntryPoint returns the entry node ID.
func (cg *CompiledGraph[S]) EntryPoint() string {
	return cg.entryPoint
}

// NodeIDs returns all node identifiers in the graph.
// The order is not guaranteed.
func (cg *CompiledGraph[S]) NodeIDs() []string {
	ids := make([]string, 0, len(cg.nodes))
	for id := range cg.nodes {
		ids = append(ids, id)
	}
	return ids
}

// HasNode checks if a node exists in the graph.
func (cg *CompiledGraph[S]) HasNode(id string) bool {
	_, exists := cg.nodes[id]
	return exists
}

// Successors returns the node IDs that can be reached from the given node
// via simple (non-conditional) edges.
// Returns nil for END or unknown nodes.
// Does not include targets of conditional edges (those are runtime-determined).
func (cg *CompiledGraph[S]) Successors(id string) []string {
	if id == END {
		return nil
	}
	return cg.successors[id]
}

// Predecessors returns the node IDs that have edges to the given node.
// Returns nil for the entry node or unknown nodes.
func (cg *CompiledGraph[S]) Predecessors(id string) []string {
	return cg.predecessors[id]
}

// IsConditional returns true if the node has a conditional edge.
func (cg *CompiledGraph[S]) IsConditional(id string) bool {
	return cg.isConditional[id]
}

// getNode returns the node function for the given ID.
// Used internally by the executor.
func (cg *CompiledGraph[S]) getNode(id string) (NodeFunc[S], bool) {
	fn, exists := cg.nodes[id]
	return fn, exists
}

// getRouter returns the router function for the given node.
// Used internally by the executor.
func (cg *CompiledGraph[S]) getRouter(id string) (RouterFunc[S], bool) {
	router, exists := cg.conditionalEdges[id]
	return router, exists
}

// getEdges returns the simple edge targets for the given node.
// Used internally by the executor.
func (cg *CompiledGraph[S]) getEdges(id string) []string {
	return cg.edges[id]
}

// IsForkNode returns true if the node is a detected fork point
// (has multiple outgoing edges that require parallel execution).
func (cg *CompiledGraph[S]) IsForkNode(id string) bool {
	_, exists := cg.forkNodes[id]
	return exists
}

// GetForkNode returns the fork information for a node, or nil if not a fork.
func (cg *CompiledGraph[S]) GetForkNode(id string) *ForkNode {
	return cg.forkNodes[id]
}

// IsJoinNode returns true if the node is a detected join point
// (where parallel branches converge).
func (cg *CompiledGraph[S]) IsJoinNode(id string) bool {
	_, exists := cg.joinNodes[id]
	return exists
}

// GetJoinNode returns the join information for a node, or nil if not a join.
func (cg *CompiledGraph[S]) GetJoinNode(id string) *JoinNode {
	return cg.joinNodes[id]
}

// ForkNodes returns all fork nodes in the graph.
// Returns an empty slice if there are no fork nodes.
func (cg *CompiledGraph[S]) ForkNodes() []*ForkNode {
	result := make([]*ForkNode, 0, len(cg.forkNodes))
	for _, fn := range cg.forkNodes {
		result = append(result, fn)
	}
	return result
}

// HasParallelExecution returns true if the graph contains any fork/join structures.
func (cg *CompiledGraph[S]) HasParallelExecution() bool {
	return len(cg.forkNodes) > 0
}

// getBranchHook returns the branch hook, or nil if not set.
func (cg *CompiledGraph[S]) getBranchHook() BranchHook[S] {
	return cg.branchHook
}

// getForkJoinConfig returns the fork/join configuration.
func (cg *CompiledGraph[S]) getForkJoinConfig() ForkJoinConfig {
	return cg.forkJoinConfig
}
