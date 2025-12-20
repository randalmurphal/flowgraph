package expr

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Resolve resolves a value from variables or returns a literal.
// It handles quoted strings, booleans, null, numbers, and variable lookups.
func Resolve(s string, vars map[string]any) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Check for quoted string (single or double quotes)
	if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) {
		if len(s) < 2 {
			return ""
		}
		return s[1 : len(s)-1]
	}

	// Check for boolean literals
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	case "null", "nil":
		return nil
	}

	// Check for number (using json.Number for precise parsing)
	var num json.Number
	if err := json.Unmarshal([]byte(s), &num); err == nil {
		// Try integer first
		if i, err := num.Int64(); err == nil {
			return i
		}
		// Fall back to float
		if f, err := num.Float64(); err == nil {
			return f
		}
	}

	// Check for variable in vars map
	if vars != nil {
		if val, ok := vars[s]; ok {
			return val
		}
	}

	// Return as string literal (unquoted identifier not in vars)
	return s
}

// IsTruthy returns whether a value is truthy.
// nil is false, bools return their value, empty strings are false,
// zero numbers are false, everything else is true.
func IsTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	case int:
		return val != 0
	case int64:
		return val != 0
	case int32:
		return val != 0
	case float64:
		return val != 0
	case float32:
		return val != 0
	default:
		return true
	}
}

// ToFloat64 converts a value to float64 for numeric comparison.
// Returns 0 for values that cannot be converted.
func ToFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		var f float64
		_, _ = fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}
