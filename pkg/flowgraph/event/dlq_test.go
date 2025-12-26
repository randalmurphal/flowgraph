package event_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/event"
)

func TestInMemoryDLQ(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		MaxSize:    100,
		MaxRetries: 3,
		RetryDelay: 10 * time.Millisecond,
	})

	evt := event.NewAny("test.event", "test", "t1", nil)
	failed := event.NewFailedEvent(evt, errors.New("handler failed"), "testHandler")

	// Enqueue
	err := dlq.Enqueue(context.Background(), failed)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Check length
	length, err := dlq.Len(context.Background())
	if err != nil {
		t.Fatalf("failed to get length: %v", err)
	}
	if length != 1 {
		t.Errorf("expected length 1, got %d", length)
	}

	// Dequeue immediately (should be empty, not ready yet)
	events, err := dlq.Dequeue(context.Background(), 10)
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events (not ready), got %d", len(events))
	}

	// Wait for retry delay
	time.Sleep(20 * time.Millisecond)

	// Dequeue should now return the event
	events, err = dlq.Dequeue(context.Background(), 10)
	if err != nil {
		t.Fatalf("failed to dequeue: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventID != evt.ID() {
		t.Errorf("wrong event returned")
	}
}

func TestDLQMaxRetries(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		MaxRetries: 2,
		RetryDelay: 1 * time.Millisecond,
	})

	evt := event.NewAny("test.event", "test", "t1", nil)

	// First failure
	failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
	dlq.Enqueue(context.Background(), failed)

	// Simulate retry failures until max retries
	for i := 0; i < 2; i++ {
		time.Sleep(5 * time.Millisecond)
		events, _ := dlq.Dequeue(context.Background(), 10)
		if len(events) > 0 {
			dlq.RecordRetryFailure(context.Background(), events[0])
		}
	}

	// Event should now be parked
	parkedLen, _ := dlq.ParkedLen(context.Background())
	if parkedLen != 1 {
		t.Errorf("expected 1 parked event, got %d", parkedLen)
	}

	// DLQ should be empty
	dlqLen, _ := dlq.Len(context.Background())
	if dlqLen != 0 {
		t.Errorf("expected DLQ to be empty, got %d", dlqLen)
	}
}

func TestDLQRecoverParked(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		MaxRetries: 1,
		RetryDelay: 1 * time.Millisecond,
	})

	evt := event.NewAny("test.event", "test", "t1", nil)
	failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
	failed.AttemptCount = 1 // Already at max

	// Enqueue should go straight to parked
	dlq.Enqueue(context.Background(), failed)

	parkedLen, _ := dlq.ParkedLen(context.Background())
	if parkedLen != 1 {
		t.Fatalf("expected 1 parked event, got %d", parkedLen)
	}

	// Recover the parked event
	err := dlq.RecoverParked(context.Background(), evt.ID())
	if err != nil {
		t.Fatalf("failed to recover: %v", err)
	}

	// Should be back in DLQ
	dlqLen, _ := dlq.Len(context.Background())
	if dlqLen != 1 {
		t.Errorf("expected 1 event in DLQ, got %d", dlqLen)
	}

	parkedLen, _ = dlq.ParkedLen(context.Background())
	if parkedLen != 0 {
		t.Errorf("expected 0 parked events, got %d", parkedLen)
	}
}

func TestDLQDeleteParked(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		MaxRetries: 1,
		RetryDelay: 1 * time.Millisecond,
	})

	evt := event.NewAny("test.event", "test", "t1", nil)
	failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
	failed.AttemptCount = 1

	dlq.Enqueue(context.Background(), failed)

	// Delete the parked event
	err := dlq.DeleteParked(context.Background(), evt.ID())
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	parkedLen, _ := dlq.ParkedLen(context.Background())
	if parkedLen != 0 {
		t.Errorf("expected 0 parked events, got %d", parkedLen)
	}
}

func TestDLQMoveToParked(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		MaxRetries: 5,
		RetryDelay: 1 * time.Millisecond,
	})

	evt := event.NewAny("test.event", "test", "t1", nil)
	failed := event.NewFailedEvent(evt, errors.New("error"), "handler")

	dlq.Enqueue(context.Background(), failed)

	// Manually move to parked
	err := dlq.MoveToParked(context.Background(), evt.ID(), "manual intervention")
	if err != nil {
		t.Fatalf("failed to move to parked: %v", err)
	}

	parkedLen, _ := dlq.ParkedLen(context.Background())
	if parkedLen != 1 {
		t.Errorf("expected 1 parked event, got %d", parkedLen)
	}

	dlqLen, _ := dlq.Len(context.Background())
	if dlqLen != 0 {
		t.Errorf("expected empty DLQ, got %d", dlqLen)
	}
}

