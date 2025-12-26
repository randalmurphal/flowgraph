// Package signal provides Temporal-inspired signal primitives for flowgraph.
//
// Signals are fire-and-forget messages sent to running workflows. They allow
// external actors to inject information or trigger actions in a running flow
// without blocking or waiting for a response.
//
// Common use cases:
//   - Cancellation requests
//   - Priority changes
//   - Human task approvals
//   - External event notifications
//
// Design Influences:
//   - Temporal Workflow Signals (fire-and-forget pattern)
//   - Go channels (non-blocking communication)
package signal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status represents the current state of a signal.
type Status string

// Signal status constants.
const (
	StatusPending   Status = "pending"
	StatusProcessed Status = "processed"
	StatusFailed    Status = "failed"
)

// Signal is a fire-and-forget message to a running workflow.
type Signal struct {
	// ID uniquely identifies this signal.
	ID string `json:"id"`

	// Name is the signal type (e.g., "cancel", "approve").
	Name string `json:"name"`

	// TargetID is the workflow/run ID this signal is sent to.
	TargetID string `json:"target_id"`

	// Payload contains signal-specific data.
	Payload map[string]any `json:"payload,omitempty"`

	// SenderID identifies who sent the signal.
	SenderID string `json:"sender_id,omitempty"`

	// Status is the current signal status.
	Status Status `json:"status"`

	// Timestamps
	SentAt      time.Time  `json:"sent_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`

	// Error contains error details if processing failed.
	Error string `json:"error,omitempty"`
}

// NewSignal creates a new signal with the given name and target.
func NewSignal(name, targetID string, payload map[string]any) *Signal {
	return &Signal{
		ID:       fmt.Sprintf("sig-%s", uuid.New().String()[:8]),
		Name:     name,
		TargetID: targetID,
		Payload:  payload,
		Status:   StatusPending,
		SentAt:   time.Now(),
	}
}

// WithSender sets the sender ID on the signal.
func (s *Signal) WithSender(senderID string) *Signal {
	s.SenderID = senderID
	return s
}

// Clone creates a deep copy of the signal.
func (s *Signal) Clone() *Signal {
	signalCopy := *s
	if s.Payload != nil {
		signalCopy.Payload = make(map[string]any, len(s.Payload))
		for k, v := range s.Payload {
			signalCopy.Payload[k] = v
		}
	}
	if s.ProcessedAt != nil {
		t := *s.ProcessedAt
		signalCopy.ProcessedAt = &t
	}
	return &signalCopy
}

// Handler processes a signal for a specific target.
type Handler func(ctx context.Context, targetID string, signal *Signal) error

// Registry manages signal handlers by signal name.
type Registry struct {
	handlers map[string]Handler
	mu       sync.RWMutex
}

// NewRegistry creates a new signal registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register adds a handler for a signal name.
func (r *Registry) Register(signalName string, handler Handler) error {
	if signalName == "" {
		return errors.New("signal name is required")
	}
	if handler == nil {
		return errors.New("handler is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[signalName]; exists {
		return fmt.Errorf("handler for signal %q already registered", signalName)
	}

	r.handlers[signalName] = handler
	return nil
}

// MustRegister registers a handler, panicking on error.
func (r *Registry) MustRegister(signalName string, handler Handler) {
	if err := r.Register(signalName, handler); err != nil {
		panic(err)
	}
}

// Get returns the handler for a signal name.
func (r *Registry) Get(signalName string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, exists := r.handlers[signalName]
	return handler, exists
}

// List returns all registered signal names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

// Unregister removes a handler for a signal name.
func (r *Registry) Unregister(signalName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, signalName)
}

// ErrSignalNotFound is returned when a signal cannot be found.
var ErrSignalNotFound = errors.New("signal not found")

// ErrNoHandler is returned when no handler exists for a signal.
var ErrNoHandler = errors.New("no handler for signal")

// Store persists and retrieves signals.
type Store interface {
	// Enqueue adds a signal for delivery.
	Enqueue(ctx context.Context, signal *Signal) error

	// Dequeue returns pending signals for a target.
	Dequeue(ctx context.Context, targetID string) ([]*Signal, error)

	// Get retrieves a signal by ID.
	Get(ctx context.Context, signalID string) (*Signal, error)

	// MarkProcessed marks a signal as successfully processed.
	MarkProcessed(ctx context.Context, signalID string) error

	// MarkFailed marks a signal as failed with an error.
	MarkFailed(ctx context.Context, signalID string, err error) error

	// ListByTarget returns all signals for a target.
	ListByTarget(ctx context.Context, targetID string) ([]*Signal, error)

	// Delete removes a signal.
	Delete(ctx context.Context, signalID string) error
}

// MemoryStore is an in-memory Store implementation.
type MemoryStore struct {
	signals  map[string]*Signal
	byTarget map[string][]string // targetID -> signal IDs
	mu       sync.RWMutex
}

// NewMemoryStore creates a new in-memory signal store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		signals:  make(map[string]*Signal),
		byTarget: make(map[string][]string),
	}
}

