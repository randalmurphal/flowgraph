package errors

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/model"
)

// discardLogger returns a logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCategoryString(t *testing.T) {
	tests := []struct {
		category Category
		expected string
	}{
		{CategoryTransient, "transient"},
		{CategoryPermanent, "permanent"},
		{CategoryEscalatable, "escalatable"},
		{CategoryHumanRequired, "human_required"},
		{Category(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.String(); got != tt.expected {
				t.Errorf("Category(%d).String() = %s, want %s", tt.category, got, tt.expected)
			}
		})
	}
}

func TestCategorize(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected Category
	}{
		{"nil error", nil, CategoryPermanent},
		{"HTTP 429", &HTTPError{StatusCode: 429}, CategoryTransient},
		{"HTTP 503", &HTTPError{StatusCode: 503}, CategoryTransient},
		{"HTTP 504", &HTTPError{StatusCode: 504}, CategoryTransient},
		{"HTTP 500", &HTTPError{StatusCode: 500}, CategoryTransient},
		{"HTTP 401", &HTTPError{StatusCode: 401}, CategoryPermanent},
		{"HTTP 403", &HTTPError{StatusCode: 403}, CategoryPermanent},
		{"HTTP 400", &HTTPError{StatusCode: 400}, CategoryEscalatable},
		{"HTTP 404", &HTTPError{StatusCode: 404}, CategoryPermanent},
		{"JSON parse error", &JSONParseError{Message: "unexpected token"}, CategoryEscalatable},
		{"Validation error", &ValidationError{Message: "missing field"}, CategoryEscalatable},
		{"Timeout error", &TimeoutError{Operation: "api call", Duration: "30s"}, CategoryTransient},
		{"Human intervention", &HumanInterventionError{Question: "what do?"}, CategoryHumanRequired},
		{"Categorized error", &CategorizedError{Category: CategoryTransient}, CategoryTransient},
		{"Unknown error", errors.New("unknown"), CategoryPermanent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Categorize(tt.err); got != tt.expected {
				t.Errorf("Categorize() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestCategorizedError(t *testing.T) {
	t.Run("error message with context", func(t *testing.T) {
		err := NewCategorized(errors.New("failed"), CategoryTransient, "api call")
		expected := "api call: failed (category: transient, attempts: 0)"
		if got := err.Error(); got != expected {
			t.Errorf("Error() = %q, want %q", got, expected)
		}
	})

	t.Run("error message without context", func(t *testing.T) {
		err := &CategorizedError{Err: errors.New("failed"), Category: CategoryTransient}
		if got := err.Error(); got != "failed (category: transient, attempts: 0)" {
			t.Errorf("Error() = %q", got)
		}
	})

	t.Run("unwrap", func(t *testing.T) {
		inner := errors.New("inner error")
		err := NewCategorized(inner, CategoryPermanent, "test")
		if !errors.Is(err, inner) {
			t.Error("Unwrap should return inner error")
		}
	})
}

func TestErrorConstructors(t *testing.T) {
	inner := errors.New("test error")

	t.Run("Transient", func(t *testing.T) {
		err := Transient(inner, "context")
		if err.Category != CategoryTransient {
			t.Errorf("Category = %s, want transient", err.Category)
		}
	})

	t.Run("Permanent", func(t *testing.T) {
		err := Permanent(inner, "context")
		if err.Category != CategoryPermanent {
			t.Errorf("Category = %s, want permanent", err.Category)
		}
	})

	t.Run("Escalatable", func(t *testing.T) {
		err := Escalatable(inner, "context")
		if err.Category != CategoryEscalatable {
			t.Errorf("Category = %s, want escalatable", err.Category)
		}
	})

	t.Run("HumanRequired", func(t *testing.T) {
		err := HumanRequired(inner, "context")
		if err.Category != CategoryHumanRequired {
			t.Errorf("Category = %s, want human_required", err.Category)
		}
	})
}

func TestHTTPError(t *testing.T) {
	t.Run("with endpoint", func(t *testing.T) {
		err := &HTTPError{StatusCode: 500, Message: "internal error", Endpoint: "/api/foo"}
		expected := "HTTP 500 at /api/foo: internal error"
		if got := err.Error(); got != expected {
			t.Errorf("Error() = %q, want %q", got, expected)
		}
	})

	t.Run("without endpoint", func(t *testing.T) {
		err := &HTTPError{StatusCode: 404, Message: "not found"}
		expected := "HTTP 404: not found"
		if got := err.Error(); got != expected {
			t.Errorf("Error() = %q, want %q", got, expected)
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	transient := &HTTPError{StatusCode: 429}
	escalatable := &JSONParseError{Message: "bad json"}
	human := &HumanInterventionError{Question: "help"}
	permanent := &HTTPError{StatusCode: 404}

	t.Run("IsRetryable", func(t *testing.T) {
		if !IsRetryable(transient) {
			t.Error("429 should be retryable")
		}
		if IsRetryable(permanent) {
			t.Error("404 should not be retryable")
		}
	})

	t.Run("IsEscalatable", func(t *testing.T) {
		if !IsEscalatable(escalatable) {
			t.Error("JSON parse error should be escalatable")
		}
		if IsEscalatable(permanent) {
			t.Error("404 should not be escalatable")
		}
	})

	t.Run("NeedsHuman", func(t *testing.T) {
		if !NeedsHuman(human) {
			t.Error("Human intervention error should need human")
		}
		if NeedsHuman(permanent) {
			t.Error("404 should not need human")
		}
	})
}

func TestWithRetry(t *testing.T) {
	t.Run("success on first try", func(t *testing.T) {
		calls := 0
		cfg := NewRetryConfig(WithMaxAttempts(3))
		result := WithRetry(cfg, func() (string, error) {
			calls++
			return "success", nil
		})

		if result.Err != nil {
			t.Errorf("Unexpected error: %v", result.Err)
		}
		if result.Value != "success" {
			t.Errorf("Value = %q, want %q", result.Value, "success")
		}
		if result.Attempts != 1 {
			t.Errorf("Attempts = %d, want 1", result.Attempts)
		}
		if calls != 1 {
			t.Errorf("Calls = %d, want 1", calls)
		}
	})

	t.Run("success on retry", func(t *testing.T) {
		calls := 0
		cfg := NewRetryConfig(
			WithMaxAttempts(3),
			WithInitialBackoff(1*time.Millisecond),
		)
		result := WithRetry(cfg, func() (string, error) {
			calls++
			if calls < 2 {
				return "", &HTTPError{StatusCode: 503} // transient
			}
			return "success", nil
		})

		if result.Err != nil {
			t.Errorf("Unexpected error: %v", result.Err)
		}
		if result.Attempts != 2 {
			t.Errorf("Attempts = %d, want 2", result.Attempts)
		}
	})

	t.Run("max attempts exceeded", func(t *testing.T) {
		cfg := NewRetryConfig(
			WithMaxAttempts(3),
			WithInitialBackoff(1*time.Millisecond),
		)
		result := WithRetry(cfg, func() (string, error) {
			return "", &HTTPError{StatusCode: 503}
		})

		if result.Err == nil {
			t.Error("Expected error after max attempts")
		}
		if result.Attempts != 3 {
			t.Errorf("Attempts = %d, want 3", result.Attempts)
		}
	})

	t.Run("non-retryable error stops immediately", func(t *testing.T) {
		calls := 0
		cfg := NewRetryConfig(WithMaxAttempts(3))
		result := WithRetry(cfg, func() (string, error) {
			calls++
			return "", &HTTPError{StatusCode: 404} // permanent
		})

		if result.Err == nil {
			t.Error("Expected error")
		}
		if calls != 1 {
			t.Errorf("Calls = %d, want 1 (should not retry permanent error)", calls)
		}
	})

	t.Run("custom retryable func", func(t *testing.T) {
		calls := 0
		cfg := NewRetryConfig(
			WithMaxAttempts(3),
			WithInitialBackoff(1*time.Millisecond),
			WithRetryableFunc(func(_ error) bool { return true }), // retry everything
		)
		result := WithRetry(cfg, func() (string, error) {
			calls++
			return "", &HTTPError{StatusCode: 404}
		})

		if calls != 3 {
			t.Errorf("Calls = %d, want 3 (custom func should retry)", calls)
		}
		if result.Attempts != 3 {
			t.Errorf("Attempts = %d, want 3", result.Attempts)
		}
	})
}

func TestWithRetryContext(t *testing.T) {
	t.Run("respects context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		cfg := NewRetryConfig(WithMaxAttempts(3))
		result := WithRetryContext(ctx, cfg, func(_ context.Context) (string, error) {
			return "never reached", nil
		})

		if result.Err == nil {
			t.Error("Expected error from cancelled context")
		}
	})

	t.Run("cancellation during backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0

		cfg := NewRetryConfig(
			WithMaxAttempts(5),
			WithInitialBackoff(100*time.Millisecond),
		)

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		result := WithRetryContext(ctx, cfg, func(_ context.Context) (string, error) {
			calls++
			return "", &HTTPError{StatusCode: 503}
		})

		if result.Err == nil {
			t.Error("Expected error from cancelled context")
		}
		if calls > 2 {
			t.Errorf("Calls = %d, expected <= 2 (should cancel during backoff)", calls)
		}
	})
}

func TestHandler(t *testing.T) {
	// Use a logger that discards output
	logger := discardLogger()

	t.Run("success on first try", func(t *testing.T) {
		h := NewHandler(
			WithLogger(logger),
			WithRetryConfig(NoRetry),
		)

		result := h.Execute(context.Background(), model.ModelSonnet, func(_ context.Context, m model.ModelName) error {
			if m != model.ModelSonnet {
				t.Errorf("Model = %s, want %s", m, model.ModelSonnet)
			}
			return nil
		})

		if result.Err != nil {
			t.Errorf("Unexpected error: %v", result.Err)
		}
		if result.FinalModel != model.ModelSonnet {
			t.Errorf("FinalModel = %s, want %s", result.FinalModel, model.ModelSonnet)
		}
		if result.Escalations != 0 {
			t.Errorf("Escalations = %d, want 0", result.Escalations)
		}
	})

	t.Run("escalates on escalatable error", func(t *testing.T) {
		h := NewHandler(
			WithLogger(logger),
			WithRetryConfig(NoRetry),
			WithEscalation(&model.EscalationChain{
				Models:      []model.ModelName{model.ModelSonnet, model.ModelOpus},
				MaxAttempts: 2,
			}),
		)

		calls := 0
		result := h.Execute(context.Background(), model.ModelSonnet, func(_ context.Context, m model.ModelName) error {
			calls++
			if m == model.ModelSonnet {
				return &JSONParseError{Message: "bad json"}
			}
			return nil // success with opus
		})

		if result.Err != nil {
			t.Errorf("Unexpected error: %v", result.Err)
		}
		if result.FinalModel != model.ModelOpus {
			t.Errorf("FinalModel = %s, want %s", result.FinalModel, model.ModelOpus)
		}
		if result.Escalations != 1 {
			t.Errorf("Escalations = %d, want 1", result.Escalations)
		}
	})

	t.Run("permanent error stops immediately", func(t *testing.T) {
		h := NewHandler(
			WithLogger(logger),
			WithRetryConfig(NoRetry),
		)

		calls := 0
		result := h.Execute(context.Background(), model.ModelSonnet, func(_ context.Context, _ model.ModelName) error {
			calls++
			return &HTTPError{StatusCode: 401}
		})

		if result.Err == nil {
			t.Error("Expected permanent error")
		}
		if calls != 1 {
			t.Errorf("Calls = %d, want 1", calls)
		}
	})

	t.Run("human required error stops", func(t *testing.T) {
		h := NewHandler(
			WithLogger(logger),
			WithRetryConfig(NoRetry),
		)

		result := h.Execute(context.Background(), model.ModelSonnet, func(_ context.Context, _ model.ModelName) error {
			return &HumanInterventionError{Question: "what should I do?"}
		})

		if result.Err == nil {
			t.Error("Expected human required error")
		}
		if !NeedsHuman(result.Err) {
			t.Error("Error should need human intervention")
		}
	})

	t.Run("onEscalate callback", func(t *testing.T) {
		escalateCalled := false
		var fromModel, toModel model.ModelName

		h := NewHandler(
			WithLogger(logger),
			WithRetryConfig(NoRetry),
			WithEscalation(&model.EscalationChain{
				Models:      []model.ModelName{model.ModelSonnet, model.ModelOpus},
				MaxAttempts: 2,
			}),
			WithOnEscalate(func(from, to model.ModelName, _ error) {
				escalateCalled = true
				fromModel = from
				toModel = to
			}),
		)

		calls := 0
		h.Execute(context.Background(), model.ModelSonnet, func(_ context.Context, m model.ModelName) error {
			calls++
			if m == model.ModelSonnet {
				return &JSONParseError{Message: "bad json"}
			}
			return nil
		})

		if !escalateCalled {
			t.Error("onEscalate callback not called")
		}
		if fromModel != model.ModelSonnet || toModel != model.ModelOpus {
			t.Errorf("Escalated from %s to %s, want sonnet to opus", fromModel, toModel)
		}
	})

	t.Run("onExhausted callback", func(t *testing.T) {
		exhaustedCalled := false

		h := NewHandler(
			WithLogger(logger),
			WithRetryConfig(NoRetry),
			WithEscalation(&model.EscalationChain{
				Models:      []model.ModelName{model.ModelSonnet},
				MaxAttempts: 1,
			}),
			WithOnExhausted(func(_ error) {
				exhaustedCalled = true
			}),
		)

		h.Execute(context.Background(), model.ModelSonnet, func(_ context.Context, _ model.ModelName) error {
			return &JSONParseError{Message: "bad json"}
		})

		if !exhaustedCalled {
			t.Error("onExhausted callback not called")
		}
	})
}

func TestExecuteWithValue(t *testing.T) {
	h := NewHandler(
		WithLogger(discardLogger()),
		WithRetryConfig(NoRetry),
	)

	result := Execute(context.Background(), h, model.ModelSonnet, func(_ context.Context, _ model.ModelName) (string, error) {
		return "result value", nil
	})

	if result.Err != nil {
		t.Errorf("Unexpected error: %v", result.Err)
	}
	if result.Value != "result value" {
		t.Errorf("Value = %q, want %q", result.Value, "result value")
	}
}

func TestSimpleHandler(t *testing.T) {
	t.Run("Execute", func(t *testing.T) {
		h := NewSimpleHandler(
			WithRetryConfig(NewRetryConfig(
				WithMaxAttempts(3),
				WithInitialBackoff(1*time.Millisecond),
			)),
		)

		calls := 0
		err := h.Execute(context.Background(), func(_ context.Context) error {
			calls++
			if calls < 2 {
				return &HTTPError{StatusCode: 503}
			}
			return nil
		})

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if calls != 2 {
			t.Errorf("Calls = %d, want 2", calls)
		}
	})

	t.Run("ExecuteWithValue", func(t *testing.T) {
		h := NewSimpleHandler(WithRetryConfig(NoRetry))

		result, err := ExecuteWithValue(context.Background(), h, func(_ context.Context) (int, error) {
			return 42, nil
		})

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result != 42 {
			t.Errorf("Result = %d, want 42", result)
		}
	})
}

func TestNewRetryConfig(t *testing.T) {
	cfg := NewRetryConfig(
		WithMaxAttempts(5),
		WithInitialBackoff(2*time.Second),
		WithMaxBackoff(60*time.Second),
		WithBackoffFactor(3.0),
		WithJitter(0.2),
	)

	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", cfg.MaxAttempts)
	}
	if cfg.InitialBackoff != 2*time.Second {
		t.Errorf("InitialBackoff = %v, want 2s", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 60*time.Second {
		t.Errorf("MaxBackoff = %v, want 60s", cfg.MaxBackoff)
	}
	if cfg.BackoffFactor != 3.0 {
		t.Errorf("BackoffFactor = %f, want 3.0", cfg.BackoffFactor)
	}
	if cfg.Jitter != 0.2 {
		t.Errorf("Jitter = %f, want 0.2", cfg.Jitter)
	}
}
