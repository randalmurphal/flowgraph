package llm

import (
	"encoding/json"
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

func TestParseJSONResponse(t *testing.T) {
	client := NewClaudeCLI(WithModel("fallback-model"))

	tests := []struct {
		name          string
		data          []byte
		wantContent   string
		wantSessionID string
		wantCostUSD   float64
		wantModel     string
		wantTokens    TokenUsage
	}{
		{
			name: "full JSON response",
			data: []byte(`{
				"type": "result",
				"subtype": "success",
				"is_error": false,
				"result": "Hello from Claude!",
				"session_id": "abc-123-def",
				"duration_ms": 2500,
				"num_turns": 1,
				"total_cost_usd": 0.05,
				"usage": {
					"input_tokens": 100,
					"output_tokens": 50,
					"cache_creation_input_tokens": 500,
					"cache_read_input_tokens": 200
				},
				"modelUsage": {
					"claude-opus-4": {
						"inputTokens": 100,
						"outputTokens": 50,
						"costUSD": 0.05
					}
				}
			}`),
			wantContent:   "Hello from Claude!",
			wantSessionID: "abc-123-def",
			wantCostUSD:   0.05,
			wantModel:     "claude-opus-4",
			wantTokens: TokenUsage{
				InputTokens:              100,
				OutputTokens:             50,
				TotalTokens:              150,
				CacheCreationInputTokens: 500,
				CacheReadInputTokens:     200,
			},
		},
		{
			name: "error response",
			data: []byte(`{
				"type": "result",
				"subtype": "error",
				"is_error": true,
				"result": "Something went wrong",
				"session_id": "err-session",
				"total_cost_usd": 0.01,
				"usage": {
					"input_tokens": 10,
					"output_tokens": 0
				}
			}`),
			wantContent:   "Something went wrong",
			wantSessionID: "err-session",
			wantCostUSD:   0.01,
			wantModel:     "fallback-model", // Falls back to client model
			wantTokens: TokenUsage{
				InputTokens:  10,
				OutputTokens: 0,
				TotalTokens:  10,
			},
		},
		{
			name:          "text response fallback",
			data:          []byte("Plain text response"),
			wantContent:   "Plain text response",
			wantSessionID: "",
			wantCostUSD:   0,
			wantModel:     "fallback-model",
			wantTokens: TokenUsage{
				InputTokens:  0,
				OutputTokens: 0,
				TotalTokens:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := client.parseResponse(tt.data)

			assert.Equal(t, tt.wantContent, resp.Content)
			assert.Equal(t, tt.wantSessionID, resp.SessionID)
			assert.Equal(t, tt.wantCostUSD, resp.CostUSD)
			assert.Equal(t, tt.wantModel, resp.Model)
			assert.Equal(t, tt.wantTokens.InputTokens, resp.Usage.InputTokens)
			assert.Equal(t, tt.wantTokens.OutputTokens, resp.Usage.OutputTokens)
			assert.Equal(t, tt.wantTokens.TotalTokens, resp.Usage.TotalTokens)
			assert.Equal(t, tt.wantTokens.CacheCreationInputTokens, resp.Usage.CacheCreationInputTokens)
			assert.Equal(t, tt.wantTokens.CacheReadInputTokens, resp.Usage.CacheReadInputTokens)
		})
	}
}

func TestBuildArgsNewOptions(t *testing.T) {
	tests := []struct {
		name     string
		client   *ClaudeCLI
		req      CompletionRequest
		contains []string
		excludes []string
	}{
		{
			name:     "default JSON output format",
			client:   NewClaudeCLI(),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--output-format", "json"},
		},
		{
			name:     "text output format",
			client:   NewClaudeCLI(WithOutputFormat(OutputFormatText)),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			excludes: []string{"--output-format"}, // Text format doesn't add the flag
		},
		{
			name:     "with JSON schema",
			client:   NewClaudeCLI(WithJSONSchema(`{"type": "object"}`)),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--json-schema", `{"type": "object"}`},
		},
		{
			name:     "with session ID",
			client:   NewClaudeCLI(WithSessionID("session-xyz")),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--session-id", "session-xyz"},
		},
		{
			name:     "with continue",
			client:   NewClaudeCLI(WithContinue()),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--continue"},
		},
		{
			name:     "with resume",
			client:   NewClaudeCLI(WithResume("prev-session")),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--resume", "prev-session"},
		},
		{
			name:     "with no session persistence",
			client:   NewClaudeCLI(WithNoSessionPersistence()),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--no-session-persistence"},
		},
		{
			name:     "with disallowed tools",
			client:   NewClaudeCLI(WithDisallowedTools([]string{"bash", "write"})),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--disallowed-tools", "bash"},
		},
		{
			name:     "with dangerously skip permissions",
			client:   NewClaudeCLI(WithDangerouslySkipPermissions()),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--dangerously-skip-permissions"},
		},
		{
			name:     "with permission mode",
			client:   NewClaudeCLI(WithPermissionMode(PermissionModeBypassPermissions)),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--permission-mode", "bypassPermissions"},
		},
		{
			name:     "with setting sources",
			client:   NewClaudeCLI(WithSettingSources([]string{"project", "local"})),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--setting-sources", "project,local"},
		},
		{
			name:     "with add dirs",
			client:   NewClaudeCLI(WithAddDirs([]string{"/tmp", "/home"})),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--add-dir", "/tmp"},
		},
		{
			name:     "with system prompt from client",
			client:   NewClaudeCLI(WithSystemPrompt("Client system prompt")),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--system-prompt", "Client system prompt"},
		},
		{
			name:     "client system prompt overrides request",
			client:   NewClaudeCLI(WithSystemPrompt("Client prompt")),
			req:      CompletionRequest{SystemPrompt: "Request prompt", Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--system-prompt", "Client prompt"},
		},
		{
			name:     "with append system prompt",
			client:   NewClaudeCLI(WithAppendSystemPrompt("Additional context")),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--append-system-prompt", "Additional context"},
		},
		{
			name:     "with max budget USD",
			client:   NewClaudeCLI(WithMaxBudgetUSD(5.0)),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--max-budget-usd", "5.000000"},
		},
		{
			name:     "with fallback model",
			client:   NewClaudeCLI(WithFallbackModel("haiku")),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--fallback-model", "haiku"},
		},
		{
			name:     "with max turns",
			client:   NewClaudeCLI(WithMaxTurns(5)),
			req:      CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{"--max-turns", "5"},
		},
		{
			name: "production configuration",
			client: NewClaudeCLI(
				WithModel("sonnet"),
				WithOutputFormat(OutputFormatJSON),
				WithDangerouslySkipPermissions(),
				WithMaxBudgetUSD(1.0),
				WithSettingSources([]string{"project", "local"}),
				WithDisallowedTools([]string{"Write", "Bash"}),
			),
			req: CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "Hi"}}},
			contains: []string{
				"--model", "sonnet",
				"--output-format", "json",
				"--dangerously-skip-permissions",
				"--max-budget-usd",
				"--setting-sources", "project,local",
				"--disallowed-tools",
			},
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

			for _, notWant := range tt.excludes {
				found := false
				for _, arg := range args {
					if arg == notWant {
						found = true
						break
					}
				}
				assert.False(t, found, "expected args NOT to contain %q, got %v", notWant, args)
			}
		})
	}
}

