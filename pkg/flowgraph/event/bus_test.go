package event_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/event"
)

func TestBus(t *testing.T) {
	bus := event.NewBus(event.BusConfig{
		BufferSize: 10,
	})
	defer bus.Close()

	var received atomic.Int32

	// Subscribe to specific types
	sub := bus.Subscribe([]string{"test.event"}, event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		received.Add(1)
		return nil, nil
	}))
	defer sub.Unsubscribe()

	// Publish matching event
	evt := event.NewAny("test.event", "test", "t1", nil)
	err := bus.Publish(context.Background(), evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 received event, got %d", received.Load())
	}

	// Publish non-matching event
	nonMatchingEvt := event.NewAny("other.event", "test", "t1", nil)
	bus.Publish(context.Background(), nonMatchingEvt)

	time.Sleep(50 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected still 1 received event, got %d", received.Load())
	}
}

func TestBusSubscribeAll(t *testing.T) {
	bus := event.NewBus(event.BusConfig{
		BufferSize: 10,
	})
	defer bus.Close()

	var received atomic.Int32

	// Subscribe to all events
	sub := bus.SubscribeAll(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		received.Add(1)
		return nil, nil
	}))
	defer sub.Unsubscribe()

	// Publish various events
	bus.Publish(context.Background(), event.NewAny("a", "test", "t1", nil))
	bus.Publish(context.Background(), event.NewAny("b", "test", "t1", nil))
	bus.Publish(context.Background(), event.NewAny("c", "test", "t1", nil))

	time.Sleep(50 * time.Millisecond)

	if received.Load() != 3 {
		t.Errorf("expected 3 received events, got %d", received.Load())
	}
}

func TestBusPauseResume(t *testing.T) {
	bus := event.NewBus(event.BusConfig{
		BufferSize: 10,
	})
	defer bus.Close()

	var received atomic.Int32

	sub := bus.Subscribe([]string{"test"}, event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		received.Add(1)
		return nil, nil
	}))
	defer sub.Unsubscribe()

	// Publish while active
	bus.Publish(context.Background(), event.NewAny("test", "test", "t1", nil))
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 event, got %d", received.Load())
	}

	// Pause
	sub.Pause()
	if !sub.IsPaused() {
		t.Error("expected subscription to be paused")
	}

	// Publish while paused
	bus.Publish(context.Background(), event.NewAny("test", "test", "t1", nil))
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected still 1 event while paused, got %d", received.Load())
	}

	// Resume
	sub.Resume()
	if sub.IsPaused() {
		t.Error("expected subscription to be resumed")
	}

	// Publish after resume
	bus.Publish(context.Background(), event.NewAny("test", "test", "t1", nil))
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 2 {
		t.Errorf("expected 2 events after resume, got %d", received.Load())
	}
}

func TestBusUnsubscribe(t *testing.T) {
	bus := event.NewBus(event.BusConfig{
		BufferSize: 10,
	})
	defer bus.Close()

	var received atomic.Int32

	sub := bus.Subscribe([]string{"test"}, event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		received.Add(1)
		return nil, nil
	}))

	// Publish before unsubscribe
	bus.Publish(context.Background(), event.NewAny("test", "test", "t1", nil))
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 event, got %d", received.Load())
	}

	// Unsubscribe
	sub.Unsubscribe()

	// Publish after unsubscribe
	bus.Publish(context.Background(), event.NewAny("test", "test", "t1", nil))
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected still 1 event after unsubscribe, got %d", received.Load())
	}
}

func TestBusDeduplication(t *testing.T) {
	bus := event.NewBus(event.BusConfig{
		BufferSize:     10,
		DeduplicateTTL: 1 * time.Second,
	})
	defer bus.Close()

	var received atomic.Int32

	sub := bus.SubscribeAll(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		received.Add(1)
		return nil, nil
	}))
	defer sub.Unsubscribe()

	// Create event with specific ID
	evt := event.NewAny("test", "test", "t1", nil, event.WithEventID("dup-id"))

	// Publish same event twice
	bus.Publish(context.Background(), evt)
	bus.Publish(context.Background(), evt)
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 1 {
		t.Errorf("expected 1 event (deduplicated), got %d", received.Load())
	}

	// Different event ID should not be deduplicated
	evt2 := event.NewAny("test", "test", "t1", nil)
	bus.Publish(context.Background(), evt2)
	time.Sleep(50 * time.Millisecond)

	if received.Load() != 2 {
		t.Errorf("expected 2 events total, got %d", received.Load())
	}
}

func TestBusNonBlocking(t *testing.T) {
	var dropped atomic.Int32

	bus := event.NewBus(event.BusConfig{
		BufferSize:  1,
		NonBlocking: true,
		OnDrop: func(evt event.Event, subscriberID string) {
			dropped.Add(1)
		},
	})
	defer bus.Close()

	// Create slow subscriber
	sub := bus.SubscribeAll(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	}))
	defer sub.Unsubscribe()

	// Flood with events
	for i := 0; i < 10; i++ {
		bus.Publish(context.Background(), event.NewAny("test", "test", "t1", nil))
	}

	time.Sleep(50 * time.Millisecond)

	// Some events should have been dropped
	if dropped.Load() == 0 {
		t.Error("expected some events to be dropped")
	}
}

func TestBusClose(t *testing.T) {
	bus := event.NewBus(event.BusConfig{
		BufferSize: 10,
	})

	// Subscribe
	sub := bus.SubscribeAll(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		return nil, nil
	}))
	_ = sub

	// Close
	err := bus.Close()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Publish after close should fail
	evt := event.NewAny("test", "test", "t1", nil)
	err = bus.Publish(context.Background(), evt)
	if err == nil {
		t.Error("expected error when publishing to closed bus")
	}
}

func TestBusFanOut(t *testing.T) {
	bus := event.NewBus(event.BusConfig{
		BufferSize: 10,
	})
	defer bus.Close()

	var received1, received2, received3 atomic.Int32

	// Create multiple subscribers for same event type
	sub1 := bus.Subscribe([]string{"test"}, event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		received1.Add(1)
		return nil, nil
	}))
	defer sub1.Unsubscribe()

	sub2 := bus.Subscribe([]string{"test"}, event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		received2.Add(1)
		return nil, nil
	}))
	defer sub2.Unsubscribe()

	sub3 := bus.Subscribe([]string{"test"}, event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		received3.Add(1)
		return nil, nil
	}))
	defer sub3.Unsubscribe()

	// Publish one event
	bus.Publish(context.Background(), event.NewAny("test", "test", "t1", nil))
	time.Sleep(50 * time.Millisecond)

	// All three should receive it (fan-out)
	if received1.Load() != 1 || received2.Load() != 1 || received3.Load() != 1 {
		t.Errorf("expected all 3 subscribers to receive event, got %d, %d, %d",
			received1.Load(), received2.Load(), received3.Load())
	}
}
