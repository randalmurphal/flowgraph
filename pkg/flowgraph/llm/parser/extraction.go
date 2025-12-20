package parser

import (
	"encoding/json"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ExtractJSON extracts and parses the first JSON block found.
// Returns nil if no valid JSON block is found.
func (p *Parser) ExtractJSON(response string) map[string]any {
	blocks := p.extractJSONBlocks(response)
	if len(blocks) > 0 {
		return blocks[0]
	}
	return nil
}

// ExtractJSONArray extracts and parses JSON arrays from code blocks.
// Returns all successfully parsed arrays.
func (p *Parser) ExtractJSONArray(response string) []map[string]any {
	var results []map[string]any

	codeBlocks := p.extractCodeBlocks(response)
	for _, block := range codeBlocks {
		if block.Language == "json" || block.Language == "" {
			// Try parsing as array first
			var arr []map[string]any
			if err := json.Unmarshal([]byte(block.Content), &arr); err == nil {
				results = append(results, arr...)
				continue
			}
		}
	}

	// Also look for inline JSON arrays (outside of code blocks)
	textWithoutCodeBlocks := p.removeCodeBlocks(response)
	inlineArrayJSON := regexp.MustCompile(`(?s)^\[.*?\]$`)
	lines := strings.Split(textWithoutCodeBlocks, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if inlineArrayJSON.MatchString(line) {
			var arr []map[string]any
			if err := json.Unmarshal([]byte(line), &arr); err == nil {
				results = append(results, arr...)
			}
		}
	}

	return results
}

// ExtractCode extracts the first code block with the given language.
// If language is empty, returns the first code block found.
func (p *Parser) ExtractCode(response, language string) string {
	blocks := p.extractCodeBlocks(response)
	for _, block := range blocks {
		if language == "" || block.Language == language {
			return block.Content
		}
	}
	return ""
}

// ExtractAllCode extracts all code blocks from the response.
func (p *Parser) ExtractAllCode(response string) []CodeBlock {
	return p.extractCodeBlocks(response)
}

// ExtractYAML extracts and parses YAML blocks.
func (p *Parser) ExtractYAML(response string) []map[string]any {
	var blocks []map[string]any

	codeBlocks := p.extractCodeBlocks(response)
	for _, block := range codeBlocks {
		if block.Language == "yaml" || block.Language == "yml" {
			var data map[string]any
			if err := yaml.Unmarshal([]byte(block.Content), &data); err == nil {
				blocks = append(blocks, data)
			}
		}
	}

	return blocks
}

// ExtractSection extracts the content of a specific section by title.
func (p *Parser) ExtractSection(response, title string) string {
	sections := p.extractSections(response)

	// Try exact match first
	if content, ok := sections[title]; ok {
		return content
	}

	// Try case-insensitive match
	for sectionTitle, content := range sections {
		if strings.EqualFold(sectionTitle, title) {
			return content
		}
	}

	return ""
}

// ExtractList extracts list items from the response.
// Supports both - and * bullet points.
func (p *Parser) ExtractList(response string) []string {
	listRegex := regexp.MustCompile(`(?m)^\s*[-*]\s+(.+)$`)
	matches := listRegex.FindAllStringSubmatch(response, -1)

	items := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			items = append(items, strings.TrimSpace(match[1]))
		}
	}

	return items
}

// ExtractNumberedList extracts numbered list items.
func (p *Parser) ExtractNumberedList(response string) []string {
	listRegex := regexp.MustCompile(`(?m)^\s*\d+[.)]\s+(.+)$`)
	matches := listRegex.FindAllStringSubmatch(response, -1)

	items := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			items = append(items, strings.TrimSpace(match[1]))
		}
	}

	return items
}

// HasCodeBlock checks if the response contains any code block.
func (p *Parser) HasCodeBlock(response string) bool {
	return p.codeBlockRegex.MatchString(response)
}

// HasJSON checks if the response contains valid JSON.
func (p *Parser) HasJSON(response string) bool {
	return len(p.extractJSONBlocks(response)) > 0
}

// ExtractJSON is a convenience function for JSON extraction.
func ExtractJSON(response string) map[string]any {
	return NewParser().ExtractJSON(response)
}

// ExtractCode is a convenience function for code extraction.
func ExtractCode(response, language string) string {
	return NewParser().ExtractCode(response, language)
}
