# llm/template

Prompt template engine with Handlebars-like syntax and Go template power.

## Quick Reference

### Syntax Conversion

| Handlebars | Go Template |
|------------|-------------|
| `{{variable}}` | `{{.variable}}` |
| `{{#if x}}...{{/if}}` | `{{if .x}}...{{end}}` |
| `{{#each items}}...{{/each}}` | `{{range .items}}...{{end}}` |
| `{{helper arg1 arg2}}` | `{{helper .arg1 .arg2}}` |

### Built-in Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `truncate` | `(s string, maxLen int) string` | Cut with ellipsis |
| `json` | `(v any) string` | Pretty-print JSON |
| `upper` | `(s string) string` | Uppercase |
| `lower` | `(s string) string` | Lowercase |
| `trim` | `(s string) string` | Strip whitespace |
| `split` | `(s, sep string) []string` | Split string |
| `join` | `(slice []string, sep string) string` | Join strings |
| `replace` | `(s, old, new string) string` | Replace all |
| `contains` | `(s, substr string) bool` | Check substring |
| `hasPrefix` | `(s, prefix string) bool` | Check prefix |
| `hasSuffix` | `(s, suffix string) bool` | Check suffix |
| `default` | `(val, defaultVal any) any` | Default for nil/empty |
| `indent` | `(s string, spaces int) string` | Indent lines |
| `wrap` | `(s string, width int) string` | Word wrap |

## Usage Patterns

### Basic Rendering

```go
import "github.com/randalmurphal/flowgraph/pkg/flowgraph/llm/template"

engine := template.NewEngine()
result, err := engine.Render("Hello, {{name}}!", map[string]any{"name": "World"})
```

### Variable Extraction

```go
vars, err := engine.Parse("{{greeting}}, {{name}}!")
// vars: ["greeting", "name"]
```

### Validate Before Render

```go
vars, _ := engine.Parse(tmpl)
if err := template.ValidateVariables(vars, provided); err != nil {
    // Handle missing variable
}
```

### Custom Functions

```go
engine.AddFunc("double", func(s string) string { return s + s })
// Use Go syntax for custom funcs: {{double .name}}
```

## Error Handling

| Error | Meaning |
|-------|---------|
| `ErrEmpty` | Template string is empty |
| `ErrParse` | Template syntax invalid |
| `ErrExecute` | Execution failed (missing data, function error) |
| `ErrVariable` | Required variable not provided |

All errors wrap the sentinel with details:

```go
if errors.Is(err, template.ErrParse) {
    // Handle parse error
}
```

## Gotchas

1. **Custom functions use Go syntax**: `{{myFunc .var}}` not `{{myFunc var}}`
2. **Missing variables render as `<no value>`**: Use `default` function or validate first
3. **Nested map access uses dots**: `{{.task.name}}` for `{"task": {"name": "Test"}}`
4. **Range context**: Inside `{{#each items}}`, use `{{.}}` for current item
