package expr

import (
	"fmt"
	"strings"
)

// BinaryOp is a function that compares two values and returns a boolean result.
type BinaryOp func(left, right any) bool

// Evaluator evaluates boolean expressions with optional custom operators.
type Evaluator struct {
	customOps map[string]BinaryOp
}

// Option configures an Evaluator.
type Option func(*Evaluator)

// WithCustomOperator registers a custom binary operator.
// The operator name should not conflict with built-in operators.
func WithCustomOperator(name string, fn BinaryOp) Option {
	return func(e *Evaluator) {
		if e.customOps == nil {
			e.customOps = make(map[string]BinaryOp)
		}
		e.customOps[name] = fn
	}
}

// New creates a new Evaluator with the given options.
func New(opts ...Option) *Evaluator {
	e := &Evaluator{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Evaluate evaluates a boolean expression against the provided variables.
func (e *Evaluator) Evaluate(expr string, vars map[string]any) (bool, error) {
	return e.evaluateCondition(expr, vars)
}

// Eval is a convenience function that evaluates an expression using
// the default evaluator (no custom operators).
func Eval(expr string, vars map[string]any) (bool, error) {
	return New().Evaluate(expr, vars)
}

// evaluateCondition evaluates a condition expression.
// Supports: ==, !=, <, >, <=, >=, and, or, not, !, contains
func (e *Evaluator) evaluateCondition(expr string, vars map[string]any) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false, nil
	}

	// Handle negation with "not " prefix
	if strings.HasPrefix(expr, "not ") {
		inner := strings.TrimPrefix(expr, "not ")
		result, err := e.evaluateCondition(strings.TrimSpace(inner), vars)
		if err != nil {
			return false, err
		}
		return !result, nil
	}

	// Handle negation with "!" prefix
	if strings.HasPrefix(expr, "!") {
		inner := strings.TrimPrefix(expr, "!")
		result, err := e.evaluateCondition(strings.TrimSpace(inner), vars)
		if err != nil {
			return false, err
		}
		return !result, nil
	}

	// Handle AND (split on first " and ")
	if parts := strings.SplitN(expr, " and ", 2); len(parts) == 2 {
		left, errL := e.evaluateCondition(parts[0], vars)
		if errL != nil {
			return false, errL
		}
		right, errR := e.evaluateCondition(parts[1], vars)
		if errR != nil {
			return false, errR
		}
		return left && right, nil
	}

	// Handle OR (split on first " or ")
	if parts := strings.SplitN(expr, " or ", 2); len(parts) == 2 {
		left, errL := e.evaluateCondition(parts[0], vars)
		if errL != nil {
			return false, errL
		}
		right, errR := e.evaluateCondition(parts[1], vars)
		if errR != nil {
			return false, errR
		}
		return left || right, nil
	}

	// Define built-in operators in order (longer operators first to avoid partial matches)
	builtinOps := []struct {
		op      string
		compare BinaryOp
	}{
		{"==", func(l, r any) bool { return fmt.Sprintf("%v", l) == fmt.Sprintf("%v", r) }},
		{"!=", func(l, r any) bool { return fmt.Sprintf("%v", l) != fmt.Sprintf("%v", r) }},
		{">=", func(l, r any) bool { return ToFloat64(l) >= ToFloat64(r) }},
		{"<=", func(l, r any) bool { return ToFloat64(l) <= ToFloat64(r) }},
		{">", func(l, r any) bool { return ToFloat64(l) > ToFloat64(r) }},
		{"<", func(l, r any) bool { return ToFloat64(l) < ToFloat64(r) }},
		{" contains ", func(l, r any) bool {
			return strings.Contains(fmt.Sprintf("%v", l), fmt.Sprintf("%v", r))
		}},
	}

	// Try built-in operators
	for _, op := range builtinOps {
		if parts := strings.SplitN(expr, op.op, 2); len(parts) == 2 {
			left := Resolve(strings.TrimSpace(parts[0]), vars)
			right := Resolve(strings.TrimSpace(parts[1]), vars)
			return op.compare(left, right), nil
		}
	}

	// Try custom operators (wrap with spaces for word boundaries)
	for name, fn := range e.customOps {
		opPattern := " " + name + " "
		if parts := strings.SplitN(expr, opPattern, 2); len(parts) == 2 {
			left := Resolve(strings.TrimSpace(parts[0]), vars)
			right := Resolve(strings.TrimSpace(parts[1]), vars)
			return fn(left, right), nil
		}
	}

	// Single value - check if truthy
	val := Resolve(expr, vars)
	return IsTruthy(val), nil
}
