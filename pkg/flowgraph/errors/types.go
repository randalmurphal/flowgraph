package errors

import "fmt"

// HTTPError represents an HTTP error with status code.
type HTTPError struct {
	StatusCode int
	Message    string
	Endpoint   string
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	if e.Endpoint != "" {
		return fmt.Sprintf("HTTP %d at %s: %s", e.StatusCode, e.Endpoint, e.Message)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// JSONParseError indicates failure to parse JSON from LLM output.
type JSONParseError struct {
	Input   string
	Message string
}

// Error implements the error interface.
func (e *JSONParseError) Error() string {
	return fmt.Sprintf("JSON parse error: %s", e.Message)
}

// ValidationError indicates validation failures in LLM output.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// TimeoutError indicates an operation timed out.
type TimeoutError struct {
	Operation string
	Duration  string
}

// Error implements the error interface.
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout after %s: %s", e.Duration, e.Operation)
}

// HumanInterventionError indicates human input is required.
type HumanInterventionError struct {
	Question string
	Options  []string
	Original error
}

// Error implements the error interface.
func (e *HumanInterventionError) Error() string {
	return fmt.Sprintf("human intervention required: %s", e.Question)
}

// Unwrap returns the original error.
func (e *HumanInterventionError) Unwrap() error {
	return e.Original
}
