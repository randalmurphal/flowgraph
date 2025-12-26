// Package saga provides the Saga pattern for distributed transactions.
//
// A Saga is a sequence of steps where each step has a forward action and
// an optional compensation action. If any step fails, all previously
// completed steps are compensated in reverse order.
//
// This package supports both orchestration (centralized coordinator) and
// choreography (event-driven) saga patterns.
//
// Design Influences:
//   - Microservices.io Saga Pattern
//   - AWS Step Functions
//   - Temporal Sagas
package saga

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status represents the state of a saga execution.
type Status string

// Saga status constants.
const (
	StatusPending      Status = "pending"
	StatusRunning      Status = "running"
	StatusCompleted    Status = "completed"
	StatusCompensating Status = "compensating"
	StatusCompensated  Status = "compensated"
	StatusFailed       Status = "failed"
)

// StepHandler executes a saga step.
type StepHandler func(ctx context.Context, input any) (output any, err error)

// Step defines a single step in a saga.
type Step struct {
	// Name identifies this step.
	Name string

	// Handler executes the forward action.
	Handler StepHandler

	// Compensation executes the rollback action.
	// It receives the output from the forward Handler.
	Compensation StepHandler

	// Timeout for this step. Zero means use saga default.
	Timeout time.Duration

	// Optional marks this step as non-critical.
	// If an optional step fails, the saga continues without compensating.
	Optional bool

	// RetryPolicy configures retries for this step.
	RetryPolicy *RetryPolicy
}

// RetryPolicy configures step retry behavior.
type RetryPolicy struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultRetryPolicy returns a sensible retry policy.
var DefaultRetryPolicy = &RetryPolicy{
	MaxAttempts: 3,
	InitialWait: 100 * time.Millisecond,
	MaxWait:     5 * time.Second,
	Multiplier:  2.0,
}

// Definition defines a complete saga workflow.
type Definition struct {
	// Name identifies this saga type.
	Name string

	// Steps are executed in order.
	Steps []Step

	// Timeout is the default timeout per step.
	Timeout time.Duration

	// OnComplete is called when the saga completes successfully.
	OnComplete func(ctx context.Context, execution *Execution)

	// OnCompensate is called when compensation completes.
	OnCompensate func(ctx context.Context, execution *Execution)
}

// Validate checks the saga definition for errors.
func (d *Definition) Validate() error {
	if d.Name == "" {
		return errors.New("saga name is required")
	}
	if len(d.Steps) == 0 {
		return errors.New("saga must have at least one step")
	}
	for i, step := range d.Steps {
		if step.Name == "" {
			return fmt.Errorf("step %d: name is required", i)
		}
		if step.Handler == nil {
			return fmt.Errorf("step %d (%s): handler is required", i, step.Name)
		}
	}
	return nil
}

// StepExecution tracks a single step's execution.
type StepExecution struct {
	StepName   string        `json:"step_name"`
	Status     Status        `json:"status"`
	Input      any           `json:"input,omitempty"`
	Output     any           `json:"output,omitempty"`
	Error      string        `json:"error,omitempty"`
	StartedAt  time.Time     `json:"started_at,omitempty"`
	FinishedAt time.Time     `json:"finished_at,omitempty"`
	Duration   time.Duration `json:"duration,omitempty"`
	Retries    int           `json:"retries"`
}

// Execution tracks the complete saga execution.
type Execution struct {
	ID              string          `json:"id"`
	SagaName        string          `json:"saga_name"`
	Status          Status          `json:"status"`
	Input           any             `json:"input,omitempty"`
	Output          any             `json:"output,omitempty"`
	Error           string          `json:"error,omitempty"`
	Steps           []StepExecution `json:"steps"`
	CurrentStep     int             `json:"current_step"`
	StartedAt       time.Time       `json:"started_at"`
	FinishedAt      time.Time       `json:"finished_at,omitempty"`
	CompensatedAt   *time.Time      `json:"compensated_at,omitempty"`
	CompensateError string          `json:"compensate_error,omitempty"`

	mu sync.Mutex
}

// Clone creates a copy of the execution without the mutex.
func (e *Execution) Clone() *Execution {
	e.mu.Lock()
	defer e.mu.Unlock()

	clone := &Execution{
		ID:              e.ID,
		SagaName:        e.SagaName,
		Status:          e.Status,
		Input:           e.Input,
		Output:          e.Output,
		Error:           e.Error,
		Steps:           make([]StepExecution, len(e.Steps)),
		CurrentStep:     e.CurrentStep,
		StartedAt:       e.StartedAt,
		FinishedAt:      e.FinishedAt,
		CompensatedAt:   e.CompensatedAt,
		CompensateError: e.CompensateError,
	}
	copy(clone.Steps, e.Steps)
	return clone
}

