package event

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// InMemoryPoisonPillDetector detects poison pill events by tracking
// failure patterns based on content hashes.
type InMemoryPoisonPillDetector struct {
	mu       sync.RWMutex
	failures map[string]*failureRecord
	cfg      InMemoryPoisonPillConfig
	stopCh   chan struct{}
}

// failureRecord tracks failures for a specific event pattern.
type failureRecord struct {
	Hash         string
	EventType    string
	FailureCount int
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
	SampleData   []byte
}

// InMemoryPoisonPillConfig configures poison pill detection.
type InMemoryPoisonPillConfig struct {
	// FailureThreshold is the number of failures before marking as poison.
	// Default: 3
	FailureThreshold int

	// WindowDuration is how long to track failures.
	// Default: 1 hour
	WindowDuration time.Duration

	// HashFunc customizes how event content is hashed.
	// Default: SHA256 of JSON-encoded event data
	HashFunc func(Event) string

	// OnDetect is called when a poison pill is detected.
	OnDetect func(Event, int)

	// CleanupInterval is how often to clean old records.
	// Default: 5 minutes
	CleanupInterval time.Duration
}

// DefaultInMemoryPoisonPillConfig provides reasonable defaults.
var DefaultInMemoryPoisonPillConfig = InMemoryPoisonPillConfig{
	FailureThreshold: 3,
	WindowDuration:   1 * time.Hour,
	CleanupInterval:  5 * time.Minute,
}

// NewInMemoryPoisonPillDetector creates a new detector.
func NewInMemoryPoisonPillDetector(cfg InMemoryPoisonPillConfig) *InMemoryPoisonPillDetector {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = DefaultInMemoryPoisonPillConfig.FailureThreshold
	}
	if cfg.WindowDuration <= 0 {
		cfg.WindowDuration = DefaultInMemoryPoisonPillConfig.WindowDuration
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultInMemoryPoisonPillConfig.CleanupInterval
	}
	if cfg.HashFunc == nil {
		cfg.HashFunc = defaultHashFunc
	}

	d := &InMemoryPoisonPillDetector{
		failures: make(map[string]*failureRecord),
		cfg:      cfg,
		stopCh:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go d.cleanupLoop()

	return d
}

// defaultHashFunc creates a hash from event type and data.
func defaultHashFunc(evt Event) string {
	h := sha256.New()

	// Include event type in hash
	h.Write([]byte(evt.Type()))

	// Include data bytes in hash (consistent across record/check)
	h.Write(evt.DataBytes())

	return hex.EncodeToString(h.Sum(nil))
}

// hashFromFailedEvent creates a consistent hash from a FailedEvent.
func hashFromFailedEvent(hashFunc func(Event) string, failed *FailedEvent) string {
	// Create a wrapper that returns the stored bytes
	evt := &failedEventWrapper{failed: failed}
	return hashFunc(evt)
}

// failedEventWrapper implements Event interface for consistent hashing.
type failedEventWrapper struct {
	failed *FailedEvent
}

func (w *failedEventWrapper) ID() string              { return w.failed.EventID }
func (w *failedEventWrapper) Type() string            { return w.failed.EventType }
func (w *failedEventWrapper) Source() string          { return "" }
func (w *failedEventWrapper) CorrelationID() string   { return "" }
func (w *failedEventWrapper) CausationID() string     { return "" }
func (w *failedEventWrapper) Timestamp() time.Time   { return w.failed.FirstFailedAt }
func (w *failedEventWrapper) Version() int            { return 1 }
func (w *failedEventWrapper) TenantID() string        { return w.failed.TenantID }
func (w *failedEventWrapper) Data() any               { return w.failed.EventData }
func (w *failedEventWrapper) DataBytes() []byte       { return w.failed.EventData }

// Check returns true if the event matches a known poison pill pattern.
func (d *InMemoryPoisonPillDetector) Check(ctx context.Context, evt Event) (bool, error) {
	hash := d.cfg.HashFunc(evt)
	return d.CheckByHash(ctx, hash)
}

