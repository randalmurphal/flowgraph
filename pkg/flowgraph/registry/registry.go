package registry

import "sync"

// Registry is a thread-safe registry for values indexed by key.
// It uses sync.RWMutex for optimal read-heavy workloads.
type Registry[K comparable, V any] struct {
	mu      sync.RWMutex
	entries map[K]V
}

// New creates a new empty registry.
func New[K comparable, V any]() *Registry[K, V] {
	return &Registry[K, V]{
		entries: make(map[K]V),
	}
}

// Register adds or updates a value in the registry.
func (r *Registry[K, V]) Register(key K, value V) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[key] = value
}

// RegisterMany adds multiple entries to the registry.
func (r *Registry[K, V]) RegisterMany(entries map[K]V) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range entries {
		r.entries[k] = v
	}
}

// Get returns the value for a key and whether it exists.
func (r *Registry[K, V]) Get(key K) (V, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.entries[key]
	return v, ok
}

// MustGet returns the value for a key, panicking if not found.
func (r *Registry[K, V]) MustGet(key K) V {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.entries[key]
	if !ok {
		panic("registry: key not found")
	}
	return v
}

// Has returns true if the key exists in the registry.
func (r *Registry[K, V]) Has(key K) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.entries[key]
	return ok
}

// Delete removes a key from the registry.
func (r *Registry[K, V]) Delete(key K) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, key)
}

// Keys returns all keys in the registry.
// The order is not guaranteed.
func (r *Registry[K, V]) Keys() []K {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]K, 0, len(r.entries))
	for k := range r.entries {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of entries in the registry.
func (r *Registry[K, V]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// Range iterates over all entries in the registry.
// The function fn is called for each entry. If fn returns false,
// iteration stops.
//
// Range iterates over a snapshot of the registry, so it is safe
// to call Register or Delete during iteration without affecting
// the current iteration.
func (r *Registry[K, V]) Range(fn func(K, V) bool) {
	// Take a snapshot under read lock
	r.mu.RLock()
	snapshot := make(map[K]V, len(r.entries))
	for k, v := range r.entries {
		snapshot[k] = v
	}
	r.mu.RUnlock()

	// Iterate over snapshot without holding lock
	for k, v := range snapshot {
		if !fn(k, v) {
			return
		}
	}
}

// GetOrCreate returns the value for a key, creating it with the factory
// function if it doesn't exist. This operation is atomic - the factory
// is called at most once per key, even under concurrent access.
func (r *Registry[K, V]) GetOrCreate(key K, factory func() V) V {
	// Fast path: check if already exists
	r.mu.RLock()
	v, ok := r.entries[key]
	r.mu.RUnlock()
	if ok {
		return v
	}

	// Slow path: create with write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if v, ok := r.entries[key]; ok {
		return v
	}

	// Create and store
	v = factory()
	r.entries[key] = v
	return v
}
