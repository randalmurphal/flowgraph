// Package event provides event-driven architecture primitives for flowgraph.
//
// # Overview
//
// This package implements industry best practices for event-driven systems:
//
//   - Event interface with correlation and causation tracking
//   - EventRegistry for schema management and validation (Confluent-style)
//   - Router for event dispatch with middleware support
//   - Bus for pub/sub fan-out distribution
//   - Aggregator for fan-in event correlation
//   - DeadLetterQueue and ParkedLetterQueue for error handling
//   - PoisonPillDetector for identifying problematic events
//
// # Design Influences
//
//   - Confluent Schema Registry: Schema versioning and compatibility
//   - AWS EventBridge: Dead letter queues, error handling patterns
//   - Temporal: Signals, queries, workflow interaction patterns
//   - Apache Kafka: Fan-out, fan-in, correlation IDs
//   - LangGraph: Agent-controlled transitions, human-in-the-loop
//
// # Event Interface
//
// All events implement the Event interface, which provides:
//
//   - Identity: ID, Type, Source
//   - Correlation: CorrelationID (traces related events), CausationID (parent event)
//   - Metadata: Timestamp, Version (schema), TenantID
//   - Payload: Data() returns the event payload
//
// Use BaseEvent[T] for type-safe event implementations:
//
//	type OrderCreated struct {
//	    event.BaseEvent[OrderPayload]
//	}
//
//	evt := event.New("order.created", "orders", tenantID, OrderPayload{...})
//
// # Event Correlation
//
// Events support distributed tracing through correlation and causation IDs:
//
//	// Parent event creates a new correlation chain
//	parent := event.New("workflow.started", "flow", tenantID, payload)
//	// parent.CorrelationID() == parent.ID() (root of chain)
//
//	// Child events inherit correlation, set causation
//	child := event.NewFromParent(parent, "step.completed", "flow", stepPayload)
//	// child.CorrelationID() == parent.ID()
//	// child.CausationID() == parent.ID()
//
// # Registry and Schema Validation
//
// EventRegistry manages event type definitions with version support:
//
//	registry := event.NewEventRegistry()
//	registry.Register(&event.EventSchema{
//	    Type:    "order.created",
//	    Source:  "orders",
//	    Version: 1,
//	    Tags:    []string{"commerce", "orders"},
//	})
//
//	// Validate events against their schema
//	if err := registry.Validate(evt); err != nil {
//	    // Handle validation error
//	}
//
// # Router and Middleware
//
// Router dispatches events to registered handlers with middleware support:
//
//	router := event.NewRouter(event.RouterConfig{
//	    MaxDepth: 10,  // Prevent infinite recursion
//	    Registry: registry,
//	    ValidateEvents: true,
//	})
//
//	// Add middleware
//	router.Use(event.RecoveryMiddleware())
//	router.Use(event.LoggingMiddleware(logger))
//
//	// Register handlers
//	router.Register(myHandler, event.WithHandlerTimeout(30*time.Second))
//
//	// Dispatch events
//	derived, err := router.Route(ctx, evt)
//
// # Bus for Pub/Sub
//
// LocalBus provides in-memory pub/sub with fan-out:
//
//	bus := event.NewBus(event.BusConfig{
//	    BufferSize:     256,
//	    DeduplicateTTL: 5*time.Minute,
//	})
//
//	// Subscribe to specific types
//	sub := bus.Subscribe([]string{"order.created"}, handler)
//	defer sub.Unsubscribe()
//
//	// Or subscribe to all events
//	sub := bus.SubscribeAll(auditHandler)
//
//	// Publish events
//	bus.Publish(ctx, evt)
//
// # Aggregation for Fan-In
//
// Aggregators combine multiple related events:
//
//	// Correlation-based aggregation
//	agg := event.NewCorrelationAggregator(correlationID, event.WindowConfig{
//	    Duration:  5*time.Minute,
//	    MinEvents: 3,
//	    MaxEvents: 10,
//	})
//
//	agg.Add(ctx, event1)
//	agg.Add(ctx, event2)
//	agg.Add(ctx, event3)
//
//	if agg.IsComplete() {
//	    aggregatedEvent, _ := agg.Complete(ctx)
//	}
//
// # Error Handling
//
// DeadLetterQueue stores failed events for retry:
//
//	type MyDLQ struct { /* implement DeadLetterQueue */ }
//
//	router := event.NewRouter(event.RouterConfig{
//	    DLQ: myDLQ,
//	})
//
// ParkedLetterQueue stores permanently failed events requiring manual review.
//
// PoisonPillDetector identifies events that consistently cause failures.
package event
