package template

import (
	"fmt"
	"strings"
	"text/template"
)

// Engine renders prompt templates with variable substitution.
// It supports both Go template syntax and Handlebars-like syntax.
type Engine struct {
	funcs template.FuncMap
}

// NewEngine creates a new template engine with default helper functions.
func NewEngine() *Engine {
	return &Engine{
		funcs: defaultFuncs(),
	}
}

// Render executes the template with the given variables.
// The template string supports Handlebars-like syntax which is automatically
// converted to Go template syntax before execution.
func (e *Engine) Render(templateStr string, variables map[string]any) (string, error) {
	if templateStr == "" {
		return "", ErrEmpty
	}

	// Convert Handlebars-like syntax to Go template syntax
	converted := convertSyntax(templateStr)

	tmpl, parseErr := template.New("prompt").Funcs(e.funcs).Parse(converted)
	if parseErr != nil {
		return "", fmt.Errorf("%w: %w", ErrParse, parseErr)
	}

	var buf strings.Builder
	if execErr := tmpl.Execute(&buf, variables); execErr != nil {
		return "", fmt.Errorf("%w: %w", ErrExecute, execErr)
	}

	return buf.String(), nil
}

// Parse validates the template and extracts variable names.
// Returns a list of variable names referenced in the template.
func (e *Engine) Parse(templateStr string) ([]string, error) {
	if templateStr == "" {
		return nil, ErrEmpty
	}

	converted := convertSyntax(templateStr)

	_, parseErr := template.New("prompt").Funcs(e.funcs).Parse(converted)
	if parseErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrParse, parseErr)
	}

	return extractVariables(templateStr), nil
}

// AddFunc adds a custom template function.
// The function will be available in templates using the given name.
func (e *Engine) AddFunc(name string, fn any) {
	e.funcs[name] = fn
}

// ValidateVariables checks that all required variables are provided.
// Returns an error wrapping ErrVariable if any required variable is missing.
func ValidateVariables(required []string, provided map[string]any) error {
	for _, name := range required {
		if _, ok := provided[name]; !ok {
			return fmt.Errorf("%w: %s", ErrVariable, name)
		}
	}
	return nil
}
