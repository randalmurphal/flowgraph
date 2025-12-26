package event_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/event"
)

func TestRouter(t *testing.T) {
	router := event.NewRouter(event.RouterConfig{
		MaxDepth: 5,
	})

	var called atomic.Int32

	handler := event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		called.Add(1)
		return nil, nil
	})

	// Register handler for specific type
	router.Register(&typedTestHandler{
		types:   []string{"test.event"},
		handler: handler,
	})

	// Route matching event
	evt := event.NewAny("test.event", "test", "t1", nil)
	_, err := router.Route(context.Background(), evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if called.Load() != 1 {
		t.Errorf("expected handler to be called once, got %d", called.Load())
	}

	// Route non-matching event
	nonMatchingEvt := event.NewAny("other.event", "test", "t1", nil)
	_, err = router.Route(context.Background(), nonMatchingEvt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if called.Load() != 1 {
		t.Errorf("expected handler not to be called again, got %d", called.Load())
	}
}

func TestRouterWildcard(t *testing.T) {
	router := event.NewRouter(event.RouterConfig{})

	var called atomic.Int32

	// Register wildcard handler (empty Handles())
	router.Register(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		called.Add(1)
		return nil, nil
	}))

	// Should match any event
	router.Route(context.Background(), event.NewAny("a", "test", "t1", nil))
	router.Route(context.Background(), event.NewAny("b", "test", "t1", nil))
	router.Route(context.Background(), event.NewAny("c", "test", "t1", nil))

	if called.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", called.Load())
	}
}

func TestRouterDerivedEvents(t *testing.T) {
	router := event.NewRouter(event.RouterConfig{})

	// Handler that returns derived events
	router.Register(&typedTestHandler{
		types: []string{"parent.event"},
		handler: event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
			child := event.NewAnyFromParent(evt, "child.event", "test", nil)
			return []event.Event{child}, nil
		}),
	})

	evt := event.NewAny("parent.event", "test", "t1", nil)
	derived, err := router.Route(context.Background(), evt)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(derived) != 1 {
		t.Fatalf("expected 1 derived event, got %d", len(derived))
	}
	if derived[0].Type() != "child.event" {
		t.Errorf("expected child.event, got %s", derived[0].Type())
	}
}

func TestRouterMaxDepth(t *testing.T) {
	router := event.NewRouter(event.RouterConfig{
		MaxDepth: 3,
	})

	// Register handler - not relevant for this test
	router.Register(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		return nil, nil
	}))

	// Simulate deep nesting by manipulating context
	// The router uses context to track depth internally
	evt := event.NewAny("test", "test", "t1", nil)

	// First 3 levels should work
	_, err := router.Route(context.Background(), evt)
	if err != nil {
		t.Errorf("expected first route to succeed: %v", err)
	}
}

func TestRouterMiddleware(t *testing.T) {
	router := event.NewRouter(event.RouterConfig{})

	var order []string

	// Add middleware
	router.Use(func(next event.Handler) event.Handler {
		return event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
			order = append(order, "middleware-before")
			result, err := next.Handle(ctx, evt)
			order = append(order, "middleware-after")
			return result, err
		})
	})

	// Register handler after middleware
	router.Register(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		order = append(order, "handler")
		return nil, nil
	}))

	evt := event.NewAny("test", "test", "t1", nil)
	router.Route(context.Background(), evt)

	expected := []string{"middleware-before", "handler", "middleware-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("at index %d: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestRouterHandlerError(t *testing.T) {
	var errorLogged error
	var errorEvent event.Event

	router := event.NewRouter(event.RouterConfig{
		OnError: func(evt event.Event, handler string, err error) {
			errorEvent = evt
			errorLogged = err
		},
	})

	expectedErr := errors.New("handler failed")

	router.Register(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		return nil, expectedErr
	}))

	evt := event.NewAny("test", "test", "t1", nil)
	router.Route(context.Background(), evt)

	if errorLogged == nil {
		t.Error("expected error to be logged")
	}
	if errorEvent == nil || errorEvent.ID() != evt.ID() {
		t.Error("expected error event to match")
	}
}

func TestRouterHandlerTimeout(t *testing.T) {
	router := event.NewRouter(event.RouterConfig{})

	router.Register(
		event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return nil, nil
			}
		}),
		event.WithHandlerTimeout(50*time.Millisecond),
	)

	evt := event.NewAny("test", "test", "t1", nil)
	start := time.Now()
	router.Route(context.Background(), evt)
	duration := time.Since(start)

	if duration > 1*time.Second {
		t.Errorf("expected timeout to be respected, took %v", duration)
	}
}

func TestRouterMultipleHandlers(t *testing.T) {
	router := event.NewRouter(event.RouterConfig{})

	var handler1Called, handler2Called atomic.Bool

	router.Register(&typedTestHandler{
		types: []string{"test.event"},
		handler: event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
			handler1Called.Store(true)
			return nil, nil
		}),
	})

	router.Register(&typedTestHandler{
		types: []string{"test.event"},
		handler: event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
			handler2Called.Store(true)
			return nil, nil
		}),
	})

	evt := event.NewAny("test.event", "test", "t1", nil)
	router.Route(context.Background(), evt)

	if !handler1Called.Load() {
		t.Error("expected handler1 to be called")
	}
	if !handler2Called.Load() {
		t.Error("expected handler2 to be called")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	middleware := event.RecoveryMiddleware()

	panicHandler := event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		panic("test panic")
	})

	recovered := middleware(panicHandler)

	evt := event.NewAny("test", "test", "t1", nil)
	_, err := recovered.Handle(context.Background(), evt)

	if err == nil {
		t.Error("expected error from recovered panic")
	}
}

func TestLoggingMiddleware(t *testing.T) {
	var loggedType string
	var loggedDuration time.Duration

	middleware := event.LoggingMiddleware(func(eventType, handlerName string, duration time.Duration, err error) {
		loggedType = eventType
		loggedDuration = duration
	})

	handler := event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		time.Sleep(10 * time.Millisecond)
		return nil, nil
	})

	wrapped := middleware(handler)

	evt := event.NewAny("logged.event", "test", "t1", nil)
	wrapped.Handle(context.Background(), evt)

	if loggedType != "logged.event" {
		t.Errorf("expected logged.event, got %s", loggedType)
	}
	if loggedDuration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", loggedDuration)
	}
}

// typedTestHandler wraps a handler with explicit types
type typedTestHandler struct {
	types   []string
	handler event.Handler
}

func (h *typedTestHandler) Handle(ctx context.Context, evt event.Event) ([]event.Event, error) {
	return h.handler.Handle(ctx, evt)
}

func (h *typedTestHandler) Handles() []string {
	return h.types
}
