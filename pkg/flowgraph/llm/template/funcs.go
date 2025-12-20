package template

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// defaultFuncs returns the built-in template functions.
func defaultFuncs() template.FuncMap {
	return template.FuncMap{
		"truncate":  truncate,
		"json":      toJSON,
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"trim":      strings.TrimSpace,
		"split":     strings.Split,
		"join":      strings.Join,
		"replace":   strings.ReplaceAll,
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"default":   defaultValue,
		"indent":    indent,
		"wrap":      wrap,
	}
}

// truncate cuts a string to the specified maximum length.
// If the string is longer than maxLen, it is truncated and "..." is appended.
// For maxLen <= 3, no ellipsis is added (the string is simply cut).
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// toJSON converts a value to a pretty-printed JSON string.
// If marshaling fails, returns the value's default string representation.
func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// defaultValue returns the default if the value is nil or an empty string.
// For other types (including zero values like 0), the original value is returned.
func defaultValue(val, defaultVal any) any {
	if val == nil {
		return defaultVal
	}
	if s, ok := val.(string); ok && s == "" {
		return defaultVal
	}
	return val
}

// indent adds a prefix string to each line of the input.
func indent(s string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

// wrap wraps text at the specified width, breaking on word boundaries.
// If width <= 0, the string is returned unchanged.
func wrap(s string, width int) string {
	if width <= 0 {
		return s
	}

	var result strings.Builder
	var lineLen int

	words := strings.Fields(s)
	for _, word := range words {
		if lineLen+len(word) > width && lineLen > 0 {
			result.WriteString("\n")
			lineLen = 0
		}
		if lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}
		result.WriteString(word)
		lineLen += len(word)
	}

	return result.String()
}
