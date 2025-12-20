package errors

import (
	"context"
	"math/rand/v2"
	"time"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including initial).
	MaxAttempts int

	// InitialBackoff is the starting backoff duration.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration.
	MaxBackoff time.Duration

	// BackoffFactor is the multiplier applied to backoff after each attempt.
	BackoffFactor float64

	// Jitter is the random jitter factor (0.0-1.0).
	Jitter float64

	// RetryableFunc optionally overrides the default retryability check.
	RetryableFunc func(error) bool
}

// DefaultRetry is the standard retry configuration.
var DefaultRetry = RetryConfig{
	MaxAttempts:    3,
	InitialBackoff: 1 * time.Second,
	MaxBackoff:     30 * time.Second,
	BackoffFactor:  2.0,
	Jitter:         0.1,
}

// AggressiveRetry retries more times with shorter backoff.
var AggressiveRetry = RetryConfig{
	MaxAttempts:    5,
	InitialBackoff: 500 * time.Millisecond,
	MaxBackoff:     10 * time.Second,
	BackoffFactor:  1.5,
	Jitter:         0.2,
}

// NoRetry disables retries.
var NoRetry = RetryConfig{
	MaxAttempts: 1,
}

// RetryResult contains the result of a retry operation.
type RetryResult[T any] struct {
	// Value is the result if successful.
	Value T

	// Err is the final error if all attempts failed.
	Err error

	// Attempts is the number of attempts made.
	Attempts int

	// Duration is the total time spent retrying.
	Duration time.Duration
}

// WithRetry executes a function with retries based on the configuration.
func WithRetry[T any](cfg RetryConfig, fn func() (T, error)) RetryResult[T] {
	return WithRetryContext(context.Background(), cfg, func(_ context.Context) (T, error) {
		return fn()
	})
}

// WithRetryContext executes a function with retries, respecting context cancellation.
func WithRetryContext[T any](
	ctx context.Context,
	cfg RetryConfig,
	fn func(context.Context) (T, error),
) RetryResult[T] {
	start := time.Now()
	backoff := cfg.InitialBackoff
	var lastErr error

	isRetryable := cfg.RetryableFunc
	if isRetryable == nil {
		isRetryable = func(err error) bool {
			cat := Categorize(err)
			return cat == CategoryTransient
		}
	}

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		// Check context before each attempt
		if err := ctx.Err(); err != nil {
			return RetryResult[T]{
				Err:      &CategorizedError{Err: err, Category: CategoryPermanent, Context: "context cancelled"},
				Attempts: attempt,
				Duration: time.Since(start),
			}
		}

		result, err := fn(ctx)
		if err == nil {
			return RetryResult[T]{
				Value:    result,
				Attempts: attempt + 1,
				Duration: time.Since(start),
			}
		}

		lastErr = err

		// Check if we should retry
		if !isRetryable(err) {
			return RetryResult[T]{
				Err: &CategorizedError{
					Err:      err,
					Category: Categorize(err),
					Retries:  attempt + 1,
				},
				Attempts: attempt + 1,
				Duration: time.Since(start),
			}
		}

		// Don't sleep after the last attempt
		if attempt < cfg.MaxAttempts-1 {
			sleepDuration := calculateBackoff(backoff, cfg.Jitter)
			select {
			case <-ctx.Done():
				return RetryResult[T]{
					Err:      &CategorizedError{Err: ctx.Err(), Category: CategoryPermanent, Context: "context cancelled during backoff"},
					Attempts: attempt + 1,
					Duration: time.Since(start),
				}
			case <-time.After(sleepDuration):
			}

			// Increase backoff for next attempt
			backoff = time.Duration(float64(backoff) * cfg.BackoffFactor)
			if backoff > cfg.MaxBackoff {
				backoff = cfg.MaxBackoff
			}
		}
	}

	return RetryResult[T]{
		Err: &CategorizedError{
			Err:      lastErr,
			Category: Categorize(lastErr),
			Retries:  cfg.MaxAttempts,
			Context:  "max retries exceeded",
		},
		Attempts: cfg.MaxAttempts,
		Duration: time.Since(start),
	}
}

// calculateBackoff returns the backoff duration with jitter applied.
func calculateBackoff(base time.Duration, jitter float64) time.Duration {
	if jitter <= 0 {
		return base
	}

	// Calculate jitter: base +/- (base * jitter * random)
	jitterAmount := float64(base) * jitter * (rand.Float64()*2 - 1)
	return time.Duration(float64(base) + jitterAmount)
}

// RetryOption configures retry behavior.
type RetryOption func(*RetryConfig)

// WithMaxAttempts sets the maximum number of attempts.
func WithMaxAttempts(n int) RetryOption {
	return func(cfg *RetryConfig) {
		cfg.MaxAttempts = n
	}
}

// WithInitialBackoff sets the initial backoff duration.
func WithInitialBackoff(d time.Duration) RetryOption {
	return func(cfg *RetryConfig) {
		cfg.InitialBackoff = d
	}
}

// WithMaxBackoff sets the maximum backoff duration.
func WithMaxBackoff(d time.Duration) RetryOption {
	return func(cfg *RetryConfig) {
		cfg.MaxBackoff = d
	}
}

// WithBackoffFactor sets the backoff multiplier.
func WithBackoffFactor(f float64) RetryOption {
	return func(cfg *RetryConfig) {
		cfg.BackoffFactor = f
	}
}

// WithJitter sets the jitter factor.
func WithJitter(j float64) RetryOption {
	return func(cfg *RetryConfig) {
		cfg.Jitter = j
	}
}

// WithRetryableFunc sets a custom retryability check.
func WithRetryableFunc(fn func(error) bool) RetryOption {
	return func(cfg *RetryConfig) {
		cfg.RetryableFunc = fn
	}
}

// NewRetryConfig creates a retry configuration with the given options.
func NewRetryConfig(opts ...RetryOption) RetryConfig {
	cfg := DefaultRetry
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
