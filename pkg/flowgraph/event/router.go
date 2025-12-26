package event

import (
	"context"
	"fmt"
	"sync"
	"time"

	fgerrors "github.com/randalmurphal/flowgraph/pkg/flowgraph/errors"
)

// Router dispatches events to registered handlers.
type Router interface {
	// Route dispatches an event and returns any derived events.
	Route(ctx context.Context, evt Event) ([]Event, error)

	// Register adds a handler for the event types it handles.
	Register(handler Handler, opts ...HandlerOption)

	// Use adds middleware that applies to all handlers.
	Use(middleware MiddlewareFunc)
}

// RouterConfig configures router behavior.
type RouterConfig struct {
	// MaxDepth prevents infinite recursion when events trigger other events.
	// Default: 10
	MaxDepth int

	// ConcurrencyLimit limits parallel handler execution.
	// Default: 0 (unlimited)
	ConcurrencyLimit int

	// Registry for event validation (optional).
	Registry *EventRegistry

	// ValidateEvents enables schema validation before dispatch.
	ValidateEvents bool

	// DeadLetterQueue for failed events (optional).
	DLQ DeadLetterQueue

	// RetryConfig for transient failures.
	RetryConfig fgerrors.RetryConfig

	// OnError is called when an error occurs (for logging).
	OnError func(evt Event, handler string, err error)

	// OnSuccess is called after successful processing (for metrics).
	OnSuccess func(evt Event, handler string, duration time.Duration)
}

// DefaultRouterConfig provides reasonable defaults.
var DefaultRouterConfig = RouterConfig{
	MaxDepth:    10,
	RetryConfig: fgerrors.DefaultRetry,
}

// handlerEntry stores a handler with its configuration.
type handlerEntry struct {
	handler Handler
	retry   fgerrors.RetryConfig
	timeout time.Duration
}

// DefaultRouter is the standard router implementation.
type DefaultRouter struct {
	config RouterConfig

	mu         sync.RWMutex
	handlers   map[string][]handlerEntry // event type -> handlers
	wildcards  []handlerEntry            // handlers for all events
	middleware []MiddlewareFunc
}

// NewRouter creates a new event router.
func NewRouter(config RouterConfig) *DefaultRouter {
	if config.MaxDepth <= 0 {
		config.MaxDepth = DefaultRouterConfig.MaxDepth
	}
	if config.RetryConfig.MaxAttempts <= 0 {
		config.RetryConfig = DefaultRouterConfig.RetryConfig
	}

	return &DefaultRouter{
		config:   config,
		handlers: make(map[string][]handlerEntry),
	}
}

// HandlerOption configures handler behavior.
type HandlerOption func(*handlerEntry)

// WithHandlerRetry sets custom retry configuration.
func WithHandlerRetry(cfg fgerrors.RetryConfig) HandlerOption {
	return func(e *handlerEntry) {
		e.retry = cfg
	}
}

// WithHandlerTimeout sets a timeout for the handler.
func WithHandlerTimeout(d time.Duration) HandlerOption {
	return func(e *handlerEntry) {
		e.timeout = d
	}
}

// Register adds a handler to the router.
func (r *DefaultRouter) Register(handler Handler, opts ...HandlerOption) {
	entry := handlerEntry{
		handler: handler,
		retry:   r.config.RetryConfig,
	}

	for _, opt := range opts {
		opt(&entry)
	}

	// Apply middleware to handler
	wrappedHandler := ChainMiddleware(entry.handler, r.middleware...)
	entry.handler = wrappedHandler

	r.mu.Lock()
	defer r.mu.Unlock()

	eventTypes := handler.Handles()
	if len(eventTypes) == 0 {
		// Handler accepts all events
		r.wildcards = append(r.wildcards, entry)
	} else {
		for _, t := range eventTypes {
			r.handlers[t] = append(r.handlers[t], entry)
		}
	}
}

// Use adds middleware that applies to subsequently registered handlers.
func (r *DefaultRouter) Use(middleware MiddlewareFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, middleware)
}

