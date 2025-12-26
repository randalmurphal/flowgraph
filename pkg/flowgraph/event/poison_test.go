package event_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/event"
)

func TestPoisonPillDetector(t *testing.T) {
	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 3,
		WindowDuration:   1 * time.Hour,
	})
	defer detector.Close()

	evt := event.NewAny("test.event", "test", "t1", map[string]string{"key": "value"})

	// Initially not a poison pill
	isPoisonPill, err := detector.Check(context.Background(), evt)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if isPoisonPill {
		t.Error("expected event not to be poison pill initially")
	}

	// Record failures
	for i := 0; i < 2; i++ {
		failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
		detector.Record(context.Background(), failed)
	}

	// Still not a poison pill (need 3)
	isPoisonPill, _ = detector.Check(context.Background(), evt)
	if isPoisonPill {
		t.Error("expected event not to be poison pill after 2 failures")
	}

	// Third failure
	failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
	detector.Record(context.Background(), failed)

	// Now it should be a poison pill
	isPoisonPill, _ = detector.Check(context.Background(), evt)
	if !isPoisonPill {
		t.Error("expected event to be poison pill after 3 failures")
	}
}

func TestPoisonPillDetectorOnDetect(t *testing.T) {
	var detected atomic.Int32

	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 2,
		OnDetect: func(evt event.Event, count int) {
			detected.Add(1)
		},
	})
	defer detector.Close()

	evt := event.NewAny("test.event", "test", "t1", nil)

	// First failure - no callback
	failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
	detector.Record(context.Background(), failed)

	if detected.Load() != 0 {
		t.Error("expected OnDetect not to be called after 1 failure")
	}

	// Second failure - callback triggered
	detector.Record(context.Background(), failed)

	if detected.Load() != 1 {
		t.Error("expected OnDetect to be called once at threshold")
	}

	// Third failure - callback not called again
	detector.Record(context.Background(), failed)

	if detected.Load() != 1 {
		t.Error("expected OnDetect to be called only once")
	}
}

func TestPoisonPillDetectorClear(t *testing.T) {
	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 2,
	})
	defer detector.Close()

	evt := event.NewAny("test.event", "test", "t1", nil)

	// Record failures to make it a poison pill
	for i := 0; i < 3; i++ {
		failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
		detector.Record(context.Background(), failed)
	}

	isPoisonPill, _ := detector.Check(context.Background(), evt)
	if !isPoisonPill {
		t.Fatal("expected poison pill")
	}

	// Clear the record
	detector.ClearEvent(context.Background(), evt)

	// Should no longer be a poison pill
	isPoisonPill, _ = detector.Check(context.Background(), evt)
	if isPoisonPill {
		t.Error("expected event not to be poison pill after clear")
	}
}

func TestPoisonPillDetectorDifferentPayloads(t *testing.T) {
	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 2,
	})
	defer detector.Close()

	// Two events with same type but different payloads
	evt1 := event.NewAny("test.event", "test", "t1", map[string]string{"key": "value1"})
	evt2 := event.NewAny("test.event", "test", "t1", map[string]string{"key": "value2"})

	// Record failures for evt1
	for i := 0; i < 3; i++ {
		failed := event.NewFailedEvent(evt1, errors.New("error"), "handler")
		detector.Record(context.Background(), failed)
	}

	// evt1 should be poison pill
	isPoisonPill, _ := detector.Check(context.Background(), evt1)
	if !isPoisonPill {
		t.Error("expected evt1 to be poison pill")
	}

	// evt2 should NOT be poison pill (different payload)
	isPoisonPill, _ = detector.Check(context.Background(), evt2)
	if isPoisonPill {
		t.Error("expected evt2 not to be poison pill")
	}
}

func TestPoisonPillDetectorList(t *testing.T) {
	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 2,
	})
	defer detector.Close()

	// Record failures for different events
	evt1 := event.NewAny("test.event.1", "test", "t1", nil)
	evt2 := event.NewAny("test.event.2", "test", "t1", nil)

	for i := 0; i < 3; i++ {
		detector.Record(context.Background(), event.NewFailedEvent(evt1, errors.New("error"), "handler"))
	}
	detector.Record(context.Background(), event.NewFailedEvent(evt2, errors.New("error"), "handler"))

	// List all patterns
	patterns, err := detector.List(context.Background())
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if len(patterns) != 2 {
		t.Errorf("expected 2 patterns, got %d", len(patterns))
	}

	// Check that one is poison pill and one is not
	poisonPills := 0
	for _, p := range patterns {
		if p.IsPoisonPill {
			poisonPills++
		}
	}

	if poisonPills != 1 {
		t.Errorf("expected 1 poison pill, got %d", poisonPills)
	}
}

