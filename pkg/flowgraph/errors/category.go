// Package errors provides error handling, categorization, and recovery strategies.
//
// The package implements a layered error handling approach:
//   - Categorization: Classify errors for appropriate handling
//   - Retry: Handle transient failures with exponential backoff
//   - Escalation: Try stronger models when weaker ones fail
//   - Checkpointing: Save state before failures for recovery
package errors

import (
	"errors"
	"fmt"
)

// Category represents how an error should be handled.
type Category int

const (
	// CategoryTransient indicates retry will likely help.
	// Examples: rate limits, timeouts, temporary network issues.
	CategoryTransient Category = iota

	// CategoryPermanent indicates retry won't help.
	// Examples: authentication failures, invalid configuration.
	CategoryPermanent

	// CategoryEscalatable indicates a stronger model might succeed.
	// Examples: JSON parse failures, complex reasoning failures.
	CategoryEscalatable

	// CategoryHumanRequired indicates human intervention is needed.
	// Examples: ambiguous requirements, merge conflicts.
	CategoryHumanRequired
)

// String returns the category name.
func (c Category) String() string {
	switch c {
	case CategoryTransient:
		return "transient"
	case CategoryPermanent:
		return "permanent"
	case CategoryEscalatable:
		return "escalatable"
	case CategoryHumanRequired:
		return "human_required"
	default:
		return "unknown"
	}
}

// CategorizedError wraps an error with its category and context.
type CategorizedError struct {
	// Err is the underlying error.
	Err error

	// Category indicates how this error should be handled.
	Category Category

	// Retries is the number of attempts that have been made.
	Retries int

	// Context describes what operation was being attempted.
	Context string
}

// Error implements the error interface.
func (e *CategorizedError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("%s: %s (category: %s, attempts: %d)",
			e.Context, e.Err, e.Category, e.Retries)
	}
	return fmt.Sprintf("%s (category: %s, attempts: %d)",
		e.Err, e.Category, e.Retries)
}

// Unwrap returns the underlying error.
func (e *CategorizedError) Unwrap() error {
	return e.Err
}

// NewCategorized creates a new categorized error.
func NewCategorized(err error, category Category, context string) *CategorizedError {
	return &CategorizedError{
		Err:      err,
		Category: category,
		Context:  context,
	}
}

// Transient creates a transient error.
func Transient(err error, context string) *CategorizedError {
	return NewCategorized(err, CategoryTransient, context)
}

// Permanent creates a permanent error.
func Permanent(err error, context string) *CategorizedError {
	return NewCategorized(err, CategoryPermanent, context)
}

// Escalatable creates an escalatable error.
func Escalatable(err error, context string) *CategorizedError {
	return NewCategorized(err, CategoryEscalatable, context)
}

// HumanRequired creates a human-required error.
func HumanRequired(err error, context string) *CategorizedError {
	return NewCategorized(err, CategoryHumanRequired, context)
}

// Categorize determines how an error should be handled.
func Categorize(err error) Category {
	if err == nil {
		return CategoryPermanent // shouldn't happen, fail safe
	}

	// Check for already-categorized errors
	var catErr *CategorizedError
	if errors.As(err, &catErr) {
		return catErr.Category
	}

	// Check for human intervention errors
	var humanErr *HumanInterventionError
	if errors.As(err, &humanErr) {
		return CategoryHumanRequired
	}

	// Check for HTTP errors
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case 429, 503, 504:
			return CategoryTransient
		case 401, 403:
			return CategoryPermanent
		case 400:
			return CategoryEscalatable // bad request might be prompt issue
		default:
			if httpErr.StatusCode >= 500 {
				return CategoryTransient // server errors are often transient
			}
			return CategoryPermanent
		}
	}

	// Check for JSON parse errors
	var jsonErr *JSONParseError
	if errors.As(err, &jsonErr) {
		return CategoryEscalatable // better model might produce valid JSON
	}

	// Check for validation errors
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		return CategoryEscalatable // fix node can handle
	}

	// Check for timeout errors
	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return CategoryTransient
	}

	// Check for context errors (deadline exceeded, canceled)
	if errors.Is(err, errors.ErrUnsupported) {
		return CategoryPermanent
	}

	// Unknown errors are permanent (fail safe)
	return CategoryPermanent
}

// IsRetryable reports whether the error should be retried.
func IsRetryable(err error) bool {
	cat := Categorize(err)
	return cat == CategoryTransient
}

// IsEscalatable reports whether trying a stronger model might help.
func IsEscalatable(err error) bool {
	cat := Categorize(err)
	return cat == CategoryEscalatable
}

// NeedsHuman reports whether human intervention is required.
func NeedsHuman(err error) bool {
	cat := Categorize(err)
	return cat == CategoryHumanRequired
}
