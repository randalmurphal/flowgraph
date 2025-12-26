package event

import (
	"context"
	"sync"
	"time"

	fgerrors "github.com/randalmurphal/flowgraph/pkg/flowgraph/errors"
)

// InMemoryDLQ is an in-memory implementation of DeadLetterQueue.
// Suitable for testing and single-instance deployments.
type InMemoryDLQ struct {
	mu     sync.RWMutex
	events map[string]*FailedEvent // keyed by event ID
	plq    map[string]*ParkedEvent // keyed by event ID
	cfg    DLQConfig

	// Metrics
	enqueued  int64
	retried   int64
	parked    int64
	recovered int64
}

// DLQConfig configures the dead letter queue.
type DLQConfig struct {
	// MaxSize limits the number of events in the DLQ.
	// Default: 10000
	MaxSize int

	// RetryConfig for reprocessing events.
	RetryConfig fgerrors.RetryConfig

	// MaxRetries before moving to PLQ.
	// Default: 5. Use NoRetries=true to disable retries.
	MaxRetries int

	// NoRetries disables retry attempts - events go straight to PLQ.
	// When true, MaxRetries is ignored.
	NoRetries bool

	// RetryDelay before first retry attempt.
	// Default: 1 minute
	RetryDelay time.Duration

	// OnEnqueue is called when an event is added.
	OnEnqueue func(*FailedEvent)

	// OnPark is called when an event is moved to PLQ.
	OnPark func(*ParkedEvent)
}

// DefaultDLQConfig provides reasonable defaults.
var DefaultDLQConfig = DLQConfig{
	MaxSize:     10000,
	MaxRetries:  5,
	RetryDelay:  1 * time.Minute,
	RetryConfig: fgerrors.DefaultRetry,
}

// NewInMemoryDLQ creates a new in-memory dead letter queue.
func NewInMemoryDLQ(cfg DLQConfig) *InMemoryDLQ {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = DefaultDLQConfig.MaxSize
	}
	if cfg.MaxRetries <= 0 && !cfg.NoRetries {
		cfg.MaxRetries = DefaultDLQConfig.MaxRetries
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = DefaultDLQConfig.RetryDelay
	}

	return &InMemoryDLQ{
		events: make(map[string]*FailedEvent),
		plq:    make(map[string]*ParkedEvent),
		cfg:    cfg,
	}
}

// Enqueue adds a failed event to the DLQ.
func (d *InMemoryDLQ) Enqueue(ctx context.Context, failed *FailedEvent) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check max size
	if len(d.events) >= d.cfg.MaxSize {
		return &EventError{
			Message: "DLQ is full",
		}
	}

	// Check if this event should go straight to PLQ
	// NoRetries mode or AttemptCount exceeded MaxRetries
	if d.cfg.NoRetries || failed.AttemptCount >= d.cfg.MaxRetries {
		return d.moveToParkedLocked(failed, "max retries exceeded")
	}

	// Calculate next retry time
	if failed.NextRetryAt.IsZero() {
		failed.NextRetryAt = time.Now().Add(d.cfg.RetryDelay)
	}

	d.events[failed.EventID] = failed
	d.enqueued++

	if d.cfg.OnEnqueue != nil {
		d.cfg.OnEnqueue(failed)
	}

	return nil
}

// Dequeue returns events ready for retry.
func (d *InMemoryDLQ) Dequeue(ctx context.Context, limit int) ([]*FailedEvent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	ready := make([]*FailedEvent, 0, limit)

	for id, evt := range d.events {
		if len(ready) >= limit {
			break
		}
		if !evt.NextRetryAt.After(now) {
			ready = append(ready, evt)
			delete(d.events, id)
		}
	}

	return ready, nil
}

// DequeueByType retrieves failed events of a specific type.
func (d *InMemoryDLQ) DequeueByType(ctx context.Context, eventType string, limit int) ([]*FailedEvent, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	ready := make([]*FailedEvent, 0, limit)

	for id, evt := range d.events {
		if len(ready) >= limit {
			break
		}
		if evt.EventType == eventType && !evt.NextRetryAt.After(now) {
			ready = append(ready, evt)
			delete(d.events, id)
		}
	}

	return ready, nil
}

