package event_test

import (
	"context"
	"testing"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/event"
)

func TestCorrelationAggregator(t *testing.T) {
	correlationID := "test-correlation"

	agg := event.NewCorrelationAggregator(correlationID, event.WindowConfig{
		Duration:  5 * time.Minute,
		MinEvents: 2,
		MaxEvents: 5,
	})

	// Initially not complete
	if agg.IsComplete() {
		t.Error("expected aggregator not to be complete initially")
	}

	// Add events
	evt1 := event.NewAny("test.event", "test", "t1", nil, event.WithCorrelationID(correlationID))
	if err := agg.Add(context.Background(), evt1); err != nil {
		t.Fatalf("failed to add event: %v", err)
	}

	// Still not complete (need 2)
	if agg.IsComplete() {
		t.Error("expected aggregator not to be complete with 1 event")
	}

	// Add second event
	evt2 := event.NewAny("test.event", "test", "t1", nil, event.WithCorrelationID(correlationID))
	if err := agg.Add(context.Background(), evt2); err != nil {
		t.Fatalf("failed to add event: %v", err)
	}

	// Now complete (min events reached)
	events := agg.Events()
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}

	// Complete the aggregation
	result, err := agg.Complete(context.Background())
	if err != nil {
		t.Fatalf("failed to complete: %v", err)
	}

	if result.Type() != "aggregation.completed" {
		t.Errorf("expected type aggregation.completed, got %s", result.Type())
	}
	if result.CorrelationID() != correlationID {
		t.Errorf("expected correlation ID %s, got %s", correlationID, result.CorrelationID())
	}
}

func TestCorrelationAggregatorMismatch(t *testing.T) {
	agg := event.NewCorrelationAggregator("expected-id", event.WindowConfig{})

	// Try to add event with different correlation ID
	evt := event.NewAny("test", "test", "t1", nil, event.WithCorrelationID("different-id"))
	err := agg.Add(context.Background(), evt)

	if err == nil {
		t.Error("expected error for correlation ID mismatch")
	}
}

func TestCorrelationAggregatorMaxEvents(t *testing.T) {
	correlationID := "test-correlation"

	agg := event.NewCorrelationAggregator(correlationID, event.WindowConfig{
		MaxEvents: 3,
	})

	// Add 3 events (max)
	for i := 0; i < 3; i++ {
		evt := event.NewAny("test", "test", "t1", nil, event.WithCorrelationID(correlationID))
		agg.Add(context.Background(), evt)
	}

	// Should be complete due to max events
	if !agg.IsComplete() {
		t.Error("expected aggregator to be complete at max events")
	}
}

func TestCountAggregator(t *testing.T) {
	correlationID := "count-test"

	agg := event.NewCountAggregator(correlationID, 3)

	// Add events
	for i := 0; i < 3; i++ {
		evt := event.NewAny("test", "test", "t1", nil, event.WithCorrelationID(correlationID))
		agg.Add(context.Background(), evt)
	}

	// Should be complete
	if !agg.IsComplete() {
		t.Error("expected aggregator to be complete")
	}

	// Complete
	result, err := agg.Complete(context.Background())
	if err != nil {
		t.Fatalf("failed to complete: %v", err)
	}

	if result.Type() != "aggregation.completed" {
		t.Errorf("expected type aggregation.completed, got %s", result.Type())
	}
}

func TestCountAggregatorNotEnough(t *testing.T) {
	correlationID := "count-test"

	agg := event.NewCountAggregator(correlationID, 5)

	// Add only 3 events
	for i := 0; i < 3; i++ {
		evt := event.NewAny("test", "test", "t1", nil, event.WithCorrelationID(correlationID))
		agg.Add(context.Background(), evt)
	}

	// Should not be complete
	if agg.IsComplete() {
		t.Error("expected aggregator not to be complete")
	}

	// Complete should fail
	_, err := agg.Complete(context.Background())
	if err == nil {
		t.Error("expected error when completing with insufficient events")
	}
}

func TestAggregatorRegistry(t *testing.T) {
	registry := event.NewAggregatorRegistry(100 * time.Millisecond)
	defer registry.Close()

	// Get non-existent
	_, ok := registry.Get("nonexistent")
	if ok {
		t.Error("expected false for non-existent aggregator")
	}

	// GetOrCreate
	created := registry.GetOrCreate("test-1", func() event.Aggregator {
		return event.NewCountAggregator("test-1", 3)
	})

	if created == nil {
		t.Fatal("expected aggregator to be created")
	}

	// GetOrCreate again should return same instance
	retrieved := registry.GetOrCreate("test-1", func() event.Aggregator {
		return event.NewCountAggregator("test-1", 5) // Different config
	})

	if retrieved != created {
		t.Error("expected same aggregator instance")
	}

	// Get should work now
	agg, ok := registry.Get("test-1")
	if !ok || agg == nil {
		t.Error("expected to get aggregator")
	}

	// Remove
	registry.Remove("test-1")

	_, ok = registry.Get("test-1")
	if ok {
		t.Error("expected aggregator to be removed")
	}
}

func TestAggregatorRegistryCleanup(t *testing.T) {
	registry := event.NewAggregatorRegistry(50 * time.Millisecond)
	defer registry.Close()

	correlationID := "cleanup-test"

	// Create and complete an aggregator
	agg := registry.GetOrCreate(correlationID, func() event.Aggregator {
		return event.NewCountAggregator(correlationID, 1)
	})

	// Add event to complete it
	evt := event.NewAny("test", "test", "t1", nil, event.WithCorrelationID(correlationID))
	agg.Add(context.Background(), evt)

	// Should be complete
	if !agg.IsComplete() {
		t.Fatal("expected aggregator to be complete")
	}

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Should be cleaned up
	_, ok := registry.Get(correlationID)
	if ok {
		t.Error("expected completed aggregator to be cleaned up")
	}
}

func TestAggregatorCorrelationID(t *testing.T) {
	correlationID := "my-correlation"

	agg := event.NewCorrelationAggregator(correlationID, event.WindowConfig{})

	if agg.CorrelationID() != correlationID {
		t.Errorf("expected correlation ID %s, got %s", correlationID, agg.CorrelationID())
	}

	countAgg := event.NewCountAggregator(correlationID, 5)
	if countAgg.CorrelationID() != correlationID {
		t.Errorf("expected correlation ID %s, got %s", correlationID, countAgg.CorrelationID())
	}
}

func TestAggregatorEvents(t *testing.T) {
	correlationID := "events-test"

	agg := event.NewCorrelationAggregator(correlationID, event.WindowConfig{})

	evt1 := event.NewAny("test.1", "test", "t1", nil, event.WithCorrelationID(correlationID))
	evt2 := event.NewAny("test.2", "test", "t1", nil, event.WithCorrelationID(correlationID))

	agg.Add(context.Background(), evt1)
	agg.Add(context.Background(), evt2)

	events := agg.Events()

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Verify returned slice is a copy
	events[0] = nil
	originalEvents := agg.Events()
	if originalEvents[0] == nil {
		t.Error("expected Events() to return a copy")
	}
}
