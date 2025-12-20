// Package template provides prompt template rendering with variable substitution.
package template

import "errors"

// Sentinel errors for template operations.
var (
	// ErrEmpty is returned when the template string is empty.
	ErrEmpty = errors.New("template is empty")

	// ErrParse is returned when the template fails to parse.
	ErrParse = errors.New("template parse error")

	// ErrExecute is returned when template execution fails.
	ErrExecute = errors.New("template execution error")

	// ErrVariable is returned when a required variable is missing.
	ErrVariable = errors.New("required variable missing")
)