// Acknowledge marks an event as successfully reprocessed.
func (d *InMemoryDLQ) Acknowledge(ctx context.Context, eventID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.events, eventID)
	d.recovered++
	return nil
}

// Retry updates retry tracking and schedules next attempt.
func (d *InMemoryDLQ) Retry(ctx context.Context, eventID string, nextRetryAt time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	evt, ok := d.events[eventID]
	if !ok {
		return &EventError{Message: "event not found in DLQ"}
	}

	evt.AttemptCount++
	evt.LastFailedAt = time.Now()
	evt.NextRetryAt = nextRetryAt

	if evt.AttemptCount >= d.cfg.MaxRetries {
		delete(d.events, eventID)
		return d.moveToParkedLocked(evt, "max retries exceeded")
	}

	d.retried++
	return nil
}

// MoveToParked moves an event to the parked letter queue.
func (d *InMemoryDLQ) MoveToParked(ctx context.Context, eventID string, reason string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	evt, ok := d.events[eventID]
	if !ok {
		return &EventError{Message: "event not found in DLQ"}
	}

	delete(d.events, eventID)
	return d.moveToParkedLocked(evt, reason)
}

// moveToParkedLocked moves an event to PLQ (must hold lock).
func (d *InMemoryDLQ) moveToParkedLocked(failed *FailedEvent, reason string) error {
	parked := &ParkedEvent{
		FailedEvent:   *failed,
		ParkReason:    reason,
		OriginalError: failed.ErrorMessage,
		ParkedAt:      time.Now(),
	}

	d.plq[failed.EventID] = parked
	d.parked++

	if d.cfg.OnPark != nil {
		d.cfg.OnPark(parked)
	}

	return nil
}

// Count returns the number of events in the queue.
func (d *InMemoryDLQ) Count(ctx context.Context) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.events), nil
}

// CountByType returns counts grouped by event type.
func (d *InMemoryDLQ) CountByType(ctx context.Context) (map[string]int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	counts := make(map[string]int)
	for _, evt := range d.events {
		counts[evt.EventType]++
	}
	return counts, nil
}

// RecordRetrySuccess removes an event from tracking after successful retry.
func (d *InMemoryDLQ) RecordRetrySuccess(ctx context.Context, eventID string) error {
	return d.Acknowledge(ctx, eventID)
}

// RecordRetryFailure updates retry count and reschedules.
func (d *InMemoryDLQ) RecordRetryFailure(ctx context.Context, failed *FailedEvent) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	failed.AttemptCount++
	failed.LastFailedAt = time.Now()

	if failed.AttemptCount >= d.cfg.MaxRetries {
		return d.moveToParkedLocked(failed, "max retries exceeded")
	}

	// Exponential backoff for next retry
	backoff := d.cfg.RetryDelay * time.Duration(1<<uint(failed.AttemptCount))
	failed.NextRetryAt = time.Now().Add(backoff)

	d.events[failed.EventID] = failed
	d.retried++

	return nil
}

// Len returns the number of events in the DLQ (alias for Count).
func (d *InMemoryDLQ) Len(ctx context.Context) (int, error) {
	return d.Count(ctx)
}

// ParkedLen returns the number of parked events.
func (d *InMemoryDLQ) ParkedLen(ctx context.Context) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.plq), nil
}

// ListParked returns parked events.
func (d *InMemoryDLQ) ListParked(ctx context.Context, limit int) ([]*ParkedEvent, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 || limit > len(d.plq) {
		limit = len(d.plq)
	}

	result := make([]*ParkedEvent, 0, limit)
	for _, evt := range d.plq {
		if len(result) >= limit {
			break
		}
		result = append(result, evt)
	}
	return result, nil
}