// Enqueue adds a signal for delivery.
func (s *MemoryStore) Enqueue(_ context.Context, signal *Signal) error {
	if signal.ID == "" {
		signal.ID = fmt.Sprintf("sig-%s", uuid.New().String()[:8])
	}
	if signal.SentAt.IsZero() {
		signal.SentAt = time.Now()
	}
	if signal.Status == "" {
		signal.Status = StatusPending
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.signals[signal.ID] = signal.Clone()
	s.byTarget[signal.TargetID] = append(s.byTarget[signal.TargetID], signal.ID)

	return nil
}

// Dequeue returns pending signals for a target.
func (s *MemoryStore) Dequeue(_ context.Context, targetID string) ([]*Signal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	signalIDs := s.byTarget[targetID]
	var pending []*Signal
	for _, id := range signalIDs {
		if sig := s.signals[id]; sig != nil && sig.Status == StatusPending {
			pending = append(pending, sig.Clone())
		}
	}
	return pending, nil
}

// Get retrieves a signal by ID.
func (s *MemoryStore) Get(_ context.Context, signalID string) (*Signal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sig, exists := s.signals[signalID]
	if !exists {
		return nil, ErrSignalNotFound
	}
	return sig.Clone(), nil
}

// MarkProcessed marks a signal as successfully processed.
func (s *MemoryStore) MarkProcessed(_ context.Context, signalID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sig, exists := s.signals[signalID]
	if !exists {
		return ErrSignalNotFound
	}

	now := time.Now()
	sig.Status = StatusProcessed
	sig.ProcessedAt = &now
	return nil
}

// MarkFailed marks a signal as failed.
func (s *MemoryStore) MarkFailed(_ context.Context, signalID string, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sig, exists := s.signals[signalID]
	if !exists {
		return ErrSignalNotFound
	}

	now := time.Now()
	sig.Status = StatusFailed
	sig.ProcessedAt = &now
	if err != nil {
		sig.Error = err.Error()
	}
	return nil
}

// ListByTarget returns all signals for a target.
func (s *MemoryStore) ListByTarget(_ context.Context, targetID string) ([]*Signal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	signalIDs := s.byTarget[targetID]
	result := make([]*Signal, 0, len(signalIDs))
	for _, id := range signalIDs {
		if sig := s.signals[id]; sig != nil {
			result = append(result, sig.Clone())
		}
	}
	return result, nil
}

// Delete removes a signal.
func (s *MemoryStore) Delete(_ context.Context, signalID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sig, exists := s.signals[signalID]
	if !exists {
		return ErrSignalNotFound
	}

	// Remove from byTarget index
	targetSignals := s.byTarget[sig.TargetID]
	for i, id := range targetSignals {
		if id == signalID {
			s.byTarget[sig.TargetID] = append(targetSignals[:i], targetSignals[i+1:]...)
			break
		}
	}

	delete(s.signals, signalID)
	return nil
}

// Dispatcher sends and processes signals.
type Dispatcher struct {
	registry *Registry
	store    Store
	logger   *slog.Logger
}

// NewDispatcher creates a new signal dispatcher.
func NewDispatcher(registry *Registry, store Store) *Dispatcher {
	return &Dispatcher{
		registry: registry,
		store:    store,
		logger:   slog.Default(),
	}
}

// WithLogger sets the logger for the dispatcher.
func (d *Dispatcher) WithLogger(logger *slog.Logger) *Dispatcher {
	d.logger = logger
	return d
}

// Send sends a signal to a target.
func (d *Dispatcher) Send(ctx context.Context, signal *Signal) error {
	if signal.TargetID == "" {
		return errors.New("target ID is required")
	}
	if signal.Name == "" {
		return errors.New("signal name is required")
	}

	if err := d.store.Enqueue(ctx, signal); err != nil {
		return fmt.Errorf("failed to enqueue signal: %w", err)
	}

	d.logger.Debug("signal sent",
		"signal_id", signal.ID,
		"signal_name", signal.Name,
		"target_id", signal.TargetID,
	)

	return nil
}

// Process processes all pending signals for a target.
func (d *Dispatcher) Process(ctx context.Context, targetID string) error {
	signals, err := d.store.Dequeue(ctx, targetID)
	if err != nil {
		return fmt.Errorf("failed to dequeue signals: %w", err)
	}

	for _, sig := range signals {
		if processErr := d.processOne(ctx, sig); processErr != nil {
			d.logger.Error("signal processing failed",
				"signal_id", sig.ID,
				"signal_name", sig.Name,
				"target_id", targetID,
				"error", processErr,
			)
			// Continue processing other signals
		}
	}

	return nil
}

// processOne processes a single signal.
func (d *Dispatcher) processOne(ctx context.Context, sig *Signal) error {
	handler, exists := d.registry.Get(sig.Name)
	if !exists {
		d.logger.Warn("no handler for signal",
			"signal_name", sig.Name,
			"signal_id", sig.ID,
		)
		if markErr := d.store.MarkFailed(ctx, sig.ID, ErrNoHandler); markErr != nil {
			d.logger.Error("failed to mark signal as failed",
				"signal_id", sig.ID,
				"error", markErr,
			)
		}
		return ErrNoHandler
	}

	if handleErr := handler(ctx, sig.TargetID, sig); handleErr != nil {
		if markErr := d.store.MarkFailed(ctx, sig.ID, handleErr); markErr != nil {
			d.logger.Error("failed to mark signal as failed",
				"signal_id", sig.ID,
				"error", markErr,
			)
		}
		return handleErr
	}

	if markErr := d.store.MarkProcessed(ctx, sig.ID); markErr != nil {
		d.logger.Error("failed to mark signal as processed",
			"signal_id", sig.ID,
			"error", markErr,
		)
	}

	d.logger.Debug("signal processed",
		"signal_id", sig.ID,
		"signal_name", sig.Name,
		"target_id", sig.TargetID,
	)

	return nil
}

// ProcessOne processes a specific signal by ID.
func (d *Dispatcher) ProcessOne(ctx context.Context, signalID string) error {
	sig, err := d.store.Get(ctx, signalID)
	if err != nil {
		return err
	}
	return d.processOne(ctx, sig)
}