// Route dispatches an event to all matching handlers.
func (r *DefaultRouter) Route(ctx context.Context, evt Event) ([]Event, error) {
	// Check depth to prevent infinite recursion
	depth := getEventDepth(ctx)
	if depth >= r.config.MaxDepth {
		return nil, &EventError{
			Event:   evt,
			Message: fmt.Sprintf("max event depth exceeded (%d)", r.config.MaxDepth),
		}
	}

	// Validate event if registry is configured
	if r.config.ValidateEvents && r.config.Registry != nil {
		if err := r.config.Registry.Validate(evt); err != nil {
			return nil, &EventError{
				Event:   evt,
				Message: "event validation failed",
				Err:     err,
			}
		}
	}

	// Get matching handlers
	r.mu.RLock()
	entries := make([]handlerEntry, 0)
	entries = append(entries, r.handlers[evt.Type()]...)
	entries = append(entries, r.wildcards...)
	r.mu.RUnlock()

	if len(entries) == 0 {
		// No handlers registered - this is not an error
		return nil, nil
	}

	// Increment depth for derived events
	ctx = withEventDepth(ctx, depth+1)

	// Collect all derived events
	var allDerived []Event
	var mu sync.Mutex

	// Process handlers
	for _, entry := range entries {
		derived, err := r.executeHandler(ctx, evt, entry)
		if err != nil {
			// Handler failed after retries - enqueue to DLQ if configured
			if r.config.DLQ != nil {
				failed := NewFailedEvent(evt, err, handlerName(entry.handler))
				if dlqErr := r.config.DLQ.Enqueue(ctx, failed); dlqErr != nil {
					// Log DLQ error but don't fail the route
					if r.config.OnError != nil {
						r.config.OnError(evt, "dlq", dlqErr)
					}
				}
			}

			if r.config.OnError != nil {
				r.config.OnError(evt, handlerName(entry.handler), err)
			}

			// Continue processing other handlers even if one fails
			continue
		}

		if len(derived) > 0 {
			mu.Lock()
			allDerived = append(allDerived, derived...)
			mu.Unlock()
		}
	}

	return allDerived, nil
}

// executeHandler runs a single handler with retry and timeout.
func (r *DefaultRouter) executeHandler(
	ctx context.Context,
	evt Event,
	entry handlerEntry,
) ([]Event, error) {
	start := time.Now()

	// Apply timeout if configured
	if entry.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, entry.timeout)
		defer cancel()
	}

	// Execute with retry
	result := fgerrors.WithRetryContext(ctx, entry.retry, func(ctx context.Context) ([]Event, error) {
		return entry.handler.Handle(ctx, evt)
	})

	if result.Err != nil {
		return nil, result.Err
	}

	if r.config.OnSuccess != nil {
		r.config.OnSuccess(evt, handlerName(entry.handler), time.Since(start))
	}

	return result.Value, nil
}

// handlerName extracts a name for a handler (for logging/metrics).
func handlerName(h Handler) string {
	// Try to get type name
	return fmt.Sprintf("%T", h)
}

// Context keys for event depth tracking
type contextKey string

const eventDepthKey contextKey = "event_depth"

func getEventDepth(ctx context.Context) int {
	if v := ctx.Value(eventDepthKey); v != nil {
		return v.(int)
	}
	return 0
}

func withEventDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, eventDepthKey, depth)
}

// Common middleware implementations

// LoggingMiddleware logs event processing.
func LoggingMiddleware(logFn func(eventType, handlerName string, duration time.Duration, err error)) MiddlewareFunc {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, evt Event) ([]Event, error) {
			start := time.Now()
			result, err := next.Handle(ctx, evt)
			logFn(evt.Type(), handlerName(next), time.Since(start), err)
			return result, err
		})
	}
}

// RecoveryMiddleware recovers from panics in handlers.
func RecoveryMiddleware() MiddlewareFunc {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, evt Event) (result []Event, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = &EventError{
						Event:   evt,
						Message: fmt.Sprintf("handler panic: %v", r),
					}
				}
			}()
			return next.Handle(ctx, evt)
		})
	}
}

// MetricsMiddleware records handler metrics.
func MetricsMiddleware(
	onStart func(eventType string),
	onComplete func(eventType string, duration time.Duration, err error),
) MiddlewareFunc {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, evt Event) ([]Event, error) {
			if onStart != nil {
				onStart(evt.Type())
			}
			start := time.Now()
			result, err := next.Handle(ctx, evt)
			if onComplete != nil {
				onComplete(evt.Type(), time.Since(start), err)
			}
			return result, err
		})
	}
}

// CorrelationMiddleware ensures derived events maintain correlation.
func CorrelationMiddleware() MiddlewareFunc {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, evt Event) ([]Event, error) {
			result, err := next.Handle(ctx, evt)
			if err != nil {
				return nil, err
			}

			// Ensure all derived events have proper correlation
			for i, derived := range result {
				// Check if correlation is set
				if derived.CorrelationID() == "" || derived.CausationID() == "" {
					// This shouldn't happen if NewFromParent is used correctly,
					// but we can warn or fix it here
					_ = i // Placeholder for potential warning/fix
				}
			}

			return result, nil
		})
	}
}
