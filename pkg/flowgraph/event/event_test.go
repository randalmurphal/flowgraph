package event_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/event"
)

func TestBaseEvent(t *testing.T) {
	type TestPayload struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
	}

	payload := TestPayload{
		Message: "hello",
		Count:   42,
	}

	evt := event.New(
		"test.created",
		"test",
		"tenant-1",
		payload,
	)

	// Test identity
	if evt.ID() == "" {
		t.Error("expected non-empty ID")
	}
	if evt.Type() != "test.created" {
		t.Errorf("expected type test.created, got %s", evt.Type())
	}
	if evt.Source() != "test" {
		t.Errorf("expected source test, got %s", evt.Source())
	}

	// Test correlation (should default to ID for root events)
	if evt.CorrelationID() != evt.ID() {
		t.Error("expected correlation ID to equal event ID for root event")
	}
	if evt.CausationID() != "" {
		t.Errorf("expected empty causation ID, got %s", evt.CausationID())
	}

	// Test metadata
	if evt.TenantID() != "tenant-1" {
		t.Errorf("expected tenant-1, got %s", evt.TenantID())
	}
	if evt.Version() != 1 {
		t.Errorf("expected version 1, got %d", evt.Version())
	}
	if evt.Timestamp().IsZero() {
		t.Error("expected non-zero timestamp")
	}

	// Test payload
	if evt.TypedData().Message != "hello" {
		t.Errorf("expected message hello, got %s", evt.TypedData().Message)
	}
	if evt.TypedData().Count != 42 {
		t.Errorf("expected count 42, got %d", evt.TypedData().Count)
	}

	// Test Data() returns the payload
	data := evt.Data()
	if data == nil {
		t.Error("expected non-nil data")
	}

	// Test DataBytes
	bytes := evt.DataBytes()
	if len(bytes) == 0 {
		t.Error("expected non-empty bytes")
	}

	var decoded TestPayload
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if decoded.Message != "hello" {
		t.Errorf("expected message hello, got %s", decoded.Message)
	}
}

func TestEventOptions(t *testing.T) {
	customTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	evt := event.New(
		"test.created",
		"test",
		"tenant-1",
		map[string]string{"key": "value"},
		event.WithEventID("custom-id"),
		event.WithCorrelationID("corr-id"),
		event.WithCausationID("cause-id"),
		event.WithTimestamp(customTime),
		event.WithSchemaVersion(2),
	)

	if evt.ID() != "custom-id" {
		t.Errorf("expected custom-id, got %s", evt.ID())
	}
	if evt.CorrelationID() != "corr-id" {
		t.Errorf("expected corr-id, got %s", evt.CorrelationID())
	}
	if evt.CausationID() != "cause-id" {
		t.Errorf("expected cause-id, got %s", evt.CausationID())
	}
	if !evt.Timestamp().Equal(customTime) {
		t.Errorf("expected %v, got %v", customTime, evt.Timestamp())
	}
	if evt.Version() != 2 {
		t.Errorf("expected version 2, got %d", evt.Version())
	}
}

func TestNewFromParent(t *testing.T) {
	parent := event.New(
		"parent.event",
		"test",
		"tenant-1",
		map[string]string{"parent": "data"},
	)

	child := event.NewFromParent(
		parent,
		"child.event",
		"test",
		map[string]string{"child": "data"},
	)

	// Child should inherit correlation ID
	if child.CorrelationID() != parent.ID() {
		t.Errorf("expected correlation ID %s, got %s", parent.ID(), child.CorrelationID())
	}

	// Child should have parent as causation
	if child.CausationID() != parent.ID() {
		t.Errorf("expected causation ID %s, got %s", parent.ID(), child.CausationID())
	}

	// Child should inherit tenant
	if child.TenantID() != parent.TenantID() {
		t.Errorf("expected tenant %s, got %s", parent.TenantID(), child.TenantID())
	}

	// Child should have its own ID
	if child.ID() == parent.ID() {
		t.Error("child should have unique ID")
	}
}

func TestEventJSON(t *testing.T) {
	evt := event.New(
		"test.created",
		"test",
		"tenant-1",
		map[string]string{"key": "value"},
	)

	// Marshal
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal
	var decoded event.BaseEvent[map[string]string]
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ID() != evt.ID() {
		t.Errorf("expected ID %s, got %s", evt.ID(), decoded.ID())
	}
	if decoded.Type() != evt.Type() {
		t.Errorf("expected type %s, got %s", evt.Type(), decoded.Type())
	}
	if decoded.TypedData()["key"] != "value" {
		t.Errorf("expected key=value, got %s", decoded.TypedData()["key"])
	}
}

func TestHandlerFunc(t *testing.T) {
	called := false
	var receivedEvt event.Event

	handler := event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		called = true
		receivedEvt = evt
		return nil, nil
	})

	evt := event.NewAny("test", "test", "t1", nil)
	_, err := handler.Handle(context.Background(), evt)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if receivedEvt.ID() != evt.ID() {
		t.Error("wrong event received")
	}

	// HandlerFunc.Handles() should return nil
	if handler.Handles() != nil {
		t.Error("expected nil from Handles()")
	}
}

func TestTypedHandler(t *testing.T) {
	type Payload struct {
		Value int `json:"value"`
	}

	var receivedPayload Payload
	var receivedMeta event.Metadata

	handler := event.TypedHandler(
		[]string{"typed.event"},
		func(ctx context.Context, payload Payload, meta event.Metadata) ([]event.Event, error) {
			receivedPayload = payload
			receivedMeta = meta
			return nil, nil
		},
	)

	evt := event.New("typed.event", "test", "t1", Payload{Value: 123})
	_, err := handler.Handle(context.Background(), evt)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPayload.Value != 123 {
		t.Errorf("expected value 123, got %d", receivedPayload.Value)
	}
	if receivedMeta.EventType != "typed.event" {
		t.Errorf("expected event type typed.event, got %s", receivedMeta.EventType)
	}

	// Check Handles
	handles := handler.Handles()
	if len(handles) != 1 || handles[0] != "typed.event" {
		t.Errorf("expected [typed.event], got %v", handles)
	}
}

func TestChainMiddleware(t *testing.T) {
	var order []string

	middleware1 := func(next event.Handler) event.Handler {
		return event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
			order = append(order, "m1-before")
			result, err := next.Handle(ctx, evt)
			order = append(order, "m1-after")
			return result, err
		})
	}

	middleware2 := func(next event.Handler) event.Handler {
		return event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
			order = append(order, "m2-before")
			result, err := next.Handle(ctx, evt)
			order = append(order, "m2-after")
			return result, err
		})
	}

	baseHandler := event.HandlerFunc(func(ctx context.Context, evt event.Event) ([]event.Event, error) {
		order = append(order, "handler")
		return nil, nil
	})

	chained := event.ChainMiddleware(baseHandler, middleware1, middleware2)

	evt := event.NewAny("test", "test", "t1", nil)
	chained.Handle(context.Background(), evt)

	expected := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("at index %d: expected %s, got %s", i, v, order[i])
		}
	}
}
