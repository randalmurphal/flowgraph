// Package parser extracts structured content from LLM responses.
//
// Core types:
//   - Response: Contains structured data extracted from an LLM response
//   - CodeBlock: A fenced code block with language and content
//   - Parser: Extracts code blocks, JSON, YAML, sections, and lists
//
// Example usage:
//
//	p := parser.NewParser()
//	resp := p.Parse(llmOutput)
//
//	// Access extracted code blocks
//	for _, block := range resp.CodeBlocks {
//	    fmt.Printf("Language: %s\nCode:\n%s\n", block.Language, block.Content)
//	}
//
//	// Access parsed JSON
//	for _, data := range resp.JSONBlocks {
//	    fmt.Printf("JSON: %v\n", data)
//	}
//
// Convenience functions:
//
//	json := parser.ExtractJSON(response)
//	code := parser.ExtractCode(response, "go")
//	parsed := parser.Parse(response)
package parser
