package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Internal tests for private functions

func TestBuildArgs(t *testing.T) {
	tests := []struct {
		name     string
		client   *ClaudeCLI
		req      CompletionRequest
		contains []string
	}{
		{
			name:   "basic request",
			client: NewClaudeCLI(),
			req: CompletionRequest{
				Messages: []Message{{Role: RoleUser, Content: "Hello"}},
			},
			contains: []string{"--print", "-p"},
		},
		{
			name:   "with system prompt",
			client: NewClaudeCLI(),
			req: CompletionRequest{
				SystemPrompt: "Be helpful",
				Messages:     []Message{{Role: RoleUser, Content: "Hi"}},
			},
			contains: []string{"--system-prompt", "Be helpful"},
		},
		{
			name:   "with model from client",
			client: NewClaudeCLI(WithModel("claude-3-opus")),
			req: CompletionRequest{
				Messages: []Message{{Role: RoleUser, Content: "Test"}},
			},
			contains: []string{"--model", "claude-3-opus"},
		},
		{
			name:   "with model from request overrides client",
			client: NewClaudeCLI(WithModel("default-model")),
			req: CompletionRequest{
				Model:    "request-model",
				Messages: []Message{{Role: RoleUser, Content: "Test"}},
			},
			contains: []string{"--model"}, // Should have model flag
		},
		{
			name:   "with max tokens",
			client: NewClaudeCLI(),
			req: CompletionRequest{
				MaxTokens: 1000,
				Messages:  []Message{{Role: RoleUser, Content: "Test"}},
			},
			contains: []string{"--max-tokens", "1000"},
		},
		{
			name:   "with allowed tools",
			client: NewClaudeCLI(WithAllowedTools([]string{"read", "write"})),
			req: CompletionRequest{
				Messages: []Message{{Role: RoleUser, Content: "Test"}},
			},
			contains: []string{"--allowedTools"},
		},
		{
			name:   "multiple messages",
			client: NewClaudeCLI(),
			req: CompletionRequest{
				Messages: []Message{
					{Role: RoleUser, Content: "First"},
					{Role: RoleAssistant, Content: "Response"},
					{Role: RoleUser, Content: "Second"},
				},
			},
			contains: []string{"-p"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.client.buildArgs(tt.req)

			for _, want := range tt.contains {
				found := false
				for _, arg := range args {
					if arg == want {
						found = true
						break
					}
				}
				assert.True(t, found, "expected args to contain %q, got %v", want, args)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	client := NewClaudeCLI(WithModel("test-model"))

	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "simple text",
			data:     []byte("Hello, world!"),
			expected: "Hello, world!",
		},
		{
			name:     "with leading/trailing whitespace",
			data:     []byte("  trimmed content  \n"),
			expected: "trimmed content",
		},
		{
			name:     "multiline",
			data:     []byte("Line 1\nLine 2\nLine 3"),
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "empty",
			data:     []byte(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := client.parseResponse(tt.data)

			assert.Equal(t, tt.expected, resp.Content)
			assert.Equal(t, "stop", resp.FinishReason)
			assert.Equal(t, "test-model", resp.Model)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		errMsg    string
		retryable bool
	}{
		{"rate limit exceeded", true},
		{"Rate Limit", true},
		{"request timeout", true},
		{"server overloaded", true},
		{"503 service unavailable", true},
		{"error 529", true},
		{"invalid request", false},
		{"authentication failed", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			result := isRetryableError(tt.errMsg)
			assert.Equal(t, tt.retryable, result)
		})
	}
}
