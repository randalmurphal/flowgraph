// Package event provides event-driven architecture primitives for flowgraph.
//
// This package implements industry best practices for event-driven systems:
//   - Event interface with correlation and causation tracking
//   - EventRegistry for schema management and validation
//   - Router for event dispatch with middleware
//   - Bus for pub/sub fan-out distribution
//   - Aggregator for fan-in event correlation
//   - DLQ/PLQ for error handling
//
// Design Influences:
//   - Confluent Schema Registry (schema versioning and compatibility)
//   - AWS EventBridge (dead letter queues, error handling)
//   - Temporal (signals, queries, workflow interaction)
//   - Apache Kafka (fan-out, fan-in, correlation IDs)
package event

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event is the core interface for all events in the system.
// Events are immutable once created - any modification creates a new event.
type Event interface {
	// Identity
	ID() string     // Unique event identifier
	Type() string   // Event type (e.g., "task.created", "flow.completed")
	Source() string // Event source (e.g., "task", "flow", "github")

	// Correlation for distributed tracing
	CorrelationID() string // Groups related events across services
	CausationID() string   // ID of event that directly caused this one

	// Metadata
	Timestamp() time.Time // When the event occurred
	Version() int         // Schema version for evolution
	TenantID() string     // Multi-tenant isolation

	// Payload
	Data() any         // Strongly-typed payload
	DataBytes() []byte // Serialized payload for transport
}

