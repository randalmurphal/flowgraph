package event

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Aggregator combines multiple events into one aggregated result.
type Aggregator interface {
	// Add contributes an event to aggregation.
	Add(ctx context.Context, evt Event) error

	// Complete returns the aggregated event.
	Complete(ctx context.Context) (Event, error)

	// IsComplete returns true if aggregation criteria are met.
	IsComplete() bool

	// Events returns all collected events.
	Events() []Event

	// CorrelationID returns the correlation ID for this aggregation.
	CorrelationID() string
}

// WindowConfig configures time-based aggregation windows.
type WindowConfig struct {
	// Duration is the window size.
	Duration time.Duration

	// MinEvents is the minimum events needed for completion.
	MinEvents int

	// MaxEvents triggers early completion.
	MaxEvents int

	// Sliding enables sliding windows (vs tumbling).
	Sliding bool
}

// DefaultWindowConfig provides reasonable defaults.
var DefaultWindowConfig = WindowConfig{
	Duration:  5 * time.Minute,
	MinEvents: 1,
	MaxEvents: 100,
}

// CorrelationAggregator aggregates events by correlation ID.
type CorrelationAggregator struct {
	correlationID string
	window        WindowConfig
	events        []Event
	mu            sync.Mutex
	startTime     time.Time
	completed     bool
}

// NewCorrelationAggregator creates a correlation-based aggregator.
func NewCorrelationAggregator(correlationID string, window WindowConfig) *CorrelationAggregator {
	return &CorrelationAggregator{
		correlationID: correlationID,
		window:        window,
		events:        make([]Event, 0),
		startTime:     time.Now(),
	}
}

// Add contributes an event to the aggregation.
func (a *CorrelationAggregator) Add(_ context.Context, evt Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.completed {
		return fmt.Errorf("aggregator already completed")
	}

	// Verify correlation ID matches
	if evt.CorrelationID() != a.correlationID {
		return fmt.Errorf("correlation ID mismatch: expected %s, got %s",
			a.correlationID, evt.CorrelationID())
	}

	a.events = append(a.events, evt)

	// Check if max events reached
	if a.window.MaxEvents > 0 && len(a.events) >= a.window.MaxEvents {
		a.completed = true
	}

	return nil
}

// Complete returns the aggregated event.
func (a *CorrelationAggregator) Complete(ctx context.Context) (Event, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.events) < a.window.MinEvents {
		return nil, fmt.Errorf("not enough events: have %d, need %d",
			len(a.events), a.window.MinEvents)
	}

	a.completed = true

	// Create aggregated event
	payload := AggregatedPayload{
		Events:        a.events,
		EventCount:    len(a.events),
		CorrelationID: a.correlationID,
		StartTime:     a.startTime,
		EndTime:       time.Now(),
	}

	// Determine tenant ID from first event
	tenantID := ""
	if len(a.events) > 0 {
		tenantID = a.events[0].TenantID()
	}

	return New(
		"aggregation.completed",
		"aggregator",
		tenantID,
		payload,
		WithCorrelationID(a.correlationID),
	), nil
}

// IsComplete returns true if aggregation criteria are met.
func (a *CorrelationAggregator) IsComplete() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.completed {
		return true
	}

	// Check time window
	if a.window.Duration > 0 && time.Since(a.startTime) >= a.window.Duration {
		return len(a.events) >= a.window.MinEvents
	}

	// Check max events
	if a.window.MaxEvents > 0 && len(a.events) >= a.window.MaxEvents {
		return true
	}

	return false
}

// Events returns all collected events.
func (a *CorrelationAggregator) Events() []Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Event(nil), a.events...)
}

// CorrelationID returns the correlation ID for this aggregation.
func (a *CorrelationAggregator) CorrelationID() string {
	return a.correlationID
}

