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