// Orchestrator manages saga executions.
type Orchestrator struct {
	sagas      map[string]*Definition
	executions map[string]*Execution
	mu         sync.RWMutex
	logger     *slog.Logger
}

// NewOrchestrator creates a new saga orchestrator.
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		sagas:      make(map[string]*Definition),
		executions: make(map[string]*Execution),
		logger:     slog.Default(),
	}
}

// WithLogger sets the logger for the orchestrator.
func (o *Orchestrator) WithLogger(logger *slog.Logger) *Orchestrator {
	o.logger = logger
	return o
}

// Register adds a saga definition.
func (o *Orchestrator) Register(saga *Definition) error {
	if err := saga.Validate(); err != nil {
		return err
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.sagas[saga.Name]; exists {
		return fmt.Errorf("saga %q already registered", saga.Name)
	}

	o.sagas[saga.Name] = saga
	return nil
}

// MustRegister registers a saga, panicking on error.
func (o *Orchestrator) MustRegister(saga *Definition) {
	if err := o.Register(saga); err != nil {
		panic(err)
	}
}

// Start begins a new saga execution.
func (o *Orchestrator) Start(ctx context.Context, sagaName string, input any) (*Execution, error) {
	o.mu.RLock()
	saga, exists := o.sagas[sagaName]
	o.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("saga %q not found", sagaName)
	}

	execution := &Execution{
		ID:        fmt.Sprintf("saga-%s", uuid.New().String()[:8]),
		SagaName:  sagaName,
		Status:    StatusRunning,
		Input:     input,
		Steps:     make([]StepExecution, len(saga.Steps)),
		StartedAt: time.Now(),
	}

	// Initialize step executions
	for i, step := range saga.Steps {
		execution.Steps[i] = StepExecution{
			StepName: step.Name,
			Status:   StatusPending,
		}
	}

	o.mu.Lock()
	o.executions[execution.ID] = execution
	o.mu.Unlock()

	// Execute saga steps asynchronously
	go o.execute(ctx, saga, execution)

	return execution, nil
}

// execute runs the saga steps sequentially.
func (o *Orchestrator) execute(ctx context.Context, saga *Definition, execution *Execution) {
	currentOutput := execution.Input
	var stepErr error

	for i := range saga.Steps {
		step := &saga.Steps[i]

		// Check for cancellation
		select {
		case <-ctx.Done():
			o.compensateFrom(ctx, saga, execution, i-1, ctx.Err())
			return
		default:
		}

		stepExec := &execution.Steps[i]
		execution.mu.Lock()
		execution.CurrentStep = i
		stepExec.Status = StatusRunning
		stepExec.StartedAt = time.Now()
		stepExec.Input = currentOutput
		execution.mu.Unlock()

		// Execute step with timeout
		var output any
		output, stepErr = o.executeStep(ctx, saga, step, currentOutput)

		execution.mu.Lock()
		stepExec.FinishedAt = time.Now()
		stepExec.Duration = stepExec.FinishedAt.Sub(stepExec.StartedAt)

		if stepErr != nil {
			stepExec.Status = StatusFailed
			stepExec.Error = stepErr.Error()

			// Handle optional steps
			if step.Optional {
				o.logger.Debug("optional saga step failed, continuing",
					"saga_id", execution.ID,
					"step", step.Name,
					"error", stepErr,
				)
				stepExec.Status = StatusCompleted
				stepErr = nil
			}
		} else {
			stepExec.Status = StatusCompleted
			stepExec.Output = output
			currentOutput = output
		}
		execution.mu.Unlock()

		if stepErr != nil {
			o.logger.Error("saga step failed",
				"saga_id", execution.ID,
				"saga_name", saga.Name,
				"step", step.Name,
				"error", stepErr,
			)
			o.compensateFrom(ctx, saga, execution, i-1, stepErr)
			return
		}

		o.logger.Debug("saga step completed",
			"saga_id", execution.ID,
			"step", step.Name,
		)
	}

	// All steps completed successfully
	execution.mu.Lock()
	execution.Status = StatusCompleted
	execution.Output = currentOutput
	execution.FinishedAt = time.Now()
	execution.mu.Unlock()

	o.logger.Info("saga completed successfully",
		"saga_id", execution.ID,
		"saga_name", saga.Name,
	)

	if saga.OnComplete != nil {
		saga.OnComplete(ctx, execution.Clone())
	}
}

