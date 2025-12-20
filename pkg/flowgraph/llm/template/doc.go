// Package template provides prompt template rendering with variable substitution.
//
// The template engine supports both Go template syntax and a simplified
// Handlebars-like syntax that is automatically converted before execution.
//
// # Syntax
//
// Simple variables use double braces:
//
//	Hello, {{name}}!
//
// Conditionals use #if and /if:
//
//	{{#if urgent}}URGENT: {{/if}}{{title}}
//
// Iteration uses #each and /each:
//
//	{{#each items}}{{.}} {{/each}}
//
// Helper functions can be called with arguments:
//
//	{{truncate description 100}}
//	{{upper name}}
//
// # Built-in Functions
//
//   - truncate(s string, maxLen int) string - Cut string to max length with ellipsis
//   - json(v any) string - Convert value to pretty-printed JSON
//   - upper(s string) string - Convert to uppercase
//   - lower(s string) string - Convert to lowercase
//   - trim(s string) string - Remove leading/trailing whitespace
//   - split(s, sep string) []string - Split string by separator
//   - join(slice []string, sep string) string - Join strings with separator
//   - replace(s, old, new string) string - Replace all occurrences
//   - contains(s, substr string) bool - Check if string contains substring
//   - hasPrefix(s, prefix string) bool - Check if string starts with prefix
//   - hasSuffix(s, suffix string) bool - Check if string ends with suffix
//   - default(val, defaultVal any) any - Return default if val is nil/empty
//   - indent(s string, spaces int) string - Add spaces to each line
//   - wrap(s string, width int) string - Wrap text at width
//
// # Example
//
//	engine := template.NewEngine()
//	result, err := engine.Render("Hello, {{name}}!", map[string]any{"name": "World"})
//	// result: "Hello, World!"
//
// # Variable Extraction
//
// The Parse method extracts variable names from a template:
//
//	vars, err := engine.Parse("{{greeting}}, {{name}}!")
//	// vars: ["greeting", "name"]
//
// # Custom Functions
//
// Add custom functions using AddFunc:
//
//	engine.AddFunc("double", func(s string) string { return s + s })
//	result, _ := engine.Render("{{double .name}}", map[string]any{"name": "ha"})
//	// result: "haha"
//
// Note: Custom functions use Go template syntax (.name) not Handlebars syntax.
//
// # Location
//
// This package is part of the flowgraph library:
//
//	import "github.com/randalmurphal/flowgraph/pkg/flowgraph/llm/template"
package template
