package parser

import (
	"reflect"
	"testing"
)

// =============================================================================
// Parser Creation Tests
// =============================================================================

func TestNewParser(t *testing.T) {
	p := NewParser()

	if p == nil {
		t.Fatal("NewParser() returned nil")
	}
	if p.codeBlockRegex == nil {
		t.Error("codeBlockRegex not initialized")
	}
	if p.jsonBlockRegex == nil {
		t.Error("jsonBlockRegex not initialized")
	}
	if p.sectionRegex == nil {
		t.Error("sectionRegex not initialized")
	}
}

// =============================================================================
// Code Block Extraction Tests
// =============================================================================

func TestExtractCode_SingleBlock(t *testing.T) {
	response := `Here is some Go code:

` + "```go\n" + `func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

That's all!`

	p := NewParser()
	code := p.ExtractCode(response, "go")

	expected := `func main() {
    fmt.Println("Hello, World!")
}
`
	if code != expected {
		t.Errorf("ExtractCode() = %q, want %q", code, expected)
	}
}

func TestExtractCode_MultipleLanguages(t *testing.T) {
	response := "```python\nprint('hello')\n```\n\n```go\nfmt.Println(\"hello\")\n```"

	p := NewParser()

	pyCode := p.ExtractCode(response, "python")
	if pyCode != "print('hello')\n" {
		t.Errorf("Python code = %q", pyCode)
	}

	goCode := p.ExtractCode(response, "go")
	if goCode != "fmt.Println(\"hello\")\n" {
		t.Errorf("Go code = %q", goCode)
	}
}

func TestExtractCode_NoLanguage(t *testing.T) {
	response := "```\nsome code\n```"

	p := NewParser()
	code := p.ExtractCode(response, "")

	if code != "some code\n" {
		t.Errorf("ExtractCode() = %q, want 'some code\\n'", code)
	}
}

func TestExtractCode_NotFound(t *testing.T) {
	response := "No code blocks here"

	p := NewParser()
	code := p.ExtractCode(response, "go")

	if code != "" {
		t.Errorf("ExtractCode() = %q, want empty string", code)
	}
}

func TestExtractCode_LanguageMismatch(t *testing.T) {
	response := "```python\nprint('hello')\n```"

	p := NewParser()
	code := p.ExtractCode(response, "go")

	if code != "" {
		t.Errorf("ExtractCode() = %q, want empty string for language mismatch", code)
	}
}

func TestExtractAllCode(t *testing.T) {
	response := "```go\nfunc a() {}\n```\n\n```python\ndef b(): pass\n```"

	p := NewParser()
	blocks := p.ExtractAllCode(response)

	if len(blocks) != 2 {
		t.Fatalf("ExtractAllCode() returned %d blocks, want 2", len(blocks))
	}

	if blocks[0].Language != "go" {
		t.Errorf("First block language = %q, want 'go'", blocks[0].Language)
	}
	if blocks[1].Language != "python" {
		t.Errorf("Second block language = %q, want 'python'", blocks[1].Language)
	}
}

func TestExtractAllCode_WithRaw(t *testing.T) {
	response := "```js\nconsole.log('test');\n```"

	p := NewParser()
	blocks := p.ExtractAllCode(response)

	if len(blocks) != 1 {
		t.Fatalf("ExtractAllCode() returned %d blocks, want 1", len(blocks))
	}

	if blocks[0].Raw != "```js\nconsole.log('test');\n```" {
		t.Errorf("Raw = %q", blocks[0].Raw)
	}
}

// =============================================================================
// JSON Extraction Tests
// =============================================================================

func TestExtractJSON_FromCodeBlock(t *testing.T) {
	response := "```json\n{\"key\": \"value\", \"num\": 42}\n```"

	p := NewParser()
	data := p.ExtractJSON(response)

	if data == nil {
		t.Fatal("ExtractJSON() returned nil")
	}
	if data["key"] != "value" {
		t.Errorf("data[key] = %v, want 'value'", data["key"])
	}
	if data["num"] != float64(42) {
		t.Errorf("data[num] = %v, want 42", data["num"])
	}
}