// executeStep runs a single step with timeout.
func (o *Orchestrator) executeStep(
	ctx context.Context,
	saga *Definition,
	step *Step,
	input any,
) (any, error) {
	timeout := step.Timeout
	if timeout == 0 {
		timeout = saga.Timeout
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return step.Handler(stepCtx, input)
}

// compensateFrom runs compensation handlers in reverse order.
func (o *Orchestrator) compensateFrom(
	ctx context.Context,
	saga *Definition,
	execution *Execution,
	fromStep int,
	originalErr error,
) {
	execution.mu.Lock()
	execution.Status = StatusCompensating
	execution.Error = originalErr.Error()
	execution.mu.Unlock()

	o.logger.Info("starting saga compensation",
		"saga_id", execution.ID,
		"saga_name", saga.Name,
		"from_step", fromStep,
		"reason", originalErr,
	)

	var compensateErrors []string

	// Run compensations in reverse order
	for i := fromStep; i >= 0; i-- {
		step := &saga.Steps[i]
		stepExec := &execution.Steps[i]

		// Skip if step wasn't completed or has no compensation
		if stepExec.Status != StatusCompleted || step.Compensation == nil {
			continue
		}

		o.logger.Debug("compensating saga step",
			"saga_id", execution.ID,
			"step", step.Name,
		)

		// Run compensation with the step's output
		_, compErr := step.Compensation(ctx, stepExec.Output)
		if compErr != nil {
			compensateErrors = append(compensateErrors,
				fmt.Sprintf("%s: %s", step.Name, compErr.Error()))
			o.logger.Error("saga compensation failed",
				"saga_id", execution.ID,
				"step", step.Name,
				"error", compErr,
			)
		}
	}

	now := time.Now()
	execution.mu.Lock()
	if len(compensateErrors) > 0 {
		execution.Status = StatusFailed
		execution.CompensateError = fmt.Sprintf("compensation errors: %v", compensateErrors)
	} else {
		execution.Status = StatusCompensated
	}
	execution.CompensatedAt = &now
	execution.FinishedAt = now
	execution.mu.Unlock()

	o.logger.Info("saga compensation completed",
		"saga_id", execution.ID,
		"saga_name", saga.Name,
		"status", execution.Status,
	)

	if saga.OnCompensate != nil {
		saga.OnCompensate(ctx, execution.Clone())
	}
}

// Compensate triggers compensation for a running or completed saga.
func (o *Orchestrator) Compensate(ctx context.Context, executionID string, reason string) error {
	o.mu.RLock()
	execution, exists := o.executions[executionID]
	o.mu.RUnlock()

	if !exists {
		return fmt.Errorf("execution %q not found", executionID)
	}

	execution.mu.Lock()
	status := execution.Status
	lastCompletedStep := -1
	for i := range execution.Steps {
		if execution.Steps[i].Status == StatusCompleted {
			lastCompletedStep = i
		}
	}
	execution.mu.Unlock()

	if status == StatusCompensating || status == StatusCompensated {
		return errors.New("saga is already compensating or compensated")
	}

	o.mu.RLock()
	saga := o.sagas[execution.SagaName]
	o.mu.RUnlock()

	go o.compensateFrom(ctx, saga, execution, lastCompletedStep, errors.New(reason))
	return nil
}

// Get returns an execution by ID.
func (o *Orchestrator) Get(executionID string) *Execution {
	o.mu.RLock()
	defer o.mu.RUnlock()

	exec, exists := o.executions[executionID]
	if !exists {
		return nil
	}

	return exec.Clone()
}

// List returns all executions.
func (o *Orchestrator) List() []*Execution {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]*Execution, 0, len(o.executions))
	for _, exec := range o.executions {
		result = append(result, exec.Clone())
	}
	return result
}

// ListByStatus returns executions with the given status.
func (o *Orchestrator) ListByStatus(status Status) []*Execution {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var result []*Execution
	for _, exec := range o.executions {
		exec.mu.Lock()
		matches := exec.Status == status
		exec.mu.Unlock()

		if matches {
			result = append(result, exec.Clone())
		}
	}
	return result
}

// Remove removes an execution from tracking.
// Only completed, compensated, or failed sagas can be removed.
func (o *Orchestrator) Remove(executionID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	exec, exists := o.executions[executionID]
	if !exists {
		return fmt.Errorf("execution %q not found", executionID)
	}

	exec.mu.Lock()
	status := exec.Status
	exec.mu.Unlock()

	if status == StatusRunning || status == StatusCompensating {
		return errors.New("cannot remove running or compensating saga")
	}

	delete(o.executions, executionID)
	return nil
}

// GetRegistered returns a registered saga definition.
func (o *Orchestrator) GetRegistered(sagaName string) *Definition {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.sagas[sagaName]
}

// ListRegistered returns all registered saga names.
func (o *Orchestrator) ListRegistered() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	names := make([]string, 0, len(o.sagas))
	for name := range o.sagas {
		names = append(names, name)
	}
	return names
}