// CheckByHash returns true if similar events have failed repeatedly.
func (d *InMemoryPoisonPillDetector) CheckByHash(ctx context.Context, hash string) (bool, error) {
	d.mu.RLock()
	record, exists := d.failures[hash]
	d.mu.RUnlock()

	if !exists {
		return false, nil
	}

	// Check if within time window
	if time.Since(record.FirstSeenAt) > d.cfg.WindowDuration {
		return false, nil
	}

	return record.FailureCount >= d.cfg.FailureThreshold, nil
}

// Record records a failure for an event.
func (d *InMemoryPoisonPillDetector) Record(ctx context.Context, failed *FailedEvent) error {
	// Hash based on event type and data bytes to ensure consistency
	hash := hashFromFailedEvent(d.cfg.HashFunc, failed)
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	record, exists := d.failures[hash]
	if !exists {
		record = &failureRecord{
			Hash:        hash,
			EventType:   failed.EventType,
			FirstSeenAt: now,
			SampleData:  failed.EventData,
		}
		d.failures[hash] = record
	}

	record.FailureCount++
	record.LastSeenAt = now

	// Trigger callback if threshold reached
	if record.FailureCount == d.cfg.FailureThreshold && d.cfg.OnDetect != nil {
		evt := &failedEventWrapper{failed: failed}
		d.cfg.OnDetect(evt, record.FailureCount)
	}

	return nil
}

// GetFailureCount returns the number of failures for an event hash.
func (d *InMemoryPoisonPillDetector) GetFailureCount(ctx context.Context, hash string) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	record, exists := d.failures[hash]
	if !exists {
		return 0, nil
	}
	return record.FailureCount, nil
}

// Clear removes the failure record for an event hash.
func (d *InMemoryPoisonPillDetector) Clear(ctx context.Context, hash string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.failures, hash)
	return nil
}

// ClearEvent removes the failure record for an event.
func (d *InMemoryPoisonPillDetector) ClearEvent(ctx context.Context, evt Event) error {
	hash := d.cfg.HashFunc(evt)
	return d.Clear(ctx, hash)
}

// List returns all tracked failure patterns.
func (d *InMemoryPoisonPillDetector) List(ctx context.Context) ([]*PoisonPillInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*PoisonPillInfo, 0, len(d.failures))
	for _, record := range d.failures {
		result = append(result, &PoisonPillInfo{
			Hash:         record.Hash,
			EventType:    record.EventType,
			FailureCount: record.FailureCount,
			FirstSeenAt:  record.FirstSeenAt,
			LastSeenAt:   record.LastSeenAt,
			IsPoisonPill: record.FailureCount >= d.cfg.FailureThreshold,
		})
	}

	return result, nil
}

// PoisonPillInfo provides information about a potential poison pill.
type PoisonPillInfo struct {
	Hash         string
	EventType    string
	FailureCount int
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
	IsPoisonPill bool
}

// cleanupLoop periodically removes old failure records.
func (d *InMemoryPoisonPillDetector) cleanupLoop() {
	ticker := time.NewTicker(d.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.cleanup()
		}
	}
}

// cleanup removes expired failure records.
func (d *InMemoryPoisonPillDetector) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for hash, record := range d.failures {
		if now.Sub(record.FirstSeenAt) > d.cfg.WindowDuration {
			delete(d.failures, hash)
		}
	}
}

// Close stops the cleanup goroutine.
func (d *InMemoryPoisonPillDetector) Close() {
	close(d.stopCh)
}

// Stats returns detector statistics.
func (d *InMemoryPoisonPillDetector) Stats() PoisonPillStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := PoisonPillStats{
		TrackedPatterns: len(d.failures),
	}

	for _, record := range d.failures {
		if record.FailureCount >= d.cfg.FailureThreshold {
			stats.PoisonPillCount++
		}
	}

	return stats
}

// PoisonPillStats provides detector statistics.
type PoisonPillStats struct {
	TrackedPatterns int // Number of unique patterns being tracked
	PoisonPillCount int // Number of patterns that are poison pills
}

