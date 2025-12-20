package template

import (
	"fmt"
	"regexp"
	"strings"
)

// Regular expressions for variable patterns.
var (
	// bracePattern matches ${varname} - varname can contain alphanumeric and underscore.
	bracePattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

	// dollarPattern matches $varname where varname is followed by a non-word character
	// or end of string. This prevents $port from matching inside $portNumber.
	dollarPattern = regexp.MustCompile(`\$([a-zA-Z_][a-zA-Z0-9_]*)(?:\b|$)`)
)

// Expander expands variable patterns in strings.
//
// Create with NewExpander() and configure with Option functions.
// Expander is safe for concurrent use after construction.
type Expander struct {
	missingAction MissingAction
	braceStyle    bool
	dollarStyle   bool
}

// NewExpander creates a new Expander with the given options.
//
// Default configuration:
//   - MissingAction: MissingKeep (keep placeholders as-is)
//   - BraceStyle: enabled (${var})
//   - DollarStyle: enabled ($var)
//
// Example:
//
//	exp := NewExpander(
//	    WithMissingAction(MissingError),
//	    WithBraceStyle(true),
//	    WithDollarStyle(false),
//	)
func NewExpander(opts ...Option) *Expander {
	e := &Expander{
		missingAction: MissingKeep,
		braceStyle:    true,
		dollarStyle:   true,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Expand expands variable patterns in s using the provided vars.
//
// Returns the expanded string and any error encountered.
// Errors are only returned when MissingAction is MissingError and
// a variable is not found.
//
// Example:
//
//	exp := NewExpander()
//	result, err := exp.Expand("Hello ${name}", map[string]any{"name": "World"})
//	// result: "Hello World"
func (e *Expander) Expand(s string, vars map[string]any) (string, error) {
	if s == "" {
		return "", nil
	}

	result := s
	var missingVars []string

	// Expand ${var} patterns first (more specific).
	if e.braceStyle {
		result = bracePattern.ReplaceAllStringFunc(result, func(match string) string {
			// Extract variable name from ${name}.
			varName := match[2 : len(match)-1]
			if val, ok := vars[varName]; ok {
				return fmt.Sprintf("%v", val)
			}
			// Variable not found.
			switch e.missingAction {
			case MissingEmpty:
				return ""
			case MissingError:
				missingVars = append(missingVars, varName)
				return match // Keep for now, will return error.
			default: // MissingKeep
				return match
			}
		})
	}

	// Expand $var patterns (less specific, after braces).
	if e.dollarStyle {
		result = dollarPattern.ReplaceAllStringFunc(result, func(match string) string {
			// Extract variable name from $name.
			varName := match[1:]
			if val, ok := vars[varName]; ok {
				return fmt.Sprintf("%v", val)
			}
			// Variable not found.
			switch e.missingAction {
			case MissingEmpty:
				return ""
			case MissingError:
				missingVars = append(missingVars, varName)
				return match // Keep for now, will return error.
			default: // MissingKeep
				return match
			}
		})
	}

	if len(missingVars) > 0 {
		return result, &UndefinedVariableError{Names: missingVars}
	}

	return result, nil
}

// MustExpand expands variable patterns in s and panics on error.
//
// Use this when you're certain all variables are present or when using
// MissingKeep/MissingEmpty which never return errors.
//
// Example:
//
//	result := exp.MustExpand("Hello ${name}", map[string]any{"name": "World"})
func (e *Expander) MustExpand(s string, vars map[string]any) string {
	result, err := e.Expand(s, vars)
	if err != nil {
		panic(fmt.Sprintf("template: %v", err))
	}
	return result
}

// ExpandAll expands variable patterns in all strings.
//
// Returns a new slice with expanded strings.
// Uses the expander's MissingAction for missing variables.
// On error (with MissingError), returns nil and the first error.
//
// Example:
//
//	exp := NewExpander()
//	results, _ := exp.ExpandAll([]string{"${a}", "${b}"}, vars)
func (e *Expander) ExpandAll(ss []string, vars map[string]any) ([]string, error) {
	if ss == nil {
		return nil, nil
	}

	results := make([]string, len(ss))
	for i, s := range ss {
		expanded, err := e.Expand(s, vars)
		if err != nil {
			return nil, err
		}
		results[i] = expanded
	}
	return results, nil
}

// ExpandMap expands variable patterns in all string values of a map recursively.
//
// Returns a new map with expanded values. Non-string values are copied as-is.
// Nested maps (map[string]any) are expanded recursively.
// On error (with MissingError), returns nil and the first error.
//
// Example:
//
//	exp := NewExpander()
//	result, _ := exp.ExpandMap(map[string]any{
//	    "url": "https://${host}/api",
//	    "port": 8080,  // Non-string, copied as-is.
//	}, vars)
func (e *Expander) ExpandMap(m map[string]any, vars map[string]any) (map[string]any, error) {
	if m == nil {
		return nil, nil
	}

	result := make(map[string]any, len(m))
	for k, v := range m {
		expanded, err := e.expandValue(v, vars)
		if err != nil {
			return nil, err
		}
		result[k] = expanded
	}
	return result, nil
}

// expandValue expands a single value, handling strings and nested maps.
func (e *Expander) expandValue(v any, vars map[string]any) (any, error) {
	switch val := v.(type) {
	case string:
		return e.Expand(val, vars)
	case map[string]any:
		return e.ExpandMap(val, vars)
	default:
		return v, nil
	}
}

// UndefinedVariableError is returned when MissingError is set and
// one or more variables are not found.
type UndefinedVariableError struct {
	// Names is the list of undefined variable names.
	Names []string
}

// Error implements the error interface.
func (e *UndefinedVariableError) Error() string {
	if len(e.Names) == 1 {
		return fmt.Sprintf("undefined variable: %s", e.Names[0])
	}
	return fmt.Sprintf("undefined variables: %s", strings.Join(e.Names, ", "))
}

// defaultExpander is the package-level expander with default settings.
var defaultExpander = NewExpander()

// Expand expands variable patterns in s using the default expander.
//
// Uses MissingKeep behavior (missing variables stay as-is).
//
// Example:
//
//	result := template.Expand("Hello ${name}", map[string]any{"name": "World"})
//	// result: "Hello World"
func Expand(s string, vars map[string]any) string {
	// Default expander never returns errors (MissingKeep).
	result, _ := defaultExpander.Expand(s, vars)
	return result
}

// ExpandAll expands variable patterns in all strings using the default expander.
//
// Uses MissingKeep behavior (missing variables stay as-is).
//
// Example:
//
//	results := template.ExpandAll([]string{"${a}", "${b}"}, vars)
func ExpandAll(ss []string, vars map[string]any) []string {
	// Default expander never returns errors (MissingKeep).
	results, _ := defaultExpander.ExpandAll(ss, vars)
	return results
}

// ExpandMap expands variable patterns in all string values using the default expander.
//
// Uses MissingKeep behavior (missing variables stay as-is).
// Nested maps are expanded recursively.
//
// Example:
//
//	result := template.ExpandMap(map[string]any{
//	    "url": "https://${host}/api",
//	}, vars)
func ExpandMap(m map[string]any, vars map[string]any) map[string]any {
	// Default expander never returns errors (MissingKeep).
	result, _ := defaultExpander.ExpandMap(m, vars)
	return result
}