func TestCLIResponseTypes(t *testing.T) {
	// Test that CLIResponse properly parses all fields
	jsonData := []byte(`{
		"type": "result",
		"subtype": "success",
		"is_error": false,
		"result": "Test content",
		"session_id": "test-session-id",
		"duration_ms": 1500,
		"duration_api_ms": 2000,
		"num_turns": 2,
		"total_cost_usd": 0.123,
		"usage": {
			"input_tokens": 500,
			"output_tokens": 250,
			"cache_creation_input_tokens": 1000,
			"cache_read_input_tokens": 300
		},
		"modelUsage": {
			"claude-sonnet-4": {
				"inputTokens": 500,
				"outputTokens": 250,
				"cacheReadInputTokens": 300,
				"cacheCreationInputTokens": 1000,
				"costUSD": 0.123
			}
		}
	}`)

	var resp CLIResponse
	err := json.Unmarshal(jsonData, &resp)
	assert.NoError(t, err)

	assert.Equal(t, "result", resp.Type)
	assert.Equal(t, "success", resp.Subtype)
	assert.False(t, resp.IsError)
	assert.Equal(t, "Test content", resp.Result)
	assert.Equal(t, "test-session-id", resp.SessionID)
	assert.Equal(t, 1500, resp.DurationMS)
	assert.Equal(t, 2000, resp.DurationAPI)
	assert.Equal(t, 2, resp.NumTurns)
	assert.Equal(t, 0.123, resp.TotalCostUSD)

	// Usage
	assert.Equal(t, 500, resp.Usage.InputTokens)
	assert.Equal(t, 250, resp.Usage.OutputTokens)
	assert.Equal(t, 1000, resp.Usage.CacheCreationInputTokens)
	assert.Equal(t, 300, resp.Usage.CacheReadInputTokens)

	// Model usage
	modelUsage, ok := resp.ModelUsage["claude-sonnet-4"]
	assert.True(t, ok)
	assert.Equal(t, 500, modelUsage.InputTokens)
	assert.Equal(t, 250, modelUsage.OutputTokens)
	assert.Equal(t, 300, modelUsage.CacheReadInputTokens)
	assert.Equal(t, 1000, modelUsage.CacheCreationInputTokens)
	assert.Equal(t, 0.123, modelUsage.CostUSD)
}

func TestTokenUsageAdd_WithCacheTokens(t *testing.T) {
	usage := TokenUsage{
		InputTokens:              100,
		OutputTokens:             50,
		TotalTokens:              150,
		CacheCreationInputTokens: 500,
		CacheReadInputTokens:     200,
	}

	other := TokenUsage{
		InputTokens:              200,
		OutputTokens:             100,
		TotalTokens:              300,
		CacheCreationInputTokens: 300,
		CacheReadInputTokens:     100,
	}

	usage.Add(other)

	assert.Equal(t, 300, usage.InputTokens)
	assert.Equal(t, 150, usage.OutputTokens)
	assert.Equal(t, 450, usage.TotalTokens)
	assert.Equal(t, 800, usage.CacheCreationInputTokens)
	assert.Equal(t, 300, usage.CacheReadInputTokens)
}

func TestOutputFormatConstants(t *testing.T) {
	assert.Equal(t, OutputFormat("text"), OutputFormatText)
	assert.Equal(t, OutputFormat("json"), OutputFormatJSON)
	assert.Equal(t, OutputFormat("stream-json"), OutputFormatStreamJSON)
}

func TestPermissionModeConstants(t *testing.T) {
	assert.Equal(t, PermissionMode(""), PermissionModeDefault)
	assert.Equal(t, PermissionMode("acceptEdits"), PermissionModeAcceptEdits)
	assert.Equal(t, PermissionMode("bypassPermissions"), PermissionModeBypassPermissions)
}
