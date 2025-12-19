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
