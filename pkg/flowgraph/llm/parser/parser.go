package parser

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

// Response contains structured data extracted from an LLM response.
type Response struct {
	// Raw is the original response text.
	Raw string

	// Text is the response with code blocks removed.
	Text string

	// CodeBlocks contains all extracted code blocks.
	CodeBlocks []CodeBlock

	// JSONBlocks contains parsed JSON blocks.
	JSONBlocks []map[string]any

	// Sections contains extracted markdown sections by title.
	Sections map[string]string
}

// CodeBlock represents a fenced code block.
type CodeBlock struct {
	// Language is the language specifier after the opening fence (e.g., "go", "python").
	Language string

	// Content is the code inside the block, excluding fences.
	Content string

	// Raw is the complete block including the fences.
	Raw string
}

// Parser extracts structured content from LLM responses.
type Parser struct {
	// codeBlockRegex matches fenced code blocks.
	codeBlockRegex *regexp.Regexp

	// jsonBlockRegex matches JSON code blocks specifically.
	jsonBlockRegex *regexp.Regexp

	// sectionRegex matches markdown headers.
	sectionRegex *regexp.Regexp
}

// NewParser creates a new response parser with compiled regexes.
func NewParser() *Parser {
	return &Parser{
		codeBlockRegex: regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```"),
		jsonBlockRegex: regexp.MustCompile("(?s)```(?:json)?\\n(\\{.*?\\})\\n?```"),
		sectionRegex:   regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`),
	}
}

// Parse extracts structured content from an LLM response.
func (p *Parser) Parse(response string) *Response {
	result := &Response{
		Raw:        response,
		CodeBlocks: p.extractCodeBlocks(response),
		JSONBlocks: p.extractJSONBlocks(response),
		Sections:   p.extractSections(response),
	}

	// Text is the response with code blocks removed
	result.Text = p.removeCodeBlocks(response)

	return result
}

// extractCodeBlocks finds all fenced code blocks in the response.
func (p *Parser) extractCodeBlocks(text string) []CodeBlock {
	matches := p.codeBlockRegex.FindAllStringSubmatch(text, -1)
	blocks := make([]CodeBlock, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 3 {
			blocks = append(blocks, CodeBlock{
				Language: match[1],
				Content:  match[2],
				Raw:      match[0],
			})
		}
	}

	return blocks
}

// extractJSONBlocks finds and parses JSON blocks.
func (p *Parser) extractJSONBlocks(text string) []map[string]any {
	var blocks []map[string]any

	// Try extracting from code blocks first
	codeBlocks := p.extractCodeBlocks(text)
	for _, block := range codeBlocks {
		if block.Language == "json" || block.Language == "" {
			var data map[string]any
			if err := json.Unmarshal([]byte(block.Content), &data); err == nil {
				blocks = append(blocks, data)
			}
		}
	}

	// Also look for inline JSON (objects starting with { on their own line)
	inlineJSON := regexp.MustCompile(`(?s)^\{.*?\}$`)
	lines := strings.Split(text, "\n")

	// Look for standalone JSON not in code blocks
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if inlineJSON.MatchString(line) {
			var data map[string]any
			if err := json.Unmarshal([]byte(line), &data); err == nil {
				// Check if this JSON wasn't already captured from code blocks
				isDuplicate := false
				for _, existing := range blocks {
					if jsonEqual(existing, data) {
						isDuplicate = true
						break
					}
				}
				if !isDuplicate {
					blocks = append(blocks, data)
				}
			}
		}
	}

	return blocks
}

// extractSections extracts markdown sections and their content.
func (p *Parser) extractSections(text string) map[string]string {
	sections := make(map[string]string)

	// Find all section headers and their positions
	matches := p.sectionRegex.FindAllStringSubmatchIndex(text, -1)

	for i, match := range matches {
		if len(match) < 6 {
			continue
		}

		headerEnd := match[1]
		titleStart := match[4]
		titleEnd := match[5]

		title := strings.TrimSpace(text[titleStart:titleEnd])

		// Content extends from after this header to the next header (or end)
		contentStart := headerEnd
		contentEnd := len(text)
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		}

		content := strings.TrimSpace(text[contentStart:contentEnd])
		sections[title] = content
	}

	return sections
}

// removeCodeBlocks removes all code blocks from the text.
func (p *Parser) removeCodeBlocks(text string) string {
	return p.codeBlockRegex.ReplaceAllString(text, "")
}

// jsonEqual compares two JSON maps for equality.
func jsonEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}

	aJSON, errA := json.Marshal(a)
	bJSON, errB := json.Marshal(b)

	if errA != nil || errB != nil {
		return false
	}

	return bytes.Equal(aJSON, bJSON)
}

// Parse is a convenience function using the default parser.
func Parse(response string) *Response {
	return NewParser().Parse(response)
}
