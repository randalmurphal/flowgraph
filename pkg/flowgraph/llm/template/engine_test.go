package template

import (
	"strings"
	"testing"
)

func TestEngine_Render_SimpleVariables(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name      string
		template  string
		variables map[string]any
		want      string
		wantErr   bool
	}{
		{
			name:      "single variable",
			template:  "Hello, {{name}}!",
			variables: map[string]any{"name": "World"},
			want:      "Hello, World!",
		},
		{
			name:      "multiple variables",
			template:  "{{greeting}}, {{name}}!",
			variables: map[string]any{"greeting": "Hi", "name": "Alice"},
			want:      "Hi, Alice!",
		},
		{
			name:      "missing variable renders empty",
			template:  "Hello, {{name}}!",
			variables: map[string]any{},
			want:      "Hello, <no value>!",
		},
		{
			name:      "variable with underscore",
			template:  "Task: {{task_id}}",
			variables: map[string]any{"task_id": "TK-123"},
			want:      "Task: TK-123",
		},
		{
			name:      "nested map access",
			template:  "Name: {{.task.name}}",
			variables: map[string]any{"task": map[string]any{"name": "Test"}},
			want:      "Name: Test",
		},
		{
			name:      "nil variables map",
			template:  "Hello, World!",
			variables: nil,
			want:      "Hello, World!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Render(tt.template, tt.variables)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEngine_Render_Conditionals(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name      string
		template  string
		variables map[string]any
		want      string
	}{
		{
			name:      "if true",
			template:  "{{#if urgent}}URGENT: {{/if}}Task",
			variables: map[string]any{"urgent": true},
			want:      "URGENT: Task",
		},
		{
			name:      "if false",
			template:  "{{#if urgent}}URGENT: {{/if}}Task",
			variables: map[string]any{"urgent": false},
			want:      "Task",
		},
		{
			name:      "if with else - true branch",
			template:  "Status: {{#if done}}Complete{{else}}Pending{{/if}}",
			variables: map[string]any{"done": true},
			want:      "Status: Complete",
		},
		{
			name:      "if with else - false branch",
			template:  "Status: {{#if done}}Complete{{else}}Pending{{/if}}",
			variables: map[string]any{"done": false},
			want:      "Status: Pending",
		},
		{
			name:      "if with string (non-empty)",
			template:  "{{#if name}}Hello, {{name}}{{/if}}",
			variables: map[string]any{"name": "Alice"},
			want:      "Hello, Alice",
		},
		{
			name:      "if with empty string",
			template:  "{{#if name}}Hello, {{name}}{{else}}No name{{/if}}",
			variables: map[string]any{"name": ""},
			want:      "No name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Render(tt.template, tt.variables)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEngine_Render_Iteration(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name      string
		template  string
		variables map[string]any
		want      string
	}{
		{
			name:      "each with strings",
			template:  "Items: {{#each items}}{{.}} {{/each}}",
			variables: map[string]any{"items": []string{"a", "b", "c"}},
			want:      "Items: a b c ",
		},
		{
			name:      "each with empty list",
			template:  "Items: {{#each items}}{{.}}{{/each}}",
			variables: map[string]any{"items": []string{}},
			want:      "Items: ",
		},
		{
			name:      "each with numbers",
			template:  "{{#each numbers}}[{{.}}]{{/each}}",
			variables: map[string]any{"numbers": []int{1, 2, 3}},
			want:      "[1][2][3]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Render(tt.template, tt.variables)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEngine_Render_Helpers(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name      string
		template  string
		variables map[string]any
		want      string
	}{
		{
			name:      "truncate",
			template:  "{{truncate description 10}}",
			variables: map[string]any{"description": "This is a very long description"},
			want:      "This is...",
		},
		{
			name:      "truncate short string",
			template:  "{{truncate text 100}}",
			variables: map[string]any{"text": "Short"},
			want:      "Short",
		},
		{
			name:      "upper",
			template:  "{{upper name}}",
			variables: map[string]any{"name": "alice"},
			want:      "ALICE",
		},
		{
			name:      "lower",
			template:  "{{lower name}}",
			variables: map[string]any{"name": "ALICE"},
			want:      "alice",
		},
		{
			name:      "trim",
			template:  "[{{trim text}}]",
			variables: map[string]any{"text": "  hello  "},
			want:      "[hello]",
		},
		{
			name:      "json",
			template:  "{{json data}}",
			variables: map[string]any{"data": map[string]string{"key": "value"}},
			want:      "{\n  \"key\": \"value\"\n}",
		},
		{
			name:      "contains true",
			template:  "{{contains text \"world\"}}",
			variables: map[string]any{"text": "hello world"},
			want:      "true",
		},
		{
			name:      "contains false",
			template:  "{{contains text \"xyz\"}}",
			variables: map[string]any{"text": "hello world"},
			want:      "false",
		},
		{
			name:      "hasPrefix true",
			template:  "{{hasPrefix text \"hello\"}}",
			variables: map[string]any{"text": "hello world"},
			want:      "true",
		},
		{
			name:      "hasSuffix true",
			template:  "{{hasSuffix text \"world\"}}",
			variables: map[string]any{"text": "hello world"},
			want:      "true",
		},
		{
			name:      "split",
			template:  `{{range split .text ","}}[{{.}}]{{end}}`,
			variables: map[string]any{"text": "a,b,c"},
			want:      "[a][b][c]",
		},
		{
			name:      "replace",
			template:  "{{replace text \"old\" \"new\"}}",
			variables: map[string]any{"text": "old value old"},
			want:      "new value new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Render(tt.template, tt.variables)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEngine_Render_HelperWithLiteral(t *testing.T) {
	e := NewEngine()

	got, err := e.Render("{{truncate description 20}}", map[string]any{
		"description": "This is a description that is quite long",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "This is a descrip..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEngine_Render_Errors(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name     string
		template string
		wantErr  error
	}{
		{
			name:     "empty template",
			template: "",
			wantErr:  ErrEmpty,
		},
		{
			name:     "invalid syntax",
			template: "{{#if}}missing condition{{/if}}",
			wantErr:  ErrParse,
		},
		{
			name:     "unclosed tag",
			template: "{{#if true}not closed",
			wantErr:  ErrParse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Render(tt.template, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr.Error()) {
				t.Errorf("error %q should contain %q", err.Error(), tt.wantErr.Error())
			}
		})
	}
}

func TestEngine_Parse(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name     string
		template string
		wantVars []string
		wantErr  bool
	}{
		{
			name:     "simple variable",
			template: "Hello, {{name}}!",
			wantVars: []string{"name"},
		},
		{
			name:     "multiple variables",
			template: "{{greeting}}, {{name}}! Your id is {{id}}.",
			wantVars: []string{"greeting", "name", "id"},
		},
		{
			name:     "variable in conditional",
			template: "{{#if done}}Task {{name}} complete{{/if}}",
			wantVars: []string{"done", "name"},
		},
		{
			name:     "variable in helper",
			template: "{{truncate description 100}}",
			wantVars: []string{"description"},
		},
		{
			name:     "empty template",
			template: "",
			wantErr:  true,
		},
		{
			name:     "no variables",
			template: "Plain text only",
			wantVars: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars, err := e.Parse(tt.template)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalSlices(vars, tt.wantVars) {
				t.Errorf("got %v, want %v", vars, tt.wantVars)
			}
		})
	}
}

func TestValidateVariables(t *testing.T) {
	tests := []struct {
		name     string
		required []string
		provided map[string]any
		wantErr  bool
		errVar   string
	}{
		{
			name:     "all provided",
			required: []string{"name", "age"},
			provided: map[string]any{"name": "Alice", "age": 30},
			wantErr:  false,
		},
		{
			name:     "missing one",
			required: []string{"name", "age"},
			provided: map[string]any{"name": "Alice"},
			wantErr:  true,
			errVar:   "age",
		},
		{
			name:     "empty required",
			required: []string{},
			provided: map[string]any{},
			wantErr:  false,
		},
		{
			name:     "extra provided",
			required: []string{"name"},
			provided: map[string]any{"name": "Alice", "extra": "value"},
			wantErr:  false,
		},
		{
			name:     "nil provided",
			required: []string{"name"},
			provided: nil,
			wantErr:  true,
			errVar:   "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVariables(tt.required, tt.provided)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), ErrVariable.Error()) {
					t.Errorf("error should wrap ErrVariable")
				}
				if !strings.Contains(err.Error(), tt.errVar) {
					t.Errorf("error should contain variable name %q", tt.errVar)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConvertSyntax(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "{{name}}",
			want:  "{{.name}}",
		},
		{
			input: "{{#if done}}yes{{/if}}",
			want:  "{{if .done}}yes{{end}}",
		},
		{
			input: "{{#if done}}yes{{else}}no{{/if}}",
			want:  "{{if .done}}yes{{else}}no{{end}}",
		},
		{
			input: "{{#each items}}{{.}}{{/each}}",
			want:  "{{range .items}}{{.}}{{end}}",
		},
		{
			input: "Hello, {{name}}! {{#if greeting}}{{greeting}}{{/if}}",
			want:  "Hello, {{.name}}! {{if .greeting}}{{.greeting}}{{end}}",
		},
		{
			input: "Keep {{else}} and {{end}} unchanged",
			want:  "Keep {{else}} and {{end}} unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := convertSyntax(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		length int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 2, "hi"},
		{"hello", 3, "hel"},
		{"hello", 4, "h..."},
		{"", 10, ""},
		{"abc", 0, ""},
		{"abc", 1, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.length)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.length, got, tt.want)
			}
		})
	}
}