func TestExtractJSON_FromUnlabeledCodeBlock(t *testing.T) {
	response := "```\n{\"unlabeled\": true}\n```"

	p := NewParser()
	data := p.ExtractJSON(response)

	if data == nil {
		t.Fatal("ExtractJSON() returned nil")
	}
	if data["unlabeled"] != true {
		t.Errorf("data[unlabeled] = %v, want true", data["unlabeled"])
	}
}

func TestExtractJSON_InlineJSON(t *testing.T) {
	response := "Here is the result:\n{\"inline\": true}\n\nThat's it."

	p := NewParser()
	data := p.ExtractJSON(response)

	if data == nil {
		t.Fatal("ExtractJSON() returned nil")
	}
	if data["inline"] != true {
		t.Errorf("data[inline] = %v, want true", data["inline"])
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	response := "No JSON here at all"

	p := NewParser()
	data := p.ExtractJSON(response)

	if data != nil {
		t.Errorf("ExtractJSON() = %v, want nil", data)
	}
}

func TestExtractJSON_InvalidJSON(t *testing.T) {
	response := "```json\n{invalid json}\n```"

	p := NewParser()
	data := p.ExtractJSON(response)

	if data != nil {
		t.Errorf("ExtractJSON() = %v, want nil for invalid JSON", data)
	}
}

func TestExtractJSON_MultipleBlocks(t *testing.T) {
	response := "```json\n{\"first\": 1}\n```\n\n```json\n{\"second\": 2}\n```"

	p := NewParser()
	parsed := p.Parse(response)

	if len(parsed.JSONBlocks) != 2 {
		t.Errorf("JSONBlocks count = %d, want 2", len(parsed.JSONBlocks))
	}
}

func TestExtractJSONArray(t *testing.T) {
	response := "```json\n[{\"id\": 1}, {\"id\": 2}]\n```"

	p := NewParser()
	arr := p.ExtractJSONArray(response)

	if len(arr) != 2 {
		t.Fatalf("ExtractJSONArray() returned %d items, want 2", len(arr))
	}
	if arr[0]["id"] != float64(1) {
		t.Errorf("arr[0][id] = %v, want 1", arr[0]["id"])
	}
	if arr[1]["id"] != float64(2) {
		t.Errorf("arr[1][id] = %v, want 2", arr[1]["id"])
	}
}

func TestExtractJSONArray_InlineArray(t *testing.T) {
	response := "Result:\n[{\"inline\": true}]"

	p := NewParser()
	arr := p.ExtractJSONArray(response)

	if len(arr) != 1 {
		t.Fatalf("ExtractJSONArray() returned %d items, want 1", len(arr))
	}
	if arr[0]["inline"] != true {
		t.Errorf("arr[0][inline] = %v, want true", arr[0]["inline"])
	}
}

func TestExtractJSONArray_Empty(t *testing.T) {
	response := "No arrays here"

	p := NewParser()
	arr := p.ExtractJSONArray(response)

	if len(arr) != 0 {
		t.Errorf("ExtractJSONArray() = %v, want nil or empty", arr)
	}
}

// =============================================================================
// YAML Extraction Tests
// =============================================================================

func TestExtractYAML(t *testing.T) {
	response := "```yaml\nname: test\nvalue: 42\n```"

	p := NewParser()
	blocks := p.ExtractYAML(response)

	if len(blocks) != 1 {
		t.Fatalf("ExtractYAML() returned %d blocks, want 1", len(blocks))
	}
	if blocks[0]["name"] != "test" {
		t.Errorf("blocks[0][name] = %v, want 'test'", blocks[0]["name"])
	}
	if blocks[0]["value"] != 42 {
		t.Errorf("blocks[0][value] = %v, want 42", blocks[0]["value"])
	}
}

func TestExtractYAML_YmlExtension(t *testing.T) {
	response := "```yml\nkey: value\n```"

	p := NewParser()
	blocks := p.ExtractYAML(response)

	if len(blocks) != 1 {
		t.Fatalf("ExtractYAML() returned %d blocks, want 1", len(blocks))
	}
	if blocks[0]["key"] != "value" {
		t.Errorf("blocks[0][key] = %v, want 'value'", blocks[0]["key"])
	}
}

func TestExtractYAML_MultipleBlocks(t *testing.T) {
	response := "```yaml\na: 1\n```\n\n```yaml\nb: 2\n```"

	p := NewParser()
	blocks := p.ExtractYAML(response)

	if len(blocks) != 2 {
		t.Fatalf("ExtractYAML() returned %d blocks, want 2", len(blocks))
	}
}

func TestExtractYAML_InvalidYAML(t *testing.T) {
	response := "```yaml\n{{{{invalid yaml\n```"

	p := NewParser()
	blocks := p.ExtractYAML(response)

	if len(blocks) != 0 {
		t.Errorf("ExtractYAML() returned %d blocks, want 0 for invalid YAML", len(blocks))
	}
}

func TestExtractYAML_NoYAML(t *testing.T) {
	response := "```go\nfunc main() {}\n```"

	p := NewParser()
	blocks := p.ExtractYAML(response)

	if len(blocks) != 0 {
		t.Errorf("ExtractYAML() returned %d blocks, want 0", len(blocks))
	}
}

// =============================================================================
// Section Extraction Tests
// =============================================================================

func TestExtractSection(t *testing.T) {
	response := `# Introduction

This is the introduction.

## Details

Here are some details.

## Summary

The summary goes here.`

	p := NewParser()

	intro := p.ExtractSection(response, "Introduction")
	if intro != "This is the introduction." {
		t.Errorf("Introduction section = %q", intro)
	}

	details := p.ExtractSection(response, "Details")
	if details != "Here are some details." {
		t.Errorf("Details section = %q", details)
	}

	summary := p.ExtractSection(response, "Summary")
	if summary != "The summary goes here." {
		t.Errorf("Summary section = %q", summary)
	}
}

func TestExtractSection_CaseInsensitive(t *testing.T) {
	response := "# My Section\n\nContent here."

	p := NewParser()

	content := p.ExtractSection(response, "my section")
	if content != "Content here." {
		t.Errorf("Case-insensitive match failed: %q", content)
	}
}

func TestExtractSection_NotFound(t *testing.T) {
	response := "# Other Section\n\nSome content."

	p := NewParser()
	content := p.ExtractSection(response, "Missing")

	if content != "" {
		t.Errorf("ExtractSection() = %q, want empty string", content)
	}
}

func TestExtractSection_VariousLevels(t *testing.T) {
	response := "### Level 3\n\nL3 content.\n\n###### Level 6\n\nL6 content."

	p := NewParser()

	l3 := p.ExtractSection(response, "Level 3")
	if l3 != "L3 content." {
		t.Errorf("Level 3 = %q", l3)
	}

	l6 := p.ExtractSection(response, "Level 6")
	if l6 != "L6 content." {
		t.Errorf("Level 6 = %q", l6)
	}
}

// =============================================================================
// List Extraction Tests
// =============================================================================

func TestExtractList_BulletDash(t *testing.T) {
	response := `Tasks:
- First item
- Second item
- Third item`

	p := NewParser()
	items := p.ExtractList(response)

	expected := []string{"First item", "Second item", "Third item"}
	if !reflect.DeepEqual(items, expected) {
		t.Errorf("ExtractList() = %v, want %v", items, expected)
	}
}

func TestExtractList_BulletAsterisk(t *testing.T) {
	response := `Tasks:
* Item A
* Item B`

	p := NewParser()
	items := p.ExtractList(response)

	expected := []string{"Item A", "Item B"}
	if !reflect.DeepEqual(items, expected) {
		t.Errorf("ExtractList() = %v, want %v", items, expected)
	}
}

func TestExtractList_Mixed(t *testing.T) {
	response := `- Dash item
* Asterisk item`

	p := NewParser()
	items := p.ExtractList(response)

	if len(items) != 2 {
		t.Errorf("ExtractList() returned %d items, want 2", len(items))
	}
}

func TestExtractList_Indented(t *testing.T) {
	response := `List:
  - Indented item
    - More indented`

	p := NewParser()
	items := p.ExtractList(response)

	if len(items) != 2 {
		t.Errorf("ExtractList() returned %d items, want 2", len(items))
	}
}

func TestExtractList_Empty(t *testing.T) {
	response := "No list here"

	p := NewParser()
	items := p.ExtractList(response)

	if len(items) != 0 {
		t.Errorf("ExtractList() returned %d items, want 0", len(items))
	}
}

func TestExtractNumberedList(t *testing.T) {
	response := `Steps:
1. First step
2. Second step
3. Third step`

	p := NewParser()
	items := p.ExtractNumberedList(response)

	expected := []string{"First step", "Second step", "Third step"}
	if !reflect.DeepEqual(items, expected) {
		t.Errorf("ExtractNumberedList() = %v, want %v", items, expected)
	}
}

func TestExtractNumberedList_Parenthesis(t *testing.T) {
	response := `Steps:
1) First
2) Second`

	p := NewParser()
	items := p.ExtractNumberedList(response)

	if len(items) != 2 {
		t.Errorf("ExtractNumberedList() returned %d items, want 2", len(items))
	}
}

func TestExtractNumberedList_LargeNumbers(t *testing.T) {
	response := `10. Tenth item
100. Hundredth item`

	p := NewParser()
	items := p.ExtractNumberedList(response)

	if len(items) != 2 {
		t.Errorf("ExtractNumberedList() returned %d items, want 2", len(items))
	}
}

// =============================================================================
// Full Parse Tests
// =============================================================================

func TestParse_ComprehensiveResponse(t *testing.T) {
	response := `# Summary

Here is my analysis.

## Code Example

` + "```go\n" + `func example() {
    return "test"
}
` + "```" + `

## Configuration

` + "```json\n" + `{"enabled": true}
` + "```" + `

That's all.`

	p := NewParser()
	result := p.Parse(response)

	// Check raw
	if result.Raw != response {
		t.Error("Raw not preserved")
	}

	// Check code blocks
	if len(result.CodeBlocks) != 2 {
		t.Errorf("CodeBlocks count = %d, want 2", len(result.CodeBlocks))
	}

	// Check JSON blocks
	if len(result.JSONBlocks) != 1 {
		t.Errorf("JSONBlocks count = %d, want 1", len(result.JSONBlocks))
	}

	// Check sections
	if len(result.Sections) != 3 {
		t.Errorf("Sections count = %d, want 3", len(result.Sections))
	}

	// Check text has code blocks removed
	if result.Text == result.Raw {
		t.Error("Text should have code blocks removed")
	}
}

func TestParse_EmptyResponse(t *testing.T) {
	p := NewParser()
	result := p.Parse("")

	if result.Raw != "" {
		t.Error("Raw should be empty")
	}
	if result.Text != "" {
		t.Error("Text should be empty")
	}
	if len(result.CodeBlocks) != 0 {
		t.Error("CodeBlocks should be empty")
	}
	if len(result.JSONBlocks) != 0 {
		t.Error("JSONBlocks should be empty")
	}
}

// =============================================================================
// Predicate Tests
// =============================================================================

func TestHasCodeBlock(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     bool
	}{
		{"with code block", "```go\ncode\n```", true},
		{"without code block", "just text", false},
		{"with unlabeled block", "```\ncode\n```", true},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.HasCodeBlock(tt.response)
			if got != tt.want {
				t.Errorf("HasCodeBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasJSON(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     bool
	}{
		{"with JSON block", "```json\n{\"a\": 1}\n```", true},
		{"with inline JSON", "{\"inline\": true}", true},
		{"without JSON", "no json here", false},
		{"invalid JSON", "```json\n{invalid}\n```", false},
	}

	p := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.HasJSON(tt.response)
			if got != tt.want {
				t.Errorf("HasJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Convenience Function Tests
// =============================================================================

func TestConvenienceParse(t *testing.T) {
	response := "```go\nfunc test() {}\n```"

	result := Parse(response)

	if result == nil {
		t.Fatal("Parse() returned nil")
	}
	if len(result.CodeBlocks) != 1 {
		t.Errorf("CodeBlocks count = %d, want 1", len(result.CodeBlocks))
	}
}

func TestConvenienceExtractJSON(t *testing.T) {
	response := "```json\n{\"test\": true}\n```"

	data := ExtractJSON(response)

	if data == nil {
		t.Fatal("ExtractJSON() returned nil")
	}
	if data["test"] != true {
		t.Errorf("data[test] = %v, want true", data["test"])
	}
}

func TestConvenienceExtractCode(t *testing.T) {
	response := "```python\nprint('hello')\n```"

	code := ExtractCode(response, "python")

	if code != "print('hello')\n" {
		t.Errorf("ExtractCode() = %q", code)
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestEdgeCase_NestedCodeFences(t *testing.T) {
	// This is tricky - markdown inside a code block
	response := "```md\n```go\nnested\n```\n```"

	p := NewParser()
	blocks := p.ExtractAllCode(response)

	// The regex is greedy so it should capture the outer block
	if len(blocks) == 0 {
		t.Error("Should extract at least one block")
	}
}

func TestEdgeCase_EmptyCodeBlock(t *testing.T) {
	response := "```go\n```"

	p := NewParser()
	blocks := p.ExtractAllCode(response)

	// Empty content between fences
	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Content != "" {
		t.Errorf("Expected empty content, got %q", blocks[0].Content)
	}
}

func TestEdgeCase_JSONWithSpecialChars(t *testing.T) {
	response := "```json\n{\"message\": \"hello\\nworld\", \"path\": \"C:\\\\Users\"}\n```"

	p := NewParser()
	data := p.ExtractJSON(response)

	if data == nil {
		t.Fatal("ExtractJSON() returned nil")
	}
	if data["message"] != "hello\nworld" {
		t.Errorf("message = %v", data["message"])
	}
}

func TestEdgeCase_SectionWithCodeBlock(t *testing.T) {
	response := "# Code Section\n\n```go\ncode here\n```\n\nText after."

	p := NewParser()
	section := p.ExtractSection(response, "Code Section")

	if section == "" {
		t.Error("Section should not be empty")
	}
	// Section should include the code block
	if len(section) == 0 {
		t.Error("Section content empty")
	}
}

func TestEdgeCase_DuplicateJSONPrevention(t *testing.T) {
	// Same JSON in code block and inline - should not duplicate
	response := "```json\n{\"dup\": true}\n```\n\n{\"dup\": true}"

	p := NewParser()
	result := p.Parse(response)

	// Should only have one JSON block (deduped)
	if len(result.JSONBlocks) != 1 {
		t.Errorf("JSONBlocks = %d, want 1 (should dedupe)", len(result.JSONBlocks))
	}
}

func TestEdgeCase_WhitespaceInListItems(t *testing.T) {
	response := "-   Extra spaces   \n- Normal"

	p := NewParser()
	items := p.ExtractList(response)

	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}
	// Items should be trimmed
	if items[0] != "Extra spaces" {
		t.Errorf("Item 0 = %q, should be trimmed", items[0])
	}
}

func TestEdgeCase_YAMLWithNestedStructure(t *testing.T) {
	response := "```yaml\nroot:\n  nested:\n    deep: value\n```"

	p := NewParser()
	blocks := p.ExtractYAML(response)

	if len(blocks) != 1 {
		t.Fatalf("Expected 1 block, got %d", len(blocks))
	}

	root, ok := blocks[0]["root"].(map[string]any)
	if !ok {
		t.Fatal("root should be a map")
	}
	nested, ok := root["nested"].(map[string]any)
	if !ok {
		t.Fatal("nested should be a map")
	}
	if nested["deep"] != "value" {
		t.Errorf("deep = %v, want 'value'", nested["deep"])
	}
}

// =============================================================================
// JSON Equal Helper Tests
// =============================================================================

func TestJsonEqual(t *testing.T) {
	tests := []struct {
		name string
		a    map[string]any
		b    map[string]any
		want bool
	}{
		{
			"equal simple",
			map[string]any{"a": 1},
			map[string]any{"a": 1},
			true,
		},
		{
			"different values",
			map[string]any{"a": 1},
			map[string]any{"a": 2},
			false,
		},
		{
			"different keys",
			map[string]any{"a": 1},
			map[string]any{"b": 1},
			false,
		},
		{
			"different lengths",
			map[string]any{"a": 1},
			map[string]any{"a": 1, "b": 2},
			false,
		},
		{
			"equal nested",
			map[string]any{"a": map[string]any{"b": 1}},
			map[string]any{"a": map[string]any{"b": 1}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("jsonEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