func TestDLQStats(t *testing.T) {
	var enqueueCalled, parkCalled atomic.Int32

	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		MaxRetries: 1,
		RetryDelay: 1 * time.Millisecond,
		OnEnqueue: func(*event.FailedEvent) {
			enqueueCalled.Add(1)
		},
		OnPark: func(*event.ParkedEvent) {
			parkCalled.Add(1)
		},
	})

	// Enqueue some events
	for i := 0; i < 3; i++ {
		evt := event.NewAny("test.event", "test", "t1", nil)
		failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
		dlq.Enqueue(context.Background(), failed)
	}

	if enqueueCalled.Load() != 3 {
		t.Errorf("expected OnEnqueue called 3 times, got %d", enqueueCalled.Load())
	}

	stats := dlq.Stats()
	if stats.QueueSize != 3 {
		t.Errorf("expected queue size 3, got %d", stats.QueueSize)
	}
	if stats.Enqueued != 3 {
		t.Errorf("expected enqueued 3, got %d", stats.Enqueued)
	}
}

func TestDLQMaxSize(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		MaxSize:    2,
		RetryDelay: 1 * time.Minute,
	})

	// Fill the DLQ
	for i := 0; i < 2; i++ {
		evt := event.NewAny("test.event", "test", "t1", nil)
		failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
		dlq.Enqueue(context.Background(), failed)
	}

	// Third one should fail
	evt := event.NewAny("test.event", "test", "t1", nil)
	failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
	err := dlq.Enqueue(context.Background(), failed)

	if err == nil {
		t.Error("expected error when DLQ is full")
	}
}

func TestDLQListParked(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		NoRetries: true, // Everything goes straight to parked
	})

	// Create parked events
	for i := 0; i < 5; i++ {
		evt := event.NewAny("test.event", "test", "t1", nil)
		failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
		dlq.Enqueue(context.Background(), failed)
	}

	// List with limit
	parked, err := dlq.ListParked(context.Background(), 3)
	if err != nil {
		t.Fatalf("failed to list parked: %v", err)
	}
	if len(parked) != 3 {
		t.Errorf("expected 3 parked events, got %d", len(parked))
	}

	// List all
	parked, _ = dlq.ListParked(context.Background(), 0)
	if len(parked) != 5 {
		t.Errorf("expected 5 parked events, got %d", len(parked))
	}
}

func TestDLQProcessor(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		RetryDelay: 1 * time.Millisecond,
	})

	var processed atomic.Int32

	router := event.NewRouter(event.RouterConfig{})
	router.Register(event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		processed.Add(1)
		return nil, nil
	}))

	processor := event.NewDLQProcessor(dlq, router, event.DLQProcessorConfig{
		BatchSize:    10,
		PollInterval: 10 * time.Millisecond,
	})

	// Enqueue some failed events
	for i := 0; i < 3; i++ {
		evt := event.NewAny("test.event", "test", "t1", nil)
		failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
		dlq.Enqueue(context.Background(), failed)
	}

	// Wait for retry delay
	time.Sleep(5 * time.Millisecond)

	// Start processor
	ctx, cancel := context.WithCancel(context.Background())
	processor.Start(ctx)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Stop processor
	processor.Stop()
	cancel()

	if processed.Load() != 3 {
		t.Errorf("expected 3 events processed, got %d", processed.Load())
	}
}

func TestDLQAcknowledge(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		RetryDelay: 1 * time.Millisecond,
	})

	evt := event.NewAny("test.event", "test", "t1", nil)
	failed := event.NewFailedEvent(evt, errors.New("error"), "handler")

	dlq.Enqueue(context.Background(), failed)

	// Wait and dequeue
	time.Sleep(5 * time.Millisecond)
	events, _ := dlq.Dequeue(context.Background(), 10)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	// Acknowledge success
	err := dlq.Acknowledge(context.Background(), events[0].EventID)
	if err != nil {
		t.Fatalf("failed to acknowledge: %v", err)
	}

	// Queue should be empty
	length, _ := dlq.Len(context.Background())
	if length != 0 {
		t.Errorf("expected empty DLQ after acknowledge, got %d", length)
	}
}

func TestDLQCountByType(t *testing.T) {
	dlq := event.NewInMemoryDLQ(event.DLQConfig{
		RetryDelay: 1 * time.Minute, // Long delay so events stay queued
	})

	// Add events of different types
	for i := 0; i < 3; i++ {
		evt := event.NewAny("type.a", "test", "t1", nil)
		failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
		dlq.Enqueue(context.Background(), failed)
	}
	for i := 0; i < 2; i++ {
		evt := event.NewAny("type.b", "test", "t1", nil)
		failed := event.NewFailedEvent(evt, errors.New("error"), "handler")
		dlq.Enqueue(context.Background(), failed)
	}

	counts, err := dlq.CountByType(context.Background())
	if err != nil {
		t.Fatalf("failed to count by type: %v", err)
	}

	if counts["type.a"] != 3 {
		t.Errorf("expected 3 type.a, got %d", counts["type.a"])
	}
	if counts["type.b"] != 2 {
		t.Errorf("expected 2 type.b, got %d", counts["type.b"])
	}
}