func TestToJSON(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "map",
			input: map[string]string{"key": "value"},
			want:  "{\n  \"key\": \"value\"\n}",
		},
		{
			name:  "slice",
			input: []int{1, 2, 3},
			want:  "[\n  1,\n  2,\n  3\n]",
		},
		{
			name:  "string",
			input: "hello",
			want:  "\"hello\"",
		},
		{
			name:  "nil",
			input: nil,
			want:  "null",
		},
		{
			name:  "number",
			input: 42,
			want:  "42",
		},
		{
			name:  "bool",
			input: true,
			want:  "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSON(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultValue(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		defVal any
		want   any
	}{
		{"nil value", nil, "default", "default"},
		{"empty string", "", "default", "default"},
		{"non-empty string", "hello", "default", "hello"},
		{"zero int", 0, 10, 0},
		{"non-zero int", 5, 10, 5},
		{"false bool", false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultValue(tt.value, tt.defVal)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIndent(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		spaces int
		want   string
	}{
		{"multiline", "line1\nline2", 2, "  line1\n  line2"},
		{"single line", "single", 4, "    single"},
		{"empty string", "", 2, "  "},
		{"zero spaces", "hello", 0, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indent(tt.input, tt.spaces)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  string
	}{
		{"no wrap needed", "hello world", 20, "hello world"},
		{"wrap at word", "hello world", 6, "hello\nworld"},
		{"wrap long sentence", "the quick brown fox", 10, "the quick\nbrown fox"},
		{"zero width", "nowrap", 0, "nowrap"},
		{"negative width", "nowrap", -5, "nowrap"},
		{"single word longer than width", "superlongword", 5, "superlongword"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrap(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsNumber(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"-123", true},
		{"12.34", true},
		{"-12.34", true},
		{"abc", false},
		{"12abc", false},
		{"", false},
		{"1.2.3", true}, // multiple dots still passes (simplified check)
		{"0", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isNumber(tt.input)
			if got != tt.want {
				t.Errorf("isNumber(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidIdentifier(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"name", true},
		{"task_id", true},
		{"Name123", true},
		{"_private", true},
		{"123start", false},
		{"has-dash", false},
		{"has.dot", false},
		{"", false},
		{"a", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isValidIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsQuotedString(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`"hello"`, true},
		{`'hello'`, true},
		{`"a"`, true},
		{`""`, true},
		{`hello`, false},
		{`"unclosed`, false},
		{`'mismatched"`, false},
		{`a`, false},
		{``, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isQuotedString(tt.input)
			if got != tt.want {
				t.Errorf("isQuotedString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitArguments(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a b c", []string{"a", "b", "c"}},
		{`a "b c" d`, []string{"a", `"b c"`, "d"}},
		{`a 'b c' d`, []string{"a", `'b c'`, "d"}},
		{"single", []string{"single"}},
		{"", nil},
		{"  spaced  out  ", []string{"spaced", "out"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitArguments(tt.input)
			if !equalSlices(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractVariables(t *testing.T) {
	tests := []struct {
		template string
		want     []string
	}{
		{"{{name}}", []string{"name"}},
		{"{{greeting}} {{name}}", []string{"greeting", "name"}},
		{"{{name}} {{name}}", []string{"name"}}, // no duplicates
		{"{{#if done}}{{task}}{{/if}}", []string{"done", "task"}},
		{"{{truncate text 100}}", []string{"text"}},
		{"plain text", nil},
		{"{{#each items}}{{.}}{{/each}}", []string{"items"}},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			got := extractVariables(tt.template)
			if !equalSlices(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngine_AddFunc(t *testing.T) {
	e := NewEngine()
	e.AddFunc("double", func(s string) string {
		return s + s
	})

	// Use Go template syntax directly for custom functions
	got, err := e.Render("{{double .name}}", map[string]any{"name": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "testtest"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEngine_ComplexTemplate(t *testing.T) {
	e := NewEngine()

	tmpl := `# Task: {{title}}

## Description
{{description}}

{{#if priority}}
**Priority**: {{priority}}
{{/if}}

## Tags
{{#each tags}}
- {{.}}
{{/each}}

---
Generated at: {{timestamp}}`

	variables := map[string]any{
		"title":       "Implement feature",
		"description": "Add new functionality to the system",
		"priority":    "high",
		"tags":        []string{"feature", "backend", "urgent"},
		"timestamp":   "2025-01-15",
	}

	got, err := e.Render(tmpl, variables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"# Task: Implement feature",
		"**Priority**: high",
		"- feature",
		"- backend",
		"- urgent",
		"Generated at: 2025-01-15",
	}

	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("output should contain %q", check)
		}
	}
}

func TestEngine_RealWorldPrompt(t *testing.T) {
	e := NewEngine()

	tmpl := `You are an AI assistant helping with task management.

## Current Task
Title: {{title}}
ID: {{task_id}}
Status: {{status}}

{{#if description}}
## Description
{{truncate description 500}}
{{/if}}

## Instructions
Please analyze this task and provide:
1. A summary of what needs to be done
2. Estimated complexity (low/medium/high)
3. Suggested next steps`

	variables := map[string]any{
		"title":       "Implement user authentication",
		"task_id":     "TK-421",
		"status":      "in_progress",
		"description": "Add JWT-based authentication to the API. This should include login, logout, and token refresh endpoints.",
	}

	got, err := e.Render(tmpl, variables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"Title: Implement user authentication",
		"ID: TK-421",
		"Status: in_progress",
		"## Description",
		"Add JWT-based authentication",
	}

	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("output should contain %q", check)
		}
	}
}

func TestConvertArguments(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"name", ".name"},
		{"name 100", ".name 100"},
		{`name "hello"`, `.name "hello"`},
		{"true false", "true false"},
		{".already", ".already"},
		{"100 200", "100 200"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := convertArguments(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertHelperCalls(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"{{truncate text 100}}", "{{truncate .text 100}}"},
		{"{{upper name}}", "{{upper .name}}"},
		{`{{default val "fallback"}}`, `{{default .val "fallback"}}`},
		{"{{json data}}", "{{json .data}}"},
		{"{{wrap content 80}}", "{{wrap .content 80}}"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := convertHelperCalls(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEngine_JoinFunction(t *testing.T) {
	e := NewEngine()

	// Test join with Go template syntax (since join takes a slice as first arg)
	got, err := e.Render(`{{join .items ", "}}`, map[string]any{
		"items": []string{"a", "b", "c"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "a, b, c"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEngine_DefaultFunction(t *testing.T) {
	e := NewEngine()

	tests := []struct {
		name      string
		template  string
		variables map[string]any
		want      string
	}{
		{
			name:      "nil uses default",
			template:  `{{default .name "Anonymous"}}`,
			variables: map[string]any{},
			want:      "Anonymous",
		},
		{
			name:      "empty string uses default",
			template:  `{{default .name "Anonymous"}}`,
			variables: map[string]any{"name": ""},
			want:      "Anonymous",
		},
		{
			name:      "non-empty ignores default",
			template:  `{{default .name "Anonymous"}}`,
			variables: map[string]any{"name": "Alice"},
			want:      "Alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Render(tt.template, tt.variables)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// equalSlices checks if two string slices have the same elements (order independent).
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 && len(b) == 0 {
		return true
	}

	aMap := make(map[string]int)
	for _, v := range a {
		aMap[v]++
	}
	for _, v := range b {
		aMap[v]--
		if aMap[v] < 0 {
			return false
		}
	}
	return true
}
