package expr

import (
	"regexp"
	"testing"
)

func TestEval_EqualityOperator(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		vars   map[string]any
		want   bool
		errMsg string
	}{
		{
			name: "string equality with quoted string",
			expr: "status == 'active'",
			vars: map[string]any{"status": "active"},
			want: true,
		},
		{
			name: "string equality with double quoted string",
			expr: `status == "active"`,
			vars: map[string]any{"status": "active"},
			want: true,
		},
		{
			name: "string equality false",
			expr: "status == 'inactive'",
			vars: map[string]any{"status": "active"},
			want: false,
		},
		{
			name: "number equality",
			expr: "count == 5",
			vars: map[string]any{"count": 5},
			want: true,
		},
		{
			name: "number equality false",
			expr: "count == 10",
			vars: map[string]any{"count": 5},
			want: false,
		},
		{
			name: "boolean equality true",
			expr: "enabled == true",
			vars: map[string]any{"enabled": true},
			want: true,
		},
		{
			name: "boolean equality false",
			expr: "enabled == false",
			vars: map[string]any{"enabled": true},
			want: false,
		},
		{
			name: "two variables equality",
			expr: "a == b",
			vars: map[string]any{"a": "test", "b": "test"},
			want: true,
		},
		{
			name: "two variables inequality",
			expr: "a == b",
			vars: map[string]any{"a": "test", "b": "other"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if tt.errMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEval_NotEqualOperator(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{
			name: "string not equal true",
			expr: "status != 'inactive'",
			vars: map[string]any{"status": "active"},
			want: true,
		},
		{
			name: "string not equal false",
			expr: "status != 'active'",
			vars: map[string]any{"status": "active"},
			want: false,
		},
		{
			name: "number not equal true",
			expr: "count != 10",
			vars: map[string]any{"count": 5},
			want: true,
		},
		{
			name: "number not equal false",
			expr: "count != 5",
			vars: map[string]any{"count": 5},
			want: false,
		},
		{
			name: "empty string not equal",
			expr: "name != ''",
			vars: map[string]any{"name": "test"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEval_NumericComparisonOperators(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		// Less than
		{
			name: "less than true",
			expr: "count < 10",
			vars: map[string]any{"count": 5},
			want: true,
		},
		{
			name: "less than false",
			expr: "count < 5",
			vars: map[string]any{"count": 5},
			want: false,
		},
		{
			name: "less than with float",
			expr: "price < 10.5",
			vars: map[string]any{"price": 9.99},
			want: true,
		},
		// Greater than
		{
			name: "greater than true",
			expr: "count > 3",
			vars: map[string]any{"count": 5},
			want: true,
		},
		{
			name: "greater than false",
			expr: "count > 5",
			vars: map[string]any{"count": 5},
			want: false,
		},
		{
			name: "greater than with float",
			expr: "price > 9.0",
			vars: map[string]any{"price": 9.99},
			want: true,
		},
		// Less than or equal
		{
			name: "less than or equal true (less)",
			expr: "count <= 10",
			vars: map[string]any{"count": 5},
			want: true,
		},
		{
			name: "less than or equal true (equal)",
			expr: "count <= 5",
			vars: map[string]any{"count": 5},
			want: true,
		},
		{
			name: "less than or equal false",
			expr: "count <= 4",
			vars: map[string]any{"count": 5},
			want: false,
		},
		// Greater than or equal
		{
			name: "greater than or equal true (greater)",
			expr: "count >= 3",
			vars: map[string]any{"count": 5},
			want: true,
		},
		{
			name: "greater than or equal true (equal)",
			expr: "count >= 5",
			vars: map[string]any{"count": 5},
			want: true,
		},
		{
			name: "greater than or equal false",
			expr: "count >= 6",
			vars: map[string]any{"count": 5},
			want: false,
		},
		// Negative numbers
		{
			name: "negative number comparison",
			expr: "temp > -10",
			vars: map[string]any{"temp": -5},
			want: true,
		},
		// Two variables
		{
			name: "two variables comparison",
			expr: "a > b",
			vars: map[string]any{"a": 10, "b": 5},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEval_ContainsOperator(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{
			name: "contains true",
			expr: "message contains 'error'",
			vars: map[string]any{"message": "an error occurred"},
			want: true,
		},
		{
			name: "contains false",
			expr: "message contains 'warning'",
			vars: map[string]any{"message": "an error occurred"},
			want: false,
		},
		{
			name: "contains empty string",
			expr: "message contains ''",
			vars: map[string]any{"message": "test"},
			want: true,
		},
		{
			name: "contains with variable",
			expr: "haystack contains needle",
			vars: map[string]any{"haystack": "hello world", "needle": "world"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEval_LogicalOperators(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		// AND
		{
			name: "and both true",
			expr: "enabled and active",
			vars: map[string]any{"enabled": true, "active": true},
			want: true,
		},
		{
			name: "and left false",
			expr: "enabled and active",
			vars: map[string]any{"enabled": false, "active": true},
			want: false,
		},
		{
			name: "and right false",
			expr: "enabled and active",
			vars: map[string]any{"enabled": true, "active": false},
			want: false,
		},
		{
			name: "and with comparison",
			expr: "status == 'ready' and count > 0",
			vars: map[string]any{"status": "ready", "count": 5},
			want: true,
		},
		{
			name: "and with comparison false",
			expr: "status == 'ready' and count > 10",
			vars: map[string]any{"status": "ready", "count": 5},
			want: false,
		},
		// OR
		{
			name: "or both true",
			expr: "enabled or override",
			vars: map[string]any{"enabled": true, "override": true},
			want: true,
		},
		{
			name: "or left true",
			expr: "enabled or override",
			vars: map[string]any{"enabled": true, "override": false},
			want: true,
		},
		{
			name: "or right true",
			expr: "enabled or override",
			vars: map[string]any{"enabled": false, "override": true},
			want: true,
		},
		{
			name: "or both false",
			expr: "enabled or override",
			vars: map[string]any{"enabled": false, "override": false},
			want: false,
		},
		// NOT
		{
			name: "not true",
			expr: "not disabled",
			vars: map[string]any{"disabled": false},
			want: true,
		},
		{
			name: "not false",
			expr: "not enabled",
			vars: map[string]any{"enabled": true},
			want: false,
		},
		{
			name: "not with comparison",
			expr: "not status == 'error'",
			vars: map[string]any{"status": "ok"},
			want: true,
		},
		// ! operator
		{
			name: "bang operator true",
			expr: "!cancelled",
			vars: map[string]any{"cancelled": false},
			want: true,
		},
		{
			name: "bang operator false",
			expr: "!active",
			vars: map[string]any{"active": true},
			want: false,
		},
		// Combined
		{
			name: "not and or combined",
			expr: "enabled and not disabled",
			vars: map[string]any{"enabled": true, "disabled": false},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEval_Truthiness(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{
			name: "true boolean",
			expr: "enabled",
			vars: map[string]any{"enabled": true},
			want: true,
		},
		{
			name: "false boolean",
			expr: "disabled",
			vars: map[string]any{"disabled": false},
			want: false,
		},
		{
			name: "non-empty string",
			expr: "name",
			vars: map[string]any{"name": "test"},
			want: true,
		},
		{
			name: "empty string",
			expr: "name",
			vars: map[string]any{"name": ""},
			want: false,
		},
		{
			name: "non-zero int",
			expr: "count",
			vars: map[string]any{"count": 5},
			want: true,
		},
		{
			name: "zero int",
			expr: "count",
			vars: map[string]any{"count": 0},
			want: false,
		},
		{
			name: "nil value",
			expr: "missing",
			vars: map[string]any{"missing": nil},
			want: false,
		},
		{
			name: "literal true",
			expr: "true",
			vars: nil,
			want: true,
		},
		{
			name: "literal false",
			expr: "false",
			vars: nil,
			want: false,
		},
		{
			name: "literal null",
			expr: "null",
			vars: nil,
			want: false,
		},
		{
			name: "literal nil",
			expr: "nil",
			vars: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEval_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{
			name: "empty expression",
			expr: "",
			vars: nil,
			want: false,
		},
		{
			name: "whitespace only",
			expr: "   ",
			vars: nil,
			want: false,
		},
		{
			name: "undefined variable is truthy as literal",
			expr: "undefined_var",
			vars: map[string]any{},
			want: true, // Returns the string "undefined_var" which is truthy
		},
		{
			name: "nil vars map",
			expr: "somevar",
			vars: nil,
			want: true, // Returns the string "somevar" which is truthy
		},
		{
			name: "number literal",
			expr: "42",
			vars: nil,
			want: true,
		},
		{
			name: "zero literal",
			expr: "0",
			vars: nil,
			want: false,
		},
		{
			name: "float literal",
			expr: "3.14",
			vars: nil,
			want: true,
		},
		{
			name: "quoted empty string",
			expr: "''",
			vars: nil,
			want: false,
		},
		{
			name: "single char quotes",
			expr: "'a'",
			vars: nil,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEvaluator_WithCustomOperator(t *testing.T) {
	// Custom "matches" operator using regex
	matchesOp := func(left, right any) bool {
		pattern, ok := right.(string)
		if !ok {
			pattern = right.(string)
		}
		value, ok := left.(string)
		if !ok {
			return false
		}
		matched, err := regexp.MatchString(pattern, value)
		return err == nil && matched
	}

	e := New(WithCustomOperator("matches", matchesOp))

	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{
			name: "matches prefix pattern",
			expr: "name matches '^test.*'",
			vars: map[string]any{"name": "test_123"},
			want: true,
		},
		{
			name: "matches suffix pattern",
			expr: "name matches '.*_suffix$'",
			vars: map[string]any{"name": "value_suffix"},
			want: true,
		},
		{
			name: "matches fails",
			expr: "name matches '^foo'",
			vars: map[string]any{"name": "bar"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Evaluate(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEvaluator_MultipleCustomOperators(t *testing.T) {
	startsWithOp := func(left, right any) bool {
		l, r := left.(string), right.(string)
		return len(l) >= len(r) && l[:len(r)] == r
	}
	endsWithOp := func(left, right any) bool {
		l, r := left.(string), right.(string)
		return len(l) >= len(r) && l[len(l)-len(r):] == r
	}

	e := New(
		WithCustomOperator("startswith", startsWithOp),
		WithCustomOperator("endswith", endsWithOp),
	)

	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{
			name: "startswith true",
			expr: "name startswith 'test'",
			vars: map[string]any{"name": "testing123"},
			want: true,
		},
		{
			name: "startswith false",
			expr: "name startswith 'foo'",
			vars: map[string]any{"name": "testing123"},
			want: false,
		},
		{
			name: "endswith true",
			expr: "name endswith '123'",
			vars: map[string]any{"name": "testing123"},
			want: true,
		},
		{
			name: "endswith false",
			expr: "name endswith 'foo'",
			vars: map[string]any{"name": "testing123"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Evaluate(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Evaluate(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEval_ComplexExpressions(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{
			name: "multiple and",
			expr: "a and b and c",
			vars: map[string]any{"a": true, "b": true, "c": true},
			want: true,
		},
		{
			name: "multiple or",
			expr: "a or b or c",
			vars: map[string]any{"a": false, "b": false, "c": true},
			want: true,
		},
		{
			name: "and has higher precedence (left associative)",
			expr: "a and b or c",
			vars: map[string]any{"a": true, "b": false, "c": true},
			want: true, // (a and b) or c = false or true = true
		},
		{
			name: "complex comparison chain",
			expr: "status == 'active' and count > 0 and enabled",
			vars: map[string]any{"status": "active", "count": 5, "enabled": true},
			want: true,
		},
		{
			name: "not with and",
			expr: "not error and success",
			vars: map[string]any{"error": false, "success": true},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name string
		s    string
		vars map[string]any
		want any
	}{
		{
			name: "single quoted string",
			s:    "'hello'",
			vars: nil,
			want: "hello",
		},
		{
			name: "double quoted string",
			s:    `"hello"`,
			vars: nil,
			want: "hello",
		},
		{
			name: "true boolean",
			s:    "true",
			vars: nil,
			want: true,
		},
		{
			name: "TRUE boolean (case insensitive)",
			s:    "TRUE",
			vars: nil,
			want: true,
		},
		{
			name: "false boolean",
			s:    "false",
			vars: nil,
			want: false,
		},
		{
			name: "null literal",
			s:    "null",
			vars: nil,
			want: nil,
		},
		{
			name: "nil literal",
			s:    "nil",
			vars: nil,
			want: nil,
		},
		{
			name: "integer",
			s:    "42",
			vars: nil,
			want: int64(42),
		},
		{
			name: "negative integer",
			s:    "-5",
			vars: nil,
			want: int64(-5),
		},
		{
			name: "float",
			s:    "3.14",
			vars: nil,
			want: 3.14,
		},
		{
			name: "variable from map",
			s:    "myvar",
			vars: map[string]any{"myvar": "value"},
			want: "value",
		},
		{
			name: "unknown identifier",
			s:    "unknown",
			vars: map[string]any{},
			want: "unknown",
		},
		{
			name: "empty string",
			s:    "",
			vars: nil,
			want: "",
		},
		{
			name: "whitespace trimmed",
			s:    "  hello  ",
			vars: map[string]any{"hello": "world"},
			want: "world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Resolve(tt.s, tt.vars)
			if got != tt.want {
				t.Errorf("Resolve(%q, %v) = %v (%T), want %v (%T)",
					tt.s, tt.vars, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want bool
	}{
		{"nil", nil, false},
		{"true", true, true},
		{"false", false, false},
		{"empty string", "", false},
		{"non-empty string", "hello", true},
		{"zero int", 0, false},
		{"positive int", 5, true},
		{"negative int", -1, true},
		{"zero int64", int64(0), false},
		{"positive int64", int64(5), true},
		{"zero int32", int32(0), false},
		{"positive int32", int32(5), true},
		{"zero float64", 0.0, false},
		{"positive float64", 3.14, true},
		{"zero float32", float32(0), false},
		{"positive float32", float32(3.14), true},
		{"slice (other type)", []int{1, 2, 3}, true},
		{"map (other type)", map[string]int{"a": 1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTruthy(tt.v)
			if got != tt.want {
				t.Errorf("IsTruthy(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want float64
	}{
		{"float64", 3.14, 3.14},
		{"float32", float32(2.5), 2.5},
		{"int", 42, 42.0},
		{"int64", int64(100), 100.0},
		{"int32", int32(50), 50.0},
		{"string number", "3.14", 3.14},
		{"string non-number", "hello", 0.0},
		{"nil", nil, 0.0},
		{"bool", true, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToFloat64(tt.v)
			if got != tt.want {
				t.Errorf("ToFloat64(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name    string
		left    any
		right   any
		op      string
		want    bool
		wantErr bool
	}{
		{"equals true", "hello", "hello", "==", true, false},
		{"equals false", "hello", "world", "==", false, false},
		{"not equals true", "hello", "world", "!=", true, false},
		{"not equals false", "hello", "hello", "!=", false, false},
		{"less than true", 5, 10, "<", true, false},
		{"less than false", 10, 5, "<", false, false},
		{"greater than true", 10, 5, ">", true, false},
		{"greater than false", 5, 10, ">", false, false},
		{"less or equal true (less)", 5, 10, "<=", true, false},
		{"less or equal true (equal)", 5, 5, "<=", true, false},
		{"less or equal false", 10, 5, "<=", false, false},
		{"greater or equal true (greater)", 10, 5, ">=", true, false},
		{"greater or equal true (equal)", 5, 5, ">=", true, false},
		{"greater or equal false", 5, 10, ">=", false, false},
		{"contains true", "hello world", "world", "contains", true, false},
		{"contains false", "hello world", "foo", "contains", false, false},
		{"unknown operator", 1, 2, "??", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compare(tt.left, tt.right, tt.op)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Compare(%v, %v, %q) = %v, want %v",
					tt.left, tt.right, tt.op, got, tt.want)
			}
		})
	}
}

func TestNew_DefaultEvaluator(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("New() returned nil")
	}

	result, err := e.Evaluate("5 > 3", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for 5 > 3")
	}
}

func TestEvaluator_EvaluateWithNestedNot(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{
			name: "double negation",
			expr: "not not enabled",
			vars: map[string]any{"enabled": true},
			want: true,
		},
		{
			name: "double bang",
			expr: "!!enabled",
			vars: map[string]any{"enabled": true},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}

func TestEval_VariableTypes(t *testing.T) {
	tests := []struct {
		name string
		expr string
		vars map[string]any
		want bool
	}{
		{
			name: "int64 variable",
			expr: "count > 5",
			vars: map[string]any{"count": int64(10)},
			want: true,
		},
		{
			name: "int32 variable",
			expr: "count > 5",
			vars: map[string]any{"count": int32(10)},
			want: true,
		},
		{
			name: "float32 variable",
			expr: "price > 5.0",
			vars: map[string]any{"price": float32(10.5)},
			want: true,
		},
		{
			name: "string numeric comparison",
			expr: "value > 5",
			vars: map[string]any{"value": "10"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.vars)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Eval(%q, %v) = %v, want %v", tt.expr, tt.vars, got, tt.want)
			}
		})
	}
}
