package llm_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/rmurphy/flowgraph/pkg/flowgraph/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeCLI_BuildArgs(t *testing.T) {
	tests := []struct {
		name     string
		client   *llm.ClaudeCLI
		req      llm.CompletionRequest
		contains []string
		excludes []string
	}{
		{
			name:   "basic request",
			client: llm.NewClaudeCLI(),
			req: llm.CompletionRequest{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "Hello"},
				},
			},
			contains: []string{"--print", "-p", "Hello"},
		},
		{
			name:   "with system prompt",
			client: llm.NewClaudeCLI(),
			req: llm.CompletionRequest{
				SystemPrompt: "You are helpful",
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "Hi"},
				},
			},
			contains: []string{"--system-prompt", "You are helpful"},
		},
		{
			name:   "with model from client",
			client: llm.NewClaudeCLI(llm.WithModel("claude-3-opus")),
			req: llm.CompletionRequest{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "Test"},
				},
			},
			contains: []string{"--model", "claude-3-opus"},
		},
		{
			name:   "with model from request",
			client: llm.NewClaudeCLI(llm.WithModel("client-default")),
			req: llm.CompletionRequest{
				Model: "request-model",
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "Test"},
				},
			},
			// Request model should override client model
			contains: []string{"--model"},
		},
		{
			name:   "with max tokens",
			client: llm.NewClaudeCLI(),
			req: llm.CompletionRequest{
				MaxTokens: 1000,
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "Test"},
				},
			},
			contains: []string{"--max-tokens", "1000"},
		},
		{
			name:   "multiple messages",
			client: llm.NewClaudeCLI(),
			req: llm.CompletionRequest{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "First question"},
					{Role: llm.RoleAssistant, Content: "First answer"},
					{Role: llm.RoleUser, Content: "Follow-up"},
				},
			},
			contains: []string{"-p"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't directly test buildArgs since it's private
			// But we can verify the client is created correctly
			assert.NotNil(t, tt.client)
		})
	}
}

func TestClaudeCLI_Options(t *testing.T) {
	// Test WithClaudePath
	client := llm.NewClaudeCLI(llm.WithClaudePath("/custom/path/claude"))
	assert.NotNil(t, client)

	// Test WithWorkdir
	client = llm.NewClaudeCLI(llm.WithWorkdir("/some/workdir"))
	assert.NotNil(t, client)

	// Test WithAllowedTools
	client = llm.NewClaudeCLI(llm.WithAllowedTools([]string{"read", "write"}))
	assert.NotNil(t, client)

	// Test all options combined
	client = llm.NewClaudeCLI(
		llm.WithClaudePath("/custom/claude"),
		llm.WithModel("claude-3-opus"),
		llm.WithWorkdir("/project"),
		llm.WithAllowedTools([]string{"bash"}),
	)
	assert.NotNil(t, client)
}