// RecoverParked moves a parked event back to DLQ for retry.
func (d *InMemoryDLQ) RecoverParked(ctx context.Context, eventID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	parked, ok := d.plq[eventID]
	if !ok {
		return &EventError{Message: "event not found in PLQ"}
	}

	// Reset retry count and move back to DLQ
	failed := &parked.FailedEvent
	failed.AttemptCount = 0
	failed.NextRetryAt = time.Now()

	d.events[eventID] = failed
	delete(d.plq, eventID)
	d.recovered++
	return nil
}

// DeleteParked permanently deletes a parked event.
func (d *InMemoryDLQ) DeleteParked(ctx context.Context, eventID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, ok := d.plq[eventID]; !ok {
		return &EventError{Message: "event not found in PLQ"}
	}

	delete(d.plq, eventID)
	return nil
}

// Stats returns DLQ statistics.
func (d *InMemoryDLQ) Stats() DLQStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return DLQStats{
		QueueSize:  len(d.events),
		ParkedSize: len(d.plq),
		Enqueued:   d.enqueued,
		Retried:    d.retried,
		Parked:     d.parked,
		Recovered:  d.recovered,
	}
}

// DLQStats provides statistics about the DLQ.
type DLQStats struct {
	QueueSize  int   // Current DLQ size
	ParkedSize int   // Current PLQ size
	Enqueued   int64 // Total events enqueued
	Retried    int64 // Total retry attempts
	Parked     int64 // Total events parked
	Recovered  int64 // Total events recovered
}

// DLQProcessor processes events from a DLQ.
type DLQProcessor struct {
	dlq     *InMemoryDLQ
	router  Router
	cfg     DLQProcessorConfig
	stopCh  chan struct{}
	running bool
	mu      sync.Mutex
}

// DLQProcessorConfig configures the DLQ processor.
type DLQProcessorConfig struct {
	// BatchSize is the number of events to process at once.
	// Default: 10
	BatchSize int

	// PollInterval is how often to check for events.
	// Default: 10 seconds
	PollInterval time.Duration

	// OnRetry is called before retrying an event.
	OnRetry func(*FailedEvent)

	// OnSuccess is called after successful retry.
	OnSuccess func(*FailedEvent)

	// OnFailure is called after retry failure.
	OnFailure func(*FailedEvent, error)
}

// DefaultDLQProcessorConfig provides reasonable defaults.
var DefaultDLQProcessorConfig = DLQProcessorConfig{
	BatchSize:    10,
	PollInterval: 10 * time.Second,
}

// NewDLQProcessor creates a new DLQ processor.
func NewDLQProcessor(dlq *InMemoryDLQ, router Router, cfg DLQProcessorConfig) *DLQProcessor {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultDLQProcessorConfig.BatchSize
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultDLQProcessorConfig.PollInterval
	}

	return &DLQProcessor{
		dlq:    dlq,
		router: router,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// Start begins processing events from the DLQ.
func (p *DLQProcessor) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	go p.run(ctx)
}

// Stop halts the processor.
func (p *DLQProcessor) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	close(p.stopCh)
	p.running = false
}

// run is the main processing loop.
func (p *DLQProcessor) run(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.processBatch(ctx)
		}
	}
}

// processBatch processes a batch of events.
func (p *DLQProcessor) processBatch(ctx context.Context) {
	events, err := p.dlq.Dequeue(ctx, p.cfg.BatchSize)
	if err != nil {
		return
	}

	for _, failed := range events {
		if p.cfg.OnRetry != nil {
			p.cfg.OnRetry(failed)
		}

		// Reconstruct event from failed event data for routing
		evt := NewAny(failed.EventType, "", failed.TenantID, failed.EventData,
			WithEventID(failed.EventID))

		_, routeErr := p.router.Route(ctx, evt)
		if routeErr != nil {
			if p.cfg.OnFailure != nil {
				p.cfg.OnFailure(failed, routeErr)
			}
			_ = p.dlq.RecordRetryFailure(ctx, failed)
		} else {
			if p.cfg.OnSuccess != nil {
				p.cfg.OnSuccess(failed)
			}
			_ = p.dlq.RecordRetrySuccess(ctx, failed.EventID)
		}
	}
}