// PoisonPillMiddleware creates middleware that checks for poison pills.
func PoisonPillMiddleware(detector *InMemoryPoisonPillDetector) MiddlewareFunc {
	return func(next Handler) Handler {
		return HandlerFunc(func(ctx context.Context, evt Event) ([]Event, error) {
			// Check if this event matches a poison pill pattern
			isPoisonPill, err := detector.Check(ctx, evt)
			if err != nil {
				// Log but don't block on detector errors
				return next.Handle(ctx, evt)
			}

			if isPoisonPill {
				return nil, &EventError{
					Event:   evt,
					Message: "event matches poison pill pattern",
				}
			}

			// Process the event
			result, err := next.Handle(ctx, evt)
			if err != nil {
				// Record failure for pattern detection
				failed := NewFailedEvent(evt, err, "")
				_ = detector.Record(ctx, failed)
			}

			return result, err
		})
	}
}

// DLQWithPoisonPillDetection wraps a DLQ to automatically detect poison pills.
type DLQWithPoisonPillDetection struct {
	dlq      DeadLetterQueue
	detector *InMemoryPoisonPillDetector

	// OnPoisonPill is called when a poison pill is detected in the DLQ.
	// Return true to auto-park the event.
	OnPoisonPill func(Event) bool
}

// NewDLQWithPoisonPillDetection creates a DLQ wrapper with poison pill detection.
func NewDLQWithPoisonPillDetection(
	dlq DeadLetterQueue,
	detector *InMemoryPoisonPillDetector,
) *DLQWithPoisonPillDetection {
	return &DLQWithPoisonPillDetection{
		dlq:      dlq,
		detector: detector,
	}
}

// Enqueue adds a failed event and checks for poison pill patterns.
func (d *DLQWithPoisonPillDetection) Enqueue(ctx context.Context, failed *FailedEvent) error {
	// Record failure for pattern detection
	if err := d.detector.Record(ctx, failed); err != nil {
		// Log but don't block on detector errors
	}

	// Create a wrapper for consistent hash checking
	evt := &failedEventWrapper{failed: failed}

	// Check if this is now a poison pill
	isPoisonPill, _ := d.detector.Check(ctx, evt)
	if isPoisonPill && d.OnPoisonPill != nil {
		if d.OnPoisonPill(evt) {
			// Auto-park poison pills
			return d.dlq.MoveToParked(ctx, failed.EventID, "poison pill detected")
		}
	}

	return d.dlq.Enqueue(ctx, failed)
}

// Dequeue returns events ready for retry.
func (d *DLQWithPoisonPillDetection) Dequeue(ctx context.Context, limit int) ([]*FailedEvent, error) {
	return d.dlq.Dequeue(ctx, limit)
}

// DequeueByType retrieves failed events of a specific type.
func (d *DLQWithPoisonPillDetection) DequeueByType(ctx context.Context, eventType string, limit int) ([]*FailedEvent, error) {
	return d.dlq.DequeueByType(ctx, eventType, limit)
}

// Acknowledge marks an event as successfully reprocessed.
func (d *DLQWithPoisonPillDetection) Acknowledge(ctx context.Context, eventID string) error {
	return d.dlq.Acknowledge(ctx, eventID)
}

// Retry updates retry tracking and schedules next attempt.
func (d *DLQWithPoisonPillDetection) Retry(ctx context.Context, eventID string, nextRetryAt time.Time) error {
	return d.dlq.Retry(ctx, eventID, nextRetryAt)
}

// MoveToParked moves an event to the parked letter queue.
func (d *DLQWithPoisonPillDetection) MoveToParked(ctx context.Context, eventID string, reason string) error {
	return d.dlq.MoveToParked(ctx, eventID, reason)
}

// Count returns the number of events in the queue.
func (d *DLQWithPoisonPillDetection) Count(ctx context.Context) (int, error) {
	return d.dlq.Count(ctx)
}

// CountByType returns counts grouped by event type.
func (d *DLQWithPoisonPillDetection) CountByType(ctx context.Context) (map[string]int, error) {
	return d.dlq.CountByType(ctx)
}
