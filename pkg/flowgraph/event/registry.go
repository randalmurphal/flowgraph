package event

import (
	"fmt"
	"sync"
)

// EventSchema defines the schema for an event type.
type EventSchema struct {
	// Type is the event type (e.g., "task.created").
	Type string

	// Source is the event source (e.g., "task", "flow").
	Source string

	// Version is the schema version number.
	Version int

	// Description explains the event's purpose.
	Description string

	// PayloadType is the expected Go type for the payload.
	// Used for runtime type checking.
	PayloadType any

	// Tags enable semantic search and categorization.
	Tags []string

	// Validator is an optional custom validation function.
	Validator func(Event) error

	// Compatible lists backward-compatible versions.
	// A consumer at version N can read events at versions in Compatible.
	Compatible []int

	// Deprecated marks the schema as deprecated.
	Deprecated bool

	// DeprecationMessage explains the deprecation.
	DeprecationMessage string
}

// IsCompatibleWith returns true if this schema can read events at the given version.
func (s *EventSchema) IsCompatibleWith(version int) bool {
	if version == s.Version {
		return true
	}
	for _, v := range s.Compatible {
		if v == version {
			return true
		}
	}
	return false
}

// Validate checks if an event conforms to this schema.
func (s *EventSchema) Validate(evt Event) error {
	if evt.Type() != s.Type {
		return fmt.Errorf("event type mismatch: expected %s, got %s", s.Type, evt.Type())
	}

	if !s.IsCompatibleWith(evt.Version()) {
		return fmt.Errorf("incompatible version: schema %d, event %d", s.Version, evt.Version())
	}

	if s.Validator != nil {
		if err := s.Validator(evt); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	return nil
}

// EventRegistry manages event type definitions with version support.
type EventRegistry struct {
	mu sync.RWMutex

	// schemas maps event type -> latest schema
	schemas map[string]*EventSchema

	// versions maps event type -> version -> schema
	versions map[string]map[int]*EventSchema
}

// NewEventRegistry creates a new event registry.
func NewEventRegistry() *EventRegistry {
	return &EventRegistry{
		schemas:  make(map[string]*EventSchema),
		versions: make(map[string]map[int]*EventSchema),
	}
}

// Register adds an event schema to the registry.
// If a schema with the same type and version exists, it's replaced.
func (r *EventRegistry) Register(schema *EventSchema) error {
	if schema.Type == "" {
		return fmt.Errorf("event type is required")
	}
	if schema.Version <= 0 {
		return fmt.Errorf("version must be positive")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize version map if needed
	if r.versions[schema.Type] == nil {
		r.versions[schema.Type] = make(map[int]*EventSchema)
	}

	// Store versioned schema
	r.versions[schema.Type][schema.Version] = schema

	// Update latest if this is a higher version
	if current, ok := r.schemas[schema.Type]; !ok || schema.Version > current.Version {
		r.schemas[schema.Type] = schema
	}

	return nil
}

// Get returns the latest schema for an event type.
func (r *EventRegistry) Get(eventType string) (*EventSchema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schema, ok := r.schemas[eventType]
	return schema, ok
}

// GetVersion returns a specific version of a schema.
func (r *EventRegistry) GetVersion(eventType string, version int) (*EventSchema, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.versions[eventType]
	if !ok {
		return nil, false
	}

	schema, ok := versions[version]
	return schema, ok
}

// Validate checks if an event conforms to its registered schema.
func (r *EventRegistry) Validate(evt Event) error {
	r.mu.RLock()
	schema, ok := r.schemas[evt.Type()]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown event type: %s", evt.Type())
	}

	return schema.Validate(evt)
}

// ValidateStrict checks using the exact schema version.
func (r *EventRegistry) ValidateStrict(evt Event) error {
	r.mu.RLock()
	schema, ok := r.GetVersion(evt.Type(), evt.Version())
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown event type %s at version %d", evt.Type(), evt.Version())
	}

	return schema.Validate(evt)
}

// Has returns true if a schema exists for the event type.
func (r *EventRegistry) Has(eventType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.schemas[eventType]
	return ok
}

// Types returns all registered event types.
func (r *EventRegistry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.schemas))
	for t := range r.schemas {
		types = append(types, t)
	}
	return types
}

// ListBySource returns all schemas for a given source.
func (r *EventRegistry) ListBySource(source string) []*EventSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var schemas []*EventSchema
	for _, schema := range r.schemas {
		if schema.Source == source {
			schemas = append(schemas, schema)
		}
	}
	return schemas
}

// ListByTag returns all schemas with a given tag.
func (r *EventRegistry) ListByTag(tag string) []*EventSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var schemas []*EventSchema
	for _, schema := range r.schemas {
		for _, t := range schema.Tags {
			if t == tag {
				schemas = append(schemas, schema)
				break
			}
		}
	}
	return schemas
}

// Versions returns all registered versions for an event type.
func (r *EventRegistry) Versions(eventType string) []int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.versions[eventType]
	if !ok {
		return nil
	}

	result := make([]int, 0, len(versions))
	for v := range versions {
		result = append(result, v)
	}
	return result
}

// LatestVersion returns the highest version number for an event type.
func (r *EventRegistry) LatestVersion(eventType string) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schema, ok := r.schemas[eventType]
	if !ok {
		return 0, false
	}
	return schema.Version, true
}

// Range iterates over all schemas.
func (r *EventRegistry) Range(fn func(*EventSchema) bool) {
	r.mu.RLock()
	// Take snapshot
	schemas := make([]*EventSchema, 0, len(r.schemas))
	for _, s := range r.schemas {
		schemas = append(schemas, s)
	}
	r.mu.RUnlock()

	for _, s := range schemas {
		if !fn(s) {
			return
		}
	}
}

// DefaultRegistry is the global event registry.
var DefaultRegistry = NewEventRegistry()

// Register adds a schema to the default registry.
func Register(schema *EventSchema) error {
	return DefaultRegistry.Register(schema)
}

// MustRegister adds a schema to the default registry, panicking on error.
func MustRegister(schema *EventSchema) {
	if err := DefaultRegistry.Register(schema); err != nil {
		panic(fmt.Sprintf("failed to register event schema: %v", err))
	}
}
