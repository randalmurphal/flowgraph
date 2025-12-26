package saga

import (
	"context"
	"fmt"
	"sync"
)

// Store persists and retrieves saga executions for durability.
// Implementations must be safe for concurrent use.
type Store interface {
	// Create persists a new execution.
	Create(ctx context.Context, execution *Execution) error

	// Update persists changes to an existing execution.
	Update(ctx context.Context, execution *Execution) error

	// Get retrieves an execution by ID.
	Get(ctx context.Context, executionID string) (*Execution, error)

	// List returns executions matching the filter.
	List(ctx context.Context, filter *ListFilter) ([]*Execution, error)

	// Delete removes an execution.
	Delete(ctx context.Context, executionID string) error
}

// ListFilter specifies criteria for listing executions.
type ListFilter struct {
	// SagaName filters by saga definition name.
	SagaName string

	// Status filters by execution status.
	Status Status

	// Limit is the maximum number of results.
	Limit int

	// Offset is the number of results to skip.
	Offset int
}

// ErrExecutionNotFound is returned when an execution cannot be found.
var ErrExecutionNotFound = fmt.Errorf("execution not found")

// MemoryStore is an in-memory Store implementation.
// Suitable for testing and single-instance deployments.
type MemoryStore struct {
	executions map[string]*Execution
	mu         sync.RWMutex
}

// NewMemoryStore creates a new in-memory saga store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		executions: make(map[string]*Execution),
	}
}

// Create persists a new execution.
func (s *MemoryStore) Create(_ context.Context, execution *Execution) error {
	if execution.ID == "" {
		return fmt.Errorf("execution ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.executions[execution.ID]; exists {
		return fmt.Errorf("execution %q already exists", execution.ID)
	}

	s.executions[execution.ID] = execution.Clone()
	return nil
}

// Update persists changes to an existing execution.
func (s *MemoryStore) Update(_ context.Context, execution *Execution) error {
	if execution.ID == "" {
		return fmt.Errorf("execution ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.executions[execution.ID]; !exists {
		return ErrExecutionNotFound
	}

	s.executions[execution.ID] = execution.Clone()
	return nil
}

// Get retrieves an execution by ID.
func (s *MemoryStore) Get(_ context.Context, executionID string) (*Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exec, exists := s.executions[executionID]
	if !exists {
		return nil, ErrExecutionNotFound
	}

	return exec.Clone(), nil
}

// List returns executions matching the filter.
func (s *MemoryStore) List(_ context.Context, filter *ListFilter) ([]*Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Execution
	for _, exec := range s.executions {
		// Apply filters
		if filter != nil {
			if filter.SagaName != "" && exec.SagaName != filter.SagaName {
				continue
			}
			if filter.Status != "" && exec.Status != filter.Status {
				continue
			}
		}
		result = append(result, exec.Clone())
	}

	// Apply pagination
	if filter != nil {
		if filter.Offset > 0 {
			if filter.Offset >= len(result) {
				return []*Execution{}, nil
			}
			result = result[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(result) {
			result = result[:filter.Limit]
		}
	}

	return result, nil
}

// Delete removes an execution.
func (s *MemoryStore) Delete(_ context.Context, executionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.executions[executionID]; !exists {
		return ErrExecutionNotFound
	}

	delete(s.executions, executionID)
	return nil
}

// Compile-time check that MemoryStore implements Store.
var _ Store = (*MemoryStore)(nil)
