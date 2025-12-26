package event

import (
	"context"
	"fmt"
	"time"
)

// EventError represents an error during event processing.
type EventError struct {
	Event     Event     // The event that failed
	Handler   string    // Handler that failed (if known)
	Message   string    // Error message
	Err       error     // Underlying error
	Attempt   int       // Which attempt this was
	Timestamp time.Time // When the error occurred
}

// Error implements error interface.
func (e *EventError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("event %s: %s: %v", e.Event.ID(), e.Message, e.Err)
	}
	return fmt.Sprintf("event %s: %s", e.Event.ID(), e.Message)
}

// Unwrap returns the underlying error.
func (e *EventError) Unwrap() error {
	return e.Err
}

// FailedEvent contains complete information about a failed event.
type FailedEvent struct {
	// Event information
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	EventData []byte `json:"event_data"`
	TenantID  string `json:"tenant_id"`

	// Error information
	ErrorMessage string `json:"error_message"`
	Handler      string `json:"handler,omitempty"`

	// Retry tracking
	AttemptCount  int       `json:"attempt_count"`
	FirstFailedAt time.Time `json:"first_failed_at"`
	LastFailedAt  time.Time `json:"last_failed_at"`
	NextRetryAt   time.Time `json:"next_retry_at,omitempty"`

	// Additional metadata
	Metadata map[string]any `json:"metadata,omitempty"`
}

// NewFailedEvent creates a FailedEvent from an error.
func NewFailedEvent(evt Event, err error, handler string) *FailedEvent {
	now := time.Now()
	return &FailedEvent{
		EventID:       evt.ID(),
		EventType:     evt.Type(),
		EventData:     evt.DataBytes(),
		TenantID:      evt.TenantID(),
		ErrorMessage:  err.Error(),
		Handler:       handler,
		AttemptCount:  0, // No retry attempts yet
		FirstFailedAt: now,
		LastFailedAt:  now,
	}
}

// ParkedEvent represents an event that has been moved to the parked letter queue.
type ParkedEvent struct {
	FailedEvent

	// Parking information
	ParkReason    string     `json:"park_reason"`
	OriginalError string     `json:"original_error,omitempty"`
	ParkedAt      time.Time  `json:"parked_at"`
	ReviewedBy    string     `json:"reviewed_by,omitempty"`
	ReviewedAt    *time.Time `json:"reviewed_at,omitempty"`
}

// DeadLetterQueue stores events that failed processing for later retry.
type DeadLetterQueue interface {
	// Enqueue adds a failed event to the queue.
	Enqueue(ctx context.Context, failed *FailedEvent) error

	// Dequeue retrieves failed events for reprocessing.
	// Events should be ordered by next_retry_at for efficient processing.
	Dequeue(ctx context.Context, limit int) ([]*FailedEvent, error)

	// DequeueByType retrieves failed events of a specific type.
	DequeueByType(ctx context.Context, eventType string, limit int) ([]*FailedEvent, error)

	// Acknowledge marks an event as successfully reprocessed (removes from DLQ).
	Acknowledge(ctx context.Context, eventID string) error

	// Retry updates retry tracking and schedules next attempt.
	Retry(ctx context.Context, eventID string, nextRetryAt time.Time) error

	// MoveToParked moves a permanently failed event to the parked queue.
	MoveToParked(ctx context.Context, eventID string, reason string) error

	// Count returns the number of events in the queue.
	Count(ctx context.Context) (int, error)

	// CountByType returns counts grouped by event type.
	CountByType(ctx context.Context) (map[string]int, error)
}

// ParkedLetterQueue stores events that cannot be processed and require
// manual intervention or permanent archival.
type ParkedLetterQueue interface {
	// Park stores an event that cannot be processed.
	Park(ctx context.Context, evt *ParkedEvent) error

	// List retrieves parked events.
	List(ctx context.Context, limit int) ([]*ParkedEvent, error)

	// Get retrieves a specific parked event.
	Get(ctx context.Context, eventID string) (*ParkedEvent, error)

	// Unpark moves an event back to the DLQ for retry.
	Unpark(ctx context.Context, eventID string) error

	// MarkReviewed records that an event has been reviewed.
	MarkReviewed(ctx context.Context, eventID string, reviewedBy string) error

	// Delete permanently removes a parked event.
	Delete(ctx context.Context, eventID string) error

	// Count returns the number of parked events.
	Count(ctx context.Context) (int, error)
}

// PoisonPillDetector identifies events that consistently cause failures.
// This helps prevent bad events from consuming processing resources indefinitely.
type PoisonPillDetector interface {
	// Record logs a failure for analysis.
	Record(ctx context.Context, failed *FailedEvent) error

	// Check returns true if an event appears to be a poison pill.
	Check(ctx context.Context, evt Event) (bool, error)

	// CheckByHash returns true if similar events have failed repeatedly.
	CheckByHash(ctx context.Context, hash string) (bool, error)

	// GetFailureCount returns the number of failures for an event hash.
	GetFailureCount(ctx context.Context, hash string) (int, error)

	// Clear resets failure tracking for an event hash.
	Clear(ctx context.Context, hash string) error
}

// PoisonPillConfig configures poison pill detection thresholds.
type PoisonPillConfig struct {
	// FailureThreshold is the number of failures before flagging as poison.
	FailureThreshold int

	// WindowDuration is the time window for counting failures.
	WindowDuration time.Duration

	// HashFunc computes a stable hash from event content.
	// If nil, a default hash based on type and payload is used.
	HashFunc func(Event) string
}

// DefaultPoisonPillConfig provides reasonable defaults.
var DefaultPoisonPillConfig = PoisonPillConfig{
	FailureThreshold: 5,
	WindowDuration:   24 * time.Hour,
}
