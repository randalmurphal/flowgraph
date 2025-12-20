package template

// MissingAction specifies how to handle missing variables.
type MissingAction int

const (
	// MissingKeep keeps the placeholder as-is when the variable is not found.
	// This is the default behavior.
	MissingKeep MissingAction = iota

	// MissingEmpty replaces the placeholder with an empty string when
	// the variable is not found.
	MissingEmpty

	// MissingError returns an error when a variable is not found.
	MissingError
)

// Option configures an Expander.
type Option func(*Expander)

// WithMissingAction sets how missing variables are handled.
//
// Default: MissingKeep (keep placeholder as-is)
//
// Example:
//
//	exp := NewExpander(WithMissingAction(MissingError))
//	_, err := exp.Expand("${missing}", nil)
//	// err: "undefined variable: missing"
func WithMissingAction(action MissingAction) Option {
	return func(e *Expander) {
		e.missingAction = action
	}
}

// WithBraceStyle enables or disables ${var} pattern expansion.
//
// Default: true (enabled)
//
// Example:
//
//	exp := NewExpander(WithBraceStyle(false))
//	result, _ := exp.Expand("${name}", map[string]any{"name": "World"})
//	// result: "${name}" (not expanded)
func WithBraceStyle(enabled bool) Option {
	return func(e *Expander) {
		e.braceStyle = enabled
	}
}

// WithDollarStyle enables or disables $var pattern expansion.
//
// Default: true (enabled)
//
// Example:
//
//	exp := NewExpander(WithDollarStyle(false))
//	result, _ := exp.Expand("$name", map[string]any{"name": "World"})
//	// result: "$name" (not expanded)
func WithDollarStyle(enabled bool) Option {
	return func(e *Expander) {
		e.dollarStyle = enabled
	}
}