// Metadata contains common event metadata fields.
type Metadata struct {
	EventID       string    `json:"id"`
	EventType     string    `json:"type"`
	EventSource   string    `json:"source"`
	CorrelationID string    `json:"correlation_id"`
	CausationID   string    `json:"causation_id,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	SchemaVersion int       `json:"schema_version"`
	TenantID      string    `json:"tenant_id"`
}

// BaseEvent provides a generic event implementation.
// T is the payload type for type-safe access.
type BaseEvent[T any] struct {
	Meta    Metadata `json:"metadata"`
	Payload T        `json:"payload"`

	// Cached serialization (computed lazily)
	cachedBytes []byte
}

// ID returns the unique event identifier.
func (e *BaseEvent[T]) ID() string {
	return e.Meta.EventID
}

// Type returns the event type.
func (e *BaseEvent[T]) Type() string {
	return e.Meta.EventType
}

// Source returns the event source.
func (e *BaseEvent[T]) Source() string {
	return e.Meta.EventSource
}

// CorrelationID returns the correlation ID for distributed tracing.
func (e *BaseEvent[T]) CorrelationID() string {
	return e.Meta.CorrelationID
}

// CausationID returns the ID of the event that caused this one.
func (e *BaseEvent[T]) CausationID() string {
	return e.Meta.CausationID
}

// Timestamp returns when the event occurred.
func (e *BaseEvent[T]) Timestamp() time.Time {
	return e.Meta.Timestamp
}

// Version returns the schema version.
func (e *BaseEvent[T]) Version() int {
	return e.Meta.SchemaVersion
}

// TenantID returns the tenant ID for multi-tenant isolation.
func (e *BaseEvent[T]) TenantID() string {
	return e.Meta.TenantID
}

// Data returns the event payload.
func (e *BaseEvent[T]) Data() any {
	return e.Payload
}

// TypedData returns the strongly-typed payload.
func (e *BaseEvent[T]) TypedData() T {
	return e.Payload
}

// DataBytes returns the serialized payload.
// The result is cached for efficiency.
func (e *BaseEvent[T]) DataBytes() []byte {
	if e.cachedBytes == nil {
		// Best effort - errors are ignored for interface compliance
		e.cachedBytes, _ = json.Marshal(e.Payload)
	}
	return e.cachedBytes
}

// MarshalJSON implements json.Marshaler.
func (e *BaseEvent[T]) MarshalJSON() ([]byte, error) {
	type alias BaseEvent[T]
	return json.Marshal((*alias)(e))
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *BaseEvent[T]) UnmarshalJSON(data []byte) error {
	type alias BaseEvent[T]
	if err := json.Unmarshal(data, (*alias)(e)); err != nil {
		return err
	}
	e.cachedBytes = nil // Clear cache on unmarshal
	return nil
}

// EventOption configures event creation.
type EventOption func(*eventConfig)

type eventConfig struct {
	id            string
	correlationID string
	causationID   string
	timestamp     time.Time
	version       int
}

// WithEventID sets a specific event ID (default: auto-generated UUID).
func WithEventID(id string) EventOption {
	return func(cfg *eventConfig) {
		cfg.id = id
	}
}

// WithCorrelationID sets the correlation ID for tracing.
func WithCorrelationID(id string) EventOption {
	return func(cfg *eventConfig) {
		cfg.correlationID = id
	}
}

// WithCausationID sets the ID of the causing event.
func WithCausationID(id string) EventOption {
	return func(cfg *eventConfig) {
		cfg.causationID = id
	}
}

// WithTimestamp sets a specific timestamp (default: time.Now()).
func WithTimestamp(t time.Time) EventOption {
	return func(cfg *eventConfig) {
		cfg.timestamp = t
	}
}

// WithSchemaVersion sets the schema version.
func WithSchemaVersion(v int) EventOption {
	return func(cfg *eventConfig) {
		cfg.version = v
	}
}

// New creates a new event with the given type, source, and payload.
func New[T any](
	eventType string,
	source string,
	tenantID string,
	payload T,
	opts ...EventOption,
) *BaseEvent[T] {
	cfg := &eventConfig{
		id:        uuid.New().String(),
		timestamp: time.Now(),
		version:   1,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// If no correlation ID, use event ID as the root
	if cfg.correlationID == "" {
		cfg.correlationID = cfg.id
	}

	return &BaseEvent[T]{
		Meta: Metadata{
			EventID:       cfg.id,
			EventType:     eventType,
			EventSource:   source,
			CorrelationID: cfg.correlationID,
			CausationID:   cfg.causationID,
			Timestamp:     cfg.timestamp,
			SchemaVersion: cfg.version,
			TenantID:      tenantID,
		},
		Payload: payload,
	}
}

// NewFromParent creates a new event caused by a parent event.
// It automatically inherits the correlation ID and sets causation ID.
func NewFromParent[T any](
	parent Event,
	eventType string,
	source string,
	payload T,
	opts ...EventOption,
) *BaseEvent[T] {
	// Prepend parent correlation options (can be overridden by opts)
	parentOpts := []EventOption{
		WithCorrelationID(parent.CorrelationID()),
		WithCausationID(parent.ID()),
	}
	allOpts := append(parentOpts, opts...)

	return New(eventType, source, parent.TenantID(), payload, allOpts...)
}

// NewAny creates a new event with an untyped (any) payload.
// This is a convenience function when you don't need type-safe payload access.
func NewAny(
	eventType string,
	source string,
	tenantID string,
	payload any,
	opts ...EventOption,
) *BaseEvent[any] {
	return New(eventType, source, tenantID, payload, opts...)
}

// NewAnyFromParent creates a new event with untyped payload from a parent event.
func NewAnyFromParent(
	parent Event,
	eventType string,
	source string,
	payload any,
	opts ...EventOption,
) *BaseEvent[any] {
	return NewFromParent(parent, eventType, source, payload, opts...)
}

// Handler processes events and optionally returns derived events.
type Handler interface {
	// Handle processes an event and returns any derived events.
	// Returning multiple events enables fan-out patterns.
	Handle(ctx context.Context, evt Event) ([]Event, error)

	// Handles returns the event types this handler processes.
	// An empty slice means the handler accepts all event types.
	Handles() []string
}

// HandlerFunc adapts a function to the Handler interface.
type HandlerFunc func(ctx context.Context, evt Event) ([]Event, error)

// Handle implements Handler.
func (f HandlerFunc) Handle(ctx context.Context, evt Event) ([]Event, error) {
	return f(ctx, evt)
}

// Handles returns nil (accepts all event types).
func (f HandlerFunc) Handles() []string {
	return nil
}

// TypedHandler wraps a function handling a specific payload type.
func TypedHandler[T any](
	eventTypes []string,
	fn func(ctx context.Context, payload T, meta Metadata) ([]Event, error),
) Handler {
	return &typedHandler[T]{
		eventTypes: eventTypes,
		fn:         fn,
	}
}

type typedHandler[T any] struct {
	eventTypes []string
	fn         func(ctx context.Context, payload T, meta Metadata) ([]Event, error)
}

func (h *typedHandler[T]) Handle(ctx context.Context, evt Event) ([]Event, error) {
	// Try to extract typed data
	var payload T

	switch d := evt.Data().(type) {
	case T:
		payload = d
	case map[string]any:
		// JSON unmarshal path
		bytes, err := json.Marshal(d)
		if err != nil {
			return nil, &EventError{
				Event:   evt,
				Message: "failed to marshal event data",
				Err:     err,
			}
		}
		if err := json.Unmarshal(bytes, &payload); err != nil {
			return nil, &EventError{
				Event:   evt,
				Message: "failed to unmarshal event data to expected type",
				Err:     err,
			}
		}
	default:
		return nil, &EventError{
			Event:   evt,
			Message: "unexpected payload type",
		}
	}

	// Extract metadata
	meta := Metadata{
		EventID:       evt.ID(),
		EventType:     evt.Type(),
		EventSource:   evt.Source(),
		CorrelationID: evt.CorrelationID(),
		CausationID:   evt.CausationID(),
		Timestamp:     evt.Timestamp(),
		SchemaVersion: evt.Version(),
		TenantID:      evt.TenantID(),
	}

	return h.fn(ctx, payload, meta)
}

func (h *typedHandler[T]) Handles() []string {
	return h.eventTypes
}

// MiddlewareFunc wraps handlers to add cross-cutting concerns.
type MiddlewareFunc func(next Handler) Handler

// ChainMiddleware applies middleware in order, with first middleware outermost.
func ChainMiddleware(handler Handler, middleware ...MiddlewareFunc) Handler {
	// Apply in reverse order so first middleware is outermost
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}