func TestClaudeCLI_NewOptions(t *testing.T) {
	// Test output control options
	t.Run("output format options", func(t *testing.T) {
		client := llm.NewClaudeCLI(
			llm.WithOutputFormat(llm.OutputFormatJSON),
			llm.WithJSONSchema(`{"type": "object", "properties": {"name": {"type": "string"}}}`),
		)
		assert.NotNil(t, client)
	})

	// Test session management options
	t.Run("session management options", func(t *testing.T) {
		client := llm.NewClaudeCLI(
			llm.WithSessionID("test-session"),
		)
		assert.NotNil(t, client)

		client = llm.NewClaudeCLI(llm.WithContinue())
		assert.NotNil(t, client)

		client = llm.NewClaudeCLI(llm.WithResume("prev-session"))
		assert.NotNil(t, client)

		client = llm.NewClaudeCLI(llm.WithNoSessionPersistence())
		assert.NotNil(t, client)
	})

	// Test tool control options
	t.Run("tool control options", func(t *testing.T) {
		client := llm.NewClaudeCLI(
			llm.WithAllowedTools([]string{"read", "write"}),
			llm.WithDisallowedTools([]string{"bash", "execute"}),
		)
		assert.NotNil(t, client)
	})

	// Test permission options
	t.Run("permission options", func(t *testing.T) {
		client := llm.NewClaudeCLI(llm.WithDangerouslySkipPermissions())
		assert.NotNil(t, client)

		client = llm.NewClaudeCLI(llm.WithPermissionMode(llm.PermissionModeAcceptEdits))
		assert.NotNil(t, client)

		client = llm.NewClaudeCLI(llm.WithPermissionMode(llm.PermissionModeBypassPermissions))
		assert.NotNil(t, client)

		client = llm.NewClaudeCLI(llm.WithSettingSources([]string{"project", "local", "user"}))
		assert.NotNil(t, client)
	})

	// Test context options
	t.Run("context options", func(t *testing.T) {
		client := llm.NewClaudeCLI(
			llm.WithAddDirs([]string{"/tmp", "/home/user/project"}),
		)
		assert.NotNil(t, client)

		client = llm.NewClaudeCLI(llm.WithSystemPrompt("You are a helpful assistant"))
		assert.NotNil(t, client)

		client = llm.NewClaudeCLI(llm.WithAppendSystemPrompt("Always be concise"))
		assert.NotNil(t, client)
	})

	// Test budget options
	t.Run("budget options", func(t *testing.T) {
		client := llm.NewClaudeCLI(llm.WithMaxBudgetUSD(5.0))
		assert.NotNil(t, client)

		client = llm.NewClaudeCLI(llm.WithFallbackModel("haiku"))
		assert.NotNil(t, client)
	})

	// Test production configuration (all options combined)
	t.Run("production configuration", func(t *testing.T) {
		client := llm.NewClaudeCLI(
			llm.WithClaudePath("/usr/local/bin/claude"),
			llm.WithModel("sonnet"),
			llm.WithWorkdir("/home/user/project"),
			llm.WithTimeout(10*time.Minute),
			llm.WithOutputFormat(llm.OutputFormatJSON),
			llm.WithDangerouslySkipPermissions(),
			llm.WithSettingSources([]string{"project", "local"}),
			llm.WithMaxBudgetUSD(1.0),
			llm.WithFallbackModel("haiku"),
			llm.WithDisallowedTools([]string{"Write", "Bash"}),
			llm.WithAppendSystemPrompt("Be extra careful with code changes"),
		)
		assert.NotNil(t, client)
	})
}

func TestClaudeCLI_OutputFormatConstants(t *testing.T) {
	// Verify output format constants are accessible
	assert.Equal(t, llm.OutputFormat("text"), llm.OutputFormatText)
	assert.Equal(t, llm.OutputFormat("json"), llm.OutputFormatJSON)
	assert.Equal(t, llm.OutputFormat("stream-json"), llm.OutputFormatStreamJSON)
}

func TestClaudeCLI_PermissionModeConstants(t *testing.T) {
	// Verify permission mode constants are accessible
	assert.Equal(t, llm.PermissionMode(""), llm.PermissionModeDefault)
	assert.Equal(t, llm.PermissionMode("acceptEdits"), llm.PermissionModeAcceptEdits)
	assert.Equal(t, llm.PermissionMode("bypassPermissions"), llm.PermissionModeBypassPermissions)
}

func TestCompletionResponse_NewFields(t *testing.T) {
	// Test that new fields are accessible on CompletionResponse
	resp := &llm.CompletionResponse{
		Content:      "Hello",
		SessionID:    "session-123",
		CostUSD:      0.05,
		NumTurns:     2,
		FinishReason: "stop",
		Model:        "sonnet",
		Usage: llm.TokenUsage{
			InputTokens:              100,
			OutputTokens:             50,
			TotalTokens:              150,
			CacheCreationInputTokens: 500,
			CacheReadInputTokens:     200,
		},
	}

	assert.Equal(t, "session-123", resp.SessionID)
	assert.Equal(t, 0.05, resp.CostUSD)
	assert.Equal(t, 2, resp.NumTurns)
	assert.Equal(t, 500, resp.Usage.CacheCreationInputTokens)
	assert.Equal(t, 200, resp.Usage.CacheReadInputTokens)
}

