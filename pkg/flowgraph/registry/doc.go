// Package registry provides a generic thread-safe registry for values indexed by key.
//
// Registry is designed for read-heavy workloads using sync.RWMutex. It supports
// any comparable key type and any value type through Go generics.
//
// # Basic Usage
//
// Create a registry and register values:
//
//	r := registry.New[string, int]()
//	r.Register("one", 1)
//	r.Register("two", 2)
//
//	value, ok := r.Get("one")
//	if ok {
//	    fmt.Println(value) // Output: 1
//	}
//
// # Factory Pattern
//
// Registries work well for factory patterns where you register constructors:
//
//	type NodeFactory func(config Config) (Node, error)
//
//	factories := registry.New[string, NodeFactory]()
//	factories.Register("start", NewStartNode)
//	factories.Register("end", NewEndNode)
//	factories.Register("action", NewActionNode)
//
//	// Later, create a node by type
//	factory, ok := factories.Get("action")
//	if ok {
//	    node, err := factory(cfg)
//	    // use node...
//	}
//
// # Lazy Initialization
//
// Use GetOrCreate for thread-safe lazy initialization:
//
//	// Connection pool per database
//	pools := registry.New[string, *Pool]()
//
//	// First call creates the pool, subsequent calls return the same one
//	pool := pools.GetOrCreate("users_db", func() *Pool {
//	    return NewPool("users_db")
//	})
//
// GetOrCreate is atomic - the factory function is called at most once per key,
// even under concurrent access.
//
// # Thread Safety
//
// All Registry methods are safe for concurrent use. The Range method iterates
// over a snapshot of the registry, allowing mutations during iteration without
// affecting the iteration itself:
//
//	r.Range(func(key string, value int) bool {
//	    // Safe to call r.Register() or r.Delete() here
//	    if value < 0 {
//	        r.Delete(key) // Won't affect current iteration
//	    }
//	    return true // continue iteration
//	})
package registry
