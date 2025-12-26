package event_test

import (
	"errors"
	"testing"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/event"
)

func TestEventRegistry(t *testing.T) {
	registry := event.NewEventRegistry()

	schema := &event.EventSchema{
		Type:        "order.created",
		Source:      "orders",
		Version:     1,
		Description: "Order was created",
		Tags:        []string{"commerce", "orders"},
	}

	// Test Register
	if err := registry.Register(schema); err != nil {
		t.Fatalf("failed to register: %v", err)
	}

	// Test Get
	retrieved, ok := registry.Get("order.created")
	if !ok {
		t.Fatal("expected schema to exist")
	}
	if retrieved.Description != "Order was created" {
		t.Errorf("expected description, got %s", retrieved.Description)
	}

	// Test Has
	if !registry.Has("order.created") {
		t.Error("expected Has to return true")
	}
	if registry.Has("nonexistent") {
		t.Error("expected Has to return false for nonexistent")
	}

	// Test Types
	types := registry.Types()
	if len(types) != 1 || types[0] != "order.created" {
		t.Errorf("expected [order.created], got %v", types)
	}
}

func TestEventRegistryVersioning(t *testing.T) {
	registry := event.NewEventRegistry()

	// Register v1
	registry.Register(&event.EventSchema{
		Type:       "order.created",
		Source:     "orders",
		Version:    1,
		Compatible: []int{},
	})

	// Register v2 (compatible with v1)
	registry.Register(&event.EventSchema{
		Type:       "order.created",
		Source:     "orders",
		Version:    2,
		Compatible: []int{1},
	})

	// Latest should be v2
	latest, ok := registry.Get("order.created")
	if !ok {
		t.Fatal("expected schema to exist")
	}
	if latest.Version != 2 {
		t.Errorf("expected version 2, got %d", latest.Version)
	}

	// Can get specific version
	v1, ok := registry.GetVersion("order.created", 1)
	if !ok {
		t.Fatal("expected v1 to exist")
	}
	if v1.Version != 1 {
		t.Errorf("expected version 1, got %d", v1.Version)
	}

	// LatestVersion
	latestVer, ok := registry.LatestVersion("order.created")
	if !ok || latestVer != 2 {
		t.Errorf("expected latest version 2, got %d", latestVer)
	}

	// Versions
	versions := registry.Versions("order.created")
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestEventSchemaValidation(t *testing.T) {
	registry := event.NewEventRegistry()

	customValidator := func(evt event.Event) error {
		data := evt.Data().(map[string]string)
		if data["required"] == "" {
			return errors.New("required field is empty")
		}
		return nil
	}

	registry.Register(&event.EventSchema{
		Type:      "validated.event",
		Source:    "test",
		Version:   1,
		Validator: customValidator,
	})

	// Valid event
	validEvt := event.NewAny("validated.event", "test", "t1",
		map[string]string{"required": "value"},
		event.WithSchemaVersion(1),
	)
	if err := registry.Validate(validEvt); err != nil {
		t.Errorf("expected valid event to pass: %v", err)
	}

	// Invalid event (wrong type)
	wrongTypeEvt := event.NewAny("wrong.type", "test", "t1", nil)
	if err := registry.Validate(wrongTypeEvt); err == nil {
		t.Error("expected error for unknown type")
	}

	// Invalid event (fails custom validation)
	invalidEvt := event.NewAny("validated.event", "test", "t1",
		map[string]string{"required": ""},
		event.WithSchemaVersion(1),
	)
	if err := registry.Validate(invalidEvt); err == nil {
		t.Error("expected validation error")
	}
}

func TestEventSchemaCompatibility(t *testing.T) {
	schema := &event.EventSchema{
		Type:       "test.event",
		Version:    3,
		Compatible: []int{1, 2},
	}

	// Same version
	if !schema.IsCompatibleWith(3) {
		t.Error("expected compatible with same version")
	}

	// Listed compatible versions
	if !schema.IsCompatibleWith(1) {
		t.Error("expected compatible with v1")
	}
	if !schema.IsCompatibleWith(2) {
		t.Error("expected compatible with v2")
	}

	// Not listed
	if schema.IsCompatibleWith(4) {
		t.Error("expected not compatible with v4")
	}
}

func TestEventRegistryListBySource(t *testing.T) {
	registry := event.NewEventRegistry()

	registry.Register(&event.EventSchema{Type: "order.created", Source: "orders", Version: 1})
	registry.Register(&event.EventSchema{Type: "order.updated", Source: "orders", Version: 1})
	registry.Register(&event.EventSchema{Type: "user.created", Source: "users", Version: 1})

	orderSchemas := registry.ListBySource("orders")
	if len(orderSchemas) != 2 {
		t.Errorf("expected 2 order schemas, got %d", len(orderSchemas))
	}

	userSchemas := registry.ListBySource("users")
	if len(userSchemas) != 1 {
		t.Errorf("expected 1 user schema, got %d", len(userSchemas))
	}
}

func TestEventRegistryListByTag(t *testing.T) {
	registry := event.NewEventRegistry()

	registry.Register(&event.EventSchema{
		Type:    "order.created",
		Source:  "orders",
		Version: 1,
		Tags:    []string{"commerce", "orders"},
	})
	registry.Register(&event.EventSchema{
		Type:    "payment.processed",
		Source:  "payments",
		Version: 1,
		Tags:    []string{"commerce", "payments"},
	})
	registry.Register(&event.EventSchema{
		Type:    "user.created",
		Source:  "users",
		Version: 1,
		Tags:    []string{"users"},
	})

	commerceSchemas := registry.ListByTag("commerce")
	if len(commerceSchemas) != 2 {
		t.Errorf("expected 2 commerce schemas, got %d", len(commerceSchemas))
	}

	userSchemas := registry.ListByTag("users")
	if len(userSchemas) != 1 {
		t.Errorf("expected 1 user schema, got %d", len(userSchemas))
	}
}

func TestEventRegistryRange(t *testing.T) {
	registry := event.NewEventRegistry()

	registry.Register(&event.EventSchema{Type: "a", Source: "test", Version: 1})
	registry.Register(&event.EventSchema{Type: "b", Source: "test", Version: 1})
	registry.Register(&event.EventSchema{Type: "c", Source: "test", Version: 1})

	var types []string
	registry.Range(func(s *event.EventSchema) bool {
		types = append(types, s.Type)
		return true
	})

	if len(types) != 3 {
		t.Errorf("expected 3 types, got %d", len(types))
	}

	// Test early termination
	count := 0
	registry.Range(func(s *event.EventSchema) bool {
		count++
		return count < 2 // Stop after 2
	})

	if count != 2 {
		t.Errorf("expected 2 iterations, got %d", count)
	}
}

func TestRegisterValidation(t *testing.T) {
	registry := event.NewEventRegistry()

	// Empty type should fail
	err := registry.Register(&event.EventSchema{Type: "", Version: 1})
	if err == nil {
		t.Error("expected error for empty type")
	}

	// Zero version should fail
	err = registry.Register(&event.EventSchema{Type: "test", Version: 0})
	if err == nil {
		t.Error("expected error for zero version")
	}

	// Negative version should fail
	err = registry.Register(&event.EventSchema{Type: "test", Version: -1})
	if err == nil {
		t.Error("expected error for negative version")
	}
}
