package template

import (
	"regexp"
	"strings"
)

// helperNames lists the built-in helper function names.
var helperNames = []string{
	"truncate", "json", "upper", "lower", "trim", "split", "join",
	"replace", "contains", "hasPrefix", "hasSuffix", "default", "indent", "wrap",
}

// goTemplateKeywords are Go template reserved words that should not be
// converted to variable references.
var goTemplateKeywords = map[string]bool{
	"else":     true,
	"end":      true,
	"if":       true,
	"range":    true,
	"with":     true,
	"define":   true,
	"template": true,
	"block":    true,
}

// convertSyntax converts Handlebars-like syntax to Go template syntax.
//
// Conversions:
//   - {{variable}} -> {{.variable}}
//   - {{#if x}}...{{/if}} -> {{if .x}}...{{end}}
//   - {{#each items}}...{{/each}} -> {{range .items}}...{{end}}
//   - {{helper arg1 arg2}} -> {{helper .arg1 .arg2}}
func convertSyntax(input string) string {
	result := input

	// Convert {{#if condition}} to {{if .condition}}
	ifPattern := regexp.MustCompile(`\{\{#if\s+(\w+)\}\}`)
	result = ifPattern.ReplaceAllString(result, "{{if .$1}}")

	// Convert {{/if}} to {{end}}
	result = strings.ReplaceAll(result, "{{/if}}", "{{end}}")

	// Convert {{#each items}} to {{range .items}}
	eachPattern := regexp.MustCompile(`\{\{#each\s+(\w+)\}\}`)
	result = eachPattern.ReplaceAllString(result, "{{range .$1}}")

	// Convert {{/each}} to {{end}}
	result = strings.ReplaceAll(result, "{{/each}}", "{{end}}")

	// Convert simple variables {{variable}} to {{.variable}}
	// Skip control structures (# or /) and Go template keywords
	varPattern := regexp.MustCompile(`\{\{([a-zA-Z_]\w*)\}\}`)
	result = varPattern.ReplaceAllStringFunc(result, func(match string) string {
		varName := match[2 : len(match)-2]
		if goTemplateKeywords[varName] {
			return match
		}
		return "{{." + varName + "}}"
	})

	// Convert helper function calls with arguments
	result = convertHelperCalls(result)

	return result
}

// convertHelperCalls converts helper function calls to Go template syntax.
// {{helper arg1 arg2}} -> {{helper .arg1 .arg2}}
func convertHelperCalls(input string) string {
	for _, helper := range helperNames {
		pattern := regexp.MustCompile(`\{\{` + helper + `\s+([^{}]+)\}\}`)
		input = pattern.ReplaceAllStringFunc(input, func(match string) string {
			argsStart := len("{{") + len(helper) + 1
			argsEnd := len(match) - 2
			args := strings.TrimSpace(match[argsStart:argsEnd])
			newArgs := convertArguments(args)
			return "{{" + helper + " " + newArgs + "}}"
		})
	}
	return input
}

// convertArguments converts a space-separated list of arguments.
// Variables become .variable, literals (numbers, quoted strings, booleans) stay as-is.
func convertArguments(args string) string {
	parts := splitArguments(args)
	for i, part := range parts {
		part = strings.TrimSpace(part)

		// Skip if already a Go template expression (starts with .)
		if strings.HasPrefix(part, ".") {
			continue
		}
		// Skip numbers
		if isNumber(part) {
			continue
		}
		// Skip quoted strings
		if isQuotedString(part) {
			continue
		}
		// Skip booleans
		if part == "true" || part == "false" {
			continue
		}
		// Otherwise, it's a variable - add dot prefix
		if isValidIdentifier(part) {
			parts[i] = "." + part
		}
	}
	return strings.Join(parts, " ")
}

// splitArguments splits arguments while respecting quoted strings.
func splitArguments(args string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range args {
		if !inQuote && (ch == '"' || ch == '\'') {
			inQuote = true
			quoteChar = ch
			current.WriteRune(ch)
		} else if inQuote && ch == quoteChar {
			inQuote = false
			current.WriteRune(ch)
		} else if !inQuote && ch == ' ' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// isNumber checks if a string represents a number (integer or float, optionally negative).
func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for i, ch := range s {
		if ch == '-' && i == 0 {
			continue
		}
		if ch == '.' {
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// isQuotedString checks if a string is wrapped in matching quotes.
func isQuotedString(s string) bool {
	if len(s) < 2 {
		return false
	}
	return (strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) ||
		(strings.HasPrefix(s, `'`) && strings.HasSuffix(s, `'`))
}

// isValidIdentifier checks if a string is a valid variable name.
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, ch := range s {
		// First character cannot be a digit
		if i == 0 && ch >= '0' && ch <= '9' {
			return false
		}
		// Must be alphanumeric or underscore
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}

// extractVariables extracts variable names from a template.
// Returns a deduplicated list of variable names found.
func extractVariables(templateStr string) []string {
	seen := make(map[string]bool)
	var result []string

	// Match {{variable}} patterns
	varPattern := regexp.MustCompile(`\{\{([a-zA-Z_]\w*)\}\}`)
	for _, match := range varPattern.FindAllStringSubmatch(templateStr, -1) {
		varName := match[1]
		if goTemplateKeywords[varName] {
			continue
		}
		if !seen[varName] {
			seen[varName] = true
			result = append(result, varName)
		}
	}

	// Match {{#if variable}} and {{#each variable}} patterns
	controlPattern := regexp.MustCompile(`\{\{#(?:if|each)\s+([a-zA-Z_]\w*)\}\}`)
	for _, match := range controlPattern.FindAllStringSubmatch(templateStr, -1) {
		varName := match[1]
		if !seen[varName] {
			seen[varName] = true
			result = append(result, varName)
		}
	}

	// Match variables in helper calls like {{truncate description 100}}
	helperPattern := regexp.MustCompile(`\{\{\w+\s+([a-zA-Z_]\w*)`)
	for _, match := range helperPattern.FindAllStringSubmatch(templateStr, -1) {
		varName := match[1]
		if !seen[varName] && isValidIdentifier(varName) && !isNumber(varName) {
			seen[varName] = true
			result = append(result, varName)
		}
	}

	return result
}
