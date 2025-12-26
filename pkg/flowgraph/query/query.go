// Package query provides Temporal-inspired query primitives for flowgraph.
//
// Queries are read-only operations that retrieve information from running
// workflows without modifying their state. They are synchronous and return
// a result immediately.
//
// Common use cases:
//   - Get current workflow status
//   - Check progress percentage
//   - Retrieve workflow variables
//   - Inspect pending human tasks
//
// Design Influences:
//   - Temporal Workflow Queries (synchronous read-only inspection)
//   - GraphQL queries (data fetching without side effects)
package query

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Handler executes a query and returns a result.
// Handlers must not modify workflow state.
type Handler func(ctx context.Context, targetID string, args any) (any, error)

// Registry manages query handlers by query name.
type Registry struct {
	handlers map[string]Handler
	mu       sync.RWMutex
}

// NewRegistry creates a new query registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register adds a handler for a query name.
func (r *Registry) Register(queryName string, handler Handler) error {
	if queryName == "" {
		return errors.New("query name is required")
	}
	if handler == nil {
		return errors.New("handler is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[queryName]; exists {
		return fmt.Errorf("handler for query %q already registered", queryName)
	}

	r.handlers[queryName] = handler
	return nil
}

// MustRegister registers a handler, panicking on error.
func (r *Registry) MustRegister(queryName string, handler Handler) {
	if err := r.Register(queryName, handler); err != nil {
		panic(err)
	}
}

// Get returns the handler for a query name.
func (r *Registry) Get(queryName string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, exists := r.handlers[queryName]
	return handler, exists
}

// List returns all registered query names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

// Unregister removes a handler for a query name.
func (r *Registry) Unregister(queryName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handlers, queryName)
}

// ErrQueryNotFound is returned when a query handler doesn't exist.
var ErrQueryNotFound = errors.New("query not found")

// ErrTargetNotFound is returned when the query target doesn't exist.
var ErrTargetNotFound = errors.New("target not found")

// Executor runs queries against targets.
type Executor struct {
	registry    *Registry
	stateLoader StateLoader
}

// StateLoader retrieves state for a target.
// This is the integration point with workflow engines.
type StateLoader func(ctx context.Context, targetID string) (*State, error)

// State represents the queryable state of a workflow.
type State struct {
	// TargetID is the workflow/run identifier.
	TargetID string `json:"target_id"`

	// Status is the current workflow status.
	Status string `json:"status"`

	// CurrentNode is the currently executing node ID.
	CurrentNode string `json:"current_node,omitempty"`

	// Progress is completion percentage (0.0 to 1.0).
	Progress float64 `json:"progress"`

	// Variables contains workflow variables.
	Variables map[string]any `json:"variables,omitempty"`

	// PendingTask contains info about a pending human task.
	PendingTask *PendingTask `json:"pending_task,omitempty"`

	// Custom contains additional queryable data.
	Custom map[string]any `json:"custom,omitempty"`
}

// PendingTask represents a task awaiting human input.
type PendingTask struct {
	TaskID      string `json:"task_id"`
	NodeID      string `json:"node_id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Assignee    string `json:"assignee,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// NewExecutor creates a new query executor.
func NewExecutor(registry *Registry, stateLoader StateLoader) *Executor {
	return &Executor{
		registry:    registry,
		stateLoader: stateLoader,
	}
}

// Execute runs a query against a target.
func (e *Executor) Execute(ctx context.Context, targetID, queryName string, args any) (any, error) {
	if targetID == "" {
		return nil, errors.New("target ID is required")
	}
	if queryName == "" {
		return nil, errors.New("query name is required")
	}

	handler, exists := e.registry.Get(queryName)
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrQueryNotFound, queryName)
	}

	return handler(ctx, targetID, args)
}

// Built-in query names.
const (
	QueryStatus      = "status"       // Returns workflow status
	QueryProgress    = "progress"     // Returns completion percentage
	QueryCurrentNode = "current_node" // Returns current node ID
	QueryVariables   = "variables"    // Returns all or specific variable
	QueryPendingTask = "pending_task" // Returns pending human task
	QueryState       = "state"        // Returns full state
)

// RegisterBuiltins registers the standard query handlers.
// The stateLoader is used to retrieve state for built-in queries.
func RegisterBuiltins(registry *Registry, stateLoader StateLoader) error {
	builtins := map[string]Handler{
		QueryStatus: func(ctx context.Context, targetID string, _ any) (any, error) {
			state, err := stateLoader(ctx, targetID)
			if err != nil {
				return nil, err
			}
			if state == nil {
				return nil, fmt.Errorf("%w: %s", ErrTargetNotFound, targetID)
			}
			return state.Status, nil
		},
		QueryProgress: func(ctx context.Context, targetID string, _ any) (any, error) {
			state, err := stateLoader(ctx, targetID)
			if err != nil {
				return nil, err
			}
			if state == nil {
				return nil, fmt.Errorf("%w: %s", ErrTargetNotFound, targetID)
			}
			return state.Progress, nil
		},
		QueryCurrentNode: func(ctx context.Context, targetID string, _ any) (any, error) {
			state, err := stateLoader(ctx, targetID)
			if err != nil {
				return nil, err
			}
			if state == nil {
				return nil, fmt.Errorf("%w: %s", ErrTargetNotFound, targetID)
			}
			return state.CurrentNode, nil
		},
		QueryVariables: func(ctx context.Context, targetID string, args any) (any, error) {
			state, err := stateLoader(ctx, targetID)
			if err != nil {
				return nil, err
			}
			if state == nil {
				return nil, fmt.Errorf("%w: %s", ErrTargetNotFound, targetID)
			}
			// If args is a string, return that specific variable
			if varName, ok := args.(string); ok && varName != "" {
				if val, exists := state.Variables[varName]; exists {
					return val, nil
				}
				return nil, fmt.Errorf("variable %q not found", varName)
			}
			return state.Variables, nil
		},
		QueryPendingTask: func(ctx context.Context, targetID string, _ any) (any, error) {
			state, err := stateLoader(ctx, targetID)
			if err != nil {
				return nil, err
			}
			if state == nil {
				return nil, fmt.Errorf("%w: %s", ErrTargetNotFound, targetID)
			}
			return state.PendingTask, nil
		},
		QueryState: func(ctx context.Context, targetID string, _ any) (any, error) {
			state, err := stateLoader(ctx, targetID)
			if err != nil {
				return nil, err
			}
			if state == nil {
				return nil, fmt.Errorf("%w: %s", ErrTargetNotFound, targetID)
			}
			return state, nil
		},
	}

	for name, handler := range builtins {
		if err := registry.Register(name, handler); err != nil {
			return fmt.Errorf("failed to register builtin query %q: %w", name, err)
		}
	}

	return nil
}

// Result wraps a query result with metadata.
type Result struct {
	// QueryName is the query that was executed.
	QueryName string `json:"query_name"`

	// TargetID is the target that was queried.
	TargetID string `json:"target_id"`

	// Value is the query result.
	Value any `json:"value"`

	// Error contains error details if the query failed.
	Error string `json:"error,omitempty"`
}

// ExecuteMultiple runs multiple queries against a target.
// Returns results for all queries, including any that failed.
func (e *Executor) ExecuteMultiple(ctx context.Context, targetID string, queries map[string]any) []Result {
	results := make([]Result, 0, len(queries))

	for queryName, args := range queries {
		result := Result{
			QueryName: queryName,
			TargetID:  targetID,
		}

		value, err := e.Execute(ctx, targetID, queryName, args)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Value = value
		}

		results = append(results, result)
	}

	return results
}