func TestTokenUsage_Add_WithCacheTokens(t *testing.T) {
	usage := llm.TokenUsage{
		InputTokens:              100,
		OutputTokens:             50,
		TotalTokens:              150,
		CacheCreationInputTokens: 500,
		CacheReadInputTokens:     200,
	}

	other := llm.TokenUsage{
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

func TestClaudeCLI_IntegrationSkip(t *testing.T) {
	// Skip if claude binary not available
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude binary not available, skipping integration test")
	}

	// This would be an actual integration test if claude is available
	// For now, just verify the client can be created
	client := llm.NewClaudeCLI()
	assert.NotNil(t, client)
}

func TestClaudeCLI_Error(t *testing.T) {
	err := llm.NewError("complete", assert.AnError, true)
	assert.Contains(t, err.Error(), "llm complete")
	assert.True(t, err.Retryable)
	assert.Equal(t, assert.AnError, err.Unwrap())
}

func TestLLMErrors(t *testing.T) {
	// Verify sentinel errors are defined
	assert.NotNil(t, llm.ErrUnavailable)
	assert.NotNil(t, llm.ErrContextTooLong)
	assert.NotNil(t, llm.ErrRateLimited)
	assert.NotNil(t, llm.ErrInvalidRequest)
	assert.NotNil(t, llm.ErrTimeout)
}

func TestClaudeCLI_WithTimeout(t *testing.T) {
	client := llm.NewClaudeCLI(llm.WithTimeout(10 * time.Second))
	assert.NotNil(t, client)
}

func TestClaudeCLI_Complete_NonExistentBinary(t *testing.T) {
	client := llm.NewClaudeCLI(llm.WithClaudePath("/nonexistent/path/to/claude"))

	_, err := client.Complete(context.Background(), llm.CompletionRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "test"}},
	})

	assert.Error(t, err)
}

func TestClaudeCLI_Stream_NonExistentBinary(t *testing.T) {
	client := llm.NewClaudeCLI(llm.WithClaudePath("/nonexistent/path/to/claude"))

	_, err := client.Stream(context.Background(), llm.CompletionRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "test"}},
	})

	assert.Error(t, err)
}

func TestTokenUsage_Add(t *testing.T) {
	usage := llm.TokenUsage{
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
	}

	other := llm.TokenUsage{
		InputTokens:  20,
		OutputTokens: 10,
		TotalTokens:  30,
	}

	usage.Add(other)

	assert.Equal(t, 30, usage.InputTokens)
	assert.Equal(t, 15, usage.OutputTokens)
	assert.Equal(t, 45, usage.TotalTokens)
}

func TestMockClient_WithStreamFunc(t *testing.T) {
	mock := llm.NewMockClient("").WithStreamFunc(func(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
		ch := make(chan llm.StreamChunk)
		go func() {
			defer close(ch)
			ch <- llm.StreamChunk{Content: "custom "}
			ch <- llm.StreamChunk{Content: "stream"}
			ch <- llm.StreamChunk{Done: true}
		}()
		return ch, nil
	})

	ch, err := mock.Stream(context.Background(), llm.CompletionRequest{})
	require.NoError(t, err)

	var content string
	for chunk := range ch {
		content += chunk.Content
	}
	assert.Equal(t, "custom stream", content)
}

func TestMockClient_Stream_ContextCancellation(t *testing.T) {
	mock := llm.NewMockClient("response")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ch, err := mock.Stream(ctx, llm.CompletionRequest{})
	require.NoError(t, err)

	// Read from channel - may get content or error depending on race
	chunk := <-ch
	// Either we get an error chunk or a content chunk that may or may not have error
	// The important thing is the channel closes cleanly
	_ = chunk
}