func TestPoisonPillDetectorStats(t *testing.T) {
	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 2,
	})
	defer detector.Close()

	evt1 := event.NewAny("test.event.1", "test", "t1", nil)
	evt2 := event.NewAny("test.event.2", "test", "t1", nil)
	evt3 := event.NewAny("test.event.3", "test", "t1", nil)

	// Make evt1 a poison pill
	for i := 0; i < 2; i++ {
		detector.Record(context.Background(), event.NewFailedEvent(evt1, errors.New("error"), "handler"))
	}

	// Record single failure for others
	detector.Record(context.Background(), event.NewFailedEvent(evt2, errors.New("error"), "handler"))
	detector.Record(context.Background(), event.NewFailedEvent(evt3, errors.New("error"), "handler"))

	stats := detector.Stats()

	if stats.TrackedPatterns != 3 {
		t.Errorf("expected 3 tracked patterns, got %d", stats.TrackedPatterns)
	}

	if stats.PoisonPillCount != 1 {
		t.Errorf("expected 1 poison pill, got %d", stats.PoisonPillCount)
	}
}

func TestPoisonPillMiddleware(t *testing.T) {
	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 2,
	})
	defer detector.Close()

	var handlerCalled atomic.Int32

	router := event.NewRouter(event.RouterConfig{})

	// Add poison pill middleware
	router.Use(event.PoisonPillMiddleware(detector))

	// Add handler
	router.Register(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		handlerCalled.Add(1)
		return nil, errors.New("always fails")
	}))

	evt := event.NewAny("test.event", "test", "t1", nil)

	// First two calls should execute handler (and record failures)
	router.Route(context.Background(), evt)
	router.Route(context.Background(), evt)

	if handlerCalled.Load() != 2 {
		t.Errorf("expected handler called 2 times, got %d", handlerCalled.Load())
	}

	// Third call should be blocked by middleware (poison pill detected)
	// The router doesn't propagate errors, so we check that the handler wasn't called
	router.Route(context.Background(), evt)

	// Handler should not have been called again (blocked by middleware)
	if handlerCalled.Load() != 2 {
		t.Errorf("expected handler still at 2 calls (blocked by middleware), got %d", handlerCalled.Load())
	}

	// Verify the event is now detected as poison pill
	isPoisonPill, _ := detector.Check(context.Background(), evt)
	if !isPoisonPill {
		t.Error("expected event to be marked as poison pill")
	}
}

func TestDLQWithPoisonPillDetection(t *testing.T) {
	baseDLQ := event.NewInMemoryDLQ(event.DLQConfig{
		MaxRetries: 5,
		RetryDelay: 1 * time.Millisecond,
	})

	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 2,
	})
	defer detector.Close()

	var parkedPoisonPill atomic.Bool

	dlq := event.NewDLQWithPoisonPillDetection(baseDLQ, detector)
	dlq.OnPoisonPill = func(evt event.Event) bool {
		parkedPoisonPill.Store(true)
		return true // Auto-park
	}

	evt := event.NewAny("test.event", "test", "t1", nil)

	// First failure
	failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
	dlq.Enqueue(context.Background(), failed)

	if parkedPoisonPill.Load() {
		t.Error("should not be parked after 1 failure")
	}

	// Second failure - should trigger poison pill detection
	dlq.Enqueue(context.Background(), failed)

	if !parkedPoisonPill.Load() {
		t.Error("expected poison pill to be detected and parked")
	}

	// Check that it was moved to parked
	parkedLen, _ := baseDLQ.ParkedLen(context.Background())
	if parkedLen != 1 {
		t.Errorf("expected 1 parked event, got %d", parkedLen)
	}
}

func TestPoisonPillCustomHashFunc(t *testing.T) {
	// Custom hash that ignores payload differences
	customHash := func(evt event.Event) string {
		return evt.Type() // Only hash by type
	}

	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 2,
		HashFunc:         customHash,
	})
	defer detector.Close()

	// Two events with same type but different payloads
	evt1 := event.NewAny("test.event", "test", "t1", map[string]string{"key": "value1"})
	evt2 := event.NewAny("test.event", "test", "t1", map[string]string{"key": "value2"})

	// Record failures for both events
	detector.Record(context.Background(), event.NewFailedEvent(evt1, errors.New("error"), "handler"))
	detector.Record(context.Background(), event.NewFailedEvent(evt2, errors.New("error"), "handler"))

	// Both should be poison pills now (same hash)
	isPoisonPill1, _ := detector.Check(context.Background(), evt1)
	isPoisonPill2, _ := detector.Check(context.Background(), evt2)

	if !isPoisonPill1 || !isPoisonPill2 {
		t.Error("expected both events to be poison pills with custom hash")
	}
}

func TestPoisonPillGetFailureCount(t *testing.T) {
	detector := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 5,
	})
	defer detector.Close()

	evt := event.NewAny("test.event", "test", "t1", nil)

	// Get hash for the event
	hash := ""
	customHash := func(e event.Event) string {
		h := "test-hash"
		hash = h
		return h
	}

	detector2 := event.NewInMemoryPoisonPillDetector(event.InMemoryPoisonPillConfig{
		FailureThreshold: 5,
		HashFunc:         customHash,
	})
	defer detector2.Close()

	// Record some failures
	for i := 0; i < 3; i++ {
		failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
		detector2.Record(context.Background(), failed)
	}

	// Check failure count
	count, err := detector2.GetFailureCount(context.Background(), hash)
	if err != nil {
		t.Fatalf("failed to get failure count: %v", err)
	}

	if count != 3 {
		t.Errorf("expected failure count 3, got %d", count)
	}
}
