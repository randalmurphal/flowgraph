# llm/parser

LLM response parser for extracting structured content from language model outputs.

## Purpose

Extracts code blocks, JSON, YAML, sections, and lists from LLM responses. Commonly used after receiving responses from Claude or other LLMs to extract actionable structured data.

## Key Types

| Type | Purpose |
|------|---------|
| `Parser` | Main parser with compiled regexes |
| `Response` | Parsed result containing all extracted content |
| `CodeBlock` | A single fenced code block with language and content |

## Common Patterns

```go
// Full parse with all extraction
p := parser.NewParser()
result := p.Parse(llmResponse)

// Access extracted content
for _, block := range result.CodeBlocks {
    if block.Language == "go" {
        // Process Go code
    }
}

// Quick JSON extraction
data := parser.ExtractJSON(response)
if data != nil {
    // Use parsed JSON
}

// Language-specific code extraction
goCode := parser.ExtractCode(response, "go")
pyCode := parser.ExtractCode(response, "python")
anyCode := parser.ExtractCode(response, "")  // First block

// YAML extraction
configs := p.ExtractYAML(response)

// List extraction
items := p.ExtractList(response)       // - or * bullets
steps := p.ExtractNumberedList(response)  // 1. or 1) numbered
```

## Response Structure

```go
type Response struct {
    Raw        string              // Original response
    Text       string              // Response with code blocks removed
    CodeBlocks []CodeBlock         // All fenced code blocks
    JSONBlocks []map[string]any    // All valid JSON blocks
    Sections   map[string]string   // Markdown sections by title
}
```

## Extraction Methods

| Method | Returns | Notes |
|--------|---------|-------|
| `ExtractJSON` | `map[string]any` | First valid JSON block |
| `ExtractJSONArray` | `[]map[string]any` | Items from JSON arrays |
| `ExtractCode(lang)` | `string` | First block matching language |
| `ExtractAllCode` | `[]CodeBlock` | All code blocks |
| `ExtractYAML` | `[]map[string]any` | Parsed YAML blocks |
| `ExtractSection(title)` | `string` | Section content by title |
| `ExtractList` | `[]string` | Bullet list items |
| `ExtractNumberedList` | `[]string` | Numbered list items |

## Convenience Functions

```go
// Package-level functions using default parser
parser.Parse(response)
parser.ExtractJSON(response)
parser.ExtractCode(response, "go")
```

## Dependencies

- `gopkg.in/yaml.v3` for YAML parsing
- Standard library `regexp` and `encoding/json`