// AggregatedPayload is the payload for aggregated events.
type AggregatedPayload struct {
	Events        []Event   `json:"-"` // Not serialized directly
	EventIDs      []string  `json:"event_ids"`
	EventTypes    []string  `json:"event_types"`
	EventCount    int       `json:"event_count"`
	CorrelationID string    `json:"correlation_id"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
}

// CountAggregator aggregates events by count.
type CountAggregator struct {
	correlationID string
	expectedCount int
	events        []Event
	mu            sync.Mutex
}

// NewCountAggregator creates a count-based aggregator.
func NewCountAggregator(correlationID string, expectedCount int) *CountAggregator {
	return &CountAggregator{
		correlationID: correlationID,
		expectedCount: expectedCount,
		events:        make([]Event, 0, expectedCount),
	}
}

// Add contributes an event.
func (a *CountAggregator) Add(_ context.Context, evt Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if evt.CorrelationID() != a.correlationID {
		return fmt.Errorf("correlation ID mismatch")
	}

	a.events = append(a.events, evt)
	return nil
}

// Complete returns the aggregated event.
func (a *CountAggregator) Complete(ctx context.Context) (Event, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.events) < a.expectedCount {
		return nil, fmt.Errorf("not enough events: have %d, expected %d",
			len(a.events), a.expectedCount)
	}

	payload := AggregatedPayload{
		Events:        a.events,
		EventCount:    len(a.events),
		CorrelationID: a.correlationID,
	}

	tenantID := ""
	if len(a.events) > 0 {
		tenantID = a.events[0].TenantID()
	}

	return New(
		"aggregation.completed",
		"aggregator",
		tenantID,
		payload,
		WithCorrelationID(a.correlationID),
	), nil
}

// IsComplete returns true if expected count is reached.
func (a *CountAggregator) IsComplete() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.events) >= a.expectedCount
}

// Events returns collected events.
func (a *CountAggregator) Events() []Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Event(nil), a.events...)
}

// CorrelationID returns the correlation ID.
func (a *CountAggregator) CorrelationID() string {
	return a.correlationID
}

// AggregatorRegistry manages active aggregations.
type AggregatorRegistry struct {
	mu          sync.RWMutex
	aggregators map[string]Aggregator
	cleanup     time.Duration
	closeCh     chan struct{}
}

// NewAggregatorRegistry creates a new registry.
func NewAggregatorRegistry(cleanupInterval time.Duration) *AggregatorRegistry {
	r := &AggregatorRegistry{
		aggregators: make(map[string]Aggregator),
		cleanup:     cleanupInterval,
		closeCh:     make(chan struct{}),
	}

	if cleanupInterval > 0 {
		go r.cleanupLoop()
	}

	return r
}

// Get retrieves an aggregator by correlation ID.
func (r *AggregatorRegistry) Get(correlationID string) (Aggregator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agg, ok := r.aggregators[correlationID]
	return agg, ok
}

// GetOrCreate retrieves or creates an aggregator.
func (r *AggregatorRegistry) GetOrCreate(
	correlationID string,
	factory func() Aggregator,
) Aggregator {
	r.mu.RLock()
	agg, ok := r.aggregators[correlationID]
	r.mu.RUnlock()

	if ok {
		return agg
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if agg, ok := r.aggregators[correlationID]; ok {
		return agg
	}

	agg = factory()
	r.aggregators[correlationID] = agg
	return agg
}

// Remove removes an aggregator.
func (r *AggregatorRegistry) Remove(correlationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.aggregators, correlationID)
}

// Close stops the registry.
func (r *AggregatorRegistry) Close() {
	close(r.closeCh)
}

// cleanupLoop removes completed aggregators.
func (r *AggregatorRegistry) cleanupLoop() {
	ticker := time.NewTicker(r.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			for id, agg := range r.aggregators {
				if agg.IsComplete() {
					delete(r.aggregators, id)
				}
			}
			r.mu.Unlock()

		case <-r.closeCh:
			return
		}
	}
}
