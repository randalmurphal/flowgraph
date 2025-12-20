package errors

import (
	"context"
	"log/slog"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/model"
)

// Handler coordinates error handling strategies.
type Handler struct {
	retry       RetryConfig
	escalation  *model.EscalationChain
	logger      *slog.Logger
	onEscalate  func(from, to model.ModelName, err error)
	onExhausted func(err error)
}

// HandlerOption configures a Handler.
type HandlerOption func(*Handler)

// NewHandler creates a new error handler with the given options.
func NewHandler(opts ...HandlerOption) *Handler {
	h := &Handler{
		retry:      DefaultRetry,
		escalation: &model.DefaultEscalation,
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// WithRetryConfig sets the retry configuration.
func WithRetryConfig(cfg RetryConfig) HandlerOption {
	return func(h *Handler) {
		h.retry = cfg
	}
}

// WithEscalation sets the escalation chain.
func WithEscalation(chain *model.EscalationChain) HandlerOption {
	return func(h *Handler) {
		h.escalation = chain
	}
}

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) HandlerOption {
	return func(h *Handler) {
		h.logger = logger
	}
}

// WithOnEscalate sets a callback for escalation events.
func WithOnEscalate(fn func(from, to model.ModelName, err error)) HandlerOption {
	return func(h *Handler) {
		h.onEscalate = fn
	}
}

// WithOnExhausted sets a callback for when all retries/escalations are exhausted.
func WithOnExhausted(fn func(err error)) HandlerOption {
	return func(h *Handler) {
		h.onExhausted = fn
	}
}

// ExecuteResult contains the result of a handled execution.
type ExecuteResult[T any] struct {
	// Value is the result if successful.
	Value T

	// Err is the error if failed.
	Err error

	// FinalModel is the model that was used for the successful attempt.
	FinalModel model.ModelName

	// Attempts is the total number of attempts made.
	Attempts int

	// Escalations is the number of times the model was escalated.
	Escalations int
}

// Execute runs a function with full error handling.
// It will retry transient errors and escalate the model for escalatable errors.
func (h *Handler) Execute(
	ctx context.Context,
	startModel model.ModelName,
	fn func(ctx context.Context, model model.ModelName) error,
) ExecuteResult[struct{}] {
	return Execute(ctx, h, startModel, func(ctx context.Context, m model.ModelName) (struct{}, error) {
		return struct{}{}, fn(ctx, m)
	})
}

// Execute runs a function with full error handling and returns a value.
func Execute[T any](
	ctx context.Context,
	h *Handler,
	startModel model.ModelName,
	fn func(ctx context.Context, m model.ModelName) (T, error),
) ExecuteResult[T] {
	currentModel := startModel
	totalAttempts := 0
	escalations := 0

	escState := model.NewEscalationState(h.escalation, startModel)

	for {
		// Run with retry for this model
		result := WithRetryContext(ctx, h.retry, func(ctx context.Context) (T, error) {
			return fn(ctx, currentModel)
		})

		totalAttempts += result.Attempts

		if result.Err == nil {
			return ExecuteResult[T]{
				Value:       result.Value,
				FinalModel:  currentModel,
				Attempts:    totalAttempts,
				Escalations: escalations,
			}
		}

		// Check if we should escalate
		category := Categorize(result.Err)

		switch category {
		case CategoryTransient:
			// Transient but retries exhausted - try escalating
			if !escState.RecordFailure(result.Err) {
				if h.onExhausted != nil {
					h.onExhausted(result.Err)
				}
				return ExecuteResult[T]{
					Err:         result.Err,
					FinalModel:  currentModel,
					Attempts:    totalAttempts,
					Escalations: escalations,
				}
			}

			if escState.CurrentModel != currentModel {
				oldModel := currentModel
				currentModel = escState.CurrentModel
				escalations++
				h.logger.Info("escalating model after transient failures",
					"from", oldModel,
					"to", currentModel,
					"error", result.Err,
				)
				if h.onEscalate != nil {
					h.onEscalate(oldModel, currentModel, result.Err)
				}
			}

		case CategoryEscalatable:
			// Try escalating to stronger model
			if !escState.RecordFailure(result.Err) {
				if h.onExhausted != nil {
					h.onExhausted(result.Err)
				}
				return ExecuteResult[T]{
					Err:         result.Err,
					FinalModel:  currentModel,
					Attempts:    totalAttempts,
					Escalations: escalations,
				}
			}

			if escState.CurrentModel != currentModel {
				oldModel := currentModel
				currentModel = escState.CurrentModel
				escalations++
				h.logger.Info("escalating model for escalatable error",
					"from", oldModel,
					"to", currentModel,
					"error", result.Err,
				)
				if h.onEscalate != nil {
					h.onEscalate(oldModel, currentModel, result.Err)
				}
			} else {
				// At highest model, can't escalate further
				if h.onExhausted != nil {
					h.onExhausted(result.Err)
				}
				return ExecuteResult[T]{
					Err:         result.Err,
					FinalModel:  currentModel,
					Attempts:    totalAttempts,
					Escalations: escalations,
				}
			}

		case CategoryHumanRequired:
			// Human intervention needed - no retry/escalation will help
			return ExecuteResult[T]{
				Err:         result.Err,
				FinalModel:  currentModel,
				Attempts:    totalAttempts,
				Escalations: escalations,
			}

		case CategoryPermanent:
			// Permanent error - no retry/escalation will help
			if h.onExhausted != nil {
				h.onExhausted(result.Err)
			}
			return ExecuteResult[T]{
				Err:         result.Err,
				FinalModel:  currentModel,
				Attempts:    totalAttempts,
				Escalations: escalations,
			}

		default:
			// Unknown category - treat as permanent
			return ExecuteResult[T]{
				Err:         result.Err,
				FinalModel:  currentModel,
				Attempts:    totalAttempts,
				Escalations: escalations,
			}
		}

		// Check if escalation state is exhausted
		if escState.Exhausted() {
			if h.onExhausted != nil {
				h.onExhausted(result.Err)
			}
			return ExecuteResult[T]{
				Err:         result.Err,
				FinalModel:  currentModel,
				Attempts:    totalAttempts,
				Escalations: escalations,
			}
		}
	}
}

// SimpleHandler provides simpler error handling without model escalation.
type SimpleHandler struct {
	retry  RetryConfig
	logger *slog.Logger
}

// NewSimpleHandler creates a handler that only retries transient errors.
func NewSimpleHandler(opts ...HandlerOption) *SimpleHandler {
	h := &Handler{
		retry:  DefaultRetry,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return &SimpleHandler{
		retry:  h.retry,
		logger: h.logger,
	}
}

// Execute runs a function with retry handling only.
func (h *SimpleHandler) Execute(
	ctx context.Context,
	fn func(ctx context.Context) error,
) error {
	result := WithRetryContext(ctx, h.retry, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return result.Err
}

// ExecuteWithValue runs a function with retry handling and returns a value.
func ExecuteWithValue[T any](
	ctx context.Context,
	h *SimpleHandler,
	fn func(ctx context.Context) (T, error),
) (T, error) {
	result := WithRetryContext(ctx, h.retry, fn)
	return result.Value, result.Err
}
